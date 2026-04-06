# Terraform Provider for Trunk.io

The Trunk provider lets you manage [Trunk.io](https://trunk.io) services with Terraform. Currently supports merge queue configuration.

## Requirements

- [Terraform](https://www.terraform.io/downloads) >= 1.0
- [Go](https://golang.org/dl/) >= 1.24 (to build the provider from source)

## Usage

```hcl
terraform {
  required_providers {
    trunk = {
      source  = "trunk-io/trunk"
      version = "~> 0.1"
    }
  }
}

provider "trunk" {
  # api_key can be set here or via the TRUNK_API_KEY environment variable
}

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
```

Full documentation is available on the [Terraform Registry](https://registry.terraform.io/providers/trunk-io/trunk/latest/docs).

## Authentication

Set your Trunk API key via the `TRUNK_API_KEY` environment variable or the `api_key` provider attribute. Org-level API tokens are required.

## Development

### Build

```bash
go build ./...
```

### Test

```bash
# Unit tests (no API key required)
go test ./internal/client/ -v

# Acceptance tests (requires a real API key and a test repository)
TF_ACC=1 TRUNK_API_KEY=<key> go test ./internal/provider/ -v -timeout 10m
```

### Local testing with Terraform

Build the binary and configure `dev_overrides` to bypass the registry:

```bash
go build -o terraform-provider-trunk .
```

Add to `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/trunk-io/trunk" = "/path/to/terraform-provider-trunk"
  }
  direct {}
}
```

Then run `terraform plan` in any example directory — no `terraform init` needed with `dev_overrides`.
