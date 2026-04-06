# TRD: Terraform Provider for Trunk Merge Queue

## Related Docs

- Trunk Merge Queue public API: `https://api.trunk.io/v1`
- [HashiCorp Terraform Plugin Framework](https://developer.hashicorp.com/terraform/plugin/framework)

## Problem

Users want to manage Trunk merge queue configuration as infrastructure-as-code. Today, queue creation and configuration is done through the UI or ad-hoc API calls. There is no declarative, version-controlled way to manage queue settings.

### Goal

Build a Terraform provider (`terraform-provider-trunk`) that enables declarative management of merge queues via the Trunk public API. The MVP covers a single resource (`trunk_merge_queue`) supporting create, read, update, delete, and import. The provider will be published to the Terraform Registry.

## Approach

### Expected Results

Users can manage merge queue lifecycle and configuration through standard Terraform workflows:

```hcl
resource "trunk_merge_queue" "main" {
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

### Implementation Description

**Provider** (`trunk`): Authenticates via org API token (`api_key` attribute or `TRUNK_API_KEY` env var). Optional `base_url` for staging.

**Resource** (`trunk_merge_queue`): Maps to four Trunk API endpoints:

| Terraform Operation | API Endpoint   | HTTP Method |
| ------------------- | -------------- | ----------- |
| Create              | `/createQueue` | POST        |
| Read                | `/getQueue`    | POST        |
| Update              | `/updateQueue` | POST        |
| Delete              | `/deleteQueue` | POST        |

**Create lifecycle detail:** The `createQueue` API only accepts `repo`, `targetBranch`, `mode`, and `concurrency`. All other configuration is set via `updateQueue`. The resource's `Create` method must:

1. Call `createQueue` with identity fields + `mode` + `concurrency`
2. Immediately call `updateQueue` with all remaining optional attributes
3. Call `getQueue` to read back the full state

If step 2 fails, the resource exists but is partially configured. The `Create` method returns an error with the resource ID set in state, so a subsequent `terraform apply` triggers `Update` to complete configuration.

**Conflict on create (409):** The `createQueue` API returns 409 if a queue already exists for that repo/branch. The `Create` method detects this and returns a descriptive error telling the user to import instead of create:

```text
A merge queue for github.com/my-org/my-repo already exists on branch "main". Import it into Terraform state with:

  terraform import trunk_merge_queue.<name> "github.com/my-org/my-repo/main"
```

**Delete lifecycle detail:** The `deleteQueue` API returns 400 if the queue still has PRs in it. Since the provider always sends well-formed requests, a 400 is treated as "queue not empty" and surfaces a descriptive error telling the user to set `state = "DRAINING"` and wait for the queue to empty before retrying. All other errors (5xx) are propagated as-is. Deleting a non-existent queue is a no-op at the API level.

**Import:** Users can import existing queues via `terraform import trunk_merge_queue.main "github.com/my-org/my-repo/main"` (format: `{host}/{owner}/{name}/{target_branch}`).

**Architecture:** Standalone HTTP client in `internal/client/` separated from Terraform provider logic in `internal/provider/`. The client is independently testable and reusable.

#### Resource Schema

**Computed attributes (set by provider, not configurable):**

| Attribute | Type   | Description                                                |
| --------- | ------ | ---------------------------------------------------------- |
| `id`      | string | Unique identifier: `{host}/{owner}/{name}/{target_branch}` |

**Required attributes (identity -- ForceNew):**

| Attribute       | Type   | Description                          |
| --------------- | ------ | ------------------------------------ |
| `repo.host`     | string | Repository host (e.g., "github.com") |
| `repo.owner`    | string | Repository owner                     |
| `repo.name`     | string | Repository name                      |
| `target_branch` | string | Target branch for the queue          |

**Optional + Computed attributes (configurable; API always returns a value):**

All optional attributes are also `Computed: true` because the API always returns a value for every field in `getQueue` responses. This prevents perpetual plan diffs when the user omits a field that has an API-applied default.

| Attribute                         | Type   | API default | Description                                        |
| --------------------------------- | ------ | ----------- | -------------------------------------------------- |
| `mode`                            | string | `"single"`  | Queue mode: `"single"` or `"parallel"`             |
| `concurrency`                     | int    | `1`         | Number of concurrent test slots                    |
| `state`                           | string | `"RUNNING"` | Queue state: `"RUNNING"`, `"PAUSED"`, `"DRAINING"` |
| `testing_timeout_minutes`         | int    | --          | Max minutes to wait for tests                      |
| `pending_failure_depth`           | int    | --          | PRs below a failure to wait for before eviction    |
| `can_optimistically_merge`        | bool   | `false`     | Optimistic merge when lower PR passes              |
| `batch`                           | bool   | `false`     | Enable batching                                    |
| `batching_max_wait_time_minutes`  | int    | --          | Max minutes to wait for batch to fill              |
| `batching_min_size`               | int    | --          | Minimum PRs per batch                              |
| `merge_method`                    | string | --          | `"MERGE_COMMIT"`, `"SQUASH"`, `"REBASE"`           |
| `comments_enabled`                | bool   | --          | Post GitHub comments on PRs                        |
| `commands_enabled`                | bool   | --          | Allow `/trunk merge` comments                      |
| `create_prs_for_testing_branches` | bool   | --          | Create PRs for testing branches                    |
| `status_check_enabled`            | bool   | --          | Post GitHub status checks                          |
| `direct_merge_mode`               | string | `"OFF"`     | `"OFF"` or `"ALWAYS"`                              |
| `optimization_mode`               | string | `"OFF"`     | `"OFF"` or `"BISECTION_SKIP_REDUNDANT_TESTS"`      |
| `bisection_concurrency`           | int    | --          | Concurrent tests during bisection                  |
| `required_statuses`               | list   | --          | Override required status checks                    |

**Note on `required_statuses`:** This field distinguishes three states:

- Omitted / `null` -- sends `deleteRequiredStatuses: true`, reverting to branch protection / trunk.yaml defaults
- `[]` (empty list) -- sends `requiredStatuses: []`, explicitly requiring no status checks
- `["ci/test"]` -- sends that list as the required statuses

The underlying `UpdateQueueRequest.RequiredStatuses` uses `*[]string` (pointer-to-slice) so that a nil pointer is omitted via `omitempty` while an empty slice `&[]string{}` serialises as `[]`.

### Infrastructure

No cloud infrastructure needed. The provider is a standalone Go binary that communicates with the existing Trunk API at `https://api.trunk.io/v1`.

**Registry publishing** requires:

- GoReleaser for multi-platform builds (`linux_{amd64,arm64}`, `darwin_{amd64,arm64}`, `windows_amd64`)
- GPG signing key for binary verification
- GitHub Actions release workflow (future, not in MVP)

### Configuration

**Provider configuration:**

| Attribute  | Type   | Required | Sensitive | Default                   | Description                                                                                                    |
| ---------- | ------ | -------- | --------- | ------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `api_key`  | string | Yes\*    | Yes       | `TRUNK_API_KEY` env var   | Org-level API token                                                                                            |
| `base_url` | string | No       | No        | `https://api.trunk.io/v1` | API base URL. Can also be set via `TRUNK_BASE_URL` env var. Use `https://api.trunk-staging.io/v1` for staging. |

\*Required unless `TRUNK_API_KEY` environment variable is set.

### Testing Strategy

**Unit tests (client):**

- Mock HTTP server (`httptest.NewServer`) for each API method
- Verify request serialization (URL, headers, body) and response deserialization
- Test error handling for non-2xx responses

**Acceptance tests (provider):**

- Use `terraform-plugin-testing` framework
- Gated by `TF_ACC=1` environment variable
- Require a real `TRUNK_API_KEY` and a test repository
- Test full lifecycle: create -> read -> update -> read -> destroy
- Test import: create externally, import, verify state matches

### Security

- API key marked `Sensitive: true` in Terraform schema (masked in logs/output)
- Environment variable fallback (`TRUNK_API_KEY`) avoids hardcoding secrets in HCL
- All API communication over HTTPS
- No secrets stored in state beyond what Terraform manages

### Metrics & Analytics

No custom metrics in the MVP. Usage is tracked implicitly via Trunk API request logs.

### Documentation

- `README.md` in the provider directory with usage instructions
- Example HCL configs in `examples/`
- Registry documentation auto-generated from schema descriptions

## Files to Create/Modify

- [x] `go/CLAUDE.md` -- Go project area conventions
- [x] `go/terraform-provider-trunk/CLAUDE.md` -- Provider conventions
- [x] `go/terraform-provider-trunk/main.go` -- Provider server entry point
- [x] `go/terraform-provider-trunk/go.mod` / `go.sum` -- Dependencies
- [x] `go/terraform-provider-trunk/.goreleaser.yml` -- Multi-platform build config
- [x] `go/terraform-provider-trunk/internal/provider/provider.go` -- Provider implementation
- [x] `.github/workflows/pr-go.yaml` -- CI workflow for Go
- [x] `go/terraform-provider-trunk/internal/client/types.go` -- Request/response structs
- [x] `go/terraform-provider-trunk/internal/client/client.go` -- HTTP client
- [x] `go/terraform-provider-trunk/internal/client/merge_queue.go` -- Queue API methods
- [x] `go/terraform-provider-trunk/internal/client/client_test.go` -- Client unit tests
- [x] `go/terraform-provider-trunk/internal/client/merge_queue_test.go` -- API method tests
- [x] `go/terraform-provider-trunk/internal/provider/merge_queue_resource.go` -- Resource CRUD
- [x] `go/terraform-provider-trunk/internal/provider/merge_queue_resource_model.go` -- Schema mapping
- [ ] `go/terraform-provider-trunk/internal/provider/merge_queue_resource_test.go` -- Acceptance tests
- [ ] `go/terraform-provider-trunk/internal/provider/provider_test.go` -- Provider tests
- [ ] `go/terraform-provider-trunk/examples/provider/main.tf` -- Provider example
- [ ] `go/terraform-provider-trunk/examples/resources/trunk_merge_queue/main.tf` -- Resource example
- [ ] `go/terraform-provider-trunk/README.md` -- Usage documentation

## Tech Ladder

### Checkpoint 1: Project scaffolding and CI

- [x] Initialize Go module with terraform-plugin-framework dependencies
- [x] Create provider stub with auth schema
- [x] Configure GoReleaser for multi-platform builds
- [x] Add GitHub Actions PR workflow
- [x] Update trunk.yaml for Go tooling

**Done when:** `go build ./...` compiles, `trunk check` passes, CI workflow triggers on Go changes.

### Checkpoint 2: API client

- [x] Implement HTTP client with auth and error handling
- [x] Implement CreateQueue, GetQueue, UpdateQueue, DeleteQueue methods
- [x] Write unit tests with mock HTTP server

**Done when:** `go test ./internal/client/ -v` passes with full coverage of request/response serialization and error paths.

### Checkpoint 3: Merge queue resource

- [x] Implement resource schema with all attributes
- [x] Implement Create (createQueue + updateQueue two-step)
- [x] Implement Read, Update, Delete
- [x] Implement ImportState
- [x] Wire provider Configure to create client

**Done when:** `go build ./...` compiles. Manual test with `dev_overrides` creates, updates, imports, and destroys a queue.

### Checkpoint 4: Testing and documentation

- [ ] Write acceptance tests (lifecycle, update, import)
- [ ] Create example HCL configs
- [ ] Write README with usage instructions

**Done when:** `TF_ACC=1 go test ./internal/provider/ -v` passes. `terraform validate` passes on examples.

## Verification

1. `go build ./...` -- compiles successfully
2. `go test ./...` -- unit tests pass
3. `trunk check` and `trunk fmt` -- linting passes
4. Manual test with `dev_overrides`:
   - Build binary locally
   - Configure `.terraformrc` with dev override
   - Run `terraform plan` and `terraform apply` against a test repo
5. `TF_ACC=1 go test ./internal/provider/ -v` -- acceptance tests pass
