package fakeapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

type v1Repo struct {
	Host  string `json:"host"`
	Owner string `json:"owner"`
	Name  string `json:"name"`
}

type v1Update struct {
	Repo                        v1Repo          `json:"repo"`
	TargetBranch                string          `json:"targetBranch"`
	Mode                        *string         `json:"mode"`
	State                       *string         `json:"state"`
	Concurrency                 *int            `json:"concurrency"`
	BisectionConcurrency        *int            `json:"bisectionConcurrency"`
	TestingTimeoutMinutes       *int            `json:"testingTimeoutMinutes"`
	PendingFailureDepth         *int            `json:"pendingFailureDepth"`
	CanOptimisticallyMerge      *bool           `json:"canOptimisticallyMerge"`
	Batch                       *bool           `json:"batch"`
	BatchingMaxWaitTimeMinutes  *int            `json:"batchingMaxWaitTimeMinutes"`
	BatchingMinSize             *int            `json:"batchingMinSize"`
	BatchingRules               *[]BatchingRule `json:"batchingRules"`
	CreatePrsForTestingBranches *bool           `json:"createPrsForTestingBranches"`
	MergeMethod                 *string         `json:"mergeMethod"`
	CommentsEnabled             *bool           `json:"commentsEnabled"`
	CommandsEnabled             *bool           `json:"commandsEnabled"`
	StatusCheckEnabled          *bool           `json:"statusCheckEnabled"`
	ExtensionEnabled            *bool           `json:"extensionEnabled"`
	LabelCommandsEnabled        *bool           `json:"labelCommandsEnabled"`
	StateLabelsEnabled          *bool           `json:"stateLabelsEnabled"`
	EnqueueingLabel             *string         `json:"enqueueingLabel"`
	DirectMergeMode             *string         `json:"directMergeMode"`
	OptimizationMode            *string         `json:"optimizationMode"`
	TestBranchConstructionMode  *string         `json:"testBranchConstructionMode"`
	NotReadyTimeoutHours        *int            `json:"notReadyTimeoutHours"`
	RequiredStatuses            *[]string       `json:"requiredStatuses"`
	DeleteRequiredStatuses      *bool           `json:"deleteRequiredStatuses"`
}

func v1Body(q *Queue) map[string]any {
	return map[string]any{
		"state": q.State, "branch": q.Branch, "mode": q.Mode,
		"concurrency": q.Concurrency, "bisectionConcurrency": q.BisectionConcurrency,
		"testingTimeoutMinutes": q.TestingTimeoutMinutes, "pendingFailureDepth": q.PendingFailureDepth,
		"canOptimisticallyMerge": q.CanOptimisticallyMerge, "batch": q.Batch,
		"batchingMaxWaitTimeMinutes": q.BatchingMaxWaitTimeMinutes, "batchingMinSize": q.BatchingMinSize,
		"batchingRules": q.BatchingRules, "createPrsForTestingBranches": q.CreatePrsForTestingBranches,
		"mergeMethod": q.MergeMethod, "commentsEnabled": q.CommentsEnabled, "commandsEnabled": q.CommandsEnabled,
		"statusCheckEnabled": q.StatusCheckEnabled, "extensionEnabled": q.ExtensionEnabled,
		"labelCommandsEnabled": q.LabelCommandsEnabled, "stateLabelsEnabled": q.StateLabelsEnabled,
		"enqueueingLabel": q.EnqueueingLabel, "directMergeMode": q.DirectMergeMode,
		"optimizationMode": q.OptimizationMode, "testBranchConstructionMode": q.TestBranchConstructionMode,
		"notReadyTimeoutHours": q.NotReadyTimeoutHours, "requiredStatuses": q.RequiredStatuses,
		"allowedBotSubmitters": []string{}, "enqueuedPullRequests": []any{},
	}
}

// v1Repo resolves a repo the way trunk1 does: ignoring case, unlike v2.
func (s *Server) v1Repo(r v1Repo) (string, bool) {
	want := strings.ToLower(r.Host + "/" + r.Owner + "/" + r.Name)
	for name := range s.repos {
		if strings.ToLower(name) == want {
			return name, true
		}
	}
	return "", false
}

