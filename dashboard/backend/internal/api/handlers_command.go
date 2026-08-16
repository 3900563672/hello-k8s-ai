package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/kubernetes"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type applyConfigurationRequest struct {
	Resources []kubernetes.ApplyIntent `json:"resources"`
	DryRun    bool                     `json:"dryRun"`
}

type tenantTrafficRequest struct {
	QPS             int    `json:"qps"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
	DryRun          bool   `json:"dryRun"`
}

type simulationRateRequest struct {
	Rate            int    `json:"rate"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
	DryRun          bool   `json:"dryRun"`
}

func (server *Server) handleApplyConfiguration(writer http.ResponseWriter, request *http.Request) {
	if !server.requireCache(writer, request) {
		return
	}
	var body applyConfigurationRequest
	if err := decodeJSON(writer, request, server.config.HTTP.MaxBodyBytes, &body); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	if len(body.Resources) == 0 || len(body.Resources) > 100 {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_RESOURCE_COUNT", "resources must contain between 1 and 100 items.", false, nil)
		return
	}

	// 持久化写入前，先通过 API Server dry-run 校验整个批次。
	if !body.DryRun {
		for _, intent := range body.Resources {
			if _, _, err := server.gateway.Apply(request.Context(), intent, true); err != nil {
				server.writeCommandError(writer, request, err)
				return
			}
		}
	}

	operationID := randomIdentifier("op")
	acceptedAt := time.Now().UTC()
	results, failed := server.applyConfigurationBatch(server.gateway, request, operationID, body.Resources, body.DryRun)
	state := map[bool]string{true: "validated", false: "accepted"}[body.DryRun]
	partial := false
	var warnings []string
	if failed != nil {
		// 前序资源已生效，不能伪装成整体失败；返回逐项成功/失败明细。
		state, partial = "partial", true
		warnings = []string{"部分资源应用失败；已成功项与失败明细见 results。"}
		results = append(results, *failed)
	}
	receipt := model.OperationReceipt{
		OperationID: operationID,
		AcceptedAt:  acceptedAt,
		State:       state,
		Results:     results,
	}
	writeData(writer, request, http.StatusAccepted, receipt, partial, warnings, sourceVersions(server.cache.SyncedAt()))
}

// resourceApplier 抽象 Kubernetes 写操作，便于对批量应用的部分失败语义做单元测试。
type resourceApplier interface {
	Apply(context.Context, kubernetes.ApplyIntent, bool) (*unstructured.Unstructured, string, error)
}

// applyConfigurationBatch 顺序应用一批配置。遇到失败时停止并返回已成功结果与失败明细，
// 由调用方以 partial 状态返回，避免"前 N-1 个已生效但客户端只看到整体失败"。
func (server *Server) applyConfigurationBatch(applier resourceApplier, request *http.Request, operationID string, resources []kubernetes.ApplyIntent, dryRun bool) ([]model.OperationResourceResult, *model.OperationResourceResult) {
	results := make([]model.OperationResourceResult, 0, len(resources))
	for _, intent := range resources {
		object, action, err := applier.Apply(request.Context(), intent, dryRun)
		if err != nil {
			server.recordAudit(request, operationID, actionOrDefault(action, "apply"), model.ResourceRef{APIVersion: "platform.study.com/v1", Kind: intent.Kind, Name: intent.Name}, "error", err)
			return results, &model.OperationResourceResult{
				Ref:         model.ResourceRef{APIVersion: "platform.study.com/v1", Kind: intent.Kind, Name: intent.Name},
				Action:      actionOrDefault(action, "apply"),
				Convergence: "failed",
				Error:       err.Error(),
			}
		}
		ref := model.ResourceRef{APIVersion: object.GetAPIVersion(), Kind: intent.Kind, Name: object.GetName(), UID: string(object.GetUID())}
		results = append(results, model.OperationResourceResult{
			Ref:             ref,
			Action:          action,
			ResourceVersion: object.GetResourceVersion(),
			Convergence:     map[bool]string{true: "dry-run", false: "pending"}[dryRun],
		})
		server.recordAudit(request, operationID, action, ref, "accepted", nil)
	}
	return results, nil
}

func (server *Server) handleDeleteConfiguration(writer http.ResponseWriter, request *http.Request) {
	if !server.requireCache(writer, request) {
		return
	}
	kind := request.PathValue("kind")
	name := request.PathValue("name")
	dryRun := request.URL.Query().Get("dryRun") == "true"
	resourceVersion := strings.Trim(strings.TrimSpace(request.Header.Get("If-Match")), "\"")
	operationID := randomIdentifier("op")
	ref := model.ResourceRef{APIVersion: "platform.study.com/v1", Kind: kind, Name: name}
	if err := server.gateway.Delete(request.Context(), kind, name, resourceVersion, dryRun); err != nil {
		server.recordAudit(request, operationID, "delete", ref, "error", err)
		server.writeCommandError(writer, request, err)
		return
	}
	server.recordAudit(request, operationID, "delete", ref, "accepted", nil)
	receipt := model.OperationReceipt{
		OperationID: operationID,
		AcceptedAt:  time.Now().UTC(),
		State:       map[bool]string{true: "validated", false: "accepted"}[dryRun],
		Results: []model.OperationResourceResult{{
			Ref: ref, Action: "delete", Convergence: map[bool]string{true: "dry-run", false: "pending"}[dryRun],
		}},
	}
	writeData(writer, request, http.StatusAccepted, receipt, false, nil, sourceVersions(server.cache.SyncedAt()))
}

