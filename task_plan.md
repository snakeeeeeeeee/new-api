# Task Plan: ImageHandle Internal Executor

## Goal
Change async image tasks so new-api reuses existing real image channels instead of treating ImageHandle as a model-serving channel. `/v1/image/tasks` should select and lock the real upstream channel, prebill and create the task, submit an executor task to image-handle with a container-reachable `execute_url`, then serve a signed idempotent internal execute endpoint for image-handle workers.

## Current Phase
Async task menu configuration complete

## Phases
- [complete] Map current ImageHandle task submission, task persistence, billing, polling, callback, and channel config paths.
- [complete] Design the minimal data contract changes for executor submission and internal execute secrets.
- [complete] Implement new-api internal execute endpoint with HMAC validation and idempotency.
- [complete] Refactor `/v1/image/tasks` submit path to lock the real channel while submitting to image-handle executor.
- [complete] Update polling/callback behavior and docs for the new architecture.
- [complete] Add targeted tests and run backend/frontend regression checks.
- [complete] Rebuild local Docker dev and verify async image submit, internal execute, callback, polling fallback, and asset creation.
- [complete] Move image-handle executor configuration into the async task management menu and persist it in options.

## Decisions Made
| Decision | Rationale |
| --- | --- |
| ImageHandle is an async executor, not a model channel | The user wants async image generation to reuse existing `gpt-image-2` and other real image channels. |
| image-handle will remove provider-direct execution | The image-handle team accepted `new_api_internal`; new-api must own the real upstream call. |
| internal execute secret must differ from callback secret | Keeps inbound callback trust separate from worker-to-new-api execution trust. |
| `execute_url` must be container-reachable | image-handle runs in Docker/production networks and cannot depend on browser/public-only URLs. |
| Keep task `platform=58` and real `channel_id` | Platform identifies the image-handle executor protocol; channel_id remains the real image provider used for billing/logging and internal execute. |
| Callback secret comes from the selected real image channel | image-handle sends `X-Callback-Secret-Id: channel_<channel_id>` and new-api resolves that channel's `callback_secret`. |
| Cache non-retry internal execute results | Prevents duplicate real upstream image generation when image-handle retries after a response/network issue. |
| image-handle executor config belongs in async task management | The user does not want this operational task setting mixed into the general operation settings page. |
| Config secrets can be echoed to admins | The user explicitly prefers values to be reviewable after saving for deployment checks. |

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| ImageHandle tests failed because config validation ran before request validation | Targeted test | Reordered validation so malformed client task IDs still return `invalid_request`. |
| Local callback URL used `localhost` and image-handle could not deliver it from Docker | Docker dev test | Set local `CustomCallbackAddress`/`ServerAddress` to `http://host.docker.internal:3001` and verified callback delivery. |
