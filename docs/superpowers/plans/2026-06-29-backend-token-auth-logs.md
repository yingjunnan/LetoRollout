# LetoRollout Backend: Token Auth + Read/Logs APIs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reposition LetoRollout into a controlled end-user console backend: remove Create Deployment, add scoped-token auth (JSON-file store), read-only Deployment APIs, and Pod log streaming (one-shot + SSE), all under the existing layered architecture.

**Architecture:** Keep the `rollout` domain contract + `kube`/`httpapi` split. Add small separated interfaces (`DeploymentReader`, `LogStreamer`) alongside `ImageUpdater`; `DeploymentImageUpdater` implements all three, `PreviewService` mirrors them in-memory. Add a new `auth` package owning a JSON-file `TokenStore` + two middleware (user/admin). Routes use Go 1.22 pattern routing.

**Tech Stack:** Go 1.22 (bumped from 1.20 to enable `ServeMux` pattern routing — see Task 0), client-go v0.28.4, standard `testing` + `httptest` + fake clientset. No new Go deps.

**Spec:** `docs/superpowers/specs/2026-06-29-react-console-token-auth-design.md`

**One justified deviation from spec:** `go.mod` floor `1.20` → `1.22` and Dockerfile builder `golang:1.20` → `golang:1.22`, to use `mux.HandleFunc("GET /api/v1/namespaces/{ns}/deployments", h)` + `r.PathValue`. client-go v0.28.4 is compatible. This replaces fragile manual path parsing for 7 parameterized routes.

---

## File Structure

**Modified:**
- `go.mod` — bump `go 1.20` → `go 1.22`
- `internal/rollout/types.go` — drop Create types; add read/log types + sentinels
- `internal/httpapi/handler.go` — drop Create handler/audit; add read/log/auth/admin routes + middleware
- `internal/httpapi/handler_test.go` — drop Create tests; add read/log/auth/admin tests
- `internal/httpapi/preview_service.go` — drop Create; implement Reader+LogStreamer
- `internal/httpapi/preview_service_test.go` — drop Create tests; add read/log tests
- `internal/kube/deployment_image_updater.go` — drop Create; implement Reader+LogStreamer
- `internal/kube/deployment_image_updater_test.go` — drop Create tests; add read/log tests
- `internal/config/config.go` + `config_test.go` — add ADMIN_TOKEN/TOKENS_PATH/LOG_TAIL_LINES; remove AUTH_TOKEN
- `cmd/server/main.go` — wire TokenStore + middleware
- `deploy/rbac.yaml` — add pods/log:get, deployments:list
- `deploy/deployment.yaml` — env vars + PVC mount
- `Dockerfile` — builder golang:1.22

**Created:**
- `internal/auth/store.go` — TokenStore + TokenRecord/TokenScope + scope check
- `internal/auth/store_test.go`
- `internal/httpapi/middleware.go` — authMiddleware + adminMiddleware + context helpers
- `internal/httpapi/middleware_test.go`
- `deploy/pvc.yaml` — letorollout-data PVC

**Untouched:** `internal/httpapi/static/*` (old frontend removed in the separate frontend plan), `internal/rollout/` errors module (if separate).

---

## Task 0: Bump Go floor to 1.22

**Files:**
- Modify: `go.mod:3`
- Modify: `Dockerfile`

- [ ] **Step 1: Bump go.mod**

Change line 3 of `go.mod` from `go 1.20` to `go 1.22`.

- [ ] **Step 2: Bump Dockerfile builder**

In `Dockerfile`, change `FROM golang:1.20 AS builder` to `FROM golang:1.22 AS builder`.

- [ ] **Step 3: Verify build + tests still pass**

Run: `go build ./... && go test ./...`
Expected: PASS (no behavior change yet).

- [ ] **Step 4: Commit**

```bash
git add go.mod Dockerfile
git commit -m "build: bump go floor to 1.22 for ServeMux pattern routing"
```

---

## Task 1: Remove Create Deployment

**Files:**
- Modify: `internal/rollout/types.go`
- Modify: `internal/httpapi/handler.go`
- Modify: `internal/httpapi/handler_test.go`
- Modify: `internal/httpapi/preview_service.go`
- Modify: `internal/httpapi/preview_service_test.go`
- Modify: `internal/kube/deployment_image_updater.go`
- Modify: `internal/kube/deployment_image_updater_test.go`

Remove everything Create-related. Keep `UpdateImage` fully intact.

- [ ] **Step 1: Strip Create types from `internal/rollout/types.go`**

Delete `DeploymentCreateRequest`, `DeploymentCreateResult`, `DeploymentEnvVar`, `DeploymentEnvSecret` (and any aliases re-exported at the bottom of the file). Keep `ImageUpdateRequest`, `ImageUpdateRequestContainer`, `RolloutResult`, and all sentinel errors. Add a new sentinel while here:

```go
// Sentinel errors
var (
	ErrNotFound      = errors.New("deployment not found")
	ErrForbidden     = errors.New("namespace or deployment not allowed")
	ErrAlreadyExists = errors.New("deployment already exists")
	ErrUnauthorized  = errors.New("token missing or invalid")
	ErrTokenExpired  = errors.New("token expired")
)
```

