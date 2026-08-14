package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type simulatorDeploymentState struct {
	Deployments       []*appsv1.Deployment
	DesiredReplicas   int
	AvailableReplicas int
	Failed            bool
}

// reconcileDeploymentObjects 把放置计划物化为 Deployment。
// 旧实例没有放置注解时仍使用单 Deployment；持久化计划后，每个节点由一个 Deployment 承载。
func (r *SimulatorInstanceReconciler) reconcileDeploymentObjects(
	ctx context.Context,
	instance *platformv1.SimulatorInstance,
	eligibleNodes []string,
) (*simulatorDeploymentState, error) {
	plan, persisted, err := decodeNodePlacementPlan(instance.Annotations[nodePlacementsAnnotation])
	if err != nil {
		return nil, fmt.Errorf("decode node placements on simulator instance %q: %w", instance.Name, err)
	}
	if persisted && nodePlacementReplicaCount(plan) != instance.Spec.Replicas {
		return nil, fmt.Errorf(
			"simulator instance %q has %d replicas but its node placement plan contains %d",
			instance.Name,
			instance.Spec.Replicas,
			nodePlacementReplicaCount(plan),
		)
	}
	desiredNames, err := deploymentNamesForPlacementPlan(instance.Name, plan, persisted)
	if err != nil {
		return nil, err
	}
	existingDeployments, err := r.listInstanceDeployments(ctx, instance.Name)
	if err != nil {
		return nil, err
	}
	existingByName := make(map[string]*appsv1.Deployment, len(existingDeployments))
	for i := range existingDeployments {
		existingByName[existingDeployments[i].Name] = &existingDeployments[i]
	}
	eligible := make(map[string]struct{}, len(eligibleNodes))
	for _, nodeName := range eligibleNodes {
		eligible[nodeName] = struct{}{}
	}
	if persisted {
		for _, placement := range plan.Placements {
			if _, allowed := eligible[placement.NodeName]; allowed {
				continue
			}
			name := placementDeploymentName(instance.Name, placement.NodeName)
			if placement.NodeName == plan.PrimaryNode {
				name = deploymentName(instance.Name)
			}
			existing := existingByName[name]
			if existing == nil ||
				existing.Annotations[placementNodeAnnotation] != placement.NodeName ||
				existing.Spec.Replicas == nil ||
				int(*existing.Spec.Replicas) < placement.Replicas {
				return nil, fmt.Errorf(
					"planned node %q for simulator instance %q is no longer allowed and cannot be safely drained",
					placement.NodeName,
					instance.Name,
				)
			}
		}
	}

	state := &simulatorDeploymentState{DesiredReplicas: instance.Spec.Replicas}
	ensure := func(name string, replicas int, nodes []string, placementNode string) error {
		deployment, ensureErr := r.ensurePlacementDeployment(
			ctx,
			instance,
			name,
			replicas,
			nodes,
			placementNode,
		)
		if ensureErr != nil {
			return ensureErr
		}
		state.Deployments = append(state.Deployments, deployment)
		state.AvailableReplicas += int(deployment.Status.AvailableReplicas)
		state.Failed = state.Failed || deploymentFailed(deployment)
		return nil
	}

	if !persisted {
		if err := ensure(deploymentName(instance.Name), instance.Spec.Replicas, eligibleNodes, ""); err != nil {
			return nil, err
		}
	} else if len(plan.Placements) == 0 {
		// 缩到零时保留主 Deployment 对象，便于后续扩容，也保持旧的运维入口可用。
		if err := ensure(deploymentName(instance.Name), 0, eligibleNodes, ""); err != nil {
			return nil, err
		}
	} else {
		for _, placement := range sortedNodePlacements(plan.Placements) {
			name := placementDeploymentName(instance.Name, placement.NodeName)
			if placement.NodeName == plan.PrimaryNode {
				name = deploymentName(instance.Name)
			}
			if err := ensure(name, placement.Replicas, []string{placement.NodeName}, placement.NodeName); err != nil {
				return nil, err
			}
		}
	}

	if err := r.deleteObsoletePlacementDeployments(ctx, instance, desiredNames); err != nil {
		return nil, err
	}
	return state, nil
}

