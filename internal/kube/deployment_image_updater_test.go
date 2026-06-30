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

	"letorollout/internal/rollout"
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

func deploymentWithStatus(namespace, name string, replicas, ready int32, containers []corev1.Container) *appsv1.Deployment {
	d := deploymentFixture(namespace, name, containers).(*appsv1.Deployment)
	d.Spec.Replicas = int32Ptr(replicas)
	d.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}}
	d.Status = appsv1.DeploymentStatus{Replicas: replicas, ReadyReplicas: ready}
	return d
}

func TestListDeployments(t *testing.T) {
	client := fake.NewSimpleClientset(
		deploymentWithStatus("default", "api", 2, 2, []corev1.Container{{Name: "api", Image: "nginx:1.27"}}),
		deploymentWithStatus("default", "web", 3, 1, []corev1.Container{{Name: "web", Image: "redis:7"}}),
		deploymentWithStatus("prod", "api", 1, 1, []corev1.Container{{Name: "api", Image: "nginx:1.27"}}),
	)
	u := NewDeploymentImageUpdater(client)

	got, err := u.ListDeployments(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 deployments, got %d: %+v", len(got), got)
	}
	// first container image + readyReplicas populated
	var apiSummary *rollout.DeploymentSummary
	for i := range got {
		if got[i].Name == "api" {
			apiSummary = &got[i]
		}
	}
	if apiSummary == nil {
		t.Fatalf("api not in list: %+v", got)
	}
	if apiSummary.ReadyReplicas != 2 || len(apiSummary.Containers) != 1 || apiSummary.Containers[0].Image != "nginx:1.27" {
		t.Fatalf("api summary = %+v", apiSummary)
	}
}

func TestGetDeployment(t *testing.T) {
	client := fake.NewSimpleClientset(
		deploymentWithStatus("default", "api", 2, 2, []corev1.Container{{Name: "api", Image: "nginx:1.27"}}),
	)
	u := NewDeploymentImageUpdater(client)

	got, err := u.GetDeployment(context.Background(), "default", "api")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Name != "api" || len(got.Containers) != 1 || got.Containers[0].Name != "api" {
		t.Fatalf("got %+v", got)
	}
	if got.Selector == "" {
		t.Fatalf("expected non-empty selector")
	}
}

func TestGetDeploymentNotFound(t *testing.T) {
	u := NewDeploymentImageUpdater(fake.NewSimpleClientset())

	_, err := u.GetDeployment(context.Background(), "default", "missing")
	if !errors.Is(err, rollout.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestStreamLogsOpensAndClosesForKnownDeployment(t *testing.T) {
	dep := deploymentFixture("default", "api", []corev1.Container{{Name: "api", Image: "nginx:1.27"}}).(*appsv1.Deployment)
	dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}}
	dep.Spec.Template.Labels = map[string]string{"app": "api"}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-abc", Namespace: "default", Labels: map[string]string{"app": "api"}},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "api"}}},
	}
	client := fake.NewSimpleClientset(dep, pod)
	u := NewDeploymentImageUpdater(client)

	ch, err := u.StreamLogs(context.Background(), rollout.LogRequest{Namespace: "default", Deployment: "api", TailLines: 10})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// fake clientset returns no log bytes; assert the stream opens and closes cleanly.
	for range ch {
	}
}

func TestStreamLogsMissingDeployment(t *testing.T) {
	u := NewDeploymentImageUpdater(fake.NewSimpleClientset())

	_, err := u.StreamLogs(context.Background(), rollout.LogRequest{Namespace: "default", Deployment: "missing"})
	if !errors.Is(err, rollout.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestStreamLogsDeploymentWithoutPods(t *testing.T) {
	dep := deploymentFixture("default", "api", []corev1.Container{{Name: "api"}}).(*appsv1.Deployment)
	dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}}
	client := fake.NewSimpleClientset(dep) // no Pods
	u := NewDeploymentImageUpdater(client)

	_, err := u.StreamLogs(context.Background(), rollout.LogRequest{Namespace: "default", Deployment: "api"})
	if !errors.Is(err, rollout.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound (no pods)", err)
	}
}