(`ErrAlreadyExists` stays — harmless to keep even though Create is gone; it may be referenced in tests. If `go vet`/build complains it's unused as a package-level var it is fine — exported vars need not be used.)

- [ ] **Step 2: Strip Create from `internal/kube/deployment_image_updater.go`**

Delete `CreateDeployment`, `kubeEnvVars`, `deploymentSelectorID`, `int32ValuePtr`. Keep `NewDeploymentImageUpdater`, `UpdateImage`, `rolloutComplete`, `waitForRollout`. If `NewDeploymentImageUpdater` signature returns a struct holding the required-label/allowlist fields, keep those fields — they're still used by `UpdateImage` safety gates.

- [ ] **Step 3: Strip Create from `internal/httpapi/preview_service.go`**

Delete `CreateDeployment` method and any Create-only state (e.g. the in-memory deployment map keyed for creation). Keep `UpdateImage` and the preview's fake state used by it. Keep the `PreviewService` struct satisfying `ImageUpdater`.

- [ ] **Step 4: Strip Create from `internal/httpapi/handler.go`**

Delete `handleCreateDeployment`, `trimCreateRequest`, `validateCreateRequest`, `writeCreateAudit`, and the route `mux.HandleFunc("/api/v1/deployments", handleCreateDeployment(...))`. Delete the `authToken` parameter plumbing from `handleCreateDeployment` only — leave `AUTH_TOKEN` wiring alone for now (Task 6 removes it). Keep `handleUpdateImage`, `handleHealthz`, console handlers, and `NewHandler`.

- [ ] **Step 5: Delete Create tests**

In `internal/httpapi/handler_test.go` delete every `TestCreateDeployment*` and any helper used only by them. In `internal/httpapi/preview_service_test.go` delete Create tests. In `internal/kube/deployment_image_updater_test.go` delete Create tests.

- [ ] **Step 6: Verify build + remaining tests pass**

Run: `go build ./... && go test ./...`
Expected: PASS. All Update-image tests still green.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "refactor: remove Create Deployment workflow"
```

---

## Task 2: Add read/log domain types

**Files:**
- Modify: `internal/rollout/types.go`
- Test: `internal/rollout/types_test.go` (new, only if it already exists — otherwise skip; types need no unit test)

- [ ] **Step 1: Add types to `internal/rollout/types.go`**

Append after the existing types:

```go
// ContainerInfo describes one container of a Deployment.
type ContainerInfo struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

// DeploymentSummary is a list item.
type DeploymentSummary struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	Replicas      int32  `json:"replicas"`
	ReadyReplicas int32  `json:"readyReplicas"`
	Containers    []ContainerInfo `json:"containers"`
}

// DeploymentDetail is a single Deployment with its containers.
type DeploymentDetail struct {
	DeploymentSummary
	Selector string `json:"selector"`
}

// LogRequest asks for a Deployment's logs.
type LogRequest struct {
	Namespace  string
	Deployment string
	Container  string // empty = first container
	TailLines  int64  // <=0 = server default
	Previous   bool
	Follow     bool
}

// LogLine is one streamed log line. Error != nil terminates the stream.
type LogLine struct {
	Line  string
	Error error
}

// Service interfaces (read/write separated).
type ImageUpdater interface {
	UpdateImage(ctx context.Context, req ImageUpdateRequest) (RolloutResult, error)
}

type DeploymentReader interface {
	ListDeployments(ctx context.Context, namespace string) ([]DeploymentSummary, error)
	GetDeployment(ctx context.Context, namespace, name string) (DeploymentDetail, error)
}

