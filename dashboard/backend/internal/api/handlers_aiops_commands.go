package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

// 本文件实现 M2 意图执行（#94）：一句话 → 解析 → 用户确认 → 执行编排。
// 执行只复用既有写通道：gateway（写流量/倍速）、store（创建/启动实验），不新增越权入口。

// aiopsCommandSteps 是 aiops_commands.steps 的单个执行步骤记录。
type aiopsCommandSteps struct {
	Step       string `json:"step"`
	Status     string `json:"status"`
	Detail     string `json:"detail,omitempty"`
	OccurredAt string `json:"occurredAt"`
}

type createAIOpsCommandRequest struct {
	RawInput string `json:"rawInput"`
}

// handleListAIOpsTemplates 返回只读模板目录（LLM 与前端确认共用，防止编造 id）。
// handleGetAIOpsLimits 返回意图执行的硬限制（前端提示条展示，单一事实源）。
func (server *Server) handleGetAIOpsLimits(writer http.ResponseWriter, request *http.Request) {
	writeData(writer, request, http.StatusOK, aiops.Limits(), false, nil, nil)
}

func (server *Server) handleListAIOpsTemplates(writer http.ResponseWriter, request *http.Request) {
	if !server.requireAIOps(writer, request) {
		return
	}
	writeData(writer, request, http.StatusOK, aiops.TemplateCatalog, false, nil, nil)
}

// handleCreateAIOpsCommand 解析一句话意图并落库（status=parsed）；解析失败返回 400，不落库。
func (server *Server) handleCreateAIOpsCommand(writer http.ResponseWriter, request *http.Request) {
	if !server.requireAIOps(writer, request) {
		return
	}
	var body createAIOpsCommandRequest
	if err := decodeJSON(writer, request, server.config.HTTP.MaxBodyBytes, &body); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	body.RawInput = strings.TrimSpace(body.RawInput)
	if body.RawInput == "" {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_RAW_INPUT", "rawInput 不能为空。", false, nil)
		return
	}
	intent, err := server.aiops.ParseCommand(request.Context(), body.RawInput)
	if err != nil {
		writeProblem(writer, request, http.StatusUnprocessableEntity, "INTENT_PARSE_FAILED", err.Error(), false, nil)
		return
	}
	parsed, err := json.Marshal(intent)
	if err != nil {
		writeProblem(writer, request, http.StatusInternalServerError, "INTENT_ENCODE_FAILED", err.Error(), false, nil)
		return
	}
	command := model.AIOpsCommand{
		CommandID: randomIdentifier("cmd"),
		RawInput:  body.RawInput,
		Parsed:    parsed,
		Status:    string(model.AIOpsCommandParsed),
		Steps:     json.RawMessage(`[]`),
	}
	if err := server.store.CreateAIOpsCommand(request.Context(), command); err != nil {
		server.writeExperimentStoreError(writer, request, "记录意图命令失败", err)
		return
	}
	writeData(writer, request, http.StatusCreated, command, false, nil, nil)
}

// handleGetAIOpsCommand 返回一条意图命令（含解析结果与执行步骤）。
func (server *Server) handleGetAIOpsCommand(writer http.ResponseWriter, request *http.Request) {
	if !server.requireAIOps(writer, request) {
		return
	}
	command, err := server.store.GetAIOpsCommand(request.Context(), request.PathValue("id"))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeProblem(writer, request, http.StatusNotFound, "AI_OPS_COMMAND_NOT_FOUND",
				"意图命令不存在。", false, nil)
			return
		}
		server.writeExperimentStoreError(writer, request, "查询意图命令失败", err)
		return
	}
	writeData(writer, request, http.StatusOK, command, false, nil, nil)
}

