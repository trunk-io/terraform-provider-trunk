package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testQueue returns a minimal Queue for use in handler responses.
func testQueue() Queue {
	return Queue{
		Mode:                        "single",
		Concurrency:                 1,
		State:                       "running",
		TestingTimeoutMinutes:       60,
		PendingFailureDepth:         0,
		CanOptimisticallyMerge:      false,
		Batch:                       false,
		BatchingMaxWaitTimeMinutes:  0,
		BatchingMinSize:             1,
		CreatePrsForTestingBranches: false,
		MergeMethod:                 "squash",
		CommentsEnabled:             true,
		CommandsEnabled:             true,
		StatusCheckEnabled:          false,
		DirectMergeMode:             "off",
		OptimizationMode:            "off",
		BisectionConcurrency:        1,
	}
}

func TestCreateQueue(t *testing.T) {
	var gotReq CreateQueueRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	req := CreateQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
		Mode:         "single",
		Concurrency:  1,
	}
	err := c.CreateQueue(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateQueue error: %v", err)
	}
	if gotReq.Repo.Host != "github.com" {
		t.Errorf("request repo.host = %q, want %q", gotReq.Repo.Host, "github.com")
	}
	if gotReq.TargetBranch != "main" {
		t.Errorf("request targetBranch = %q, want %q", gotReq.TargetBranch, "main")
	}
	if gotReq.Mode != "single" {
		t.Errorf("request mode = %q, want %q", gotReq.Mode, "single")
	}
	if gotReq.Concurrency != 1 {
		t.Errorf("request concurrency = %d, want 1", gotReq.Concurrency)
	}
}

func TestCreateQueue_ReturnsErrorOnAPIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("server error"))
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	err := c.CreateQueue(context.Background(), CreateQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
	})
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
}

func TestGetQueue(t *testing.T) {
	var gotReq GetQueueRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testQueue())
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	req := GetQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
	}
	queue, err := c.GetQueue(context.Background(), req)
	if err != nil {
		t.Fatalf("GetQueue error: %v", err)
	}
	if queue.State != "running" {
		t.Errorf("State = %q, want %q", queue.State, "running")
	}
	if queue.Mode != "single" {
		t.Errorf("Mode = %q, want %q", queue.Mode, "single")
	}
	// Identity fields should be populated from the request, not the response body.
	if queue.TargetBranch != "main" {
		t.Errorf("TargetBranch = %q, want %q", queue.TargetBranch, "main")
	}
	if queue.Repo.Owner != "my-org" {
		t.Errorf("Repo.Owner = %q, want %q", queue.Repo.Owner, "my-org")
	}
	if gotReq.Repo.Owner != "my-org" {
		t.Errorf("request repo.owner = %q, want %q", gotReq.Repo.Owner, "my-org")
	}
	if gotReq.TargetBranch != "main" {
		t.Errorf("request targetBranch = %q, want %q", gotReq.TargetBranch, "main")
	}
}

func TestGetQueue_AllFields(t *testing.T) {
	statuses := []string{"ci/test", "ci/lint"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		q := Queue{
			Mode:                        "parallel",
			Concurrency:                 3,
			State:                       "running",
			TestingTimeoutMinutes:       60,
			PendingFailureDepth:         2,
			CanOptimisticallyMerge:      true,
			Batch:                       true,
			BatchingMaxWaitTimeMinutes:  10,
			BatchingMinSize:             3,
			CreatePrsForTestingBranches: true,
			MergeMethod:                 "squash",
			CommentsEnabled:             true,
			CommandsEnabled:             true,
			StatusCheckEnabled:          true,
			DirectMergeMode:             "off",
			OptimizationMode:            "bisection_skip_redundant_tests",
			BisectionConcurrency:        5,
			RequiredStatuses:            &statuses,
		}
		_ = json.NewEncoder(w).Encode(q)
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	queue, err := c.GetQueue(context.Background(), GetQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
	})
	if err != nil {
		t.Fatalf("GetQueue error: %v", err)
	}

	if queue.Mode != "parallel" {
		t.Errorf("Mode = %q, want %q", queue.Mode, "parallel")
	}
	if queue.MergeMethod != "squash" {
		t.Errorf("MergeMethod = %q, want %q", queue.MergeMethod, "squash")
	}
	if !queue.CommentsEnabled {
		t.Error("CommentsEnabled = false, want true")
	}
	if !queue.CommandsEnabled {
		t.Error("CommandsEnabled = false, want true")
	}
	if !queue.StatusCheckEnabled {
		t.Error("StatusCheckEnabled = false, want true")
	}
	if queue.DirectMergeMode != "off" {
		t.Errorf("DirectMergeMode = %q, want %q", queue.DirectMergeMode, "off")
	}
	if queue.OptimizationMode != "bisection_skip_redundant_tests" {
		t.Errorf("OptimizationMode = %q, want %q", queue.OptimizationMode, "bisection_skip_redundant_tests")
	}
	if queue.BisectionConcurrency != 5 {
		t.Errorf("BisectionConcurrency = %d, want 5", queue.BisectionConcurrency)
	}
	if queue.RequiredStatuses == nil || len(*queue.RequiredStatuses) != 2 {
		t.Fatalf("RequiredStatuses = %v, want 2 elements", queue.RequiredStatuses)
	}
}

