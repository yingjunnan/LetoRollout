package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"letorollout/internal/auth"
	"letorollout/internal/rollout"
)

type fakeService struct {
	result        rollout.RolloutResult
	err           error
	req           rollout.ImageUpdateRequest
	calls         int
	deployments   []rollout.DeploymentSummary
	logs          []string
	logRequest    rollout.LogRequest
	logCalls      int
	getDeployment rollout.DeploymentDetail
	getErr        error
}

func (f *fakeService) UpdateImage(ctx context.Context, req rollout.ImageUpdateRequest) (rollout.RolloutResult, error) {
	f.calls++
	f.req = req
	return f.result, f.err
}

func (f *fakeService) ListDeployments(ctx context.Context, namespace string) ([]rollout.DeploymentSummary, error) {
	var out []rollout.DeploymentSummary
	for _, d := range f.deployments {
		if d.Namespace == namespace {
			out = append(out, d)
		}
	}
	return out, nil
}

func (f *fakeService) GetDeployment(ctx context.Context, namespace, name string) (rollout.DeploymentDetail, error) {
	if f.getErr != nil {
		return rollout.DeploymentDetail{}, f.getErr
	}
	if f.getDeployment.Name != "" {
		return f.getDeployment, nil
	}
	for _, d := range f.deployments {
		if d.Namespace == namespace && d.Name == name {
			return rollout.DeploymentDetail{DeploymentSummary: d}, nil
		}
	}
	return rollout.DeploymentDetail{}, rollout.ErrNotFound
}

func (f *fakeService) StreamLogs(ctx context.Context, req rollout.LogRequest) (<-chan rollout.LogLine, error) {
	f.logCalls++
	f.logRequest = req
	out := make(chan rollout.LogLine)
	go func() {
		defer close(out)
		for _, l := range f.logs {
			out <- rollout.LogLine{Line: l}
		}
	}()
	return out, nil
}

// newTestHandler builds a handler with a token scoped to namespace "dev" and
// admin token "adm". Returns the handler, the store, and the user token record.
func newTestHandler(t *testing.T, svc *fakeService) (http.Handler, *auth.TokenStore, auth.TokenRecord) {
	t.Helper()
	store, _ := auth.LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	rec, err := store.Create(auth.TokenRecord{Scopes: []auth.TokenScope{{Namespace: "dev"}}})
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	if svc == nil {
		svc = &fakeService{
			deployments: []rollout.DeploymentSummary{{Name: "api", Namespace: "dev"}},
			logs:        []string{"hello"},
		}
	}
	h := NewHandler(Config{AdminToken: "adm", LogTailLines: 10}, svc, store)
	return h, store, rec
}

func TestHealthz(t *testing.T) {
	h, _, _ := newTestHandler(t, nil)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusOK)
	}
}

func TestRootRedirectsToConsole(t *testing.T) {
	h, _, _ := newTestHandler(t, nil)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusMovedPermanently)
	}
	if got := resp.Header().Get("Location"); got != "/console/" {
		t.Fatalf("location = %q, want /console/", got)
	}
}

func TestConsoleRedirectsToTrailingSlash(t *testing.T) {
	h, _, _ := newTestHandler(t, nil)

	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/console", nil)
	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", resp.Code, http.StatusMovedPermanently)
	}
	if got := resp.Header().Get("Location"); got != "/console/" {
		t.Fatalf("location = %q, want /console/", got)
	}
}

func TestConsoleServesIndexHTML(t *testing.T) {
	h, _, _ := newTestHandler(t, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/console/", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if got := rr.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("content type = %q, want text/html; charset=utf-8", got)
	}
	// the React build's index.html mounts #root and loads the JS bundle
	body := rr.Body.String()
	if !strings.Contains(body, "id=\"root\"") {
		t.Fatalf("index.html missing root mount point; body=%s", body)
	}
}

func TestUserRoutesRequireToken(t *testing.T) {
	h, _, _ := newTestHandler(t, nil)

	for _, path := range []string{
		"/api/v1/namespaces/dev/deployments",
		"/api/v1/namespaces/dev/deployments/api",
		"/api/v1/namespaces/dev/deployments/api/logs",
	} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("%s: want 401, got %d", path, rr.Code)
		}
	}
}

func TestListDeploymentsRoute(t *testing.T) {
	h, _, rec := newTestHandler(t, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/dev/deployments", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body)
	}
}

func TestListDeploymentsDeniedNamespace(t *testing.T) {
	h, _, rec := newTestHandler(t, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/prod/deployments", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", rr.Code)
	}
}

func TestGetDeploymentRoute(t *testing.T) {
	h, _, rec := newTestHandler(t, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/dev/deployments/api", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body)
	}
}

func TestGetDeploymentNotFound(t *testing.T) {
	h, _, rec := newTestHandler(t, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/dev/deployments/missing", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", rr.Code)
	}
}

func TestLogsOneShotRoute(t *testing.T) {
	h, _, rec := newTestHandler(t, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/dev/deployments/api/logs", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "hello") {
		t.Fatalf("expected log text, got %q", rr.Body.String())
	}
}

func TestLogsStreamRoute(t *testing.T) {
	h, _, rec := newTestHandler(t, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/dev/deployments/api/logs/stream", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content type = %q, want text/event-stream", ct)
	}
	if !strings.Contains(rr.Body.String(), "event: log") {
		t.Fatalf("expected SSE log event, got %q", rr.Body.String())
	}
}

