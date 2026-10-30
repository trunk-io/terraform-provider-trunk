# Not shown in the docs, which render resource.tf only.
terraform {
  required_version = ">= 1.0"
  required_providers {
    trunk = {
      source  = "trunk-io/trunk"
      version = "~> 2.0"
    }
  }
}
