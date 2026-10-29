# internal/provider

The provider (`provider.go`) and the `trunk_merge_queue` resource (`merge_queue_resource.go`, `merge_queue_resource_model.go`).

## Conventions

- Every setting is Optional + Computed with `UseStateForUnknown`. Only attributes set in the configuration are sent; an omitted one is left as the queue has it. `settings()` builds the request from config-set attributes.
- `required_statuses` and `batching_rules` are sets: the API sorts required statuses, and order means nothing for either.
- `fromQueue` writes the API's view into the model. While the queue is switching modes it keeps the expected mode and state, so an apply that starts a switch matches its plan and the next plan doesn't fight it.
- `ModifyPlan` plans `managed_state`: unchanged when nothing else changes, `terraform` when an apply will write (v2 only; null on v1).
- Enum values are lowercase snake_case, matching v1.
- Import IDs: `{host}/{owner}/{name}/{target_branch}` (`SplitN(..., 4)` keeps slashes in branch names), or a v2 queue ID with an API key.

## Tests

`merge_queue_resource_test.go` runs Terraform against `internal/fakeapi` for both credentials (`resource.UnitTest`, no `TF_ACC`). `upgrade_test.go` applies with the last 1.x release from the registry, then plans with this build; it needs `TF_ACC=1`.
