package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoRequest_SetsAuthAndContentTypeHeaders(t *testing.T) {
	var gotAPIToken, gotContentType, gotMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIToken = r.Header.Get("x-api-token")
		gotContentType = r.Header.Get("Content-Type")
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	c := NewClient("secret-key", server.URL)
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
}

func TestDoRequest_Returns4xxAsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer server.Close()

	c := NewClient("bad-key", server.URL)
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

	c := NewClient("key", server.URL)
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

	c := NewClient("key", server.URL)
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

	c := NewClient("key", server.URL)
	err := c.doRequest(context.Background(), "test", struct{}{}, nil)
	if err == nil {
		t.Fatal("expected error on network failure, got nil")
	}
}
