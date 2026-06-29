package httpapi

import (
	"context"
	"testing"
)

func TestPreviewServiceUpdateImage(t *testing.T) {
	svc := NewPreviewService()
	svc.SeedDeployment("default", "nginx", "nginx:1.27.0")

	result, err := svc.UpdateImage(context.Background(), ImageUpdateRequest{
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

	_, err := svc.UpdateImage(context.Background(), ImageUpdateRequest{
		Namespace:  "default",
		Deployment: "missing",
		Container:  "app",
		Image:      "nginx:1.28.0",
	})
	if err != ErrNotFound {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestPreviewServiceUpdateImageDryRunDoesNotPersist(t *testing.T) {
	svc := NewPreviewService()
	svc.SeedDeployment("default", "nginx", "nginx:1.27.0")

	_, err := svc.UpdateImage(context.Background(), ImageUpdateRequest{
		Namespace:  "default",
		Deployment: "nginx",
		Container:  "app",
		Image:      "nginx:1.28.0",
		DryRun:     true,
	})
	if err != nil {
		t.Fatalf("UpdateImage returned error: %v", err)
	}

	result, err := svc.UpdateImage(context.Background(), ImageUpdateRequest{
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
