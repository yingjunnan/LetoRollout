# LetoRollout

Languages: [English](#english) | [中文](#中文)

## English

LetoRollout is a small Go service that runs inside Kubernetes and patches a Deployment container image through an HTTP API.

### Web Console

Open the built-in console at `/console/`.

It provides two workflows:

- Create a Deployment with image and environment variables.
- Update an existing Deployment image, with optional dry-run and wait.

The console is open access by default and relies on network boundaries rather
than a separate login.

For a local browser preview without Kubernetes, run the server with
`LOCAL_PREVIEW=1`.

```bash
LOCAL_PREVIEW=1 ADDR=:8081 go run ./cmd/server
```

For a local browser preview without Kubernetes, run the server with
`LOCAL_PREVIEW=1`. That mode serves the same console and API shape from an
in-memory backend so you can exercise the UI quickly.

### API

Health check:

```bash
curl http://localhost:8080/healthz
```

Create a Deployment:

```bash
curl -X POST http://localhost:8080/api/v1/deployments \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer change-me' \
  -d '{
    "namespace": "default",
    "name": "nginx",
    "image": "nginx:1.27.0",
    "env": [
      { "name": "APP_ENV", "value": "prod" },
      {
        "name": "DATABASE_URL",
        "secret": {
          "name": "nginx-secret",
          "key": "database-url"
        }
      }
    ]
  }'
```

The optional `env` array on create requests supports literal values and Secret
key references. Each item must set `name` and exactly one of `value` or
`secret`. Secret references use a Secret in the same namespace as the created
Deployment.

Update a Deployment image:

```bash
curl -X POST http://localhost:8080/api/v1/deployments/image \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer change-me' \
  -d '{
    "namespace": "default",
    "deployment": "nginx",
    "container": "nginx",
    "image": "nginx:1.27.0",
    "dryRun": false,
    "wait": true,
    "timeoutSeconds": 300
  }'
```

If `AUTH_TOKEN` is empty, the create and update APIs do not require the `Authorization` header.

Optional request fields:

| Field | Default | Description |
| --- | --- | --- |
| `dryRun` | `false` | Preview the old and new image without patching the Deployment. |
| `wait` | `false` | Wait until the Deployment reports all desired replicas updated and available. |
| `timeoutSeconds` | `300` | Rollout wait timeout when `wait` is true. |

### Configuration

| Environment variable | Default | Description |
| --- | --- | --- |
| `ADDR` | `:8080` | HTTP listen address. |
| `AUTH_TOKEN` | empty | Optional bearer token for update requests. |
| `ALLOWED_NAMESPACES` | empty | Optional comma-separated namespace allowlist, for example `dev,staging,prod`. Empty allows all namespaces permitted by RBAC. |
| `REQUIRED_DEPLOYMENT_LABEL` | empty | Optional `key=value` label required on target Deployments, for example `letorollout/enabled=true`. |
| `LOCAL_PREVIEW` | `0` | Set to `1` to run the in-memory console preview backend instead of Kubernetes. |

The service uses Kubernetes in-cluster credentials through `rest.InClusterConfig()`.

Each create or update attempt writes one JSON audit log line to stdout with the target, image, status, and error when present. Update audit logs also include the dry-run and wait flags.

### Test And Build

```bash
go test ./...
go build ./cmd/server
docker build -t letorollout:latest .
```

### Deploy

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
apps/deployments: get, patch, create
```

The ServiceAccount and example Deployment run in `default`, while `ClusterRoleBinding` lets that ServiceAccount create and patch Deployments in any namespace. If you deploy LetoRollout into another namespace, update the ServiceAccount namespace in `deploy/rbac.yaml` and `deploy/deployment.yaml`.

For an extra safety gate, label Deployments that LetoRollout may update and set `REQUIRED_DEPLOYMENT_LABEL`:

```bash
kubectl label deployment nginx letorollout/enabled=true -n default
```

### Response Example

```json
{
  "namespace": "default",
  "deployment": "nginx",
  "container": "nginx",
  "oldImage": "nginx:1.26.0",
  "newImage": "nginx:1.27.0",
  "generation": 2,
  "rolloutComplete": true
}
```

## 中文

LetoRollout 是一个运行在 Kubernetes 内部的小型 Go 服务，通过 HTTP API 更新 Deployment 的容器镜像。

### API

健康检查：

```bash
curl http://localhost:8080/healthz
```

更新 Deployment 镜像：

```bash
curl -X POST http://localhost:8080/api/v1/deployments/image \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer change-me' \
  -d '{
    "namespace": "default",
    "deployment": "nginx",
    "container": "nginx",
    "image": "nginx:1.27.0",
    "dryRun": false,
    "wait": true,
    "timeoutSeconds": 300
  }'
```

如果 `AUTH_TOKEN` 为空，更新 API 不需要 `Authorization` 请求头。

可选请求字段：

| 字段 | 默认值 | 描述 |
| --- | --- | --- |
| `dryRun` | `false` | 只预览旧镜像和新镜像，不实际 patch Deployment。 |
| `wait` | `false` | 等待 Deployment 报告所有期望副本已更新且可用。 |
| `timeoutSeconds` | `300` | `wait` 为 true 时的 rollout 等待超时时间。 |

### 配置

| 环境变量 | 默认值 | 描述 |
| --- | --- | --- |
| `ADDR` | `:8080` | HTTP 监听地址。 |
| `AUTH_TOKEN` | 空 | 更新请求的可选 Bearer token。 |
| `ALLOWED_NAMESPACES` | 空 | 可选的 namespace 白名单，使用英文逗号分隔，例如 `dev,staging,prod`。为空时允许 RBAC 授权范围内的所有 namespace。 |
| `REQUIRED_DEPLOYMENT_LABEL` | 空 | 目标 Deployment 必须具备的可选 `key=value` 标签，例如 `letorollout/enabled=true`。 |

服务通过 `rest.InClusterConfig()` 使用 Kubernetes 集群内凭证。

每次更新请求都会向 stdout 写入一行 JSON 审计日志，包含目标资源、镜像、dry-run、wait、状态，以及失败时的错误信息。

### 测试与构建

```bash
go test ./...
go build ./cmd/server
docker build -t letorollout:latest .
```

### 部署

如果需要 API 认证，创建 token Secret：

```bash
kubectl create secret generic letorollout-auth \
  --from-literal=token='change-me' \
  -n default
```

应用 RBAC 和示例 Deployment：

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/deployment.yaml
```

默认 RBAC 授予服务集群范围的 Deployment 访问权限，仅包含以下操作：

```text
apps/deployments: get, patch
```

ServiceAccount 和示例 Deployment 在 `default` 命名空间中运行，`ClusterRoleBinding` 允许该 ServiceAccount 在任何命名空间中更新 Deployment。如果将 LetoRollout 部署到其他命名空间，请更新 `deploy/rbac.yaml` 和 `deploy/deployment.yaml` 中的 ServiceAccount 命名空间。

如果想加一道额外保护，可以给允许 LetoRollout 更新的 Deployment 打标签，并设置 `REQUIRED_DEPLOYMENT_LABEL`：

```bash
kubectl label deployment nginx letorollout/enabled=true -n default
```

### 响应示例

```json
{
  "namespace": "default",
  "deployment": "nginx",
  "container": "nginx",
  "oldImage": "nginx:1.26.0",
  "newImage": "nginx:1.27.0",
  "generation": 2,
  "rolloutComplete": true
}
```
