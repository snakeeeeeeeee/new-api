# GPT Cache-Write Billing Progress

## 2026-07-11
- Resumed the completed backend/frontend implementation from the prior context.
- Confirmed backend coverage includes Responses, Chat, Compact, streaming, non-streaming, and format conversions.
- Confirmed frontend tests cover old/new logs, Claude split fields, explicit zero, and configured ratio zero.
- Started independent backend review and Docker/live integration reconnaissance in parallel.
- Independent review found and is fixing a fixed-price log-classification issue; OpenRouter precedence tests are being added.
- Live reconnaissance confirmed sub2api passes through official `cache_write_tokens`; sampled authorized calls returned explicit zero and can be re-run after the Docker rebuild.
- Fixed-price and OpenRouter review findings were corrected with passing targeted tests.
- Unified the new unconfigured-write frontend wording across all locales.
- Targeted Go tests passed for `dto`, `relay/helper`, `relay/channel/openai`, `service/openaicompat`, and `service`.
- Bun unit tests passed 9/9; `git diff --check` passed.
- Full `go test -count=1 ./...`, targeted `go vet`, and `bun run build` passed; only existing frontend build warnings remain.
- Built image `new-api-local:dev` from the final code, force-recreated `new-api-dev`, and confirmed `/api/status` is healthy on port 3001.
- Authorized live sub2api retest passed for two non-streaming requests and one streaming request; explicit-zero cache write billing and quota matched exactly.
- Started a temporary deterministic OpenAI/Anthropic mock on host port 39001 and verified it is reachable from the rebuilt application container.
- Added and verified isolated temporary Docker-dev models/group/two channels/seven abilities/one token, then restarted the app so ratio and routing caches use the fixtures.
- Deterministic Responses fixtures passed for configured, unconfigured, missing, explicit zero, negative, and oversized writes; planned configured/unconfigured dollar amounts matched exactly.
- Deterministic Chat and Responses streaming/non-streaming requests all returned the official write field for configured and unconfigured models.
- Compact configured/unconfigured/zero requests passed after adding the internal suffixed fixtures.
- Claude 5m/1h compatibility passed with unchanged split fields and no new GPT log flags.
- Attempted desktop/narrow visual log verification; the local management session was expired, so no credentials were changed and that visual check remains unavailable.
- Removed every temporary mock token/channel/ability/ratio/group key, stopped and deleted the mock source, confirmed original token 122/channel 85/create ratio 1.25 remained intact, restarted Docker dev, and rechecked health.
- Final `git diff --check` and temporary-resource audit passed; worktree scope matches implementation plus task planning records, while unrelated user files remain untouched.
- Added the final hybrid-log explicit-zero regression; Bun tests, targeted ESLint, frontend build, and diff check passed again.
- Rebuilt and recreated Docker dev from the final source; `http://localhost:3001/api/status` is healthy.

---

# Progress

## 2026-06-23
- User clarified that ImageHandle should not be a duplicate model channel. Async image tasks should reuse existing real image channels.
- image-handle team accepted the `new_api_internal` executor model and will remove provider-direct execution.
- Created a Markdown handoff document for image-handle at `docs/image-handle-new-api-internal-executor.md`.
- Replaced old asset-center planning files with this internal executor implementation plan.
- Added new-api executor env config, `executor.new_api_internal` task submission payload, signed internal execute route, and request snapshot storage.
- Refactored `/v1/image/tasks` to force the ImageHandle task adaptor while preserving the selected real image channel on `task.channel_id`.
- Aligned with image-handle's final integration doc: provider-direct mode is removed, callback secret comes from the selected real image channel, and internal execute secret is separate.
- Added async image edit reconstruction from saved input URLs to multipart `/v1/images/edits` requests using existing download safety checks.
- Added internal execute result caching and retryable failure claim release so repeated worker calls do not duplicate upstream generation.
- Updated `.env.example` and `docs/image-handle-new-api-internal-executor.md`.
- Rebuilt local Docker dev image and tested against the running image-handle Docker service with `PROVIDER_API_KEYS=test-api-key`.
- Local Docker callback test succeeded with `task_codex_callback_1782166197`: new-api returned queued, image-handle called internal execute, uploaded R2, delivered batch callback, new-api moved the task to `SUCCESS`, and wrote one image asset.
- Added `image_handle_setting` persisted configuration, dedicated admin APIs under `/api/task/async/image-handle/config`, and an `异步图片执行器` card inside `异步任务管理`.
- Updated channel edit UI so real image channels can display, save, and clear `异步图片 Callback Secret`; the field is no longer limited to the deprecated ImageHandle model-channel type.

