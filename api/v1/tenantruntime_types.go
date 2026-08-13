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

// TenantRuntimeSpec 指定运行时状态所属的租户。
type TenantRuntimeSpec struct {
	// +required
	TenantRef TenantReference `json:"tenantRef"`
}

// TenantReference 表示租户名称引用。
type TenantReference struct {
	// 租户名称
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`
}

// TenantRuntimeStatus 运行时状态，SimulatorInstanceReconciler 会更新
type TenantRuntimeStatus struct {
	// 当前运行的实例总数
	// +optional
	InstanceCount int `json:"instanceCount,omitempty"`

	// 运行阶段，Running/Pending/Failed
	// +kubebuilder:validation:Enum=Running;Pending;Failed;Unknown
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
// +kubebuilder:printcolumn:name="Instances",type=integer,JSONPath=`.status.instanceCount`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// TenantRuntime 记录单个租户的当前可用副本总数。
type TenantRuntime struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec TenantRuntimeSpec `json:"spec"`
	// +optional
	Status TenantRuntimeStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// TenantRuntimeList 列表
type TenantRuntimeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []TenantRuntime `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &TenantRuntime{}, &TenantRuntimeList{})
		return nil
	})
}
