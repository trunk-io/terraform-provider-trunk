terraform {
  required_providers {
    trunk = {
      source = "registry.terraform.io/trunk-io/trunk"
    }
  }
}

# Configure the Trunk provider.
# api_key can be set here or via the TRUNK_API_KEY environment variable.
provider "trunk" {}