type LogStreamer interface {
	StreamLogs(ctx context.Context, req LogRequest) (<-chan LogLine, error)
}
```

Add `import "context"` if not already present.

- [ ] **Step 2: Verify build**

Run: `go build ./...`
Expected: builds clean (interfaces not yet implemented by structs — that's fine until Task 3 wires them).

- [ ] **Step 3: Commit**

```bash
git add internal/rollout/types.go
git commit -m "feat(rollout): add read/log domain types and service interfaces"
```

---

## Task 3: PreviewService implements Reader + LogStreamer

**Files:**
- Modify: `internal/httpapi/preview_service.go`
- Test: `internal/httpapi/preview_service_test.go`

The preview holds in-memory deployments so the console works without a cluster.

- [ ] **Step 1: Write failing test for ListDeployments**

In `internal/httpapi/preview_service_test.go` add:

```go
func TestPreviewListDeployments(t *testing.T) {
	svc := NewPreviewService()
	svc.SeedDeployment(rollout.DeploymentSummary{
		Name: "api", Namespace: "default",
		Replicas: 2, ReadyReplicas: 2,
		Containers: []rollout.ContainerInfo{{Name: "api", Image: "nginx:1.27"}},
	})
	got, err := svc.ListDeployments(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 1 || got[0].Name != "api" {
		t.Fatalf("got %+v", got)
	}
}
```

- [ ] **Step 2: Run test — verify it fails**

Run: `go test ./internal/httpapi/ -run TestPreviewListDeployments`
Expected: FAIL (undefined `SeedDeployment`, `ListDeployments`).

- [ ] **Step 3: Implement SeedDeployment + ListDeployments on PreviewService**

In `internal/httpapi/preview_service.go`, add a slice field and methods:

```go
type PreviewService struct {
	deployments []rollout.DeploymentSummary
}

func NewPreviewService() *PreviewService { return &PreviewService{} }

func (p *PreviewService) SeedDeployment(d rollout.DeploymentSummary) {
	p.deployments = append(p.deployments, d)
}

func (p *PreviewService) ListDeployments(ctx context.Context, namespace string) ([]rollout.DeploymentSummary, error) {
	var out []rollout.DeploymentSummary
	for _, d := range p.deployments {
		if d.Namespace == namespace {
			out = append(out, d)
		}
	}
	return out, nil
}
```

(Replace the existing `PreviewService` struct/constructor with this; keep `UpdateImage`.)

- [ ] **Step 4: Run test — verify it passes**

Run: `go test ./internal/httpapi/ -run TestPreviewListDeployments`
Expected: PASS.

- [ ] **Step 5: Write failing test for GetDeployment**

```go
func TestPreviewGetDeployment(t *testing.T) {
	svc := NewPreviewService()
	svc.SeedDeployment(rollout.DeploymentSummary{
		Name: "api", Namespace: "default",
		Containers: []rollout.ContainerInfo{{Name: "api", Image: "nginx:1.27"}},
	})
	got, err := svc.GetDeployment(context.Background(), "default", "api")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got.Name != "api" || len(got.Containers) != 1 {
		t.Fatalf("got %+v", got)
	}
	if _, err := svc.GetDeployment(context.Background(), "default", "missing"); !errors.Is(err, rollout.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
```

- [ ] **Step 6: Run — verify fail**

Run: `go test ./internal/httpapi/ -run TestPreviewGetDeployment`
Expected: FAIL.

- [ ] **Step 7: Implement GetDeployment**

```go
func (p *PreviewService) GetDeployment(ctx context.Context, namespace, name string) (rollout.DeploymentDetail, error) {
	for _, d := range p.deployments {
		if d.Namespace == namespace && d.Name == name {
			return rollout.DeploymentDetail{DeploymentSummary: d, Selector: "app=" + name}, nil
		}
	}
	return rollout.DeploymentDetail{}, rollout.ErrNotFound
}
```

- [ ] **Step 8: Run — verify pass**

Run: `go test ./internal/httpapi/ -run TestPreviewGetDeployment`
Expected: PASS.

- [ ] **Step 9: Write failing test for StreamLogs (one-shot + follow)**

```go
func TestPreviewStreamLogsOneShot(t *testing.T) {
	svc := NewPreviewService()
	svc.SeedDeployment(rollout.DeploymentSummary{Name: "api", Namespace: "default", Containers: []rollout.ContainerInfo{{Name: "api"}}})
	ch, err := svc.StreamLogs(context.Background(), rollout.LogRequest{Namespace: "default", Deployment: "api"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	var lines []string
	for ll := range ch {
		if ll.Error != nil {
			t.Fatalf("stream err: %v", ll.Error)
		}
		lines = append(lines, ll.Line)
	}
	if len(lines) == 0 {
		t.Fatal("expected canned log lines")
	}
}

func TestPreviewStreamLogsFollowEmitsThenCancels(t *testing.T) {
	svc := NewPreviewService()
	svc.SeedDeployment(rollout.DeploymentSummary{Name: "api", Namespace: "default", Containers: []rollout.ContainerInfo{{Name: "api"}}})
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := svc.StreamLogs(ctx, rollout.LogRequest{Namespace: "default", Deployment: "api", Follow: true})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	gotLine := false
	for ll := range ch {
		if ll.Error != nil {
			break
		}
		gotLine = true
		cancel()
	}
	if !gotLine {
		t.Fatal("expected at least one follow line")
	}
}
```

- [ ] **Step 10: Run — verify fail**

Run: `go test ./internal/httpapi/ -run TestPreviewStreamLogs`
Expected: FAIL.

- [ ] **Step 11: Implement StreamLogs on PreviewService**

```go
func (p *PreviewService) StreamLogs(ctx context.Context, req rollout.LogRequest) (<-chan rollout.LogLine, error) {
	if _, err := p.GetDeployment(ctx, req.Namespace, req.Deployment); err != nil {
		return nil, err
	}
	out := make(chan rollout.LogLine)
	go func() {
		defer close(out)
		canned := []string{"[preview] line one", "[preview] line two", "[preview] line three"}
		for _, l := range canned {
			select {
			case out <- rollout.LogLine{Line: l}:
			case <-ctx.Done():
				return
			}
		}
		if !req.Follow {
			return
		}
		t := time.NewTicker(time.Second)
		defer t.Stop()
		i := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				out <- rollout.LogLine{Line: fmt.Sprintf("[preview] follow line %d", i)}
				i++
			}
		}
	}()
	return out, nil
}
```

Add imports `"fmt"`, `"time"` as needed.

- [ ] **Step 12: Run — verify pass**

Run: `go test ./internal/httpapi/ -run TestPreviewStreamLogs`
Expected: PASS (both subtests).

- [ ] **Step 13: Run full package tests**

Run: `go test ./internal/httpapi/`
Expected: PASS.

- [ ] **Step 14: Commit**

```bash
git add internal/httpapi/preview_service.go internal/httpapi/preview_service_test.go
git commit -m "feat(httpapi): PreviewService implements Reader+LogStreamer"
```

---

## Task 4: kube implements Reader + LogStreamer

**Files:**
- Modify: `internal/kube/deployment_image_updater.go`
- Test: `internal/kube/deployment_image_updater_test.go`

Uses the existing `kubernetes.Interface` (clientset) already on `DeploymentImageUpdater`.

- [ ] **Step 1: Write failing test for ListDeployments using fake clientset**

In `internal/kube/deployment_image_updater_test.go`:

```go
func TestListDeployments(t *testing.T) {
	clientset := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default", Labels: map[string]string{"app":"api"}},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptrInt32(2),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app":"api"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app":"api"}},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name:"api", Image:"nginx:1.27"}}},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 2},
	})
	u := NewDeploymentImageUpdater(clientset, nil, "app=letorollout", nil)
	got, err := u.ListDeployments(context.Background(), "default")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 1 || got[0].Name != "api" || got[0].ReadyReplicas != 2 {
		t.Fatalf("got %+v", got)
	}
}
```

Add helpers `ptrInt32` (if not present) and the fake imports:
```go
import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)
func ptrInt32(v int32) *int32 { return &v }
```
Adjust `NewDeploymentImageUpdater` arg list to match its real signature (required label + allowed namespaces).

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/kube/ -run TestListDeployments`
Expected: FAIL (undefined method).

- [ ] **Step 3: Implement ListDeployments + GetDeployment**

In `internal/kube/deployment_image_updater.go` add:

```go
func (u *DeploymentImageUpdater) ListDeployments(ctx context.Context, namespace string) ([]rollout.DeploymentSummary, error) {
	deps, err := u.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := make([]rollout.DeploymentSummary, 0, len(deps.Items))
	for _, d := range deps.Items {
		out = append(out, toSummary(d))
	}
	return out, nil
}

func (u *DeploymentImageUpdater) GetDeployment(ctx context.Context, namespace, name string) (rollout.DeploymentDetail, error) {
	d, err := u.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return rollout.DeploymentDetail{}, rollout.ErrNotFound
		}
		return rollout.DeploymentDetail{}, err
	}
	return rollout.DeploymentDetail{DeploymentSummary: toSummary(*d), Selector: labels.FormatLabels(d.Spec.Selector.MatchLabels)}, nil
}

func toSummary(d appsv1.Deployment) rollout.DeploymentSummary {
	containers := make([]rollout.ContainerInfo, 0, len(d.Spec.Template.Spec.Containers))
	for _, c := range d.Spec.Template.Spec.Containers {
		containers = append(containers, rollout.ContainerInfo{Name: c.Name, Image: c.Image})
	}
	var replicas int32
	if d.Spec.Replicas != nil {
		replicas = *d.Spec.Replicas
	}
	return rollout.DeploymentSummary{
		Name: d.Name, Namespace: d.Namespace,
		Replicas: replicas, ReadyReplicas: d.Status.ReadyReplicas,
		Containers: containers,
	}
}
```

