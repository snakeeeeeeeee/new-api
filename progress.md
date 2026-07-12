# Usage Statistics Split Progress (2026-07-12)

## Phase 1: Backend contract and attribution
- **Status:** complete
- Loaded required planning, brainstorming, UI/UX, and browser workflows.
- Recovered prior completed planning context and preserved unrelated untracked files.
- Confirmed API compatibility, billing-source classification, lazy-loading, and responsive layout decisions.
- Added section/source query validation, additive split fields, subscription ranking, one-pass aggregation, and section-gated data loading.
- Added wallet/subscription/unknown summary, trend, model, filtering, and controller regression tests.
- Added task, Midjourney, and violation-fee billing metadata; task and Midjourney paths have direct regression tests.

## Phase 2: Aggregation and backend verification
- **Status:** complete
- Focused service/model/controller packages pass after source split, subscription ranking, and lazy sections.

## Phase 3: Frontend redesign
- **Status:** complete
- Replaced the oversized page with a request/state coordinator and separate filter, overview, ranking, funding, and detail modules.
- Added draft/applied filters, primary and secondary tabs, per-section caching, stale-response guards, and lazy funding queries.
- Added compact source-split metrics, an unknown-source warning, source charts, responsive tables, and detail sheets.
- Added 29 feature translations to all seven locale files through scoped edits.

## Phase 4: Verification and delivery
- **Status:** complete_with_visual_qa_blocked
- Full Go tests and the frontend production build pass.
- Targeted UsageStats ESLint and Prettier checks pass; the page adds no project-level i18n lint findings.
- Vite is running at `http://127.0.0.1:5173/`.
- Browser navigation to the protected page redirects to `/login`; responsive light/dark screenshot checks require an authenticated admin session.

## Test Results
| Test | Status | Notes |
| --- | --- | --- |
| Baseline inspection | passed | Current implementation and missing log-attribution paths identified. |
| `go test ./model ./controller ./service` | passed | Split aggregation and API compatibility changes compile and pass focused tests. |
| Special log attribution tests | passed | Task and Midjourney metadata include wallet/subscription source fields. |
| `go test ./...` | passed | All repository Go packages compile and pass. |
| UsageStats ESLint and Prettier | passed | New modules and locale changes are clean. |
| `bun run build` | passed | Existing dependency/chunk warnings only. |
| Authenticated responsive browser QA | blocked | Existing browser session redirects to `/login`. |

## Errors
| Error | Attempt | Resolution |
| --- | --- | --- |
| Initial task-plan patch made no content change | 1 | Replaced it with an explicit top insertion. |
| Waited on an exec session with the cell-wait tool | 1 | Switched to terminal-session polling; the test process was unaffected. |
| Special-log test used `Action` as a direct struct field | 1 | Initialize the embedded `TaskRelayInfo` fixture. |
| First patch for the fixture used stale spacing context | 1 | Re-read the formatted code and applied a narrow field replacement. |
| Task-log test panicked at promoted channel fields | 1 | Add the embedded `ChannelMeta` that production initializes before logging. |
| Frontend build failed on the `useIsMobile` import | 1 | Replaced the default import with the established named import. |
| `bun run i18n:lint` found 426 repository-wide issues | 1 | Fix the five findings introduced in UsageStats and treat the remaining baseline warnings as unrelated. |
| `bun run i18n:extract` produced large unrelated locale churn | 1 | Reversed only the generated locale changes and switched to scoped feature-key updates. |
| In-app browser has no administrator session | 1 | Kept authentication intact and documented the remaining screenshot QA requirement. |

---

# GPT Cache-Write Billing Progress

## 2026-07-11 Upstream rc.21 comparison
- Loaded the GitHub and planning-with-files skills.
- Confirmed the current branch is `main` tracking fork `origin/main` with no tracked modifications.
- Found the prior GPT cache-write billing plan, findings, and verification history already present in the workspace.
- Started a read-only comparison against upstream `v1.0.0-rc.21`; product code remains unchanged.
- Cloned the public upstream tag to `/tmp/new-api-upstream-rc21` and recorded commit `bde9b2f44887d34ec54799ae191d50f97914359e`.
- Enumerated the local cache-write implementation surface and its targeted tests.
- Located the local feature introduction commit `614d134cebba4eef4cb9fae2d411612f5252c5e7`.
- Initial upstream inspection found non-pointer cache-write DTO semantics and no preservation of the ratio configuration-presence flag.
- Compared DTO and quota implementation directly: upstream uses max-and-clamp normalization and subtract-then-clamp billing; local uses official-field precedence, validity checks, and explicit configuration gating.
- Confirmed upstream bills missing-ratio models with the fallback 1.25 ratio, while local requires explicit key presence; upstream also ships default 1.25 entries for the three GPT-5.6 variants.
- Deepened the temporary upstream clone. The attempted `gh release view` failed because `gh` is not installed; switched to the public REST API.
- Read the rc.21 release body and located upstream feature commit `48068ce9236e7bfcf923f8d20ca39fb8e611ef86`.
- Compared feature-commit scope: upstream is a focused 18-file/158-line patch; local is a broader 28-file/1,580-line implementation with audit/UI/test layers.
- Identified the main semantic conflict: upstream intentionally permits overlapping cached-read/write prefix counts and clamps only the ordinary-input remainder; local rejects writes exceeding `prompt-cached`.
- Reviewed conversion and UI paths: upstream propagates a unified integer token class into Claude and tiered billing; local preserves native-vs-legacy provenance and adds explicit log/UI audit states.
- Confirmed by test search that upstream does not cover explicit-zero, absent-field, unconfigured-ratio, negative-value, or audit-state cases.
- Ran local focused Go tests across DTO, price helper, OpenAI relay, conversion, and quota packages: all passed.
- Ran upstream rc.21 focused quota/conversion tests from the temporary clone: all passed.
- Ran local `bun test src/helpers/promptCacheUsage.test.js`: 11 passed, 0 failed.
- Completed the comparison. Only the three planning records were modified; unrelated untracked files remain untouched.

