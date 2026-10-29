// Package fakeapi is an in-memory stand-in for the Trunk merge queue API, v1 and v2, for tests.
//
// It mirrors the behavior the provider depends on, as the real services implement it:
//   - v1 takes the organization token in x-api-token; v2 takes an API key as a bearer token.
//   - Required statuses are stored sorted. v1 ignores an empty requiredStatuses and removes the
//     override with deleteRequiredStatuses; v2 removes it with an empty list.
//   - Changing the mode puts the queue in switching_modes, with the old mode, until
//     FinishSwitches runs. v2 refuses config and mode changes meanwhile.
//   - A write with x-source: terraform marks the queue Terraform-managed; Edit marks drift.
//   - v2's PATCH rejects unknown fields, and v2 enforces its ranges.
package fakeapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

const (
	// V1Token is the organization API token the server accepts on v1.
	V1Token = "org-token-123"
	// V2Key is the API key the server accepts on v2.
	V2Key = "trunk_v2_testkey0000000000000000000000000000000000"
)

// BatchingRule is a stored batching rule.
type BatchingRule struct {
	TargetPattern string `json:"targetPattern"`
	Policy        string `json:"policy"`
}

// Queue is a stored queue, in v1 spelling.
type Queue struct {
	ID           string
	Host         string
	Owner        string
	Name         string
	Branch       string
	Mode         string
	State        string
	ManagedState string
	// Entries is how many pull requests are queued; delete refuses while it is above zero.
	Entries int

	switchTo    string
	resumeState string

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
	RequiredStatuses            []string
}

// Request is one request the server received.
type Request struct {
	Method string
	Path   string
	Body   map[string]any
}

// Server is a fake Trunk API.
type Server struct {
	*httptest.Server

	// MaxPageSize, if set, caps every list page below what the request asked for, so tests can
	// reach later pages without creating 100 queues.
	MaxPageSize int

	mu       sync.Mutex
	repos    map[string]string   // "host/owner/name" -> repository ID
	dupes    map[string][]string // "host/owner/name" -> extra, unconnected records for it
	queues   map[string]*Queue   // queue ID -> queue
	nextID   int
	requests []Request
	failNext map[string][]int // "METHOD /path" -> statuses to answer, in turn, after doing the work
}

// New starts a server that knows the given repositories ("host/owner/name").
func New(repos ...string) *Server {
	s := &Server{repos: map[string]string{}, dupes: map[string][]string{}, queues: map[string]*Queue{}, failNext: map[string][]int{}}
	for i, r := range repos {
		s.repos[r] = fmt.Sprintf("repo-%d", i+1)
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// Requests returns the requests received so far.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Request(nil), s.requests...)
}

// Queue returns a copy of the queue on host/owner/name and branch, or nil.
func (s *Server) Queue(repo, branch string) *Queue {
	s.mu.Lock()
	defer s.mu.Unlock()
	if q := s.find(repo, branch); q != nil {
		c := *q
		return &c
	}
	return nil
}

// PassNext lets the next request to method and path through normally, ahead of failures queued
// with FailAfter.
func (s *Server) PassNext(method, path string) {
	s.FailAfter(method, path, 0)
}

// AddDuplicateRepo adds a second record for a repository, as some organizations have. Its ID sorts
// first, so v2 lists it first.
func (s *Server) AddDuplicateRepo(repo string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dupes[repo] = append(s.dupes[repo], fmt.Sprintf("repo-0dup%d", len(s.dupes[repo])+1))
}

// FailAfter makes the next request to method and path do its work and then answer status, as a
// real server does when a later step fails or the response is lost. Each call fails one more
// request.
func (s *Server) FailAfter(method, path string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := method + " " + path
	s.failNext[key] = append(s.failNext[key], status)
}

// Edit changes a queue as the web app would, outside Terraform.
func (s *Server) Edit(repo, branch string, fn func(*Queue)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := s.find(repo, branch)
	fn(q)
	if q.ManagedState == "terraform" {
		q.ManagedState = "terraform_with_drift"
	}
}

// FinishSwitches completes every pending mode switch.
func (s *Server) FinishSwitches() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, q := range s.queues {
		if q.State == "switching_modes" {
			q.Mode, q.State, q.switchTo = q.switchTo, q.resumeState, ""
		}
	}
}

func (s *Server) find(repo, branch string) *Queue {
	for _, q := range s.queues {
		if q.Host+"/"+q.Owner+"/"+q.Name == repo && q.Branch == branch {
			return q
		}
	}
	return nil
}

func newQueue(id, host, owner, name, branch string) *Queue {
	return &Queue{
		ID: id, Host: host, Owner: owner, Name: name, Branch: branch,
		Mode: "single", State: "running", ManagedState: "manual",
		Concurrency: 5, BisectionConcurrency: 5, BatchingMaxWaitTimeMinutes: 5, BatchingMinSize: 2,
		BatchingRules: []BatchingRule{}, CreatePrsForTestingBranches: true, MergeMethod: "squash",
		CommentsEnabled: true, CommandsEnabled: true, StatusCheckEnabled: true, ExtensionEnabled: true,
		LabelCommandsEnabled: true, EnqueueingLabel: "trunk-merge-queue-submit", DirectMergeMode: "off",
		OptimizationMode: "off", TestBranchConstructionMode: "rename_temp_branch", RequiredStatuses: []string{},
	}
}

func fromTerraform(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("x-source")+r.Header.Get("User-Agent")), "terraform")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func v2Error(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": msg}})
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var raw json.RawMessage
	_ = json.NewDecoder(r.Body).Decode(&raw)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	s.requests = append(s.requests, Request{Method: r.Method, Path: r.URL.Path, Body: body})

	key := r.Method + " " + r.URL.Path
	if statuses := s.failNext[key]; len(statuses) > 0 {
		s.failNext[key] = statuses[1:]
		if statuses[0] != 0 {
			w = &failingWriter{ResponseWriter: w, status: statuses[0]}
		}
	}

	switch {
	case strings.HasPrefix(r.URL.Path, "/v1/"):
		s.serveV1(w, r, raw, r.Header.Get("x-api-token") == V1Token)
	case strings.HasPrefix(r.URL.Path, "/v2/"):
		if r.Header.Get("Authorization") != "Bearer "+V2Key {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		s.serveV2(w, r, raw)
	default:
		http.NotFound(w, r)
	}
}

// failingWriter replaces whatever the handler answers with an error status.
type failingWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (f *failingWriter) WriteHeader(int) {
	if !f.written {
		f.written = true
		f.ResponseWriter.WriteHeader(f.status)
	}
}

func (f *failingWriter) Write(b []byte) (int, error) {
	f.WriteHeader(0)
	return len(b), nil
}
