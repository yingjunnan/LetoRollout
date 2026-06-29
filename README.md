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

### Project Structure

```text
letorollout/
├── cmd/server/main.go            # Entry point: load config, wire handler, graceful shutdown
├── internal/
│   ├── config/                  # Environment-based configuration
│   ├── rollout/                 # Domain types & sentinel errors (shared contract)
│   ├── httpapi/                 # HTTP layer: routing, validation, audit, embedded console
│   │   └── static/              # Embedded frontend (vanilla JS, no build step)
│   └── kube/                    # Kubernetes client implementation of RolloutService
├── deploy/                      # RBAC + example Deployment manifests
├── docs/superpowers/            # Design specs & implementation plans
├── Dockerfile
└── go.mod
```

### Architecture

LetoRollout is layered around a shared domain contract so the HTTP layer never depends directly on Kubernetes:

- `rollout` owns the request/result types and sentinel errors (`ErrNotFound`, `ErrForbidden`, `ErrAlreadyExists`). Both `httpapi` and `kube` depend on it, keeping error semantics consistent and mapping cleanly to HTTP status codes (404 / 403 / 409).
- `httpapi` depends on a `RolloutService` interface rather than Kubernetes. That makes the HTTP layer unit-testable with a fake service and lets `LOCAL_PREVIEW=1` swap in an in-memory `PreviewService` so the console works without a cluster.
- `kube.DeploymentImageUpdater` is the production implementation. It uses in-cluster credentials and applies two safety gates before any mutation: a namespace allowlist and a required Deployment label.
- Every create or update writes one JSON audit line to stdout.

Request flow: HTTP → trim & validate → service (kube or preview) → audit → JSON response.

When extending the service, keep new request/result types and sentinel errors in `rollout`, implement behavior behind the `RolloutService` interface in `kube`, and expose it through `httpapi`. Frontend assets under `internal/httpapi/static/` are embedded via `//go:embed`, so a plain `go build` picks up changes with no separate build step.

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

LetoRollout 是一个运行在 Kubernetes 内部的小型 Go 服务，通过 HTTP API 创建或更新 Deployment 的容器镜像。

### 项目结构

```text
letorollout/
├── cmd/server/main.go            # 入口：加载配置、组装 handler、优雅关闭
├── internal/
│   ├── config/                   # 基于环境变量的配置
│   ├── rollout/                  # 领域类型与哨兵错误（共享契约）
│   ├── httpapi/                  # HTTP 层：路由、校验、审计、内嵌控制台
│   │   └── static/               # 内嵌前端（原生 JS，无需构建步骤）
│   └── kube/                     # RolloutService 的 Kubernetes 客户端实现
├── deploy/                       # RBAC 与示例 Deployment 清单
├── docs/superpowers/             # 设计文档与实现计划
├── Dockerfile
└── go.mod
```

### 架构

LetoRollout 围绕一份共享的领域契约分层设计，HTTP 层不直接依赖 Kubernetes：

- `rollout` 持有请求/响应类型与哨兵错误（`ErrNotFound`、`ErrForbidden`、`ErrAlreadyExists`）。`httpapi` 与 `kube` 都依赖它，使错误语义保持一致，并能干净地映射到 HTTP 状态码（404 / 403 / 409）。
- `httpapi` 依赖 `RolloutService` 接口而非 Kubernetes。这让 HTTP 层可以用 fake service 做单元测试，也让 `LOCAL_PREVIEW=1` 能用内存版 `PreviewService` 替换实现，使控制台在没有集群时也能运行。
- `kube.DeploymentImageUpdater` 是生产实现。它使用集群内凭证，并在任何变更前施加两道安全闸门：namespace 白名单和必需的 Deployment 标签。
- 每次创建或更新都会向 stdout 写入一行 JSON 审计日志。

请求流程：HTTP → 裁剪与校验 → service（kube 或 preview）→ 审计 → JSON 响应。

扩展服务时，请把新的请求/响应类型与哨兵错误放在 `rollout`，在 `kube` 中通过 `RolloutService` 接口实现行为，并在 `httpapi` 暴露出来。`internal/httpapi/static/` 下的前端资源通过 `//go:embed` 内嵌，普通 `go build` 即可打包变更，无需单独的构建步骤。

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
