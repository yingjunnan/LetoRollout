# LetoRollout

LetoRollout is a small Go service that runs inside Kubernetes and patches a Deployment container image through an HTTP API.

## API

Health check:

```bash
curl http://localhost:8080/healthz
```

Update a Deployment image:

```bash
curl -X POST http://localhost:8080/api/v1/deployments/image \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer change-me' \
  -d '{
    "namespace": "default",
    "deployment": "nginx",
    "container": "nginx",
    "image": "nginx:1.27.0"
  }'
```

If `AUTH_TOKEN` is empty, the update API does not require the `Authorization` header.

## Configuration

| Environment variable | Default | Description |
| --- | --- | --- |
| `ADDR` | `:8080` | HTTP listen address. |
| `AUTH_TOKEN` | empty | Optional bearer token for update requests. |

The service uses Kubernetes in-cluster credentials through `rest.InClusterConfig()`.

## Test And Build

```bash
go test ./...
go build ./cmd/server
docker build -t letorollout:latest .
```

## Deploy

Create a token Secret if you want API auth:

```bash
kubectl create secret generic letorollout-auth \
  --from-literal=token='change-me' \
  -n default
```

Apply RBAC and the example Deployment:

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/deployment.yaml
```

The default RBAC grants the service cluster-wide Deployment access with only these verbs:

```text
apps/deployments: get, patch
```

The ServiceAccount and example Deployment run in `default`, while `ClusterRoleBinding` lets that ServiceAccount patch Deployments in any namespace. If you deploy LetoRollout into another namespace, update the ServiceAccount namespace in `deploy/rbac.yaml` and `deploy/deployment.yaml`.

## Response Example

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
