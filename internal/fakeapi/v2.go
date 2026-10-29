package fakeapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

var (
	modeToV2   = map[string]string{"single": "queue", "parallel": "graph"}
	modeFromV2 = map[string]string{"queue": "single", "graph": "parallel"}
)

func camel(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}

func snake(s string) string {
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

func (s *Server) v2Body(q *Queue) map[string]any {
	return map[string]any{
		"id": q.ID, "repositoryId": s.repos[q.Host+"/"+q.Owner+"/"+q.Name], "targetBranch": q.Branch,
		"state": camel(q.State), "mode": modeToV2[q.Mode], "managedState": camel(q.ManagedState),
		"repositoryUrl": fmt.Sprintf("https://%s/%s/%s", q.Host, q.Owner, q.Name),
		"config": map[string]any{
			"concurrency": q.Concurrency, "bisectionConcurrency": q.BisectionConcurrency,
			"pendingFailureDepth": q.PendingFailureDepth, "canOptimisticallyMerge": q.CanOptimisticallyMerge,
			"allowBatching": q.Batch, "batchingMaxWaitTimeMinutes": q.BatchingMaxWaitTimeMinutes,
			"batchingMinSize": q.BatchingMinSize, "testingTimeoutMinutes": q.TestingTimeoutMinutes,
			"notReadyTimeoutMinutes": q.NotReadyTimeoutHours * 60, "createsPrsForTestingBranches": q.CreatePrsForTestingBranches,
			"areCommentsEnabled": q.CommentsEnabled, "areCommandsEnabled": q.CommandsEnabled,
			"isStatusCheckEnabled": q.StatusCheckEnabled, "isExtensionEnabled": q.ExtensionEnabled,
			"areLabelCommandsEnabled": q.LabelCommandsEnabled, "areStateLabelsEnabled": q.StateLabelsEnabled,
			"requiredStatuses": q.RequiredStatuses, "allowedBotSubmitters": []string{}, "batchingRules": q.BatchingRules,
			"directMergeMode": q.DirectMergeMode, "optimizationMode": camel(q.OptimizationMode),
			"mergeMethod": camel(q.MergeMethod), "testBranchConstructionMode": camel(q.TestBranchConstructionMode),
			"enqueueingLabel": q.EnqueueingLabel,
		},
	}
}

type v2Config struct {
	Concurrency                  *int            `json:"concurrency"`
	BisectionConcurrency         *int            `json:"bisectionConcurrency"`
	PendingFailureDepth          *int            `json:"pendingFailureDepth"`
	CanOptimisticallyMerge       *bool           `json:"canOptimisticallyMerge"`
	AllowBatching                *bool           `json:"allowBatching"`
	BatchingMaxWaitTimeMinutes   *int            `json:"batchingMaxWaitTimeMinutes"`
	BatchingMinSize              *int            `json:"batchingMinSize"`
	TestingTimeoutMinutes        *int            `json:"testingTimeoutMinutes"`
	NotReadyTimeoutMinutes       *int            `json:"notReadyTimeoutMinutes"`
	CreatesPrsForTestingBranches *bool           `json:"createsPrsForTestingBranches"`
	AreCommentsEnabled           *bool           `json:"areCommentsEnabled"`
	AreCommandsEnabled           *bool           `json:"areCommandsEnabled"`
	IsStatusCheckEnabled         *bool           `json:"isStatusCheckEnabled"`
	IsExtensionEnabled           *bool           `json:"isExtensionEnabled"`
	AreLabelCommandsEnabled      *bool           `json:"areLabelCommandsEnabled"`
	AreStateLabelsEnabled        *bool           `json:"areStateLabelsEnabled"`
	RequiredStatuses             *[]string       `json:"requiredStatuses"`
	AllowedBotSubmitters         *[]string       `json:"allowedBotSubmitters"`
	BatchingRules                *[]BatchingRule `json:"batchingRules"`
	DirectMergeMode              *string         `json:"directMergeMode"`
	OptimizationMode             *string         `json:"optimizationMode"`
	MergeMethod                  *string         `json:"mergeMethod"`
	TestBranchConstructionMode   *string         `json:"testBranchConstructionMode"`
	EnqueueingLabel              *string         `json:"enqueueingLabel"`
}

func inRange(v *int, lo, hi int) bool {
	return v == nil || (*v >= lo && *v <= hi)
}

func snakePtr(v *string) *string {
	if v == nil {
		return nil
	}
	s := snake(*v)
	return &s
}

func applyV2(q *Queue, c *v2Config) string {
	switch {
	case !inRange(c.Concurrency, 1, 1000), !inRange(c.BisectionConcurrency, 0, 1000), !inRange(c.PendingFailureDepth, 0, 1000),
		!inRange(c.BatchingMinSize, 1, 1000), !inRange(c.BatchingMaxWaitTimeMinutes, 0, 10080),
		!inRange(c.TestingTimeoutMinutes, 0, 10080), !inRange(c.NotReadyTimeoutMinutes, 0, 43200):
		return "a value is out of range"
	case c.NotReadyTimeoutMinutes != nil && *c.NotReadyTimeoutMinutes%60 != 0:
		return "notReadyTimeoutMinutes must be a whole number of hours"
	case c.RequiredStatuses != nil && len(*c.RequiredStatuses) > 100:
		return "requiredStatuses takes at most 100 entries"
	}
	if c.RequiredStatuses != nil {
		for _, st := range *c.RequiredStatuses {
			if st == "" {
				return "requiredStatuses entries can't be empty"
			}
		}
	}
	setInt(&q.Concurrency, c.Concurrency)
	setInt(&q.BisectionConcurrency, c.BisectionConcurrency)
	setInt(&q.PendingFailureDepth, c.PendingFailureDepth)
	setBool(&q.CanOptimisticallyMerge, c.CanOptimisticallyMerge)
	setBool(&q.Batch, c.AllowBatching)
	setInt(&q.BatchingMaxWaitTimeMinutes, c.BatchingMaxWaitTimeMinutes)
	setInt(&q.BatchingMinSize, c.BatchingMinSize)
	setInt(&q.TestingTimeoutMinutes, c.TestingTimeoutMinutes)
	if c.NotReadyTimeoutMinutes != nil {
		q.NotReadyTimeoutHours = *c.NotReadyTimeoutMinutes / 60
	}
	setBool(&q.CreatePrsForTestingBranches, c.CreatesPrsForTestingBranches)
	setBool(&q.CommentsEnabled, c.AreCommentsEnabled)
	setBool(&q.CommandsEnabled, c.AreCommandsEnabled)
	setBool(&q.StatusCheckEnabled, c.IsStatusCheckEnabled)
	setBool(&q.ExtensionEnabled, c.IsExtensionEnabled)
	setBool(&q.LabelCommandsEnabled, c.AreLabelCommandsEnabled)
	setBool(&q.StateLabelsEnabled, c.AreStateLabelsEnabled)
	if c.RequiredStatuses != nil {
		q.RequiredStatuses = sorted(*c.RequiredStatuses)
	}
	if c.BatchingRules != nil {
		q.BatchingRules = append([]BatchingRule{}, *c.BatchingRules...)
	}
	setStr(&q.DirectMergeMode, c.DirectMergeMode)
	setStr(&q.OptimizationMode, snakePtr(c.OptimizationMode))
	setStr(&q.MergeMethod, snakePtr(c.MergeMethod))
	setStr(&q.TestBranchConstructionMode, snakePtr(c.TestBranchConstructionMode))
	setStr(&q.EnqueueingLabel, c.EnqueueingLabel)
	return ""
}

func (s *Server) markWrite(q *Queue, r *http.Request) {
	if fromTerraform(r) {
		q.ManagedState = "terraform"
	} else if q.ManagedState == "terraform" {
		q.ManagedState = "terraform_with_drift"
	}
}

func (s *Server) serveV2(w http.ResponseWriter, r *http.Request, raw json.RawMessage) {
	path := strings.TrimPrefix(r.URL.Path, "/v2/")
	parts := strings.Split(path, "/")

	switch {
	case r.Method == http.MethodGet && path == "repositories":
		// Exact match on the name, every record, ordered by ID, like trunk2.
		providerURL := strings.TrimPrefix(r.URL.Query().Get("providerUrl"), "https://")
		id, ok := s.repos[providerURL]
		if !ok {
			v2Error(w, http.StatusNotFound, "PROVIDER_URL_NOT_FOUND", "no repository matches providerUrl")
			return
		}
		ids := append(append([]string{}, s.dupes[providerURL]...), id)
		sort.Strings(ids)
		data := []any{}
		for _, id := range ids {
			data = append(data, map[string]any{"id": id})
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data, "hasMore": false, "nextCursor": nil})

	case r.Method == http.MethodGet && path == "merge-queues":
		repoID, branch := r.URL.Query().Get("repositoryId"), r.URL.Query().Get("targetBranch")
		limit, err := pageLimit(r)
		if err != "" {
			v2Error(w, http.StatusBadRequest, "VALIDATION_FAILED", err)
			return
		}
		var ids []string
		for id, q := range s.queues {
			if (repoID == "" || s.repos[q.Host+"/"+q.Owner+"/"+q.Name] == repoID) && (branch == "" || q.Branch == branch) {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		start := 0
		if c := r.URL.Query().Get("cursor"); c != "" {
			start, _ = strconv.Atoi(strings.TrimPrefix(c, "c"))
		}
		if s.MaxPageSize > 0 {
			limit = min(limit, s.MaxPageSize)
		}
		end := min(start+limit, len(ids))
		data := []any{}
		for _, id := range ids[min(start, len(ids)):end] {
			data = append(data, s.v2Body(s.queues[id]))
		}
		var next any
		if end < len(ids) {
			next = fmt.Sprintf("c%d", end)
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data, "hasMore": end < len(ids), "nextCursor": next})

	case r.Method == http.MethodPost && path == "merge-queues":
		var req struct {
			RepositoryID         string  `json:"repositoryId"`
			TargetBranch         string  `json:"targetBranch"`
			Mode                 *string `json:"mode"`
			Concurrency          *int    `json:"concurrency"`
			BisectionConcurrency *int    `json:"bisectionConcurrency"`
			EnqueueingLabel      *string `json:"enqueueingLabel"`
		}
		if err := strictDecode(raw, &req); err != nil {
			v2Error(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
			return
		}
		// Like trunk2: input, then the repository, then conflicts.
		if _, ok := modeFromV2[deref(req.Mode, "queue")]; !ok || !inRange(req.Concurrency, 1, 1000) {
			v2Error(w, http.StatusBadRequest, "VALIDATION_FAILED", "mode or concurrency is invalid")
			return
		}
		var repo string
		for name, id := range s.repos {
			if id == req.RepositoryID {
				repo = name
			}
		}
		if repo == "" {
			v2Error(w, http.StatusNotFound, "REPOSITORY_NOT_FOUND", "no such repository")
			return
		}
		if s.find(repo, req.TargetBranch) != nil {
			v2Error(w, http.StatusConflict, "QUEUE_ALREADY_EXISTS", "a merge queue already exists on this branch")
			return
		}
		rp := strings.SplitN(repo, "/", 3)
		s.nextID++
		q := newQueue(fmt.Sprintf("mq%d", s.nextID), rp[0], rp[1], rp[2], req.TargetBranch)
		q.Mode = modeFromV2[deref(req.Mode, "queue")]
		setInt(&q.Concurrency, req.Concurrency)
		if fromTerraform(r) {
			q.ManagedState = "terraform"
		}
		s.queues[q.ID] = q
		// Like trunk2, bisectionConcurrency and enqueueingLabel are a second write after the queue
		// exists, so their failure leaves the queue behind.
		if msg := applyV2(q, &v2Config{BisectionConcurrency: req.BisectionConcurrency, EnqueueingLabel: req.EnqueueingLabel}); msg != "" {
			v2Error(w, http.StatusBadRequest, "VALIDATION_FAILED", msg)
			return
		}
		writeJSON(w, http.StatusCreated, s.v2Body(q))

	case len(parts) >= 2 && parts[0] == "merge-queues":
		q := s.queues[parts[1]]
		if q == nil {
			v2Error(w, http.StatusNotFound, "MERGE_QUEUE_NOT_FOUND", "no such merge queue")
			return
		}
		action := strings.Join(parts[2:], "/")
		switch {
		case r.Method == http.MethodGet && action == "":
			writeJSON(w, http.StatusOK, s.v2Body(q))
		case r.Method == http.MethodDelete && action == "":
			if q.Entries > 0 {
				v2Error(w, http.StatusConflict, "QUEUE_HAS_ACTIVE_ENTRIES", "This merge queue still has active pull requests")
				return
			}
			delete(s.queues, q.ID)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPatch && action == "config":
			if q.State == "switching_modes" {
				v2Error(w, http.StatusConflict, "MODE_CHANGE_IN_PROGRESS", "the queue is switching modes")
				return
			}
			var c v2Config
			if err := strictDecode(raw, &c); err != nil {
				v2Error(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
				return
			}
			if msg := applyV2(q, &c); msg != "" {
				v2Error(w, http.StatusBadRequest, "VALIDATION_FAILED", msg)
				return
			}
			s.markWrite(q, r)
			writeJSON(w, http.StatusOK, s.v2Body(q))
		case r.Method == http.MethodPost && action == "switch-modes":
			if q.State == "switching_modes" {
				v2Error(w, http.StatusConflict, "MODE_CHANGE_IN_PROGRESS", "the queue is switching modes")
				return
			}
			var req struct {
				Mode string `json:"mode"`
			}
			if err := strictDecode(raw, &req); err != nil {
				v2Error(w, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
				return
			}
			if mode := modeFromV2[req.Mode]; mode != q.Mode {
				startSwitch(q, mode)
				s.markWrite(q, r)
			}
			writeJSON(w, http.StatusOK, map[string]any{"id": q.ID, "state": camel(q.State), "mode": modeToV2[q.Mode]})
		case r.Method == http.MethodPost && (action == "pause" || action == "resume" || action == "drain"):
			state := map[string]string{"pause": "paused", "resume": "running", "drain": "draining"}[action]
			setState(q, &state)
			s.markWrite(q, r)
			writeJSON(w, http.StatusOK, map[string]any{"id": q.ID, "state": camel(q.State), "mode": modeToV2[q.Mode]})
		default:
			http.NotFound(w, r)
		}
	default:
		http.NotFound(w, r)
	}
}

func deref(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	return *v
}

// pageLimit reads limit as trunk2 does: 1 to 100, default 25.
func pageLimit(r *http.Request) (int, string) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 25, ""
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > 100 {
		return 0, "limit must be 1 to 100"
	}
	return n, ""
}

func strictDecode(raw json.RawMessage, v any) error {
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
