# React Console + Token Auth Design

## Goal

Rewrite the LetoRollout console in React, and reposition it from a
developer-facing deployment tool into a controlled console for end users.

The console does exactly two things for an end user (entered via a scoped
token):

- Update the image of a Deployment the user is authorized to access.
- View Pod logs of an authorized Deployment (one-shot pull + optional live
  follow).

Create Deployment is removed entirely. A separate admin menu (gated by
`ADMIN_TOKEN`) lets an operator mint scoped user tokens with expiry.

## Scope

### Removed
- Create Deployment workflow: API handler, validation, audit, types
  (`DeploymentCreateRequest`/`DeploymentCreateResult`/`DeploymentEnvVar`/
  `DeploymentEnvSecret`), `kubeEnvVars`, `deploymentSelectorID`,
  `int32ValuePtr`, and the old vanilla-JS frontend
  (`console.html`/`app.js`/`app.css`).
- `AUTH_TOKEN` mechanism (replaced by the TokenStore + `ADMIN_TOKEN`).

### Kept
- `UpdateImage` full chain, `rolloutComplete`, `waitForRollout`,
  namespace allowlist + required label safety gates, stdout JSON audit for
  image updates only.

### Added
- Read-only Deployment APIs (list, get detail) — `list` is new RBAC;
  `get` already present.
- Pod logs API (one-shot + SSE follow) — requires new `pods/log:get` RBAC.
- Token store (JSON file) + auth middleware (user + admin).
- React + Vite + Tailwind frontend embedded via `//go:embed`.
- Token management admin API.

## Architecture

Layered around the shared `rollout` domain contract, unchanged in spirit:
`httpapi` depends on small interfaces, never on Kubernetes directly.

### Package layout

```
internal/
├── config/          # + ADMIN_TOKEN, TOKENS_PATH, LOG_TAIL_LINES
├── rollout/         # types + sentinel errors (Create types removed)
├── auth/            # NEW: TokenStore (JSON file + mutex), scope checks, middleware
├── httpapi/         # routing/handlers; + read-only, logs, admin endpoints
├── kube/            # DeploymentImageUpdater implements Reader + LogStreamer too
│   └── static/      # Vite build output (//go:embed)
```

### Interfaces (read/write separated — Approach 1)

```go
// Update image (kept)
type ImageUpdater interface {
    UpdateImage(ctx context.Context, req ImageUpdateRequest) (RolloutResult, error)
}

// Read-only (new; `get` RBAC present, `list` newly added)
type DeploymentReader interface {
    ListDeployments(ctx context.Context, namespace string) ([]DeploymentSummary, error)
    GetDeployment(ctx context.Context, namespace, name string) (DeploymentDetail, error)
}

// Logs (new, RBAC pods/log:get added)
type LogStreamer interface {
    StreamLogs(ctx context.Context, req LogRequest) (<-chan LogLine, error)
}
```

`DeploymentSummary` = `{name, image, containers[], readyReplicas, replicas}`.
`DeploymentDetail` adds the container list (name + image) so the UI can
auto-fill the container field after selecting a Deployment.

`kube.DeploymentImageUpdater` implements all three; `httpapi.PreviewService`
implements all three against in-memory state so `LOCAL_PREVIEW=1` exercises
the full console without a cluster.

## Token Model

```go
package auth

type TokenScope struct {
    Namespace  string `json:"namespace"`
    Deployment string `json:"deployment,omitempty"` // empty = whole namespace
}

type TokenRecord struct {
    ID        string        `json:"id"`        // UUID; front end deletes by id
    Token     string        `json:"token"`     // bearer value; random 32-byte hex
    Label     string        `json:"label"`     // optional note, e.g. "alice-prod"
    Scopes    []TokenScope  `json:"scopes"`
    ExpiresAt *time.Time    `json:"expiresAt,omitempty"` // nil = never expires
    CreatedAt time.Time     `json:"createdAt"`
}
```

Stored as `tokens.json`:
```json
{ "tokens": [ { "id": "...", "token": "...", "label": "...", "scopes": [...], "expiresAt": null, "createdAt": "..." } ] }
```

### TokenStore (`internal/auth/store.go`)
- `LoadStore(path)`: reads file on startup; missing file = empty store.
- `Verify(token) (*TokenRecord, error)`: returns matching record or
  `ErrUnauthorized` / `ErrTokenExpired`.
- `(*TokenRecord).Allows(namespace, deployment) bool`: scope membership.
- `Create / List / Delete`: mutate in-memory + atomic write-back
  (`os.WriteFile` to temp file, `os.Rename`), guarded by a `sync.RWMutex`.
