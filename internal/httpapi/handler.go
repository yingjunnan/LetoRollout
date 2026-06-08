package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"letorollout/internal/rollout"
)

var ErrNotFound = rollout.ErrNotFound
var ErrForbidden = rollout.ErrForbidden
var ErrAlreadyExists = rollout.ErrAlreadyExists

type ImageUpdateRequest = rollout.ImageUpdateRequest
type RolloutResult = rollout.RolloutResult
type DeploymentCreateRequest = rollout.DeploymentCreateRequest
type DeploymentCreateResult = rollout.DeploymentCreateResult

type RolloutService interface {
	CreateDeployment(ctx context.Context, req DeploymentCreateRequest) (DeploymentCreateResult, error)
	UpdateImage(ctx context.Context, req ImageUpdateRequest) (RolloutResult, error)
}

func NewHandler(service RolloutService, authToken string) http.Handler {
	return NewHandlerWithAuditWriter(service, authToken, os.Stdout)
}

func NewHandlerWithAuditWriter(service RolloutService, authToken string, audit io.Writer) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/api/v1/deployments", handleCreateDeployment(service, authToken, audit))
	mux.HandleFunc("/api/v1/deployments/image", handleUpdateImage(service, authToken, audit))
	return mux
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleCreateDeployment(service RolloutService, authToken string, audit io.Writer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if authToken != "" && r.Header.Get("Authorization") != "Bearer "+authToken {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req DeploymentCreateRequest
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json body")
			return
		}
		req = trimCreateRequest(req)
		if err := validateCreateRequest(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		result, err := service.CreateDeployment(r.Context(), req)
		writeCreateAudit(audit, req, result, err)
		if err != nil {
			if errors.Is(err, ErrAlreadyExists) {
				writeError(w, http.StatusConflict, err.Error())
				return
			}
			if errors.Is(err, ErrForbidden) {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, result)
	}
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

func trimCreateRequest(req DeploymentCreateRequest) DeploymentCreateRequest {
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.Name = strings.TrimSpace(req.Name)
	req.Image = strings.TrimSpace(req.Image)
	return req
}

func trimRequest(req ImageUpdateRequest) ImageUpdateRequest {
	req.Namespace = strings.TrimSpace(req.Namespace)
	req.Deployment = strings.TrimSpace(req.Deployment)
	req.Container = strings.TrimSpace(req.Container)
	req.Image = strings.TrimSpace(req.Image)
	return req
}

func validateCreateRequest(req DeploymentCreateRequest) error {
	switch {
	case req.Namespace == "":
		return errors.New("namespace is required")
	case req.Name == "":
		return errors.New("name is required")
	case req.Image == "":
		return errors.New("image is required")
	default:
		return nil
	}
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

func writeCreateAudit(out io.Writer, req DeploymentCreateRequest, result DeploymentCreateResult, err error) {
	if out == nil {
		return
	}

	event := map[string]any{
		"ts":        time.Now().UTC().Format(time.RFC3339),
		"event":     "deployment_create",
		"namespace": req.Namespace,
		"name":      req.Name,
		"image":     req.Image,
		"status":    "ok",
	}
	if err != nil {
		event["status"] = "error"
		event["error"] = err.Error()
	} else {
		event["generation"] = result.Generation
		event["replicas"] = result.Replicas
	}

	_ = json.NewEncoder(out).Encode(event)
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
