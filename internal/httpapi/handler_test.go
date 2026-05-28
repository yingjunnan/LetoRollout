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
	result RolloutResult
	err    error
	req    ImageUpdateRequest
	calls  int
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
