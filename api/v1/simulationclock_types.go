/*
Copyright 2026 3900563672.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// DefaultSimulationClockName 是集群唯一的 Simulator 倍速配置名称。
	DefaultSimulationClockName = "default"
	// DefaultSimulationRate 表示模拟时间与真实 tick 等速推进。
	DefaultSimulationRate = 1
	// MaxSimulationRate 限制单个 tick 的工作量，避免误配置耗尽 Simulator 资源。
	MaxSimulationRate = 20
)

// SimulationClockSpec 定义 Simulator 离散事件引擎的期望时间倍速。
type SimulationClockSpec struct {
	// Rate 表示每个真实 tick 需要推进的模拟时间倍数。
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	// +required
	Rate int `json:"rate"`
}

// SimulationClockStatus 记录倍速配置向 SimulatorInstance 的收敛情况。
type SimulationClockStatus struct {
	// ObservedGeneration 是 Controller 已处理的配置版本。
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// AppliedRate 是所有现存 SimulatorInstance 已同步的倍速。
	// +optional
	AppliedRate int `json:"appliedRate,omitempty"`

	// SynchronizedInstances 是已同步到期望倍速的实例数。
	// +kubebuilder:validation:Minimum=0
	// +optional
	SynchronizedInstances int `json:"synchronizedInstances,omitempty"`

	// TotalInstances 是本轮收敛观察到的实例总数。
	// +kubebuilder:validation:Minimum=0
	// +optional
	TotalInstances int `json:"totalInstances,omitempty"`

	// Conditions 描述倍速配置是否已完成收敛。
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'default'",message="SimulationClock 名称必须为 default"
// +kubebuilder:printcolumn:name="Rate",type=integer,JSONPath=`.spec.rate`
// +kubebuilder:printcolumn:name="Applied",type=integer,JSONPath=`.status.appliedRate`
// +kubebuilder:printcolumn:name="Synced",type=integer,JSONPath=`.status.synchronizedInstances`
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SimulationClock 是集群唯一的 Simulator 倍速控制资源。
type SimulationClock struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec SimulationClockSpec `json:"spec"`
	// +optional
	Status SimulationClockStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SimulationClockList 表示 SimulationClock 资源列表。
type SimulationClockList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []SimulationClock `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &SimulationClock{}, &SimulationClockList{})
		return nil
	})
}
