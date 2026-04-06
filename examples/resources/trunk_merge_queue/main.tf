resource "trunk_merge_queue" "example" {
  repo = {
    host  = "github.com"
    owner = "my-org"
    name  = "my-repo"
  }
  target_branch = "main"
  mode          = "parallel"
  concurrency   = 3
  merge_method  = "SQUASH"
}