// handleConfirmAIOpsCommand 确认并执行：gate 校验 → 写流量/调倍速 → 创建并启动实验 → done。
// 任一步失败即 failed（已执行步骤保留在 steps 中，不部分成功返回）。
func (server *Server) handleConfirmAIOpsCommand(writer http.ResponseWriter, request *http.Request) {
	if !server.requireAIOps(writer, request) {
		return
	}
	if !server.requireExperimentStore(writer, request) || !server.requireCache(writer, request) {
		return
	}
	commandID := request.PathValue("id")
	command, err := server.store.GetAIOpsCommand(request.Context(), commandID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeProblem(writer, request, http.StatusNotFound, "AI_OPS_COMMAND_NOT_FOUND",
				"意图命令不存在。", false, nil)
			return
		}
		server.writeExperimentStoreError(writer, request, "查询意图命令失败", err)
		return
	}
	if command.Status != string(model.AIOpsCommandParsed) {
		writeProblem(writer, request, http.StatusConflict, "AI_OPS_COMMAND_NOT_PARSED",
			fmt.Sprintf("意图命令当前状态为 %s，只有 parsed 状态可以确认执行。", command.Status), false, nil)
		return
	}
	var intent aiops.CommandIntent
	if err := json.Unmarshal(command.Parsed, &intent); err != nil {
		server.failAIOpsCommand(request.Context(), commandID, "解析确认请求失败", err)
		writeProblem(writer, request, http.StatusInternalServerError, "INTENT_DECODE_FAILED", err.Error(), false, nil)
		return
	}
	if err := aiops.ValidateCommandIntent(&intent); err != nil {
		server.failAIOpsCommand(request.Context(), commandID, "意图校验失败", err)
		writeProblem(writer, request, http.StatusUnprocessableEntity, "INTENT_INVALID", err.Error(), false, nil)
		return
	}
	if err := server.gateAIOpsCommand(request.Context(), commandID, &intent); err != nil {
		server.failAIOpsCommand(request.Context(), commandID, "执行门禁校验失败", err)
		writeProblem(writer, request, http.StatusUnprocessableEntity, "INTENT_GATE_REJECTED", err.Error(), false, nil)
		return
	}
	if err := server.executeAIOpsCommand(request.Context(), commandID, &intent); err != nil {
		server.failAIOpsCommand(request.Context(), commandID, "执行失败", err)
		writeProblem(writer, request, http.StatusInternalServerError, "AI_OPS_COMMAND_EXECUTION_FAILED", err.Error(), false, nil)
		return
	}
	updated, err := server.store.GetAIOpsCommand(request.Context(), commandID)
	if err != nil {
		server.writeExperimentStoreError(writer, request, "查询意图命令失败", err)
		return
	}
	writeData(writer, request, http.StatusOK, updated, false, nil, nil)
}

// gateAIOpsCommand 执行前校验：节点必须存在于集群（真实 Node 或 WorkerNode CR）、目标租户必须存在（模板 id 已由解析校验）。
func (server *Server) gateAIOpsCommand(ctx context.Context, commandID string, intent *aiops.CommandIntent) error {
	for _, name := range intent.TemplateSelection.NodeNames {
		if !server.nodeExists(name) {
			return fmt.Errorf("节点 %q 不存在", name)
		}
	}
	if intent.TargetTenant != "" {
		if !server.tenantExists(intent.TargetTenant) {
			return fmt.Errorf("目标租户 %q 不存在", intent.TargetTenant)
		}
	}
	return nil
}

func (server *Server) nodeExists(name string) bool {
	// 真实 Node 或抽象 WorkerNode CR 均视为可调度目标（模板节点与 CR 同名）。
	for _, node := range server.cache.ListNodes() {
		if node.Name == name {
			return true
		}
	}
	for _, object := range server.cache.ListPlatform("WorkerNode") {
		if object.GetName() == name {
			return true
		}
	}
	return false
}

func (server *Server) tenantExists(name string) bool {
	for _, object := range server.cache.ListPlatform("Tenant") {
		if object.GetName() == name {
			return true
		}
	}
	return false
}