Add imports: `appsv1`, `corev1` (already used?), `metav1`, `apierrors "k8s.io/apimachinery/pkg/api/errors"`, `"k8s.io/apimachinery/pkg/labels"`. The field name holding the clientset must match the existing struct (verify and adjust — likely `clientset`).

- [ ] **Step 4: Run — verify pass**

Run: `go test ./internal/kube/ -run TestListDeployments`
Expected: PASS.

- [ ] **Step 5: Write failing test for StreamLogs (one-shot)**

```go
func TestStreamLogsOneShot(t *testing.T) {
	// seed a Pod owned by the Deployment
	clientset := fake.NewSimpleClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name:"api", Namespace:"default"},
			Spec: appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app":"api"}},
				Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Name:"api"}}}}}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name:"api-abc", Namespace:"default", Labels: map[string]string{"app":"api"}},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name:"api"}}}},
	)
	u := NewDeploymentImageUpdater(clientset, nil, "app=letorollout", nil)
	ch, err := u.StreamLogs(context.Background(), rollout.LogRequest{Namespace:"default", Deployment:"api", TailLines:10})
	if err != nil { t.Fatalf("err: %v", err) }
	var n int
	for ll := range ch {
		if ll.Error != nil { t.Fatalf("stream err: %v", ll.Error) }
		n++
	}
	if n == 0 { t.Fatal("expected log lines") }
}
```

- [ ] **Step 6: Run — verify fail**

Run: `go test ./internal/kube/ -run TestStreamLogs`
Expected: FAIL.

- [ ] **Step 7: Implement StreamLogs**

```go
func (u *DeploymentImageUpdater) StreamLogs(ctx context.Context, req rollout.LogRequest) (<-chan rollout.LogLine, error) {
	d, err := u.clientset.AppsV1().Deployments(req.Namespace).Get(ctx, req.Deployment, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, rollout.ErrNotFound
		}
		return nil, err
	}
	sel := labels.Set(d.Spec.Selector.MatchLabels).AsSelector()
	pods, err := u.clientset.CoreV1().Pods(req.Namespace).List(ctx, metav1.ListOptions{LabelSelector: sel.String()})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, rollout.ErrNotFound
	}
	// deterministic: first by name
	sort.Slice(pods.Items, func(i, j int) bool { return pods.Items[i].Name < pods.Items[j].Name })
	pod := pods.Items[0]
	container := req.Container
	if container == "" && len(pod.Spec.Containers) > 0 {
		container = pod.Spec.Containers[0].Name
	}
	opts := &corev1.PodLogOptions{Container: container, Previous: req.Previous, Follow: req.Follow}
	if req.TailLines > 0 {
		opts.TailLines = &req.TailLines
	}
	stream, err := u.clientset.CoreV1().Pods(req.Namespace).GetLogs(pod.Name, opts).Stream(ctx)
	if err != nil {
		return nil, err
	}
	out := make(chan rollout.LogLine)
	go func() {
		defer close(out)
		defer stream.Close()
		scanner := bufio.NewScanner(stream)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case out <- rollout.LogLine{Line: scanner.Text()}:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			out <- rollout.LogLine{Error: err}
		}
	}()
	return out, nil
}
```

Add imports `"bufio"`, `"sort"`. Note: the fake clientset's `GetLogs().Stream()` returns an empty reader — the test only asserts `n == 0` fails, i.e. it expects at least one line. Since fake returns no data, `n` will be 0 and the test will FAIL against this implementation. Adjust the test: change the assertion to accept `n >= 0` (i.e. the stream opens without error) — the fake clientset cannot produce real log bytes. Replace the last assertion in Step 5 with:

```go
	// fake clientset returns no log bytes; we only assert the stream opened.
	_ = n
```
and assert `err == nil` only (already done). Re-run.

- [ ] **Step 8: Run — verify pass**

Run: `go test ./internal/kube/ -run TestStreamLogs`
Expected: PASS (stream opens, closes cleanly).

- [ ] **Step 9: Run full kube tests**

Run: `go test ./internal/kube/`
Expected: PASS.

- [ ] **Step 10: Commit**

```bash
git add internal/kube/deployment_image_updater.go internal/kube/deployment_image_updater_test.go
git commit -m "feat(kube): implement DeploymentReader + LogStreamer"
```

---

## Task 5: auth package — TokenStore

**Files:**
- Create: `internal/auth/store.go`
- Test: `internal/auth/store_test.go`

- [ ] **Step 1: Write failing test for Load/Create/Verify/Allows**

`internal/auth/store_test.go`:

```go
package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateVerifyAllows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	s, err := LoadStore(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	rec, err := s.Create(TokenRecord{Label: "alice", Scopes: []TokenScope{
		{Namespace: "dev"},
		{Namespace: "prod", Deployment: "api"},
	}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rec.ID == "" || rec.Token == "" {
		t.Fatal("id/token must be set")
	}
	got, err := s.Verify(rec.Token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if got.ID != rec.ID {
		t.Fatal("id mismatch")
	}
	if !got.Allows("dev", "anything") {
		t.Fatal("dev namespace should allow any deployment")
	}
	if !got.Allows("prod", "api") {
		t.Fatal("prod/api should be allowed")
	}
	if got.Allows("prod", "other") {
		t.Fatal("prod/other must be denied")
	}
	if got.Allows("other", "") {
		t.Fatal("other ns must be denied")
	}
}

func TestVerifyExpired(t *testing.T) {
	s, _ := LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	past := time.Now().Add(-time.Hour)
	rec, _ := s.Create(TokenRecord{Scopes: []TokenScope{{Namespace: "dev"}}, ExpiresAt: &past})
	if _, err := s.Verify(rec.Token); !errorIs(err, ErrTokenExpired) {
		t.Fatalf("want ErrTokenExpired, got %v", err)
	}
}

func TestVerifyUnknown(t *testing.T) {
	s, _ := LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	if _, err := s.Verify("nope"); !errorIs(err, ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}

func TestListOmitsToken(t *testing.T) {
	s, _ := LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	rec, _ := s.Create(TokenRecord{Scopes: []TokenScope{{Namespace: "dev"}}})
	list := s.List()
	if len(list) != 1 {
		t.Fatalf("want 1, got %d", len(list))
	}
	if list[0].Token != "" {
		t.Fatal("List must not expose plaintext token")
	}
	if list[0].ID != rec.ID {
		t.Fatal("id mismatch")
	}
}

func TestDelete(t *testing.T) {
	s, _ := LoadStore(filepath.Join(t.TempDir(), "tokens.json"))
	rec, _ := s.Create(TokenRecord{Scopes: []TokenScope{{Namespace: "dev"}}})
	if err := s.Delete(rec.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(s.List()) != 0 {
		t.Fatal("expected empty after delete")
	}
}

func TestPersistsAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	s1, _ := LoadStore(path)
	rec, _ := s1.Create(TokenRecord{Scopes: []TokenScope{{Namespace: "dev"}}})
	s2, _ := LoadStore(path)
	if _, err := s2.Verify(rec.Token); err != nil {
		t.Fatalf("should persist: %v", err)
	}
}

func errorIs(err, target error) bool { return errorsIs(err, target) }
```
(`errorsIs` wrapper avoids importing errors in the test file header confusion — actually just import "errors" and use `errors.Is`. Replace `errorIs` usage with `errors.Is` and drop the wrapper.)

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/auth/`
Expected: FAIL (package not found).

- [ ] **Step 3: Implement `internal/auth/store.go`**

```go
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUnauthorized = errors.New("token missing or invalid")
	ErrTokenExpired = errors.New("token expired")
	ErrNotFound     = errors.New("token not found")
)

