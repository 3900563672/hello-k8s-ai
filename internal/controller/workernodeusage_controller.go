package controller

import (
	"context"
	"fmt"
	"math"
	"slices"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	"github.com/3900563672/hello-k8s-ai/internal/k8sutil"
	"github.com/3900563672/hello-k8s-ai/internal/observability"

	"go.opentelemetry.io/otel/attribute"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// podNodeNameIndex 是 Pod 的调度节点索引，避免每个节点 Reconcile 都遍历全部 Pod。
const podNodeNameIndex = "spec.nodeName"

// WorkerNodeUsageReconciler 是 WorkerNode 状态里用量的唯一写入者。
// 根据调度上去的、非终态的模拟器 Pod 推算 GPU 和并发占用。
type WorkerNodeUsageReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=platform.study.com,resources=models,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=workernodes,verbs=get;list;watch
// +kubebuilder:rbac:groups=platform.study.com,resources=workernodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

func (r *WorkerNodeUsageReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (result ctrl.Result, reconcileErr error) {
	ctx, observation := beginReconcile(ctx, "worker-node-usage", req)
	defer func() { observation.finish(result, reconcileErr) }()

	var node platformv1.WorkerNode
	if err := r.Get(ctx, req.NamespacedName, &node); err != nil {
		if apierrors.IsNotFound(err) {
			// 节点没了，清理对应的指标
			workerNodeGPUUnitsUsed.DeleteLabelValues(req.Name)
			workerNodeConcurrencyUsed.DeleteLabelValues(req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("get worker node %q: %w", req.Name, err)
	}

	usageCtx, usageSpan := startOperation(
		ctx,
		"worker-node-usage",
		"calculate",
		attribute.String("worker_node.name", node.Name),
	)
	// 根据当前 Pod 分布计算已用量
	usedGPU, usedConcurrency, err := r.calculateNodeUsage(usageCtx, node.Name)
	if err != nil {
		return ctrl.Result{}, err
	}
	// 物理水位：读同名真实 Node 的 allocatable 与节点全部非终态 Pod 的 requests。
	// 模拟环境下 WorkerNode 可能与真实 Node 同名（部署脚本按节点名生成）；
	// 找不到同名 Node 时保持缺省水位，不阻塞逻辑用量更新。
	memoryPercent, cpuPercent, hasPhysicalNode, err := r.calculateNodePhysicalPressure(usageCtx, node.Name)
	observability.EndSpan(
		usageSpan,
		err,
		attribute.Int("worker_node.gpu_units_used", usedGPU),
		attribute.Int("worker_node.concurrency_used", usedConcurrency),
		attribute.Int("worker_node.memory_usage_percent", memoryPercent),
		attribute.Int("worker_node.cpu_usage_percent", cpuPercent),
		attribute.Bool("worker_node.physical_node_found", hasPhysicalNode),
	)
	observeOperation("worker-node-usage", "physical-pressure", err)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.updateWorkerNodeStatus(
		ctx,
		node.Name,
		usedGPU,
		usedConcurrency,
		memoryPercent,
		cpuPercent,
		hasPhysicalNode,
	); err != nil {
		return ctrl.Result{}, err
	}
	// 更新 Prometheus 指标
	workerNodeGPUUnitsUsed.WithLabelValues(node.Name).Set(float64(usedGPU))
	workerNodeConcurrencyUsed.WithLabelValues(node.Name).Set(float64(usedConcurrency))
	workerNodeMemoryUsagePercent.WithLabelValues(node.Name).Set(float64(memoryPercent))
	workerNodeCPUUsagePercent.WithLabelValues(node.Name).Set(float64(cpuPercent))
	observation.span.SetAttributes(
		attribute.Int("worker_node.gpu_units_used", usedGPU),
		attribute.Int("worker_node.concurrency_used", usedConcurrency),
		attribute.Int("worker_node.memory_usage_percent", memoryPercent),
		attribute.Int("worker_node.cpu_usage_percent", cpuPercent),
		attribute.Bool("worker_node.physical_node_found", hasPhysicalNode),
	)
	return ctrl.Result{}, nil
}

// calculateNodeUsage 统计指定节点上所有非终态模拟器 Pod 的 GPU 和并发占用。
func (r *WorkerNodeUsageReconciler) calculateNodeUsage(ctx context.Context, nodeName string) (int, int, error) {
	// 先把所有 Model 拉出来，后面用 Pod 上的 model 标注去找
	var models platformv1.ModelList
	if err := r.List(ctx, &models); err != nil {
		return 0, 0, fmt.Errorf("list models for worker node usage: %w", err)
	}
	modelByName := make(map[string]*platformv1.Model, len(models.Items))
	for i := range models.Items {
		modelByName[models.Items[i].Name] = &models.Items[i]
	}

	var pods corev1.PodList
	// 只拉取调度到当前节点的 Pod，避免每个节点都遍历集群全部 Pod。
	if err := r.List(ctx, &pods, client.MatchingFields{podNodeNameIndex: nodeName}); err != nil {
		return 0, 0, fmt.Errorf("list pods for worker node usage: %w", err)
	}
	usedGPU := 0
	usedConcurrency := 0
	for i := range pods.Items {
		pod := &pods.Items[i]
		// 只统计非 Succeeded/Failed 的 Pod（NodeName 已由索引保证）
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		// 确认是模拟器 Pod（有标签或注解）
		if pod.Labels[instanceLabelKey] == "" && pod.Annotations[instanceNameAnnotation] == "" {
			continue
		}
		// 从注解或标签取模型名
		modelName := pod.Annotations[modelNameAnnotation]
		if modelName == "" {
			modelName = pod.Labels[modelLabelKey]
		}
		model := modelByName[modelName]
		if model == nil || !model.DeletionTimestamp.IsZero() {
			continue
		}
		usedGPU += nonNegative(model.Spec.GPUUnits)
		usedConcurrency += nonNegative(model.Spec.MaxConcurrency)
	}
	return usedGPU, usedConcurrency, nil
}

// calculateNodePhysicalPressure 计算节点物理水位：同名真实 Node 的 allocatable 与
// 节点上全部非终态 Pod 的 requests 之比（0-100）。模拟环境下 WorkerNode 与真实
// Node 同名（部署脚本按节点名生成）；找不到同名 Node 时返回 hasPhysicalNode=false，
// 调用方保留缺省水位。
func (r *WorkerNodeUsageReconciler) calculateNodePhysicalPressure(
	ctx context.Context,
	nodeName string,
) (memoryPercent int, cpuPercent int, hasPhysicalNode bool, err error) {
	var physicalNode corev1.Node
	if err := r.Get(ctx, client.ObjectKey{Name: nodeName}, &physicalNode); err != nil {
		if apierrors.IsNotFound(err) {
			return 0, 0, false, nil
		}
		return 0, 0, false, fmt.Errorf("get physical node %q: %w", nodeName, err)
	}
	allocatableMemory := physicalNode.Status.Allocatable[corev1.ResourceMemory]
	allocatableCPU := physicalNode.Status.Allocatable[corev1.ResourceCPU]
	if allocatableMemory.IsZero() || allocatableCPU.IsZero() {
		return 0, 0, true, nil
	}

	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.MatchingFields{podNodeNameIndex: nodeName}); err != nil {
		return 0, 0, true, fmt.Errorf("list pods for physical pressure: %w", err)
	}
	var requestedMemory, requestedCPU resource.Quantity
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		containers := append(pod.Spec.InitContainers, pod.Spec.Containers...)
		for _, container := range containers {
			requestedMemory.Add(container.Resources.Requests[corev1.ResourceMemory])
			requestedCPU.Add(container.Resources.Requests[corev1.ResourceCPU])
		}
	}
	return usagePercent(requestedMemory.Value(), allocatableMemory.Value()),
		usagePercent(requestedCPU.MilliValue(), allocatableCPU.MilliValue()),
		true,
		nil
}

// usagePercent 计算用量占容量的百分比并截断到 0-100；容量非正时返回 0。
func usagePercent(used, capacity int64) int {
	if capacity <= 0 {
		return 0
	}
	return max(0, min(100, int(math.Round(float64(used)*100/float64(capacity)))))
}

func (r *WorkerNodeUsageReconciler) updateWorkerNodeStatus(
	ctx context.Context,
	nodeName string,
	usedGPU int,
	usedConcurrency int,
	memoryPercent int,
	cpuPercent int,
	hasPhysicalNode bool,
) error {
	usedGPU = nonNegative(usedGPU)
	usedConcurrency = nonNegative(usedConcurrency)
	memoryPercent = nonNegative(memoryPercent)
	cpuPercent = nonNegative(cpuPercent)
	return k8sutil.PatchStatusWithRetry(ctx, r.Client, nodeName, true,
		func() *platformv1.WorkerNode { return &platformv1.WorkerNode{} },
		func(node *platformv1.WorkerNode) error {
			node.Status.UsedGPU = usedGPU
			node.Status.UsedConcurrency = usedConcurrency
			node.Status.MemoryUsagePercent = memoryPercent
			node.Status.CPUUsagePercent = cpuPercent
			meta.SetStatusCondition(&node.Status.Conditions, metav1.Condition{
				Type:               "UsageReady",
				Status:             metav1.ConditionTrue,
				ObservedGeneration: node.Generation,
				Reason:             "PodsAccounted",
				Message:            "scheduled simulator pods have been accounted",
			})
			if hasPhysicalNode {
				pressureStatus := metav1.ConditionFalse
				pressureReason := "PressureNormal"
				if memoryPercent >= physicalPressureThresholdPercent ||
					cpuPercent >= physicalPressureThresholdPercent {
					pressureStatus = metav1.ConditionTrue
					pressureReason = "PressureThresholdExceeded"
				}
				meta.SetStatusCondition(&node.Status.Conditions, metav1.Condition{
					Type:               conditionTypePhysicalPressure,
					Status:             pressureStatus,
					ObservedGeneration: node.Generation,
					Reason:             pressureReason,
					Message:            fmt.Sprintf("memory=%d%% cpu=%d%%", memoryPercent, cpuPercent),
				})
			}
			return nil
		})
}

// allWorkerNodeRequests 把 Pod 或 Model 变化映射为 WorkerNode Reconcile 请求。
// 已调度 Pod 只影响其所在节点，直接入队该节点；Model 事件与未调度 Pod 无法定位节点，广播全部节点兜底。
func (r *WorkerNodeUsageReconciler) allWorkerNodeRequests(ctx context.Context, obj client.Object) []reconcile.Request {
	if pod, ok := obj.(*corev1.Pod); ok && pod.Spec.NodeName != "" {
		return []reconcile.Request{{NamespacedName: client.ObjectKey{Name: pod.Spec.NodeName}}}
	}
	var nodes platformv1.WorkerNodeList
	if err := r.List(ctx, &nodes); err != nil {
		return nil
	}
	names := make([]string, 0, len(nodes.Items))
	for i := range nodes.Items {
		names = append(names, nodes.Items[i].Name)
	}
	slices.Sort(names)
	requests := make([]reconcile.Request, 0, len(names))
	for _, name := range names {
		requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Name: name}})
	}
	return requests
}

