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

// WorkerNodeSpec 定义由运维侧配置的节点规格。
type WorkerNodeSpec struct {
	// 前端展示名
	// +kubebuilder:validation:MinLength=1
	// +required
	DisplayName string `json:"displayName"`

	// 节点总 GPU 量
	// +kubebuilder:validation:Minimum=1
	// +required
	GPU int `json:"gpu"`

	// 最大并发数
	// +kubebuilder:validation:Minimum=1
	// +required
	MaxConcurrency int `json:"maxConcurrency"`
}

// WorkerNodeStatus 运行时统计，控制器维护
type WorkerNodeStatus struct {
	// 已占用的 GPU
	// +kubebuilder:validation:Minimum=0
	// +optional
	UsedGPU int `json:"usedGPU,omitempty"`

	// 已占用的并发
	// +kubebuilder:validation:Minimum=0
	// +optional
	UsedConcurrency int `json:"usedConcurrency,omitempty"`

	// 物理内存水位（真实 Node 已分配 requests 占 allocatable 的百分比，0-100；
	// 由 WorkerNodeUsage 控制器按同名 Kubernetes Node 计算，找不到 Node 时保持缺省）
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	MemoryUsagePercent int `json:"memoryUsagePercent,omitempty"`

	// 物理 CPU 水位（同上）
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	// +optional
	CPUUsagePercent int `json:"cpuUsagePercent,omitempty"`

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
// +kubebuilder:printcolumn:name="Total GPU",type=integer,JSONPath=`.spec.gpu`
// +kubebuilder:printcolumn:name="Used GPU",type=integer,JSONPath=`.status.usedGPU`
// +kubebuilder:printcolumn:name="MaxConcurrency",type=integer,JSONPath=`.spec.maxConcurrency`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// WorkerNode 表示一个 GPU 计算节点。
type WorkerNode struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// +required
	Spec WorkerNodeSpec `json:"spec"`
	// +optional
	Status WorkerNodeStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// WorkerNodeList 列表
type WorkerNodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []WorkerNode `json:"items"`
}

func init() {
	SchemeBuilder.Register(func(s *runtime.Scheme) error {
		s.AddKnownTypes(SchemeGroupVersion, &WorkerNode{}, &WorkerNodeList{})
		return nil
	})
}
