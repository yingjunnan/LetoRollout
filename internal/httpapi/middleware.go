package httpapi

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"letorollout/internal/auth"
)

// ctxKey is the context key under which authMiddleware stores the verified
// TokenRecord. Handlers retrieve it via tokenFromContext.
type ctxKey struct{}

func tokenFromContext(ctx context.Context) *auth.TokenRecord {
	v, _ := ctx.Value(ctxKey{}).(*auth.TokenRecord)
	return v
}

// bearerToken extracts the bearer token from the Authorization header,
// falling back to the ?token= query parameter. The fallback exists because
// EventSource (used for SSE log streaming) cannot set request headers.
func bearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

// authMiddleware verifies the bearer token against the TokenStore and stashes
// the verified record in the request context for downstream scope checks.
func authMiddleware(store *auth.TokenStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec, err := store.Verify(bearerToken(r))
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, &rec)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authOrAdminMiddleware accepts either a scoped user token (from the store) or
// the admin token. It is used by /api/v1/auth/verify so the endpoint can tell
// the frontend whether the supplied token is a user or admin token.
func authOrAdminMiddleware(store *auth.TokenStore, adminToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok := bearerToken(r)
			if adminToken != "" && subtle.ConstantTimeCompare([]byte(tok), []byte(adminToken)) == 1 {
				// synthesize an admin record so handleVerify can mark isAdmin
				rec := auth.TokenRecord{Token: tok, Label: "admin", Scopes: []auth.TokenScope{{Namespace: "*"}}}
				ctx := context.WithValue(r.Context(), ctxKey{}, &rec)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			rec, err := store.Verify(tok)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, &rec)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// adminMiddleware gates routes that mint or delete user tokens. It compares
// the bearer against ADMIN_TOKEN with a constant-time compare. When
// ADMIN_TOKEN is empty the admin API is disabled entirely (503).
func adminMiddleware(adminToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if adminToken == "" {
				http.Error(w, "admin not configured", http.StatusServiceUnavailable)
				return
			}
			if subtle.ConstantTimeCompare([]byte(bearerToken(r)), []byte(adminToken)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
