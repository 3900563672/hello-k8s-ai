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

// PerformanceSpec 定义模型打分和资源计算所需的静态性能参数。
type PerformanceSpec struct {
	// prefill 基础延迟，单位 ms，默认 50
	// +kubebuilder:default=50
	// +optional
	PrefillBaseMs int `json:"prefillBaseMs,omitempty"`

	// 每个 prompt token 额外增加的 prefill 时间，单位微秒，默认 500µs
	// +kubebuilder:default=500
	// +optional
	PrefillPerTokenUs int `json:"prefillPerTokenUs,omitempty"`

	// 每个生成 token 耗时，单位 ms，默认 20
	// +kubebuilder:default=20
	// +optional
	DecodePerTokenMs int `json:"decodePerTokenMs,omitempty"`
}

// ModelSpec 定义模型的期望状态，由运维侧配置。
type ModelSpec struct {
	// 前端展示名
	// +kubebuilder:validation:MinLength=1
	// +required
	DisplayName string `json:"displayName"`

	// 实例占的 GPU 量，比如 800 表示 0.8 卡
	// +kubebuilder:validation:Minimum=1
	// +required
	GPUUnits int `json:"gpuUnits"`

	// 最大并发
	// +kubebuilder:validation:Minimum=1
	// +required
	MaxConcurrency int `json:"maxConcurrency"`

	// 冷启动时间，单位 ms
	// +kubebuilder:validation:Minimum=0
	// +required
	ColdStartMs int `json:"coldStartMs"`

	// 性能参数，未填写时使用内置默认值
	// +optional
	Performance PerformanceSpec `json:"performance,omitempty"`
}

// ModelStatus 运行时状态，由控制器维护
type ModelStatus struct {
	// 单个已预热副本在理想条件下的能力基准分数，由后端或运维维护
	// +optional
	AbsoluteScore *int `json:"absoluteScore,omitempty"`

	// K8s 标准 conditions，记录一些状态
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="DisplayName",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="GPUUnits",type=integer,JSONPath=`.spec.gpuUnits`
// +kubebuilder:printcolumn:name="MaxConcurrency",type=integer,JSONPath=`.spec.maxConcurrency`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Model 是描述模型基本属性和性能的集群级资源。
type Model struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec ModelSpec `json:"spec"`
	// +optional
	Status ModelStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// ModelList 表示 Model 资源列表。
type ModelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Model `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &Model{}, &ModelList{})
		return nil
	})
}
