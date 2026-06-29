package kube

import (
	"context"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestUpdateImagePatchesDeploymentContainerImage(t *testing.T) {
	deployment := deploymentFixture("default", "nginx", []corev1.Container{
		{Name: "sidecar", Image: "busybox:1.36"},
		{Name: "nginx", Image: "nginx:1.26.0"},
	})
	client := fake.NewSimpleClientset(deployment)
	updater := NewDeploymentImageUpdater(client)

	result, err := updater.UpdateImage(context.Background(), ImageUpdateRequest{
		Namespace:  "default",
		Deployment: "nginx",
		Container:  "nginx",
		Image:      "nginx:1.27.0",
	})
	if err != nil {
		t.Fatalf("UpdateImage returned error: %v", err)
	}

	updated, err := client.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get updated deployment: %v", err)
	}
	if updated.Spec.Template.Spec.Containers[1].Image != "nginx:1.27.0" {
		t.Fatalf("patched image = %q, want nginx:1.27.0", updated.Spec.Template.Spec.Containers[1].Image)
	}
	if updated.Spec.Template.Spec.Containers[0].Image != "busybox:1.36" {
		t.Fatalf("sidecar image = %q, want unchanged busybox:1.36", updated.Spec.Template.Spec.Containers[0].Image)
	}
	if result.OldImage != "nginx:1.26.0" || result.NewImage != "nginx:1.27.0" {
		t.Fatalf("result = %+v, want old and new images", result)
	}
}

func TestUpdateImageReturnsNotFoundForMissingContainer(t *testing.T) {
	deployment := deploymentFixture("default", "nginx", []corev1.Container{
		{Name: "nginx", Image: "nginx:1.26.0"},
	})
	updater := NewDeploymentImageUpdater(fake.NewSimpleClientset(deployment))

	_, err := updater.UpdateImage(context.Background(), ImageUpdateRequest{
		Namespace:  "default",
		Deployment: "nginx",
		Container:  "api",
		Image:      "api:1.0.0",
	})

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateImageDryRunDoesNotPatchDeployment(t *testing.T) {
	deployment := deploymentFixture("default", "nginx", []corev1.Container{
		{Name: "nginx", Image: "nginx:1.26.0"},
	})
	client := fake.NewSimpleClientset(deployment)
	updater := NewDeploymentImageUpdater(client)

	result, err := updater.UpdateImage(context.Background(), ImageUpdateRequest{
		Namespace:  "default",
		Deployment: "nginx",
		Container:  "nginx",
		Image:      "nginx:1.27.0",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("UpdateImage returned error: %v", err)
	}

	updated, err := client.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if updated.Spec.Template.Spec.Containers[0].Image != "nginx:1.26.0" {
		t.Fatalf("image = %q, want unchanged nginx:1.26.0", updated.Spec.Template.Spec.Containers[0].Image)
	}
	if !result.DryRun || result.OldImage != "nginx:1.26.0" || result.NewImage != "nginx:1.27.0" {
		t.Fatalf("result = %+v, want dry run preview", result)
	}
}

func TestUpdateImageRejectsDisallowedNamespace(t *testing.T) {
	deployment := deploymentFixture("prod", "nginx", []corev1.Container{
		{Name: "nginx", Image: "nginx:1.26.0"},
	})
	updater := NewDeploymentImageUpdater(fake.NewSimpleClientset(deployment), UpdaterOptions{
		AllowedNamespaces: []string{"dev"},
	})

	_, err := updater.UpdateImage(context.Background(), ImageUpdateRequest{
		Namespace:  "prod",
		Deployment: "nginx",
		Container:  "nginx",
		Image:      "nginx:1.27.0",
	})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestUpdateImageRequiresDeploymentLabel(t *testing.T) {
	deployment := deploymentFixture("default", "nginx", []corev1.Container{
		{Name: "nginx", Image: "nginx:1.26.0"},
	})
	updater := NewDeploymentImageUpdater(fake.NewSimpleClientset(deployment), UpdaterOptions{
		RequiredDeploymentLabel: "letorollout/enabled=true",
	})

	_, err := updater.UpdateImage(context.Background(), ImageUpdateRequest{
		Namespace:  "default",
		Deployment: "nginx",
		Container:  "nginx",
		Image:      "nginx:1.27.0",
	})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestUpdateImageWaitsForRollout(t *testing.T) {
	deployment := deploymentFixture("default", "nginx", []corev1.Container{
		{Name: "nginx", Image: "nginx:1.26.0"},
	})
	deployment.(*appsv1.Deployment).Spec.Replicas = int32Ptr(2)
	client := fake.NewSimpleClientset(deployment)
	client.PrependReactor("patch", "deployments", func(action ktesting.Action) (bool, runtime.Object, error) {
		patched := deployment.(*appsv1.Deployment).DeepCopy()
		patched.Generation = 4
		patched.Status.ObservedGeneration = 4
		patched.Status.Replicas = 2
		patched.Status.UpdatedReplicas = 2
		patched.Status.AvailableReplicas = 2
		patched.Spec.Template.Spec.Containers[0].Image = "nginx:1.27.0"
		return true, patched, nil
	})
	updater := NewDeploymentImageUpdater(client)

	result, err := updater.UpdateImage(context.Background(), ImageUpdateRequest{
		Namespace:      "default",
		Deployment:     "nginx",
		Container:      "nginx",
		Image:          "nginx:1.27.0",
		Wait:           true,
		TimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatalf("UpdateImage returned error: %v", err)
	}

	if !result.RolloutComplete {
		t.Fatalf("result = %+v, want rollout complete", result)
	}
}

func deploymentFixture(namespace, name string, containers []corev1.Container) runtime.Object {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  namespace,
			Name:       name,
			Generation: 3,
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: containers,
				},
			},
		},
	}
}

func int32Ptr(v int32) *int32 {
	return &v
}

func stringPtr(v string) *string {
	return &v
}
