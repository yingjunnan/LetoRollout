package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"letorollout/internal/auth"
)

func TestAuthMiddlewareMissingHeader(t *testing.T) {
	store, _ := auth.LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	h := authMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not be called without a token")
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}

func TestAuthMiddlewareValidToken(t *testing.T) {
	store, _ := auth.LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	rec, _ := store.Create(auth.TokenRecord{Scopes: []auth.TokenScope{{Namespace: "dev"}}})

	called := false
	h := authMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		got := tokenFromContext(r.Context())
		if got == nil || got.ID != rec.ID {
			t.Fatalf("record not in context: %+v", got)
		}
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestAuthMiddlewareBadToken(t *testing.T) {
	store, _ := auth.LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	h := authMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not be called with a bad token")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer nope")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}

func TestAuthMiddlewareTokenFromQuery(t *testing.T) {
	store, _ := auth.LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	rec, _ := store.Create(auth.TokenRecord{Scopes: []auth.TokenScope{{Namespace: "dev"}}})

	called := false
	h := authMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	// EventSource cannot set headers, so the token may arrive via ?token=
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?token="+rec.Token, nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestAdminMiddlewareValidToken(t *testing.T) {
	h := adminMiddleware("s3cr3t")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer s3cr3t")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestAdminMiddlewareWrongToken(t *testing.T) {
	h := adminMiddleware("s3cr3t")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not be called with a wrong admin token")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}

func TestAdminMiddlewareEmptyAdminToken(t *testing.T) {
	h := adminMiddleware("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler must not be called when admin is unconfigured")
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer whatever")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", rr.Code)
	}
}
