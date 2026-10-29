# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.0.0]

### Upgrading from 1.x

Change the version constraint to `~> 2.0` and run `terraform init -upgrade`. Existing state loads as is, the first plan is empty, and the organization API token keeps working.

- **Leaving out `required_statuses` no longer clears the queue's override.** 1.x reset it on every apply. To remove an override, set `required_statuses = []`. (`[]` never meant "require no checks": 1.x documented that, but the API ignored an empty list.)
- **`required_statuses` is a set**, so order doesn't matter. References such as `required_statuses[0]` need `tolist()`.
- **`base_url` is the API host**, such as `https://api.trunk.io`. A value ending in `/v1` still works.
- **To use an API key**, swap it in for the token. Nothing else changes; `queue_id` and `managed_state` fill in on the next refresh.

### Added

- API keys. A `trunk_v2_` key uses the v2 API; the organization API token still works on v1, with the same settings.
- `trunk_merge_queue` settings: `test_branch_construction_mode` and `batching_rules`.
- `queue_id`, and `managed_state` (whether the queue has been changed outside Terraform). Only reported with an API key.
- Import by v2 queue ID, with an API key.
- `testing_timeout_minutes = 0` (no timeout), and with an API key, `batching_max_wait_time_minutes = 0` (don't wait).

### Fixed

- Applying `required_statuses` in any order but alphabetical failed with "Provider produced inconsistent result after apply".
- A refresh with a wrong or revoked organization API token, or for a repository Trunk can't find, removed the queue from state. v1 answers both as "not found"; the provider now checks, and reports the token or repository.
- While a queue switched modes, the next apply sent its state back, which abandoned the switch. Terraform now shows the new mode and refuses changes until the switch finishes.
- A retried mode change could leave a queue switching modes for good. Mode changes are no longer retried after a server error.
- A create that failed after the queue was made left nothing in state, so the next apply failed with "already exists". The provider now checks the branch before creating, and after a server error looks for the queue and records it; Terraform replaces it on the next apply, or `terraform untaint` keeps it.
- Creates are no longer retried after a server error, which could turn a lost response into "already exists".
- Requests that hit the rate limit (429) are retried.
- The "queue is not empty" error and the example used uppercase values (`DRAINING`, `SQUASH`) the provider rejects.
- A v1 401 now says the repository may not be connected to Trunk, since v1 also answers that way for an unknown repository.

## [1.1.0] - 2026-07-01

### Added

- `trunk_merge_queue`: `not_ready_timeout_hours`.

## [1.0.4] - 2026-06-16

### Added

- `trunk_merge_queue`: `extension_enabled`, `enqueueing_label`, `label_commands_enabled` and `state_labels_enabled`.

## [0.1.3] - 2026-04-09

- First releases: the `trunk_merge_queue` resource, with create, read, update, delete and import, through the v1 API with the organization API token.

[Unreleased]: https://github.com/trunk-io/terraform-provider-trunk/compare/v2.0.0...HEAD
[2.0.0]: https://github.com/trunk-io/terraform-provider-trunk/compare/v1.1.0...v2.0.0
[1.1.0]: https://github.com/trunk-io/terraform-provider-trunk/compare/v1.0.4...v1.1.0
[1.0.4]: https://github.com/trunk-io/terraform-provider-trunk/compare/v0.1.3...v1.0.4
[0.1.3]: https://github.com/trunk-io/terraform-provider-trunk/releases/tag/v0.1.3
