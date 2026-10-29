# Terraform Provider for Trunk

Manage [Trunk](https://trunk.io) merge queues with Terraform or OpenTofu. Documentation is on the [Terraform Registry](https://registry.terraform.io/providers/trunk-io/trunk/latest/docs).

```hcl
terraform {
  required_providers {
    trunk = {
      source  = "trunk-io/trunk"
      version = "~> 2.0"
    }
  }
}

provider "trunk" {
  # Or set TRUNK_API_KEY.
}

resource "trunk_merge_queue" "main" {
  repo = {
    host  = "github.com"
    owner = "my-org"
    name  = "my-repo"
  }
  target_branch = "main"
  mode          = "parallel"
  concurrency   = 10
  merge_method  = "squash"
}
```

## Authentication

Use an API key: an organization admin creates one under **Settings** → **Developer** → **API Keys** in the Trunk app, with the **Update Merge Queue settings** permission. Pass it in `api_key` or `TRUNK_API_KEY`.

The provider picks the API from the credential. An API key (`trunk_v2_...`) uses v2. The organization API token, on the same page, still works but uses v1, which will be deprecated soon.

Upgrading from 1.x? See the [upgrade notes](CHANGELOG.md#200).

## Development

Requires Go (see `go.mod`) and a `terraform` binary on `PATH`.

```sh
make build      # go build ./...
make test       # unit tests, plus Terraform runs against an in-memory fake of the Trunk API
make testacc    # also checks that state from the last 1.x release upgrades (downloads it from the registry)
make generate   # regenerates docs/ from the schema, templates/ and examples/
make lint
```

The tests need no Trunk account: `internal/fakeapi` serves both API versions. Run `make generate` after changing the schema, a template or an example, and commit `docs/`.

### Trying a local build

Build the binary, then point Terraform at it in `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "trunk-io/trunk" = "/path/to/terraform-provider-trunk"
  }
  direct {}
}
```

`terraform plan` and `apply` then use the local build without `terraform init`. Remove the override when you're done.

## Releasing

Push a `v*` tag. The release workflow builds, signs and publishes with GoReleaser, and the registry picks it up.
