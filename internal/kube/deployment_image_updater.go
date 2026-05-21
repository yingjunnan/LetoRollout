package kube

import (
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"letorollout/internal/rollout"
)

var ErrNotFound = rollout.ErrNotFound

type ImageUpdateRequest = rollout.ImageUpdateRequest
type RolloutResult = rollout.RolloutResult

type DeploymentImageUpdater struct {
	client kubernetes.Interface
}

func NewDeploymentImageUpdater(client kubernetes.Interface) *DeploymentImageUpdater {
	return &DeploymentImageUpdater{client: client}
}

func NewInClusterDeploymentImageUpdater() (*DeploymentImageUpdater, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("load in-cluster config: %w", err)
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create kubernetes client: %w", err)
	}

	return NewDeploymentImageUpdater(client), nil
}

func (u *DeploymentImageUpdater) UpdateImage(ctx context.Context, req ImageUpdateRequest) (RolloutResult, error) {
	deploymentClient := u.client.AppsV1().Deployments(req.Namespace)
	deployment, err := deploymentClient.Get(ctx, req.Deployment, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return RolloutResult{}, fmt.Errorf("%w: deployment %s/%s", ErrNotFound, req.Namespace, req.Deployment)
		}
		return RolloutResult{}, fmt.Errorf("get deployment %s/%s: %w", req.Namespace, req.Deployment, err)
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

	return RolloutResult{
		Namespace:  req.Namespace,
		Deployment: req.Deployment,
		Container:  req.Container,
		OldImage:   oldImage,
		NewImage:   req.Image,
		Generation: patched.Generation,
	}, nil
}
