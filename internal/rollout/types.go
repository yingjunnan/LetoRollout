package rollout

import "errors"

var ErrNotFound = errors.New("not found")

type ImageUpdateRequest struct {
	Namespace  string `json:"namespace"`
	Deployment string `json:"deployment"`
	Container  string `json:"container"`
	Image      string `json:"image"`
}

type RolloutResult struct {
	Namespace  string `json:"namespace"`
	Deployment string `json:"deployment"`
	Container  string `json:"container"`
	OldImage   string `json:"oldImage"`
	NewImage   string `json:"newImage"`
	Generation int64  `json:"generation"`
}
