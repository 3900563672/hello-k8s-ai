package controller

import (
	"context"
	"fmt"
	"slices"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	"github.com/3900563672/hello-k8s-ai/internal/k8sutil"
	"github.com/3900563672/hello-k8s-ai/internal/observability"

	"go.opentelemetry.io/otel/attribute"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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
	observability.EndSpan(
		usageSpan,
		err,
		attribute.Int("worker_node.gpu_units_used", usedGPU),
		attribute.Int("worker_node.concurrency_used", usedConcurrency),
	)
	observeOperation("worker-node-usage", "calculate", err)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := r.updateWorkerNodeStatus(ctx, node.Name, usedGPU, usedConcurrency); err != nil {
		return ctrl.Result{}, err
	}
	// 更新 Prometheus 指标
	workerNodeGPUUnitsUsed.WithLabelValues(node.Name).Set(float64(usedGPU))
	workerNodeConcurrencyUsed.WithLabelValues(node.Name).Set(float64(usedConcurrency))
	observation.span.SetAttributes(
		attribute.Int("worker_node.gpu_units_used", usedGPU),
		attribute.Int("worker_node.concurrency_used", usedConcurrency),
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

func (r *WorkerNodeUsageReconciler) updateWorkerNodeStatus(
	ctx context.Context,
	nodeName string,
	usedGPU int,
	usedConcurrency int,
) error {
	usedGPU = nonNegative(usedGPU)
	usedConcurrency = nonNegative(usedConcurrency)
	return k8sutil.PatchStatusWithRetry(ctx, r.Client, nodeName, true,
		func() *platformv1.WorkerNode { return &platformv1.WorkerNode{} },
		func(node *platformv1.WorkerNode) error {
			node.Status.UsedGPU = usedGPU
			node.Status.UsedConcurrency = usedConcurrency
			meta.SetStatusCondition(&node.Status.Conditions, metav1.Condition{
				Type:               "UsageReady",
				Status:             metav1.ConditionTrue,
				ObservedGeneration: node.Generation,
				Reason:             "PodsAccounted",
				Message:            "scheduled simulator pods have been accounted",
			})
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