- `List()` never returns the `Token` plaintext — only id/label/scopes/
  expiresAt; admins delete by id.
- Token comparison uses `subtle.ConstantTimeCompare`.

## HTTP API

| Method | Path | Auth | Notes |
| --- | --- | --- | --- |
| GET | `/healthz` | none | kept |
| POST | `/api/v1/auth/verify` | bearer | returns scope summary + `isAdmin` flag |
| GET | `/api/v1/namespaces/{ns}/deployments` | user | list within token scope |
| GET | `/api/v1/namespaces/{ns}/deployments/{name}` | user | detail incl. containers |
| POST | `/api/v1/namespaces/{ns}/deployments/{name}/image` | user | update image (dryRun/wait/timeout) |
| GET | `/api/v1/namespaces/{ns}/deployments/{name}/logs` | user | one-shot logs (`text/plain`) |
| GET | `/api/v1/namespaces/{ns}/deployments/{name}/logs/stream` | user | SSE follow (`text/event-stream`) |
| GET | `/api/v1/admin/tokens` | admin | list (no plaintext) |
| POST | `/api/v1/admin/tokens` | admin | create (scope + expiry); returns plaintext once |
| DELETE | `/api/v1/admin/tokens/{id}` | admin | delete |

### Auth flow
- Middleware reads `Authorization: Bearer <t>` (fallback to `?token=` for
  SSE, since `EventSource` cannot set headers).
- User middleware: `TokenStore.Verify` → on fail 401; on success the record
  is placed in `r.Context()`. Each handler additionally checks the path
  `{ns}`/`{name}` against `record.Allows`; mismatch → 403.
- Admin middleware: compares against `ADMIN_TOKEN`; if `ADMIN_TOKEN` is
  empty, admin endpoints return 503.

### Error mapping
Sentinel errors in `rollout`: `ErrNotFound`→404, `ErrForbidden`→403,
`ErrAlreadyExists`→409. New: `ErrUnauthorized`→401, `ErrTokenExpired`→401.

## Logs

### One-shot — `GET .../logs`
Query params: `container` (optional, default first), `tailLines`
(optional, default `LOG_TAIL_LINES`), `previous` (optional).
Returns `text/plain` body.

### Live follow — `GET .../logs/stream` (SSE)
Same query params. `event:log data:{"ts":"...","line":"..."}`; on stream
end `event:error data:{"error":"..."}`. Client closes via
`EventSource.close()`.

### Backend (`kube.StreamLogs`)
1. `Get` Deployment → find its Pods via `spec.selector` label match; pick
   the first Pod (sorted by name) to avoid interleaved logs.
2. Resolve `container` (default first container of the Pod).
3. `clientset.CoreV1().Pods(ns).GetLogs(pod, &corev1.PodLogOptions{
   Container, TailLines, Previous, Follow}).Stream(ctx)`.
4. `Follow=false`: read to EOF, close channel.
5. `Follow=true`: line-by-line into `<-chan LogLine` until ctx cancel or
   stream end.
6. Keepalive: every 15s emit a `:keepalive\n\n` SSE comment to defeat idle
   proxy/ browser disconnects.

`LogRequest{Namespace, Deployment, Container, TailLines, Previous, Follow}`;
`LogLine{Line string; Error error}` (Error non-nil = stream terminal).

Logs are read-only → **not** audited (avoid stdout noise). Image updates
keep their audit line.

### Preview mode
`PreviewService.StreamLogs` returns canned lines (a few static lines;
follow pushes one line/sec) so the console works under `LOCAL_PREVIEW=1`.

## Frontend

### Stack
React 18 + TypeScript, Vite, Tailwind CSS (GitHub dark), Zustand. No
router, no UI component library. Build output → `internal/httpapi/static/`,
embedded via the existing `//go:embed static/*`.

### Structure
```
web/
├── package.json, vite.config.ts, tailwind.config.js, postcss.config.js, tsconfig.json, index.html
└── src/
    ├── main.tsx, App.tsx
    ├── api/            # client.ts (fetch + bearer + 401 handling), types.ts
    ├── state/          # Zustand store: token, scope, current view
    ├── components/
    │   ├── layout/     # Shell, Sidebar, Topbar
    │   ├── auth/       # TokenGate
    │   ├── deploy/     # DeployList, DeployDetail, ImageUpdateForm
    │   ├── logs/       # LogViewer (pull + follow)
    │   └── ui/         # Button, Input, Select, Field, Toast
    └── hooks/          # useDeployments, useLogs, useToken
```

