# LetoRollout

Languages: [English](#english) | [中文](#中文)

## English

LetoRollout is a small Go service that runs inside Kubernetes and gives
end users a controlled console to **update a Deployment container image** and
**view its Pod logs**. Access is gated by scoped tokens; an admin mints those
tokens with namespace- or deployment-level permission and an optional expiry.

### Web Console

Open the built-in console at `/console/`.

For an end user it exposes two workflows:

- Update an authorized Deployment's image, with optional dry-run and wait.
- View an authorized Deployment's Pod logs, one-shot or live follow.

> The current `internal/httpapi/static/` console is the original vanilla-JS
> page. A React rewrite is planned and tracks the API below.

For a local browser preview without Kubernetes, run the server with
`LOCAL_PREVIEW=1`. That mode serves the same API from an in-memory backend so
you can exercise the endpoints quickly.

```bash
LOCAL_PREVIEW=1 ADMIN_TOKEN=adm TOKENS_PATH=./tokens.json go run ./cmd/server
```

### Project Structure

```text
letorollout/
├── cmd/server/main.go            # Entry point: load config, wire handler, graceful shutdown
├── internal/
│   ├── config/                   # Environment-based configuration
│   ├── rollout/                  # Domain types & sentinel errors (shared contract)
│   ├── auth/                     # Scoped-token store (JSON file) + middleware
│   ├── httpapi/                  # HTTP layer: routing, validation, audit, embedded console
│   │   └── static/               # Embedded frontend (//go:embed)
│   └── kube/                     # Kubernetes client: image update + read + log streaming
├── deploy/                       # RBAC, example Deployment, data PVC
├── docs/superpowers/             # Design specs & implementation plans
├── Dockerfile
└── go.mod
```

### Architecture

LetoRollout is layered around a shared domain contract so the HTTP layer never depends directly on Kubernetes:

- `rollout` owns the request/result types and sentinel errors (`ErrNotFound`, `ErrForbidden`, `ErrAlreadyExists`, `ErrUnauthorized`, `ErrTokenExpired`). Both `httpapi` and `kube` depend on it, keeping error semantics consistent and mapping cleanly to HTTP status codes (404 / 403 / 409 / 401).
- `httpapi` depends on small service interfaces — `ImageUpdater`, `DeploymentReader`, `LogStreamer` — rather than Kubernetes. That makes the HTTP layer unit-testable with a fake service and lets `LOCAL_PREVIEW=1` swap in an in-memory `PreviewService` so the console works without a cluster.
- `kube.DeploymentImageUpdater` is the production implementation. It uses in-cluster credentials and applies two safety gates before any mutation: a namespace allowlist and a required Deployment label.
- `auth.TokenStore` persists scoped user tokens as a JSON file and verifies each request; `authMiddleware` enforces that the path's `{ns}`/`{name}` falls inside the token's scope.
- Every image update writes one JSON audit line to stdout.

Request flow: HTTP → auth middleware (verify token + scope) → trim & validate → service (kube or preview) → audit → JSON response.

When extending the service, keep new request/result types and sentinel errors in `rollout`, implement behavior behind the service interfaces in `kube`, and expose it through `httpapi`. Frontend assets under `internal/httpapi/static/` are embedded via `//go:embed`, so a plain `go build` picks up changes with no separate build step.

### API

All `/api/v1/*` routes except `/healthz` require `Authorization: Bearer <token>`.
User routes accept a scoped user token; admin routes require the `ADMIN_TOKEN`.
Because `EventSource` cannot set headers, the SSE log stream also accepts
`?token=<token>`.

Health check (no auth):

```bash
curl http://localhost:8080/healthz
```

Verify a token (reports whether it is admin and its scopes):

```bash
curl -X POST http://localhost:8080/api/v1/auth/verify \
  -H 'Authorization: Bearer <token>'
```

List Deployments in a namespace (must be in the token's scope):

```bash
curl http://localhost:8080/api/v1/namespaces/default/deployments \
  -H 'Authorization: Bearer <token>'
```

Get a Deployment (includes its containers, so the UI can auto-fill the container):

```bash
curl http://localhost:8080/api/v1/namespaces/default/deployments/nginx \
  -H 'Authorization: Bearer <token>'
```

Update a Deployment image (namespace/deployment come from the path):

```bash
curl -X POST http://localhost:8080/api/v1/namespaces/default/deployments/nginx/image \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <token>' \
  -d '{
    "container": "nginx",
    "image": "nginx:1.27.0",
    "dryRun": false,
    "wait": true,
    "timeoutSeconds": 300
  }'
```

View logs (one-shot, `text/plain`):

```bash
curl 'http://localhost:8080/api/v1/namespaces/default/deployments/nginx/logs?tailLines=500&container=nginx' \
  -H 'Authorization: Bearer <token>'
```

Live-follow logs (SSE, `text/event-stream`). Use the query param for the token
since browsers' `EventSource` cannot set headers:

```bash
curl -N 'http://localhost:8080/api/v1/namespaces/default/deployments/nginx/logs/stream?token=<token>&container=nginx'
```

Optional image-update request fields:

| Field | Default | Description |
| --- | --- | --- |
| `dryRun` | `false` | Preview the old and new image without patching the Deployment. |
| `wait` | `false` | Wait until the Deployment reports all desired replicas updated and available. |
| `timeoutSeconds` | `300` | Rollout wait timeout when `wait` is true. |

Optional log query parameters:

| Param | Default | Description |
| --- | --- | --- |
| `container` | first container | Container to read logs from. |
| `tailLines` | `LOG_TAIL_LINES` | Number of recent lines (one-shot) or initial tail (follow). |
| `previous` | absent | If present, read the previous container instance's logs. |

Admin token management (requires `ADMIN_TOKEN`):

```bash
# create a scoped user token (plaintext returned once)
curl -X POST http://localhost:8080/api/v1/admin/tokens \
  -H 'Authorization: Bearer <ADMIN_TOKEN>' \
  -H 'Content-Type: application/json' \
  -d '{"label":"alice","scopes":[{"namespace":"dev"},{"namespace":"prod","deployment":"api"}],"expiresAt":"2026-12-31T00:00:00Z"}'

# list tokens (plaintext never exposed)
curl http://localhost:8080/api/v1/admin/tokens \
  -H 'Authorization: Bearer <ADMIN_TOKEN>'

# delete a token by id
curl -X DELETE http://localhost:8080/api/v1/admin/tokens/<id> \
  -H 'Authorization: Bearer <ADMIN_TOKEN>'
```

A token scope is `{namespace, deployment?}`: `deployment` empty authorizes the
whole namespace, otherwise only that single Deployment. Each request is
additionally checked against the token's scope; out-of-scope requests return
403.

### Configuration

| Environment variable | Default | Description |
| --- | --- | --- |
| `ADDR` | `:8080` | HTTP listen address. |
| `ADMIN_TOKEN` | empty | Admin bearer token for minting/managing user tokens. When empty, admin API returns 503. |
| `TOKENS_PATH` | `/data/tokens.json` | Path to the scoped-token store file (mounted as a PVC in the manifests). |
| `LOG_TAIL_LINES` | `500` | Default number of log lines when `tailLines` is not given. |
| `ALLOWED_NAMESPACES` | empty | Optional comma-separated namespace allowlist, for example `dev,staging,prod`. Empty allows all namespaces permitted by RBAC. |
| `REQUIRED_DEPLOYMENT_LABEL` | empty | Optional `key=value` label required on target Deployments, for example `letorollout/enabled=true`. |
| `LOCAL_PREVIEW` | `0` | Set to `1` to run the in-memory console preview backend instead of Kubernetes. |

The service uses Kubernetes in-cluster credentials through `rest.InClusterConfig()`.

Each image update writes one JSON audit log line to stdout with the target, image, status, and error when present, plus the dry-run and wait flags.

### Test And Build

```bash
go test ./...
go build ./cmd/server
docker build -t letorollout:latest .
```

### Deploy

Create the admin token Secret (optional — without it the admin API is disabled):

```bash
kubectl create secret generic letorollout-admin-auth \
  --from-literal=admin-token='change-me' \
  -n default
```

Apply RBAC, the data PVC, and the example Deployment:

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/pvc.yaml
kubectl apply -f deploy/deployment.yaml
```

The default RBAC grants the service cluster-wide access with these verbs:

```text
apps/deployments: get, list, patch, create
""/pods/log:      get
```

`list` powers the read-only Deployment listing; `pods/log` powers log viewing.
The data PVC (`letorollout-data`, 1Gi) is mounted at `/data` so the token store
survives Pod restarts. The ServiceAccount and example Deployment run in
`default`, while `ClusterRoleBinding` lets that ServiceAccount act in any
namespace. If you deploy LetoRollout into another namespace, update the
ServiceAccount namespace in `deploy/rbac.yaml` and `deploy/deployment.yaml`.

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
  "dryRun": false,
  "rolloutComplete": true
}
```

## 中文

LetoRollout 是一个运行在 Kubernetes 内部的小型 Go 服务，为最终用户提供一个受控控制台，用来**更新 Deployment 容器镜像**和**查看其 Pod 日志**。访问由带 scope 的 token 闸门控制；管理员可签发带 namespace 级或 deployment 级权限、可选过期时间的普通用户 token。

### Web 控制台

在内置控制台 `/console/` 进入。

对最终用户暴露两个工作流：

- 更新授权 Deployment 的镜像，可选 dry-run 与 wait。
- 查看授权 Deployment 的 Pod 日志，支持一次性拉取或实时跟随。

> 当前 `internal/httpapi/static/` 仍是原始的原生 JS 页面。React 重写已规划，并追踪下文的 API。

不连接 Kubernetes 时可用 `LOCAL_PREVIEW=1` 本地预览，该模式用内存后端提供相同 API，便于快速验证接口。

```bash
LOCAL_PREVIEW=1 ADMIN_TOKEN=adm TOKENS_PATH=./tokens.json go run ./cmd/server
```

### 项目结构

```text
letorollout/
├── cmd/server/main.go            # 入口：加载配置、组装 handler、优雅关闭
├── internal/
│   ├── config/                   # 基于环境变量的配置
│   ├── rollout/                  # 领域类型与哨兵错误（共享契约）
│   ├── auth/                     # 带 scope 的 token 存储（JSON 文件）+ 中间件
│   ├── httpapi/                  # HTTP 层：路由、校验、审计、内嵌控制台
│   │   └── static/               # 内嵌前端（//go:embed）
│   └── kube/                     # Kubernetes 客户端：镜像更新 + 只读 + 日志流
├── deploy/                       # RBAC、示例 Deployment、数据 PVC
├── docs/superpowers/             # 设计文档与实现计划
├── Dockerfile
└── go.mod
```

### 架构

LetoRollout 围绕一份共享的领域契约分层设计，HTTP 层不直接依赖 Kubernetes：

- `rollout` 持有请求/响应类型与哨兵错误（`ErrNotFound`、`ErrForbidden`、`ErrAlreadyExists`、`ErrUnauthorized`、`ErrTokenExpired`）。`httpapi` 与 `kube` 都依赖它，使错误语义保持一致，并能干净地映射到 HTTP 状态码（404 / 403 / 409 / 401）。
- `httpapi` 依赖小而专注的服务接口——`ImageUpdater`、`DeploymentReader`、`LogStreamer`——而非 Kubernetes。这让 HTTP 层可以用 fake service 做单元测试，也让 `LOCAL_PREVIEW=1` 能用内存版 `PreviewService` 替换实现，使控制台在没有集群时也能运行。
- `kube.DeploymentImageUpdater` 是生产实现。它使用集群内凭证，并在任何变更前施加两道安全闸门：namespace 白名单和必需的 Deployment 标签。
- `auth.TokenStore` 把带 scope 的用户 token 持久化为 JSON 文件并校验每个请求；`authMiddleware` 确保路径中的 `{ns}`/`{name}` 落在 token 的 scope 内。
- 每次镜像更新都会向 stdout 写入一行 JSON 审计日志。

请求流程：HTTP → 鉴权中间件（校验 token + scope）→ 裁剪与校验 → service（kube 或 preview）→ 审计 → JSON 响应。

扩展服务时，请把新的请求/响应类型与哨兵错误放在 `rollout`，在 `kube` 中通过服务接口实现行为，并在 `httpapi` 暴露出来。`internal/httpapi/static/` 下的前端资源通过 `//go:embed` 内嵌，普通 `go build` 即可打包变更，无需单独的构建步骤。

### API

除 `/healthz` 外，所有 `/api/v1/*` 路由都需要 `Authorization: Bearer <token>`。用户路由接受带 scope 的用户 token；管理路由需要 `ADMIN_TOKEN`。由于 `EventSource` 无法设置请求头，SSE 日志流也接受 `?token=<token>`。

健康检查（无需鉴权）：

```bash
curl http://localhost:8080/healthz
```

校验 token（返回是否为管理员及其 scope）：

```bash
curl -X POST http://localhost:8080/api/v1/auth/verify \
  -H 'Authorization: Bearer <token>'
```

列出某 namespace 下的 Deployment（必须在 token 的 scope 内）：

```bash
curl http://localhost:8080/api/v1/namespaces/default/deployments \
  -H 'Authorization: Bearer <token>'
```

获取单个 Deployment（含容器列表，便于前端自动填入 container）：

```bash
curl http://localhost:8080/api/v1/namespaces/default/deployments/nginx \
  -H 'Authorization: Bearer <token>'
```

更新 Deployment 镜像（namespace/deployment 来自路径）：

```bash
curl -X POST http://localhost:8080/api/v1/namespaces/default/deployments/nginx/image \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <token>' \
  -d '{
    "container": "nginx",
    "image": "nginx:1.27.0",
    "dryRun": false,
    "wait": true,
    "timeoutSeconds": 300
  }'
```

查看日志（一次性拉取，`text/plain`）：

```bash
curl 'http://localhost:8080/api/v1/namespaces/default/deployments/nginx/logs?tailLines=500&container=nginx' \
  -H 'Authorization: Bearer <token>'
```

实时跟随日志（SSE，`text/event-stream`）。由于浏览器 `EventSource` 无法设置请求头，token 走 query 参数：

```bash
curl -N 'http://localhost:8080/api/v1/namespaces/default/deployments/nginx/logs/stream?token=<token>&container=nginx'
```

镜像更新可选请求字段：

| 字段 | 默认值 | 描述 |
| --- | --- | --- |
| `dryRun` | `false` | 只预览旧镜像和新镜像，不实际 patch Deployment。 |
| `wait` | `false` | 等待 Deployment 报告所有期望副本已更新且可用。 |
| `timeoutSeconds` | `300` | `wait` 为 true 时的 rollout 等待超时时间。 |

日志可选 query 参数：

| 参数 | 默认值 | 描述 |
| --- | --- | --- |
| `container` | 第一个容器 | 读取哪个容器的日志。 |
| `tailLines` | `LOG_TAIL_LINES` | 一次性拉取的行数，或实时跟随的初始尾行数。 |
| `previous` | 不带 | 带上时读取上一个容器实例的日志。 |

管理 token（需要 `ADMIN_TOKEN`）：

```bash
# 创建带 scope 的用户 token（明文仅返回一次）
curl -X POST http://localhost:8080/api/v1/admin/tokens \
  -H 'Authorization: Bearer <ADMIN_TOKEN>' \
  -H 'Content-Type: application/json' \
  -d '{"label":"alice","scopes":[{"namespace":"dev"},{"namespace":"prod","deployment":"api"}],"expiresAt":"2026-12-31T00:00:00Z"}'

# 列出 token（永不返回明文）
curl http://localhost:8080/api/v1/admin/tokens \
  -H 'Authorization: Bearer <ADMIN_TOKEN>'

# 按 id 删除 token
curl -X DELETE http://localhost:8080/api/v1/admin/tokens/<id> \
  -H 'Authorization: Bearer <ADMIN_TOKEN>'
```

scope 结构为 `{namespace, deployment?}`：`deployment` 为空表示授权整个 namespace，否则仅授权该 Deployment。每个请求还会额外校验是否在 token 的 scope 内，越权返回 403。

### 配置

| 环境变量 | 默认值 | 描述 |
| --- | --- | --- |
| `ADDR` | `:8080` | HTTP 监听地址。 |
| `ADMIN_TOKEN` | 空 | 管理员 Bearer token，用于签发/管理用户 token。为空时管理接口返回 503。 |
| `TOKENS_PATH` | `/data/tokens.json` | 带 scope 的 token 存储文件路径（在清单中作为 PVC 挂载）。 |
| `LOG_TAIL_LINES` | `500` | 未指定 `tailLines` 时的默认日志行数。 |
| `ALLOWED_NAMESPACES` | 空 | 可选的 namespace 白名单，使用英文逗号分隔，例如 `dev,staging,prod`。为空时允许 RBAC 授权范围内的所有 namespace。 |
| `REQUIRED_DEPLOYMENT_LABEL` | 空 | 目标 Deployment 必须具备的可选 `key=value` 标签，例如 `letorollout/enabled=true`。 |
| `LOCAL_PREVIEW` | `0` | 设为 `1` 时用内存版控制台后端替代 Kubernetes。 |

服务通过 `rest.InClusterConfig()` 使用 Kubernetes 集群内凭证。

每次镜像更新都会向 stdout 写入一行 JSON 审计日志，包含目标资源、镜像、状态、失败时的错误信息，以及 dry-run 与 wait 标志。

### 测试与构建

```bash
go test ./...
go build ./cmd/server
docker build -t letorollout:latest .
```

### 部署

创建管理员 token Secret（可选——不配置时管理接口被禁用）：

```bash
kubectl create secret generic letorollout-admin-auth \
  --from-literal=admin-token='change-me' \
  -n default
```

应用 RBAC、数据 PVC 与示例 Deployment：

```bash
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/pvc.yaml
kubectl apply -f deploy/deployment.yaml
```

默认 RBAC 授予服务集群范围访问权限，仅包含以下操作：

```text
apps/deployments: get, list, patch, create
""/pods/log:      get
```

`list` 支撑只读 Deployment 列表；`pods/log` 支撑日志查看。数据 PVC（`letorollout-data`，1Gi）挂载到 `/data`，保证 token 存储在 Pod 重启后不丢失。ServiceAccount 与示例 Deployment 运行在 `default`，`ClusterRoleBinding` 允许该 ServiceAccount 在任何 namespace 中操作。如果将 LetoRollout 部署到其他命名空间，请更新 `deploy/rbac.yaml` 和 `deploy/deployment.yaml` 中的 ServiceAccount 命名空间。

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
  "dryRun": false,
  "rolloutComplete": true
}
```
