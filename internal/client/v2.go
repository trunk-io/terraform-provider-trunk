package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"
)

// The v2 API: REST resources under /v2, queues addressed by ID, and a scoped API key as a bearer
// token. Settings, mode and state each have their own endpoint, and enums are camelCase.

type v2BatchingRule struct {
	TargetPattern string `json:"targetPattern"`
	Policy        string `json:"policy"`
}

type v2Config struct {
	Concurrency                  *int              `json:"concurrency,omitempty"`
	BisectionConcurrency         *int              `json:"bisectionConcurrency,omitempty"`
	PendingFailureDepth          *int              `json:"pendingFailureDepth,omitempty"`
	CanOptimisticallyMerge       *bool             `json:"canOptimisticallyMerge,omitempty"`
	AllowBatching                *bool             `json:"allowBatching,omitempty"`
	BatchingMaxWaitTimeMinutes   *int              `json:"batchingMaxWaitTimeMinutes,omitempty"`
	BatchingMinSize              *int              `json:"batchingMinSize,omitempty"`
	TestingTimeoutMinutes        *int              `json:"testingTimeoutMinutes,omitempty"`
	NotReadyTimeoutMinutes       *int              `json:"notReadyTimeoutMinutes,omitempty"`
	CreatesPrsForTestingBranches *bool             `json:"createsPrsForTestingBranches,omitempty"`
	AreCommentsEnabled           *bool             `json:"areCommentsEnabled,omitempty"`
	AreCommandsEnabled           *bool             `json:"areCommandsEnabled,omitempty"`
	IsStatusCheckEnabled         *bool             `json:"isStatusCheckEnabled,omitempty"`
	IsExtensionEnabled           *bool             `json:"isExtensionEnabled,omitempty"`
	AreLabelCommandsEnabled      *bool             `json:"areLabelCommandsEnabled,omitempty"`
	AreStateLabelsEnabled        *bool             `json:"areStateLabelsEnabled,omitempty"`
	RequiredStatuses             *[]string         `json:"requiredStatuses,omitempty"`
	BatchingRules                *[]v2BatchingRule `json:"batchingRules,omitempty"`
	DirectMergeMode              *string           `json:"directMergeMode,omitempty"`
	OptimizationMode             *string           `json:"optimizationMode,omitempty"`
	MergeMethod                  *string           `json:"mergeMethod,omitempty"`
	TestBranchConstructionMode   *string           `json:"testBranchConstructionMode,omitempty"`
	EnqueueingLabel              *string           `json:"enqueueingLabel,omitempty"`
}

func (c v2Config) empty() bool {
	return c == v2Config{}
}

type v2CreateRequest struct {
	RepositoryID         string  `json:"repositoryId"`
	TargetBranch         string  `json:"targetBranch"`
	Mode                 *string `json:"mode,omitempty"`
	Concurrency          *int    `json:"concurrency,omitempty"`
	BisectionConcurrency *int    `json:"bisectionConcurrency,omitempty"`
	EnqueueingLabel      *string `json:"enqueueingLabel,omitempty"`
}

