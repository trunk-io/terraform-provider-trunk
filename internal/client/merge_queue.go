package client

import "context"

// CreateQueue creates a new merge queue. Per the API contract, only repo, targetBranch,
// mode, and concurrency are accepted; remaining configuration must be applied via UpdateQueue.
func (c *Client) CreateQueue(ctx context.Context, req CreateQueueRequest) (*Queue, error) {
	var resp CreateQueueResponse
	if err := c.doRequest(ctx, "createQueue", req, &resp); err != nil {
		return nil, err
	}
	return &resp.Queue, nil
}

// GetQueue retrieves the current state of a merge queue.
func (c *Client) GetQueue(ctx context.Context, req GetQueueRequest) (*Queue, error) {
	var resp getQueueAPIResponse
	if err := c.doRequest(ctx, "getQueue", req, &resp); err != nil {
		return nil, err
	}
	return resp.toQueue(req.Repo, req.TargetBranch), nil
}

// UpdateQueue updates configuration on an existing merge queue. Only non-nil pointer fields
// in the request are sent to the API, leaving unspecified fields unchanged.
func (c *Client) UpdateQueue(ctx context.Context, req UpdateQueueRequest) (*Queue, error) {
	var resp UpdateQueueResponse
	if err := c.doRequest(ctx, "updateQueue", req, &resp); err != nil {
		return nil, err
	}
	return &resp.Queue, nil
}

// DeleteQueue deletes a merge queue.
func (c *Client) DeleteQueue(ctx context.Context, req DeleteQueueRequest) error {
	return c.doRequest(ctx, "deleteQueue", req, nil)
}
