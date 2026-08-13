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

// ModelNodePolicySpec 哪个模型在哪个节点上，Allow 或 Deny
type ModelNodePolicySpec struct {
	// 模型引用
	// +required
	ModelRef ObjectRef `json:"modelRef"`

	// 节点引用
	// +required
	NodeRef ObjectRef `json:"nodeRef"`

	// Allow 或 Deny
	// +kubebuilder:validation:Enum=Allow;Deny
	// +required
	Effect string `json:"effect"`
}

// ModelNodePolicyStatus 保存策略的标准 Conditions。
type ModelNodePolicyStatus struct {
	// 标准的 K8s conditions
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.modelRef.name`
// +kubebuilder:printcolumn:name="Node",type=string,JSONPath=`.spec.nodeRef.name`
// +kubebuilder:printcolumn:name="Effect",type=string,JSONPath=`.spec.effect`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// ModelNodePolicy 集群级，控制模型和节点的亲和/反亲和
type ModelNodePolicy struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec ModelNodePolicySpec `json:"spec"`
	// +optional
	Status ModelNodePolicyStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ModelNodePolicyList 列表
type ModelNodePolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []ModelNodePolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &ModelNodePolicy{}, &ModelNodePolicyList{})
		return nil
	})
}