func (server *Server) handleTenantTraffic(writer http.ResponseWriter, request *http.Request) {
	if !server.requireCache(writer, request) {
		return
	}
	var body tenantTrafficRequest
	if err := decodeJSON(writer, request, server.config.HTTP.MaxBodyBytes, &body); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	name := request.PathValue("name")
	operationID := randomIdentifier("op")
	object, err := server.gateway.SetTenantQPS(request.Context(), name, body.QPS, body.ResourceVersion, body.DryRun)
	ref := model.ResourceRef{APIVersion: "platform.study.com/v1", Kind: "Tenant", Name: name}
	if err != nil {
		server.recordAudit(request, operationID, "set-tenant-qps", ref, "error", err)
		server.writeCommandError(writer, request, err)
		return
	}
	ref.UID = string(object.GetUID())
	server.recordAudit(request, operationID, "set-tenant-qps", ref, "accepted", nil)
	receipt := model.OperationReceipt{
		OperationID: operationID,
		AcceptedAt:  time.Now().UTC(),
		State:       map[bool]string{true: "validated", false: "accepted"}[body.DryRun],
		Results: []model.OperationResourceResult{{
			Ref:             ref,
			Action:          "update",
			ResourceVersion: object.GetResourceVersion(),
			Convergence:     map[bool]string{true: "dry-run", false: "pending"}[body.DryRun],
		}},
	}
	writeData(writer, request, http.StatusAccepted, receipt, false, nil, sourceVersions(server.cache.SyncedAt()))
}

func (server *Server) handleSimulationRate(writer http.ResponseWriter, request *http.Request) {
	if !server.requireCache(writer, request) {
		return
	}
	var body simulationRateRequest
	if err := decodeJSON(writer, request, server.config.HTTP.MaxBodyBytes, &body); err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_REQUEST", err.Error(), false, nil)
		return
	}
	operationID := randomIdentifier("op")
	object, action, err := server.gateway.SetSimulationRate(
		request.Context(),
		body.Rate,
		body.ResourceVersion,
		body.DryRun,
	)
	ref := model.ResourceRef{
		APIVersion: "platform.study.com/v1",
		Kind:       "SimulationClock",
		Name:       "default",
	}
	if err != nil {
		server.recordAudit(request, operationID, "set-simulation-rate", ref, "error", err)
		server.writeCommandError(writer, request, err)
		return
	}
	ref.UID = string(object.GetUID())
	server.recordAudit(request, operationID, "set-simulation-rate", ref, "accepted", nil)
	receipt := model.OperationReceipt{
		OperationID: operationID,
		AcceptedAt:  time.Now().UTC(),
		State:       map[bool]string{true: "validated", false: "accepted"}[body.DryRun],
		Results: []model.OperationResourceResult{{
			Ref:             ref,
			Action:          action,
			ResourceVersion: object.GetResourceVersion(),
			Convergence:     map[bool]string{true: "dry-run", false: "pending"}[body.DryRun],
		}},
	}
	writeData(writer, request, http.StatusAccepted, receipt, false, nil, sourceVersions(server.cache.SyncedAt()))
}

func (server *Server) recordAudit(request *http.Request, operationID, action string, ref model.ResourceRef, outcome string, commandErr error) {
	if !server.store.Available() {
		return
	}
	actor := actorName(request, server.config.HTTP.TrustRemoteUser)
	details, _ := json.Marshal(map[string]any{
		"idempotencyKey": request.Header.Get("Idempotency-Key"),
		"error":          errorText(commandErr),
	})
	record := model.AuditRecord{
		OperationID: operationID,
		OccurredAt:  time.Now().UTC(),
		Actor:       actor,
		Action:      action,
		Ref:         ref,
		Outcome:     outcome,
		RequestID:   requestID(request.Context()),
		Details:     details,
	}
	// 客户端断开不能丢审计：使用独立于请求生命周期的超时上下文。
	auditContext, cancel := context.WithTimeout(context.WithoutCancel(request.Context()), 5*time.Second)
	defer cancel()
	if err := server.store.RecordAudit(auditContext, record); err != nil {
		server.logger.Error("Could not persist Dashboard command audit event", "operationId", operationID, "error", err)
	}
}

func (server *Server) writeCommandError(writer http.ResponseWriter, request *http.Request, err error) {
	status := http.StatusBadRequest
	code := "COMMAND_REJECTED"
	retryable := false
	switch {
	case apierrors.IsNotFound(err):
		status, code = http.StatusNotFound, "RESOURCE_NOT_FOUND"
	case apierrors.IsConflict(err):
		status, code = http.StatusConflict, "RESOURCE_VERSION_CONFLICT"
		retryable = true
	case apierrors.IsForbidden(err):
		status, code = http.StatusForbidden, "KUBERNETES_FORBIDDEN"
	case apierrors.IsInvalid(err):
		status, code = http.StatusUnprocessableEntity, "KUBERNETES_VALIDATION_FAILED"
	case apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) || apierrors.IsTooManyRequests(err):
		status, code, retryable = http.StatusServiceUnavailable, "KUBERNETES_TEMPORARILY_UNAVAILABLE", true
	}
	writeProblem(writer, request, status, code, err.Error(), retryable, nil)
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, maxBytes int64, target any) error {
	request.Body = http.MaxBytesReader(writer, request.Body, maxBytes)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode JSON request: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("request body must contain exactly one JSON value")
		}
		return fmt.Errorf("decode trailing JSON data: %w", err)
	}
	return nil
}

func actionOrDefault(action, fallback string) string {
	if action == "" {
		return fallback
	}
	return action
}
