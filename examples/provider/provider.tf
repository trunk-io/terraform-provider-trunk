terraform {
  required_version = ">= 1.0"
  required_providers {
    trunk = {
      source  = "trunk-io/trunk"
      version = "~> 2.0"
    }
  }
}

# Set the credential here or in the TRUNK_API_KEY environment variable.
# An API key (trunk_v2_...) uses the v2 API; the organization API token uses v1.
provider "trunk" {}
