package rollout

import "errors"

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")
var ErrAlreadyExists = errors.New("already exists")

type ImageUpdateRequest struct {
	Namespace      string `json:"namespace"`
	Deployment     string `json:"deployment"`
	Container      string `json:"container"`
	Image          string `json:"image"`
	DryRun         bool   `json:"dryRun,omitempty"`
	Wait           bool   `json:"wait,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}

type RolloutResult struct {
	Namespace       string `json:"namespace"`
	Deployment      string `json:"deployment"`
	Container       string `json:"container"`
	OldImage        string `json:"oldImage"`
	NewImage        string `json:"newImage"`
	Generation      int64  `json:"generation"`
	DryRun          bool   `json:"dryRun,omitempty"`
	RolloutComplete bool   `json:"rolloutComplete,omitempty"`
}

type DeploymentCreateRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Image     string `json:"image"`
}

type DeploymentCreateResult struct {
	Namespace  string            `json:"namespace"`
	Name       string            `json:"name"`
	Container  string            `json:"container"`
	Image      string            `json:"image"`
	Replicas   int32             `json:"replicas"`
	Generation int64             `json:"generation"`
	Labels     map[string]string `json:"labels,omitempty"`
}
