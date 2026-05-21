package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"letorollout/internal/rollout"
)

var ErrNotFound = rollout.ErrNotFound

type ImageUpdateRequest = rollout.ImageUpdateRequest
type RolloutResult = rollout.RolloutResult

type RolloutService interface {
	UpdateImage(ctx context.Context, req ImageUpdateRequest) (RolloutResult, error)
}

func NewHandler(service RolloutService, authToken string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthz)
	mux.HandleFunc("/api/v1/deployments/image", handleUpdateImage(service, authToken))
	return mux
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func handleUpdateImage(service RolloutService, authToken string) http.HandlerFunc {
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
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				writeError(w, http.StatusNotFound, err.Error())
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
