package client

import (
	"context"
	"errors"
	"fmt"
)

// CreateQueue creates a merge queue and applies the rest of its settings.
//
// Neither API creates and configures a queue in one write, so this takes several requests. If a
// later one fails, the queue still exists: the returned Queue is its current state (when it could
// be read) alongside the error, so the caller can record it and finish on the next apply.
func (c *Client) CreateQueue(ctx context.Context, p CreateParams) (*Queue, error) {
	if c.version == V2 {
		return c.createV2(ctx, p)
	}
	return c.createV1(ctx, p)
}

// GetQueue reads a merge queue. It returns ErrNotFound when the queue doesn't exist.
func (c *Client) GetQueue(ctx context.Context, ref QueueRef) (*Queue, error) {
	if c.version == V2 {
		return c.getV2(ctx, ref)
	}
	return c.getV1(ctx, ref)
}

// UpdateQueue applies changes to a merge queue and returns its new state.
//
// While a queue is switching modes it refuses with ErrSwitchingModes: changing its state then
// would abandon the switch on both APIs, and v2 also refuses settings changes.
func (c *Client) UpdateQueue(ctx context.Context, p UpdateParams) (*Queue, error) {
	current, err := c.GetQueue(ctx, p.Ref)
	if err != nil {
		return nil, err
	}
	// GetQueue falls back to the branch when the ID is gone. Writing to that queue would change one
	// Terraform doesn't know about, so refuse before writing.
	if p.Ref.ID != "" && current.ID != "" && current.ID != p.Ref.ID {
		return nil, &QueueReplacedError{OldID: p.Ref.ID, NewID: current.ID}
	}
	if current.State == StateSwitchingModes {
		return nil, fmt.Errorf("%w: run apply again once the switch finishes", ErrSwitchingModes)
	}
	mode := changed(p.Mode, current.Mode)
	state := changed(p.State, current.State)
	if c.version == V2 {
		return c.updateV2(ctx, current, mode, state, p.Settings)
	}
	return c.updateV1(ctx, current, mode, state, p.Settings)
}

// DeleteQueue deletes a merge queue. Deleting one that doesn't exist succeeds. A queue that
// still has pull requests is refused with ErrQueueNotEmpty.
func (c *Client) DeleteQueue(ctx context.Context, ref QueueRef) error {
	if c.version == V2 {
		return c.deleteV2(ctx, ref)
	}
	return c.deleteV1(ctx, ref)
}

// changed returns want when it is set and differs from have, and nil otherwise.
func changed(want *string, have string) *string {
	if want == nil || *want == have {
		return nil
	}
	return want
}

// createFailed handles a create request that failed with something other than a conflict. After
// a server error or a lost response the request may still have created the queue, so it looks for
// one to return with the error, for the caller to record. A 4xx was refused before any write (v2
// checks permissions and input before conflicts), so a queue found then isn't this request's.
func (c *Client) createFailed(ctx context.Context, ref QueueRef, err error) (*Queue, error) {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode < 500 {
		return nil, err
	}
	q, getErr := c.GetQueue(ctx, ref)
	if getErr != nil {
		return nil, err
	}
	return q, fmt.Errorf("the request to create it failed, but it exists: %w", err)
}

// afterCreate wraps a failure that happened once the queue existed, attaching its current state.
// If the queue can't be read either, the error wraps ErrCreatedNotRecorded.
func (c *Client) afterCreate(ctx context.Context, ref QueueRef, err error) (*Queue, error) {
	q, getErr := c.GetQueue(ctx, ref)
	if getErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreatedNotRecorded, err)
	}
	return q, err
}