type v2Queue struct {
	ID            string `json:"id"`
	TargetBranch  string `json:"targetBranch"`
	State         string `json:"state"`
	Mode          string `json:"mode"`
	ManagedState  string `json:"managedState"`
	RepositoryURL string `json:"repositoryUrl"`
	Config        struct {
		Concurrency                  int              `json:"concurrency"`
		BisectionConcurrency         int              `json:"bisectionConcurrency"`
		PendingFailureDepth          int              `json:"pendingFailureDepth"`
		CanOptimisticallyMerge       bool             `json:"canOptimisticallyMerge"`
		AllowBatching                bool             `json:"allowBatching"`
		BatchingMaxWaitTimeMinutes   int              `json:"batchingMaxWaitTimeMinutes"`
		BatchingMinSize              int              `json:"batchingMinSize"`
		TestingTimeoutMinutes        int              `json:"testingTimeoutMinutes"`
		NotReadyTimeoutMinutes       int              `json:"notReadyTimeoutMinutes"`
		CreatesPrsForTestingBranches bool             `json:"createsPrsForTestingBranches"`
		AreCommentsEnabled           bool             `json:"areCommentsEnabled"`
		AreCommandsEnabled           bool             `json:"areCommandsEnabled"`
		IsStatusCheckEnabled         bool             `json:"isStatusCheckEnabled"`
		IsExtensionEnabled           bool             `json:"isExtensionEnabled"`
		AreLabelCommandsEnabled      bool             `json:"areLabelCommandsEnabled"`
		AreStateLabelsEnabled        bool             `json:"areStateLabelsEnabled"`
		RequiredStatuses             []string         `json:"requiredStatuses"`
		BatchingRules                []v2BatchingRule `json:"batchingRules"`
		DirectMergeMode              string           `json:"directMergeMode"`
		OptimizationMode             string           `json:"optimizationMode"`
		MergeMethod                  string           `json:"mergeMethod"`
		TestBranchConstructionMode   string           `json:"testBranchConstructionMode"`
		EnqueueingLabel              string           `json:"enqueueingLabel"`
	} `json:"config"`
}

type v2List[T any] struct {
	Data []T `json:"data"`
}

// v2 names the modes after how the queue tests: one at a time (queue) or as a dependency graph.
var (
	modeToV2   = map[string]string{ModeSingle: "queue", ModeParallel: "graph"}
	modeFromV2 = map[string]string{"queue": ModeSingle, "graph": ModeParallel}
)