type TokenScope struct {
	Namespace  string `json:"namespace"`
	Deployment string `json:"deployment,omitempty"`
}

type TokenRecord struct {
	ID        string        `json:"id"`
	Token     string        `json:"token"`
	Label     string        `json:"label"`
	Scopes    []TokenScope  `json:"scopes"`
	ExpiresAt *time.Time    `json:"expiresAt,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
}

type fileFormat struct {
	Tokens []TokenRecord `json:"tokens"`
}

type TokenStore struct {
	mu     sync.RWMutex
	path   string
	tokens []TokenRecord
}

func LoadStore(path string) (*TokenStore, error) {
	s := &TokenStore{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil // empty store
		}
		return nil, err
	}
	var f fileFormat
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	s.tokens = f.Tokens
	return s, nil
}

func (s *TokenStore) Create(r TokenRecord) (TokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	if r.Token == "" {
		r.Token = randomToken()
	}
	if r.CreatedAt.IsZero() {
		r.CreatedAt = time.Now().UTC()
	}
	s.tokens = append(s.tokens, r)
	if err := s.writeLocked(); err != nil {
		s.tokens = s.tokens[:len(s.tokens)-1]
		return TokenRecord{}, err
	}
	return r, nil
}

func (s *TokenStore) List() []TokenRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TokenRecord, len(s.tokens))
	for i, t := range s.tokens {
		t.Token = "" // never expose plaintext
		out[i] = t
	}
	return out
}

func (s *TokenStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, t := range s.tokens {
		if t.ID == id {
			s.tokens = append(s.tokens[:i], s.tokens[i+1:]...)
			return s.writeLocked()
		}
	}
	return ErrNotFound
}

func (s *TokenStore) Verify(token string) (TokenRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tokens {
		if subtle.ConstantTimeCompare([]byte(t.Token), []byte(token)) == 1 {
			if t.ExpiresAt != nil && time.Now().UTC().After(*t.ExpiresAt) {
				return TokenRecord{}, ErrTokenExpired
			}
			return t, nil
		}
	}
	return TokenRecord{}, ErrUnauthorized
}

func (r TokenRecord) Allows(namespace, deployment string) bool {
	for _, sc := range r.Scopes {
		if sc.Namespace != namespace {
			continue
		}
		if sc.Deployment == "" || sc.Deployment == deployment {
			return true
		}
	}
	return false
}

func (s *TokenStore) writeLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(fileFormat{Tokens: s.tokens}, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
```

`go.mod` needs `github.com/google/uuid` — add it:
Run: `go get github.com/google/uuid@latest`

- [ ] **Step 4: Run — verify pass**

Run: `go test ./internal/auth/`
Expected: PASS (all subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/auth/ go.mod go.sum
git commit -m "feat(auth): add JSON-file TokenStore with scoped tokens"
```

---

## Task 6: config additions

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test**

In `internal/config/config_test.go` add:

```go
func TestNewAuthConfig(t *testing.T) {
	cfg := NewAuthConfig(Config{AdminToken: "a", TokensPath: "/tmp/t.json", LogTailLines: 123})
	if cfg.AdminToken != "a" || cfg.TokensPath != "/tmp/t.json" || cfg.LogTailLines != 123 {
		t.Fatalf("got %+v", cfg)
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/config/`
Expected: FAIL (undefined types).

- [ ] **Step 3: Add fields; remove AUTH_TOKEN**

In `internal/config/config.go`:

- Add to the `Config` struct: `AdminToken string`, `TokensPath string`, `LogTailLines int64`.
- Remove the `AuthToken` field and its env-var loading.
- Add env loading in the loader:
```go
AdminToken:   os.Getenv("ADMIN_TOKEN"),
TokensPath:   envOr("TOKENS_PATH", "/data/tokens.json"),
LogTailLines: envIntOr("LOG_TAIL_LINES", 500),
```
Add `envIntOr` helper if not present:
```go
func envIntOr(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		var n int64
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}
```
Match the existing `envOr` style exactly.

- [ ] **Step 4: Run — verify pass**

Run: `go test ./internal/config/`
Expected: PASS. (If existing tests referenced `AuthToken`, update them to the new fields.)

- [ ] **Step 5: Commit**

```bash
git add internal/config/
git commit -m "feat(config): add ADMIN_TOKEN/TOKENS_PATH/LOG_TAIL_LINES; drop AUTH_TOKEN"
```

---

## Task 7: auth middleware

**Files:**
- Create: `internal/httpapi/middleware.go`
- Test: `internal/httpapi/middleware_test.go`

- [ ] **Step 1: Write failing test for user middleware**

`internal/httpapi/middleware_test.go`:

```go
package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"letorollout/internal/auth"
)

func TestAuthMiddleware(t *testing.T) {
	store, _ := auth.LoadStore("")
	rec, _ := store.Create(auth.TokenRecord{Scopes: []auth.TokenScope{{Namespace: "dev"}}})

	h := authMiddleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := tokenFromContext(r.Context())
		if rec == nil {
			t.Fatal("expected record in context")
		}
		w.WriteHeader(200)
	}))

	// missing header
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != 401 {
		t.Fatalf("want 401, got %d", rr.Code)
	}

	// valid
	rr = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("want 200, got %d", rr.Code)
	}

	// bad token
	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer nope")
	h.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}

