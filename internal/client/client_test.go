package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestClient creates a client with retries disabled for fast unit tests.
func newTestClient(apiKey, baseURL string) *Client {
	c := NewClient(apiKey, baseURL)
	c.maxRetries = 0
	c.baseRetryDelay = 0
	return c
}

func TestDoRequest_SetsAuthAndContentTypeHeaders(t *testing.T) {
	var gotAPIToken, gotContentType, gotSource, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIToken = r.Header.Get("x-api-token")
		gotContentType = r.Header.Get("Content-Type")
		gotSource = r.Header.Get("x-source")
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	c := newTestClient("secret-key", server.URL)
	err := c.doRequest(context.Background(), "test", struct{}{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAPIToken != "secret-key" {
		t.Errorf("x-api-token = %q, want %q", gotAPIToken, "secret-key")
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotContentType, "application/json")
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotSource != "terraform" {
		t.Errorf("x-source = %q, want %q", gotSource, "terraform")
	}
}

func TestDoRequest_Returns4xxAsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	c := newTestClient("bad-key", server.URL)
	err := c.doRequest(context.Background(), "test", struct{}{}, nil)

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
	if apiErr.Body != `{"error":"unauthorized"}` {
		t.Errorf("Body = %q, want %q", apiErr.Body, `{"error":"unauthorized"}`)
	}
}

func TestDoRequest_Returns5xxAsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("internal server error"))
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	err := c.doRequest(context.Background(), "test", struct{}{}, nil)

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusInternalServerError)
	}
	if apiErr.Body != "internal server error" {
		t.Errorf("Body = %q, want %q", apiErr.Body, "internal server error")
	}
}

func TestDoRequest_ReturnsErrorOnMalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	var out struct{ Field string }
	err := c.doRequest(context.Background(), "test", struct{}{}, &out)
	if err == nil {
		t.Fatal("expected error for malformed JSON response, got nil")
	}
}

func TestAPIError_Error(t *testing.T) {
	err := &APIError{StatusCode: 404, Body: "not found"}
	want := "API error 404: not found"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestNewClient_DefaultBaseURL(t *testing.T) {
	c := NewClient("key", "")
	if c.baseURL != defaultBaseURL {
		t.Errorf("baseURL = %q, want %q", c.baseURL, defaultBaseURL)
	}
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	c := NewClient("key", "https://example.com/v1/")
	if c.baseURL != "https://example.com/v1" {
		t.Errorf("baseURL = %q, want %q", c.baseURL, "https://example.com/v1")
	}
}

func TestDoRequest_ReturnsErrorOnNetworkFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close() // close before the request so Do() returns a network error

	c := newTestClient("key", server.URL)
	err := c.doRequest(context.Background(), "test", struct{}{}, nil)
	if err == nil {
		t.Fatal("expected error on network failure, got nil")
	}
}

func TestDoRequest_Retries5xxThenSucceeds(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	c.maxRetries = 3
	c.baseRetryDelay = time.Millisecond // keep test fast
	err := c.doRequest(context.Background(), "test", struct{}{}, nil)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoRequest_DoesNotRetry4xx(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad request"))
	}))
	defer server.Close()

	c := newTestClient("key", server.URL)
	c.maxRetries = 3
	c.baseRetryDelay = time.Millisecond
	err := c.doRequest(context.Background(), "test", struct{}{}, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retries for 4xx), got %d", attempts)
	}
}