// executeAIOpsCommand 按顺序执行：写流量 → 调倍速 → 创建实验 → 启动实验。
// 步骤逐步追加到 steps；任何失败返回错误，由调用方置 failed。
func (server *Server) executeAIOpsCommand(ctx context.Context, commandID string, intent *aiops.CommandIntent) error {
	steps := []aiopsCommandSteps{}
	record := func(step, status, detail string) {
		steps = append(steps, aiopsCommandSteps{
			Step: step, Status: status, Detail: detail, OccurredAt: time.Now().UTC().Format(time.RFC3339),
		})
		if err := server.store.UpdateAIOpsCommand(ctx, commandID, string(model.AIOpsCommandExecuting), mustJSON(steps), ""); err != nil {
			server.logger.Warn("AIOps command step persistence failed", "commandId", commandID, "step", step, "error", err)
		}
	}
	// 1. 写流量（目标租户 + 自由设计 QPS）
	if intent.TargetTenant != "" && intent.Traffic != nil && intent.Traffic.QPS != nil {
		if _, err := server.gateway.SetTenantQPS(ctx, intent.TargetTenant, *intent.Traffic.QPS, "", false); err != nil {
			record("set-traffic", "failed", err.Error())
			return fmt.Errorf("写流量失败：%w", err)
		}
		record("set-traffic", "done", fmt.Sprintf("%s qps=%d", intent.TargetTenant, *intent.Traffic.QPS))
	}
	// 2. 调倍速
	if intent.Rate != nil {
		if _, _, err := server.gateway.SetSimulationRate(ctx, *intent.Rate, "", false); err != nil {
			record("set-rate", "failed", err.Error())
			return fmt.Errorf("调倍速失败：%w", err)
		}
		record("set-rate", "done", fmt.Sprintf("rate=%d", *intent.Rate))
	}
	// 2.5 波形流量（潮汐/脉冲/斜坡）：倍速已生效后按模拟时间推进 QPS，墙钟结束归零
	if intent.Traffic != nil && intent.Traffic.Shape != "" &&
		intent.Traffic.Shape != string(aiops.TrafficShapeSteady) && intent.Traffic.PeakQPS != nil {
		periodMinutes := aiops.DefaultTidalPeriod
		if intent.Traffic.PeriodMinutes != nil && *intent.Traffic.PeriodMinutes > 0 {
			periodMinutes = *intent.Traffic.PeriodMinutes
		}
		wallMinutes := 1
		if intent.DurationMinutes > 0 && intent.Rate != nil && *intent.Rate > 0 {
			wallMinutes = max(1, intent.DurationMinutes/(*intent.Rate))
		}
		server.startTrafficShape(context.Background(), commandID, intent.TargetTenant, intent)
		record("traffic-shape", "done", fmt.Sprintf("%s peak=%d period=%dmin wall=%dmin",
			intent.Traffic.Shape, *intent.Traffic.PeakQPS, periodMinutes, wallMinutes))
	}
	// 3. 创建实验（配置快照定格当前集群状态）
	snapshot, err := json.Marshal(server.aggregator.CurrentSnapshot(time.Now().UTC()))
	if err != nil {
		record("create-experiment", "failed", err.Error())
		return fmt.Errorf("生成配置快照失败：%w", err)
	}
	tenant := intent.TargetTenant
	if tenant == "" {
		tenant = "default"
	}
	name := aiopsExperimentName(intent)
	segmentRecord := store.SegmentRecord{
		SegmentID:      randomIdentifier("segment"),
		Tenant:         tenant,
		Name:           name,
		Status:         string(model.SegmentPending),
		ConfigSnapshot: snapshot,
	}
	if err := server.store.CreateSegment(ctx, segmentRecord); err != nil {
		record("create-experiment", "failed", err.Error())
		return fmt.Errorf("创建实验失败：%w", err)
	}
	record("create-experiment", "done", segmentRecord.SegmentID)
	// 4. 启动实验
	startSnapshot, err := json.Marshal(server.aggregator.CurrentSnapshot(time.Now().UTC()))
	if err != nil {
		record("start-experiment", "failed", err.Error())
		return fmt.Errorf("生成起点快照失败：%w", err)
	}
	if err := server.store.UpdateSegmentLifecycle(ctx, segmentRecord.SegmentID, string(model.SegmentRunning), "", startSnapshot, nil); err != nil {
		record("start-experiment", "failed", err.Error())
		return fmt.Errorf("启动实验失败：%w", err)
	}
	record("start-experiment", "done", segmentRecord.SegmentID)
	return server.store.UpdateAIOpsCommand(ctx, commandID, string(model.AIOpsCommandDone), mustJSON(steps), "")
}

