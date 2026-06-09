package kube

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
type DeploymentCreateRequest = rollout.DeploymentCreateRequest
type DeploymentCreateResult = rollout.DeploymentCreateResult
type DeploymentEnvVar = rollout.DeploymentEnvVar
type DeploymentEnvSecret = rollout.DeploymentEnvSecret

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

func (u *DeploymentImageUpdater) CreateDeployment(ctx context.Context, req DeploymentCreateRequest) (DeploymentCreateResult, error) {
	if !u.namespaceAllowed(req.Namespace) {
		return DeploymentCreateResult{}, fmt.Errorf("%w: namespace %s is not allowed", ErrForbidden, req.Namespace)
	}

	const containerName = "app"
	const replicas = int32(1)

	selectorLabels := map[string]string{
		"letorollout.io/deployment-id": deploymentSelectorID(req.Name),
	}
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "letorollout",
	}
	for key, value := range selectorLabels {
		labels[key] = value
	}
	if u.requiredLabelKey != "" {
		labels[u.requiredLabelKey] = u.requiredLabelValue
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: req.Namespace,
			Name:      req.Name,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32ValuePtr(replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  containerName,
							Image: req.Image,
							Env:   kubeEnvVars(req.Env),
						},
					},
				},
			},
		},
	}

	created, err := u.client.AppsV1().Deployments(req.Namespace).Create(ctx, deployment, metav1.CreateOptions{})
	if err != nil {
		if apierrors.IsAlreadyExists(err) {
			return DeploymentCreateResult{}, fmt.Errorf("%w: deployment %s/%s", rollout.ErrAlreadyExists, req.Namespace, req.Name)
		}
		return DeploymentCreateResult{}, fmt.Errorf("create deployment %s/%s: %w", req.Namespace, req.Name, err)
	}

	createdReplicas := replicas
	if created.Spec.Replicas != nil {
		createdReplicas = *created.Spec.Replicas
	}

	return DeploymentCreateResult{
		Namespace:  created.Namespace,
		Name:       created.Name,
		Container:  containerName,
		Image:      req.Image,
		Replicas:   createdReplicas,
		Generation: created.Generation,
		Labels:     created.Labels,
		Env:        req.Env,
	}, nil
}

func kubeEnvVars(env []DeploymentEnvVar) []corev1.EnvVar {
	if len(env) == 0 {
		return nil
	}

	out := make([]corev1.EnvVar, 0, len(env))
	for _, item := range env {
		kubeEnv := corev1.EnvVar{Name: item.Name}
		if item.Value != nil {
			kubeEnv.Value = *item.Value
		}
		if item.Secret != nil {
			kubeEnv.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: item.Secret.Name},
					Key:                  item.Secret.Key,
				},
			}
		}
		out = append(out, kubeEnv)
	}
	return out
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

func deploymentSelectorID(name string) string {
	sum := sha256.Sum256([]byte(name))
	return fmt.Sprintf("d-%x", sum[:8])
}

func int32ValuePtr(v int32) *int32 {
	return &v
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
