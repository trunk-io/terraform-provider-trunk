package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// The v1 API: POST endpoints at /v1/<operation>, a queue addressed by repo and branch, and the
// organization API token in x-api-token.

type v1Repo struct {
	Host  string `json:"host"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

func toV1Repo(r Repo) v1Repo {
	return v1Repo(r)
}

type v1QueueRef struct {
	Repo         v1Repo `json:"repo"`
	TargetBranch string `json:"targetBranch"`
}

type v1BatchingRule struct {
	TargetPattern string `json:"targetPattern"`
	Policy        string `json:"policy"`
}

type v1CreateRequest struct {
	Repo         v1Repo  `json:"repo"`
	TargetBranch string  `json:"targetBranch"`
	Mode         *string `json:"mode,omitempty"`
	Concurrency  *int    `json:"concurrency,omitempty"`
}

type v1UpdateRequest struct {
	Repo         v1Repo `json:"repo"`
	TargetBranch string `json:"targetBranch"`

	Mode                        *string           `json:"mode,omitempty"`
	State                       *string           `json:"state,omitempty"`
	Concurrency                 *int              `json:"concurrency,omitempty"`
	BisectionConcurrency        *int              `json:"bisectionConcurrency,omitempty"`
	TestingTimeoutMinutes       *int              `json:"testingTimeoutMinutes,omitempty"`
	PendingFailureDepth         *int              `json:"pendingFailureDepth,omitempty"`
	CanOptimisticallyMerge      *bool             `json:"canOptimisticallyMerge,omitempty"`
	Batch                       *bool             `json:"batch,omitempty"`
	BatchingMaxWaitTimeMinutes  *int              `json:"batchingMaxWaitTimeMinutes,omitempty"`
	BatchingMinSize             *int              `json:"batchingMinSize,omitempty"`
	BatchingRules               *[]v1BatchingRule `json:"batchingRules,omitempty"`
	CreatePrsForTestingBranches *bool             `json:"createPrsForTestingBranches,omitempty"`
	MergeMethod                 *string           `json:"mergeMethod,omitempty"`
	CommentsEnabled             *bool             `json:"commentsEnabled,omitempty"`
	CommandsEnabled             *bool             `json:"commandsEnabled,omitempty"`
	StatusCheckEnabled          *bool             `json:"statusCheckEnabled,omitempty"`
	ExtensionEnabled            *bool             `json:"extensionEnabled,omitempty"`
	LabelCommandsEnabled        *bool             `json:"labelCommandsEnabled,omitempty"`
	StateLabelsEnabled          *bool             `json:"stateLabelsEnabled,omitempty"`
	EnqueueingLabel             *string           `json:"enqueueingLabel,omitempty"`
	DirectMergeMode             *string           `json:"directMergeMode,omitempty"`
	OptimizationMode            *string           `json:"optimizationMode,omitempty"`
	TestBranchConstructionMode  *string           `json:"testBranchConstructionMode,omitempty"`
	NotReadyTimeoutHours        *int              `json:"notReadyTimeoutHours,omitempty"`
	RequiredStatuses            *[]string         `json:"requiredStatuses,omitempty"`
	// v1 reads an empty requiredStatuses as "not sent", so removing the override takes this flag.
	DeleteRequiredStatuses *bool `json:"deleteRequiredStatuses,omitempty"`
}

// v1Queue is the body of getQueue and updateQueue. Neither includes the repo or branch.
type v1Queue struct {
	State                       string           `json:"state"`
	Mode                        string           `json:"mode"`
	Concurrency                 int              `json:"concurrency"`
	BisectionConcurrency        int              `json:"bisectionConcurrency"`
	TestingTimeoutMinutes       int              `json:"testingTimeoutMinutes"`
	PendingFailureDepth         int              `json:"pendingFailureDepth"`
	CanOptimisticallyMerge      bool             `json:"canOptimisticallyMerge"`
	Batch                       bool             `json:"batch"`
	BatchingMaxWaitTimeMinutes  int              `json:"batchingMaxWaitTimeMinutes"`
	BatchingMinSize             int              `json:"batchingMinSize"`
	BatchingRules               []v1BatchingRule `json:"batchingRules"`
	CreatePrsForTestingBranches bool             `json:"createPrsForTestingBranches"`
	MergeMethod                 string           `json:"mergeMethod"`
	CommentsEnabled             bool             `json:"commentsEnabled"`
	CommandsEnabled             bool             `json:"commandsEnabled"`
	StatusCheckEnabled          bool             `json:"statusCheckEnabled"`
	ExtensionEnabled            bool             `json:"extensionEnabled"`
	LabelCommandsEnabled        bool             `json:"labelCommandsEnabled"`
	StateLabelsEnabled          bool             `json:"stateLabelsEnabled"`
	EnqueueingLabel             string           `json:"enqueueingLabel"`
	DirectMergeMode             string           `json:"directMergeMode"`
	OptimizationMode            string           `json:"optimizationMode"`
	TestBranchConstructionMode  string           `json:"testBranchConstructionMode"`
	NotReadyTimeoutHours        int              `json:"notReadyTimeoutHours"`
	RequiredStatuses            []string         `json:"requiredStatuses"`
}

func (w *v1Queue) toQueue(ref QueueRef) *Queue {
	rules := make([]BatchingRule, 0, len(w.BatchingRules))
	for _, r := range w.BatchingRules {
		rules = append(rules, BatchingRule(r))
	}
	statuses := w.RequiredStatuses
	if statuses == nil {
		statuses = []string{}
	}
	return &Queue{
		Repo:                        ref.Repo,
		TargetBranch:                ref.TargetBranch,
		Mode:                        strings.ToLower(w.Mode),
		State:                       strings.ToLower(w.State),
		Concurrency:                 w.Concurrency,
		BisectionConcurrency:        w.BisectionConcurrency,
		TestingTimeoutMinutes:       w.TestingTimeoutMinutes,
		PendingFailureDepth:         w.PendingFailureDepth,
		CanOptimisticallyMerge:      w.CanOptimisticallyMerge,
		Batch:                       w.Batch,
		BatchingMaxWaitTimeMinutes:  w.BatchingMaxWaitTimeMinutes,
		BatchingMinSize:             w.BatchingMinSize,
		BatchingRules:               rules,
		CreatePrsForTestingBranches: w.CreatePrsForTestingBranches,
		MergeMethod:                 strings.ToLower(w.MergeMethod),
		CommentsEnabled:             w.CommentsEnabled,
		CommandsEnabled:             w.CommandsEnabled,
		StatusCheckEnabled:          w.StatusCheckEnabled,
		ExtensionEnabled:            w.ExtensionEnabled,
		LabelCommandsEnabled:        w.LabelCommandsEnabled,
		StateLabelsEnabled:          w.StateLabelsEnabled,
		EnqueueingLabel:             w.EnqueueingLabel,
		DirectMergeMode:             strings.ToLower(w.DirectMergeMode),
		OptimizationMode:            strings.ToLower(w.OptimizationMode),
		TestBranchConstructionMode:  strings.ToLower(w.TestBranchConstructionMode),
		NotReadyTimeoutHours:        w.NotReadyTimeoutHours,
		RequiredStatuses:            statuses,
	}
}

// v1Unauthorized explains a 401, which v1 also returns for a repository it can't find.
func v1Unauthorized(err error, repo Repo) error {
	if IsStatus(err, http.StatusUnauthorized) {
		return fmt.Errorf("the organization API token was refused, or %s/%s/%s is not connected to Trunk: %w",
			repo.Host, repo.Owner, repo.Name, err)
	}
	return err
}

func v1Ref(ref QueueRef) v1QueueRef {
	return v1QueueRef{Repo: toV1Repo(ref.Repo), TargetBranch: ref.TargetBranch}
}

func (c *Client) getV1(ctx context.Context, ref QueueRef) (*Queue, error) {
	var w v1Queue
	if err := c.do(ctx, http.MethodPost, "/v1/getQueue", nil, v1Ref(ref), &w, retrySafe); err != nil {
		if IsStatus(err, http.StatusNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return w.toQueue(ref), nil
}

func (c *Client) createV1(ctx context.Context, p CreateParams) (*Queue, error) {
	ref := QueueRef{Repo: p.Repo, TargetBranch: p.TargetBranch}
	create := v1CreateRequest{
		Repo:         toV1Repo(p.Repo),
		TargetBranch: p.TargetBranch,
		Mode:         p.Mode,
		Concurrency:  p.Settings.Concurrency,
	}
	if err := c.do(ctx, http.MethodPost, "/v1/createQueue", nil, create, nil, retryRateLimitOnly); err != nil {
		if IsStatus(err, http.StatusConflict) {
			return nil, ErrAlreadyExists
		}
		return nil, v1Unauthorized(err, p.Repo)
	}

	// createQueue takes only mode and concurrency; everything else is an update. This update
	// always goes out, even with nothing to change, because its response is the queue's state.
	settings := p.Settings
	settings.Concurrency = nil
	update := v1UpdateFrom(ref, nil, nil, settings)
	if p.State != nil && *p.State != StateRunning {
		update.State = p.State
	}
	var w v1Queue
	if err := c.do(ctx, http.MethodPost, "/v1/updateQueue", nil, update, &w, retrySafe); err != nil {
		return c.afterCreate(ctx, ref, err)
	}
	return w.toQueue(ref), nil
}

func (c *Client) updateV1(ctx context.Context, current *Queue, mode, state *string, s Settings) (*Queue, error) {
	ref := QueueRef{Repo: current.Repo, TargetBranch: current.TargetBranch}
	var w v1Queue
	if err := c.do(ctx, http.MethodPost, "/v1/updateQueue", nil, v1UpdateFrom(ref, mode, state, s), &w, retrySafe); err != nil {
		if IsStatus(err, http.StatusNotFound) {
			return nil, ErrNotFound
		}
		return nil, v1Unauthorized(err, ref.Repo)
	}
	return w.toQueue(ref), nil
}

func v1UpdateFrom(ref QueueRef, mode, state *string, s Settings) v1UpdateRequest {
	req := v1UpdateRequest{
		Repo:                        toV1Repo(ref.Repo),
		TargetBranch:                ref.TargetBranch,
		Mode:                        mode,
		State:                       state,
		Concurrency:                 s.Concurrency,
		BisectionConcurrency:        s.BisectionConcurrency,
		TestingTimeoutMinutes:       s.TestingTimeoutMinutes,
		PendingFailureDepth:         s.PendingFailureDepth,
		CanOptimisticallyMerge:      s.CanOptimisticallyMerge,
		Batch:                       s.Batch,
		BatchingMaxWaitTimeMinutes:  s.BatchingMaxWaitTimeMinutes,
		BatchingMinSize:             s.BatchingMinSize,
		CreatePrsForTestingBranches: s.CreatePrsForTestingBranches,
		MergeMethod:                 s.MergeMethod,
		CommentsEnabled:             s.CommentsEnabled,
		CommandsEnabled:             s.CommandsEnabled,
		StatusCheckEnabled:          s.StatusCheckEnabled,
		ExtensionEnabled:            s.ExtensionEnabled,
		LabelCommandsEnabled:        s.LabelCommandsEnabled,
		StateLabelsEnabled:          s.StateLabelsEnabled,
		EnqueueingLabel:             s.EnqueueingLabel,
		DirectMergeMode:             s.DirectMergeMode,
		OptimizationMode:            s.OptimizationMode,
		TestBranchConstructionMode:  s.TestBranchConstructionMode,
		NotReadyTimeoutHours:        s.NotReadyTimeoutHours,
	}
	if s.RequiredStatuses != nil {
		if len(*s.RequiredStatuses) == 0 {
			t := true
			req.DeleteRequiredStatuses = &t
		} else {
			req.RequiredStatuses = s.RequiredStatuses
		}
	}
	if s.BatchingRules != nil {
		rules := make([]v1BatchingRule, 0, len(*s.BatchingRules))
		for _, r := range *s.BatchingRules {
			rules = append(rules, v1BatchingRule(r))
		}
		req.BatchingRules = &rules
	}
	return req
}

func (c *Client) deleteV1(ctx context.Context, ref QueueRef) error {
	err := c.do(ctx, http.MethodPost, "/v1/deleteQueue", nil, v1Ref(ref), nil, retrySafe)
	switch {
	case err == nil, IsStatus(err, http.StatusNotFound):
		return nil
	case IsStatus(err, http.StatusBadRequest) && strings.Contains(strings.ToLower(err.Error()), "not empty"):
		return ErrQueueNotEmpty
	}
	return v1Unauthorized(err, ref.Repo)
}
