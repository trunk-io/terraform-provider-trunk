terraform {
  required_version = ">= 1.0"
  required_providers {
    trunk = {
      source  = "trunk-io/trunk"
      version = "~> 0.1"
    }
  }
}

# Configure the Trunk provider.
# api_key can be set here or via the TRUNK_API_KEY environment variable.
provider "trunk" {}
