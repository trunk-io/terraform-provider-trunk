# internal/provider

Terraform provider and resource implementations. Bridges the `internal/client` package to the Terraform Plugin Framework.

## Files

- `provider.go` — Provider definition, auth schema, `Configure` (creates `*client.Client` from `api_key`/`TRUNK_API_KEY`), and resource registration
- `merge_queue_resource.go` — `trunk_merge_queue` resource: schema, CRUD operations, and `ImportState`
- `merge_queue_resource_model.go` — Terraform state model (`mergeQueueResourceModel`) and conversion helpers between Terraform types and `internal/client` request/response structs

## Conventions

- Keep all Terraform Plugin Framework imports here; `internal/client` must remain free of them
- `mergeQueueResourceModel.fromQueue` populates state from the API; `toCreateRequest`/`toUpdateRequest` build API request structs
- All configurable fields are Computed + Optional with `UseStateForUnknown`; the API returns every field in getQueue/updateQueue responses
- All enum values are lowercase (e.g., `"running"`, `"squash"`, `"off"`, `"bisection_skip_redundant_tests"`)
- `null` `required_statuses` sends `deleteRequiredStatuses: true` to revert to branch protection / trunk.yaml defaults
- Import ID format: `{host}/{owner}/{name}/{target_branch}` (uses `strings.SplitN(..., 4)` to support branch names with slashes)

## References

- TRD: `docs/trd/terraform-provider-trunk.md`
