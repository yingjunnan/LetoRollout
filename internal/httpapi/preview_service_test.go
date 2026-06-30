package httpapi

import (
	"context"
	"errors"
	"testing"

	"letorollout/internal/rollout"
)

func TestPreviewServiceUpdateImage(t *testing.T) {
	svc := NewPreviewService()
	svc.SeedDeployment(rollout.DeploymentSummary{
		Name: "nginx", Namespace: "default",
		Containers: []rollout.ContainerInfo{{Name: "app", Image: "nginx:1.27.0"}},
	})

	result, err := svc.UpdateImage(context.Background(), rollout.ImageUpdateRequest{
		Namespace:  "default",
		Deployment: "nginx",
		Container:  "app",
		Image:      "nginx:1.28.0",
	})
	if err != nil {
		t.Fatalf("UpdateImage returned error: %v", err)
	}
	if result.OldImage != "nginx:1.27.0" || result.NewImage != "nginx:1.28.0" {
		t.Fatalf("update result = %+v", result)
	}
}

func TestPreviewServiceUpdateImageMissingDeployment(t *testing.T) {
	svc := NewPreviewService()

	_, err := svc.UpdateImage(context.Background(), rollout.ImageUpdateRequest{
		Namespace:  "default",
		Deployment: "missing",
		Container:  "app",
		Image:      "nginx:1.28.0",
	})
	if !errors.Is(err, rollout.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPreviewServiceUpdateImageDryRunDoesNotPersist(t *testing.T) {
	svc := NewPreviewService()
	svc.SeedDeployment(rollout.DeploymentSummary{
		Name: "nginx", Namespace: "default",
		Containers: []rollout.ContainerInfo{{Name: "app", Image: "nginx:1.27.0"}},
	})

	_, err := svc.UpdateImage(context.Background(), rollout.ImageUpdateRequest{
		Namespace:  "default",
		Deployment: "nginx",
		Container:  "app",
		Image:      "nginx:1.28.0",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("UpdateImage returned error: %v", err)
	}

	result, err := svc.UpdateImage(context.Background(), rollout.ImageUpdateRequest{
		Namespace:  "default",
		Deployment: "nginx",
		Container:  "app",
		Image:      "nginx:1.29.0",
	})
	if err != nil {
		t.Fatalf("UpdateImage returned error: %v", err)
	}
	if result.OldImage != "nginx:1.27.0" {
		t.Fatalf("dry run should not persist; old image = %q", result.OldImage)
	}
}

func TestPreviewListDeployments(t *testing.T) {
	svc := NewPreviewService()
	svc.SeedDeployment(rollout.DeploymentSummary{
		Name: "api", Namespace: "default",
		Replicas: 2, ReadyReplicas: 2,
		Containers: []rollout.ContainerInfo{{Name: "api", Image: "nginx:1.27"}},
	})
	got, err := svc.ListDeployments(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 1 || got[0].Name != "api" {
		t.Fatalf("got %+v", got)
	}
	// wrong namespace returns empty, not error
	got, err = svc.ListDeployments(context.Background(), "other")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %+v", got)
	}
}

func TestPreviewGetDeployment(t *testing.T) {
	svc := NewPreviewService()
	svc.SeedDeployment(rollout.DeploymentSummary{
		Name: "api", Namespace: "default",
		Containers: []rollout.ContainerInfo{{Name: "api", Image: "nginx:1.27"}},
	})
	got, err := svc.GetDeployment(context.Background(), "default", "api")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Name != "api" || len(got.Containers) != 1 {
		t.Fatalf("got %+v", got)
	}
	if _, err := svc.GetDeployment(context.Background(), "default", "missing"); !errors.Is(err, rollout.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPreviewStreamLogsOneShot(t *testing.T) {
	svc := NewPreviewService()
	svc.SeedDeployment(rollout.DeploymentSummary{
		Name: "api", Namespace: "default",
		Containers: []rollout.ContainerInfo{{Name: "api"}},
	})
	ch, err := svc.StreamLogs(context.Background(), rollout.LogRequest{Namespace: "default", Deployment: "api"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	var lines []string
	for ll := range ch {
		if ll.Error != nil {
			t.Fatalf("stream err: %v", ll.Error)
		}
		lines = append(lines, ll.Line)
	}
	if len(lines) == 0 {
		t.Fatal("expected canned log lines")
	}
}

func TestPreviewStreamLogsMissingDeployment(t *testing.T) {
	svc := NewPreviewService()
	_, err := svc.StreamLogs(context.Background(), rollout.LogRequest{Namespace: "default", Deployment: "missing"})
	if !errors.Is(err, rollout.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestPreviewStreamLogsFollowEmitsThenCancels(t *testing.T) {
	svc := NewPreviewService()
	svc.SeedDeployment(rollout.DeploymentSummary{
		Name: "api", Namespace: "default",
		Containers: []rollout.ContainerInfo{{Name: "api"}},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := svc.StreamLogs(ctx, rollout.LogRequest{Namespace: "default", Deployment: "api", Follow: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	gotLine := false
	for ll := range ch {
		if ll.Error != nil {
			break
		}
		gotLine = true
		cancel()
	}
	if !gotLine {
		t.Fatal("expected at least one follow line")
	}
}
