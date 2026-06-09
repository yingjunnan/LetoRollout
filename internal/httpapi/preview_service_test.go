package httpapi

import (
	"context"
	"testing"
)

func TestPreviewServiceCreatesAndUpdatesDeployments(t *testing.T) {
	svc := NewPreviewService()

	createResult, err := svc.CreateDeployment(context.Background(), DeploymentCreateRequest{
		Namespace: "default",
		Name:      "nginx",
		Image:     "nginx:1.27.0",
		Env: []DeploymentEnvVar{
			{Name: "APP_ENV", Value: stringPtr("prod")},
		},
	})
	if err != nil {
		t.Fatalf("CreateDeployment returned error: %v", err)
	}
	if createResult.Namespace != "default" || createResult.Name != "nginx" || createResult.Image != "nginx:1.27.0" {
		t.Fatalf("create result = %+v", createResult)
	}
	if len(createResult.Env) != 1 || createResult.Env[0].Value == nil || *createResult.Env[0].Value != "prod" {
		t.Fatalf("create env = %+v", createResult.Env)
	}

	updateResult, err := svc.UpdateImage(context.Background(), ImageUpdateRequest{
		Namespace:  "default",
		Deployment: "nginx",
		Container:  "app",
		Image:      "nginx:1.28.0",
	})
	if err != nil {
		t.Fatalf("UpdateImage returned error: %v", err)
	}
	if updateResult.OldImage != "nginx:1.27.0" || updateResult.NewImage != "nginx:1.28.0" {
		t.Fatalf("update result = %+v", updateResult)
	}
}
