terraform {
  required_providers {
    trunk = {
      source = "registry.terraform.io/trunk-io/trunk"
    }
  }
}

# api_key is read from TRUNK_API_KEY environment variable.
# trunk2 targets staging; base_url can also be set via TRUNK_BASE_URL.
provider "trunk" {
  base_url = "https://api.trunk-staging.io/v1"
}

resource "trunk_merge_queue" "trunk2" {
  repo = {
    host  = "github.com"
    owner = "trunk-io"
    name  = "trunk2"
  }
  target_branch = "main"
}