// toCamel converts the provider's snake_case enum values to v2's camelCase.
func toCamel(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] != "" {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// toSnake converts v2's camelCase enum values to the provider's snake_case.
func toSnake(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsUpper(r) {
			b.WriteByte('_')
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}

func camelPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := toCamel(*s)
	return &v
}

func modeV2Ptr(s *string) *string {
	if s == nil {
		return nil
	}
	v, ok := modeToV2[*s]
	if !ok {
		v = *s
	}
	return &v
}

func hoursToMinutes(h *int) *int {
	if h == nil {
		return nil
	}
	m := *h * 60
	return &m
}

func (w *v2Queue) toQueue(repo Repo) *Queue {
	if repo.Name == "" {
		repo = repoFromURL(w.RepositoryURL)
	}
	cfg := w.Config
	rules := make([]BatchingRule, 0, len(cfg.BatchingRules))
	for _, r := range cfg.BatchingRules {
		rules = append(rules, BatchingRule(r))
	}
	statuses := cfg.RequiredStatuses
	if statuses == nil {
		statuses = []string{}
	}
	mode, ok := modeFromV2[w.Mode]
	if !ok {
		mode = w.Mode
	}
	return &Queue{
		ID:                          w.ID,
		Repo:                        repo,
		TargetBranch:                w.TargetBranch,
		Mode:                        mode,
		State:                       toSnake(w.State),
		ManagedState:                toSnake(w.ManagedState),
		Concurrency:                 cfg.Concurrency,
		BisectionConcurrency:        cfg.BisectionConcurrency,
		TestingTimeoutMinutes:       cfg.TestingTimeoutMinutes,
		PendingFailureDepth:         cfg.PendingFailureDepth,
		CanOptimisticallyMerge:      cfg.CanOptimisticallyMerge,
		Batch:                       cfg.AllowBatching,
		BatchingMaxWaitTimeMinutes:  cfg.BatchingMaxWaitTimeMinutes,
		BatchingMinSize:             cfg.BatchingMinSize,
		BatchingRules:               rules,
		CreatePrsForTestingBranches: cfg.CreatesPrsForTestingBranches,
		MergeMethod:                 toSnake(cfg.MergeMethod),
		CommentsEnabled:             cfg.AreCommentsEnabled,
		CommandsEnabled:             cfg.AreCommandsEnabled,
		StatusCheckEnabled:          cfg.IsStatusCheckEnabled,
		ExtensionEnabled:            cfg.IsExtensionEnabled,
		LabelCommandsEnabled:        cfg.AreLabelCommandsEnabled,
		StateLabelsEnabled:          cfg.AreStateLabelsEnabled,
		EnqueueingLabel:             cfg.EnqueueingLabel,
		DirectMergeMode:             toSnake(cfg.DirectMergeMode),
		OptimizationMode:            toSnake(cfg.OptimizationMode),
		TestBranchConstructionMode:  toSnake(cfg.TestBranchConstructionMode),
		NotReadyTimeoutHours:        cfg.NotReadyTimeoutMinutes / 60,
		RequiredStatuses:            statuses,
	}
}

// repoFromURL splits a repository URL such as https://github.com/acme/widgets.
func repoFromURL(raw string) Repo {
	u, err := url.Parse(raw)
	if err != nil {
		return Repo{}
	}
	owner, name, _ := strings.Cut(strings.Trim(u.Path, "/"), "/")
	return Repo{Host: u.Host, Owner: owner, Name: strings.TrimSuffix(name, ".git")}
}

func v2ConfigFrom(s Settings) v2Config {
	cfg := v2Config{
		Concurrency:                  s.Concurrency,
		BisectionConcurrency:         s.BisectionConcurrency,
		PendingFailureDepth:          s.PendingFailureDepth,
		CanOptimisticallyMerge:       s.CanOptimisticallyMerge,
		AllowBatching:                s.Batch,
		BatchingMaxWaitTimeMinutes:   s.BatchingMaxWaitTimeMinutes,
		BatchingMinSize:              s.BatchingMinSize,
		TestingTimeoutMinutes:        s.TestingTimeoutMinutes,
		NotReadyTimeoutMinutes:       hoursToMinutes(s.NotReadyTimeoutHours),
		CreatesPrsForTestingBranches: s.CreatePrsForTestingBranches,
		AreCommentsEnabled:           s.CommentsEnabled,
		AreCommandsEnabled:           s.CommandsEnabled,
		IsStatusCheckEnabled:         s.StatusCheckEnabled,
		IsExtensionEnabled:           s.ExtensionEnabled,
		AreLabelCommandsEnabled:      s.LabelCommandsEnabled,
		AreStateLabelsEnabled:        s.StateLabelsEnabled,
		// v2 reads an empty list as "remove the override", which is what Settings means by it.
		RequiredStatuses:           s.RequiredStatuses,
		DirectMergeMode:            camelPtr(s.DirectMergeMode),
		OptimizationMode:           camelPtr(s.OptimizationMode),
		MergeMethod:                camelPtr(s.MergeMethod),
		TestBranchConstructionMode: camelPtr(s.TestBranchConstructionMode),
		EnqueueingLabel:            s.EnqueueingLabel,
	}
	if s.BatchingRules != nil {
		rules := make([]v2BatchingRule, 0, len(*s.BatchingRules))
		for _, r := range *s.BatchingRules {
			rules = append(rules, v2BatchingRule(r))
		}
		cfg.BatchingRules = &rules
	}
	return cfg
}

// repositoryID looks up the v2 ID of the repository at host/owner/name. v2 matches the name
// exactly, where v1 ignores case.
func (c *Client) repositoryID(ctx context.Context, repo Repo) (string, error) {
	q := url.Values{}
	q.Set("providerUrl", fmt.Sprintf("https://%s/%s/%s", repo.Host, repo.Owner, repo.Name))
	var list v2List[struct {
		ID string `json:"id"`
	}]
	err := c.do(ctx, http.MethodGet, "/v2/repositories", q, nil, &list, retrySafe)
	if IsStatus(err, http.StatusNotFound) || (err == nil && len(list.Data) == 0) {
		return "", repositoryNotFound(repo)
	}
	if err != nil {
		return "", err
	}
	return list.Data[0].ID, nil
}

func repositoryNotFound(repo Repo) error {
	return fmt.Errorf("%w: %s/%s/%s isn't connected to Trunk, or Trunk spells its owner or name with different capitals",
		ErrRepositoryNotFound, repo.Host, repo.Owner, repo.Name)
}

func (c *Client) getV2ByID(ctx context.Context, id string, repo Repo) (*Queue, error) {
	var w v2Queue
	if err := c.do(ctx, http.MethodGet, "/v2/merge-queues/"+url.PathEscape(id), nil, nil, &w, retrySafe); err != nil {
		if IsStatus(err, http.StatusNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return w.toQueue(repo), nil
}

func (c *Client) getV2(ctx context.Context, ref QueueRef) (*Queue, error) {
	if ref.ID != "" {
		q, err := c.getV2ByID(ctx, ref.ID, ref.Repo)
		// A queue deleted and recreated outside Terraform has a new ID; find it by branch.
		if !errors.Is(err, ErrNotFound) || ref.Repo.Name == "" {
			return q, err
		}
	}
	repoID, err := c.repositoryID(ctx, ref.Repo)
	if errors.Is(err, ErrRepositoryNotFound) {
		// A configuration written for v1 may differ in case from Trunk's repository name.
		return c.findV2IgnoringCase(ctx, ref, err)
	}
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("repositoryId", repoID)
	q.Set("targetBranch", ref.TargetBranch)
	var list v2List[v2Queue]
	if err := c.do(ctx, http.MethodGet, "/v2/merge-queues", q, nil, &list, retrySafe); err != nil {
		return nil, err
	}
	if len(list.Data) == 0 {
		return nil, ErrNotFound
	}
	return list.Data[0].toQueue(ref.Repo), nil
}

// findV2IgnoringCase looks through every queue in the organization for one on ref's branch in a
// repository whose URL matches ref's apart from case. Without a match it returns notFound, not
// ErrNotFound: the repository can't be found, so the queue can't be said to be gone.
func (c *Client) findV2IgnoringCase(ctx context.Context, ref QueueRef, notFound error) (*Queue, error) {
	want := strings.ToLower(fmt.Sprintf("https://%s/%s/%s", ref.Repo.Host, ref.Repo.Owner, ref.Repo.Name))
	q := url.Values{}
	q.Set("limit", "100")
	for {
		var page struct {
			Data       []v2Queue `json:"data"`
			NextCursor *string   `json:"nextCursor"`
			HasMore    bool      `json:"hasMore"`
		}
		if err := c.do(ctx, http.MethodGet, "/v2/merge-queues", q, nil, &page, retrySafe); err != nil {
			return nil, err
		}
		for i := range page.Data {
			w := &page.Data[i]
			if strings.ToLower(strings.TrimSuffix(w.RepositoryURL, ".git")) == want && w.TargetBranch == ref.TargetBranch {
				// Keep the caller's spelling, so Terraform doesn't see a renamed repository.
				return w.toQueue(ref.Repo), nil
			}
		}
		if !page.HasMore || page.NextCursor == nil || *page.NextCursor == "" {
			return nil, notFound
		}
		q.Set("cursor", *page.NextCursor)
	}
}

func (c *Client) createV2(ctx context.Context, p CreateParams) (*Queue, error) {
	repoID, err := c.repositoryID(ctx, p.Repo)
	if err != nil {
		return nil, err
	}
	// Only fields v2 writes with the create itself. It applies bisectionConcurrency and
	// enqueueingLabel in a second write after creating the queue, so a failure there would leave
	// a queue behind an error; the PATCH below sends them instead.
	create := v2CreateRequest{
		RepositoryID: repoID,
		TargetBranch: p.TargetBranch,
		Mode:         modeV2Ptr(p.Mode),
		Concurrency:  p.Settings.Concurrency,
	}
	ref := QueueRef{Repo: p.Repo, TargetBranch: p.TargetBranch}
	var created v2Queue
	if err := c.do(ctx, http.MethodPost, "/v2/merge-queues", nil, create, &created, retryRateLimitOnly); err != nil {
		if IsStatus(err, http.StatusConflict) {
			return nil, ErrAlreadyExists
		}
		return c.createFailed(ctx, ref, err)
	}
	ref.ID = created.ID

	s := p.Settings
	s.Concurrency = nil
	if cfg := v2ConfigFrom(s); !cfg.empty() {
		if err := c.patchConfigV2(ctx, created.ID, cfg); err != nil {
			return c.afterCreate(ctx, ref, err)
		}
	}
	if p.State != nil && *p.State != StateRunning {
		if err := c.setStateV2(ctx, created.ID, *p.State); err != nil {
			return c.afterCreate(ctx, ref, err)
		}
	}
	q, err := c.getV2ByID(ctx, created.ID, p.Repo)
	if err != nil {
		return c.afterCreate(ctx, ref, err)
	}
	return q, nil
}

func (c *Client) patchConfigV2(ctx context.Context, id string, cfg v2Config) error {
	err := c.do(ctx, http.MethodPatch, "/v2/merge-queues/"+url.PathEscape(id)+"/config", nil, cfg, nil, retrySafe)
	return v2ModeConflict(err)
}

// setStateV2 moves a queue to running, paused or draining through its own endpoint.
func (c *Client) setStateV2(ctx context.Context, id, state string) error {
	action := map[string]string{StateRunning: "resume", StatePaused: "pause", StateDraining: "drain"}[state]
	if action == "" {
		return fmt.Errorf("unsupported merge queue state %q", state)
	}
	return c.do(ctx, http.MethodPost, "/v2/merge-queues/"+url.PathEscape(id)+"/"+action, nil, nil, nil, retrySafe)
}

func v2ModeConflict(err error) error {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == "MODE_CHANGE_IN_PROGRESS" {
		return fmt.Errorf("%w: run apply again once the switch finishes", ErrSwitchingModes)
	}
	return err
}

func (c *Client) updateV2(ctx context.Context, current *Queue, mode, state *string, s Settings) (*Queue, error) {
	id := current.ID
	// Always sent, even with nothing to change: a write from Terraform marks the queue
	// Terraform-managed, which clears drift the plan expects to clear.
	if err := c.patchConfigV2(ctx, id, v2ConfigFrom(s)); err != nil {
		return nil, err
	}
	// State before mode: the switch keeps whatever state the queue has when it starts, and a
	// state change during a switch would abandon it.
	if state != nil {
		if err := c.setStateV2(ctx, id, *state); err != nil {
			return nil, err
		}
	}
	if mode != nil {
		body := map[string]string{"mode": *modeV2Ptr(mode)}
		// Not retried after a server error: if the first attempt started the switch, a retry is
		// refused as MODE_CHANGE_IN_PROGRESS and would report a failure that didn't happen.
		if err := c.do(ctx, http.MethodPost, "/v2/merge-queues/"+url.PathEscape(id)+"/switch-modes", nil, body, nil, retryRateLimitOnly); err != nil {
			return nil, v2ModeConflict(err)
		}
	}
	return c.getV2ByID(ctx, id, current.Repo)
}

func (c *Client) deleteV2(ctx context.Context, ref QueueRef) error {
	q, err := c.getV2(ctx, ref)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	err = c.do(ctx, http.MethodDelete, "/v2/merge-queues/"+url.PathEscape(q.ID), nil, nil, nil, retrySafe)
	var apiErr *APIError
	switch {
	case err == nil, IsStatus(err, http.StatusNotFound):
		return nil
	case errors.As(err, &apiErr) && apiErr.Code == "QUEUE_HAS_ACTIVE_ENTRIES":
		return ErrQueueNotEmpty
	}
	return err
}
