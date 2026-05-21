# LetoRollout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Go HTTP service that patches Kubernetes Deployment container images through an API.

**Architecture:** The service is split into config loading, HTTP API handling, and Kubernetes Deployment patching. It uses in-cluster Kubernetes config by default and keeps deployment permissions limited to `get` and `patch` on Deployments.

**Tech Stack:** Go 1.20, `net/http`, `k8s.io/client-go`, Kubernetes JSON Patch, Docker multi-stage build.

---

## File Structure

- `go.mod`: module and Kubernetes dependencies.
- `cmd/server/main.go`: process entrypoint and graceful HTTP server startup.
- `internal/config/config.go`: environment-driven configuration.
- `internal/httpapi/handler.go`: routes, auth, request validation, JSON responses.
- `internal/kube/deployment_image_updater.go`: Kubernetes client interface and Deployment image patching.
- `internal/config/config_test.go`: config unit tests.
- `internal/httpapi/handler_test.go`: API behavior tests.
- `internal/kube/deployment_image_updater_test.go`: fake clientset patch tests.
- `deploy/rbac.yaml`: namespace-scoped RBAC.
- `deploy/deployment.yaml`: example in-cluster Deployment manifest.
- `Dockerfile`: multi-stage container build.
- `README.md`: usage and deployment notes.

### Task 1: Module And Config

**Files:**
- Create: `go.mod`
- Create: `internal/config/config_test.go`
- Create: `internal/config/config.go`

- [ ] Write tests for default config and env overrides.
- [ ] Run `go test ./internal/config` and confirm it fails because implementation is missing.
- [ ] Implement `Load()` with `ADDR` defaulting to `:8080` and optional `AUTH_TOKEN`.
- [ ] Run `go test ./internal/config` and confirm it passes.

### Task 2: HTTP API

**Files:**
- Create: `internal/httpapi/handler_test.go`
- Create: `internal/httpapi/handler.go`

- [ ] Write tests for health check, method restrictions, auth failure, invalid request, and successful rollout response.
- [ ] Run `go test ./internal/httpapi` and confirm it fails because implementation is missing.
- [ ] Implement `RolloutService`, request/response structs, `NewHandler`, bearer auth, JSON helpers, and route handling.
- [ ] Run `go test ./internal/httpapi` and confirm it passes.

### Task 3: Kubernetes Deployment Patching

**Files:**
- Create: `internal/kube/deployment_image_updater_test.go`
- Create: `internal/kube/deployment_image_updater.go`

- [ ] Write tests using `kubernetes/fake` for replacing an existing container image and reporting missing containers.
- [ ] Run `go test ./internal/kube` and confirm it fails because implementation is missing.
- [ ] Implement in-cluster client creation and JSON Patch image replacement.
- [ ] Run `go test ./internal/kube` and confirm it passes.

### Task 4: Server Entrypoint And Manifests

**Files:**
- Create: `cmd/server/main.go`
- Create: `deploy/rbac.yaml`
- Create: `deploy/deployment.yaml`
- Create: `Dockerfile`
- Create: `README.md`

- [ ] Implement `main.go` to load config, create Kubernetes updater, register HTTP handler, and shut down gracefully.
- [ ] Add Dockerfile using Go 1.20 builder and a distroless/static runtime.
- [ ] Add namespace-scoped RBAC for `deployments` `get` and `patch`.
- [ ] Add example Deployment manifest with `AUTH_TOKEN` sourced from a Secret.
- [ ] Add README commands for testing, building, deploying, and calling the API.

### Task 5: Verification

**Files:**
- All project files.

- [ ] Run `go mod tidy`.
- [ ] Run `gofmt` on Go files.
- [ ] Run `go test ./...`.
- [ ] Run `go build ./cmd/server`.
- [ ] Review files for spec coverage and remove accidental placeholders.

## Self-Review

The plan covers every design requirement: API endpoints, in-cluster Kubernetes client, Deployment patching, optional bearer auth, RBAC, Dockerfile, example deployment, README, tests, and build verification. No deferred features are included in first scope.
