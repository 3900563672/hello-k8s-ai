package aiops

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// entityFact 是喂给 L1 模型的单实体事实（紧凑、全字符串，控制 token）。
type entityFact struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Phase    string `json:"phase,omitempty"`
	Ready    string `json:"ready,omitempty"`
	Restarts int    `json:"restarts,omitempty"`
	Tenant   string `json:"tenant,omitempty"`
	Model    string `json:"model,omitempty"`
	Role     string `json:"role,omitempty"`
	Changes  string `json:"changes,omitempty"`
	Events   string `json:"events,omitempty"`
}

// l1EntityResult 是 L1 模型返回的单实体结果。
type l1EntityResult struct {
	EntityKind     string `json:"entityKind"`
	EntityName     string `json:"entityName"`
	Phenomenon     string `json:"phenomenon"`
	IssueFlag      bool   `json:"issueFlag"`
	Classification string `json:"classification"`
	Conclusion     string `json:"conclusion"`
}

// l1UserPrompt 序列化实体事实数组为模型输入。
func l1UserPrompt(entities []entityFact) (string, error) {
	payload, err := json.Marshal(entities)
	if err != nil {
		return "", fmt.Errorf("marshal entity facts: %w", err)
	}
	return string(payload), nil
}

// l2UserPrompt 组装 L2 输入：硬指标 + L1 摘要（只读摘要，不看原始数据）。
// #112 预算：摘要区超预算时按 分数>结论>现象>事件 优先级裁剪；返回是否发生裁剪。
func l2UserPrompt(hard hardMetrics, summaries []model.AIOpsEntitySummary, budget int, logger *slog.Logger) (string, bool, error) {
	trimmed, truncated := truncateSummaries(summaries, budget, logger)
	type input struct {
		Hard      hardMetrics                `json:"hard"`
		Summaries []model.AIOpsEntitySummary `json:"summaries"`
	}
	payload, err := json.Marshal(input{Hard: hard, Summaries: trimmed})
	if err != nil {
		return "", truncated, fmt.Errorf("marshal L2 input: %w", err)
	}
	return string(payload), truncated, nil
}

// extractEntities 从起点/终点快照提取实体事实（L1 全量覆盖，不做健康过滤）。
// 返回顺序：Pod（按名排序）→ Node → Tenant，便于前端稳定展示。
func extractEntities(start, end *model.CurrentSnapshot) []entityFact {
	if end == nil {
		return nil
	}
	byName := make(map[string]entityFact)
	order := make([]string, 0)

	// 起点 Pod 用于比较阶段变化；只保留仍存在的（在 end 里也会出现）。
	startPods := make(map[string]model.Pod)
	if start != nil {
		for _, pod := range start.Workloads.Pods {
			startPods[pod.Ref.Name] = pod
		}
	}
	for _, pod := range end.Workloads.Pods {
		name := pod.Ref.Name
		fact := entityFact{
			Kind:     "Pod",
			Name:     name,
			Phase:    pod.Phase,
			Ready:    fmt.Sprintf("%t", pod.Ready),
			Tenant:   pod.Tenant,
			Model:    pod.Model,
			Restarts: podRestarts(pod),
		}
		if previous, exists := startPods[name]; exists && previous.Phase != pod.Phase {
			fact.Changes = fmt.Sprintf("phase %s→%s", previous.Phase, pod.Phase)
		} else if !exists {
			fact.Changes = "created during segment"
		}
		if _, seen := byName[name]; !seen {
			order = append(order, name)
		}
		byName[name] = fact
	}
	// 起点存在但终点消失的 Pod。
	for _, pod := range start.Workloads.Pods {
		name := pod.Ref.Name
		if _, exists := byName[name]; exists {
			continue
		}
		fact := entityFact{
			Kind:    "Pod",
			Name:    name,
			Phase:   pod.Phase + "→gone",
			Ready:   "false",
			Tenant:  pod.Tenant,
			Model:   pod.Model,
			Changes: "removed during segment",
		}
		order = append(order, name)
		byName[name] = fact
	}

	seenNodes := make(map[string]bool)
	for _, node := range end.Workloads.Nodes {
		name := node.Ref.Name
		if seenNodes[name] {
			continue
		}
		seenNodes[name] = true
		fact := entityFact{
			Kind:    "Node",
			Name:    name,
			Ready:   fmt.Sprintf("%t", node.Ready),
			Role:    node.Role,
			Phase:   node.Phase,
			Changes: fmt.Sprintf("schedulable=%t", node.Schedulable),
		}
		order = append(order, "node:"+name)
		byName["node:"+name] = fact
	}

	tenantNames := make(map[string]bool)
	for _, tenant := range end.Configuration.Tenants {
		name := tenant.Ref.Name
		if tenantNames[name] {
			continue
		}
		tenantNames[name] = true
		fact := entityFact{Kind: "Tenant", Name: name}
		for _, traffic := range end.Traffic.Tenants {
			if traffic.Tenant.Name == name {
				fact.Changes = fmt.Sprintf("requestedQPS=%d allocatedQPS=%d readyReplicas=%d",
					traffic.RequestedQPS, traffic.AllocatedQPS, traffic.ReadyReplicaCount)
				break
			}
		}
		order = append(order, "tenant:"+name)
		byName["tenant:"+name] = fact
	}

	facts := make([]entityFact, 0, len(order))
	sort.Strings(order)
	for _, key := range order {
		facts = append(facts, byName[key])
	}
	return facts
}

func podRestarts(pod model.Pod) int {
	restarts := 0
	for _, container := range pod.Containers {
		restarts += int(container.RestartCount)
	}
	return restarts
}

// classifyEntity 规则兜底分类：问题（错误事件/重启/未就绪）→ 可疑（事件）→ 健康。
func classifyEntity(entity entityFact, relatedEvents []string) l1EntityResult {
	result := l1EntityResult{
		EntityKind:     entity.Kind,
		EntityName:     entity.Name,
		Classification: string(model.AIOpsHealthy),
		Phenomenon:     entity.Changes,
		Conclusion:     "未见异常。",
	}
	joined := strings.Join(relatedEvents, ";")
	if strings.Contains(joined, "error") || entity.Restarts > 0 ||
		(entity.Kind == "Pod" && (entity.Ready == "false" || strings.Contains(entity.Phase, "Failed") || strings.Contains(entity.Phase, "Pending"))) {
		result.IssueFlag = true
		result.Classification = string(model.AIOpsProblem)
		result.Conclusion = fmt.Sprintf("存在异常：错误事件/重启/未就绪（restarts=%d）。", entity.Restarts)
		if result.Phenomenon == "" {
			result.Phenomenon = joined
		}
		return result
	}
	if joined != "" {
		result.IssueFlag = true
		result.Classification = string(model.AIOpsSuspect)
		result.Conclusion = "存在可疑事件，建议关注。"
		if result.Phenomenon == "" {
			result.Phenomenon = joined
		}
	}
	return result
}
