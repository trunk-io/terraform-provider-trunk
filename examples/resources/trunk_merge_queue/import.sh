# By repository and target branch. Branch names may contain slashes.
terraform import trunk_merge_queue.main github.com/my-org/my-repo/main

# Or, with an API key, by the queue's v2 ID.
terraform import trunk_merge_queue.main a3e06cc2-798d-4907-a021-7966dd00b555
