package kubernetes

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"sync/atomic"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const (
	nativePods        = "Pod"
	nativeNodes       = "Node"
	nativeServices    = "Service"
	nativeEvents      = "Event"
	nativeDeployments = "Deployment"
	nativeReplicaSets = "ReplicaSet"
	nativeLeases      = "Lease"
)

type ChangeSink func(model.ResourceChange)

type Cache struct {
	clients        *Clients
	config         config.KubernetesConfig
	logger         *slog.Logger
	typedFactory   informers.SharedInformerFactory
	dynamicFactory dynamicinformer.DynamicSharedInformerFactory
	platform       map[string]cache.SharedIndexInformer
	native         map[string]cache.SharedIndexInformer
	syncFunctions  []cache.InformerSynced
	changeSink     ChangeSink
	started        atomic.Bool
	synced         atomic.Bool
	syncedAt       atomic.Value
}

func NewCache(clients *Clients, cfg config.KubernetesConfig, logger *slog.Logger, sink ChangeSink) (*Cache, error) {
	if clients == nil || clients.Kubernetes == nil || clients.Dynamic == nil {
		return nil, errors.New("Kubernetes clients are required")
	}
	typedFactory := informers.NewSharedInformerFactory(clients.Kubernetes, cfg.ResyncPeriod)
	dynamicFactory := dynamicinformer.NewDynamicSharedInformerFactory(clients.Dynamic, cfg.ResyncPeriod)
	result := &Cache{
		clients:        clients,
		config:         cfg,
		logger:         logger,
		typedFactory:   typedFactory,
		dynamicFactory: dynamicFactory,
		platform:       make(map[string]cache.SharedIndexInformer, len(PlatformResources)),
		native:         make(map[string]cache.SharedIndexInformer, 7),
		changeSink:     sink,
	}

	for _, descriptor := range PlatformResources {
		informer := dynamicFactory.ForResource(descriptor.GVR).Informer()
		result.platform[descriptor.Kind] = informer
		result.syncFunctions = append(result.syncFunctions, informer.HasSynced)
		if err := result.registerHandler(informer, descriptor.Kind, descriptor.GVR.GroupVersion().String()); err != nil {
			return nil, err
		}
	}

	nativeInformers := []struct {
		kind       string
		apiVersion string
		informer   cache.SharedIndexInformer
	}{
		{nativePods, "v1", typedFactory.Core().V1().Pods().Informer()},
		{nativeNodes, "v1", typedFactory.Core().V1().Nodes().Informer()},
		{nativeServices, "v1", typedFactory.Core().V1().Services().Informer()},
		{nativeEvents, "v1", typedFactory.Core().V1().Events().Informer()},
		{nativeDeployments, "apps/v1", typedFactory.Apps().V1().Deployments().Informer()},
		{nativeReplicaSets, "apps/v1", typedFactory.Apps().V1().ReplicaSets().Informer()},
		{nativeLeases, "coordination.k8s.io/v1", typedFactory.Coordination().V1().Leases().Informer()},
	}
	for _, item := range nativeInformers {
		result.native[item.kind] = item.informer
		result.syncFunctions = append(result.syncFunctions, item.informer.HasSynced)
		if err := result.registerHandler(item.informer, item.kind, item.apiVersion); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (state *Cache) registerHandler(informer cache.SharedIndexInformer, kind, apiVersion string) error {
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(object any) {
			state.emit("add", kind, apiVersion, object)
		},
		UpdateFunc: func(oldObject, newObject any) {
			oldAccessor, oldErr := apiMeta.Accessor(oldObject)
			newAccessor, newErr := apiMeta.Accessor(newObject)
			if oldErr == nil && newErr == nil && oldAccessor.GetResourceVersion() == newAccessor.GetResourceVersion() {
				return
			}
			state.emit("update", kind, apiVersion, newObject)
		},
		DeleteFunc: func(object any) {
			if tombstone, ok := object.(cache.DeletedFinalStateUnknown); ok {
				object = tombstone.Obj
			}
			state.emit("delete", kind, apiVersion, object)
		},
	})
	if err != nil {
		return fmt.Errorf("register %s informer handler: %w", kind, err)
	}
	return nil
}

