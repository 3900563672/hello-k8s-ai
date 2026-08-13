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

// ObjectRef 通用的对象引用，包一层 Name，给策略类资源用
type ObjectRef struct {
	// 被引用对象的名称
	// +kubebuilder:validation:MinLength=1
	// +required
	Name string `json:"name"`
}
