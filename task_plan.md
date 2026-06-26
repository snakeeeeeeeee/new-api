# Task Plan: ImageHandle sync URL/base64 compatibility

## Goal
Support image-handle's latest `result_data_format` contract for gray-enabled synchronous image execution. `/v1/images/generations` and `/v1/images/edits` must keep old direct-upstream behavior when the switch is off, return URL results by default when image-handle sync is enabled, return `b64_json` when the client requests `response_format=b64_json`, and keep ordinary async `/v1/image/tasks` URL-only.

## Current Phase
Complete

## Phases
- [complete] Map current sync image-handle path, async image adaptor, and updated image-handle docs.
- [complete] Add `result_data_format` to sync payloads and response conversion.
- [complete] Restrict `/v1/images/edits` sync mode to URL-only input/mask and fall back otherwise.
- [complete] Reject async `/v1/image/tasks` requests that explicitly ask for base64.
- [complete] Add unit tests for payloads, URL/base64 response mapping, edit fallback, and async rejection.
- [complete] Run Go/frontend regression checks.
- [complete] Build Docker dev and联调 switch-off, URL sync, base64 sync, force_on/force_off, edit URL/non-URL, failed/202 handling as far as local image-handle permits.

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
| Sync image-handle `base64` is response-only | It must not be saved to assets, callback, or resource center. |
| Async image tasks remain URL-only | image-handle docs reject `result_data_format=base64` on `/v1/image/tasks`; new-api should fail fast with 400. |
| Edit sync only for URL inputs | Existing multipart/base64 edit clients must keep old direct-upstream behavior. |
| Channel override lives in `channels.settings` | `channels.other` is legacy; UI and backend read/write `settings` for `image_handle_sync_mode` and `callback_secret`. |

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Old docs and plan still described `new_api_internal execute` | Code review | Rewrote the integration doc and plan to `provider_direct_lease`. |
| Local image-handle mock edit route returned 415 for multipart edits | Docker联调 | Verified URL edit input reached image-handle sync and new-api refunded on failed terminal status; non-URL multipart edit correctly fell back to direct upstream. The 415 is a mock-new-api multipart parser limitation, not a new-api routing issue. |
| Channel `force_on` did not appear to work during first SQL test | Docker联调 | Test SQL wrote `image_handle_sync_mode` to legacy `channels.other`; corrected to `channels.settings`, matching frontend/backend field usage. |
| image-handle 202 processing could not be triggered safely in Docker | Docker联调 | Current local `SYNC_TASK_TIMEOUT_MS` is 300s. Added unit coverage for HTTP 202 -> `image_handle_sync_timeout`; did not wait 300s in Docker. |
