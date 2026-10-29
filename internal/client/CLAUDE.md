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
- v2 takes the not-ready timeout in minutes (multiples of 60 only); the client converts.
- v1's `getQueue` answers 404 for a bad token or an unknown repository too, so a 404 is confirmed with a no-op `updateQueue` (401 for those, 404 only when the queue is gone) before returning `ErrNotFound`.
- v2 matches repository names exactly, v1 ignoring case. A failed v2 lookup is `ErrRepositoryNotFound`, never `ErrNotFound`; reads fall back to matching every queue's `repositoryUrl` ignoring case, keeping the caller's spelling.
- v2 creates send only `mode` and `concurrency`: v2 applies the other create fields in a second write after the queue exists. Any failed create looks the queue up and returns it with the error.
- Mode changes are retried only on 429: trunk1 restarts a switch from `switching_modes`, and v2 refuses the retry.
