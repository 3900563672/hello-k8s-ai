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

// TenantPerformanceSpec 指定性能数据所属的租户。
type TenantPerformanceSpec struct {
	// +required
	TenantRef ObjectRef `json:"tenantRef"`
}

// PerformanceMetric 表示带数值和单位的性能指标。
type PerformanceMetric struct {
	Value int    `json:"value,omitempty"`
	Unit  string `json:"unit,omitempty"`
}

// PerformanceStatus 存平均值
type PerformanceStatus struct {
	AvgTTFT  *PerformanceMetric `json:"avgTTFT,omitempty"`
	AvgQueue *PerformanceMetric `json:"avgQueue,omitempty"`
}

// TenantPerformanceStatus 保存供 Orchestrator 使用的聚合性能状态。
type TenantPerformanceStatus struct {
	// 平均性能指标
	// +optional
	Performance PerformanceStatus `json:"performance,omitempty"`

	// 聚合结果所使用的最新实例快照时间
	// +optional
	ObservedAt *metav1.Time `json:"observedAt,omitempty"`

	// 本次聚合中有效的实例快照数
	// +kubebuilder:validation:Minimum=0
	// +optional
	SampleCount int `json:"sampleCount,omitempty"`

	// 数据是否有效，Running / Stale
	// +kubebuilder:validation:Enum=Running;Stale;Unknown
	// +optional
	Phase string `json:"phase,omitempty"`

	// 标准 K8s conditions
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Tenant",type=string,JSONPath=`.spec.tenantRef.name`
// +kubebuilder:printcolumn:name="AvgTTFT",type=string,JSONPath=`.status.performance.avgTTFT.value`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// TenantPerformance 保存单个租户的平均性能指标。
type TenantPerformance struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec TenantPerformanceSpec `json:"spec"`
	// +optional
	Status TenantPerformanceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// TenantPerformanceList 列表
type TenantPerformanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []TenantPerformance `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &TenantPerformance{}, &TenantPerformanceList{})
		return nil
	})
}
