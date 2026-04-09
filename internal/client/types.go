package client

import "fmt"

// Repo identifies a repository in the Trunk API.
type Repo struct {
	Host  string `json:"host"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

// Queue represents the full state of a Trunk merge queue as returned by the
// getQueue and updateQueue APIs. All fields use lowercase enum values to match
// the API (e.g., "running", "single", "squash", "off").
type Queue struct {
	Repo         Repo   `json:"repo"`
	TargetBranch string `json:"targetBranch"`
	Mode         string `json:"mode"`
	Concurrency  int    `json:"concurrency"`
	State        string `json:"state"`

	TestingTimeoutMinutes       int    `json:"testingTimeoutMinutes"`
	PendingFailureDepth         int    `json:"pendingFailureDepth"`
	CanOptimisticallyMerge      bool   `json:"canOptimisticallyMerge"`
	Batch                       bool   `json:"batch"`
	BatchingMaxWaitTimeMinutes  int    `json:"batchingMaxWaitTimeMinutes"`
	BatchingMinSize             int    `json:"batchingMinSize"`
	CreatePrsForTestingBranches bool   `json:"createPrsForTestingBranches"`
	MergeMethod                 string `json:"mergeMethod"`
	CommentsEnabled             bool   `json:"commentsEnabled"`
	CommandsEnabled             bool   `json:"commandsEnabled"`
	StatusCheckEnabled          bool   `json:"statusCheckEnabled"`
	DirectMergeMode             string `json:"directMergeMode"`
	OptimizationMode            string `json:"optimizationMode"`
	BisectionConcurrency        int    `json:"bisectionConcurrency"`

	// RequiredStatuses is null when no manual override is set (uses branch protection / trunk.yaml defaults).
	RequiredStatuses *[]string `json:"requiredStatuses"`
}

// APIError represents a non-2xx response from the Trunk API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Body)
}

// CreateQueueRequest contains the fields accepted by the createQueue endpoint.
// Per the API contract, only identity fields, mode, and concurrency are accepted at
// create time; all other configuration must be applied via UpdateQueue.
type CreateQueueRequest struct {
	Repo         Repo   `json:"repo"`
	TargetBranch string `json:"targetBranch"`
	Mode         string `json:"mode,omitempty"`
	Concurrency  int    `json:"concurrency,omitempty"`
}

// GetQueueRequest identifies the queue to retrieve.
type GetQueueRequest struct {
	Repo         Repo   `json:"repo"`
	TargetBranch string `json:"targetBranch"`
}

// UpdateQueueRequest contains all fields that can be changed on an existing queue.
// Nil pointer fields are omitted from the JSON body, leaving them unchanged on the API side.
type UpdateQueueRequest struct {
	Repo         Repo   `json:"repo"`
	TargetBranch string `json:"targetBranch"`

	Mode        *string `json:"mode,omitempty"`
	Concurrency *int    `json:"concurrency,omitempty"`
	State       *string `json:"state,omitempty"`

	TestingTimeoutMinutes       *int      `json:"testingTimeoutMinutes,omitempty"`
	PendingFailureDepth         *int      `json:"pendingFailureDepth,omitempty"`
	CanOptimisticallyMerge      *bool     `json:"canOptimisticallyMerge,omitempty"`
	Batch                       *bool     `json:"batch,omitempty"`
	BatchingMaxWaitTimeMinutes  *int      `json:"batchingMaxWaitTimeMinutes,omitempty"`
	BatchingMinSize             *int      `json:"batchingMinSize,omitempty"`
	MergeMethod                 *string   `json:"mergeMethod,omitempty"`
	CommentsEnabled             *bool     `json:"commentsEnabled,omitempty"`
	CommandsEnabled             *bool     `json:"commandsEnabled,omitempty"`
	CreatePrsForTestingBranches *bool     `json:"createPrsForTestingBranches,omitempty"`
	StatusCheckEnabled          *bool     `json:"statusCheckEnabled,omitempty"`
	DirectMergeMode             *string   `json:"directMergeMode,omitempty"`
	OptimizationMode            *string   `json:"optimizationMode,omitempty"`
	BisectionConcurrency        *int      `json:"bisectionConcurrency,omitempty"`
	RequiredStatuses            *[]string `json:"requiredStatuses,omitempty"`

	// DeleteRequiredStatuses reverts required statuses to branch protection / trunk.yaml defaults
	// when set to true.
	DeleteRequiredStatuses *bool `json:"deleteRequiredStatuses,omitempty"`
}

// DeleteQueueRequest identifies the queue to delete.
type DeleteQueueRequest struct {
	Repo         Repo   `json:"repo"`
	TargetBranch string `json:"targetBranch"`
}
