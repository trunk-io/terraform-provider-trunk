terraform {
  required_providers {
    trunk = {
      source = "registry.terraform.io/trunk-io/trunk"
    }
  }

  backend "s3" {
    bucket         = "trunk-terraform-state-staging"
    key            = "merge-queues/terraform.tfstate"
    region         = "us-west-2"
    dynamodb_table = "terraform-state-lock"
    encrypt        = true
  }
}

# api_key is read from TRUNK_API_KEY environment variable.
# base_url is read from TRUNK_BASE_URL environment variable.
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
