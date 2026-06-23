# Task Plan: ImageHandle provider_direct_lease

## Goal
Change async image tasks so new-api selects and locks the real image channel, creates a short-lived credential lease, and submits a `provider_direct_lease` job to image-handle. image-handle resolves the lease before execution, directly calls the real upstream in its worker, uploads R2, and callbacks new-api for billing/assets.

## Current Phase
Implementation and regression testing

## Phases
- [complete] Map current ImageHandle task submission, task persistence, billing, polling, callback, asset creation, and async task config paths.
- [complete] Replace old internal execute contract with `provider_direct_lease` and lease resolve.
- [complete] Add `image_credential_leases` model and migration entry.
- [complete] Refactor `/v1/image/tasks` to create local task + lease before submitting to image-handle.
- [complete] Add signed resolve endpoint returning the locked real channel `base_url/api_key/model`.
- [complete] Extend callback parsing for `usage`, `raw_response`, truncation flags, and safe result URLs.
- [in_progress] Add targeted unit tests and run backend/frontend regression checks.
- [pending] Run live联调 after image-handle finishes its matching worker changes.

## Decisions Made
| Decision | Rationale |
| --- | --- |
| ImageHandle is an async executor, not a model channel | Async image generation must reuse existing real image channels and pricing. |
| Use `provider_direct_lease` | Avoid putting real API keys in task payload while keeping worker execution in image-handle. |
| Lease stores only real `channel_id` | The real key is resolved from existing encrypted/channel config at execution time. |
| Create task and lease before submit | Prevent image-handle worker from resolving a lease that new-api has not persisted yet. |
| Resolve returns plaintext key only over signed internal call | image-handle uses it briefly in worker memory and must not persist or log it. |
| Keep config in Async Task Management | User explicitly wants image-handle executor config there, not in operation settings. |
| Callback secret remains separate from internal resolve secret | Keeps inbound terminal notification trust separate from credential resolve trust. |
| Callback/轮询 still use `ApplyTaskResult` | Existing CAS + DB transaction keeps task terminal update and assets creation atomic. |

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Old docs and plan still described `new_api_internal execute` | Code review | Rewrote the integration doc and plan to `provider_direct_lease`. |