## 2026-06-24
- Switched the async image integration from old `new_api_internal execute` to `provider_direct_lease`.
- Added `image_credential_leases` for short-lived credential leases. The lease stores the locked real `channel_id`, task reference, model, operation, status, and expiry, but never stores plaintext provider keys.
- Added signed resolve endpoint `/api/internal/image/credential-leases/:lease_id/resolve`, returning OpenAI-compatible `base_url/api_key/model/channel_id` for the locked real channel.
- Refactored ImageHandle submit payload to send `executor.type=provider_direct_lease`, `lease_id`, `resolve_url`, and `secret_id`; no `execute_url` or real upstream key is included.
- Changed async image task creation so `tasks` and `image_credential_leases` are inserted in the same DB transaction before image-handle submission.
- Extended callback parsing for `usage.input_tokens/output_tokens`, `raw_response`, `raw_response_truncated`, and `raw_response_omitted_fields`, with a 256KB raw response cap.
- Rewrote `docs/image-handle-new-api-internal-executor.md` to describe the new lease protocol.

## Test Results
| Test | Status | Notes |
| --- | --- | --- |
| `go test ./controller ./relay/channel/task/imagehandle ./relay ./model ./service` | passed | Covers internal execute HMAC success/failure and image-handle executor payload. |
| `go test ./...` | passed | Full backend regression after internal executor refactor. |
| `go test ./...` | passed | Full backend regression after adding async task menu image-handle config. |
| `cd web && bun run build` | passed | Frontend build after adding async task config card and callback secret field changes. |
| `go test ./controller ./relay/channel/task/imagehandle ./service ./model` | passed | Re-run after image-handle final doc alignment and edit support. |
| `docker compose -f docker-compose-dev.yml up -d --build --force-recreate new-api-dev` | passed | Built `new-api-local:dev` and recreated the dev container. |
| Local `/v1/image/tasks` submit against image-handle | passed | `task_codex_callback_1782166197` reached `SUCCESS`; callback event `evt_d10d4cc7-21f9-4777-9af2-531c3305cbf1` was delivered on first attempt. |
| Local asset query | passed | `/api/assets/self?task_id=task_codex_callback_1782166197` returned one available image asset. |
| `go test ./controller ./relay/channel/task/imagehandle ./relay` | passed | Covers provider_direct_lease submit payload, task+lease creation, resolve HMAC, callback raw_response truncation, and ImageHandle adaptor parsing. |
| `go test ./model ./service ./relay/channel/task/imagehandle ./relay ./controller` | passed | Broader backend package regression after lease refactor. |
| `go test ./...` | passed | Full backend regression after fixing fast callback/provider_task_id and submit-result update race. |
| `cd web && bun run build` | passed | Frontend build after AsyncTask wording update; only existing Vite warnings were emitted. |
| `go test ./relay ./relay/channel/task/imagehandle` | passed | Covers sync `result_data_format`, base64 response mapping, URL-only edit gating, and async base64 rejection. |
| `go test ./...` | passed | Full backend regression after sync URL/base64 compatibility changes. |
| `cd web && bun run build` | passed | Frontend build after current changes; only existing Browserslist/lottie/chunk-size warnings were emitted. |
| `docker compose -f docker-compose-dev.yml up -d --build new-api-dev` | passed | Built image `new-api-local:dev` and recreated `new-api-dev`. |
| Docker sync switch off | passed | Global off + channel inherit returned old direct upstream response and logged `execution_mode=direct_upstream`; no `/v1/image/tasks/sync` call. |
| Docker sync URL mode | passed | Global on + channel inherit called image-handle `/v1/image/tasks/sync`, returned `data[].url`, resolved lease, and logged `execution_mode=image_handle_sync`. |
| Docker sync base64 mode | passed | `response_format=b64_json` called image-handle sync with base64 result and returned only `data[].b64_json` after final rebuild. |
| Docker channel override | passed | `settings.image_handle_sync_mode=force_on` overrode global off; `force_off` overrode global on. |
| Docker edit URL input | partial | URL edit input reached image-handle sync and new-api handled failed terminal status with refund; local mock upstream returned 415 for multipart `/v1/images/edits`. |
| Docker edit non-URL input | passed | Multipart edit input fell back to direct upstream and did not call image-handle sync; local mock returned 415 and new-api refunded. |
| Docker async base64 rejection | passed | `/v1/image/tasks` with `metadata.result_data_format=base64` returned 400 before image-handle received a task. |
| Docker sync 202 timeout | not run | Local image-handle timeout is 300s; added unit coverage for HTTP 202 -> `image_handle_sync_timeout` instead of waiting in Docker. |
| `go test ./relay` | passed | Covers image-handle sync edit upload normalization for multipart/base64 inputs and final URL-only edit payloads. |
| `go test ./...` | passed | Full backend regression after image-handle edit upload support. |
| `cd web && bun run build` | passed | Frontend build after backend change; only existing Browserslist/lottie/chunk-size warnings. |
| `docker compose -f docker-compose-dev.yml up -d --build new-api-dev` | passed | Built image `new-api-local:dev` and recreated `new-api-dev`. |
| Docker switch-off multipart edit | passed | With sync disabled, request stayed on old direct-upstream path; local mock returned 415 and new-api refunded. |
| Docker sync multipart edit upload | partial | With sync enabled, new-api called image-handle `/v1/image/uploads` then `/v1/image/tasks/sync`; final worker call failed because local mock upstream does not support multipart `/v1/images/edits`. |
| Docker sync base64 edit upload | partial | With sync enabled, new-api called `/v1/image/uploads/base64` then `/v1/image/tasks/sync`; final worker call failed at the same local mock multipart limitation. |
| Docker sync URL edit | partial | URL input skipped upload and went directly to `/v1/image/tasks/sync`; test URL was intentionally not fetchable, so worker returned `fetch failed`. |
| Docker sync generation URL/base64 | passed | `/v1/images/generations` returned OpenAI-compatible `data[].url`; `response_format=b64_json` returned `data[].b64_json`. |

## Error Log
| Timestamp | Error | Attempt | Resolution |
| --- | --- | --- | --- |
| 2026-06-23 | `TestBuildRequestBodyMatchesImageHandleContract` failed after adding mandatory internal secret config | Targeted test run | Added test env vars and callback secret settings, then re-ran targeted tests. |
| 2026-06-23 | Invalid `client_task_id` test returned config error before validation error | Targeted test run | Reordered ImageHandle adaptor validation so request shape errors are returned before deployment config errors. |
| 2026-06-23 | Local token `qArd...` returned 401 | Docker dev test | Token row was soft-deleted; created a local test token `codexasyncimage20260623localtest0000abcdef123456`. |
| 2026-06-23 | Local token could not access `ikun_gpt-image-2` | Docker dev test | Added `ikun_gpt-image-2` to dev `UserUsableGroups`. |
| 2026-06-23 | First callback event stayed pending | Docker dev test | Callback URL was `localhost:3001`, which points to the image-handle container. Changed local callback address to `http://host.docker.internal:3001`. |
