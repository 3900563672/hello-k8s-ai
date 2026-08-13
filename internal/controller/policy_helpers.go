package controller

import (
	"slices"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
)

// tenantModelAllowed 判断某个租户是否被允许使用某个模型。
// 策略语义：必须有显式的 Allow，Deny 优先；已删除的策略不参与判断。
func tenantModelAllowed(policies []platformv1.TenantModelPolicy, tenantName, modelName string) bool {
	allowed := false
	for i := range policies {
		policy := &policies[i]
		// 删除中的、不是这个租户或不是这个模型的都跳过
		if !policy.DeletionTimestamp.IsZero() ||
			policy.Spec.TenantRef.Name != tenantName ||
			policy.Spec.ModelRef.Name != modelName {
			continue
		}
		switch policy.Spec.Effect {
		case policyEffectDeny:
			// Deny 优先，直接否决
			return false
		case policyEffectAllow:
			allowed = true
		}
	}
	return allowed
}

// eligibleNodeNames 计算一个模型可以调度到哪些节点。
// 租户-节点策略构成基础 allowlist；模型-节点策略在此基础上进一步过滤：
//   - 模型 Deny 优先（节点被排除）；
//   - 只要存在至少一条模型 Allow，就用模型 Allow 作为额外的 allowlist；
//   - 如果没有模型 Allow，则所有租户允许的节点都算可用（向后兼容）。
func eligibleNodeNames(
	tenantName string,
	modelName string,
	tenantPolicies []platformv1.TenantNodePolicy,
	modelPolicies []platformv1.ModelNodePolicy,
) []string {
	// 收集租户-节点策略中每个节点的 Allow/Deny
	tenantEffects := make(map[string]map[string]bool)
	for i := range tenantPolicies {
		policy := &tenantPolicies[i]
		if !policy.DeletionTimestamp.IsZero() || policy.Spec.TenantRef.Name != tenantName {
			continue
		}
		effects := tenantEffects[policy.Spec.NodeRef.Name]
		if effects == nil {
			effects = make(map[string]bool)
			tenantEffects[policy.Spec.NodeRef.Name] = effects
		}
		effects[policy.Spec.Effect] = true
	}

	// 收集模型-节点策略，并记录是否存在模型 Allow
	modelEffects := make(map[string]map[string]bool)
	hasModelAllow := false
	for i := range modelPolicies {
		policy := &modelPolicies[i]
		if !policy.DeletionTimestamp.IsZero() || policy.Spec.ModelRef.Name != modelName {
			continue
		}
		effects := modelEffects[policy.Spec.NodeRef.Name]
		if effects == nil {
			effects = make(map[string]bool)
			modelEffects[policy.Spec.NodeRef.Name] = effects
		}
		effects[policy.Spec.Effect] = true
		if policy.Spec.Effect == policyEffectAllow {
			hasModelAllow = true
		}
	}

	// 筛选：租户必须 Allow 且没有 Deny，模型有 Deny 就踢掉；
	// 如果模型有 Allow，则节点必须在模型 Allow 里。
	nodes := make([]string, 0, len(tenantEffects))
	for nodeName, tenantEffect := range tenantEffects {
		if tenantEffect[policyEffectDeny] || !tenantEffect[policyEffectAllow] {
			continue
		}
		modelEffect := modelEffects[nodeName]
		if modelEffect[policyEffectDeny] || (hasModelAllow && !modelEffect[policyEffectAllow]) {
			continue
		}
		nodes = append(nodes, nodeName)
	}
	slices.Sort(nodes)
	return nodes
}
