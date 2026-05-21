# LetoRollout Design

## Goal

Build a small Go backend service that runs inside a Kubernetes cluster and updates a Deployment image when called through a simple HTTP API.

## Scope

First version supports direct API-triggered rollouts only. It does not include webhook parsing, persistent release history, async queues, multi-cluster support, or image registry polling.

## API

`GET /healthz` returns service health.

`POST /api/v1/deployments/image` updates one container image in one Deployment.

Request body:

```json
{
  "namespace": "default",
  "deployment": "nginx",
  "container": "nginx",
  "image": "nginx:1.27.0"
}
```

Successful response:

```json
{
  "namespace": "default",
  "deployment": "nginx",
  "container": "nginx",
  "oldImage": "nginx:1.26.0",
  "newImage": "nginx:1.27.0",
  "generation": 2
}
```

## Architecture

The service has three small layers:

- `internal/config`: reads server address and optional bearer token from environment variables.
- `internal/httpapi`: owns HTTP routing, JSON validation, auth, and response formatting.
- `internal/kube`: owns Kubernetes client creation and Deployment image patching.

The server defaults to `rest.InClusterConfig()` for Kubernetes access. A kubeconfig fallback is not part of the first version because the target deployment mode is in-cluster.

## Kubernetes Update Strategy

The service reads the target Deployment, finds the requested container by name, records the old image, then applies a JSON Patch replacing:

```text
/spec/template/spec/containers/<index>/image
```

Patching the pod template image causes Kubernetes Deployment controller to create a new ReplicaSet and roll out the new image according to the Deployment's existing rollout strategy.

## Security

If `AUTH_TOKEN` is set, requests to the update API must include:

```text
Authorization: Bearer <AUTH_TOKEN>
```

If `AUTH_TOKEN` is empty, the update API allows requests. This keeps local and test usage simple, while production manifests should set the token from a Secret.

The ServiceAccount needs only Deployment `get` and `patch` verbs. RBAC is namespace-scoped by default through a Role and RoleBinding.

## Error Handling

The API returns JSON errors with HTTP status codes:

- `400` for invalid JSON or missing fields.
- `401` for missing or invalid bearer token.
- `404` when the Deployment or container does not exist.
- `405` for unsupported methods.
- `500` for Kubernetes API or patch failures.

## Deliverables

- Go HTTP service source code.
- Unit tests for config, API handler behavior, and Kubernetes patch logic.
- `Dockerfile` for a small static Linux image.
- `deploy/rbac.yaml` with ServiceAccount, Role, and RoleBinding.
- `deploy/deployment.yaml` example.
- `README.md` with build, deploy, and curl usage.

## Verification

Run:

```bash
go test ./...
go build ./cmd/server
```

This validates the Go code compiles and the core behaviors pass unit tests.
