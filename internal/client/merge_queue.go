package client

import (
	"context"
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

// afterCreate wraps a failure that happened once the queue already existed, attaching its
// current state when it can be read.
func (c *Client) afterCreate(ctx context.Context, ref QueueRef, err error) (*Queue, error) {
	q, getErr := c.GetQueue(ctx, ref)
	if getErr != nil {
		q = nil
	}
	return q, fmt.Errorf("the merge queue was created, but configuring it failed: %w", err)
}
