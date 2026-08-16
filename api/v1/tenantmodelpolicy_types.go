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

// TenantModelPolicySpec 租户和模型之间的关系，Allow 或 Deny
type TenantModelPolicySpec struct {
	// 租户引用
	// +required
	// +kubebuilder:validation:XValidation:rule="!has(oldSelf) || self.name == oldSelf.name",message="租户引用不可变，变更请删除重建"
	TenantRef ObjectRef `json:"tenantRef"`

	// 模型引用
	// +required
	// +kubebuilder:validation:XValidation:rule="!has(oldSelf) || self.name == oldSelf.name",message="模型引用不可变，变更请删除重建"
	ModelRef ObjectRef `json:"modelRef"`

	// 策略效果，Allow 可用，Deny 禁止
	// +kubebuilder:validation:Enum=Allow;Deny
	// +required
	Effect string `json:"effect"`
}

// TenantModelPolicyStatus 保存策略的标准 Conditions。
type TenantModelPolicyStatus struct {
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
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.modelRef.name`
// +kubebuilder:printcolumn:name="Effect",type=string,JSONPath=`.spec.effect`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// TenantModelPolicy 集群级，决定租户能不能用某个模型
type TenantModelPolicy struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec TenantModelPolicySpec `json:"spec"`
	// +optional
	Status TenantModelPolicyStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// TenantModelPolicyList 列表
type TenantModelPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []TenantModelPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &TenantModelPolicy{}, &TenantModelPolicyList{})
		return nil
	})
}
