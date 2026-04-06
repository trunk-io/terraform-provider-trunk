# Terraform Provider for Trunk.io

Terraform provider for managing [Trunk.io](https://trunk.io) merge queue configuration.

## Architecture

- `internal/client/` — Standalone HTTP client for the Trunk API (no Terraform dependency). See [internal/client/CLAUDE.md](internal/client/CLAUDE.md).
- `internal/provider/` — Terraform Plugin Framework resources and provider definition. See [internal/provider/CLAUDE.md](internal/provider/CLAUDE.md).
- `main.go` — Provider server entry point.

## Build

```bash
go build -o terraform-provider-trunk .
```

## Tests

```bash
# Unit tests (no API key required)
make test

# Acceptance tests (requires a real API key and test repository)
TF_ACC=1 TRUNK_API_KEY=<key> make testacc
```

## Local Testing with dev_overrides

To test a locally-built provider against a real Terraform config without publishing to the Registry:

1. **Build the binary** in the repo root:

   ```bash
   go build -o terraform-provider-trunk .
   ```

2. **Create `~/.terraformrc`** with dev_overrides pointing at the repo root:

   ```hcl
   provider_installation {
     dev_overrides {
       "trunk-io/trunk" = "/path/to/terraform-provider-trunk"
     }
     direct {}
   }
   ```

3. **Run Terraform** in a directory with a config that uses the `trunk-io/trunk` provider:

   ```bash
   export TRUNK_API_KEY=<your-api-key>
   terraform apply    # no `terraform init` needed with dev_overrides
   ```

4. **Verify** with `terraform plan` — expect no changes if the provider is working correctly.

5. **Clean up** — remove or revert `~/.terraformrc` when done so you return to using the Registry version.

> **Note:** `terraform init` is not required (and will warn) when `dev_overrides` is active.
