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

// OrchestratorSpec 每个租户的扩缩容配置
// +kubebuilder:validation:XValidation:rule="self.minReplicas <= self.maxReplicas",message="minReplicas must not exceed maxReplicas"
type OrchestratorSpec struct {
	// 属于哪个租户
	// +required
	TenantRef ObjectRef `json:"tenantRef"`

	// 扩容后多长时间内不再扩容，单位秒，默认 60
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=60
	// +optional
	ScaleUpCooldownSeconds int `json:"scaleUpCooldownSeconds,omitempty"`

	// 缩容冷却时间，单位秒，默认 120
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=120
	// +optional
	ScaleDownCooldownSeconds int `json:"scaleDownCooldownSeconds,omitempty"`

	// 允许缩到 0 吗？只在没流量时有用
	// +optional
	AllowScaleToZero bool `json:"allowScaleToZero,omitempty"`

	// 租户所有实例的最小副本数（无流量且允许缩到 0 时可被覆盖）
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=1
	// +optional
	MinReplicas int `json:"minReplicas,omitempty"`

	// 租户所有实例的最大副本数
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=100
	// +optional
	MaxReplicas int `json:"maxReplicas,omitempty"`
}

// OrchestratorStatus 记录最近一次扩缩操作和标准 Conditions。
type OrchestratorStatus struct {
	// 最近一次扩缩详情；尚未发生扩缩时为空
	// +optional
	LastScaling *ScalingRecord `json:"lastScaling,omitempty"`

	// 最近一次扩容时间；与缩容时间独立，避免两个方向共用冷却窗口
	// +optional
	LastScaleUpTime *metav1.Time `json:"lastScaleUpTime,omitempty"`

	// 最近一次缩容时间；与扩容时间独立，避免两个方向共用冷却窗口
	// +optional
	LastScaleDownTime *metav1.Time `json:"lastScaleDownTime,omitempty"`

	// 标准 K8s conditions
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// ScalingRecord 表示一条扩缩记录。
type ScalingRecord struct {
	// 时间
	// +required
	Time metav1.Time `json:"time"`

	// 操作：ScaleUp 或 ScaleDown
	// +kubebuilder:validation:Enum=ScaleUp;ScaleDown
	// +required
	Action string `json:"action"`

	// 被操作的实例
	// +required
	InstanceName string `json:"instanceName"`

	// 操作前副本数
	// +required
	OldReplicas int `json:"oldReplicas"`

	// 操作后副本数
	// +required
	NewReplicas int `json:"newReplicas"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Tenant",type=string,JSONPath=`.spec.tenantRef.name`
// +kubebuilder:printcolumn:name="ScaleUpCooldown",type=integer,JSONPath=`.spec.scaleUpCooldownSeconds`
// +kubebuilder:printcolumn:name="ScaleDownCooldown",type=integer,JSONPath=`.spec.scaleDownCooldownSeconds`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Orchestrator 是租户级的编排配置，控制决策器的冷却和副本范围策略。
type Orchestrator struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec OrchestratorSpec `json:"spec"`
	// +optional
	Status OrchestratorStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// OrchestratorList 表示 Orchestrator 资源列表。
type OrchestratorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Orchestrator `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Orchestrator{}, &OrchestratorList{})
		return nil
	})
}
