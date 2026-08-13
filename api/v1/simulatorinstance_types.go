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

// TrafficSpec 流量参数，给模拟器压测用
type TrafficSpec struct {
	// 每秒请求数
	// +kubebuilder:validation:Minimum=0
	// +required
	QPS int `json:"qps"`
}

// SimulatorInstanceSpec 实例期望状态
type SimulatorInstanceSpec struct {
	// 所属租户
	// +required
	TenantRef ObjectRef `json:"tenantRef"`

	// 目标模型
	// +required
	ModelRef ObjectRef `json:"modelRef"`

	// 副本数
	// +kubebuilder:validation:Minimum=0
	// +required
	Replicas int `json:"replicas"`

	// 流量配置
	// +required
	Traffic TrafficSpec `json:"traffic"`
}

// InstancePerformanceMetric 性能指标，带单位
type InstancePerformanceMetric struct {
	// 数值
	// +optional
	Value int `json:"value,omitempty"`
	// 单位，比如 "ms"、"s"
	// +optional
	Unit string `json:"unit,omitempty"`
}

// InstancePerformance 单个实例的性能快照
type InstancePerformance struct {
	// 首 token 时间
	// +optional
	TTFT *InstancePerformanceMetric `json:"ttft,omitempty"`
	// 队列长度或排队时间
	// +optional
	Queue *InstancePerformanceMetric `json:"queue,omitempty"`
}

// SimulatorInstanceStatus 实例运行时状态，控制器维护
type SimulatorInstanceStatus struct {
	// 当前性能数据
	// +optional
	Performance *InstancePerformance `json:"performance,omitempty"`

	// Orchestrator 计算的单副本静态能力分数
	// +optional
	EffectiveScore *int `json:"effectiveScore,omitempty"`

	// 模拟器计算的可用副本池实时能力分数，Traffic 控制器根据它分配 QPS
	// +optional
	Score *int `json:"score,omitempty"`

	// 当前 Deployment 中可用的副本数，由 SimulatorInstance 控制器维护
	// +kubebuilder:validation:Minimum=0
	// +optional
	AvailableReplicas int `json:"availableReplicas,omitempty"`

	// 模拟器最近一次成功发布性能快照的时间
	// +optional
	ObservedAt *metav1.Time `json:"observedAt,omitempty"`

	// 当前性能快照的报告者身份，用于诊断 Lease 选主
	// +optional
	ReporterID string `json:"reporterID,omitempty"`

	// 运行阶段：Running, Pending, Failed 等
	// +kubebuilder:validation:Enum=Running;Pending;Failed;Unknown
	// +optional
	Phase string `json:"phase,omitempty"`

	// 标准的 K8s conditions
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Tenant",type=string,JSONPath=`.spec.tenantRef.name`
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.modelRef.name`
// +kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.spec.replicas`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// SimulatorInstance 集群级，一个租户下一个模型只对应一个实例
type SimulatorInstance struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec SimulatorInstanceSpec `json:"spec"`
	// +optional
	Status SimulatorInstanceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// SimulatorInstanceList 列表
type SimulatorInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []SimulatorInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &SimulatorInstance{}, &SimulatorInstanceList{})
		return nil
	})
}
