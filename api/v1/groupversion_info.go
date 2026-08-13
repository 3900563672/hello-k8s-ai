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

// Package v1 定义 platform v1 API 组的 Schema。
// +kubebuilder:object:generate=true
// +groupName=platform.study.com
package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// SchemeGroupVersion 是注册这些对象时使用的组版本。
	// applyconfiguration 生成器（例如 controller-gen）也会使用该名称。
	SchemeGroupVersion = schema.GroupVersion{Group: "platform.study.com", Version: "v1"}

	// GroupVersion 是为兼容旧调用保留的 SchemeGroupVersion 别名。
	GroupVersion = SchemeGroupVersion

	// SchemeBuilder 将 Go 类型注册到 GroupVersionKind Scheme。
	SchemeBuilder = runtime.NewSchemeBuilder(func(scheme *runtime.Scheme) error {
		metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
		return nil
	})

	// AddToScheme 将该组版本的类型加入指定 Scheme。
	AddToScheme = SchemeBuilder.AddToScheme
)
