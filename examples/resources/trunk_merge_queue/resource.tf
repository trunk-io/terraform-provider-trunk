resource "trunk_merge_queue" "main" {
  repo = {
    host  = "github.com"
    owner = "my-org"
    name  = "my-repo"
  }
  target_branch = "main"

  mode         = "parallel"
  concurrency  = 10
  merge_method = "squash"

  batch                          = true
  batching_min_size              = 4
  batching_max_wait_time_minutes = 10
  batching_rules = [
    { target_pattern = "//db/migrations:*", policy = "isolate" },
  ]

  required_statuses = ["build", "test"]
}
