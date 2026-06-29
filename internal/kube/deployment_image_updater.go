package kube

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"letorollout/internal/rollout"
)

var ErrNotFound = rollout.ErrNotFound
var ErrForbidden = rollout.ErrForbidden
var ErrAlreadyExists = rollout.ErrAlreadyExists

type ImageUpdateRequest = rollout.ImageUpdateRequest
type RolloutResult = rollout.RolloutResult

type DeploymentImageUpdater struct {
	client             kubernetes.Interface
	allowedNamespaces  map[string]struct{}
	requiredLabelKey   string
	requiredLabelValue string
}

type UpdaterOptions struct {
	AllowedNamespaces       []string
	RequiredDeploymentLabel string
}

func NewDeploymentImageUpdater(client kubernetes.Interface, options ...UpdaterOptions) *DeploymentImageUpdater {
	updater := &DeploymentImageUpdater{client: client}
	if len(options) > 0 {
		updater.allowedNamespaces = namespaceSet(options[0].AllowedNamespaces)
		updater.requiredLabelKey, updater.requiredLabelValue = parseRequiredLabel(options[0].RequiredDeploymentLabel)
	}
	return updater
}

func NewInClusterDeploymentImageUpdater(options UpdaterOptions) (*DeploymentImageUpdater, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("load in-cluster config: %w", err)
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	return NewDeploymentImageUpdater(client, options), nil
}

func (u *DeploymentImageUpdater) UpdateImage(ctx context.Context, req ImageUpdateRequest) (RolloutResult, error) {
	if !u.namespaceAllowed(req.Namespace) {
		return RolloutResult{}, fmt.Errorf("%w: namespace %s is not allowed", ErrForbidden, req.Namespace)
	}

	deploymentClient := u.client.AppsV1().Deployments(req.Namespace)
	deployment, err := deploymentClient.Get(ctx, req.Deployment, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return RolloutResult{}, fmt.Errorf("%w: deployment %s/%s", ErrNotFound, req.Namespace, req.Deployment)
		}
		return RolloutResult{}, fmt.Errorf("get deployment %s/%s: %w", req.Namespace, req.Deployment, err)
	}
	if !u.deploymentLabelAllowed(deployment.Labels) {
		return RolloutResult{}, fmt.Errorf("%w: deployment %s/%s does not have required label", ErrForbidden, req.Namespace, req.Deployment)
	}

	containerIndex := -1
	oldImage := ""
	for i, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == req.Container {
			containerIndex = i
			oldImage = container.Image
			break
		}
	}
	if containerIndex == -1 {
		return RolloutResult{}, fmt.Errorf("%w: container %s in deployment %s/%s", ErrNotFound, req.Container, req.Namespace, req.Deployment)
	}

	result := RolloutResult{
		Namespace:  req.Namespace,
		Deployment: req.Deployment,
		Container:  req.Container,
		OldImage:   oldImage,
		NewImage:   req.Image,
		Generation: deployment.Generation,
		DryRun:     req.DryRun,
	}
	if req.DryRun {
		return result, nil
	}

	patch, err := json.Marshal([]map[string]any{
		{
			"op":    "replace",
			"path":  fmt.Sprintf("/spec/template/spec/containers/%d/image", containerIndex),
			"value": req.Image,
		},
	})
	if err != nil {
		return RolloutResult{}, fmt.Errorf("build image patch: %w", err)
	}

	patched, err := deploymentClient.Patch(ctx, req.Deployment, types.JSONPatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return RolloutResult{}, fmt.Errorf("patch deployment %s/%s image: %w", req.Namespace, req.Deployment, err)
	}

	result.Generation = patched.Generation
	if req.Wait {
		complete, err := u.waitForRollout(ctx, req, patched)
		if err != nil {
			return RolloutResult{}, err
		}
		result.RolloutComplete = complete
	}

	return result, nil
}

func namespaceSet(namespaces []string) map[string]struct{} {
	if len(namespaces) == 0 {
		return nil
	}
	out := make(map[string]struct{}, len(namespaces))
	for _, namespace := range namespaces {
		namespace = strings.TrimSpace(namespace)
		if namespace != "" {
			out[namespace] = struct{}{}
		}
	}
	return out
}

func parseRequiredLabel(label string) (string, string) {
	key, value, ok := strings.Cut(strings.TrimSpace(label), "=")
	if !ok {
		return "", ""
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return "", ""
	}
	return key, value
}

func (u *DeploymentImageUpdater) namespaceAllowed(namespace string) bool {
	if len(u.allowedNamespaces) == 0 {
		return true
	}
	_, ok := u.allowedNamespaces[namespace]
	return ok
}

func (u *DeploymentImageUpdater) deploymentLabelAllowed(labels map[string]string) bool {
	if u.requiredLabelKey == "" {
		return true
	}
	return labels[u.requiredLabelKey] == u.requiredLabelValue
}

func (u *DeploymentImageUpdater) waitForRollout(ctx context.Context, req ImageUpdateRequest, current *appsv1.Deployment) (bool, error) {
	if rolloutComplete(current) {
		return true, nil
	}

	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return false, fmt.Errorf("wait rollout %s/%s: %w", req.Namespace, req.Deployment, waitCtx.Err())
		case <-ticker.C:
			deployment, err := u.client.AppsV1().Deployments(req.Namespace).Get(waitCtx, req.Deployment, metav1.GetOptions{})
			if err != nil {
				return false, fmt.Errorf("get rollout status %s/%s: %w", req.Namespace, req.Deployment, err)
			}
			if rolloutComplete(deployment) {
				return true, nil
			}
		}
	}
}

func rolloutComplete(deployment *appsv1.Deployment) bool {
	if deployment == nil {
		return false
	}

	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}

	return deployment.Status.ObservedGeneration >= deployment.Generation &&
		deployment.Status.UpdatedReplicas == desired &&
		deployment.Status.AvailableReplicas == desired
}
