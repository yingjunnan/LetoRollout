package httpapi

import (
	"context"
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

	"letorollout/internal/rollout"
)

//go:embed static/*
var staticFS embed.FS

var ErrNotFound = rollout.ErrNotFound
var ErrForbidden = rollout.ErrForbidden
var ErrAlreadyExists = rollout.ErrAlreadyExists

type ImageUpdateRequest = rollout.ImageUpdateRequest
type RolloutResult = rollout.RolloutResult

type RolloutService interface {
	UpdateImage(ctx context.Context, req ImageUpdateRequest) (RolloutResult, error)
}

func NewHandler(service RolloutService, authToken string) http.Handler {
	return NewHandlerWithAuditWriter(service, authToken, os.Stdout)
}

func NewHandlerWithAuditWriter(service RolloutService, authToken string, audit io.Writer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc("/console", handleConsoleRedirect)
	mux.Handle("/console/", consoleHandler())
	mux.HandleFunc("/api/v1/deployments/image", handleUpdateImage(service, authToken, audit))
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

func handleUpdateImage(service RolloutService, authToken string, audit io.Writer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if authToken != "" && r.Header.Get("Authorization") != "Bearer "+authToken {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req ImageUpdateRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		req = trimRequest(req)
		if err := validateRequest(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		result, err := service.UpdateImage(r.Context(), req)
		writeAudit(audit, req, result, err)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			if errors.Is(err, ErrForbidden) {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, result)
	}
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
