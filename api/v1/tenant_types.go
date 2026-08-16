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

// TenantSpec 租户的配置，优先级、QPS、扩缩容阈值都在这里
// +kubebuilder:validation:XValidation:rule="self.ttftScaleDownThresholdMs < self.ttftThresholdMs",message="ttftScaleDownThresholdMs must be less than ttftThresholdMs"
// +kubebuilder:validation:XValidation:rule="self.queueScaleDownThreshold < self.queueThreshold",message="queueScaleDownThreshold must be less than queueThreshold"
type TenantSpec struct {
	// 前端显示的名字
	// +kubebuilder:validation:MinLength=1
	// +required
	DisplayName string `json:"displayName"`

	// 优先级，P1 最高，P5 最低
	// +kubebuilder:validation:Enum=P1;P2;P3;P4;P5
	// +required
	Priority string `json:"priority"`

	// 模拟的总 QPS
	// +kubebuilder:validation:Minimum=0
	// +required
	QPS int `json:"qps"`

	// 扩容阈值

	// TTFT 上限，单位为毫秒；超过时触发扩容，必填
	// +kubebuilder:validation:Minimum=1
	// +required
	TTFTThresholdMs int `json:"ttftThresholdMs"`

	// 队列长度上限；超过时触发扩容，必填
	// +kubebuilder:validation:Minimum=1
	// +required
	QueueThreshold int `json:"queueThreshold"`

	// 缩容阈值

	// TTFT 下限，单位为毫秒；低于该值时允许缩容，必填
	// +kubebuilder:validation:Minimum=1
	// +required
	TTFTScaleDownThresholdMs int `json:"ttftScaleDownThresholdMs"`

	// 队列长度下限；低于该值时允许缩容，必填
	// +kubebuilder:validation:Minimum=1
	// +required
	QueueScaleDownThreshold int `json:"queueScaleDownThreshold"`
}

// TenantStatus 保存租户的标准 Conditions。
type TenantStatus struct {
	// 标准的 K8s conditions
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="DisplayName",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Priority",type=string,JSONPath=`.spec.priority`
// +kubebuilder:printcolumn:name="QPS",type=integer,JSONPath=`.spec.qps`
// +kubebuilder:printcolumn:name="TTFT(Up)",type=integer,JSONPath=`.spec.ttftThresholdMs`
// +kubebuilder:printcolumn:name="TTFT(Down)",type=integer,JSONPath=`.spec.ttftScaleDownThresholdMs`
// +kubebuilder:printcolumn:name="Queue(Up)",type=integer,JSONPath=`.spec.queueThreshold`
// +kubebuilder:printcolumn:name="Queue(Down)",type=integer,JSONPath=`.spec.queueScaleDownThreshold`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Tenant 表示一个集群级租户资源。
type Tenant struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec TenantSpec `json:"spec"`
	// +optional
	Status TenantStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// TenantList 列表
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Tenant `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Tenant{}, &TenantList{})
		return nil
	})
}
