package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeRolloutService struct {
	result       RolloutResult
	err          error
	req          ImageUpdateRequest
	calls        int
	createResult DeploymentCreateResult
	createErr    error
	createReq    DeploymentCreateRequest
	createCalls  int
}

func (f *fakeRolloutService) CreateDeployment(ctx context.Context, req DeploymentCreateRequest) (DeploymentCreateResult, error) {
	f.createCalls++
	f.createReq = req
	return f.createResult, f.createErr
}

func (f *fakeRolloutService) UpdateImage(ctx context.Context, req ImageUpdateRequest) (RolloutResult, error) {
	f.calls++
	f.req = req
	return f.result, f.err
}

func TestHealthz(t *testing.T) {
	handler := NewHandler(&fakeRolloutService{}, "")

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
}

func TestUpdateImageRequiresPost(t *testing.T) {
	handler := NewHandler(&fakeRolloutService{}, "")

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/deployments/image", nil)
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusMethodNotAllowed)
	}
}

func TestCreateDeploymentRequiresPost(t *testing.T) {
	handler := NewHandler(&fakeRolloutService{}, "")

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/deployments", nil)
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusMethodNotAllowed)
	}
}

func TestCreateDeploymentRejectsInvalidBearerToken(t *testing.T) {
	svc := &fakeRolloutService{}
	handler := NewHandler(svc, "secret")

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments", bytes.NewBufferString(`{}`))
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusUnauthorized)
	}
	if svc.createCalls != 0 {
		t.Fatalf("service called %d times, want 0", svc.createCalls)
	}
}

func TestCreateDeploymentRejectsMissingFields(t *testing.T) {
	svc := &fakeRolloutService{}
	handler := NewHandler(svc, "")

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments", bytes.NewBufferString(`{"namespace":"default"}`))
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
	}
	if svc.createCalls != 0 {
		t.Fatalf("service called %d times, want 0", svc.createCalls)
	}
}

func TestCreateDeploymentReturnsCreatedDeployment(t *testing.T) {
	svc := &fakeRolloutService{
		createResult: DeploymentCreateResult{
			Namespace:  "default",
			Name:       "nginx",
			Container:  "app",
			Image:      "nginx:1.27.0",
			Replicas:   1,
			Generation: 1,
			Env: []DeploymentEnvVar{
				{Name: "APP_ENV", Value: stringPtr("prod")},
				{Name: "DATABASE_URL", Secret: &DeploymentEnvSecret{Name: "nginx-secret", Key: "database-url"}},
			},
		},
	}
	handler := NewHandler(svc, "secret")

	body := bytes.NewBufferString(`{"namespace":"default","name":"nginx","image":"nginx:1.27.0","env":[{"name":"APP_ENV","value":"prod"},{"name":"DATABASE_URL","secret":{"name":"nginx-secret","key":"database-url"}}]}`)
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments", body)
	req.Header.Set("Authorization", "Bearer secret")
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusCreated, resp.Body.String())
	}
	if svc.createReq.Namespace != "default" || svc.createReq.Name != "nginx" || svc.createReq.Image != "nginx:1.27.0" {
		t.Fatalf("request = %+v, want parsed request", svc.createReq)
	}
	if len(svc.createReq.Env) != 2 || svc.createReq.Env[0].Name != "APP_ENV" || svc.createReq.Env[0].Value == nil || *svc.createReq.Env[0].Value != "prod" {
		t.Fatalf("request env = %+v, want literal APP_ENV=prod", svc.createReq.Env)
	}
	if svc.createReq.Env[1].Secret == nil || svc.createReq.Env[1].Secret.Name != "nginx-secret" || svc.createReq.Env[1].Secret.Key != "database-url" {
		t.Fatalf("request env = %+v, want secret ref", svc.createReq.Env)
	}

	var got DeploymentCreateResult
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Name != "nginx" || got.Container != "app" || got.Image != "nginx:1.27.0" || got.Replicas != 1 {
		t.Fatalf("response = %+v, want create result", got)
	}
	if len(got.Env) != 2 || got.Env[0].Value == nil || *got.Env[0].Value != "prod" || got.Env[1].Secret == nil {
		t.Fatalf("response env = %+v, want accepted env", got.Env)
	}
}