## 2026-07-11 GPT-5.6 overlap merge
- Loaded the brainstorming and planning-with-files skills and recovered the completed comparison context.
- Selected the hybrid design: preserve explicit-zero backend semantics, hide zero/missing values in visible logs, retain explicit configuration gating, and adopt upstream overlap-aware billing.
- Started Phase 1 discovery; no product code changed yet.
- Inspected current backend and frontend paths. Identified the exact obsolete bound and the two visible-log predicates that expose explicit zero.
- Confirmed quota math needs the upstream zero clamp and that GPT-5.6 default ratios can use the existing ratio map without changing configuration APIs.
- Completed Phase 1. Defined exact expected quotas and a centralized positive-only visible-log flag; implementation is starting.
- Implemented overlap-aware configured billing with a zero-clamped ordinary-input base and a total-input malformed-value bound.
- Added default 1.25 creation ratios for `gpt-5.6-sol`, `gpt-5.6-terra`, and `gpt-5.6-luna`.
- Added the positive-only frontend visibility flag and switched compact/expanded usage-log consumers to it.
- Updated backend/frontend unit fixtures, including the upstream 4,884-quota overlap case.
- Reviewed the implementation diff. The only discovery-command error was an unnecessary nonexistent root `package.json`; frontend scripts were still resolved from `web/package.json`.
- Focused Go tests passed across `service`, `dto`, `relay/helper`, `relay/channel/openai`, and `service/openaicompat`.
- Frontend prompt-cache normalization tests passed: 11 tests, 58 assertions, including visible-zero suppression.
- Inspected ratio initialization and Docker helpers. Existing persisted options may override source defaults; no dedicated cache-write simulation script is available, so Docker testing will use temporary isolated fixtures.
- Confirmed persisted ratio maps intentionally replace defaults. Existing installations remain operator-controlled; fresh defaults will include the three GPT-5.6 variants.
- Added and passed `TestDefaultCreateCacheRatioIncludesGPT56Models` for the three 1.25 defaults.
- Audited every frontend use of reported cache-write tokens; all visible render paths now require a positive value.
- Completed Phase 2 and started broad automated verification.
- `go test ./...` passed.
- `bun run build` passed with existing Browserslist, lottie `eval`, and chunk-size warnings.
- Full `bun run lint` failed on generated `dist` churn plus 111 pre-existing format warnings; this is outside the change scope, so verification is narrowed to touched frontend files.
- Targeted Prettier check passed for all four touched frontend files.
- Completed Phase 3 and started Docker dev verification; unrelated untracked files remain untouched.
- Docker baseline is healthy on port 3001 with PostgreSQL and Redis; current app container is 19 hours old and will be rebuilt.
- Rebuilt and recreated `new-api-dev` successfully from the changed source; `/api/status` returned `success=true` immediately after startup.
- Confirmed the Compose network and relevant PostgreSQL schemas for isolated channel, token, ability, option, and log verification.
- Selected user id 2/default group and type-1 OpenAI fixtures without exposing or modifying existing channel credentials.
- Started `codex-cache-write-mock` on the Compose network.
- Adjusted the fixture plan after discovering the live default group ratio is 999: tests will use a unique temporary group at ratio 1.
- Created temporary ratio/group/channel/ability/token fixtures transactionally and restarted the app.
- First Docker request batch: explicit-zero and missing-field requests succeeded; configured, unconfigured, and oversized requests returned 403 and require fixture diagnostics before retry.
- Diagnosed the 403 response as user id 2 having zero quota. Routing and ratio fixtures are correct; will snapshot/temporarily raise only this user's quota and restore it after log verification.
- Verified the two successful logs: zero and missing both cost 1,062; only explicit zero retained the raw reported/enabled snapshot.
- Chose a safer correction: reverse the two token-scoped charges, remove those rows, and use a disposable high-quota user for the complete rerun.
- Reversed the first two charges and removed their logs, then rebound the token to disposable user id 994183 with isolated quota.
- Final Docker batch passed all five scenarios with exact quotas: configured 4,884; unconfigured/zero/missing/oversized 1,062 each.
- Verified raw log metadata, the oversized warning, and disposable-user total consumption of 9,132 quota.
- Removed all temporary Docker/DB fixtures, stopped the mock, restored options, and restarted the app.
- Residue audit passed: all temporary row counts are zero, all option keys are absent, user id 2 is restored to 2,124/999,000, and `new-api-dev` is healthy.
- Final status endpoint and whitespace audit passed. Product changes remain limited to billing, ratio defaults/tests, and visible-log normalization/consumers/tests.
- Final product diff reviewed. Phase 4 and the full implementation are complete.

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
