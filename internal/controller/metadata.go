package controller

import (
	"crypto/sha256"
	"encoding/hex"

	"k8s.io/apimachinery/pkg/api/validate/content"
)

const (
	managedByLabelKey = "platform.study.com/managed-by"
	managedByLabelVal = "simulator-instance-controller"
	instanceLabelKey  = "platform.study.com/instance"
	tenantLabelKey    = "platform.study.com/tenant"
	modelLabelKey     = "platform.study.com/model"

	instanceNameAnnotation = "platform.study.com/instance-name"
	tenantNameAnnotation   = "platform.study.com/tenant-name"
	modelNameAnnotation    = "platform.study.com/model-name"
)

// labelValue 把名称转成合法的标签值。如果本身合法就直接用，不合法就换成哈希。
// 原始值始终保留在注解里，标签只用于索引和选择。
func labelValue(value string) string {
	// content.IsLabelValue 返回不合规的原因列表，空列表说明合法
	if value != "" && len(content.IsLabelValue(value)) == 0 {
		return value
	}
	// 不合法或为空时用 SHA256 前 8 字节，加上前缀避免纯数字
	sum := sha256.Sum256([]byte(value))
	return "sha256-" + hex.EncodeToString(sum[:8])
}

// setIdentityMetadata 给资源打上标识用的标签和注解。
// 标签值可能被截断或哈希化，注解里存完整的原始名称。
func setIdentityMetadata(
	labels map[string]string,
	annotations map[string]string,
	instanceName string,
	tenantName string,
	modelName string,
) {
	// 统一打上 managed-by，方便筛选哪些资源是这个控制器管的
	labels[managedByLabelKey] = managedByLabelVal

	// 实例、租户、模型分别存标签和注解，标签用于 List 过滤，注解用于读取原始值
	labels[instanceLabelKey] = labelValue(instanceName)
	labels[tenantLabelKey] = labelValue(tenantName)
	labels[modelLabelKey] = labelValue(modelName)

	annotations[instanceNameAnnotation] = instanceName
	annotations[tenantNameAnnotation] = tenantName
	annotations[modelNameAnnotation] = modelName
}

// ensureStringMap 在 map 为空时初始化，避免后续赋值触发 panic。
func ensureStringMap(values *map[string]string) map[string]string {
	if *values == nil {
		*values = make(map[string]string)
	}
	return *values
}