func TestVerifyRoute(t *testing.T) {
	h, _, rec := newTestHandler(t, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	var resp struct {
		IsAdmin bool `json:"isAdmin"`
		Scopes  []struct {
			Namespace  string `json:"namespace"`
			Deployment string `json:"deployment"`
		} `json:"scopes"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.IsAdmin {
		t.Fatal("user token should not be admin")
	}
	if len(resp.Scopes) != 1 || resp.Scopes[0].Namespace != "dev" {
		t.Fatalf("scopes = %+v", resp.Scopes)
	}
}

func TestVerifyRouteAdmin(t *testing.T) {
	h, _, _ := newTestHandler(t, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify", nil)
	req.Header.Set("Authorization", "Bearer adm")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	var resp struct {
		IsAdmin bool `json:"isAdmin"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if !resp.IsAdmin {
		t.Fatal("admin token should set isAdmin=true")
	}
}

func TestAdminCreateToken(t *testing.T) {
	h, store, _ := newTestHandler(t, nil)

	body := `{"label":"x","scopes":[{"namespace":"prod"}]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tokens", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer adm")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body)
	}
	if len(store.List()) != 2 { // the seeded dev token + the new one
		t.Fatalf("expected 2 tokens, got %d", len(store.List()))
	}

	var created auth.TokenRecord
	if err := json.Unmarshal(rr.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Token == "" {
		t.Fatal("create response should return plaintext token once")
	}
}

func TestAdminCreateTokenRequiresAdminToken(t *testing.T) {
	h, _, _ := newTestHandler(t, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/tokens", bytes.NewBufferString(`{"scopes":[{"namespace":"prod"}]}`))
	req.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}

func TestAdminListTokensOmitsPlaintext(t *testing.T) {
	h, _, _ := newTestHandler(t, nil)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tokens", nil)
	req.Header.Set("Authorization", "Bearer adm")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	var list []auth.TokenRecord
	if err := json.Unmarshal(rr.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 token, got %d", len(list))
	}
	if list[0].Token != "" {
		t.Fatal("list must not expose plaintext token")
	}
}

func TestAdminDeleteToken(t *testing.T) {
	h, store, _ := newTestHandler(t, nil)

	list := store.List()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/tokens/"+list[0].ID, nil)
	req.Header.Set("Authorization", "Bearer adm")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("want 204, got %d", rr.Code)
	}
	if len(store.List()) != 0 {
		t.Fatalf("expected 0 tokens after delete, got %d", len(store.List()))
	}
}

func TestUpdateImageRejectsGet(t *testing.T) {
	svc := &fakeService{}
	h, _, rec := newTestHandler(t, svc)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/namespaces/dev/deployments/api/image", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	// POST-only pattern route: GET does not match, falls through to 404.
	if rr.Code == http.StatusOK {
		t.Fatalf("GET should not succeed; got %d", rr.Code)
	}
	if svc.calls != 0 {
		t.Fatalf("service called %d times, want 0", svc.calls)
	}
}

func TestUpdateImageRejectsMissingFields(t *testing.T) {
	h, _, rec := newTestHandler(t, &fakeService{})

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/dev/deployments/api/image", bytes.NewBufferString(`{"namespace":"dev"}`))
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestUpdateImageReturnsRolloutResult(t *testing.T) {
	svc := &fakeService{
		result: rollout.RolloutResult{
			Namespace:       "dev",
			Deployment:      "api",
			Container:       "api",
			OldImage:        "nginx:1.26.0",
			NewImage:        "nginx:1.27.0",
			Generation:      7,
			DryRun:          true,
			RolloutComplete: true,
		},
	}
	h, _, rec := newTestHandler(t, svc)

	body := bytes.NewBufferString(`{"namespace":"dev","deployment":"api","container":"api","image":"nginx:1.27.0","dryRun":true,"wait":true,"timeoutSeconds":120}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/dev/deployments/api/image", body)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if svc.req.Container != "api" || svc.req.Image != "nginx:1.27.0" || !svc.req.DryRun || !svc.req.Wait || svc.req.TimeoutSeconds != 120 {
		t.Fatalf("request = %+v, want parsed request", svc.req)
	}

	var got rollout.RolloutResult
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.OldImage != "nginx:1.26.0" || got.NewImage != "nginx:1.27.0" || got.Generation != 7 {
		t.Fatalf("response = %+v, want rollout result", got)
	}
}

func TestUpdateImageMapsNotFoundTo404(t *testing.T) {
	h, _, rec := newTestHandler(t, &fakeService{err: rollout.ErrNotFound})

	body := bytes.NewBufferString(`{"namespace":"dev","deployment":"missing","container":"api","image":"nginx:1.27.0"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/dev/deployments/missing/image", body)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusNotFound)
	}
}

func TestUpdateImageMapsUnexpectedErrorsTo500(t *testing.T) {
	h, _, rec := newTestHandler(t, &fakeService{err: errors.New("api failed")})

	body := bytes.NewBufferString(`{"namespace":"dev","deployment":"api","container":"api","image":"nginx:1.27.0"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/dev/deployments/api/image", body)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusInternalServerError)
	}
}

func TestUpdateImageMapsForbiddenTo403(t *testing.T) {
	// token scoped to dev, request targets prod -> 403
	h, _, rec := newTestHandler(t, &fakeService{})

	body := bytes.NewBufferString(`{"namespace":"prod","deployment":"api","container":"api","image":"nginx:1.27.0"}`)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/namespaces/prod/deployments/api/image", body)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusForbidden)
	}
}
