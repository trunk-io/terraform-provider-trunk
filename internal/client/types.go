package client

import "fmt"

// Repo identifies a repository in the Trunk API.
type Repo struct {
	Host  string `json:"host"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

// Queue represents the full state of a Trunk merge queue as returned by the API.
type Queue struct {
	Repo         Repo   `json:"repo"`
	TargetBranch string `json:"targetBranch"`
	Mode         string `json:"mode"`
	Concurrency  int    `json:"concurrency"`
	State        string `json:"state"`

	// Optional configuration fields.
	TestingTimeoutMinutes       *int     `json:"testingTimeoutMinutes,omitempty"`
	PendingFailureDepth         *int     `json:"pendingFailureDepth,omitempty"`
	CanOptimisticallyMerge      *bool    `json:"canOptimisticallyMerge,omitempty"`
	Batch                       *bool    `json:"batch,omitempty"`
	BatchingMaxWaitTimeMinutes  *int     `json:"batchingMaxWaitTimeMinutes,omitempty"`
	BatchingMinSize             *int     `json:"batchingMinSize,omitempty"`
	MergeMethod                 *string  `json:"mergeMethod,omitempty"`
	CommentsEnabled             *bool    `json:"commentsEnabled,omitempty"`
	CommandsEnabled             *bool    `json:"commandsEnabled,omitempty"`
	CreatePrsForTestingBranches *bool    `json:"createPrsForTestingBranches,omitempty"`
	StatusCheckEnabled          *bool    `json:"statusCheckEnabled,omitempty"`
	DirectMergeMode             *string  `json:"directMergeMode,omitempty"`
	OptimizationMode            *string  `json:"optimizationMode,omitempty"`
	BisectionConcurrency        *int     `json:"bisectionConcurrency,omitempty"`
	RequiredStatuses            []string `json:"requiredStatuses,omitempty"`
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
//
// Mode and Concurrency use value types (not pointers) because:
//   - The Create method in the Terraform resource always supplies explicit values for both fields.
//   - The API treats omitted fields as defaults (mode="single", concurrency=1), so dropping a
//     zero value via omitempty is safe — a zero concurrency is invalid for a queue anyway.
//
// This is intentionally different from UpdateQueueRequest, where pointer fields are required
// to distinguish "leave unchanged" (nil) from "set to default" (non-nil zero).
type CreateQueueRequest struct {
	Repo         Repo   `json:"repo"`
	TargetBranch string `json:"targetBranch"`
	Mode         string `json:"mode,omitempty"`
	Concurrency  int    `json:"concurrency,omitempty"`
}

// CreateQueueResponse is returned by the createQueue endpoint.
type CreateQueueResponse struct {
	Queue Queue `json:"queue"`
}

// GetQueueRequest identifies the queue to retrieve.
type GetQueueRequest struct {
	Repo         Repo   `json:"repo"`
	TargetBranch string `json:"targetBranch"`
}

// getQueueAPIResponse matches the flat JSON structure returned by the getQueue endpoint.
// Several field names differ from the internal Queue type:
//   - branch → TargetBranch
//   - testingTimeoutMins → TestingTimeoutMinutes
//   - isBatching → Batch
//   - batchingMaxWaitTimeMins → BatchingMaxWaitTimeMinutes
//   - mode is uppercase ("SINGLE"/"PARALLEL") and is normalized to lowercase by toQueue
type getQueueAPIResponse struct {
	Branch      string `json:"branch"`
	Concurrency int    `json:"concurrency"`
	Mode        string `json:"mode"`
	State       string `json:"state"`

	TestingTimeoutMins          *int     `json:"testingTimeoutMins,omitempty"`
	PendingFailureDepth         *int     `json:"pendingFailureDepth,omitempty"`
	CanOptimisticallyMerge      *bool    `json:"canOptimisticallyMerge,omitempty"`
	IsBatching                  *bool    `json:"isBatching,omitempty"`
	BatchingMaxWaitTimeMins     *int     `json:"batchingMaxWaitTimeMins,omitempty"`
	BatchingMinSize             *int     `json:"batchingMinSize,omitempty"`
	MergeMethod                 *string  `json:"mergeMethod,omitempty"`
	CommentsEnabled             *bool    `json:"commentsEnabled,omitempty"`
	CommandsEnabled             *bool    `json:"commandsEnabled,omitempty"`
	CreatePrsForTestingBranches *bool    `json:"createPrsForTestingBranches,omitempty"`
	StatusCheckEnabled          *bool    `json:"statusCheckEnabled,omitempty"`
	DirectMergeMode             *string  `json:"directMergeMode,omitempty"`
	OptimizationMode            *string  `json:"optimizationMode,omitempty"`
	BisectionConcurrency        *int     `json:"bisectionConcurrency,omitempty"`
	RequiredStatuses            []string `json:"requiredStatuses,omitempty"`
}

// toQueue maps the flat API response to the internal Queue type, normalizing field
// names and mode casing.
func (r *getQueueAPIResponse) toQueue(repo Repo, targetBranch string) *Queue {
	q := &Queue{
		Repo:         repo,
		TargetBranch: targetBranch,
		Concurrency:  r.Concurrency,
		State:        r.State,

		PendingFailureDepth:         r.PendingFailureDepth,
		CanOptimisticallyMerge:      r.CanOptimisticallyMerge,
		BatchingMinSize:             r.BatchingMinSize,
		MergeMethod:                 r.MergeMethod,
		CommentsEnabled:             r.CommentsEnabled,
		CommandsEnabled:             r.CommandsEnabled,
		CreatePrsForTestingBranches: r.CreatePrsForTestingBranches,
		StatusCheckEnabled:          r.StatusCheckEnabled,
		DirectMergeMode:             r.DirectMergeMode,
		OptimizationMode:            r.OptimizationMode,
		BisectionConcurrency:        r.BisectionConcurrency,
		RequiredStatuses:            r.RequiredStatuses,

		// Renamed fields.
		TestingTimeoutMinutes:      r.TestingTimeoutMins,
		Batch:                      r.IsBatching,
		BatchingMaxWaitTimeMinutes: r.BatchingMaxWaitTimeMins,
	}
	// The API returns uppercase mode values; normalize to lowercase for the schema.
	switch r.Mode {
	case "SINGLE":
		q.Mode = "single"
	case "PARALLEL":
		q.Mode = "parallel"
	default:
		q.Mode = r.Mode
	}
	return q
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

// UpdateQueueResponse is returned by the updateQueue endpoint.
type UpdateQueueResponse struct {
	Queue Queue `json:"queue"`
}

// DeleteQueueRequest identifies the queue to delete.
type DeleteQueueRequest struct {
	Repo         Repo   `json:"repo"`
	TargetBranch string `json:"targetBranch"`
}

// DeleteQueueResponse is the empty response from the deleteQueue endpoint.
type DeleteQueueResponse struct{}
