package httpapi

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"letorollout/internal/auth"
	"letorollout/internal/rollout"
)

//go:embed static/*
var staticFS embed.FS

var ErrNotFound = rollout.ErrNotFound
var ErrForbidden = rollout.ErrForbidden
var ErrAlreadyExists = rollout.ErrAlreadyExists
var ErrUnauthorized = rollout.ErrUnauthorized
var ErrTokenExpired = rollout.ErrTokenExpired

type ImageUpdateRequest = rollout.ImageUpdateRequest
type RolloutResult = rollout.RolloutResult

// Service is the union of the read/write/log capabilities a handler needs.
// Both kube.DeploymentImageUpdater and PreviewService satisfy it.
type Service interface {
	rollout.ImageUpdater
	rollout.DeploymentReader
	rollout.LogStreamer
}

// Config holds the per-handler configuration wired in main.
type Config struct {
	AdminToken   string
	LogTailLines int64
}

func NewHandler(cfg Config, service Service, store *auth.TokenStore) http.Handler {
	return NewHandlerWithAuditWriter(cfg, service, store, os.Stdout)
}

func NewHandlerWithAuditWriter(cfg Config, service Service, store *auth.TokenStore, audit io.Writer) http.Handler {
	userMw := authMiddleware(store)
	adminMw := adminMiddleware(cfg.AdminToken)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /", handleRoot)
	mux.HandleFunc("GET /console", handleConsoleRedirect)
	mux.Handle("GET /console/", consoleHandler())

	// auth (accepts either a user token or the admin token, so verify can
	// report which kind it is)
	mux.Handle("POST /api/v1/auth/verify", authOrAdminMiddleware(store, cfg.AdminToken)(handleVerify(cfg.AdminToken)))

	// user (token-scoped)
	mux.Handle("GET /api/v1/namespaces/{ns}/deployments", userMw(handleListDeployments(service)))
	mux.Handle("GET /api/v1/namespaces/{ns}/deployments/{name}", userMw(handleGetDeployment(service)))
	mux.Handle("POST /api/v1/namespaces/{ns}/deployments/{name}/image", userMw(handleUpdateImage(service, audit)))
	mux.Handle("GET /api/v1/namespaces/{ns}/deployments/{name}/logs", userMw(handleLogs(service, cfg.LogTailLines)))
	mux.Handle("GET /api/v1/namespaces/{ns}/deployments/{name}/logs/stream", userMw(handleLogsStream(service, cfg.LogTailLines)))

	// admin
	mux.Handle("GET /api/v1/admin/tokens", adminMw(handleAdminListTokens(store)))
	mux.Handle("POST /api/v1/admin/tokens", adminMw(handleAdminCreateToken(store)))
	mux.Handle("DELETE /api/v1/admin/tokens/{id}", adminMw(handleAdminDeleteToken(store)))

	return mux
}

type PreviewService struct {
	deployments []rollout.DeploymentSummary
}

func NewPreviewService() *PreviewService {
	return &PreviewService{}
}

// SeedDeployment registers a deployment in the preview store.
func (s *PreviewService) SeedDeployment(d rollout.DeploymentSummary) {
	s.deployments = append(s.deployments, d)
}

func (s *PreviewService) findDeployment(namespace, name string) (rollout.DeploymentSummary, int, bool) {
	for i, d := range s.deployments {
		if d.Namespace == namespace && d.Name == name {
			return d, i, true
		}
	}
	return rollout.DeploymentSummary{}, -1, false
}

func (s *PreviewService) UpdateImage(ctx context.Context, req rollout.ImageUpdateRequest) (rollout.RolloutResult, error) {
	dep, idx, exists := s.findDeployment(req.Namespace, req.Deployment)
	if !exists {
		return rollout.RolloutResult{}, rollout.ErrNotFound
	}

	var oldImage string
	containerIndex := -1
	for i, c := range dep.Containers {
		if c.Name == req.Container {
			containerIndex = i
			oldImage = c.Image
			break
		}
	}
	if containerIndex == -1 {
		return rollout.RolloutResult{}, rollout.ErrNotFound
	}

	result := rollout.RolloutResult{
		Namespace:       req.Namespace,
		Deployment:      req.Deployment,
		Container:       req.Container,
		OldImage:        oldImage,
		NewImage:        req.Image,
		Generation:      2,
		DryRun:          req.DryRun,
		RolloutComplete: true,
	}
	if !req.DryRun {
		s.deployments[idx].Containers[containerIndex].Image = req.Image
	}
	return result, nil
}