func (state *Cache) emit(operation, kind, apiVersion string, object any) {
	if state.changeSink == nil {
		return
	}
	accessor, err := apiMeta.Accessor(object)
	if err != nil {
		state.logger.Error("Could not read changed Kubernetes object", "kind", kind, "error", err)
		return
	}
	payload, err := json.Marshal(object)
	if err != nil {
		state.logger.Error("Could not serialize changed Kubernetes object", "kind", kind, "name", accessor.GetName(), "error", err)
		return
	}
	state.changeSink(model.ResourceChange{
		EventID:    randomID(),
		OccurredAt: time.Now().UTC(),
		Operation:  operation,
		Ref: model.ResourceRef{
			APIVersion: apiVersion,
			Kind:       kind,
			Namespace:  accessor.GetNamespace(),
			Name:       accessor.GetName(),
			UID:        string(accessor.GetUID()),
		},
		ResourceVersion: accessor.GetResourceVersion(),
		Generation:      accessor.GetGeneration(),
		Payload:         payload,
	})
}

func (state *Cache) Run(ctx context.Context) error {
	if !state.started.CompareAndSwap(false, true) {
		return errors.New("Kubernetes cache has already been started")
	}
	state.typedFactory.Start(ctx.Done())
	state.dynamicFactory.Start(ctx.Done())

	syncContext, cancel := context.WithTimeout(ctx, state.config.CacheSyncTimeout)
	defer cancel()
	if !cache.WaitForCacheSync(syncContext.Done(), state.syncFunctions...) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("Kubernetes informer cache did not sync within %s", state.config.CacheSyncTimeout)
	}
	state.synced.Store(true)
	state.syncedAt.Store(time.Now().UTC())
	state.logger.Info("Kubernetes informer cache synchronized", "resources", len(state.syncFunctions))

	<-ctx.Done()
	state.synced.Store(false)
	return nil
}

func (state *Cache) WaitUntilSynced(ctx context.Context) error {
	return wait.PollUntilContextCancel(ctx, 100*time.Millisecond, true, func(context.Context) (bool, error) {
		return state.synced.Load(), nil
	})
}

func (state *Cache) Synced() bool {
	return state.synced.Load()
}

func (state *Cache) SyncedAt() time.Time {
	value := state.syncedAt.Load()
	if value == nil {
		return time.Time{}
	}
	return value.(time.Time)
}

func (state *Cache) ContextName() string {
	return state.clients.Context
}

func (state *Cache) ServerVersion() string {
	return state.clients.Version
}

func (state *Cache) DynamicClient() *Clients {
	return state.clients
}

func (state *Cache) ListPlatform(kind string) []*unstructured.Unstructured {
	informer := state.platform[kind]
	if informer == nil {
		return []*unstructured.Unstructured{}
	}
	items := make([]*unstructured.Unstructured, 0, len(informer.GetStore().List()))
	for _, object := range informer.GetStore().List() {
		if item, ok := object.(*unstructured.Unstructured); ok {
			items = append(items, item.DeepCopy())
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].GetName() < items[j].GetName() })
	return items
}

func (state *Cache) GetPlatform(kind, name string) (*unstructured.Unstructured, bool, error) {
	informer := state.platform[kind]
	if informer == nil {
		return nil, false, fmt.Errorf("unknown platform resource kind %q", kind)
	}
	object, exists, err := informer.GetStore().GetByKey(name)
	if err != nil || !exists {
		return nil, exists, err
	}
	item, ok := object.(*unstructured.Unstructured)
	if !ok {
		return nil, false, fmt.Errorf("cached %s %q has unexpected type %T", kind, name, object)
	}
	return item.DeepCopy(), true, nil
}

func (state *Cache) ListPods() []*corev1.Pod {
	return typedList[*corev1.Pod](state.native[nativePods])
}

func (state *Cache) ListNodes() []*corev1.Node {
	return typedList[*corev1.Node](state.native[nativeNodes])
}

func (state *Cache) ListServices() []*corev1.Service {
	return typedList[*corev1.Service](state.native[nativeServices])
}

func (state *Cache) ListEvents() []*corev1.Event {
	return typedList[*corev1.Event](state.native[nativeEvents])
}

func (state *Cache) ListDeployments() []*appsv1.Deployment {
	return typedList[*appsv1.Deployment](state.native[nativeDeployments])
}

func (state *Cache) ListLeases() []*coordinationv1.Lease {
	return typedList[*coordinationv1.Lease](state.native[nativeLeases])
}

func typedList[T runtime.Object](informer cache.SharedIndexInformer) []T {
	if informer == nil {
		return []T{}
	}
	items := make([]T, 0, len(informer.GetStore().List()))
	for _, object := range informer.GetStore().List() {
		if item, ok := object.(T); ok {
			items = append(items, item.DeepCopyObject().(T))
		}
	}
	return items
}

func randomID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func apiVersionForGVR(gvr schema.GroupVersionResource) string {
	return gvr.GroupVersion().String()
}
