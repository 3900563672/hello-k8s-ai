package controller

import (
	"encoding/json"
	"fmt"
	"slices"
)

const (
	nodePlacementsAnnotation = "platform.study.com/node-placements"
	placementNodeAnnotation  = "platform.study.com/placement-node"
	placementLabelKey        = "platform.study.com/placement"
	placementPlanVersion     = 1
)

// nodePlacementPlan 是 Orchestrator 与 SimulatorInstance Controller 之间的放置契约。
// 它保存在 SimulatorInstance 注解中，不改变公开 CRD；每个节点的副本数之和必须等于 spec.replicas。
type nodePlacementPlan struct {
	Version     int             `json:"version"`
	PrimaryNode string          `json:"primaryNode,omitempty"`
	Placements  []nodePlacement `json:"placements"`
}

type nodePlacement struct {
	NodeName string `json:"nodeName"`
	Replicas int    `json:"replicas"`
}

// decodeNodePlacementPlan 解析并校验持久化的放置计划。
// 第二个返回值用于区分“尚未迁移的旧实例”和“显式的空计划”。
func decodeNodePlacementPlan(raw string) (nodePlacementPlan, bool, error) {
	if raw == "" {
		return nodePlacementPlan{}, false, nil
	}

	var plan nodePlacementPlan
	if err := json.Unmarshal([]byte(raw), &plan); err != nil {
		return nodePlacementPlan{}, true, fmt.Errorf("decode node placement plan: %w", err)
	}
	if plan.Version != placementPlanVersion {
		return nodePlacementPlan{}, true, fmt.Errorf("unsupported node placement plan version %d", plan.Version)
	}

	seen := make(map[string]struct{}, len(plan.Placements))
	for _, placement := range plan.Placements {
		if placement.NodeName == "" {
			return nodePlacementPlan{}, true, fmt.Errorf("node placement contains an empty node name")
		}
		if placement.Replicas <= 0 {
			return nodePlacementPlan{}, true, fmt.Errorf(
				"node placement for %q has invalid replicas %d",
				placement.NodeName,
				placement.Replicas,
			)
		}
		if _, exists := seen[placement.NodeName]; exists {
			return nodePlacementPlan{}, true, fmt.Errorf("node placement contains duplicate node %q", placement.NodeName)
		}
		seen[placement.NodeName] = struct{}{}
	}
	if len(plan.Placements) > 0 {
		if plan.PrimaryNode == "" {
			return nodePlacementPlan{}, true, fmt.Errorf("node placement plan has no primary node")
		}
		if _, exists := seen[plan.PrimaryNode]; !exists {
			return nodePlacementPlan{}, true, fmt.Errorf(
				"primary node %q is not present in the placement list",
				plan.PrimaryNode,
			)
		}
	} else if plan.PrimaryNode != "" {
		return nodePlacementPlan{}, true, fmt.Errorf("empty node placement plan has primary node %q", plan.PrimaryNode)
	}

	plan.Placements = sortedNodePlacements(plan.Placements)
	return plan, true, nil
}

func encodeNodePlacementPlan(plan nodePlacementPlan) (string, error) {
	plan.Version = placementPlanVersion
	plan.Placements = sortedNodePlacements(plan.Placements)
	payload, err := json.Marshal(plan)
	if err != nil {
		return "", fmt.Errorf("encode node placement plan: %w", err)
	}
	return string(payload), nil
}

func newNodePlacementPlan(counts map[string]int) (nodePlacementPlan, error) {
	plan := nodePlacementPlan{Version: placementPlanVersion}
	for nodeName, replicas := range counts {
		if nodeName == "" || replicas <= 0 {
			return nodePlacementPlan{}, fmt.Errorf("invalid observed placement %q=%d", nodeName, replicas)
		}
		plan.Placements = append(plan.Placements, nodePlacement{NodeName: nodeName, Replicas: replicas})
	}
	plan.Placements = sortedNodePlacements(plan.Placements)
	if len(plan.Placements) > 0 {
		plan.PrimaryNode = plan.Placements[0].NodeName
	}
	return plan, nil
}

func sortedNodePlacements(placements []nodePlacement) []nodePlacement {
	result := append([]nodePlacement(nil), placements...)
	slices.SortFunc(result, func(left, right nodePlacement) int {
		if left.NodeName < right.NodeName {
			return -1
		}
		if left.NodeName > right.NodeName {
			return 1
		}
		return 0
	})
	return result
}

func nodePlacementReplicaCount(plan nodePlacementPlan) int {
	total := 0
	for _, placement := range plan.Placements {
		total += placement.Replicas
	}
	return total
}

func addNodePlacement(plan nodePlacementPlan, nodeName string) (nodePlacementPlan, error) {
	if nodeName == "" {
		return nodePlacementPlan{}, fmt.Errorf("scale-up placement has an empty node name")
	}
	plan.Version = placementPlanVersion
	for i := range plan.Placements {
		if plan.Placements[i].NodeName == nodeName {
			plan.Placements[i].Replicas++
			return plan, nil
		}
	}
	plan.Placements = append(plan.Placements, nodePlacement{NodeName: nodeName, Replicas: 1})
	plan.Placements = sortedNodePlacements(plan.Placements)
	if plan.PrimaryNode == "" {
		plan.PrimaryNode = nodeName
	}
	return plan, nil
}

func removeNodePlacement(plan nodePlacementPlan, nodeName string) (nodePlacementPlan, error) {
	if nodeName == "" {
		return nodePlacementPlan{}, fmt.Errorf("scale-down placement has an empty node name")
	}
	for i := range plan.Placements {
		if plan.Placements[i].NodeName != nodeName {
			continue
		}
		plan.Placements[i].Replicas--
		if plan.Placements[i].Replicas == 0 {
			plan.Placements = append(plan.Placements[:i], plan.Placements[i+1:]...)
		}
		if len(plan.Placements) == 0 {
			plan.PrimaryNode = ""
		} else if plan.PrimaryNode == nodeName && !nodePlacementContains(plan, nodeName) {
			plan.Placements = sortedNodePlacements(plan.Placements)
			plan.PrimaryNode = plan.Placements[0].NodeName
		}
		return plan, nil
	}
	return nodePlacementPlan{}, fmt.Errorf("node %q is not present in the placement plan", nodeName)
}

// scaleDownPlacementNode 优先回收非主节点，避免主 Deployment 在普通缩容时更名或重建。
func scaleDownPlacementNode(plan nodePlacementPlan) (string, bool) {
	placements := sortedNodePlacements(plan.Placements)
	for _, placement := range slices.Backward(placements) {
		if placement.NodeName != plan.PrimaryNode {
			return placement.NodeName, true
		}
	}
	if len(placements) == 0 {
		return "", false
	}
	return placements[0].NodeName, true
}

func nodePlacementContains(plan nodePlacementPlan, nodeName string) bool {
	for _, placement := range plan.Placements {
		if placement.NodeName == nodeName {
			return true
		}
	}
	return false
}