func TestCreateDeploymentRejectsInvalidEnv(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "missing env name",
			body: `{"namespace":"default","name":"nginx","image":"nginx:1.27.0","env":[{"value":"prod"}]}`,
		},
		{
			name: "missing env source",
			body: `{"namespace":"default","name":"nginx","image":"nginx:1.27.0","env":[{"name":"APP_ENV"}]}`,
		},
		{
			name: "both value and secret",
			body: `{"namespace":"default","name":"nginx","image":"nginx:1.27.0","env":[{"name":"DATABASE_URL","value":"x","secret":{"name":"nginx-secret","key":"database-url"}}]}`,
		},
		{
			name: "missing secret name",
			body: `{"namespace":"default","name":"nginx","image":"nginx:1.27.0","env":[{"name":"DATABASE_URL","secret":{"key":"database-url"}}]}`,
		},
		{
			name: "missing secret key",
			body: `{"namespace":"default","name":"nginx","image":"nginx:1.27.0","env":[{"name":"DATABASE_URL","secret":{"name":"nginx-secret"}}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeRolloutService{}
			handler := NewHandler(svc, "")

			resp := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments", bytes.NewBufferString(tt.body))
			handler.ServeHTTP(resp, req)

			if resp.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
			}
			if svc.createCalls != 0 {
				t.Fatalf("service called %d times, want 0", svc.createCalls)
			}
		})
	}
}

func TestCreateDeploymentMapsAlreadyExistsTo409(t *testing.T) {
	handler := NewHandler(&fakeRolloutService{createErr: ErrAlreadyExists}, "")

	body := bytes.NewBufferString(`{"namespace":"default","name":"nginx","image":"nginx:1.27.0"}`)
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments", body)
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusConflict)
	}
}

func TestUpdateImageRejectsInvalidBearerToken(t *testing.T) {
	svc := &fakeRolloutService{}
	handler := NewHandler(svc, "secret")

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/image", bytes.NewBufferString(`{}`))
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusUnauthorized)
	}
	if svc.calls != 0 {
		t.Fatalf("service called %d times, want 0", svc.calls)
	}
}

func TestUpdateImageRejectsMissingFields(t *testing.T) {
	svc := &fakeRolloutService{}
	handler := NewHandler(svc, "")

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/image", bytes.NewBufferString(`{"namespace":"default"}`))
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusBadRequest)
	}
	if svc.calls != 0 {
		t.Fatalf("service called %d times, want 0", svc.calls)
	}
}

func TestUpdateImageReturnsRolloutResult(t *testing.T) {
	svc := &fakeRolloutService{
		result: RolloutResult{
			Namespace:       "default",
			Deployment:      "nginx",
			Container:       "nginx",
			OldImage:        "nginx:1.26.0",
			NewImage:        "nginx:1.27.0",
			Generation:      7,
			DryRun:          true,
			RolloutComplete: true,
		},
	}
	handler := NewHandler(svc, "secret")

	body := bytes.NewBufferString(`{"namespace":"default","deployment":"nginx","container":"nginx","image":"nginx:1.27.0","dryRun":true,"wait":true,"timeoutSeconds":120}`)
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/image", body)
	req.Header.Set("Authorization", "Bearer secret")
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if svc.req.Namespace != "default" || svc.req.Deployment != "nginx" || svc.req.Container != "nginx" || svc.req.Image != "nginx:1.27.0" || !svc.req.DryRun || !svc.req.Wait || svc.req.TimeoutSeconds != 120 {
		t.Fatalf("request = %+v, want parsed request", svc.req)
	}

	var got RolloutResult
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.OldImage != "nginx:1.26.0" || got.NewImage != "nginx:1.27.0" || got.Generation != 7 {
		t.Fatalf("response = %+v, want rollout result", got)
	}
	if !got.DryRun || !got.RolloutComplete {
		t.Fatalf("response = %+v, want dry run and complete flags", got)
	}
}

func TestUpdateImageMapsNotFoundTo404(t *testing.T) {
	handler := NewHandler(&fakeRolloutService{err: ErrNotFound}, "")

	body := bytes.NewBufferString(`{"namespace":"default","deployment":"missing","container":"nginx","image":"nginx:1.27.0"}`)
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/image", body)
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusNotFound)
	}
}

func TestUpdateImageMapsUnexpectedErrorsTo500(t *testing.T) {
	handler := NewHandler(&fakeRolloutService{err: errors.New("api failed")}, "")

	body := bytes.NewBufferString(`{"namespace":"default","deployment":"nginx","container":"nginx","image":"nginx:1.27.0"}`)
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/image", body)
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusInternalServerError)
	}
}

func TestUpdateImageMapsForbiddenTo403(t *testing.T) {
	handler := NewHandler(&fakeRolloutService{err: ErrForbidden}, "")

	body := bytes.NewBufferString(`{"namespace":"prod","deployment":"nginx","container":"nginx","image":"nginx:1.27.0"}`)
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deployments/image", body)
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusForbidden)
	}
}

func stringPtr(v string) *string {
	return &v
}
