package client

import "context"

// CreateQueue creates a new merge queue. Per the API contract, only repo, targetBranch,
// mode, and concurrency are accepted; remaining configuration must be applied via UpdateQueue.
// The createQueue endpoint returns an empty response body.
func (c *Client) CreateQueue(ctx context.Context, req CreateQueueRequest) error {
	return c.doRequest(ctx, "createQueue", req, nil)
}

// GetQueue retrieves the current state of a merge queue.
func (c *Client) GetQueue(ctx context.Context, req GetQueueRequest) (*Queue, error) {
	var q Queue
	if err := c.doRequest(ctx, "getQueue", req, &q); err != nil {
		return nil, err
	}
	// Identity fields are not in the response body; populate from the request.
	q.Repo = req.Repo
	q.TargetBranch = req.TargetBranch
	return &q, nil
}

// UpdateQueue updates configuration on an existing merge queue and returns the updated state.
func (c *Client) UpdateQueue(ctx context.Context, req UpdateQueueRequest) (*Queue, error) {
	var q Queue
	if err := c.doRequest(ctx, "updateQueue", req, &q); err != nil {
		return nil, err
	}
	// Identity fields are not in the response body; populate from the request.
	q.Repo = req.Repo
	q.TargetBranch = req.TargetBranch
	return &q, nil
}

// DeleteQueue deletes a merge queue.
func (c *Client) DeleteQueue(ctx context.Context, req DeleteQueueRequest) error {
	return c.doRequest(ctx, "deleteQueue", req, nil)
}
