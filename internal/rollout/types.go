package rollout

import (
	"context"
	"errors"
)

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

// ContainerInfo describes one container of a Deployment.
type ContainerInfo struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

// DeploymentSummary is a list item.
type DeploymentSummary struct {
	Name          string          `json:"name"`
	Namespace     string          `json:"namespace"`
	Replicas      int32           `json:"replicas"`
	ReadyReplicas int32           `json:"readyReplicas"`
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

// ImageUpdater updates a Deployment's container image.
type ImageUpdater interface {
	UpdateImage(ctx context.Context, req ImageUpdateRequest) (RolloutResult, error)
}

// DeploymentReader reads Deployment summaries and details.
type DeploymentReader interface {
	ListDeployments(ctx context.Context, namespace string) ([]DeploymentSummary, error)
	GetDeployment(ctx context.Context, namespace, name string) (DeploymentDetail, error)
}

// LogStreamer streams a Deployment's Pod logs.
type LogStreamer interface {
	StreamLogs(ctx context.Context, req LogRequest) (<-chan LogLine, error)
}