func (s *PreviewService) ListDeployments(ctx context.Context, namespace string) ([]rollout.DeploymentSummary, error) {
	out := make([]rollout.DeploymentSummary, 0, len(s.deployments))
	for _, d := range s.deployments {
		if d.Namespace == namespace {
			out = append(out, d)
		}
	}
	return out, nil
}

func (s *PreviewService) GetDeployment(ctx context.Context, namespace, name string) (rollout.DeploymentDetail, error) {
	d, _, ok := s.findDeployment(namespace, name)
	if !ok {
		return rollout.DeploymentDetail{}, rollout.ErrNotFound
	}
	return rollout.DeploymentDetail{DeploymentSummary: d, Selector: "app=" + name}, nil
}

func (s *PreviewService) StreamLogs(ctx context.Context, req rollout.LogRequest) (<-chan rollout.LogLine, error) {
	if _, _, ok := s.findDeployment(req.Namespace, req.Deployment); !ok {
		return nil, rollout.ErrNotFound
	}
	out := make(chan rollout.LogLine)
	go func() {
		defer close(out)
		canned := []string{"[preview] line one", "[preview] line two", "[preview] line three"}
		for _, l := range canned {
			select {
			case out <- rollout.LogLine{Line: l}:
			case <-ctx.Done():
				return
			}
		}
		if !req.Follow {
			return
		}
		t := time.NewTicker(time.Second)
		defer t.Stop()
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				select {
				case out <- rollout.LogLine{Line: fmt.Sprintf("[preview] follow line %d", i)}:
				case <-ctx.Done():
					return
				}
				i++
			}
		}
	}()
	return out, nil
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	http.Redirect(w, r, "/console/", http.StatusMovedPermanently)
}

func handleConsoleRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/console/", http.StatusMovedPermanently)
}

func consoleHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writeError(w, http.StatusInternalServerError, "console unavailable")
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/console/" {
			data, err := staticFS.ReadFile("static/console.html")
			if err != nil {
				writeError(w, http.StatusInternalServerError, "console unavailable")
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(data)
			return
		}

		r2 := r.Clone(r.Context())
		r2.URL.Path = path.Clean(strings.TrimPrefix(r.URL.Path, "/console"))
		if r2.URL.Path == "." {
			r2.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r2)
	})
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleUpdateImage(service Service, audit io.Writer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := tokenFromContext(r.Context())
		ns, name := r.PathValue("ns"), r.PathValue("name")
		if !rec.Allows(ns, name) {
			writeServiceError(w, rollout.ErrForbidden)
			return
		}

		var req rollout.ImageUpdateRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		req.Namespace = ns
		req.Deployment = name
		req = trimRequest(req)
		if err := validateRequest(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		result, err := service.UpdateImage(r.Context(), req)
		writeAudit(audit, req, result, err)
		if err != nil {
			writeServiceError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
}

func handleListDeployments(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := tokenFromContext(r.Context())
		ns := r.PathValue("ns")
		if !rec.Allows(ns, "") {
			writeServiceError(w, rollout.ErrForbidden)
			return
		}
		deps, err := service.ListDeployments(r.Context(), ns)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, deps)
	}
}

func handleGetDeployment(service Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := tokenFromContext(r.Context())
		ns, name := r.PathValue("ns"), r.PathValue("name")
		if !rec.Allows(ns, name) {
			writeServiceError(w, rollout.ErrForbidden)
			return
		}
		d, err := service.GetDeployment(r.Context(), ns, name)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, d)
	}
}