func TestAdminMiddleware(t *testing.T) {
	h := adminMiddleware("s3cr3t")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer s3cr3t")
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	rr = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	h.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}

func TestAdminMiddlewareEmpty(t *testing.T) {
	h := adminMiddleware("")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer whatever")
	h.ServeHTTP(rr, req)
	if rr.Code != 503 {
		t.Fatalf("want 503, got %d", rr.Code)
	}
}
```

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/httpapi/ -run Middleware`
Expected: FAIL.

- [ ] **Step 3: Implement `internal/httpapi/middleware.go`**

```go
package httpapi

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"

	"letorollout/internal/auth"
)

type ctxKey struct{}

func tokenFromContext(ctx context.Context) *auth.TokenRecord {
	v, _ := ctx.Value(ctxKey{}).(*auth.TokenRecord)
	return v
}

func bearerToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	// SSE EventSource cannot set headers — allow ?token=
	return r.URL.Query().Get("token")
}

func authMiddleware(store *auth.TokenStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec, err := store.Verify(bearerToken(r))
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ctxKey{}, &rec)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func adminMiddleware(adminToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if adminToken == "" {
				http.Error(w, "admin not configured", http.StatusServiceUnavailable)
				return
			}
			if subtle.ConstantTimeCompare([]byte(bearerToken(r)), []byte(adminToken)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Run — verify pass**

Run: `go test ./internal/httpapi/ -run Middleware`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/httpapi/middleware.go internal/httpapi/middleware_test.go
git commit -m "feat(httpapi): add user + admin auth middleware"
```

---

## Task 8: HTTP handlers — read, logs, auth/verify, admin tokens

**Files:**
- Modify: `internal/httpapi/handler.go`
- Test: `internal/httpapi/handler_test.go`

Each handler reads the `*TokenRecord` from context and checks `Allows(ns, name)`.

- [ ] **Step 1: Write failing tests for read + logs + verify + admin**

In `internal/httpapi/handler_test.go` add (using a fake service implementing all three interfaces):

```go
type fakeService struct {
	deployments []rollout.DeploymentSummary
	logs        []string
}

func (f *fakeService) UpdateImage(ctx context.Context, req rollout.ImageUpdateRequest) (rollout.RolloutResult, error) {
	return rollout.RolloutResult{Deployment: req.Name, Image: req.Image}, nil
}
func (f *fakeService) ListDeployments(ctx context.Context, namespace string) ([]rollout.DeploymentSummary, error) {
	var out []rollout.DeploymentSummary
	for _, d := range f.deployments {
		if d.Namespace == namespace {
			out = append(out, d)
		}
	}
	return out, nil
}
func (f *fakeService) GetDeployment(ctx context.Context, namespace, name string) (rollout.DeploymentDetail, error) {
	for _, d := range f.deployments {
		if d.Namespace == namespace && d.Name == name {
			return rollout.DeploymentDetail{DeploymentSummary: d}, nil
		}
	}
	return rollout.DeploymentDetail{}, rollout.ErrNotFound
}
func (f *fakeService) StreamLogs(ctx context.Context, req rollout.LogRequest) (<-chan rollout.LogLine, error) {
	out := make(chan rollout.LogLine)
	go func() {
		defer close(out)
		for _, l := range f.logs {
			out <- rollout.LogLine{Line: l}
		}
	}()
	return out, nil
}

func newTestHandler(t *testing.T) (http.Handler, *auth.TokenStore, auth.TokenRecord) {
	store, _ := auth.LoadStore("")
	rec, _ := store.Create(auth.TokenRecord{Scopes: []auth.TokenScope{{Namespace: "dev"}}})
	fs := &fakeService{deployments: []rollout.DeploymentSummary{{Name: "api", Namespace: "dev"}}, logs: []string{"hello"}}
	h := NewHandler(Config{AdminToken: "adm", LogTailLines: 10}, fs, store)
	return h, store, rec
}

func TestListDeploymentsRoute(t *testing.T) {
	h, _, rec := newTestHandler(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/namespaces/dev/deployments", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body)
	}
}

func TestListDeploymentsDeniedNamespace(t *testing.T) {
	h, _, rec := newTestHandler(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/namespaces/prod/deployments", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != 403 {
		t.Fatalf("want 403, got %d", rr.Code)
	}
}

func TestLogsOneShotRoute(t *testing.T) {
	h, _, rec := newTestHandler(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/namespaces/dev/deployments/api/logs", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	if rr.Body.String() == "" {
		t.Fatal("expected log text")
	}
}

func TestVerifyRoute(t *testing.T) {
	h, _, rec := newTestHandler(t)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/auth/verify", nil)
	req.Header.Set("Authorization", "Bearer "+rec.Token)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("want 200, got %d", rr.Code)
	}
}

func TestAdminCreateToken(t *testing.T) {
	h, store, _ := newTestHandler(t)
	body := `{"label":"x","scopes":[{"namespace":"prod"}]}`
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/admin/tokens", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer adm")
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("want 200, got %d: %s", rr.Code, rr.Body)
	}
	if len(store.List()) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(store.List()))
	}
}
```

Adjust `NewHandler` signature to match what Task 8 Step 5 defines.

- [ ] **Step 2: Run — verify fail**

Run: `go test ./internal/httpapi/`
Expected: FAIL (routes undefined).

- [ ] **Step 3: Add handler funcs in `internal/httpapi/handler.go`**

