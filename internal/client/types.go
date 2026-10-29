package client

import (
	"errors"
	"fmt"
)

// Values below use the provider's spelling. The v1 API uses the same spelling; the v2 client
// translates to and from v2's camelCase enums.

const (
	ModeSingle   = "single"
	ModeParallel = "parallel"

	StateRunning        = "running"
	StatePaused         = "paused"
	StateDraining       = "draining"
	StateSwitchingModes = "switching_modes"

	ManagedStateManual             = "manual"
	ManagedStateTerraform          = "terraform"
	ManagedStateTerraformWithDrift = "terraform_with_drift"
)

var (
	// ErrNotFound means the queue does not exist.
	ErrNotFound = errors.New("merge queue not found")
	// ErrQueueNotEmpty means a delete was refused because pull requests are still queued.
	ErrQueueNotEmpty = errors.New("merge queue still has pull requests")
	// ErrSwitchingModes means a change was refused because the queue is between modes.
	ErrSwitchingModes = errors.New("merge queue is switching modes")
	// ErrAlreadyExists means a create found a queue already on that branch.
	ErrAlreadyExists = errors.New("merge queue already exists")
	// ErrRepositoryNotFound means the API couldn't find the repository. It is not ErrNotFound:
	// the queue may well exist, so a caller must not treat it as deleted.
	ErrRepositoryNotFound = errors.New("repository not found")
	// ErrCreatedNotRecorded means a create made the queue but it couldn't be read back, so the
	// caller has nothing to record.
	ErrCreatedNotRecorded = errors.New("couldn't read it back")
	// ErrCreateUnknown means a create request failed in a way that may have created the queue,
	// and checking for it failed too.
	ErrCreateUnknown = errors.New("the merge queue may have been created")
)

// QueueReplacedError means the queue at a reference's repository and branch has a different ID
// than the reference: it was deleted and recreated outside Terraform.
type QueueReplacedError struct {
	OldID, NewID string
}

func (e *QueueReplacedError) Error() string {
	return fmt.Sprintf("the merge queue was replaced outside Terraform: it now has ID %s, not %s", e.NewID, e.OldID)
}

// Repo identifies a repository by its host, owner and name.
type Repo struct {
	Host  string
	Owner string
	Name  string
}

// QueueRef identifies a merge queue. v1 uses Repo and TargetBranch; v2 uses ID when set and
// otherwise looks the queue up from Repo and TargetBranch.
type QueueRef struct {
	ID           string
	Repo         Repo
	TargetBranch string
}

// BatchingRule changes how pull requests impacting matching targets are batched.
type BatchingRule struct {
	TargetPattern string
	Policy        string
}

// Queue is a merge queue's full state.
type Queue struct {
	// ID is the v2 queue ID. Empty on v1, which has no queue IDs.
	ID           string
	Repo         Repo
	TargetBranch string

	Mode  string
	State string
	// ManagedState says whether Terraform manages the queue. Only v2 reports it; empty on v1.
	ManagedState string

	Concurrency                 int
	BisectionConcurrency        int
	TestingTimeoutMinutes       int
	PendingFailureDepth         int
	CanOptimisticallyMerge      bool
	Batch                       bool
	BatchingMaxWaitTimeMinutes  int
	BatchingMinSize             int
	BatchingRules               []BatchingRule
	CreatePrsForTestingBranches bool
	MergeMethod                 string
	CommentsEnabled             bool
	CommandsEnabled             bool
	StatusCheckEnabled          bool
	ExtensionEnabled            bool
	LabelCommandsEnabled        bool
	StateLabelsEnabled          bool
	EnqueueingLabel             string
	DirectMergeMode             string
	OptimizationMode            string
	TestBranchConstructionMode  string
	NotReadyTimeoutHours        int
	// RequiredStatuses is the queue's override, sorted. Empty means no override: the queue uses
	// .trunk/trunk.yaml or branch protection.
	RequiredStatuses []string
}

// Settings are a queue's configurable fields. A nil field is left unchanged.
type Settings struct {
	Concurrency                 *int
	BisectionConcurrency        *int
	TestingTimeoutMinutes       *int
	PendingFailureDepth         *int
	CanOptimisticallyMerge      *bool
	Batch                       *bool
	BatchingMaxWaitTimeMinutes  *int
	BatchingMinSize             *int
	CreatePrsForTestingBranches *bool
	MergeMethod                 *string
	CommentsEnabled             *bool
	CommandsEnabled             *bool
	StatusCheckEnabled          *bool
	ExtensionEnabled            *bool
	LabelCommandsEnabled        *bool
	StateLabelsEnabled          *bool
	EnqueueingLabel             *string
	DirectMergeMode             *string
	OptimizationMode            *string
	TestBranchConstructionMode  *string
	NotReadyTimeoutHours        *int
	// RequiredStatuses replaces the override. An empty list removes it, so the queue goes back
	// to .trunk/trunk.yaml or branch protection; neither API can store an empty override.
	RequiredStatuses *[]string
	// BatchingRules replaces the rules. An empty list removes them all.
	BatchingRules *[]BatchingRule
}

// CreateParams describes a queue to create.
type CreateParams struct {
	Repo         Repo
	TargetBranch string
	Mode         *string
	// State, if set and not running, is applied right after the queue is created.
	State    *string
	Settings Settings
}

// UpdateParams describes changes to an existing queue. Mode and State are only sent when they
// differ from the queue's current values.
type UpdateParams struct {
	Ref      QueueRef
	Mode     *string
	State    *string
	Settings Settings
}