func handleLogs(service Service, defaultTail int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := tokenFromContext(r.Context())
		ns, name := r.PathValue("ns"), r.PathValue("name")
		if !rec.Allows(ns, name) {
			writeServiceError(w, rollout.ErrForbidden)
			return
		}
		req := rollout.LogRequest{
			Namespace:  ns,
			Deployment: name,
			Container:  r.URL.Query().Get("container"),
			Previous:   r.URL.Query().Has("previous"),
		}
		req.TailLines = parseTailLines(r, defaultTail)
		ch, err := service.StreamLogs(r.Context(), req)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		flusher, _ := w.(http.Flusher)
		for ll := range ch {
			if ll.Error != nil {
				return
			}
			fmt.Fprintln(w, ll.Line)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

func handleLogsStream(service Service, defaultTail int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := tokenFromContext(r.Context())
		ns, name := r.PathValue("ns"), r.PathValue("name")
		if !rec.Allows(ns, name) {
			writeServiceError(w, rollout.ErrForbidden)
			return
		}
		req := rollout.LogRequest{
			Namespace:  ns,
			Deployment: name,
			Container:  r.URL.Query().Get("container"),
			Previous:   r.URL.Query().Has("previous"),
			Follow:     true,
		}
		req.TailLines = parseTailLines(r, defaultTail)
		ch, err := service.StreamLogs(r.Context(), req)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case ll, ok := <-ch:
				if !ok {
					return
				}
				if ll.Error != nil {
					fmt.Fprintf(w, "event: error\ndata: {\"error\":%q}\n\n", ll.Error.Error())
					if flusher != nil {
						flusher.Flush()
					}
					return
				}
				fmt.Fprintf(w, "event: log\ndata: {\"line\":%q}\n\n", ll.Line)
				if flusher != nil {
					flusher.Flush()
				}
			case <-ticker.C:
				fmt.Fprintf(w, ":keepalive\n\n")
				if flusher != nil {
					flusher.Flush()
				}
			case <-r.Context().Done():
				return
			}
		}
	}
}

func handleVerify(adminToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := tokenFromContext(r.Context())
		isAdmin := subtle.ConstantTimeCompare([]byte(rec.Token), []byte(adminToken)) == 1
		type scopeJSON struct {
			Namespace  string `json:"namespace"`
			Deployment string `json:"deployment"`
		}
		scopes := make([]scopeJSON, 0, len(rec.Scopes))
		for _, s := range rec.Scopes {
			scopes = append(scopes, scopeJSON{Namespace: s.Namespace, Deployment: s.Deployment})
		}
		writeJSON(w, http.StatusOK, map[string]any{"isAdmin": isAdmin, "scopes": scopes})
	}
}

func handleAdminListTokens(store *auth.TokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, store.List())
	}
}

func handleAdminCreateToken(store *auth.TokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req auth.TokenRecord
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		// force the server to mint id/token/createdAt
		req.ID, req.Token, req.CreatedAt = "", "", time.Time{}
		rec, err := store.Create(req)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, rec) // plaintext token returned once
	}
}

func handleAdminDeleteToken(store *auth.TokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := store.Delete(r.PathValue("id")); err != nil {
			writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func parseTailLines(r *http.Request, defaultTail int64) int64 {
	if t := r.URL.Query().Get("tailLines"); t != "" {
		var n int64
		if _, err := fmt.Sscanf(t, "%d", &n); err == nil {
			return n
		}
	}
	return defaultTail
}

func trimRequest(req ImageUpdateRequest) ImageUpdateRequest {
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.Deployment = strings.TrimSpace(req.Deployment)
	req.Container = strings.TrimSpace(req.Container)
	req.Image = strings.TrimSpace(req.Image)
	return req
}

func validateRequest(req ImageUpdateRequest) error {
	switch {
	case req.Namespace == "":
		return errors.New("namespace is required")
	case req.Deployment == "":
		return errors.New("deployment is required")
	case req.Container == "":
		return errors.New("container is required")
	case req.Image == "":
		return errors.New("image is required")
	default:
		return nil
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// writeServiceError maps a service-layer error to an HTTP status using the
// rollout sentinel errors. Unknown errors become 500.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, rollout.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, rollout.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, rollout.ErrAlreadyExists):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, rollout.ErrUnauthorized), errors.Is(err, rollout.ErrTokenExpired):
		writeError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, auth.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

func writeAudit(out io.Writer, req ImageUpdateRequest, result RolloutResult, err error) {
	if out == nil {
		return
	}

	event := map[string]any{
		"ts":         time.Now().UTC().Format(time.RFC3339),
		"event":      "deployment_image_update",
		"namespace":  req.Namespace,
		"deployment": req.Deployment,
		"container":  req.Container,
		"image":      req.Image,
		"dryRun":     req.DryRun,
		"wait":       req.Wait,
		"status":     "ok",
	}
	if err != nil {
		event["status"] = "error"
		event["error"] = err.Error()
	} else {
		event["oldImage"] = result.OldImage
		event["newImage"] = result.NewImage
		event["generation"] = result.Generation
		event["rolloutComplete"] = result.RolloutComplete
	}

	_ = json.NewEncoder(out).Encode(event)
}
