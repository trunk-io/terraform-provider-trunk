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
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
		Mode:         "single",
		Concurrency:  1,
		State:        "RUNNING",
	}
}

func TestCreateQueue(t *testing.T) {
	var gotReq CreateQueueRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			t.Errorf("decoding request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(CreateQueueResponse{Queue: testQueue()})
	}))
	defer server.Close()

	c := NewClient("key", server.URL)
	req := CreateQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
		Mode:         "single",
		Concurrency:  1,
	}
	queue, err := c.CreateQueue(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateQueue error: %v", err)
	}
	if queue.TargetBranch != "main" {
		t.Errorf("TargetBranch = %q, want %q", queue.TargetBranch, "main")
	}
	if queue.Mode != "single" {
		t.Errorf("Mode = %q, want %q", queue.Mode, "single")
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

	c := NewClient("key", server.URL)
	_, err := c.CreateQueue(context.Background(), CreateQueueRequest{
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
		// Respond with the actual flat API format (no queue wrapper, uppercase mode, branch field).
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(getQueueAPIResponse{
			Branch:      "main",
			Mode:        "SINGLE",
			Concurrency: 1,
			State:       "RUNNING",
		})
	}))
	defer server.Close()

	c := NewClient("key", server.URL)
	req := GetQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
	}
	queue, err := c.GetQueue(context.Background(), req)
	if err != nil {
		t.Fatalf("GetQueue error: %v", err)
	}
	if queue.State != "RUNNING" {
		t.Errorf("State = %q, want %q", queue.State, "RUNNING")
	}
	// Mode should be normalized from API uppercase to schema lowercase.
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

func TestGetQueue_ReturnsErrorOnAPIFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer server.Close()

	c := NewClient("key", server.URL)
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
		_ = json.NewEncoder(w).Encode(UpdateQueueResponse{Queue: testQueue()})
	}))
	defer server.Close()

	c := NewClient("key", server.URL)
	_, err := c.UpdateQueue(context.Background(), UpdateQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
		// All optional fields left nil.
	})
	if err != nil {
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
		_ = json.NewEncoder(w).Encode(UpdateQueueResponse{Queue: testQueue()})
	}))
	defer server.Close()

	mode := "parallel"
	concurrency := 3
	mergeMethod := "SQUASH"
	batch := true
	c := NewClient("key", server.URL)
	_, err := c.UpdateQueue(context.Background(), UpdateQueueRequest{
		Repo:         Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch: "main",
		Mode:         &mode,
		Concurrency:  &concurrency,
		MergeMethod:  &mergeMethod,
		Batch:        &batch,
	})
	if err != nil {
		t.Fatalf("UpdateQueue error: %v", err)
	}
	if rawBody["mode"] != "parallel" {
		t.Errorf("mode = %v, want %q", rawBody["mode"], "parallel")
	}
	if rawBody["concurrency"] != float64(3) {
		t.Errorf("concurrency = %v, want 3", rawBody["concurrency"])
	}
	if rawBody["mergeMethod"] != "SQUASH" {
		t.Errorf("mergeMethod = %v, want %q", rawBody["mergeMethod"], "SQUASH")
	}
	if rawBody["batch"] != true {
		t.Errorf("batch = %v, want true", rawBody["batch"])
	}
}

func TestUpdateQueue_DeleteRequiredStatuses(t *testing.T) {
	var rawBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&rawBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(UpdateQueueResponse{Queue: testQueue()})
	}))
	defer server.Close()

	deleteStatuses := true
	c := NewClient("key", server.URL)
	_, err := c.UpdateQueue(context.Background(), UpdateQueueRequest{
		Repo:                   Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch:           "main",
		DeleteRequiredStatuses: &deleteStatuses,
	})
	if err != nil {
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
		_ = json.NewEncoder(w).Encode(UpdateQueueResponse{Queue: testQueue()})
	}))
	defer server.Close()

	empty := []string{}
	c := NewClient("key", server.URL)
	_, err := c.UpdateQueue(context.Background(), UpdateQueueRequest{
		Repo:             Repo{Host: "github.com", Owner: "my-org", Name: "my-repo"},
		TargetBranch:     "main",
		RequiredStatuses: &empty,
	})
	if err != nil {
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

	c := NewClient("key", server.URL)
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

	c := NewClient("key", server.URL)
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

	c := NewClient("key", server.URL)
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