func (r *WorkerNodeUsageReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Pod 调度节点变化、Phase 变化、model 标注变化或删除时触发更新
	podChanged := lifecyclePredicate(func(e event.UpdateEvent) bool {
		oldPod, oldOK := e.ObjectOld.(*corev1.Pod)
		newPod, newOK := e.ObjectNew.(*corev1.Pod)
		return oldOK && newOK &&
			(oldPod.Spec.NodeName != newPod.Spec.NodeName ||
				oldPod.Status.Phase != newPod.Status.Phase ||
				oldPod.Annotations[modelNameAnnotation] != newPod.Annotations[modelNameAnnotation] ||
				oldPod.Labels[modelLabelKey] != newPod.Labels[modelLabelKey] ||
				!oldPod.DeletionTimestamp.Equal(newPod.DeletionTimestamp))
	})

	if err := registerFieldIndexes(
		context.Background(),
		mgr,
		"workernodeusage",
		fieldIndex{
			object: &corev1.Pod{},
			field:  podNodeNameIndex,
			extractor: func(obj client.Object) []string {
				return nonEmptyIndexValue(obj.(*corev1.Pod).Spec.NodeName)
			},
		},
	); err != nil {
		return err
	}

	controllerBuilder := ctrl.NewControllerManagedBy(mgr).
		Named("workernodeusage").
		For(&platformv1.WorkerNode{}).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.allWorkerNodeRequests),
			builder.WithPredicates(podChanged),
		)
	watchGenerationChanges(controllerBuilder, &platformv1.Model{}, r.allWorkerNodeRequests)
	return controllerBuilder.Complete(r)
}
