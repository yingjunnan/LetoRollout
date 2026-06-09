# Web Console Design

## Goal

Add a built-in web console to LetoRollout for creating Deployments and updating
container images through a browser UI.

## Scope

The first version covers two workflows only:

- Create a Deployment.
- Update a container image in an existing Deployment.

The console does not provide list, delete, audit history, or auth login in this
version. Access is intentionally open and relies on network boundaries.

## Layout

The console is a single `/console` page served by the Go application.

- Left rail: recent targets, recent results, and quick template shortcuts.
- Main panel: tabbed create/update workspace.
- Header: product name, live connection indicator, and a light environment hint.

The left rail is not a live Kubernetes list. It is local UI state used for
reusing recent namespaces, deployment names, containers, and images.

## Create Flow

The create tab exposes:

- Namespace
- Name
- Image
- Environment variables

Environment variables support:

- Literal values, with `name` and `value`
- Secret key references, with `name` and `secret.name` / `secret.key`

The form validates required fields before submitting. On success, the created
Deployment summary is pushed into recent targets and recent results.

## Update Flow

The update tab exposes:

- Namespace
- Deployment
- Container
- Image
- Dry-run toggle
- Wait toggle
- Timeout seconds

The form reuses recent targets to prefill fields. On success, the updated image
summary is shown inline and also added to the recent results list.

## Interaction Model

- Switching tabs does not clear in-progress inputs.
- Clicking a recent target repopulates the active form.
- Validation errors stay inline next to the relevant field group.
- Network or API errors appear in a visible result panel with the server error
  text.
- Success feedback is immediate and retains the submitted payload summary.

## Implementation Shape

- Serve the console shell from the Go server at `/console`.
- Keep the existing API routes unchanged.
- Put browser-facing state and data submission logic in a small frontend
  module instead of inlining everything into a single file.
- Use a simple, production-oriented layout with a left rail and a compact form
  work area rather than a marketing page.

## Testing

Verify:

- `/console` loads and renders the shell.
- Create and update submissions call the existing API endpoints.
- Recent targets and results update after successful responses.
- Validation prevents invalid create submissions from being sent.
- Responsive layout remains usable on a narrow viewport.
