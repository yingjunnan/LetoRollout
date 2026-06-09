# Web Console Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a built-in `/console` page to LetoRollout for creating Deployments and updating container images with a polished, usable browser UI.

**Architecture:** Serve a static console shell from the existing Go HTTP server using `embed`. Build the UI with vanilla HTML/CSS/JS so it stays lightweight, has no build step, and can call the existing JSON API endpoints directly. Persist recent targets/results and form drafts in `localStorage` to make repeated rollout operations faster.

**Tech Stack:** Go 1.20, `net/http`, `embed`, vanilla HTML/CSS/JavaScript, `localStorage`

---

### Task 1: Console Routes and Embedded Assets

**Files:**
- Modify: `internal/httpapi/handler.go`
- Create: `internal/httpapi/static/console.html`
- Create: `internal/httpapi/static/app.css`
- Create: `internal/httpapi/static/app.js`
- Modify: `internal/httpapi/handler_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests that expect:

```go
GET /              -> 301 redirect to /console/
GET /console       -> 301 redirect to /console/
GET /console/      -> 200 HTML shell containing "LetoRollout Console"
GET /console/app.js -> 200 JavaScript content with "fetch" and "localStorage"
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/httpapi -run 'TestRootRedirectsToConsole|TestConsoleRedirectsToTrailingSlash|TestConsoleServesShell|TestConsoleServesJavaScriptAsset' -v`
Expected: FAIL with 404 responses before the route exists.

- [ ] **Step 3: Implement the minimal route + asset serving**

Add an embedded `static/` filesystem, redirect `/` and `/console` to `/console/`, and serve the console HTML plus static JS/CSS from `/console/...`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/httpapi -run 'TestRootRedirectsToConsole|TestConsoleRedirectsToTrailingSlash|TestConsoleServesShell|TestConsoleServesJavaScriptAsset' -v`
Expected: PASS.

### Task 2: Console UI and Browser Logic

**Files:**
- Create: `internal/httpapi/static/console.html`
- Create: `internal/httpapi/static/app.css`
- Create: `internal/httpapi/static/app.js`

- [ ] **Step 1: Implement the shell markup**

Build a two-column workbench with:

```html
header, left rail, create tab, update tab, result panel
```

The left rail should show recent targets, recent results, and templates. The main panel should keep create and update forms in separate tabs.

- [ ] **Step 2: Implement stateful form behavior**

Use vanilla JS to:

```js
loadState();
render();
handleCreateSubmit();
handleUpdateSubmit();
saveRecentTargets();
saveRecentResults();
```

The create form must support literal env values and Secret key references. Inline validation should block invalid submissions.

- [ ] **Step 3: Implement polished styling**

Use a restrained operational palette, compact spacing, clear form groups, chip-style recent items, visible result states, and responsive layout that collapses cleanly on narrow screens.

### Task 3: Documentation and Verification

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update the README**

Document the `/console` page, the open-access assumption, and the main operations available in the UI.

- [ ] **Step 2: Run full verification**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 3: Check the console in a browser**

Open `/console/` locally and verify the create/update workflows, recent items, inline validation, and result panel behavior.
