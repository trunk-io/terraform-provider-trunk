# internal/client

Standalone HTTP client for the Trunk API. Intentionally free of Terraform types — this package can be imported and tested without any Terraform dependency.

## Conventions

- Keep this package free of `terraform-plugin-framework` imports
- `CreateQueue` returns `error` only (empty response body); `GetQueue` and `UpdateQueue` return `(*Queue, error)`
- `APIError` is the error type for non-2xx responses; callers in `internal/provider/` should type-assert it to detect specific status codes (e.g. 404 for "queue not found, remove from state")
- Optional fields use pointer types (`*string`, `*int`, `*bool`) with `omitempty` so nil values are omitted from the JSON body

## References

- TRD: `docs/trd/terraform-provider-trunk.md`