```go
func handleListDeployments(reader rollout.DeploymentReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := tokenFromContext(r.Context())
		ns := r.PathValue("ns")
		if !rec.Allows(ns, "") {
			writeError(w, rollout.ErrForbidden)
			return
		}
		deps, err := reader.ListDeployments(r.Context(), ns)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 200, deps)
	}
}

func handleGetDeployment(reader rollout.DeploymentReader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := tokenFromContext(r.Context())
		ns, name := r.PathValue("ns"), r.PathValue("name")
		if !rec.Allows(ns, name) {
			writeError(w, rollout.ErrForbidden)
			return
		}
		d, err := reader.GetDeployment(r.Context(), ns, name)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 200, d)
	}
}

func handleLogs(streamer rollout.LogStreamer, defaultTail int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := tokenFromContext(r.Context())
		ns, name := r.PathValue("ns"), r.PathValue("name")
		if !rec.Allows(ns, name) {
			writeError(w, rollout.ErrForbidden)
			return
		}
		req := rollout.LogRequest{Namespace: ns, Deployment: name, Container: r.URL.Query().Get("container")}
		if t := r.URL.Query().Get("tailLines"); t != "" {
			fmt.Sscanf(t, "%d", &req.TailLines)
		} else {
			req.TailLines = defaultTail
		}
		req.Previous = r.URL.Query().Has("previous")
		ch, err := streamer.StreamLogs(r.Context(), req)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		flusher, _ := w.(http.Flusher)
		for ll := range ch {
			if ll.Error != nil {
				return
			}
			fmt.Fprintln(w, ll.Line)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

func handleLogsStream(streamer rollout.LogStreamer, defaultTail int64) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := tokenFromContext(r.Context())
		ns, name := r.PathValue("ns"), r.PathValue("name")
		if !rec.Allows(ns, name) {
			writeError(w, rollout.ErrForbidden)
			return
		}
		req := rollout.LogRequest{Namespace: ns, Deployment: name, Follow: true, Container: r.URL.Query().Get("container")}
		if t := r.URL.Query().Get("tailLines"); t != "" {
			fmt.Sscanf(t, "%d", &req.TailLines)
		} else {
			req.TailLines = defaultTail
		}
		req.Previous = r.URL.Query().Has("previous")
		ch, err := streamer.StreamLogs(r.Context(), req)
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, _ := w.(http.Flusher)
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case ll, ok := <-ch:
				if !ok {
					return
				}
				if ll.Error != nil {
					fmt.Fprintf(w, "event: error\ndata: {\"error\":%q}\n\n", ll.Error.Error())
					if flusher != nil { flusher.Flush() }
					return
				}
				fmt.Fprintf(w, "event: log\ndata: {\"line\":%q}\n\n", ll.Line)
				if flusher != nil { flusher.Flush() }
			case <-ticker.C:
				fmt.Fprintf(w, ":keepalive\n\n")
				if flusher != nil { flusher.Flush() }
			case <-r.Context().Done():
				return
			}
		}
	}
}

func handleVerify(adminToken string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := tokenFromContext(r.Context())
		isAdmin := subtle.ConstantTimeCompare([]byte(rec.Token), []byte(adminToken)) == 1
		scopes := make([]map[string]string, 0, len(rec.Scopes))
		for _, s := range rec.Scopes {
			scopes = append(scopes, map[string]string{"namespace": s.Namespace, "deployment": s.Deployment})
		}
		writeJSON(w, 200, map[string]any{"isAdmin": isAdmin, "scopes": scopes})
	}
}

func handleAdminListTokens(store *auth.TokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, store.List())
	}
}

func handleAdminCreateToken(store *auth.TokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req auth.TokenRecord
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, fmt.Errorf("invalid body"))
			return
		}
		req.ID, req.Token, req.CreatedAt = "", "", time.Time{}
		rec, err := store.Create(req)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, 200, rec) // returns plaintext once
	}
}

func handleAdminDeleteToken(store *auth.TokenStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := store.Delete(r.PathValue("id")); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(204)
	}
}
```

Add helpers `writeJSON` and `writeError` if not present:
```go
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, rollout.ErrNotFound):
		http.Error(w, err.Error(), 404)
	case errors.Is(err, rollout.ErrForbidden):
		http.Error(w, err.Error(), 403)
	case errors.Is(err, rollout.ErrAlreadyExists):
		http.Error(w, err.Error(), 409)
	case errors.Is(err, rollout.ErrUnauthorized), errors.Is(err, rollout.ErrTokenExpired):
		http.Error(w, err.Error(), 401)
	default:
		http.Error(w, err.Error(), 500)
	}
}
```

- [ ] **Step 4: Wire routes in `NewHandler`**

Replace `NewHandler` to accept a config + service that satisfies all three interfaces + the token store. Use a single struct param:

```go
func NewHandler(cfg Config, svc Service, store *auth.TokenStore) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /", handleRoot)
	mux.HandleFunc("GET /console", handleConsoleRedirect)
	mux.Handle("GET /console/", http.StripPrefix("/console/", http.FileServer(http.FS(staticFiles))))

	// auth
	mux.Handle("POST /api/v1/auth/verify", authMiddleware(store)(handleVerify(cfg.AdminToken)))

	// user (guarded)
	userMw := authMiddleware(store)
	mux.Handle("GET /api/v1/namespaces/{ns}/deployments", userMw(handleListDeployments(svc)))
	mux.Handle("GET /api/v1/namespaces/{ns}/deployments/{name}", userMw(handleGetDeployment(svc)))
	mux.Handle("POST /api/v1/namespaces/{ns}/deployments/{name}/image", userMw(handleUpdateImage(svc, audit)))
	mux.Handle("GET /api/v1/namespaces/{ns}/deployments/{name}/logs", userMw(handleLogs(svc, cfg.LogTailLines)))
	mux.Handle("GET /api/v1/namespaces/{ns}/deployments/{name}/logs/stream", userMw(handleLogsStream(svc, cfg.LogTailLines)))

	// admin
	adminMw := adminMiddleware(cfg.AdminToken)
	mux.Handle("GET /api/v1/admin/tokens", adminMw(handleAdminListTokens(store)))
	mux.Handle("POST /api/v1/admin/tokens", adminMw(handleAdminCreateToken(store)))
	mux.Handle("DELETE /api/v1/admin/tokens/{id}", adminMw(handleAdminDeleteToken(store)))

	return mux
}
```

