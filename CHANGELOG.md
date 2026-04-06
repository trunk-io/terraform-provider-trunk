# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `trunk_merge_queue` resource for managing Trunk merge queues
  - Full CRUD operations (create, read, update, delete)
  - Import support via `terraform import`
  - Configurable queue mode (single/parallel), concurrency, merge method, batching, and more
  - Override required status checks or revert to branch protection defaults
- Provider authentication via `api_key` attribute or `TRUNK_API_KEY` environment variable
- Configurable API base URL via `base_url` attribute or `TRUNK_BASE_URL` environment variable