// startTrafficShape 按模拟时间推进波形流量（倍速下墙钟 = 模拟时长/倍速）；
// 使用独立 context（请求结束不中断），墙钟到点后把租户 QPS 归零，避免残留流量。
func (server *Server) startTrafficShape(ctx context.Context, commandID, tenant string, intent *aiops.CommandIntent) {
	rate := 1
	if intent.Rate != nil && *intent.Rate > 0 {
		rate = *intent.Rate
	}
	wallSeconds := 1
	if intent.DurationMinutes > 0 {
		wallSeconds = max(1, intent.DurationMinutes*60/rate)
	}
	periodMinutes := aiops.DefaultTidalPeriod
	if intent.Traffic.PeriodMinutes != nil && *intent.Traffic.PeriodMinutes > 0 {
		periodMinutes = *intent.Traffic.PeriodMinutes
	}
	shape := aiops.TrafficShape(intent.Traffic.Shape)
	peak := *intent.Traffic.PeakQPS
	server.logger.Info("AIOps traffic shape started",
		"commandId", commandID, "tenant", tenant, "shape", string(shape),
		"peakQps", peak, "periodMinutes", periodMinutes, "wallSeconds", wallSeconds)
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		elapsed := 0
		for range ticker.C {
			elapsed++
			simMinutes := float64(elapsed) / 60 * float64(rate)
			qps := aiops.TrafficShapeQPS(simMinutes, shape, peak, periodMinutes)
			if _, err := server.gateway.SetTenantQPS(ctx, tenant, qps, "", false); err != nil {
				server.logger.Warn("AIOps traffic shape write failed", "commandId", commandID, "error", err)
			}
			if elapsed >= wallSeconds {
				if _, err := server.gateway.SetTenantQPS(ctx, tenant, 0, "", false); err != nil {
					server.logger.Warn("AIOps traffic shape reset failed", "commandId", commandID, "error", err)
				}
				server.logger.Info("AIOps traffic shape finished, qps reset to 0", "commandId", commandID, "tenant", tenant)
				return
			}
		}
	}()
}

func (server *Server) failAIOpsCommand(ctx context.Context, commandID, summary string, cause error) {
	message := summary + "：" + cause.Error()
	if len(message) > 400 {
		message = message[:400]
	}
	if err := server.store.UpdateAIOpsCommand(ctx, commandID, string(model.AIOpsCommandFailed), nil, message); err != nil {
		server.logger.Warn("AIOps command fail persistence failed", "commandId", commandID, "error", err)
	}
}

// aiopsExperimentName 生成实验名：场景类型优先，回退到 rawInput 截断（≤63 字符）。
func aiopsExperimentName(intent *aiops.CommandIntent) string {
	if strings.TrimSpace(intent.SceneType) != "" {
		name := strings.TrimSpace(intent.SceneType)
		if len([]rune(name)) > 63 {
			name = string([]rune(name)[:63])
		}
		return name
	}
	return "AI 意图实验"
}

func mustJSON(value any) json.RawMessage {
	payload, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`[]`)
	}
	return payload
}