func TestGetQueue_NullRequiredStatuses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"mode":"single","concurrency":1,"state":"running","testingTimeoutMinutes":60,"pendingFailureDepth":0,"canOptimisticallyMerge":false,"batch":false,"batchingMaxWaitTimeMinutes":0,"batchingMinSize":1,"createPrsForTestingBranches":false,"mergeMethod":"squash","commentsEnabled":true,"commandsEnabled":true,"statusCheckEnabled":false,"directMergeMode":"off","optimizationMode":"off","bisectionConcurrency":1,"requiredStatuses":null}`))
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	queue, err := c.GetQueue(context.Background(), GetQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
	})
	if err != nil {
		t.Fatalf("GetQueue error: %v", err)
	}
	if queue.RequiredStatuses != nil {
		t.Errorf("RequiredStatuses = %v, want nil", queue.RequiredStatuses)
	}
}

func TestGetQueue_ReturnsErrorOnAPIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	_, err := c.GetQueue(context.Background(), GetQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
	})
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}

func TestUpdateQueue_OmitsNilFields(t *testing.T) {
	var rawBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&rawBody); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testQueue())
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	if _, err := c.UpdateQueue(context.Background(), UpdateQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
		// All optional fields left nil.
	}); err != nil {
		t.Fatalf("UpdateQueue error: %v", err)
	}
	for _, field := range []string{
		"mode", "concurrency", "state", "mergeMethod", "batch",
		"deleteRequiredStatuses", "testingTimeoutMinutes", "requiredStatuses",
	} {
		if _, present := rawBody[field]; present {
			t.Errorf("field %q should be absent when nil, but was present in request body", field)
		}
	}
}

func TestUpdateQueue_IncludesNonNilFields(t *testing.T) {
	var rawBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&rawBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testQueue())
	}))
	defer server.Close()

	mode := "parallel"
	concurrency := 3
	mergeMethod := "squash"
	batch := true
	c := newTestClient("key", server.URL)
	if _, err := c.UpdateQueue(context.Background(), UpdateQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
		Mode:         &mode,
		Concurrency:  &concurrency,
		MergeMethod:  &mergeMethod,
		Batch:        &batch,
	}); err != nil {
		t.Fatalf("UpdateQueue error: %v", err)
	}
	if rawBody["mode"] != "parallel" {
		t.Errorf("mode = %v, want %q", rawBody["mode"], "parallel")
	}
	if rawBody["concurrency"] != float64(3) {
		t.Errorf("concurrency = %v, want 3", rawBody["concurrency"])
	}
	if rawBody["mergeMethod"] != "squash" {
		t.Errorf("mergeMethod = %v, want %q", rawBody["mergeMethod"], "squash")
	}
	if rawBody["batch"] != true {
		t.Errorf("batch = %v, want true", rawBody["batch"])
	}
}

func TestUpdateQueue_ReturnsQueue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		q := testQueue()
		q.Mode = "parallel"
		q.Concurrency = 3
		q.MergeMethod = "squash"
		q.DirectMergeMode = "off"
		_ = json.NewEncoder(w).Encode(q)
	}))
	defer server.Close()

	mode := "parallel"
	c := newTestClient("key", server.URL)
	queue, err := c.UpdateQueue(context.Background(), UpdateQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
		Mode:         &mode,
	})
	if err != nil {
		t.Fatalf("UpdateQueue error: %v", err)
	}
	if queue.Mode != "parallel" {
		t.Errorf("Mode = %q, want %q", queue.Mode, "parallel")
	}
	if queue.MergeMethod != "squash" {
		t.Errorf("MergeMethod = %q, want %q", queue.MergeMethod, "squash")
	}
	if queue.Repo.Owner != "my-org" {
		t.Errorf("Repo.Owner = %q, want %q", queue.Repo.Owner, "my-org")
	}
}

func TestUpdateQueue_DeleteRequiredStatuses(t *testing.T) {
	var rawBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&rawBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testQueue())
	}))
	defer server.Close()

	deleteStatuses := true
	c := newTestClient("key", server.URL)
	if _, err := c.UpdateQueue(context.Background(), UpdateQueueRequest{
		Repo:                   Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch:           "main",
		DeleteRequiredStatuses: &deleteStatuses,
	}); err != nil {
		t.Fatalf("UpdateQueue error: %v", err)
	}
	if rawBody["deleteRequiredStatuses"] != true {
		t.Errorf("deleteRequiredStatuses = %v, want true", rawBody["deleteRequiredStatuses"])
	}
}

func TestUpdateQueue_SendsEmptyRequiredStatuses(t *testing.T) {
	var rawBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&rawBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(testQueue())
	}))
	defer server.Close()

	empty := []string{}
	c := newTestClient("key", server.URL)
	if _, err := c.UpdateQueue(context.Background(), UpdateQueueRequest{
		Repo:             Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch:     "main",
		RequiredStatuses: &empty,
	}); err != nil {
		t.Fatalf("UpdateQueue error: %v", err)
	}
	val, present := rawBody["requiredStatuses"]
	if !present {
		t.Fatal("requiredStatuses should be present in request body when set to empty slice")
	}
	statuses, ok := val.([]any)
	if !ok || len(statuses) != 0 {
		t.Errorf("requiredStatuses = %v, want []", val)
	}
}

func TestUpdateQueue_ReturnsErrorOnAPIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	_, err := c.UpdateQueue(context.Background(), UpdateQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
	})
	if _, ok := err.(*APIError); !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
}

func TestDeleteQueue(t *testing.T) {
	var gotReq DeleteQueueRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotReq)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	err := c.DeleteQueue(context.Background(), DeleteQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
	})
	if err != nil {
		t.Fatalf("DeleteQueue error: %v", err)
	}
	if gotReq.Repo.Name != "my-repo" {
		t.Errorf("request repo.name = %q, want %q", gotReq.Repo.Name, "my-repo")
	}
	if gotReq.TargetBranch != "main" {
		t.Errorf("request targetBranch = %q, want %q", gotReq.TargetBranch, "main")
	}
}

func TestDeleteQueue_ReturnsAPIErrorOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("queue not found"))
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	err := c.DeleteQueue(context.Background(), DeleteQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
	})
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
}
