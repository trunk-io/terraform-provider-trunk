package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.trunk.io/v1"

// Client communicates with the Trunk API.
type Client struct {
	baseURL        string
	apiKey         string
	httpClient     *http.Client
	maxRetries     int
	baseRetryDelay time.Duration
}

// NewClient creates a new Client. If baseURL is empty, it defaults to the production Trunk API.
func NewClient(apiKey, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL:        strings.TrimRight(baseURL, "/"),
		apiKey:         apiKey,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		maxRetries:     3,
		baseRetryDelay: 500 * time.Millisecond,
	}
}

// doRequest sends a POST request to the given endpoint, marshaling reqBody as JSON and
// unmarshaling the response into respBody. Returns an *APIError for non-2xx responses.
// Retries up to maxRetries times with exponential backoff on 5xx errors and network failures.
func (c *Client) doRequest(ctx context.Context, endpoint string, reqBody any, respBody any) error {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
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

		lastErr = c.doRequestOnce(ctx, endpoint, bodyBytes, respBody)
		if lastErr == nil {
			return nil
		}

		// Only retry on 5xx errors or network failures; not on 4xx.
		if apiErr, ok := lastErr.(*APIError); ok && apiErr.StatusCode < 500 {
			return lastErr
		}
	}
	return lastErr
}

func (c *Client) doRequestOnce(ctx context.Context, endpoint string, bodyBytes []byte, respBody any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/"+strings.TrimPrefix(endpoint, "/"), bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-token", c.apiKey)
	req.Header.Set("x-source", "terraform")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{StatusCode: resp.StatusCode, Body: string(respBytes)}
	}

	if respBody != nil {
		if err := json.Unmarshal(respBytes, respBody); err != nil {
			return fmt.Errorf("unmarshaling response: %w", err)
		}
	}

	return nil
}