func (r *SimulatorInstanceReconciler) listInstanceDeployments(
	ctx context.Context,
	instanceName string,
) ([]appsv1.Deployment, error) {
	var deployments appsv1.DeploymentList
	if err := r.List(
		ctx,
		&deployments,
		client.InNamespace(r.simulatorNamespace()),
		client.MatchingLabels{
			managedByLabelKey: managedByLabelVal,
			instanceLabelKey:  labelValue(instanceName),
		},
	); err != nil {
		return nil, fmt.Errorf("list Deployments for simulator instance %q: %w", instanceName, err)
	}

	result := make([]appsv1.Deployment, 0, len(deployments.Items))
	for i := range deployments.Items {
		deployment := &deployments.Items[i]
		if deployment.Annotations[instanceNameAnnotation] == instanceName {
			result = append(result, *deployment)
		}
	}
	return result, nil
}

func (r *SimulatorInstanceReconciler) deleteObsoletePlacementDeployments(
	ctx context.Context,
	instance *platformv1.SimulatorInstance,
	desiredNames map[string]struct{},
) error {
	deployments, err := r.listInstanceDeployments(ctx, instance.Name)
	if err != nil {
		return err
	}
	var deleteErrors []error
	for i := range deployments {
		deployment := &deployments[i]
		if _, desired := desiredNames[deployment.Name]; desired || !deployment.DeletionTimestamp.IsZero() {
			continue
		}
		if err := r.Delete(ctx, deployment, &client.DeleteOptions{
			PropagationPolicy: new(metav1.DeletePropagationBackground),
		}); err != nil && !apierrors.IsNotFound(err) {
			deleteErrors = append(deleteErrors, fmt.Errorf(
				"delete obsolete placement Deployment %s/%s: %w",
				deployment.Namespace,
				deployment.Name,
				err,
			))
		}
	}
	return errors.Join(deleteErrors...)
}

func (r *SimulatorInstanceReconciler) deleteDeploymentObjects(
	ctx context.Context,
	instance *platformv1.SimulatorInstance,
) (bool, error) {
	deployments, err := r.listInstanceDeployments(ctx, instance.Name)
	if err != nil {
		return false, err
	}
	if len(deployments) == 0 {
		return true, nil
	}

	var deleteErrors []error
	for i := range deployments {
		deployment := &deployments[i]
		if !deployment.DeletionTimestamp.IsZero() {
			continue
		}
		if err := r.Delete(ctx, deployment, &client.DeleteOptions{
			PropagationPolicy: new(metav1.DeletePropagationBackground),
		}); err != nil && !apierrors.IsNotFound(err) {
			deleteErrors = append(deleteErrors, fmt.Errorf(
				"delete Deployment %s/%s: %w",
				deployment.Namespace,
				deployment.Name,
				err,
			))
		}
	}
	return false, errors.Join(deleteErrors...)
}

func placementDeploymentName(instanceName, nodeName string) string {
	const maxNameLength = 253
	base := deploymentName(instanceName)
	sum := sha256.Sum256([]byte(nodeName))
	suffix := "-node-" + hex.EncodeToString(sum[:6])
	if len(base)+len(suffix) <= maxNameLength {
		return base + suffix
	}
	trimmed := strings.TrimRight(base[:maxNameLength-len(suffix)], ".-")
	return trimmed + suffix
}

func deploymentNamesForPlacementPlan(
	instanceName string,
	plan nodePlacementPlan,
	persisted bool,
) (map[string]struct{}, error) {
	names := map[string]struct{}{deploymentName(instanceName): {}}
	if !persisted || len(plan.Placements) == 0 {
		return names, nil
	}
	for _, placement := range plan.Placements {
		name := placementDeploymentName(instanceName, placement.NodeName)
		if placement.NodeName == plan.PrimaryNode {
			name = deploymentName(instanceName)
		}
		if _, duplicate := names[name]; duplicate && name != deploymentName(instanceName) {
			return nil, fmt.Errorf(
				"node placements for simulator instance %q resolve to duplicate Deployment name %q",
				instanceName,
				name,
			)
		}
		names[name] = struct{}{}
	}
	return names, nil
}
