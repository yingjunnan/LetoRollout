# Admin Token Form: Namespace Picker & Expiry Picker

Date: 2026-07-01

## Problem

In the admin token-creation form (`web/src/components/admin/AdminPanel.tsx`),
both the **namespace** and the **expires-at** fields are free-text inputs:

- Namespace: admin types a ns name from memory; typos produce tokens scoped to
  non-existent namespaces.
- Expires-at: admin must type a raw RFC3339 string (`2026-12-31T00:00:00Z`).

## Goals

- Namespace field becomes a strict `<select>` populated from the cluster.
- Expiry becomes a preset-button + custom datetime picker, no raw RFC3339 typing.

## Non-goals

- Multi-namespace scopes per token (the form still grants one scope).
- Editing existing tokens.

## Decisions (from brainstorming)

1. Namespace list respects `ALLOWED_NAMESPACES`: if set, only those; if empty,
   all active namespaces in the cluster. Consistent with the image-updater's
   security boundary (`namespaceAllowed`).
2. Expiry picker: preset durations (1h / 1d / 7d / 30d / Never) plus a "Custom"
   toggle that reveals a `datetime-local` input.
3. Namespace field is a strict `<select>` (no free typing).

## Architecture

The HTTP layer must not depend on Kubernetes directly (project rule:
`httpapi` depends only on `rollout` service interfaces). Namespace listing
therefore flows through a new `rollout.NamespaceLister` interface.

```
AdminPanel.tsx
   │ GET /api/v1/namespaces   (adminMw-gated)
   ▼
handleListNamespaces ── Service.ListNamespaces(ctx)
                          │  (interface in rollout, impl in kube + preview)
                          ▼
                   kube: CoreV1().Namespaces().List()
                        + namespaceAllowed() filter
```

## Backend changes

### `internal/rollout/types.go`
```go
type NamespaceLister interface {
    ListNamespaces(ctx context.Context) ([]string, error)
}
```

### `internal/httpapi/handler.go`
- Fold `rollout.NamespaceLister` into `type Service interface { ... }`.
- `PreviewService.ListNamespaces`: return distinct namespaces from seeded
  deployments (sorted).
- New route: `mux.Handle("GET /api/v1/namespaces", adminMw(handleListNamespaces(service)))`.
- `handleListNamespaces`: returns `{"namespaces": [...]}`.

### `internal/kube/deployment_image_updater.go`
- `DeploymentImageUpdater.ListNamespaces`: `CoreV1().Namespaces().List`,
  skip `Terminating`, apply `namespaceAllowed`, sort.

### `deploy/rbac.yaml`
Add rule:
```yaml
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["list"]
```

## Frontend changes

### `web/src/api/client.ts`
- `adminListNamespaces(token) -> string[]` hitting `GET /api/v1/namespaces`.

### `web/src/components/admin/AdminPanel.tsx`
- On mount, fetch namespaces; render `<select>` (with a disabled "Select…"
  placeholder option). Empty state: show a message + retry.
- Replace the expiry `<Input>` with an inline `ExpiryPicker`:
  - Preset buttons: 1h, 1d, 7d, 30d, Never.
  - "Custom" toggle reveals `<input type="datetime-local">`.
  - All options resolve to an RFC3339 UTC string (or `null` for Never),
    matching the existing `adminCreateToken` body shape.

Expiry resolution (client-side): presets compute `now + duration` in UTC and
format RFC3339; `datetime-local` value is parsed as local time then converted
to UTC RFC3339.

## Testing

- Go unit tests:
  - `PreviewService.ListNamespaces` returns distinct seeded ns.
  - `handleListNamespaces` returns 200 + JSON, gated behind admin token.
- Frontend: build succeeds (`npm run build`); manual smoke via port-forward.
- Cluster: re-apply RBAC, verify `GET /api/v1/namespaces` returns ns list.