### Views
1. **TokenGate** (no token) — input + "Enter"; calls
   `POST /api/v1/auth/verify`; stores token + scope in Zustand + localStorage.
2. **User console** (token) — sidebar layout (Approach A):
   left lists in-scope Deployments; main has two tabs — Image Update and
   Logs. Selecting a Deployment fills the main area.
3. **Admin console** (`isAdmin`) — user console + a "Token management"
   panel in the sidebar (CRUD tokens).

### Theme tokens (Tailwind CSS vars)
bg `#0d1117`, panel `#161b22`, border `#30363d`, primary blue `#1f6feb`,
success green `#238636`, danger red `#da3633`, text `#c9d1d9`, muted
`#7d8590`.

### Auth/API client
- Token in store + localStorage; each request adds `Authorization: Bearer`.
- 401 → clear token, return to TokenGate, prompt re-login.
- 403 → toast "no access to this resource".
- SSE follow uses `EventSource` with `?token=` query (header fallback
  impossible for `EventSource`).

### Local dev
```bash
# Terminal 1: Go backend
LOCAL_PREVIEW=1 ADMIN_TOKEN=admin123 TOKENS_PATH=./tokens.json go run ./cmd/server
# Terminal 2: Vite dev server (proxies /api, /healthz to :8080)
cd web && npm run dev
```

## Config (new env vars)

| Env | Default | Notes |
| --- | --- | --- |
| `ADMIN_TOKEN` | empty | admin bearer; empty ⇒ admin API 503 |
| `TOKENS_PATH` | `/data/tokens.json` | token store file |
| `LOG_TAIL_LINES` | `500` | default one-shot log tail |

`AUTH_TOKEN` removed.

## Deployment changes

### `deploy/rbac.yaml`
Add:
```yaml
- apiGroups: [""]
  resources: ["pods/log"]
  verbs: ["get"]
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "patch", "create"]
```
`create` retained; `list` added for the read-only Deployment listing.

### `deploy/deployment.yaml`
- Remove `AUTH_TOKEN`; add `ADMIN_TOKEN` (from Secret
  `letorollout-admin-auth` key `admin-token`, optional), `TOKENS_PATH`,
  `LOG_TAIL_LINES`.
- Add a PVC `letorollout-data` (1Gi) mounted at `/data` for token
  persistence. `emptyDir` is an alternative but loses tokens on Pod
  rebuild; PVC is the recommended default.
- Secret renamed `letorollout-admin-auth`.

### `deploy/pvc.yaml` (new)
`letorollout-data` PVC template (1Gi, default storage class).

## Build

Multi-stage Dockerfile adds a Node web build stage before the Go build:

```dockerfile
FROM node:20-slim AS web
WORKDIR /web
COPY web/package*.json ./
RUN npm ci
COPY web/ .
RUN npm run build            # output -> ../internal/httpapi/static

FROM golang:1.20 AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .                     # includes web build output
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/letorollout ./cmd/server

FROM gcr.io/distroless/static-debian11:nonroot
COPY --from=builder /out/letorollout /letorollout
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/letorollout"]
```

`//go:embed static/*` unchanged; `go build` embeds the Vite output
(`index.html` + `assets/`). Go 1.20 stays (client-go v0.28.4 compatible).

## Testing

Standard `testing` + `httptest` + fake service; no new framework.

- `auth`: TokenStore CRUD, file read/write, Verify (valid/expired/missing),
  Allows (namespace-level / deployment-level / out-of-scope).
- `httpapi`: each new endpoint happy path + 401/403/404/400; middleware
  auth chains.
- `kube`: ListDeployments/GetDeployment via fake clientset; StreamLogs via
  fake log stream.
- Preview: PreviewService implementations of new interfaces.
- Frontend: optional Vitest for pure functions (scope filter, token
  storage, 401 handling); not blocking.

## Verification

1. `go test ./...` green (incl. new auth/httpapi/kube tests).
2. `cd web && npm run build` outputs into static/; `go build ./cmd/server`
   succeeds.
3. `docker build` succeeds; image starts and `/healthz` returns 200.
4. Preview mode end-to-end: user enters token → sees in-scope Deployments
   → updates image → views logs (pull + follow); admin enters
   `ADMIN_TOKEN` → CRUDs tokens.
5. No residual Create references (API, form, types).

## Migration order

Two phases, each independently verifiable:

1. **Backend first** — implement new Go interfaces + routes + token store;
   old frontend untouched (Create form will 404, image update still works).
   Verify each endpoint with curl.
2. **React rewrite** — scaffold `web/`, build output replaces
   `internal/httpapi/static/`; delete old frontend files and removed code.
