package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	"github.com/3900563672/hello-k8s-ai/internal/observability"

	"go.opentelemetry.io/otel/attribute"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errStaleDecision = errors.New("scaling decision is stale")

// scalingPlan 跟副本数更新一起写入注解，保证扩缩容操作可恢复。
// 如果副本变更成功后状态写入失败，下一次 reconcile 会继续完成同一个计划，不会重复扩缩。
type scalingPlan struct {
	TriggerID        string      `json:"triggerID"`
	TenantName       string      `json:"tenantName"`
	OrchestratorName string      `json:"orchestratorName"`
	Action           string      `json:"action"`
	InstanceName     string      `json:"instanceName"`
	SourceNodeName   string      `json:"sourceNodeName,omitempty"`
	NodeName         string      `json:"nodeName"`
	OldReplicas      int         `json:"oldReplicas"`
	NewReplicas      int         `json:"newReplicas"`
	EffectiveScore   int         `json:"effectiveScore,omitempty"`
	Time             metav1.Time `json:"time"`
}

// applyDecision 执行扩缩容决策：先持久化计划，再完成后续的状态更新。
func (r *OrchestratorReconciler) applyDecision(
	ctx context.Context,
	decision Decision,
	input DecisionInput,
) (operationErr error) {
	if decision.Action == NoOp {
		return nil
	}

	direction := decisionActionLabel(decision.Action)
	defer func() {
		outcome := operationOutcomeSuccess
		if operationErr != nil {
			outcome = operationOutcomeError
		}
		orchestratorScalingOperations.WithLabelValues(direction, outcome).Inc()
		observeOperation(componentOrchestrator, "scale", operationErr)
	}()

	// 安全校验，防止写出非法的副本数
	if decision.TargetReplicas < 0 {
		return fmt.Errorf("refusing to write invalid replicas %d", decision.TargetReplicas)
	}
	if decision.NodeName == "" {
		return fmt.Errorf("refusing to apply a scaling decision without a placement node")
	}
	if decision.Action == Rebalance && decision.SourceNodeName == "" {
		return fmt.Errorf("refusing to rebalance without a source node")
	}

	plan := scalingPlan{
		TriggerID:        input.TriggerID,
		TenantName:       input.TenantName,
		OrchestratorName: input.OrchestratorName,
		Action:           actionToString(decision.Action),
		InstanceName:     decision.InstanceName,
		SourceNodeName:   decision.SourceNodeName,
		NodeName:         decision.NodeName,
		OldReplicas:      decision.ObservedReplicas,
		NewReplicas:      decision.TargetReplicas,
		EffectiveScore:   decision.EffectiveScore,
		Time:             metav1.NewTime(r.currentTime()),
	}
	// 极端情况下 triggerID 为空（比如手动触发），补一个手工 ID
	if plan.TriggerID == "" {
		plan.TriggerID = fmt.Sprintf("manual:%s:%d:%d", plan.InstanceName, plan.OldReplicas, plan.NewReplicas)
	}

	persistCtx, persistSpan := startOperation(
		ctx,
		componentOrchestrator,
		"persist-scale-plan",
		attribute.String("scaling.direction", direction),
		attribute.String(traceAttributeSimulatorInstanceName, plan.InstanceName),
		attribute.String("placement.node_name", plan.NodeName),
		attribute.Int("scaling.old_replicas", plan.OldReplicas),
		attribute.Int("scaling.new_replicas", plan.NewReplicas),
	)
	if err := r.persistScalePlan(persistCtx, plan); err != nil {
		observability.EndSpan(persistSpan, err)
		return err
	}
	observability.EndSpan(persistSpan, nil)

	completeCtx, completeSpan := startOperation(ctx, componentOrchestrator, "complete-scale-plan")
	err := r.completeScalePlan(completeCtx, plan)
	observability.EndSpan(completeSpan, err)
	return err
}

