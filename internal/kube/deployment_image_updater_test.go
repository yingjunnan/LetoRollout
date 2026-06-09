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

func TestCreateDeploymentCreatesMinimalDeployment(t *testing.T) {
	client := fake.NewSimpleClientset()
	updater := NewDeploymentImageUpdater(client)

	result, err := updater.CreateDeployment(context.Background(), DeploymentCreateRequest{
		Namespace: "default",
		Name:      "nginx",
		Image:     "nginx:1.27.0",
	})
	if err != nil {
		t.Fatalf("CreateDeployment returned error: %v", err)
	}

	created, err := client.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get created deployment: %v", err)
	}
	if created.Spec.Replicas == nil || *created.Spec.Replicas != 1 {
		t.Fatalf("replicas = %v, want 1", created.Spec.Replicas)
	}
	if got := created.Spec.Template.Spec.Containers[0]; got.Name != "app" || got.Image != "nginx:1.27.0" {
		t.Fatalf("container = %+v, want app with nginx:1.27.0", got)
	}
	for key, value := range created.Spec.Selector.MatchLabels {
		if created.Spec.Template.Labels[key] != value {
			t.Fatalf("template label %s = %q, want selector value %q", key, created.Spec.Template.Labels[key], value)
		}
	}
	if result.Namespace != "default" || result.Name != "nginx" || result.Container != "app" || result.Image != "nginx:1.27.0" || result.Replicas != 1 {
		t.Fatalf("result = %+v, want created deployment details", result)
	}
}

func TestCreateDeploymentAddsLiteralAndSecretEnv(t *testing.T) {
	client := fake.NewSimpleClientset()
	updater := NewDeploymentImageUpdater(client)

	result, err := updater.CreateDeployment(context.Background(), DeploymentCreateRequest{
		Namespace: "default",
		Name:      "nginx",
		Image:     "nginx:1.27.0",
		Env: []DeploymentEnvVar{
			{Name: "APP_ENV", Value: stringPtr("prod")},
			{Name: "DATABASE_URL", Secret: &DeploymentEnvSecret{Name: "nginx-secret", Key: "database-url"}},
		},
	})
	if err != nil {
		t.Fatalf("CreateDeployment returned error: %v", err)
	}

	created, err := client.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get created deployment: %v", err)
	}

	env := created.Spec.Template.Spec.Containers[0].Env
	if len(env) != 2 {
		t.Fatalf("env length = %d, want 2: %+v", len(env), env)
	}
	if env[0].Name != "APP_ENV" || env[0].Value != "prod" || env[0].ValueFrom != nil {
		t.Fatalf("env[0] = %+v, want literal APP_ENV=prod", env[0])
	}
	if env[1].Name != "DATABASE_URL" || env[1].ValueFrom == nil || env[1].ValueFrom.SecretKeyRef == nil {
		t.Fatalf("env[1] = %+v, want SecretKeyRef", env[1])
	}
	if env[1].ValueFrom.SecretKeyRef.Name != "nginx-secret" || env[1].ValueFrom.SecretKeyRef.Key != "database-url" {
		t.Fatalf("secret ref = %+v, want nginx-secret/database-url", env[1].ValueFrom.SecretKeyRef)
	}
	if len(result.Env) != 2 || result.Env[0].Name != "APP_ENV" || result.Env[1].Secret == nil || result.Env[1].Secret.Name != "nginx-secret" {
		t.Fatalf("result env = %+v, want accepted env", result.Env)
	}
}

func TestCreateDeploymentRejectsDisallowedNamespace(t *testing.T) {
	updater := NewDeploymentImageUpdater(fake.NewSimpleClientset(), UpdaterOptions{
		AllowedNamespaces: []string{"dev"},
	})

	_, err := updater.CreateDeployment(context.Background(), DeploymentCreateRequest{
		Namespace: "prod",
		Name:      "nginx",
		Image:     "nginx:1.27.0",
	})

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestCreateDeploymentAddsRequiredDeploymentLabel(t *testing.T) {
	client := fake.NewSimpleClientset()
	updater := NewDeploymentImageUpdater(client, UpdaterOptions{
		RequiredDeploymentLabel: "letorollout/enabled=true",
	})

	_, err := updater.CreateDeployment(context.Background(), DeploymentCreateRequest{
		Namespace: "default",
		Name:      "nginx",
		Image:     "nginx:1.27.0",
	})
	if err != nil {
		t.Fatalf("CreateDeployment returned error: %v", err)
	}

	created, err := client.AppsV1().Deployments("default").Get(context.Background(), "nginx", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get created deployment: %v", err)
	}
	if created.Labels["letorollout/enabled"] != "true" {
		t.Fatalf("label = %q, want true", created.Labels["letorollout/enabled"])
	}
	if created.Spec.Template.Labels["letorollout/enabled"] != "true" {
		t.Fatalf("template label = %q, want true", created.Spec.Template.Labels["letorollout/enabled"])
	}
}

func TestCreateDeploymentReturnsAlreadyExists(t *testing.T) {
	deployment := deploymentFixture("default", "nginx", []corev1.Container{
		{Name: "nginx", Image: "nginx:1.26.0"},
	})
	updater := NewDeploymentImageUpdater(fake.NewSimpleClientset(deployment))

	_, err := updater.CreateDeployment(context.Background(), DeploymentCreateRequest{
		Namespace: "default",
		Name:      "nginx",
		Image:     "nginx:1.27.0",
	})

	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("err = %v, want ErrAlreadyExists", err)
	}
}

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
