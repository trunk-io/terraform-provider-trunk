# Terraform Provider for Trunk

Manages Trunk merge queues (`trunk_merge_queue`) through the Trunk API.

## Layout

- `internal/client/` — the API client, free of Terraform types. See [its notes](internal/client/CLAUDE.md).
- `internal/provider/` — the provider and resource, on the Terraform Plugin Framework. See [its notes](internal/provider/CLAUDE.md).
- `internal/fakeapi/` — an in-memory fake of the v1 and v2 merge queue APIs that the tests run against.
- `templates/` and `examples/` — sources for `docs/`, which `make generate` (tfplugindocs) writes. Don't edit `docs/` by hand.

## Commands

```sh
make build
make test       # needs terraform on PATH; no Trunk account
make testacc    # adds the upgrade test from the last 1.x release (downloads it)
make generate   # after any schema, template or example change; commit docs/
make lint
```

## Two APIs

The credential picks the API: a `trunk_v2_` key uses v2 (bearer auth, queues by ID, camelCase enums, a separate endpoint for mode and for each state); anything else is the organization API token on v1 (`x-api-token`, queues by repo and branch, one `updateQueue`). The client hides the difference, so the resource never branches on the version except for `queue_id`, `managed_state` and import by ID, which only v2 has.

When adding a setting, add it to both APIs' request and response types in `internal/client`, to `Settings` and `Queue`, to the schema and model, and to both versions of `internal/fakeapi`. The trunk1 merge service accepts every queue field, but each API exposes only what its own layer maps (v1: trunk-io/trunk `public-api`; v2: trunk-io/trunk2 `ts/apps/public-api`).

Both APIs mark a queue Terraform-managed when a write carries `x-source: terraform` (or a User-Agent naming Terraform). Keep sending it.