// persistScalePlan 原子性地更新实例副本数，并把计划存进注解。
// 如果并发导致实例状态已变（租户名或副本数不匹配），直接返回 errStaleDecision，让外层重试。
func (r *OrchestratorReconciler) persistScalePlan(ctx context.Context, plan scalingPlan) error {
	payload, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal scaling plan: %w", err)
	}

	return retryOnConflict(func() error {
		var instance platformv1.SimulatorInstance
		if err := r.Get(ctx, client.ObjectKey{Name: plan.InstanceName}, &instance); err != nil {
			return err
		}
		// 如果已经应用过同一个 triggerID 的计划，说明本次是重试，不用再写了
		if plan.TriggerID != "" && instance.Annotations[lastScaleTriggerKey] == plan.TriggerID {
			return nil
		}
		// 检查实例状态是否跟决策时一致，不一致说明决策已经过时
		if instance.Spec.TenantRef.Name != plan.TenantName || instance.Spec.Replicas != plan.OldReplicas {
			return fmt.Errorf(
				"%w: instance %q expected tenant %q replicas %d, found tenant %q replicas %d",
				errStaleDecision,
				instance.Name,
				plan.TenantName,
				plan.OldReplicas,
				instance.Spec.TenantRef.Name,
				instance.Spec.Replicas,
			)
		}
		placementPlan, persisted, err := decodeNodePlacementPlan(instance.Annotations[nodePlacementsAnnotation])
		if err != nil {
			return fmt.Errorf("decode node placements on simulator instance %q: %w", instance.Name, err)
		}
		if !persisted {
			if instance.Spec.Replicas == 0 {
				placementPlan = nodePlacementPlan{Version: placementPlanVersion}
			} else {
				placementPlan, err = r.observeNodePlacementPlan(ctx, instance.Name)
				if err != nil {
					return err
				}
			}
		}
		if nodePlacementReplicaCount(placementPlan) != plan.OldReplicas {
			return fmt.Errorf(
				"%w: instance %q expected %d placed replicas, found %d",
				errStaleDecision,
				instance.Name,
				plan.OldReplicas,
				nodePlacementReplicaCount(placementPlan),
			)
		}
		switch plan.Action {
		case scalingActionUp:
			placementPlan, err = addNodePlacement(placementPlan, plan.NodeName)
		case scalingActionDown:
			placementPlan, err = removeNodePlacement(placementPlan, plan.NodeName)
		case scalingActionRebalance:
			placementPlan, err = removeNodePlacement(placementPlan, plan.SourceNodeName)
			if err == nil {
				placementPlan, err = addNodePlacement(placementPlan, plan.NodeName)
			}
		default:
			err = fmt.Errorf("unsupported scaling action %q", plan.Action)
		}
		if err != nil {
			return fmt.Errorf("update node placement plan for simulator instance %q: %w", instance.Name, err)
		}
		if nodePlacementReplicaCount(placementPlan) != plan.NewReplicas {
			return fmt.Errorf(
				"node placement plan for simulator instance %q contains %d replicas after scaling, want %d",
				instance.Name,
				nodePlacementReplicaCount(placementPlan),
				plan.NewReplicas,
			)
		}
		placementPayload, err := encodeNodePlacementPlan(placementPlan)
		if err != nil {
			return err
		}
		annotations := ensureStringMap(&instance.Annotations)
		annotations[lastScaleTriggerKey] = plan.TriggerID
		annotations[pendingScalePlanKey] = string(payload)
		annotations[nodePlacementsAnnotation] = placementPayload
		instance.Spec.Replicas = plan.NewReplicas
		// 利用 resourceVersion 防止并发覆盖
		return r.Update(ctx, &instance)
	})
}

// observeNodePlacementPlan 从旧版单 Deployment 的已调度 Pod 中恢复逐节点副本分布。
// 只有观察到的副本总数与 spec.replicas 一致时，调用方才会接受这份迁移快照。
func (r *OrchestratorReconciler) observeNodePlacementPlan(
	ctx context.Context,
	instanceName string,
) (nodePlacementPlan, error) {
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.MatchingLabels{managedByLabelKey: managedByLabelVal}); err != nil {
		return nodePlacementPlan{}, fmt.Errorf("list simulator pods for instance %q: %w", instanceName, err)
	}
	counts := make(map[string]int)
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Annotations[instanceNameAnnotation] != instanceName ||
			pod.Spec.NodeName == "" ||
			pod.Status.Phase == corev1.PodSucceeded ||
			pod.Status.Phase == corev1.PodFailed {
			continue
		}
		counts[pod.Spec.NodeName]++
	}
	plan, err := newNodePlacementPlan(counts)
	if err != nil {
		return nodePlacementPlan{}, fmt.Errorf("build observed node placements for instance %q: %w", instanceName, err)
	}
	return plan, nil
}

// completeScalePlan 完成扩缩容计划的收尾工作：写入有效分数、记录状态、清除待处理注解。
func (r *OrchestratorReconciler) completeScalePlan(ctx context.Context, plan scalingPlan) error {
	if plan.Action == scalingActionRebalance {
		return r.clearPendingScalePlan(ctx, plan.InstanceName, plan.TriggerID)
	}
	// 扩容时才需要更新有效分数，缩容不用
	if plan.Action == scalingActionUp {
		if err := r.updateInstanceEffectiveScore(ctx, plan.InstanceName, plan.EffectiveScore); err != nil {
			return err
		}
	}

	record := &platformv1.ScalingRecord{
		Time:         plan.Time,
		Action:       plan.Action,
		InstanceName: plan.InstanceName,
		OldReplicas:  plan.OldReplicas,
		NewReplicas:  plan.NewReplicas,
	}
	if err := r.updateOrchestratorStatusByName(ctx, plan.OrchestratorName, record); err != nil {
		return err
	}
	return r.clearPendingScalePlan(ctx, plan.InstanceName, plan.TriggerID)
}

