# internal/client

The Trunk API client. No Terraform imports here.

- `client.go` — HTTP, auth, retries and `APIError`. `NewClient` picks v1 or v2 from the credential.
- `types.go` — the version-neutral `Queue`, `Settings` and params, in the provider's snake_case spelling.
- `merge_queue.go` — the public methods; they dispatch to `v1.go` or `v2.go`.

## Conventions

- `Settings` fields are pointers; nil means "leave unchanged". An empty `RequiredStatuses` means "remove the override" (v1 needs `deleteRequiredStatuses` for that; v2 takes `[]`). Neither API can store an empty override.
- Methods return `ErrNotFound`, `ErrAlreadyExists`, `ErrQueueNotEmpty`, `ErrCreatedNotRecorded`, `ErrCreateUnknown` (check with `errors.Is`) and `*QueueReplacedError` (`errors.As`) for the cases the resource handles; `ErrSwitchingModes` carries its own advice.
- `UpdateQueue` reads the queue first, sends `Mode`/`State` only when they differ, and refuses while the queue is switching modes: a state change then abandons the switch on both APIs.
- Reads and other safe requests retry up to 3 times, with backoff, on 429, 5xx, network errors and timeouts. Creates and mode changes retry only on 429. A retried create after a lost response would report a conflict with itself.
- v2 takes the not-ready timeout in minutes (multiples of 60 only); the client converts.
- v1's `getQueue` answers 404 for a bad token or an unknown repository too, so a 404 is confirmed with a no-op `updateQueue` (401 for those, 404 only when the queue is gone) before returning `ErrNotFound`. `getQueue` also hard-codes the host as `github.com`, so for any other host a 404 is expected even when the queue exists: return `ErrV1HostUnsupported` without writing (Read keeps state and warns; Create skips the pre-check on those hosts).
- v2 matches repository names exactly, v1 ignoring case. A failed v2 lookup is `ErrRepositoryNotFound`, never `ErrNotFound`; reads fall back to matching every queue's `repositoryUrl` ignoring case, keeping the caller's spelling.
- A GetQueue with a queue ID that 404s is `ErrNotFound` (the managed queue is gone). It does not adopt a different queue on the branch. `UpdateQueue` looks the branch up itself and returns `QueueReplacedError` so a no-refresh plan can't overwrite the replacement.
- v2 creates send only `mode` and `concurrency`: v2 applies the other create fields in a second write after the queue exists. `CreateQueue` first checks that no queue is on the branch (`ErrAlreadyExists` if one is), so a queue found after a create that fails with a 5xx or a network error is that create's (barring a concurrent create): it is returned with the error. If the check after the failure fails too, the error is `ErrCreateUnknown`. A 4xx was refused before any write, so it doesn't look.
- v2 can return several records for one repository. Reads try each; creates refuse rather than guess.
- `UpdateQueue` refuses with `QueueReplacedError` before writing when the queue on the branch has a different ID than the reference. `DeleteQueue` with an ID deletes only that queue: if it's gone, it succeeds.
- Mode changes are retried only on 429: trunk1 restarts a switch from `switching_modes`, and v2 refuses the retry.