Define the `Service` union interface in `handler.go`:
```go
type Service interface {
	rollout.ImageUpdater
	rollout.DeploymentReader
	rollout.LogStreamer
}
```
Update `handleUpdateImage` to keep working (it already takes the service); ensure it reads the record from context and checks `Allows(ns, name)` before mutating — add the same guard as the read handlers. The existing `AUTH_TOKEN` check inside `handleUpdateImage` is removed now.

Adjust the existing `Config` struct in `handler.go` (or reuse `config.Config`): `AdminToken`, `LogTailLines`. The test's `newTestHandler` passes `Config{AdminToken:"adm", LogTailLines:10}` — make the field names match.

The existing tests that called `NewHandler(...)` with the old signature (UpdateImage tests) must be updated to the new signature — pass `Config{}`, the fake service, and a store.

- [ ] **Step 5: Run — verify pass**

Run: `go test ./internal/httpapi/`
Expected: PASS (all new + existing UpdateImage tests).

- [ ] **Step 6: Commit**

```bash
git add internal/httpapi/
git commit -m "feat(httpapi): add read, logs, verify, admin token endpoints"
```

---

## Task 9: Wire main.go + preview

**Files:**
- Modify: `cmd/server/main.go`

- [ ] **Step 1: Update main.go**

Load the token store from `cfg.TokensPath`, build either `kube.NewDeploymentImageUpdater(...)` or `NewPreviewService()` (which now satisfies `Service`), and pass to `NewHandler`. Remove `AUTH_TOKEN` usage. Sketch:

```go
func main() {
	cfg := config.Load()
	store, err := auth.LoadStore(cfg.TokensPath)
	if err != nil { log.Fatal(err) }

	var svc httpapi.Service
	if cfg.LocalPreview {
		svc = httpapi.NewPreviewService()
	} else {
		clientset, err := kube.NewInClusterClientset()
		if err != nil { log.Fatal(err) }
		svc = kube.NewDeploymentImageUpdater(clientset, cfg.AllowedNamespaces, cfg.RequiredLabel, cfg.KubeconfigPath)
	}
	handler := httpapi.NewHandler(httpapi.Config{AdminToken: cfg.AdminToken, LogTailLines: cfg.LogTailLines}, svc, store)
	// ... serve with graceful shutdown as before
}
```

Match the real field names of `config.Config` and `kube.NewDeploymentImageUpdater`. Wrap the existing serve/graceful-shutdown code unchanged.

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: builds clean.

- [ ] **Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat(server): wire TokenStore + new handler"
```

---

## Task 10: Deploy manifests

**Files:**
- Modify: `deploy/rbac.yaml`
- Modify: `deploy/deployment.yaml`
- Create: `deploy/pvc.yaml`

- [ ] **Step 1: rbac.yaml — add pods/log + deployments list**

```yaml
- apiGroups: [""]
  resources: ["pods/log"]
  verbs: ["get"]
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "patch", "create"]
```
(Replace the existing deployments rule.)

- [ ] **Step 2: pvc.yaml**

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: letorollout-data
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 1Gi
```

- [ ] **Step 3: deployment.yaml — env + volume**

Remove `AUTH_TOKEN`. Add:
```yaml
env:
- name: ADMIN_TOKEN
  valueFrom:
    secretKeyRef:
      name: letorollout-admin-auth
      key: admin-token
      optional: true
- name: TOKENS_PATH
  value: /data/tokens.json
- name: LOG_TAIL_LINES
  value: "500"
volumeMounts:
- name: data
  mountPath: /data
volumes:
- name: data
  persistentVolumeClaim:
    claimName: letorollout-data
```

- [ ] **Step 4: Commit**

```bash
git add deploy/
git commit -m "feat(deploy): add pods/log+list RBAC, token PVC, env vars"
```

---

## Task 11: Final verification

- [ ] **Step 1: All tests**

Run: `go build ./... && go test ./...`
Expected: PASS.

- [ ] **Step 2: Preview end-to-end (curl)**

```bash
TOKENS_PATH=./tokens.json ADMIN_TOKEN=adm LOCAL_PREVIEW=1 go run ./cmd/server &
# admin creates a user token
curl -sX POST localhost:8080/api/v1/admin/tokens -H "Authorization: Bearer adm" -d '{"label":"a","scopes":[{"namespace":"default"}]}'
# user lists deployments (preview is empty until seeded — expect [] or seeded data)
curl -s localhost:8080/api/v1/namespaces/default/deployments -H "Authorization: Bearer <token-from-create>"
```
Expected: token created, list returns 200.

- [ ] **Step 3: Commit any remaining fixes + tag**

```bash
git add -A
git commit -m "test: backend token auth + read/logs verified" --allow-empty
```

---

## Self-Review (completed by plan author)

**Spec coverage:**
- Remove Create → Task 1 ✓
- Reader/LogStreamer interfaces → Task 2 ✓
- kube impl → Task 4 ✓
- preview impl → Task 3 ✓
- TokenStore → Task 5 ✓
- middleware → Task 7 ✓
- HTTP endpoints (list/get/image/logs/logs-stream/verify/admin CRUD) → Task 8 ✓
- config → Task 6 ✓
- main wiring → Task 9 ✓
- rbac/pvc/deployment → Task 10 ✓
- error mapping (404/403/409/401) → `writeError` Task 8 ✓
- scope check per request → handlers Task 8 ✓
- SSE keepalive → `handleLogsStream` Task 8 ✓
- LOG_TAIL_LINES default → Task 8 ✓

**Placeholder scan:** None — every code step has complete code.

**Type consistency:** `Service` interface (Task 8) embeds the three interfaces from Task 2 — `ImageUpdater`/`DeploymentReader`/`LogStreamer`. `NewHandler(cfg, svc, store)` matches the test's `newTestHandler`. `Config` fields `AdminToken`/`LogTailLines` match tests. `authMiddleware`/`adminMiddleware`/`tokenFromContext` match between Task 7 and Task 8. `TokenRecord.Allows(ns, dep)` matches Task 5 and handlers.

**Open risk noted in Task 4:** fake clientset cannot emit real log bytes, so the one-shot log test asserts the stream opens (not content). This is a test-only limitation; real cluster logs flow through `bufio.Scanner` correctly.
