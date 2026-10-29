package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the production Trunk API host. The client appends /v1 or /v2 itself.
const DefaultBaseURL = "https://api.trunk.io"

// v2KeyPrefix starts every v2 API key. Any other credential is treated as the organization
// API token, which only the v1 API accepts.
const v2KeyPrefix = "trunk_v2_"

// APIVersion is the Trunk API generation a Client talks to.
type APIVersion int

const (
	// V1 is the v1 API, authenticated with the organization API token in x-api-token.
	V1 APIVersion = 1
	// V2 is the v2 API, authenticated with a scoped API key as a bearer token.
	V2 APIVersion = 2
)

// APIVersionForKey picks the API a credential works with.
func APIVersionForKey(apiKey string) APIVersion {
	if strings.HasPrefix(apiKey, v2KeyPrefix) {
		return V2
	}
	return V1
}

// Client communicates with the Trunk API. It talks to v2 when given a v2 API key and to v1
// otherwise; the merge queue methods hide the difference.
type Client struct {
	host           string
	apiKey         string
	version        APIVersion
	userAgent      string
	httpClient     *http.Client
	maxRetries     int
	baseRetryDelay time.Duration
}

// NewClient creates a Client. baseURL is the API host, such as https://api.trunk.io; a trailing
// /v1 or /v2 is accepted and ignored. An empty baseURL means production. providerVersion goes
// into the User-Agent.
func NewClient(apiKey, baseURL, providerVersion string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	host := strings.TrimRight(baseURL, "/")
	host = strings.TrimSuffix(host, "/v1")
	host = strings.TrimSuffix(host, "/v2")
	if providerVersion == "" {
		providerVersion = "dev"
	}
	return &Client{
		host:           host,
		apiKey:         apiKey,
		version:        APIVersionForKey(apiKey),
		userAgent:      "terraform-provider-trunk/" + providerVersion,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		maxRetries:     3,
		baseRetryDelay: 500 * time.Millisecond,
	}
}

// Version reports which API generation this client uses.
func (c *Client) Version() APIVersion {
	return c.version
}

// APIError is a non-2xx response from the Trunk API.
type APIError struct {
	StatusCode int
	// Code is the v2 error envelope's code, such as QUEUE_HAS_ACTIVE_ENTRIES. Empty for v1.
	Code string
	// Message is the v2 envelope's message, or the raw body.
	Message string
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("Trunk API returned %d %s: %s", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("Trunk API returned %d: %s", e.StatusCode, e.Message)
}

// IsStatus reports whether err is an APIError with the given HTTP status.
func IsStatus(err error, status int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == status
}

// retryPolicy says which failures a request may be retried on.
type retryPolicy int

const (
	// retrySafe retries rate limits, 5xx responses and network failures. For reads and for
	// writes that are safe to repeat.
	retrySafe retryPolicy = iota
	// retryRateLimitOnly retries only 429s, which the server rejects before doing anything.
	// For creates, where a retry after a lost response would conflict with itself.
	retryRateLimitOnly
)

// do sends a request and decodes a JSON response into out (if non-nil). path is relative to the
// host and includes the version, such as "/v1/getQueue".
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any, policy retryPolicy) error {
	var bodyBytes []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		bodyBytes = b
	}
	target := c.host + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(math.Pow(2, float64(attempt-1))) * c.baseRetryDelay
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
		lastErr = c.doOnce(ctx, method, target, bodyBytes, out)
		if lastErr == nil || !retryable(lastErr, policy) {
			break
		}
	}
	var apiErr *APIError
	if errors.As(lastErr, &apiErr) && apiErr.Code == "INSUFFICIENT_PERMISSIONS" {
		// The message names the internal permission; point at the one people pick in the app.
		return fmt.Errorf("the API key needs the Update Merge Queue settings permission: %w", lastErr)
	}
	return lastErr
}

func retryable(err error, policy retryPolicy) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.StatusCode == http.StatusTooManyRequests {
			return true
		}
		return policy == retrySafe && apiErr.StatusCode >= 500
	}
	// A network failure: the request may or may not have reached the server.
	return policy == retrySafe && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

func (c *Client) doOnce(ctx context.Context, method, target string, bodyBytes []byte, out any) error {
	var reader io.Reader
	if bodyBytes != nil {
		reader = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	// Both APIs mark a queue as Terraform-managed when either of these names Terraform.
	req.Header.Set("x-source", "terraform")
	req.Header.Set("User-Agent", c.userAgent)
	if c.version == V2 {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	} else {
		req.Header.Set("x-api-token", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling the Trunk API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newAPIError(resp.StatusCode, respBytes)
	}
	if out != nil && len(bytes.TrimSpace(respBytes)) > 0 {
		if err := json.Unmarshal(respBytes, out); err != nil {
			return fmt.Errorf("unmarshaling response: %w", err)
		}
	}
	return nil
}

func newAPIError(status int, body []byte) *APIError {
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil && envelope.Error.Code != "" {
		return &APIError{StatusCode: status, Code: envelope.Error.Code, Message: envelope.Error.Message}
	}
	msg := strings.TrimSpace(string(body))
	if msg == "" {
		msg = http.StatusText(status)
	}
	return &APIError{StatusCode: status, Message: msg}
}
