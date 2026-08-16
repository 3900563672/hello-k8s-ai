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

// TenantNodePolicySpec 租户和节点的关系，Allow 或 Deny
type TenantNodePolicySpec struct {
	// 租户引用
	// +required
	// +kubebuilder:validation:XValidation:rule="!has(oldSelf) || self.name == oldSelf.name",message="租户引用不可变，变更请删除重建"
	TenantRef ObjectRef `json:"tenantRef"`

	// 节点引用
	// +required
	// +kubebuilder:validation:XValidation:rule="!has(oldSelf) || self.name == oldSelf.name",message="节点引用不可变，变更请删除重建"
	NodeRef ObjectRef `json:"nodeRef"`

	// Allow 或 Deny
	// +kubebuilder:validation:Enum=Allow;Deny
	// +required
	Effect string `json:"effect"`
}

// TenantNodePolicyStatus 保存策略的标准 Conditions。
type TenantNodePolicyStatus struct {
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
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.spec.nodeRef.name`
// +kubebuilder:printcolumn:name="Effect",type=string,JSONPath=`.spec.effect`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// TenantNodePolicy 集群级，决定租户能不能用某个节点
type TenantNodePolicy struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec TenantNodePolicySpec `json:"spec"`
	// +optional
	Status TenantNodePolicyStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// TenantNodePolicyList 列表
type TenantNodePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []TenantNodePolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &TenantNodePolicy{}, &TenantNodePolicyList{})
		return nil
	})
}