// resumePendingScaling 恢复租户下所有未完成的扩缩容计划。
// 返回 true 表示至少恢复了一个计划，调用方应重新拉取快照再做决策。
func (r *OrchestratorReconciler) resumePendingScaling(ctx context.Context, input DecisionInput) (bool, error) {
	found := false
	for _, instance := range input.ExistingInstances {
		if instance.PendingScalePlan == "" {
			continue
		}
		found = true
		var plan scalingPlan
		if err := json.Unmarshal([]byte(instance.PendingScalePlan), &plan); err != nil {
			return true, fmt.Errorf("decode pending scaling plan on %q: %w", instance.Name, err)
		}
		// 计划必须与当前实例和租户匹配
		if plan.InstanceName != instance.Name || plan.TenantName != input.TenantName {
			return true, fmt.Errorf("pending scaling plan on %q does not match its instance or tenant", instance.Name)
		}
		if err := r.completeScalePlan(ctx, plan); err != nil {
			return true, err
		}
	}
	return found, nil
}

// clearPendingScalePlan 清除实例上的待处理计划注解。
// 只有当注解里记录的 triggerID 与当前一致时才清除，防止误删已被新计划覆盖的注解。
func (r *OrchestratorReconciler) clearPendingScalePlan(ctx context.Context, instanceName, triggerID string) error {
	return retryOnConflict(func() error {
		var instance platformv1.SimulatorInstance
		if err := r.Get(ctx, client.ObjectKey{Name: instanceName}, &instance); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if instance.Annotations[pendingScalePlanKey] == "" {
			return nil
		}
		if instance.Annotations[lastScaleTriggerKey] != triggerID {
			return fmt.Errorf("refusing to clear a superseded scaling plan on %q", instanceName)
		}
		before := instance.DeepCopy()
		delete(instance.Annotations, pendingScalePlanKey)
		return r.Patch(ctx, &instance, client.MergeFrom(before))
	})
}

// updateInstanceEffectiveScore 更新实例的有效分数，负数归零。
func (r *OrchestratorReconciler) updateInstanceEffectiveScore(ctx context.Context, instanceName string, score int) error {
	score = nonNegative(score)
	return retryOnConflict(func() error {
		var instance platformv1.SimulatorInstance
		if err := r.Get(ctx, client.ObjectKey{Name: instanceName}, &instance); err != nil {
			return err
		}
		if instance.Status.EffectiveScore != nil && *instance.Status.EffectiveScore == score {
			return nil
		}
		before := instance.DeepCopy()
		instance.Status.EffectiveScore = new(score)
		return r.Status().Patch(ctx, &instance, client.MergeFrom(before))
	})
}

// updateOrchestratorStatusByName 更新 Orchestrator 状态的 LastScaling 字段和方向时间戳。
func (r *OrchestratorReconciler) updateOrchestratorStatusByName(
	ctx context.Context,
	name string,
	record *platformv1.ScalingRecord,
) error {
	return retryOnConflict(func() error {
		var config platformv1.Orchestrator
		if err := r.Get(ctx, client.ObjectKey{Name: name}, &config); err != nil {
			return err
		}
		// 如果记录和时间戳都已经是最新的，不用再写
		if scalingRecordsEqual(config.Status.LastScaling, record) && scaleTimestampRecorded(&config, record) {
			return nil
		}
		before := config.DeepCopy()
		config.Status.LastScaling = record.DeepCopy()
		switch record.Action {
		case scalingActionUp:
			config.Status.LastScaleUpTime = record.Time.DeepCopy()
		case scalingActionDown:
			config.Status.LastScaleDownTime = record.Time.DeepCopy()
		}
		return r.Status().Patch(ctx, &config, client.MergeFrom(before))
	})
}

// scaleTimestampRecorded 检查指定方向的时间戳是否已经是 record 的时间。
func scaleTimestampRecorded(config *platformv1.Orchestrator, record *platformv1.ScalingRecord) bool {
	switch record.Action {
	case scalingActionUp:
		return config.Status.LastScaleUpTime != nil && config.Status.LastScaleUpTime.Equal(&record.Time)
	case scalingActionDown:
		return config.Status.LastScaleDownTime != nil && config.Status.LastScaleDownTime.Equal(&record.Time)
	default:
		return false
	}
}

// scalingRecordsEqual 比较两个 ScalingRecord 是否相等。
func scalingRecordsEqual(left, right *platformv1.ScalingRecord) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Time.Equal(&right.Time) &&
		left.Action == right.Action &&
		left.InstanceName == right.InstanceName &&
		left.OldReplicas == right.OldReplicas &&
		left.NewReplicas == right.NewReplicas
}

func actionToString(action DecisionAction) string {
	switch action {
	case ScaleUp:
		return scalingActionUp
	case ScaleDown:
		return scalingActionDown
	case Rebalance:
		return scalingActionRebalance
	default:
		return ""
	}
}
