# internal/client

The Trunk API client. No Terraform imports here.

- `client.go` — HTTP, auth, retries and `APIError`. `NewClient` picks v1 or v2 from the credential.
- `types.go` — the version-neutral `Queue`, `Settings` and params, in the provider's snake_case spelling.
- `merge_queue.go` — the public methods; they dispatch to `v1.go` or `v2.go`.

## Conventions

- `Settings` fields are pointers; nil means "leave unchanged". An empty `RequiredStatuses` means "remove the override" (v1 needs `deleteRequiredStatuses` for that; v2 takes `[]`). Neither API can store an empty override.
- Methods return `ErrNotFound`, `ErrAlreadyExists`, `ErrQueueNotEmpty` and `ErrSwitchingModes` for the cases the resource handles; check them with `errors.Is`.
- `UpdateQueue` reads the queue first, sends `Mode`/`State` only when they differ, and refuses while the queue is switching modes: a state change then abandons the switch on both APIs.
- Creates retry only on 429. A retried create after a lost response would report a conflict with itself.
- v2 stores the not-ready timeout in minutes (whole hours); the client converts.
