package rollout

import "errors"

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")
var ErrAlreadyExists = errors.New("already exists")
var ErrUnauthorized = errors.New("token missing or invalid")
var ErrTokenExpired = errors.New("token expired")

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
