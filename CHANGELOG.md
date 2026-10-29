# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.0.0]

### Upgrading from 1.x

- **`required_statuses` is left alone when you leave it out.** In 1.x, leaving it out reset the queue's required statuses to `.trunk/trunk.yaml` or branch protection on every apply, removing any override set in the Trunk app. To reset them now, set `required_statuses = []`.
- **`required_statuses` is a set.** Order no longer matters. Existing state upgrades without changes.
- **`base_url` is the API host**, such as `https://api.trunk.io`. A value ending in `/v1` still works.

### Added

- API keys. A `trunk_v2_` key uses the v2 API; the organization API token keeps using v1. Both manage every setting below.
- `trunk_merge_queue` settings: `test_branch_construction_mode` and `batching_rules`.
- `queue_id` and `managed_state` (whether the queue has been changed outside Terraform). Only reported with an API key.
- Import by v2 queue ID, with an API key.
- `testing_timeout_minutes = 0` (no timeout). With an API key, `bisection_concurrency = 0` and `batching_max_wait_time_minutes = 0`.

### Fixed

- Applying `required_statuses` in any order but alphabetical failed with "Provider produced inconsistent result after apply".
- While a queue switched modes, the next apply sent its state back, which abandoned the switch. Terraform now shows the new mode and refuses changes until the switch finishes.
- A create that failed after the queue was made left nothing in state, so the next apply failed with "already exists". The queue is now recorded and the next apply finishes configuring it.
- Creates are no longer retried after a server error, which could turn a lost response into "already exists". Rate-limited requests are retried.
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