// serveV1 answers v1. Like trunk1, getQueue resolves the repo without checking the token first,
// so a bad token or an unknown repo is a 404 there and a 401 everywhere else.
func (s *Server) serveV1(w http.ResponseWriter, r *http.Request, raw json.RawMessage, authorized bool) {
	var req v1Update
	if err := json.Unmarshal(raw, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	repo, known := s.v1Repo(req.Repo)
	op := strings.TrimPrefix(r.URL.Path, "/v1/")
	if !authorized || !known {
		if op == "getQueue" {
			http.Error(w, "Not Found", http.StatusNotFound)
		} else {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}
	q := s.find(repo, req.TargetBranch)

	switch op {
	case "createQueue":
		if q != nil {
			http.Error(w, "Merge queue already exists", http.StatusConflict)
			return
		}
		s.nextID++
		rp := strings.SplitN(repo, "/", 3)
		q = newQueue(fmt.Sprintf("mq-%d", s.nextID), rp[0], rp[1], rp[2], req.TargetBranch)
		if req.Mode != nil {
			q.Mode = *req.Mode
		}
		if req.Concurrency != nil {
			q.Concurrency = *req.Concurrency
		}
		if fromTerraform(r) {
			q.ManagedState = "terraform"
		}
		s.queues[q.ID] = q
		w.WriteHeader(http.StatusOK)
	case "getQueue":
		if q == nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, v1Body(q))
	case "updateQueue":
		if q == nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		if msg := applyV1(q, &req); msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		if fromTerraform(r) {
			q.ManagedState = "terraform"
		} else if q.ManagedState == "terraform" {
			q.ManagedState = "terraform_with_drift"
		}
		writeJSON(w, http.StatusOK, v1Body(q))
	case "deleteQueue":
		if q == nil {
			w.WriteHeader(http.StatusOK)
			return
		}
		if q.Entries > 0 {
			http.Error(w, "Cannot delete merge instance as it is not empty", http.StatusBadRequest)
			return
		}
		delete(s.queues, q.ID)
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

func applyV1(q *Queue, u *v1Update) string {
	if u.BisectionConcurrency != nil && *u.BisectionConcurrency < 1 {
		return "bisectionConcurrency must be at least 1"
	}
	if u.BatchingMaxWaitTimeMinutes != nil && *u.BatchingMaxWaitTimeMinutes < 1 {
		return "batchingMaxWaitTimeMinutes must be at least 1"
	}
	setInt(&q.Concurrency, u.Concurrency)
	setInt(&q.BisectionConcurrency, u.BisectionConcurrency)
	setInt(&q.TestingTimeoutMinutes, u.TestingTimeoutMinutes)
	setInt(&q.PendingFailureDepth, u.PendingFailureDepth)
	setBool(&q.CanOptimisticallyMerge, u.CanOptimisticallyMerge)
	setBool(&q.Batch, u.Batch)
	setInt(&q.BatchingMaxWaitTimeMinutes, u.BatchingMaxWaitTimeMinutes)
	setInt(&q.BatchingMinSize, u.BatchingMinSize)
	if u.BatchingRules != nil {
		q.BatchingRules = append([]BatchingRule{}, *u.BatchingRules...)
	}
	setBool(&q.CreatePrsForTestingBranches, u.CreatePrsForTestingBranches)
	setStr(&q.MergeMethod, u.MergeMethod)
	setBool(&q.CommentsEnabled, u.CommentsEnabled)
	setBool(&q.CommandsEnabled, u.CommandsEnabled)
	setBool(&q.StatusCheckEnabled, u.StatusCheckEnabled)
	setBool(&q.ExtensionEnabled, u.ExtensionEnabled)
	setBool(&q.LabelCommandsEnabled, u.LabelCommandsEnabled)
	setBool(&q.StateLabelsEnabled, u.StateLabelsEnabled)
	setStr(&q.EnqueueingLabel, u.EnqueueingLabel)
	setStr(&q.DirectMergeMode, u.DirectMergeMode)
	setStr(&q.OptimizationMode, u.OptimizationMode)
	setStr(&q.TestBranchConstructionMode, u.TestBranchConstructionMode)
	setInt(&q.NotReadyTimeoutHours, u.NotReadyTimeoutHours)
	// Like trunk1: an empty list is indistinguishable from an absent one.
	if u.RequiredStatuses != nil && len(*u.RequiredStatuses) > 0 {
		q.RequiredStatuses = sorted(*u.RequiredStatuses)
	}
	if u.DeleteRequiredStatuses != nil && *u.DeleteRequiredStatuses {
		q.RequiredStatuses = []string{}
	}
	setState(q, u.State)
	if u.Mode != nil && *u.Mode != q.Mode {
		startSwitch(q, *u.Mode)
	}
	return ""
}

func setState(q *Queue, state *string) {
	if state == nil {
		return
	}
	// As in trunk1, any state change during a switch abandons it.
	q.State = *state
	q.switchTo = ""
}

func startSwitch(q *Queue, mode string) {
	q.resumeState = q.State
	q.switchTo = mode
	q.State = "switching_modes"
}

func sorted(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func setInt(dst *int, v *int) {
	if v != nil {
		*dst = *v
	}
}

func setBool(dst *bool, v *bool) {
	if v != nil {
		*dst = *v
	}
}

func setStr(dst *string, v *string) {
	if v != nil {
		*dst = *v
	}
}
