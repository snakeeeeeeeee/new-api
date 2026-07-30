# Multi-provider Async Video Final Progress (2026-07-23)

- Completed the normalized `/v1/video/tasks` create/list/get/batch contract, durable request/idempotency storage, provider-neutral public DTO, structured multi-output Assets, and split Token/`ak_` authentication.
- Completed xAI generation/edit/extension conversion, provider namespace validation, explicit-zero preservation, upstream-model pass-through, and the official 1080p restriction to 1.5 single-image generation.
- Completed Asset content resolution for public CDN, relative authenticated channels, provider resolvers, data URLs, redirects, Range, expiry mapping, and legacy task-content aliases.
- Completed `video.task.succeeded` and `video.task.failed` creation through the existing durable Webhook event/outbox using the shared account `wk-`.
- Added the Async Videos Resource Center page, four normalized operations, query/download examples, credential semantics, temporary-resource behavior, Webhook events, OpenAPI 3.1 schemas, filters, and all seven locale catalogs.
- Focused Go tests, `go test ./... -count=1`, OpenAPI drift, scoped ESLint/Prettier, frontend production build, i18n status, and `git diff --check` pass. Full i18n lint remains at the repository's existing 420-item baseline with no changed-page finding.
- Rebuilt Docker dev as image `sha256:191f82bf460d...`; `new-api-dev` is healthy at port 3001.
- Desktop-only browser QA at 1280x720 confirmed the Async Videos tab, all documented operations, query/download guidance, credential semantics, and no document-level horizontal overflow. Mobile QA was intentionally skipped per user instruction.
- Real task `task_Z75TXf0icNrn3pQcckUrZCdfLkRSoKJc` completed on channel 109 with a 2-second MP4; normalized `ak_` query, video Asset lookup, Asset Range, and legacy task Range all passed with HTTP 206 and 1,024 bytes.
- The generated sub2api output is a relative authenticated `/v1/videos/{uuid}/content` path, so the public task correctly returns an Asset proxy with `url_auth=resource_api_key` rather than claiming a public xAI CDN URL.
- The real upstream rejected edit submissions using both a downloaded data URL and the generated provider content UUID. Both became durable failed public tasks and each emitted one delivered `video.task.failed`; the successful generation emitted one delivered `video.task.succeeded`.
- Replaying the failed edit with the same request and `Idempotency-Key` returned HTTP 202, `Idempotent-Replayed: true`, and the original failed task without another upstream call.
- Restored the original admin Webhook URL/status/version/ciphertext/update timestamp, reset mock metrics, retained the real task records as acceptance evidence, and confirmed Docker health and `/api/status`.

---

# Adobe2API Seedance 2.0 Fast Integration Planning Progress (2026-07-29)

- Loaded the required brainstorming and file-planning workflows; this turn remains an investigation and design pass only.
- Located the Adobe2API repository and confirmed two advertised Seedance 2.0 variants, including Fast.
- Confirmed from documentation that a 4-second Fast request is valid and that exact aliases encode duration, aspect ratio, and resolution.
- Started tracing the actual payload builder, upstream model version, task lifecycle, local Docker configuration, and new-api adaptor fit.
- Logged the missing planning recovery script and continued through direct planning-file recovery without changing business code.
- Confirmed Adobe2API's client-facing video call is blocking even though Adobe submit/poll is asynchronous; a direct proxy cannot provide correct new-api task semantics.
- Compared three integration choices and selected an Adobe2API async video contract plus a dedicated new-api AdobeVideo adaptor.
- Verified the local Docker network, live model catalog, active positive-credit account summary, historical successful Seedance calls, and a 4.042-second Fast MP4 without exposing secrets or making a paid request.
- Ran the current Seedance suite in a network-disabled ephemeral container with writable tmpfs data: all 13 tests pass, including 451 policy-error preservation.
- Completed the mock-first and one-paid-call acceptance design. No business code, channel rows, pricing Options, credentials, or upstream tasks were changed in this planning turn.
- The user approved the recommended implementation and one real 4-second paid call. Phase 4 Adobe2API implementation is now in progress; the paid call remains gated behind focused tests and Docker mock acceptance.


# Per-second Video Billing and Subscription Eligibility Progress (2026-07-29)

- Loaded the required brainstorming and planning-with-files workflows.
- Preserved existing workspace changes and historical planning records.
- Started read-only discovery of pricing, async-task settlement, duration, and subscription deduction paths.
- Located relay-level subscription billing metadata and confirmed async tasks persist their selected subscription identity.
- Found the centralized BillingSession/funding-source abstraction and existing duration-aware video billing estimation in Gemini/Vertex task adaptors.
- Confirmed task billing has estimate/submit/complete hooks and identified xAI's current hard-coded per-call branch as a direct conflict.
- Traced the exact task quota formula and confirmed `ModelPrice` lacks billing-unit metadata despite supporting multiplicative `seconds` ratios.
- Audited subscription-plan selection and both target adaptors; confirmed no plan-level model restriction and no current per-second settlement for xAI/Seedance.
- Confirmed both target adaptors can be extended through existing billing hooks and that no current setting represents per-second units or per-model subscription eligibility.
- Verified submit/terminal settlement ordering and identified the successful-task undercharge risk when completion requires a video quota supplement.
- Incorporated the user's exact-model resolution convention; removed resolution multipliers from the recommended formula.
- Completed the read-only design with exact file-level ownership, compatibility behavior, and focused verification scope.
- Received explicit implementation authorization and loaded the required implementation workflows.
- Recovered the existing planning state, confirmed the dirty worktree contains unrelated user files, and started the isolated implementation phase without reverting them.
- Confirmed the configuration/public-price/task-snapshot structure can mirror ImagePricing while keeping video duration and funding policy independent from provider fields.
- Chose a request-scoped billing-preference override as the enforcement mechanism for wallet-only video requests, including unbound legacy models.
- Added the strong estimator contract, dedicated pricing calculator, funding override, and initial relay integration; the first compile exposed and fixed one missing import and one snapshot placement mismatch.
- Completed the backend VideoPricing config, immutable snapshots, exact-model takeover, wallet-only default, provider duration adaptors, and terminal audit-only duration handling with focused tests.
- Resumed from the implementation handoff, reloaded the planning and UI workflows, and synchronized the active plan to Phase 4 frontend work.
- Selected the existing dense Semi Design ImagePricing layout as the admin UI pattern, with explicit profile/binding controls and responsive 375/768/1440 verification.
- Added VideoPricing frontend helpers for normalization, validation, profile lifecycle, exact/policy-only bindings, preview calculations, and immutable log/audit summaries.
- Extended shared billing-type resolution for `per_video_second`; all 17 focused image/video pricing helper tests pass.
- Added the responsive VideoPricing administrator editor with template create/copy/delete, USD-per-second preview, exact model bindings, policy-only bindings, subscription toggles, funding-policy labels, and the ratio-settings tab.
- Targeted Prettier/ESLint and `git diff --check` pass for the new editor and tab integration.
- Added `per_video_second` filters and consistent USD/second rendering across pricing table, card, model detail, legacy price editor, and upstream ratio-sync protection; policy-only bindings retain their legacy billing label.
- Added video snapshot and returned-duration audit rendering to usage logs, including funding policy and an explicit no-repricing mismatch note.
- Added 46 feature-scoped translations to zh-CN, zh-TW, en, fr, ru, ja, and vi without importing the repository's historical missing-key backlog.
- Locale JSON formatting, 17 focused pricing helper tests, i18n status, and whitespace checks pass.
- Fixed the two handoff test fixtures: relay preconsume now uses an isolated zero-balance SQLite user/token, and video audit logging uses the same public task ID ownership key as production.
- Focused backend verification now passes for `setting/ratio_setting`, `relay/helper`, `relay`, `controller`, and `service`.
- All 46 feature locale keys are present and non-empty in zh-CN, zh-TW, en, fr, ru, ja, and vi; `i18n:status` passes and full lint is back to the repository's existing 441 findings with no new VideoPricing finding.
- Final changed-file Prettier/ESLint, all 17 pricing helper tests, `git diff --check`, `go test ./... -count=1`, and `bun run build` pass.
- Confirmed the group-ratio input in the VideoPricing editor is preview-only and never persisted; real billing resolves the request's effective user/aggregate/model group ratio through `HandleGroupRatio`.
- Renamed the field to `预览分组倍率` in all seven frontend locales. Targeted formatting, ESLint, locale validation, 17 helper tests, focused Go packages, the production frontend build, and whitespace checks pass after the clarification.
- Rebuilt image `sha256:722d776575c6...`, recreated only `new-api-dev`, and confirmed the container is healthy while PostgreSQL and Redis remained running.
- Docker browser QA confirms the new label and all configured profile/binding/funding-policy states. Layout geometry at 1440, 768, and 375 pixels has no page-level horizontal overflow.
- Docker funding-source matrix passed with the live `default` ratio of 999: wallet-only charged 74,925,000 quota and never fell back to subscription; enabling the exact model binding charged the same amount from subscription while leaving wallet quota unchanged.
- Docker xAI mock verification passed for both compatibility `duration` and normalized `output.duration`; missing, zero, fractional, and provider-options override requests failed before upstream dispatch.
- Public pricing metadata passed for `per_video_second`, seconds, unit price, and subscription policy. Removed the contradictory legacy xAI `按次计费` log suffix for explicit video/image pricing and added a focused regression assertion.
- Removed all Docker QA fixtures by exact user/channel/plan IDs, deleted the temporary `VideoPricing` Option that was absent before testing, removed the matching Redis plan cache, and stopped the xAI mock server.
- Final `go test ./... -count=1`, frontend production build, targeted Prettier/ESLint, 17 pricing helper tests, and whitespace checks pass. Full i18n lint remains at the repository's pre-existing 441-item baseline with no VideoPricing finding.
- Rebuilt final Docker image `sha256:096ade9a4ce9...`, recreated only `new-api-dev`, and confirmed PostgreSQL/Redis stayed running, all QA residue counts are zero, the mock port is closed, and the application is healthy at port 3001.

---

# Async Image Public Error 524 Progress (2026-07-28)

- Recovered the existing implementation and design document from the current workspace.
- Added public quota-error masking for structured provider codes and a narrow legacy Chinese-message fallback.
- Added unit coverage for structured masking, legacy masking, non-quota regression, polling projection, and failed Webhook payload privacy.
- Formatted the Go changes and passed the focused service test set.
- Audited the generated Resource Center OpenAPI plus GPT-Image and Gemini async/Webhook/error pages; identified the stale failure example and incorrect terminal-retry idempotency guidance.
- Updated the OpenAPI source and both provider document sets with the `"524"` business-code contract, generic public message, privacy boundary, and correct new-key terminal retry behavior.
- Generated and drift-checked the Resource Center OpenAPI successfully; the full supertokendoc VitePress build also passes.
- Full `go test ./... -count=1` and the new-api production frontend build pass.
- Fixed the callback DTO gap so image-handle provider status/code/type/message/param/raw error fields survive task persistence; focused controller/service regressions pass.
- Rebuilt Docker dev image `sha256:dc7d0136b9b6...`, recreated only `new-api-dev`, and confirmed its live `/api/status` response succeeds.
- Created a uniquely scoped disposable Docker user, Resource API key, queued image task, and request record; the first callback identified a runtime secret-ID resolution mismatch before changing task state.
- Submitted the corrected real HMAC callback and verified the live public task endpoint plus durable `image.task.failed` event both return only `"524"` and the generic retryable message, while the stored task retains structured administrator diagnostics.
- Added a final outbound-delivery privacy boundary so pending legacy quota-failure Webhooks are sanitized at send time without mutating stored diagnostics; focused legacy and non-quota tests pass.
- Removed every disposable fixture row and verified the exact remaining count is zero.
- Final full Go tests, OpenAPI drift check, VitePress build, diff whitespace checks, and Docker health all pass.
- Rebuilt the final Docker source as image `sha256:4ffd3d3f8556...`; `new-api-dev` is running healthy, and a final source scan found no disposable callback secret, Resource key, balance, or Request ID outside tests.
- Preserved unrelated untracked diagnostics and historical planning records.

# Image-handle Trace Search and Task Table Progress (2026-07-23)

- Loaded the required brainstorming, file-planning, and UI/UX workflows.
- Inspected both repositories without modifying existing application code.
- Confirmed the traceability gap, existing image-handle identifier persistence, pagination-only admin API, missing request-ID index, table overflow cause, and timestamp-derived duration design.
- Locked an administrator-only unified exact search across new-api Request ID, new-api client task ID, and image-handle provider task ID.
- Started Phase 1 implementation planning while preserving all unrelated worktree artifacts.
- Inspected the image-handle admin route, PostgreSQL task-store projection, React task table, scripts, and CSS structure.
- Confirmed the admin task response already carries all required trace identifiers and timestamps; implementation can remain backward-compatible by adding optional query parameters and derived display fields.
- Added early new-api trace context capture, structured-response provider task capture, and correlated error-log fields with focused relay/controller coverage.
- Added the image-handle request-ID index, administrator-only exact trace filtering, and timestamp-derived `duration_ms`.
- Added the task-table trace search, Request ID and duration columns, fixed column tracks, ellipsis, and accessible full-value titles.
- `go test ./relay ./controller`, image-handle server/admin builds, 73 image-handle tests, and both repositories' `git diff --check` pass.
- Desktop and 390px mobile browser QA passed search/apply/clear behavior, long-error containment, responsive search controls, page-width bounds, and console-error checks.
- Stopped the isolated local UI fixture and preserved all unrelated worktree files.

---

# Task Log Public Video URL Follow-up Progress (2026-07-23)

- Reviewed the supplied task-log modal screenshot and traced its URL to `TaskModel2Dto` rather than the OpenAI video status converter.
- Selected DTO-boundary conversion so the dashboard gets the public `task_.../content` URL without losing the raw upstream URL needed by the backend proxy.
- Started focused backend implementation and regression coverage; Docker task-log acceptance remains pending.
- Confirmed no frontend component change is required: the existing task-log modal and action buttons already consume a public-compatible URL when the DTO supplies one.
- Added a shared relative proxy-path helper and changed successful video `TaskDto.result_url` serialization to use it for all registered video actions.
- Added regression coverage proving all video actions use the public task ID, non-video URLs remain unchanged, failure URLs remain hidden, and internal upstream storage is untouched.
- Focused `relay`, `taskcommon`, `controller`, and `router` tests pass; Docker rebuild and browser acceptance are starting.
- Full `go test ./... -count=1` passes.
- Docker dev rebuilt successfully as image `sha256:9b369e0812ba...`; browser/session acceptance is next.
- Browser acceptance reached the local login page because the controllable browser has no session. An isolated disposable administrator will be used and removed after validation.
- The first disposable registration was rejected by the username length validator before creating data; retrying with a shorter unique fixture.
- Registered local fixture user ID 994207 and promoted it to administrator. The task page loads the administrator layout but requires a task-log menu permission entry before data can be read.
- Granted only `async_task`; both existing xAI rows now load in the administrator task table.
- The first automated preview click did not create a video element, so the locator will be rebuilt from the refreshed DOM before evaluating media playback.
- A second click with the rebuilt unique enabled locator also produced no modal; switching to component/log inspection rather than repeating that interaction.
- The visible DOM node click also produced no modal and no application error. Final browser verification will navigate shared-session tabs directly to the task JSON and public video content endpoints.
- Direct JSON-tab navigation was blocked locally before reaching the server; switching to an authenticated same-origin read from the task page.
- The page sandbox also omits `fetch`; relying on the successfully rendered task rows, focused DTO tests, and a direct shared-session media navigation instead.
- Authenticated browser media acceptance passes: the public task route fully decodes the MP4 with no error (`readyState=4`, 5.04 seconds, 848x480).
- The first CLI session DTO check logged in but omitted `New-Api-User`, so middleware rejected it before serialization; retrying with the same header used by the frontend.
- The corrected authenticated task API check passes and its CLI session was logged out: the existing xAI row exposes only the public `task_.../content` path.
- The isolated browser could not invoke logout through API navigation or physical menu interaction; cleanup will close all test tabs and remove the fixture user and permission rows directly.
- Removed fixture user 994207 and its `async_task` permission; final user/permission/token/task/log residue counts are all zero and the isolated test tabs are closed.
- Task-log follow-up acceptance is complete: full Go tests pass, Docker is rebuilt, the real task DTO exposes the public path, and the browser decodes the authenticated MP4 successfully.

---

# xAI Video Provider Compatibility Progress (2026-07-23)

- Reproduced the local Docker failure with a real completed xAI video task and isolated the contract mismatch from upstream generation health.
- Removed xAI video model normalization and added canonical `grok-imagine-video-1.5` discovery support; `UpstreamModelName` is now sent unchanged.
- Changed xAI completed status conversion to publish the public `task_...` content URL.
- Added safe relative URL resolution, same-origin Bearer attachment, cross-origin redirect stripping, Range forwarding, and all-2xx video response handling.
- Extended the video proxy transfer timeout from 60 seconds to 10 minutes after Docker acceptance reached the correct upstream content but a larger MP4 did not finish within the old window.
- Added focused model pass-through, mapping, public URL, relative URL, CDN auth, unsafe URL, and redirect-secret tests.
- Focused tests pass for `relay/channel/task/xai`, `controller`, and `router`; full regression and Docker acceptance remain in progress.
- Final `go test ./... -count=1` and `git diff --check` pass.
- Rebuilt Docker dev image `sha256:0ec505dac8e0...`; `new-api-dev` is healthy and `/api/status` returns 200.
- Real canonical task `task_7gnfNoVeZOj4YrPqZ8yi3Sp2SU2gS7SS` completed with a public `task_...` metadata URL. Reusing completed task `task_dwECb8BLNtzhUNm8taOUgknsekhWgmk5`, the public content route returned HTTP 200, `video/mp4`, and a valid 1,591,016-byte MP4.

---

# Async Image Final Usage Log Reconciliation Progress (2026-07-22)

- Loaded the required brainstorming and file-planning workflows; the user approved original-row reconciliation after diagnosis.
- Confirmed the current two-row behavior, missing Request ID source, and consume-only aggregate distortion from the screenshot and code paths.
- Locked scope to async image user-facing logs while retaining existing wallet/subscription/token delta settlement and generic task behavior.
- Mapped terminal ordering and selected a persisted final-log snapshot so submit-side compensation updates logs without rerunning wallet billing.
- Implemented Request ID persistence, terminal consume-log snapshots, row-locked submit persistence, and guarded finalization of the original consume row.
- Updated async image success/failure settlement so balances still reconcile by delta while the original log stores final quota, real tokens, duration, content, and settlement audit metadata; generic task refund logging remains unchanged.
- Added regression coverage for refund, supplement, exact charge, failure, Request ID fallback, early callback ordering, stale callbacks, fencing, actual-quota fallback, usage aggregation, and the fast-callback double-refund case.
- `go test ./... -count=1` and `git diff --check` pass.
- Rebuilt ordinary Docker dev and the opt-in `async-test` profile. Image `sha256:214db9b26c8...`; new-api, PostgreSQL, Redis, and the mock are healthy.
- Real PostgreSQL/mock E2E passed with one task, one consume row, no refund row, final quota `4913`, real token usage `5/196`, preserved Request ID, and request count `1`.
- Removed all disposable users, tokens, channels, tasks, and logs; restored the image-handle base URL and reset mock metrics to zero.
- Final diff review fixed the zero-precharge timeout edge case and added coverage proving that it updates the original failure log without changing wallet or token balances.
- Rebuilt final Docker dev image `sha256:0f9583d0b63b...` after the edge-case fix; new-api, PostgreSQL, Redis, and the async mock all report healthy.

---

# OpenAI Null Required Tool Schema Compatibility Progress (2026-07-22)

- Confirmed the exact cleanup boundary, default-disabled behavior, nested JSON Schema locations, desktop-only UI scope, and required Docker real-request A/B.
- Restored the file-based planning state and mapped settings, option validation, serialized relay, raw passthrough relay, tests, and Compatibility Management integration points.
- Added the independent hot global switch with default disabled and controller/model option coverage.
- Implemented a bounded schema walker for modern `tools` and legacy `functions`; it recursively removes only schema-keyword `required: null` from recognized child-schema positions.
- Integrated the cleaner into both serialized and raw passthrough OpenAI Chat Completions branches without changing unrelated request fields.
- Added the switch to Compatibility Management -> OpenAI Compatibility and translated it across all seven locale catalogs.
- Added focused cleaner and relay integration coverage. Affected-package tests, `go test ./... -count=1`, frontend production build, scoped ESLint/Prettier, `bun run i18n:status`, and `git diff --check` pass.
- Rebuilt Docker dev and completed desktop-only UI validation: the switch starts disabled, toggles on, and restores off. No mobile compatibility test was run per the user's instruction.
- Completed the identical-payload real A/B through `test-gpt兼容`, token ID 141, channel 85, and model `gpt-5.4`. Disabled Request ID `20260722094557387149380GU70SYWZ` returned the target HTTP 400; enabled Request ID `20260722094558315168755H4btF3TY` returned HTTP 200 with the forced `knowledge_list_documents` tool call.
- Restored all temporary state: the new option row is absent so the feature remains default-disabled, upstream-error passthrough is false, the root access token is null, disposable UI user count is zero, and Docker dev remains healthy at port 3001.

---

# OpenAI Reserved Python Tool Compatibility Progress (2026-07-22)

- Started Phase 6 to reproduce the production reserved-name 400 and, if possible, run an identical-payload disabled/enabled A/B. The supplied production Request ID is absent from all local logs and artifacts, so protocol-shape variants will be tested instead of claiming a byte-for-byte replay.
- Phase 6 Chat Completions reproduction matrix completed five real remote attempts with compatibility disabled; every variant returned 200 and `finish_reason=tool_calls` with function name `python`. Exact cleanup restored absent option rows, null admin access token, and healthy Docker dev.
- Completed one final direct `/v1/responses` probe through the same token and remote channel; it also returned 200/completed with tool name `python`, confirming the current remote OpenAI context accepts the name even without the Chat Completions compatibility layer.
- Phase 6 ended without reproducing the production 400. Six authoritative Request IDs and all attempted shapes are recorded; further repetition was stopped because the remaining variable is remote sub2api account selection or unavailable production payload/context.
- Final restoration check passes: option-row count 0, admin access token null, Docker running/healthy, `/api/status` 200, and `git diff --check` clean.
- Started the user-requested Docker live contrast matrix. Acceptance requires 400 with the switch disabled, 400 with an enabled but nonmatching list, and 200 plus response-name restoration with an enabled matching `python` list.
- Snapshotted the exact starting state: both new option rows are absent (code defaults apply), admin ID 1 has no access token, token `test-gpt兼容` uniquely maps to ID 141/group `gpt-new`, and `new-api-dev` is healthy on port 3001.
- The test harness will use a process-local temporary root access token to exercise the real option API, keep the paid API token secret out of output/files, then restore the absent option rows and null admin access token before restarting to the original defaults.
- Completed four real upstream contrast requests through `test-gpt兼容` -> `gpt-new` -> `sub2api-gpt` -> channel 85 -> OpenAI. Disabled and nonmatching configurations exposed `python` upstream; matching configuration exposed `run_python` upstream in both non-streaming and streaming calls while returning structured name `python` to the client.
- Authoritative Request IDs are `20260722063844971541042YsFcz03T`, `20260722063848780488585coz2N5ZT`, `20260722063850406377461JHAgdMx4`, and `20260722063852209810962TUfapqgC`; all returned 200 and the streaming call emitted 13 SSE events with a usage chunk.
- The screenshot's upstream 400 did not reproduce when the switch was disabled, so acceptance used model-observed schema names rather than incorrectly treating that external behavior as deterministic.
- Restored the exact pre-test state: no persisted option rows, no admin access token, and healthy Docker dev on port 3001. No mobile test was performed.
- Added focused request/response coverage for every known Chat Completions name path, disabled and empty configuration, final-format scope, collision suffixes, 64-character aliases, and content/arguments preservation.
- Added serialized and raw passthrough `TextHelper` integration coverage; both branches forward `python` as `run_python` without changing message content.
- Added the compact OpenAI Compatibility switch and disabled-state multiline input with matching frontend validation and translations across all seven locales; mobile testing is explicitly excluded at the user's request.
- Desktop frontend ESLint, scoped Prettier, seven-locale i18n status, controller save/normalization coverage, and `git diff --check` pass.
- Full `go test ./... -count=1` and the frontend production build pass. Full i18n lint remains at the existing 420-item baseline with no Compatibility-page finding.
- Added hot global enable/list settings with bounded OpenAI-name validation and normalized comma/newline parsing.
- Added collision-safe request-scoped aliases plus structured request rewriting and response restoration, integrated into serialized, passthrough, streaming, and non-streaming OpenAI Chat Completions paths.
- Initial compilation and existing focused tests pass for `setting/model_setting`, `relay/common`, `relay/channel/openai`, `relay`, and `controller`.
- Resumed the approved configurable design, loaded the required brainstorming and file-planning workflows, and confirmed the implementation diff is still empty.
- Reconfirmed the active paths: hot `global` settings, both serialized and passthrough branches in `TextHelper`, request-scoped relay state, and OpenAI stream/non-stream response handlers.
- Confirmed sub2api is an immutable bridge and moved the compatibility boundary to new-api.
- Replaced the initial GPT-5.4-only scope with model-independent activation for the exact custom function name `python`; this covers other and future OpenAI models without claiming they all reserve the name.
- Locked request-scoped bidirectional structured rewriting across normal relay, raw passthrough, streaming response, and non-streaming response paths.
- Expanded the contract to a hot global switch plus administrator-configurable reserved-name list in the existing OpenAI Compatibility tab; default enabled list is `python`.

---

# Async Image Token Usage Log Backfill Progress (2026-07-22)

- Loaded the required brainstorming and file-planning workflows.
- Confirmed the approved behavior: real upstream usage should appear in the original consume log while image-parameter charges remain unchanged.
- Mapped the direct `consume_log_id` association and existing CAS-protected audit merge; implementation will reuse the same update rather than add a second write.
- Extended the guarded log merge with optional token columns and wired image execution audit usage into the original consume log without touching quota settlement.
- Added focused coverage for direct guarded updates, callback DTO usage, persisted task-result usage, missing usage, and mismatched task association.
- Focused model and service tests pass; diff whitespace validation is clean.
- Added a three-attempt CAS conflict retry so concurrent terminal audit merges preserve the latest metadata without changing normal-path query counts.
- Added direct mapping coverage for canonical input/output names, prompt/completion aliases, explicit zero values, and total-only usage.
- Full `go test ./... -count=1` passes, including model, service, controller, relay, router, settings, and async mock packages.
- Final `git diff --check` is clean; unrelated untracked workspace content remains untouched.

---

# Credential Separation Progress (2026-07-22)

- Final verification passes `go test ./...`, production frontend build, OpenAPI drift, changed-file Prettier/ESLint, i18n status, and `git diff --check`; the previously documented repository-wide `i18n:lint` baseline remains unrelated to these changed files.
- Rebuilt ordinary Docker dev and the opt-in `async-test` profile from final source. Image `sha256:8e8c4404ce5c...`, `new-api-dev`, PostgreSQL, Redis, and the mock are healthy; both application and mock health endpoints succeed.
- Tightened Webhook Key validation to the exact 51-character canonical shape and added positive/negative regression assertions; focused service, middleware, and router tests pass.
- Removed the disposable UI account and endpoint in one precise transaction; final PostgreSQL counts for users/endpoints/events/deliveries/attempts/asset keys/tokens/tasks are all zero, Redis has no matching cache key, and all Docker services remain healthy.
- Mobile Resource API Key and documentation checks pass: no page-level overflow, and wide credential/flow tables scroll only inside their constrained containers.
- Exact `375x812` mobile geometry passes through CDP device emulation: zero document overflow, full-width main content, wrapped long key text, and all visible controls contained within the viewport.
- Webhook regeneration confirmation passes: the warning is explicit, the replacement key has the canonical prefix/length, and the encrypted database value changed without storing plaintext.
- Webhook key interaction QA passes through real Docker APIs: generate returns a `wk-` key, hide switches to a 16-character mask and a show action, and copy reports success while masked.
- Temporarily restored the already-cleaned local user row `994203` without creating credentials or unrelated data; the live Webhook tab now loads the expected unconfigured state and will be cleaned back to absence after interaction checks.
- Diagnosed the initial Webhook UI-generation timeout as a stale browser session for already-cleaned user `994203`; no endpoint row or key was created, and browser QA will continue with a disposable live local account.
- Desktop UI checks pass for the Resource API Key scope copy and the documentation's three-credential overview table, including exact `Bearer sk-...`, `Bearer ak_...`, and `Bearer wk-...` examples.
- Rebuilt Docker dev and the `async-test` profile are healthy; backend E2E has passed `sk-` create, `ak_` query, encrypted-at-rest `wk-` delivery, key regeneration, restart persistence, and exact fixture cleanup.
- Started final browser QA against `http://localhost:3001/console/assets`; the existing ordinary-user session loads all four Resource Center tabs without requiring a new test account.
- Confirmed the final contract with the user: `sk_` submits asynchronous images, `ak_` reads Resource Center tasks/assets, and independent `wk-` authenticates outbound Webhooks.
- Loaded the required brainstorming, file-planning, and UI/UX workflows; retained the existing Semi Design visual language and rejected irrelevant marketing-page recommendations.
- Recovered a clean tracked `main` baseline at pushed commit `5bf1e37d3`; unrelated untracked `2dev/`, `outputs/`, and `tmp/` content remains out of scope.
- Started Phase 1 discovery across current authentication, Webhook delivery, encrypted endpoint storage, historical credential behavior, UI, and OpenAPI contracts.
- Compared the current and pre-unification Webhook tabs; selected the established reveal/copy/regenerate interaction as the lowest-risk UI restoration, with corrected `wk-` naming and no Resource Key dependency.
- Recovered the historical encrypted-key backend and corrected the design notation from `wk_` to the user-approved canonical `wk-` prefix.
- Mapped current service and test deltas: restore encrypted credential helpers around the concurrent delivery worker, remove active Resource Key checks, and rewrite only authentication-specific tests.
- Audited Resource Center docs/OpenAPI and route tests; credential names and security schemes will be split rather than leaving one ambiguous `$API_KEY` example.
- Located existing locale coverage and the sole Webhook-to-API-Key parent prop; UI changes can remain inside the Webhook tab, its hook, the parent call site, and scoped locale keys.

---

# Async Worker Operations Progress (2026-07-21)

- Final audit passes: targeted AsyncTask Prettier/ESLint, `git diff --check`, implementation-scope secret review, fixture cleanup, and temporary-database cleanup. Unrelated `2dev/`, `outputs/`, and `tmp/` content remains untouched.
- The generic file-planning completion script reports old pending image-pricing phases elsewhere in the shared planning file; all four phases of the current Async Worker Operations plan are independently checked complete.
- Cross-database schema/query validation passed against in-memory SQLite plus real MySQL 8.0.33 and PostgreSQL 18.1 containers. Both explicitly named disposable databases were dropped afterward and verified absent; minimum-version compatibility remains based on portable GORM/Unix-time paths rather than live 5.7/9.6 containers.
- Rebuilt Docker dev and the `async-test` profile from the latest source; `new-api-dev` is healthy on image `sha256:975b7acfe7ca...`, and the mock remains healthy on port 18081.
- Browser QA resumed against the rebuilt image. The pre-existing browser session belongs to non-admin user `codexwhux0717` and correctly redirects `/console/async-task` to `/forbidden`; administrator login is the next step.
- Chrome automation is unavailable in this environment, and the in-app backend does not trigger the existing hover-only account dropdown even with exact DOM-derived coordinates; QA is continuing through the application's direct login route.
- Direct `/login` navigation successfully cleared the stale session and authenticated the disposable root user. The admin async-task route is now available for UI acceptance.
- The in-app browser exposes a `1139x1204` page viewport but no high-level viewport-resize capability; current-width validation comes first, followed by a check for supported device-emulation controls.
- Overview UI passes at the available `1139x1204` viewport with zero document-level horizontal overflow. KPI density, both worker/queue panels, refresh controls, and the legacy platform/action/channel summaries render without overlap.
- Async Tasks tab passes with the 25-row acceptance fixture split over three pages at 20/page. Exact task-ID filtering returns only the requested row and collapses pagination correctly; dispatch status, attempts, HTTP status, timestamps, and error summary are visible.
- Webhook Deliveries tab exposes all planned filters including the time range, paginates the 53-row fixture, and renders failed/discarded retry actions. The detail SideSheet shows the capped payload, safe endpoint, current error, and the full two-attempt timeline without layout overlap.
- Retry confirmation passes end to end. Browser QA found and fixed stale open-detail state after a fast worker transition; on rebuilt image `sha256:23ab6e3648d1...`, the next five-second active-tab refresh updates both the row and open SideSheet to the same discarded status/error while retaining historical attempts.
- Settings tab contains the five new worker limits/timeouts alongside existing task, retry, image-handle, and timeout-override controls. Its auto-refresh selector is disabled, and an unsaved concurrency edit remained unchanged beyond a full poll interval.
- Local CDP emulation enabled exact responsive QA despite the browser's missing high-level viewport controls. The Settings tab passes at `1440x1000` in true dark media mode with the expected dark body background and zero document-level horizontal overflow.
- The Overview tab passes at exact `768x1024` light mode: body/root are 768px, the 180px sidebar plus 588px main span the viewport exactly, and document overflow remains zero.
- The Webhook tab passes at exact `375x812`: advanced filters are collapsed by default and reveal user/event/HTTP/time-range controls on demand; the 2,456px table scrolls inside a 347px body while document overflow stays zero; the SideSheet occupies exactly `x=0..375` and the full 812px height.
- The Async Tasks tab now follows the same mobile pattern after browser QA found the initial gap: task ID/status remain visible, user/dispatch/platform/action reveal through More Filters, and its 1,430px table remains inside a 347px horizontal scroller with zero document overflow.
- The `375x812` dark SideSheet also passes with distinct dark body/dialog backgrounds and zero dialog/document overflow.
- Final Docker image `sha256:4c9d29289809...` is healthy. All async E2E fixtures, the disposable root user, temporary root token, and seven dynamic option rows were removed; endpoint `id=7` remains disabled at its original URL, and mock counters/config are reset to zero-delay defaults.
- Added scoped translations for every new operations-page label in all seven locales, including locale-specific plural forms; `i18n:status` passes.
- Added the opt-in `async-test` Compose mock with success/failure/delay Webhooks, image-handle-compatible submission, hot controls, resettable concurrency metrics, a container healthcheck, and focused tests.
- Mock unit tests, Compose profile rendering, and targeted AsyncTask Prettier/ESLint checks pass.
- Full `go test ./... -count=1` and the production frontend build pass; full i18n lint is back to the repository's 420-item baseline with no AsyncTask findings.
- After the final production build, full `bun run lint` still fails only on the repository-wide Prettier baseline: 113 files including generated hashed `dist` assets; every changed AsyncTask source file passes targeted formatting.
- `bun run i18n:lint` remains at the repository-wide 420-item baseline with no `src/pages/AsyncTask` findings, while `bun run i18n:status` passes for all seven locales.
- Loaded the approved implementation plan and the required brainstorming, file-planning, and UI/UX workflows.
- Preserved unrelated untracked scripts, outputs, and historical planning sections.
- Reconfirmed the two serial worker bottlenecks, Webhook stale-lease gap, unbounded image request timeout, and per-request Webhook transport allocation.
- Locked implementation order: worker/runtime, admin API, operations UI, Docker mock profile, then full verification.
- Applied UI guidance: retain the existing Semi Design language, favor compact metrics and tables, make controls accessible, and verify 375/768/1440 widths in light and dark themes.
- Added normalized worker concurrency/timeouts with an atomic runtime snapshot, capacity-aware schedulers, request-level image timeout, shared validated Webhook transport, telemetry, and stale Webhook lease recovery.
- The first focused test run found only the intentionally obsolete no-reclaim assertion; it is now a reclaim-and-fencing regression test.
- A legacy task-only stats fixture exposed that the repository logger requires a non-nil context; the monitoring fallback now logs safely and preserves zero-valued queue sections when optional test tables are absent.
- Added compatible nested queue/worker stats plus paginated admin task and Webhook delivery list, detail, and CAS retry routes.
- Added safe public DTOs that omit dispatch request bodies, lock tokens, credentials, and authorization material; detail response text is capped at 4 KiB.
- Added worker capacity, endpoint limit, timeout, transport reuse, cross-database query, admin API, retry, and lease-fencing coverage.
- Full affected backend package tests pass: `setting/async_task_setting`, `model`, `service`, `controller`, and `router`.
- Added the four-tab operations UI with active-tab polling, queue/worker overview, task filters, Webhook detail/retry workflow, responsive tables, and the complete existing image-handle settings surface.
- The first frontend formatter invocation used repository-relative paths from inside `web/`; it matched nothing and will be rerun with web-relative paths.

---

# Image-handle Channel Override and Signed URL Progress (2026-07-15)

- Traced the sync generation flow from model mapping through credential lease, image-handle execution, URL extraction, and client response serialization.
- Confirmed omitted `response_format` allows Base64 upstream output and R2 fallback, while explicit `url` produces direct signed-URL passthrough.
- Confirmed the selected channel's parameter override is available in new-api but bypassed by the early image-handle sync branch.
- Confirmed image-handle remains provider-agnostic; new-api will apply the selected Adobe channel's override before task submission.
- Planned modifications: `common/json.go`, `relay/image_handle_sync.go`, and focused tests.
- Initial combined planning-file patch failed because the findings heading differed from the template; no partial write occurred, and the insertions were retried against actual file headers.
- Added `common.MarshalNoEscapeHTML` and limited its production use to the image-handle sync client response.
- Added selected-channel parameter override application before sync task/lease construction, then restored image-pricing-owned parameters from the immutable pricing snapshot.
- Added focused coverage for signed URL raw output, channel response-format override, unknown-field preservation, and pricing parameter protection.
- Focused relay tests passed; `gofmt` and `git diff --check` passed.
- Added payload-level generation/edit coverage for an aggregate-group public alias mapped to upstream `gpt-image-2`.
- Added independent `MarshalNoEscapeHTML` coverage and a nil channel-metadata compatibility guard.
- Focused `common`, `relay`, `relay/common`, and `relay/helper` tests pass.
- Full `go test ./... -count=1` passes.
- Configured both local Adobe channels with request parameter override `response_format=url` and rebuilt `new-api-local:dev` as image `sha256:45f7cb878333...`.
- Recreated `new-api-dev`; the count alias succeeded without a client `response_format`, returned a directly accessible Adobe signed URL, skipped R2, and emitted literal `&` separators.
- Confirmed the successful count request retained image-parameter billing and the mapped upstream model in the consume log.
- Token alias contract verification passed through task persistence and debug logs, but two image-handle executions disconnected before an upstream HTTP response.
- A host-direct token-upstream diagnostic also failed at the transport layer (`HTTP2 framing layer`, status 000); stopped after the third failure instead of issuing more paid requests.
- Restored `image_handle_setting.debug_upstream=false`, retained both Adobe channel overrides, and preserved all generated request/response/container logs under `tmp/`.

---

# Aggregate Group Categories Progress (2026-07-17)

- Resumed after backend completion, reloaded the task plan, and started the aggregate-group admin UI phase.
- Applied the UI/UX review: preserve current Semi Design styling, use responsive card selection, accessible controls, confirmations, and disabled/loading states.
- Implemented the category manager, category filter/column, desktop and mobile selection, batch assignment bar, and aggregate-group category field.
- Implemented token option sections with ordered custom categories, the Other fallback, hidden `auto`, and one-way historical-value preservation.
- Targeted frontend ESLint and whitespace checks pass; focused aggregate category model/controller tests still pass after frontend integration.
- The first group-option unit test attempt exposed `api.js` browser-side imports in Bun; extracting the new logic into a pure helper module before rerunning.
- The repository-wide i18n extractor produced broad unrelated locale churn; reverting only that generated diff and switching to targeted locale updates.
- Added 30 scoped interface translations across all seven frontend locales; locale JSON, frontend build, 24 Bun tests, and related Go package tests pass.
- Full i18n lint still reports the repository's existing 421 hardcoded-string baseline; it reports no new category or token component findings.
- Full `go test ./... -count=1` passed. Production frontend build passed.
- Full frontend formatting/header checks retain repository baselines (116 Prettier files and 68 header errors, including generated `dist`); targeted changed-file checks pass.
- **Status:** Docker dev integration and responsive UI verification starting.
- First Docker Compose rebuild attempt stalled on three remote pinned-image metadata lookups for over four minutes; the old healthy container remained untouched and the stalled build was stopped.
- **Status:** backend implementation starting.
- Loaded and applied the approved product plan.
- Inspected aggregate-group persistence, APIs, admin UI, token editor, group-option helpers, and responsive CardTable behavior.
- Confirmed the category-management entry will be a side sheet on the existing aggregate-group page.
- Confirmed custom categories, a virtual Other fallback, grouped token options, and no new-token auto option.
- Added category persistence, migrations, CRUD/order/delete/assign APIs, aggregate-group category assignments, and category metadata in admin/user responses.
- Added focused model/controller coverage; the backend category lifecycle and metadata tests pass.


# Image Parameter Pricing Progress (2026-07-14)

- Resumed the approved implementation after context recovery; no paid/local generation request has been sent yet.
- Existing diff contains backend configuration, billing snapshots, sync/async image relay handling, marketplace/log presentation, frontend settings, and focused tests.
- Started three independent workstreams: backend pricing review, frontend audit/verification, and image-handle/live-contract audit.
- Local runtime audit found count/token token records but missing channel abilities, mappings, group ratios, and `ImagePricing`; these will be corrected only after code review and Docker rebuild.
- Focused `go test ./relay -count=1` passed in the preceding implementation session.
- Recovered and reviewed the active plan plus core configuration/resolver/snapshot types.
- Confirmed the running Docker stack is healthy but not yet rebuilt from the final implementation.
- Confirmed image-handle has a scoped dirty diff for parameter forwarding/audit tests; no unrelated files were reverted.
- Audited local container availability: new-api and the complete image-handle execution stack are running and reachable by their published ports.
- Initial whitespace checks pass in both repositories.
- Focused new-api tests passed: `setting/ratio_setting`, `relay/helper`, image-handle adaptor, `relay`, `service`, `controller`, and `model`.
- image-handle `npm test` passed all 61 tests after its TypeScript build, including leased execution, sync/async paths, generation/edit forwarding, and callback contracts.
- Audited runtime aliases/mappings/profile without reading or printing secrets.
- Verified existing live sync count and token executions succeeded through image-handle with the expected distinct billing modes.
- Verified async count snapshot/mapping/lease correctness and terminal failure refund behavior; failure source is an external `fetch failed` inside image-handle execution.
- Monitoring the existing async token request to terminal state; no duplicate request has been sent.
- Existing async token request completed successfully with exact usage settlement, one stored asset, and correct precharge-difference refund.
- Verified `/api/pricing` and authenticated polling contracts for both aliases without exposing token values.
- Asked backend review to classify or fix cumulative used-quota counters that retain async precharge amounts after refunds.
- Compared async task accounting with the standard synchronous billing session and narrowed a possible counter fix to image-handle terminal refund/negative-delta paths only.
- Frontend independent review is complete: helper unit tests, targeted lint/format, production build, and diff checks pass; full i18n retains only the known repository baseline.
- Full backend regression `go test -count=1 ./...` passed across every package.
- Reviewed the complete image-handle source/test diff and confirmed it stays within resolution passthrough and audit-contract scope.
- Fixed synchronous and asynchronous image-handle `response_format` passthrough, including JSON, multipart, metadata precedence, mapped async persistence, and force/default result-policy separation.
- `go test -count=1 ./relay/common ./relay/channel/task/imagehandle ./relay` and `git diff --check` pass after the contract fix.
- Converted async top-level `quality` and `resolution` to optional pointers, updated the pricing resolver/writeback contract, and trimmed synchronous `response_format` before forwarding; focused tests now include `relay/helper` and pass.

---

# Multi-level Token Tier Pricing Progress (2026-07-13)

## Phase 1: Configuration and billing core
- **Status:** complete
- Recovered the approved implementation plan and confirmed no tracked implementation changes were left by the prior session.
- Loaded the required planning, OpenAI documentation, brainstorming, and UI/UX workflows.
- Preserved unrelated untracked scripts and output artifacts.
- Added generic rule types, GPT-5.6 built-ins, strict validation, exact-name overrides, disabled overrides, hashes, and atomic runtime snapshots.
- Added GPT-5.6 base input/output/cache-read/cache-write defaults including the unsuffixed Sol alias.
- Added estimated precharge tier selection and Decimal final settlement with structured and readable audit details.
- Added `/api/option` metadata and optional `/api/pricing` tier payloads.
- Added the admin editor and marketplace tier presentation with responsive layouts and inline validation.
- Completed desktop and mobile visual QA for the admin tier editor, marketplace card/table, and pricing detail sidebar; corrected missing cache component translations in all locales.
- Added the secure Docker validator and completed all seven disabled, official short, synthetic three-tier, real long-context, and streaming scenarios.
- Rebuilt the final Docker image `e1c0d1bdf24c...`; `new-api-dev` and `sub2api-dev` are healthy at completion.
- Verified the validator restored configuration: no `TokenTierPricingRules` option row or temporary visual user remains, the original usable groups remain, and the root access token is null.
- Fixed the disabled-rule marketplace regression by reconciling cached pricing rows against the current effective rule during `/api/pricing` response cloning.
- Added regression coverage for stale enabled cache data, disabled rules, restored system defaults, and fixed-price models.
- Rebuilt Docker image `0cc94fe3acd7...`; API and browser checks confirm disabled Luna cards omit both the tier badge and base-price suffix, while restoring the default immediately restores both labels.

## Verification
| Check | Status | Notes |
| --- | --- | --- |
| Worktree baseline | passed | No tracked changes; unrelated untracked files preserved. |
| Official pricing behavior | confirmed | Whole-request switch above 272K total input tokens. |
| Focused backend tests | passed | `ratio_setting`, `relay/helper`, `service`, `controller`, and `model`. |
| Initial frontend build | passed | Existing bundle-size and browserslist warnings only. |
| Full Go suite | passed | `go test ./...` completed successfully. |
| Final frontend/Docker build | passed | Final source built into image `e1c0d1bdf24c...`; warnings are unchanged dependency/chunk-size warnings. |
| Docker real-upstream validation | passed | Seven scenarios passed; report `tmp/token-tier-pricing-report-1783875609.json` independently matches every log and quota delta. |
| Final residue and whitespace audit | passed | Configuration restored, temporary credentials removed, all containers ready, and `git diff --check` clean. |
| Disabled marketplace visibility | passed | Immediate disable/restore verified through the management API, public pricing API, and rendered Luna marketplace card. |

---

# Usage Statistics Split Progress (2026-07-12)

## Phase 5: Docker table layout audit
- **Status:** in_progress
- User reported that some UsageStats tables do not visually fill their available width.
- Current plan is to rebuild Docker dev, inspect all ranking/funding tables with real data, and fix column sizing based on rendered evidence.
- Built Docker image `1aa4938c...`, recreated `new-api-dev`, and verified `/api/status` succeeds.
- Source review identified unconstrained columns plus desktop `max-content` scrolling in both main table components.
- Initial browser verification is authentication-blocked; checking another available local browser session before requesting user action.

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

# 2026-07-12 Model marketplace dynamic-route labels

- Located the label and ratio suffix in card, table, shared price formatting, and pricing-detail modal paths.
- Confirmed backend model-specific aggregate pricing exposes both configured base ratio and maximum reachable child-route ratio.
- Removed the user-facing maximum-price/maximum-ratio wording while preserving the pricing calculation.
- Preserved the detail modal's dynamic-route numeric precedence so user-specific base overrides cannot replace the correct model-route value.
- Targeted Prettier and ESLint checks passed; the production Vite build passed with existing dependency/chunk warnings.
- Browser verification on `http://localhost:3000/pricing` found `claude-fable-5` pricing and confirmed both removed labels are absent from the page.

---

# Progress

## 2026-07-18 multipart async edit and Webhook retry completion
- Added multipart local-file edit support to `POST /v1/image/tasks` while retaining the normalized JSON URL contract.
- Added administrator-configurable Webhook total attempts and fixed retry interval, defaulting to 3 attempts and 30 seconds; any 2xx succeeds and the response body is ignored.
- Full Go tests, frontend production build, focused ESLint, OpenAPI generation check, and whitespace audit passed.
- Rebuilt `new-api-dev` from the final source and completed multipart, task polling, stable-event retry, Bearer authentication, and idempotency E2E coverage.
- Resource Center Webhook/docs browser QA passed at the browser's 560px minimum width with no horizontal overflow; the active ordinary-user session correctly remained blocked from the administrator-only async-task page.

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
# 2026-07-12 UsageStats table layout audit
- Rebuilt and recreated `new-api-dev` from the final source; image `sha256:948eb79a1ad3b4663b682a5f2e4784606fd0b4ac5111b07213d51dbf19449c22` is healthy on port 3001.
- Replaced shrinking `max-content` table widths with container-filling desktop widths and bounded mobile scroll widths.
- Added explicit column widths and first-column ellipsis so Semi Table uses fixed layout and long usernames/order numbers cannot distort adjacent columns.
- Authenticated browser checks passed for total usage, subscription usage, recharge, subscription purchase, user model details, and funding order details.
- At 1440px the main table is 1174/1174px; at 1024px and 768px only the table body scrolls; at 375px the four-column table is exactly 580px inside a 327px viewport container.
- Document `scrollWidth` matched viewport width at 1440, 1024, 768, and 375px. Mobile header height is stable at 58px.
- Targeted Prettier and ESLint, production Vite build, Docker build, and `git diff --check` passed.
- Removed the temporary local administrator used for authenticated verification.
# 2026-07-12 Wallet usage ranking
- Started a new implementation phase for an independent wallet/usage-based ranking placed after total usage.
- Scope is backend `wallet_ranking`, frontend tab/order/detail source, focused tests, and Docker dev rebuild.
- Located the one-pass aggregation and existing mixed-source regression test in `model/log.go` and `model/log_test.go`.
- Confirmed source-specific rank rows reuse `usageStatsSortedUserRows`, preserving quota/request/user-ID ordering and the configured limit.
- Traced the frontend ranking mode and user-detail source flow; no new route or request cache layer is required.
- First focused model test reached the new wallet assertions, then failed only because an existing model-level expected wallet total needed to include the added fixture.
- Full i18n lint remains blocked by 421 pre-existing repository findings; none reference the changed UsageStats modules.
- Focused UsageStats model tests passed after updating the fixture-dependent model total.
- Targeted ESLint/Prettier, seven-locale key validation, and the production Vite build passed.
- Reviewed the complete wallet-ranking diff and reference graph; backend response, frontend mode, empty state, and wallet detail scope are connected.
- Built Docker image `sha256:f44fbd674575a60103588c21b3b8ebd74d3f0d6fd46bca96a404543a676986e8`; recreated app is healthy on port 3001.
- Created a disposable local root account for authenticated browser verification; it will be deleted after the audit.
- Authenticated successfully against the rebuilt Docker app and opened `/console/usage-stats`.
- Loaded the existing 2026-04-27 mixed-source dataset; overview reports `$0.83` wallet usage and 22 wallet requests.
- Verified the new wallet tab title, quota column, selected state, populated rows, and independent wallet ordering in the rebuilt Docker UI.
- Clicked the top wallet user and verified wallet-scoped detail data and copy.
- Mobile browser verification passed for tab fit, selected state, populated ranking, and absence of document-level horizontal overflow.
- Full `go test ./model`, final whitespace audit, Docker health/status check, and temporary-account residue check passed.
- Phase 6 is complete; unrelated untracked files remain untouched.
# Claude `Content block not found` Analysis Progress (2026-07-14)

- **Status:** complete
- Started repository and external-protocol investigation.
- Preserved the existing dirty worktree and added only scoped planning notes.
- Searched the complete repository for the literal error, `claude-fable-5`, Claude channel files, and content-block event handling.
- Initial result: local code does not originate the literal error; investigation is now tracing relay event ordering and alias routing.
- Read the native Claude adaptor, both response-conversion directions, stream state structures, focused tests, and relevant git history.
- Identified one concrete invalid-sequence candidate: tool argument delta emitted before a tool block start when an OpenAI-compatible upstream streams arguments before the function name.
- Searched the web for the exact error and current official Fable 5 documentation.
- Found an official Claude Code client fix, official confirmation that Fable 5 is a real GA model, a detailed Ollama invalid-index reproduction, and a same-repository issue pointing at OpenAI-channel conversion.
- Confirmed the Claude Code client-side fix landed in v2.1.186 and read new-api issues #4389, #5102, and #5126 via the public GitHub API.
- Distinguished request-block compatibility (#5126) from the response-stream state error, and identified missing converter test cases for sparse/unstarted tool blocks.
- Reproduced an invalid `delta(index=1)` before `start(index=1)` directly against the current converter using a temporary external diagnostic, then removed the diagnostic file.
- Confirmed the local app is healthy in Docker and mapped the risky `/v1/messages` OpenAI-channel round trip to the exact adaptor/helper functions.
- Searched 48 hours of local container logs; no matching Fable/error record was present. Logged two harmless SQL quoting failures before a safer metadata-query retry.
- Audited safe local DB metadata: Fable uses an Anthropic-type third-party channel, and available historical calls are successful non-streaming June tests.
- Ruled out ping/data interleaving after confirming both writers share a mutex; added Fable's signature-only thinking block shape to the compatibility analysis.
- Checked the local Claude Code version (2.1.209) and searched its debug directory for the exact error; no matching trace was retained.
- Incorporated the user's exact channel/request metadata and corrected the direct-path attribution from OpenAI-response synthesis to OpenAI-request/Claude-response conversion.
- Proved with a temporary round-trip diagnostic that OpenAI message thinking signatures are discarded before the Claude tool continuation is sent upstream; removed the diagnostic afterward.
- Focused service and Claude-channel tests passed, as did planning-file diff checks. No product code was changed.
- Product code changes: none.

---

# 2026-07-17 Aggregate group categories and token group UX

- Completed backend persistence, cross-database migrations, category CRUD/order/delete/assign APIs, response metadata, and focused model/controller coverage.
- Completed the category manager, aggregate-group category editing/filtering/batch assignment, responsive CardTable selection, and category-grouped token selector with historical-value handling.
- Full `go test ./...`, Bun tests, the production frontend build, targeted Prettier/ESLint, locale JSON validation, and `git diff --check` passed. Repository-wide i18n/lint commands retain only their documented pre-existing baselines.
- Built the current embedded-web Linux arm64 binary into `new-api-local:dev`, recreated `new-api-dev`, and confirmed the status endpoint and PostgreSQL migrations.
- Authenticated Docker browser QA passed at 1440px, 768px, and 375px in light/dark themes for category management, filtering, selection, batch assignment, edit-category display, token grouping/search, `auto` exclusion, and historical one-way selection.
- Fixed the delete confirmation trigger discovered during browser QA, rebuilt Docker dev, and verified deletion count messaging plus fallback of two assigned groups to Other.
- Removed all disposable categories, category assignments, historical token, administrator, and generated Docker build files; restored the final page to the 1440px light aggregate-group view.
- Follow-up: increased token OptGroup contrast and added option dividers after screenshot feedback, rebuilt Docker dev as image `a94c456e...`, and verified the result at 1440px/375px in both themes with no final mobile overflow.

---
# Async Image Open API and Webhook Progress (2026-07-17)

- Loaded brainstorming and planning-with-files instructions, recovered prior task context, and confirmed the user-approved implementation plan is decision complete.
- Inspected both worktrees. No existing product-code modifications overlap this work; prior planning records and diagnostics are being preserved.
- Started Phase 1 with persistence and public API contracts.
- Added initial cross-database image request/dispatch and Webhook models, public async-image DTOs, task/asset query helpers, and the public task mapping service.
- Focused `go test ./model ./service` passed.
- Added durable public task creation and dispatch wiring, formatted the new backend files, and confirmed relay/router/service/model packages compile and pass. Controller contract tests now need the planned public-response fixture migration.
- Completed and formatted the public async-image controller contract tests. `go test ./controller ./relay ./service ./model ./middleware ./router` passed.
- Confirmed the Webhook backend is implemented end to end: management APIs, scoped keys, stable signed events, dual-secret rotation, retry/lease behavior, SSRF protection, manual retry, and retention cleanup.
- Added a per-claim fencing token to ImageTaskDispatch. Stale workers can no longer mark, reschedule, or fail a dispatch after another worker has reclaimed its expired lease.
- Added the independent Resource Center Webhook UI and API Key scope controls, merged all new strings into zh-CN/zh-TW/en/fr/ru/ja/vi, and verified that every newly introduced translation key exists in all seven locales.
- Generated a deterministic OpenAPI 3.1 Resource Center document covering 21 Assets/async-image/upload/Webhook operations plus two outbound Webhook definitions, and switched the frontend away from its embedded Assets-only OpenAPI 3.0.3 object.
- Validated the generated OpenAPI 3.1 document with Redocly at zero warnings and completed another successful frontend production build using the canonical document import.
- Fixed permanent dispatch terminal ordering and added passing regressions for failure refund/Webhook creation, terminal-transaction recovery, and 16-way durable idempotency record competition.
- Resumed from the implementation handoff, re-read both repositories' plans and diffs, and narrowed remaining work to Docker E2E, responsive browser QA, full regression, and final limitation documentation.
- Audited the live Docker state: new-api and image-handle are healthy but still use old images, and the external `ai-gateway` network has no attached containers. Rebuild/recreation is required before gateway integration testing.
- The first new-api Docker rebuild exposed that `.dockerignore` excluded the newly canonical OpenAPI document; narrowed the ignore rule to include only `docs/openapi/**` for the frontend build context.
- Rebuilt both dev images successfully, recreated only application-layer containers, and verified shared-network DNS plus HTTP health in both directions without replacing PostgreSQL/Redis data volumes.
- Created a scoped local user/token/asset-key/channel fixture and passed endpoint creation plus signed test delivery. The first task attempt was safely rejected before persistence because its custom group was not registered; the fixture is being changed to a unique model alias in the valid default group.
- Docker generation E2E passed task creation, replay/conflict, polling, asset result, public redaction, filtered list, ordered batch/missing, and a signed succeeded Webhook whose first receiver response was HTTP 500. Manual and automatic retry state paths remain to be exercised separately.
- Automatic retry delivered the same stable event ID after the real one-minute delay (500 then 204); an explicit failed fixture also passed manual retry and a third signed delivery.
- Base64 and multipart pre-upload passed with repeated images, mask, temporary metadata, and real R2 URLs. The first edit exposed and then verified the image-handle pinned-DNS Undici fix; the rebuilt worker completed pre-uploaded edit, resource creation, and succeeded Webhook end to end.
- Completed the deliberate permanent-failure E2E: normalized failed task, zero assets, signed failed Webhook, exact user refund, and restored/healthy channel configuration.
- Full regression passed `go test ./...`, image-handle `npm test` (72/72), `bun run build`, `bun run openapi:check`, and `bun run i18n:status`. Repository-wide i18n lint still reports its existing 422 hardcoded-string baseline and is being supplemented with change-scoped checks.
- Added and passed the schema contract on SQLite, PostgreSQL 15, and MySQL 5.7/utf8mb4 using isolated disposable databases.
- Browser QA at desktop, 560px, and true 375x812 found and fixed fixed-width Resource Center SideSheets; final Webhook and API Key scope drawers fit the viewport with all inputs/actions visible.
- Completed the final source audit: durable task/request/lease/dispatch creation is one SQL transaction, billing precharge remains a documented compensating workflow rather than a cross-store atomic reservation, changed Go files use the common JSON wrapper, dispatch writes are lock-token guarded, and the local Webhook receiver secret was cleared.
- Rebuilt `new-api-local:dev`, recreated only `new-api-dev`, and confirmed the application is healthy with the final frontend.
- Removed all disposable new-api/image-handle/PostgreSQL/BullMQ/receiver fixtures; final audit counts are zero and R2 cleanup remains delegated to its one-day lifecycle.
- Final checks pass OpenAPI drift, seven Compose combinations, full Go/image-handle/frontend builds/tests, 63-key locale completeness, targeted Prettier/ESLint, both repository diff checks, and unchanged `web/bun.lock`.
- 2026-07-17: Started the Webhook simplification follow-up. The approved contract is one independent image-task URL plus user-supplied Bearer key; public management APIs/scopes and multi-endpoint UI will be removed while durable delivery remains internal.
- 2026-07-17: Generalized the configuration boundary to an account-level task Webhook so future video events can reuse it; this change still implements only the existing image success/failure event producers.
- 2026-07-17: Reconfirmed the future-video constraint during final verification: the Webhook configuration and durable delivery substrate remain task-generic, while this iteration intentionally emits and documents only image terminal events.
- 2026-07-17: Docker Webhook receiver observed two Bearer-authenticated attempts for one stable test event after a forced first 500; event ID and body were identical across retry.
- 2026-07-17: PostgreSQL audit confirmed attempt 1=500, attempt 2=204, final delivery status `delivered`, and encrypted Key storage with no plaintext occurrence.
- 2026-07-17: Docker 410 flow passed. The endpoint auto-disabled, URL-only save re-enabled it without replacing the Key, and the next delivery authenticated and completed with 204.
- 2026-07-17: Removed an unused asset-key scope updater, synchronized the returned `updated_at` after automatic disable on Key decryption failure, and added a deterministic database/API timestamp regression assertion. Focused lifecycle/crypto/410 tests pass.
- 2026-07-17: Pre-cleanup audit resolved the disposable E2E target exactly as user `994191` / `whk07172110`, with one endpoint and three test events/deliveries.
- 2026-07-17: Deleted the disposable account through `/api/user/self`, then removed only its durable Webhook attempts/deliveries/events/endpoint and hard-deleted its already-soft-deleted user row. Final database and receiver audits are all zero; the receiver Key is cleared.
- 2026-07-17: Final regression passed `go test ./...`, image-handle `npm test` (72/72), image-handle production/TypeScript build, new-api `bun run build`, and `bun run openapi:check`. Full i18n lint still exits nonzero on 422 repository-wide pre-existing hardcoded strings; the new Webhook component files are absent from the report.
- 2026-07-17: Final route/scope/signature audit passed. The only remaining `X-Webhook-Signature` implementation belongs to the separate quota-notification Webhook. Both repository diff checks and Docker health checks pass; the simplified task Webhook phase is complete.
- 2026-07-17: Rebuilt and force-recreated `new-api-dev` from the final source after the timestamp cleanup. The embedded frontend build, Go image build, status endpoint, and Docker health check all pass; local service is available on port 3001.
- 2026-07-17: Started the saved-view/generated-Key UX follow-up from the user's configured-state screenshot. Locked the token-style behavior: system-generated `wk-...`, one-time plaintext display, read-only saved detail, explicit edit, and explicit regeneration.
- 2026-07-17: Implemented server-generated Webhook Keys. Create returns a one-time `wk-` plus 48 random characters; GET and URL-only updates remain redacted; explicit regeneration rotates the encrypted credential. Focused lifecycle, crypto recovery, Bearer retry, and 410 tests pass.
- 2026-07-17: Implemented the frontend state split and one-time Key modal, then passed targeted Prettier and ESLint. The first combined locale patch changed nothing due to one mismatched Traditional Chinese value; exact locale tails were inspected before retrying per file.
- 2026-07-17: User replaced the one-time reveal requirement with persistent authenticated reveal/copy. Updating the in-progress implementation to return decrypted Key on account GET/PUT, show it behind an eye toggle, and retain system-only regeneration for modification.
- 2026-07-17: Completed persistent authenticated Key reveal/copy. Focused controller/service/router tests, JSON validation, OpenAPI regeneration/check, i18n status, targeted frontend lint/format, and production frontend build all pass.
# Webhook Saved View and Generated Key UX Progress (2026-07-17)

- Recovered the completed backend/frontend implementation and latest requirement: generated `wk-...` Keys remain revealable and copyable after creation.
- Confirmed the rebuilt `new-api-dev` container is healthy and `/api/status` succeeds on port 3001.
- Compared the supplied old-state screenshot with the new component contract; saved state is now a detail view and URL editing is explicit.
- Opened the rebuilt app in the in-app browser; the existing browser session is logged out, so responsive and interaction checks will use a disposable local account rather than a real configuration.
- Registered and signed in as disposable local user `codexwhux0717`; confirmed the Webhook create state and captured its desktop rendering.
- Created a disposable Webhook and verified generated prefix/length, saved detail state, hide/show, reload persistence, and URL/Key copy feedback in the rebuilt Docker UI.
- Simplified the crowded Key row, rebuilt Docker dev, and verified the updated desktop layout.
- Verified URL edit/cancel/save preserves the existing Key and confirmed explicit regeneration produces a different valid Key; adjusted URL saves to keep the Key masked.
- Rebuilt Docker dev again and confirmed ordinary URL saves keep the Key masked.
- Completed responsive browser QA at 560px and 375x812 with the Key revealed; both viewports have matching client/scroll widths and no overlapping controls.
- Full Go tests and all 72 image-handle tests pass; frontend build and OpenAPI check pass. Full i18n lint exposed one change-scoped literal on top of the known repository baseline, which is being removed.
- Removed the change-scoped i18n finding; targeted Webhook formatting/ESLint is clean and the repository returned to its existing 422-item lint baseline.
- Deleted disposable local user id 994192 and its one endpoint after confirming no deliveries/events/attempts/tokens existed.
- Rebuilt and recreated `new-api-dev` from the final source; `/api/status` succeeds on port 3001.
- Marked the saved-view/generated-Key UX phase complete.
- Removed obsolete one-time-display and saved-status locale entries from all seven languages; locale JSON/status and the 422-item lint baseline remain valid.
- Performed the final Docker rebuild after locale cleanup; `new-api-dev` reports healthy and its status endpoint succeeds.
# Multipart Async Image Editing Progress (2026-07-18)

- Recovered the clean `main` baseline at `e1aeeaba4`; unrelated untracked diagnostic files remain untouched.
- Confirmed the user-approved route/content-type design and scoped it to asynchronous image editing.
- Inspected the current upload validation/proxy, normalized task DTO, idempotency preflight, relay persistence context, synchronous multipart field names, docs, and OpenAPI generator.
- Started backend implementation.
- Implemented strict multipart parsing and synchronous-style edit field mapping on `POST /v1/image/tasks`.
- Added content-hash fingerprints, pre-upload idempotency replay, shared upload forwarding/response parsing, and normalized URL materialization.
- Added the distributor's async-image multipart model extraction without changing other multipart routes.
- Focused `go test ./controller ./middleware` passes.
- Added the follow-up Webhook retry requirement to the active scope after inspecting the current one-shot worker, durable delivery model, and Async Task Management settings page.
- Implemented administrator-configurable Webhook attempts/interval, 2xx-only success, and bounded retries with no database migration.
- Updated Async Task Management, Resource Center docs, all seven locales, and generated OpenAPI; focused Go tests and OpenAPI generation/check pass.

---
# Resource Center API Documentation Progress (2026-07-18)

- Audited the screenshots and existing `ResourceCenterDocs.jsx` structure.
- Confirmed the missing coverage spans async task list/batch lookup, Base64 upload, and four asset operations beyond list.
- Started an OpenAPI/backend contract audit before editing examples.
- Completed the 11-operation contract audit against the generated OpenAPI document and relevant controller/DTO locations.
- Began implementing the per-operation example structure.
- Added complete example payload constants for task list/get/batch, Base64 upload, asset get/query/URLs/export, and updated multipart examples to show repeated image fields.
- Replaced the combined partial sections with complete per-operation request/response sections for all 11 advertised endpoints.
- Added the new operation titles and guidance to all seven frontend locales.
- Formatted the documentation component and all changed locale files; `git diff --check` passes.
- Targeted ESLint, OpenAPI generation check, and production build pass.
- Full i18n lint remains at the existing 421-item repository baseline and reports no finding in the changed documentation component.
- Rebuilt and recreated the healthy `new-api-dev` container from the changed source.
- Desktop browser QA passed for the async image documentation; identified one missing legacy dynamic-title translation to correct before mobile QA.
- Added the missing `创建异步图片任务` translation to all seven locales.
- Completed 560px and requested 375x812 responsive QA plus full asset-operation example verification; no layout overflow was found.
- Final Prettier, targeted ESLint, OpenAPI check, production build, and whitespace checks pass.
- Rebuilt/recreated Docker dev after the last locale change and verified the final container serves the translated create operation, all 15 async example cards, and the repeated-image multipart example.
- Final Docker health is `healthy`; unrelated user diagnostic files and older pending planning work remain untouched.

---
# Automatic Error Snapshots Progress (2026-07-20)

- Resumed from a completed backend implementation and passing focused backend test suite.
- Re-read the brainstorming, file-planning, and UI/UX skill instructions; the user-provided implementation plan is treated as the validated design.
- Inspected the existing Request Dump page and selected a tab shell plus isolated error-snapshot component.
- Started Phase 2 frontend contract discovery.
- Confirmed all error-snapshot API response shapes and existing Semi Design patterns for SideSheet, pagination, date filters, confirmation, and copy actions.
- Implemented the top-level button tabs and complete Error Snapshot management UI.
- Added real translations for the new feature in zh-CN, zh-TW, en, fr, ru, ja, and vi, plus eight previously missing temporary-Dump strings.
- Targeted Prettier and ESLint pass; `bun run build` passes with only existing dependency/chunk-size warnings.
- Full `i18n:lint` exposes the repository baseline; all new component findings were removed and page-key coverage is zero-missing in every locale.
- Added oldest-first file-count/storage cleanup, queue-full nonblocking, and fallback-outcome ordering tests; all focused packages pass.
- `go test ./...` passes.
- Claude integrity benchmarks pass and show no regression: integrity 33.9 us/op versus legacy 50.9 us/op; first-block p95 33.5 us versus 54.3 us on this machine.
- Full frontend ESLint remains blocked by 68 existing generated/source header findings; targeted changed-file ESLint passes. Scoped vet only reports the existing `model/invite_code.go` self-assignment.
- Resumed Phase 4 in the running Docker app at an exact 375x812 CSS viewport. The Error Snapshot tab loads and exposes all expected controls. A misleading fractional-scale full-page screenshot was checked against DOM geometry: the page is actually 375 px wide with a 349 px content area and no document overflow. Continue acceptance with scrolled viewport captures and element-bound assertions.
- Scrolled mobile QA confirms the complete settings form and destructive/cleanup actions fit the viewport. Expanded the CardTable mobile action area and verified all five filters, search/reset controls, and the empty-list state are present and usable.
- Completed a final implementation audit and added multipart/binary metadata plus broader credential-redaction coverage. Added focused tests for metadata-only capture and embedded secret assignments; the service package passes after correcting a test that initially asserted against the escaped outer JSON instead of the decoded envelope.
- Final verification passes: `go test ./...`, targeted Request Dump ESLint, production build, and `git diff --check`. Repository-wide i18n lint remains at its known 421-item baseline and reports no finding in the new Error Snapshot component.
- Rebuilt/recreated Docker dev with the final source and reran the complete Claude/error-snapshot fault-injection suite; all 15 checks passed. The container is healthy at `http://localhost:3001`, default snapshot settings and an empty index were restored, and the temporary browser-test user remains role 1 with zero menu permissions.

---

# Per-user Aggregate Route Model Ratio Progress (2026-07-21)

- Loaded the validated implementation plan and re-read the brainstorming, file-planning, and UI/UX instructions.
- Confirmed the current storage, resolver precedence, pricing response, user-group response, log sanitization, cache refresh, admin permission, and Docker dev topology.
- Locked a backward-compatible user-setting list and extension of the existing user ratio endpoint; backend implementation is starting.
- Added per-user aggregate child-route exact-model rules to the existing user-setting JSON, including backward-compatible PUT semantics, strict validation, disabled-rule fallback, exact case-sensitive matching, and valid zero ratios.
- Unified resolver precedence as user exact, global exact, user aggregate default, then aggregate default; synchronized relay billing, task snapshots, final settlement, pricing aggregation, and audit-source metadata.
- Extended the user-management API and SideSheet with child-route model rules plus a user-menu-scoped model candidate endpoint; ordinary pricing, group, and log responses now expose only final effective ratios.
- Removed comparison strike-throughs and exclusive-ratio labels from model pricing, model detail, token, and Playground renderers, including compatibility behavior against older backend payloads.
- Added backend resolver/controller/pricing/relay/task/log coverage and pure frontend helper tests. `go test ./... -count=1`, five Bun helper tests, targeted ESLint/Prettier, production build, and changed-scope i18n checks pass.
- Rebuilt Docker dev and completed four real billed requests: user exact `0.5`, global exact fallback `3`, user aggregate default fallback `0.8`, and aggregate default fallback `1.2`; quota deltas and administrator audit metadata matched each source.
- Browser QA passed administrator add/edit/enable/disable/delete flows and ordinary-user model pricing, model detail, token, Playground, and expanded log views on desktop and mobile. No final-ratio surface has strike-throughs, exclusive labels, sensitive override fields, or horizontal overflow.
- Removed the isolated users, token, channel, abilities, aggregate group, targets, exact rule, logs, Redis state, and mock server. All fixture residue counts are zero; `new-api-dev` remains healthy and `/api/status` succeeds.

---
# Temporary Video Resource and Webhook Progress (2026-07-23)
- Confirmed with the user that upstream-backed temporary access is sufficient; no object-storage archival will be added.
- Recovered the completed xAI/sub2api compatibility work and audited current Resource Center authentication, asset creation, video proxying, and image-only Webhook event creation.
- Started validating a minimal shared contract that reuses existing Asset APIs, publishes only new-api task-based proxy URLs for relative upstream results, and adds video success/failure events to the existing durable Webhook outbox.
- Confirmed that the current content route already accepts Resource Center Keys via `TokenOrUserAuth -> AssetOrTokenAuth`; no new download endpoint or authentication system is needed.
- Narrowed the design to task status via `/v1/videos/{task_id}`, resource discovery via `/v1/assets?asset_type=video`, and terminal `video.task.succeeded` / `video.task.failed` events.
- The user approved the expanded multi-provider plan: normalized create/query APIs, structured multi-output results, xAI edit/extension mapping, per-asset downloads, shared video Webhooks, public docs, and Docker real-call acceptance.
- Started backend contract implementation while preserving the existing uncommitted xAI/sub2api compatibility fixes.

---
# Multi-provider Async Video Progress (2026-07-23)

- Resumed the user-approved implementation after context handoff and loaded the required brainstorming and file-planning workflows.
- Preserved the existing xAI compatibility/proxy changes and all unrelated untracked diagnostic artifacts.
- Confirmed the first implementation phase will reuse the image-task idempotency, user-isolation, Asset, and durable Webhook architecture while keeping a separate provider-neutral video contract.
- Started detailed inspection of the existing relay submission, task persistence, public image DTO, Asset projection, and Webhook terminal transaction.
- Confirmed multi-output Asset ordering is already supported by schema; no Asset migration is needed.
- Identified the upstream-before-persistence race in ordinary video submission and added a pre-upstream idempotency reservation requirement to the implementation approach.
- Confirmed normalized provider behavior will be an optional adaptor capability, so all existing video providers remain source-compatible and can adopt explicit normalized operations incrementally.
- Confirmed the existing content proxy will be refactored into a shared task/Asset streaming path, retaining its current security and Range behavior.
- Verified current xAI generation enums/ranges from the official schema and recorded the conservative 1080p model restriction used by the adaptor.
- Added normalized video DTOs, strict preparation, durable request persistence, cursor/batch query APIs, task public projection, split mutation/query authentication, and cross-database migration registration.
- Added the optional normalized-video adaptor contract and xAI generation/edit/extension conversion with official ranges/enums, provider namespace isolation, file references, model-aware 1080p validation, and normalized-response handling.
- Added structured `VideoOutputs`, legacy fallback Asset creation, public CDN versus Asset proxy projection, per-Asset content streaming, internal metadata filtering, and upstream expiration mapping.
- Generalized terminal Webhook creation so normalized video success/failure uses the same transaction/outbox and account `wk-` as image tasks.
- First focused compile/test pass succeeded for dto, model, service, xAI adaptor, relay, controller, and router packages.
- 2026-07-23: Started the video image-input capability correction after user confirmation. The public DTO shape remains singular `image`, array `reference_images`, and singular `video`; validation ownership is moving from the shared controller to the xAI adaptor.
- 2026-07-23: Relaxed provider-neutral video validation while retaining operation semantics, added xAI-specific image combination/count/model/duration/output rules, corrected the reference-generation model example, and regenerated the public OpenAPI. Focused controller/xAI tests pass; broad verification is in progress.
- 2026-07-23: Completed the video image-input correction. Full Go tests, frontend build, OpenAPI drift, targeted ESLint/Prettier, i18n status, Docker image rebuild, desktop Resource Center QA, and two Docker xAI capability probes pass. The disposable browser account was self-deleted and hard-cleaned; `new-api-dev` is healthy on image `sha256:b8c9fa2e0805...`.
# Resource Center DTO Documentation Progress (2026-07-24)

- Loaded the required brainstorming, UI/UX, and file-planning workflows and recovered the existing workspace state.
- Locked the implementation direction: audit real DTOs, complete OpenAPI field metadata, then render reusable request/response definitions in the existing documentation page.
- Preserved all unrelated modified planning records and untracked diagnostics.
- Audited all 16 public Resource Center operations against image/video/Asset DTOs and controller validation.
- Added a standalone OpenAPI schema renderer with desktop tables and mobile stacked rows, including path/query/header parameters, JSON and multipart bodies, response headers, JSON DTOs, CSV, and binary response bodies.
- Connected definitions to every request example in Async Images, Async Videos, Assets, the common error envelope, and the outbound Webhook payload.
- Added generated OpenAPI descriptions for every public schema property and corrected the single-Asset example so it no longer exposes nonexistent public `platform`/`action` fields.
- Added the new presentation labels to all seven locale catalogs. Targeted ESLint, OpenAPI drift, i18n status, and `git diff --check` pass.
- Browser acceptance exposed English OpenAPI descriptions inside the Chinese dashboard. Added localized OpenAPI description extensions and changed the renderer so Chinese locales never fall back to English descriptions; constraint labels are localized through the normal seven-locale catalogs.
- Completed the final union-requiredness pass: shared fields across every `oneOf` object remain required, variant-only and nullable nested fields are labeled conditionally required.
- Filled the existing empty `可选` translation in zh-TW, en, fr, ru, ja, and vi so the new requirement tag is visible in every supported locale.
- Final targeted ESLint, OpenAPI drift check, i18n status, production frontend build, displayed Chinese-description completeness check, and `git diff --check` pass.
- Rebuilt and recreated `new-api-dev` as image `sha256:226fb59e02bc...`; the container is healthy and `/api/status` succeeds at `http://localhost:3001`.
- Desktop browser acceptance confirms Chinese descriptions, video-source conditional requirements, Webhook shared required fields, and zero console errors. Final 375x812 acceptance confirms stacked field rows, no page-level horizontal overflow, and no text overlap.
- Started the field-table visibility follow-up after confirming the existing definitions were hidden by an outer Collapse.
- Removed the schema Collapse, split desktop tables into 名称/类型/是否必须/描述/备注, moved parameter location and structural constraints into Notes, and retained a labeled mobile stacked layout.
- Added localized location and required-column labels across all seven locales. Targeted ESLint, OpenAPI drift, i18n status, production build, and whitespace checks pass.
- Rebuilt Docker dev on image `sha256:8e4867473528...`; the container is healthy. Desktop browser acceptance confirms the five-column table is directly visible with Chinese field descriptions and no schema-collapse control.
- Mobile browser acceptance confirms the direct field definitions retain every column's meaning in stacked form at 375x812 with no page-level horizontal overflow.
- Final desktop/mobile console error counts are zero. The field-table visibility follow-up is complete, and the rebuilt Async Images page is left open for user inspection.

---
# Adobe2API Seedance 2.0 Fast Integration Progress (2026-07-29, resumed)

- Recovered the active task from the planning files and preserved all unrelated dirty-worktree changes.
- Adobe2API now exposes asynchronous `POST /v1/videos`, `GET /v1/videos/{id}`, and `GET /v1/videos/{id}/content` routes backed by its scheduler, retry, account, generated-file, and in-memory job services.
- Added stable resolution-bound Fast and standard Seedance 2.0 provider SKUs; the isolated Seedance suite passes 15/15, image regression passes 31/31, compile validation passes, and the Adobe2API diff is whitespace-clean.
- Advanced the active work to the dedicated new-api AdobeVideo channel/adaptor integration.
- Added channel type 59, task-adaptor registration, channel-management label, async-task labels, and the dedicated AdobeVideo adaptor.
- The adaptor accepts only normalized text generation, requires 4-15 integer seconds, rejects public resolution and protected-field overrides, forwards exact mapped provider SKUs, and resolves completed content through the authenticated Adobe2API content endpoint.
- Focused adaptor lifecycle/content tests pass. A relay-level pricing test confirms the approved 4-second request at `$0.03/second` and group ratio `1.5` produces quota `90000`, has no legacy other ratios, and forces wallet-only billing.
- Full affected backend packages (`relay/...`, `controller`, and `service`) pass. Changed frontend files pass Prettier and the new-api diff passes whitespace validation.
- Phase 5 is complete; Docker-dev mock acceptance is now in progress.
- Extended the opt-in async-test mock with AdobeVideo submit/poll/content routes, configurable success/failure terminal states, Range content, request counters, and normalized-payload capture; its focused tests pass.
- Rebuilt new-api, async-test-mock, and Adobe2API images and recreated only those containers. All three are healthy; PostgreSQL and Redis retained their existing 27-hour uptime.
- Docker mock acceptance passed 202 submission, queued/in-progress/succeeded polling, normalized upstream payload, Asset proxying, 206 Range, idempotent replay, terminal and submit failure refunds, and wallet-only billing despite an active subscription-only preference.
- The one authorized real request completed through channel 114 as `task_lAWYIj6zvti1beLvqsuZKwd1sWMMJvrP`: Fast 480p, 4 seconds, 16:9, text-only, and audio disabled.
- The real MP4 is H.264, 864x496, 4.042 seconds, and 549065 bytes. It is retained under `outputs/` with SHA-256 `e94e7b8481221e7506766b7d7b5ff9f4a20380ab83c8d84952977d598457ca2b`.
- Real billing matched the immutable snapshot: `$0.03/second * 4 * 1.5 = 90000` quota, wallet `910000 -> 820000`, subscription usage unchanged at zero, and `subscription_enabled=false`.
- Real content testing exposed that the installed Starlette `FileResponse` ignored Range. Adobe2API now implements authenticated single-byte ranges, suffix ranges, `If-Range`, and 416 responses; all 72 Adobe2API tests pass and its Docker image was rebuilt.
- Removed the disposable user, token, Resource Key, subscriptions, plan, tasks, Assets, logs, Webhook events, mock/real channels, abilities, pricing Option, and backup Options. Restored the exact original GroupRatio and reset the async mock to zero counters/default behavior.
- Final verification passes `go test ./... -count=1`, 17 frontend helper tests, the production frontend build, i18n status, both repository whitespace checks, and Docker health. Repository i18n lint remains at its 441-item pre-existing baseline with no newly introduced finding.
## 2026-07-29 — Claude 渠道“空任务回复”诊断

- 已读取 `planning-with-files` 技能说明并执行 session catch-up，无未同步报告输出。
- 已检查工作树：发现既有大型规划文件及多个无关未跟踪文件，均保持不变。
- 下一步：读取客户 `main.go`，然后对照 channel 70/Claude SSE 代码路径。
- 已完整读取客户 `main.go`（710 行），确认判定器存在证据口径偏宽问题。
- 下一步：寻找实际 capture 样本，并追踪当前源码的 channel 70、Claude 请求与 SSE 转发路径。
- 未找到客户 capture 样本；递归临时目录时遇到 macOS 系统目录权限错误，已停止扩大搜索范围。
- 已确认 DB channel ID 70 不能等同于 channel type；当前源码 Anthropic type 为 14。
- 已追踪旧 Claude 原生 SSE 路径，确认网关本地 ping + 吞掉 EOF/scanner error 可形成“200 只有 ping”。
- 已追踪新增完整性保护路径：首个 content block 前缓冲，空流会转 502；需继续核对默认开关、客户版本和重试条件。
- 用户澄清 supertoken 即己方服务，channel 70 无诊断价值；后续聚焦己方请求转发与异常成功判定。
- 已确认完整性修复 commit 日期为 2026-07-20，当前默认开关关闭；这可解释昨天生产仍出现旧行为。
- 已定位既有 Claude content-block 诊断与完整性验证材料，准备交叉核验。
- 已完整读取两组既有诊断文档和 task-local repro：空流/空内容均属于旧 handler 的异常成功判定；有文本问候仍需从请求顺序或上游响应链定位。
- 已核对 role 归一化：native Claude 路径不会自动改中途/尾部 system role；正在排除 body storage/并发复用并查找 Request ID 证据。
- 已排除当前实现中的跨请求 BodyStorage 共享；两个截图 Request ID 在本机无匹配日志。
- 普通相关包测试通过；race detector 暴露 legacy stream stop-channel race 和 Claude global settings race，正在隔离验证。
- 单例 legacy stream race 测试首次通过；settings race 已确认可由纯并行请求读取触发，但不直接改请求正文。
- 30 次单例 legacy stream race run 已复现并确认 stop-channel send/close 竞争。
- 已校验小样本置信区间并核对 request-dump 能力；准备汇总结论和取证边界。
- 诊断完成：未改业务代码；普通相关包测试通过，race run 揭示并确认两类竞争，其中 legacy stream stop-channel race 与 ping-only 直接相关。
- 通用 completion script 因共享 task plan 中较早任务的两个 pending phase 返回非零；未修改无关历史任务，本次四阶段已全部完成。
## 2026-07-29 — 公开讨论检索

- 已读取 `github` 与 `planning-with-files` 技能，完成 session catch-up。
- 已建立检索计划；下一步确认两个官方仓库 remote，并加载 GitHub/网页搜索工具。
- 已确认 sub2api upstream remote，定位 new-api 官方候选，并加载 GitHub 与 Exa 工具。
- GitHub connector 已确认 `QuantumNous/new-api` 仓库元数据。
- 已完成首轮两个官方仓库 Issue 搜索，定位多条直接相似报告；下一步核对 issue 元数据、评论、关联 PR/commit。
- `gh` CLI 不可用；已切换到 connector + GitHub public REST 的只读方案。
- 已通过 public REST 核验四个高相关 Issue 的作者、日期、标签与状态。
- 已读取 new-api #5411/#6429 评论，确认关闭理由与证据权重。
- 已核验 new-api #4067/#4139/#4389/#4697、sub2api #1493/#1651/#1661/#2064/#2377/#4077/#4114/#4177/#4193 等高相关报告的作者、时间、状态和正文。
- 已核验 sub2api #1501/#2972/#4138/#4179/#4294/#4295 及 new-api #4128 等关联 PR 的合并状态，区分“已修复代码缺陷”和“仅有用户报告”。
- 已完成 LiteLLM、llama.cpp、9router、Claude Code 官方 tracker、LINUX DO、V2EX 与 X 的定向检索；未对搜索未命中的平台作否定性结论。
- 公开讨论检索完成：确认多个独立用户和多个网关项目遇到相似空成功/SSE block/上下文丢失问题；未发现能证明本次 Fable 5 被暗中替换的公开证据。

## 2026-07-29 — Claude 可疑成功响应被动采集

- 已确认范围：所有 Claude 成功响应、仅匹配后落盘、不重试、不改写响应、不改变计费或渠道健康。
- 已读取 brainstorming 与 planning-with-files 技能，并完成 session catch-up。
- 已建立四阶段实现计划；正在梳理三条 Claude 响应路径和快照生命周期。
- 已定位非流式、legacy 流式和完整性保护流式的安全采集点；下一步核对 DTO block 字段、快照页面 outcome/filter 和测试夹具。
- 已确认无需数据库迁移，但需扩展成功快照入口、Claude 可见文本收集、Claude-only 上游请求临时留存和 UI outcome 展示。
- 已确定有界 trace 和可见文本的实现边界；正在落代码前核对辅助写流函数和现有测试覆盖。
- 已完成主要代码路径审计，准备实现 service 侧匹配/快照入口和 Claude 侧有界诊断 trace。
- 已确认成功采集不会穿透到 controller 的 retry/渠道失败分支；开始业务代码修改。
- 已新增 diagnostic capture level、suspicious outcome、上游请求临时留存入口和 service 侧匹配/快照构建器。
- 后端和 Claude 聚焦测试通过；阶段 1-2 完成，开始更新 Dump 分析页面的结果标签、采集级别和响应详情。
- Dump 页面与七语言文案已更新，Prettier 通过；i18n lint 仍为仓库既有 441 条基线，本次页面无新增 finding。
- 前端生产构建与 `git diff --check` 通过；Go 回归仅发现新测试夹具缺少 diagnostics，已按设计修正，待重跑。
- 修正夹具后相关 Go 包和聚焦 race 测试全部通过；额外覆盖 content_block_start 首段文本及 OpenAI-to-Claude 上游请求留存。
- 全量 `go test ./... -count=1` 通过；前端生产构建、Prettier、差异检查通过。
- requested upstream model 与 response-reported model 已分开冻结和保存，相关包与 race 测试再次通过。
- 四个阶段全部完成；本地 Vite 前端运行于 `http://127.0.0.1:5173/`，未擅自启动会接触现有数据库的后端。

## 2026-07-29 — Claude 客户令牌实流量复现与诊断快照验收

- 已读取 `planning-with-files` 工作流并运行 session catch-up；没有返回未同步上下文。
- 已确认客户附件是独立 `gwprobe`，支持 capture 与 burst，按哨兵命中、空任务问候和 HTTP 200 空流分类。
- 当前正在核对完整 burst 参数、样本文件、目标端点和 Docker dev 拓扑；尚未发出任何带客户令牌的网络请求。
- 已完整审计 burst 实现：默认并发 12、单轮、`max_tokens=256`，会掩码打印凭据并读取 capture 样本；当前默认样本和目标端点均缺失。
- Docker dev 使用 `localhost:3001`、独立 PostgreSQL/Redis 和持久 `data-dev`/`logs-dev`，可用于验证本次快照代码，但不能自动观察直接发往生产网关的流量。
- 已确认本机 Claude Code `2.1.220` 可执行；现有 `new-api-dev`、PostgreSQL、Redis 均健康，`/api/status` 正常。
- 仓库既有 Anthropic 探针示例将 `https://supertoken.cc` 作为目标，因此先将其作为本轮客户令牌端点的可验证推测，以单请求确认，不直接扩大并发。
- 首次无计费 capture 等待 60 秒后未收到 `/v1/messages`；客户脚本吞掉了 CLI 输出，下一步改为可见错误的本地直连诊断。
- 直接运行同配置后确认 Claude Code 在发送前持续 `401 authentication_failed`；第 6 次重试时终止，结果显示 `total_cost_usd=0`、usage 全 0。停止 capture 重试，转向现有请求样本。
- 已检查三组既有 Claude 诊断目录和 `tmp/2dev` JSON/JSONL；没有完整 Messages 请求样本。将使用临时本地捕获器验证仅 AUTH_TOKEN 的当前 CLI 兼容方式。
- 用户纠正验收拓扑：临时令牌是本地 new-api 的上游渠道凭据；停止本地 Claude CLI 路径，改为创建隔离渠道并调用 `localhost:3001`。
- 用户指定上游香港加速线路 `https://hk.supertoken.cc`。已确认本地 Claude 渠道类型为 14，计划使用六个独立 group 保证请求与渠道一一对应。
- 已确认本地仅启用了 `claude.response_integrity_fallback_enabled=true`，尚无 `error_snapshot.*` 持久配置；验收前需启用自动错误快照。
- 已读取 channels/users/tokens schema；尚未写入任何渠道或令牌。
- 鉴权代码确认管理员令牌支持 `specific_channel_id` 后缀，普通用户不允许；这可用一个隔离的本地管理员令牌逐个固定命中六个渠道，避免新建六套 group ratio。
- abilities 表结构与 channels 一致，若仍使用普通分组路由可直接配套写入；当前优先采用指定渠道路径。
- 已确认精确格式为 `sk-<数据库48字符key>-<channel_id>`；distributor 直接读取启用渠道，指定渠道请求在 relay 错误时也不会跨渠道重试。
- 快照配置在启动时从 `options` 加载；写入 `error_snapshot.enabled=true` 等选项并重启即可生效，无需管理后台会话。
- `docker compose -f docker-compose-dev.yml build new-api-dev` 已成功，镜像 `new-api-local:dev` 已包含当前后端和前端改动。
- 拓扑审计阶段完成；开始创建事务性实验夹具并建立逐渠道基线。
- 已在单个 PostgreSQL 事务中创建渠道 `115–120`、一个本地管理员探针令牌，并写入六项 `error_snapshot.*` 配置；查询输出只包含别名、ID 和配置，不包含完整上游凭据。
- `new-api-dev` 已使用新镜像 recreate，健康检查通过；渠道缓存与快照配置已从数据库重新加载。
- 已创建权限 `0600` 的无凭据 Claude Code 形态样本，包含 system blocks、2 个工具、历史对话、中途 system role 和最终 user 任务；附件 `gwprobe` 已编译到 `/tmp`。
- dra/channel 115 在 `keep-system=false`、单请求基线中 4.464 秒正确返回唯一哨兵，input usage=2094，未出现问候或空流。
- 其余五渠道单请求基线：maidoucoding/118 与 yimo/120 正确返回哨兵；vll/116 在 5.165 秒被完整性保护转换为 502 `invalid_sequence_before_first_block`；doubv5/117 与 9527/119 均在约 30.04 秒被转换为 502 `first content block timeout`。
- 已以原始分辨率读取客户新增两张截图，确认其 12 并发约 30k-input 实验和生产 24 小时零输出统计；下一阶段将放大输入并保留中途 system role。
- 本地 consume/error logs 已验证 6 条请求均命中指定渠道；三条 502 未重试，失败请求预扣费返还。开始检查自动错误快照的落盘内容。
- 三条完整性失败均生成 `priority/final_failure` 快照，文件权限 `0600`，请求体与渠道证据完整且未截断。
- 验收发现完整性失败快照当前没有 `stream` 字段：非法序列只保存合成 502 response，首块超时只保存请求与错误。已记录为后续针对性修正项。
- 首个大输入样本在 dra/115 上正确返回哨兵，耗时 10.293 秒，usage input=1259、cache_creation=47685；高于客户约 27.5k–27.9k cache-creation，正在按实测比例缩小。
- 将 system 扩展块从 1900 行缩到 1100 行后，dra/115 校准请求耗时 4.807 秒，usage input=1271、cache_creation=27953、cache_read=0，已精确贴近客户正常样本。
- 开始 dra/115 的 12 并发、保留中途 system role 实验。
- 保留中途 system role 的 12 条均在本地 31–41 ms 被 400 拒绝，未调用上游；当前原生 Claude DTO 只允许 user/assistant message roles。
- 移除 system role 后的 12 并发中，1 条到达上游并正确返回哨兵，11 条因本地管理员组倍率 99 导致预扣费不足而 403；开始调整隔离 token 的本地计费组。
- `ccmax-yimo` 倍率虽为 1 但管理员无权限，单条被本地 403；已改用允许的 `local-adobe2api` 倍率 1 分组并精确清除 token cache。
- 新分组下 dra/115 单条大输入到达上游，但 30.064 秒首内容块超时并返回 502，进一步确认相同令牌的间歇性空流。
- dra/115 正式 12 并发全部在 30.108–30.137 秒首内容块超时，12/12 返回明确 502；没有本地配额或路由错误。
- 客户问候样本耗时 58.9 秒，当前 30 秒 first-block timeout 会提前取消，无法验证成功问候采集；准备把本地超时临时提高到 90 秒。
- 本地 first-block timeout 临时提高到 90 秒并重启后，dra/115 重复 12 并发仍全部在 90.117–90.132 秒超时；跨过客户 58.9 秒问候点后仍无内容事件。
- 开始在曾成功的 maidoucoding/118 上重复 12 并发大输入，继续寻找结构完整的空任务问候。
- maidoucoding/118 的 12 并发在 33.9–36.5 秒全部返回客户端 500；本地内部日志显示上游实际为 5 条 400 与 7 条 502，均已生成 priority 快照。
- yimo/120 的 12 并发全部正确返回哨兵，耗时 38.736–62.913 秒，usage 全部 input=1271、cache_creation=27953、cache_read=0；未出现问候或空流。
- 已打开 maidoucoding 的一份 400 和一份 502 快照：upstream response 仅保留嵌套 request IDs 与 `bad response status code`，更深层 provider 原因未透出。
- 当前六渠道实验共 18 条成功、40 条错误，已生成 40 份 priority error snapshots；准备在 yimo/120 加一轮 12 并发作为有限追加采样。
- yimo/120 第二轮 12 并发仍全部正确返回哨兵，耗时 10.215–53.656 秒；两轮合计 24/24 正常，真实采样停止。
- 已读取 brainstorming 工作流并选择最小方案：在 response-integrity 失败构造处复用现有有界 Claude diagnostics，附加 StreamSummary；不包裹全局 writer、不加重试。
- 已实现完整性失败诊断附加：首块超时、首块前 EOF/scanner/malformed/error/非法序列/buffer limit 和写失败均保留 reason、commit 状态、事件计数与有界 SSE trace。
- malformed JSON 与 upstream error event 现会在提前返回前保存原始事件；已提交流的不完整结束同时保存此前下发事件和合成 `error` 事件。
- 修正错误快照 `is_stream=false`：请求解析出 `RelayInfo.IsStream` 后立即写入 context，不再等到成功日志/计费阶段。
- 聚焦单测通过，覆盖 ping 后超时、malformed raw event、非法序列、双向 committed trace 与 stream context 时序。
- Docker 故障注入 E2E 通过：问候保持 HTTP 200 并生成含 6 入/6 出 SSE 的 `suspicious_success`；ping-only 在 commit 前返回 502，快照为 received=1/sent=0；首块后 EOF 返回流内 error 并生成含 3 入/4 出 SSE 的 `stream_incomplete`。
- `go test ./...` 与 `bun run build` 全部通过；前端仅有既有 eval/chunk-size 警告。
- 本次新增测试的定向 `-race` 全部通过；三个相关包的全量 `-race` 仍会命中既有 logger 全局状态和 Claude settings 归一化竞争，未在本任务中扩大修改范围。
- 已删除渠道 115–123、9 条 abilities、令牌 147、44 份实验快照及临时探针/假上游文件；移除全部临时 `error_snapshot.*` 和 90 秒 timeout option，恢复管理员 quota=73,802,740，保留 `claude.response_integrity_fallback_enabled=true`。
- 最终 `new-api-dev` 镜像为 `sha256:561eec1cc0b9...`，容器健康，默认快照关闭且首块超时恢复 30 秒。

## 2026-07-30 — AdobeVideo 异步参考图片

- 已读取 brainstorming 与 planning-with-files 工作流并运行 session catch-up。
- 已确认 Adobe2API Chat 路径底层支持参考图，但异步 DTO/worker 与 new-api AdobeVideo adaptor 尚未桥接。
- 已锁定 V1 契约：统一 `input.image + reference_images`，provider option 选择 `frame|media`，仅 URL/Data URL，参考图不影响按秒计费。
- 当前开始扩展 Adobe2API 异步 worker 与聚焦测试；保留 new-api 工作树中另一组 Claude 诊断修改。
- Adobe2API 已增加异步参考图片 DTO、提交结构校验、worker 内媒体加载和稳定失败错误；README 中英文已补充异步接口说明。
- new-api AdobeVideo adaptor 已映射主图与参考图，支持 `provider_options.adobe_video.reference_mode=frame|media`，并保留原 4-15 秒计费估算。
- async-test mock 已记录并校验参考模式与图片列表，覆盖成功轮询、失败终态和 Range 内容。
- Adobe2API Docker 完整测试 75/75 通过；new-api `go test ./... -count=1` 通过；两仓库 `git diff --check` 通过。
- Docker dev 已重建 `new-api-dev` 与 `async-test-mock`。4 秒 media 参考图任务成功，计费 60000；失败任务退款 60000；临时数据库夹具和测试额度变化已完整清理。

## 2026-07-30 — Seedance URL-only 多媒体异步链路

- 已读取 brainstorming 与 planning-with-files 工作流并运行 session catch-up。
- 用户确认上传采用 R2 预签名直传，上传会话公开入口放在 new-api 控制面。
- 已合并旧多媒体计划与最新 URL-only 决策：保留 frame/media、9/3/3、15 秒、Adobe/Higgsfield 映射；移除 new-api 任务 multipart/Base64 暂存。
- 正在审计四个服务现有代码与测试，下一步按现有边界拆分最小修改。
- 第一轮代码审计完成：Adobe 已有 media references 主体，Higgsfield 有通用上传骨架但公共接口仅 frame，两者缺口已拆分。
- 已定位 R2 预签名所需现有 S3/Redis 注入点，以及 Adobe ffprobe 和 Higgsfield严格 schema 的具体缺口。
- 审计阶段完成，合并设计已写入 `docs/plans/2026-07-30-seedance-url-media-design.md`；开始实现上传控制面。
- image-handle 已新增内部媒体上传会话创建/完成接口：Redis 保存会话，R2 预签名 PUT，HEAD 校验大小与 MIME，失败对象删除。
- 恢复跨四仓库实施状态并核对工作树：new-api 与 image-handle 保留本任务改动；Adobe2API 只有无关未跟踪设计文档；Higgsfield2API 当前工作树干净。
- 完成 provider 第二轮代码审计：Higgsfield 可复用现有通用上游上传骨架，Adobe 已有多媒体映射测试；下一步补 schema、媒体探测、multipart 和批量上传编排。
- new-api 已完成 HiggsfieldVideo 精确 SKU、注册、provider options 和统一多媒体请求测试；相关 Go 包全部通过。
- Higgsfield2API 已完成严格 schema、JSON/multipart、9/3/3、总 12、全局名称唯一、通用媒体准备、ffprobe 15 秒规则、脱敏任务快照、单 batch/并行 PUT/串行 confirm、frame/media 角色映射和账号重试重传。
- Higgsfield 定向测试 26 项与 Ruff 检查通过；旧 frame、管理端单图上传和完整任务生命周期均保持通过。
- image-handle TypeScript 编译及 3 个媒体上传测试通过；文件 body 不经过该接口。
- new-api 已新增 `/v1/media/uploads` 与 `/v1/media/uploads/complete` 小型 JSON 控制面，使用 Resource API Key 和上传限流，仅转发元数据。
- new-api controller/router/dto 聚焦测试通过，上传控制阶段完成。
- Adobe2API 已完成严格请求、JSON/multipart、9/3/3/12、ffprobe 15 秒探测、稳定错误码和 Docker ffmpeg runtime；Seedance 聚焦测试通过。
- Higgsfield2API 已完成严格请求、通用媒体上传、单 batch/并行 PUT/串行 confirm、frame/media 角色映射、脱敏持久化和账号切换重传；62 项测试与 Ruff 通过。
- Resource Center OpenAPI/React 文档及 supertokendoc Seedance 新异步章节已更新为 URL-only 任务与 R2 预签名直传流程，覆盖 mixed media、Webhook 和按秒计费。
- 收敛七个 locale 的提取器噪声并补齐真实翻译；键级差异为每种语言新增 19、修改 0、删除 0、空新增 0。
- 修正 HiggsfieldVideo provider options 命名空间歧义和媒体上传 session object 名称；new-api 聚焦测试通过，image-handle 全量 91 项通过。
- new-api 公开任务/Webhook 现可保留两个参考素材时长错误码，其他未知 provider 码仍使用 `video_task_failed`；service 聚焦测试通过。
- 构建并启动最终源码的 new-api、async mock、image-handle/MinIO、Adobe2API 和 Higgsfield2API Docker dev 容器；所有健康检查通过，两个 provider 镜像均包含 ffprobe 5.1.9。
- Adobe URL-only media 联动已创建 4 秒成功任务并返回 MP4 Asset；消费日志精确扣费 60000、资金来源为钱包，幂等请求快照仅保存参考素材 SHA-256 标识。
- 首次综合脚本在打印 Adobe 成功任务后退出；不盲目重跑，正在按真实 Mock/数据库 schema 从断点核查 Range、Webhook、Higgsfield 成功与异步失败退款。
- Adobe 上游映射、终态审计和成功 Webhook 已从 Mock 与数据库逐项确认；首次脚本误读 `/control`（配置）而非 `/metrics`（请求指标），产品链路没有因此失败。
- Adobe Asset 通过 Resource API Key 执行 `Range: bytes=0-3` 返回 206、4 字节和 `video/mp4`，成功链路已闭环；准备从断点提交 HiggsfieldVideo。
- HiggsfieldVideo 4 秒 media 任务成功：Mock 收到 `seedance-2.0-480p` 和 1 图/1 视频/1 音频，公开 Asset Range 返回 206；第二次精确扣费 60000，累计 `used_quota=120000`。
- Higgsfield 异步失败任务已验证：预扣后公开终态为 failed，可用钱包全额恢复并记录 60000 退款日志；修正了将累计 `used_quota` 误当净消费的验收断言。
- 已确认两个运行中 provider 的本地服务凭据均可访问 `/v1/models`，开始通过各自 `/v1/videos` multipart 入口执行真实 ffprobe 15.001 秒拒绝检查。
- 已用 Docker ffmpeg 生成并由 Docker ffprobe 确认 15.001000 秒 WAV；Adobe 边界任务已创建，轮询脚本变量冲突已修正并从已有任务续跑。
- Adobe/Higgsfield Docker 15.001 秒边界均以 `reference_media_duration_exceeded` 拒绝；两个成功和一个失败 Webhook 均一次 204 投递。
- 重复查询失败任务未重复退款；三个请求快照无 URL/Base64/Data URL，三个计费快照均固定 wallet-only 策略。开始精确清理一次性 fixture。
- 3 个 MinIO 对象和 3 个 image-handle Redis 会话已删除并验证 404/不存在；首次 PostgreSQL here-doc 因 `docker exec` 缺少 stdin 参数未执行，数据库尚未变化，准备补执行同一精确事务。
- PostgreSQL fixture 第二次事务已完整删除并验证 14 项计数全 0；Mock 指标已清零且配置恢复 completed/3。Adobe 重启后的脚本误探测 `/health` 404，改用 `/v1/models` 完成健康检查。
- Adobe 容器经自身 healthcheck 和鉴权 `/v1/models` 验证健康，内存边界任务已清除；4 个本地媒体 fixture 已精确删除，清理阶段完成。
- new-api 全量 `go test ./... -count=1` 与前端 `bun run build` 通过；i18n status 通过。
- i18n lint 的本次新增 21 项均为 OpenAPI operation/schema 标识，配置为非展示属性后 ResourceCenterDocs 为 0 项，仓库恢复既有 420 项基线。
- supertokendoc VitePress 生产构建通过；五仓库最终 `git diff --check` 全部通过。
- 最终 Docker 健康、Mock 归零、fixture 数据/Option/本地文件清理全部核验通过；Seedance URL-only 多媒体异步链路阶段完成。

## 2026-07-30 — 四服务 main 分支发布

- 已确认四仓库均位于 main，当前 HEAD 与本地记录的 origin/main 一致。
- 已锁定排除项：new-api 的 Claude/请求转储/临时内容、Adobe 的无关设计文档、以及不在四服务范围内的 supertokendoc。
- 四个 origin/main fetch 后均为 `0 ahead / 0 behind`，不存在需要合并的远程提交。
- new-api 七个 locale 同时包含 Claude 诊断键和 13 个媒体文档键；发布将从 HEAD 文本构造仅追加媒体键的 index blob，保留工作树中的 Claude 翻译但不纳入提交。
- 四仓库目标文件已暂存；new-api locale 索引通过 HEAD 文本加 13 个媒体键构造，未改写工作树。
- 四仓库 staged diff 通过 whitespace 检查；new-api staged 列表不含 Claude/RequestDump/规划文件，Adobe 无关设计文档保持未跟踪。
- new-api 将在临时 detached worktree 验证仅 staged 快照，防止未暂存 Claude 改动影响测试可信度。
- staged 快照首次 Go setup 因临时 worktree 没有 ignored `web/dist` 而停止；未进入业务测试，改为先 build 前端再测试后端。
- 仅 staged 快照的 Vite build 与 `go test ./... -count=1` 全量通过；临时 detached worktree 已全部清理。
- 已创建四个 main 提交：new-api `97160e952`、image-handle `93147e2`、Adobe2API `6a58df9`、Higgsfield2API `01ad0a0`。

## 2026-07-30 — Adobe Fast 480p 真实多媒体联调

- 已按 planning-with-files 恢复现有规划状态并建立本次独立联调阶段。
- 已读取 supertokendoc 的异步、参考素材和 Webhook章节，完成六个媒体素材的预签名上传与完成确认。
- 已验证本地 URL 主机改写 workaround，准备提交 4 秒 frame 双图任务。
- 首次 frame 创建在上游鉴权阶段失败，没有任务和扣费；已确认是渠道 124 Key 与 Adobe2API 当前配置不一致，不是 frame 请求语义错误。
- 已把本地渠道 124 的服务 Key 对齐到 Adobe2API 实际配置并重启 new-api；第二次 frame 创建返回 202。
- frame 真实任务进入上游后被 Adobe 拒绝，错误为 `Unauthorized to perform request.`；2000000 额度预扣和失败退款均正确。
- media 的 3 图、2 视频、1 音频任务返回 202 并进入异步执行，最终被同一 Adobe 授权错误拒绝；请求脱敏、4 秒计费和退款均正确。
- 通过 Adobe 管理接口完成一次账号刷新；刷新后只重试一次 frame，错误不变，因此停止继续提交。
- 三个失败任务的钱包均已全额恢复，消费/退款日志一一对应；因没有生成 Asset，无法执行结果下载和实际输出时长检查。

## 2026-07-30 — Adobe 新账号 10000 积分复测

- 已确认新账号 active 且可用积分 10000；开始执行渠道、模型和素材预检。
- 渠道、模型、六个素材预检全部通过。
- 4 秒 frame 双图任务成功；Range、完整下载、ffprobe 和 2000000 钱包扣费验证通过，开始 media 任务。
- 4 秒 media 三图、双视频、单音频任务成功；Range、完整下载、视频/音频流和 2000000 钱包扣费验证通过。
- 两个任务总扣费 4000000、无退款，快照素材计数与脱敏均正确；Docker 服务最终健康。
# Adobe Multi-model and Console Upgrade Progress (2026-07-30)

- Loaded the required brainstorming, file-planning, and UI/UX workflows.
- Recovered existing planning state and inspected both Git worktrees without modifying or discarding user changes.
- Confirmed the implementation phases and started the backend/API contract audit.
- Located the current Adobe model catalog, video submit/poll/download branches, admin routes, and Higgsfield console reference implementation.
- Traced new-api normalized Adobe validation, task parsing, direct public Asset projection, legacy content resolver, and channel model-fetch controls.
- Added Adobe stable Kling/Veo capability definitions, normalized validation, provider payload modes, direct result URL validation, SQLite video-task/API-key stores, and restart recovery.
- Adobe Docker-mounted focused suite passes all 31 capability and Seedance tests, including no result-media download and legacy Range content compatibility.
- Completed Adobe2API SQLite task/API-key/request-log persistence, restart recovery, React/Vite operations console, direct result URLs, and multi-stage Docker runtime.
- Completed new-api eight-SKU capability validation, pre-billing rejection, direct Asset projection, legacy internal-reference fallback, and channel model-discovery strategies.
- Rebuilt both Docker dev services; `new-api-dev` and `adobe2api` are healthy and share the `ai-gateway` network.
- Adobe2API full suite passes 99/99 and new-api `go test ./... -count=1` passes after the final controller and Kling duration changes.
- Real Kling 3.0 `3s frame` reached Adobe and persisted its upstream ID, then failed on a downstream 408; wallet quota was fully refunded and no local result file was added.
- Real Kling Omni `3s images` reached Adobe but exposed an incorrect `usage=asset` mapping. Adobe requires `style`; corrected the payload and regression test before one controlled retry.
- Rebuilt Adobe2API after the Omni role correction; focused capability tests pass and the live payload reports three `usage=style` references.
- The corrected Omni task reached upstream execution but failed on the same Adobe downstream 408; its 1,500,000 quota was refunded and no local media file was created.
- Real Veo Standard `8s/16:9 images` succeeded as `task_W8CwZxE98qwt4M1FD9EfNmnAHx6ofveT` and returned direct Asset `asset_XOHH7SjE1G3MR4LQQBBFQ6OALscHFDMv`.
- Real Veo Fast `4s frame` succeeded as `task_2ki2AEKok1CBH4QCKpCVrwG9UcTUo9zX` and returned direct Asset `asset_EKCtKGF9iDc19sG7HHatGtGJbbl9Nb9t`.
- Verified signed URL persistence without log leakage, zero new generated files, correct temporary/no-auth public projection, and exact 6,000,000 net wallet charge for 12 successful seconds.
- Recovered the final verification state and confirmed the task detail already includes direct Adobe `<video>` preview plus distinct open/download actions; no implementation patch is needed before desktop browser verification.
- Updated the verification scope per the user's instruction: mobile compatibility and mobile browser QA are excluded; desktop console QA remains in progress.
- Live desktop task-list QA passed against `http://127.0.0.1:16000/#/tasks`: successful Veo rows use Adobe CDN links and failed Kling rows correctly have no result URL.
- The first detail click exposed a stale narrow viewport override from earlier mobile QA; logged it and switched the existing tab back to desktop before retrying.
- Browser capability inspection confirmed the existing tab supports the CDP capability needed to restore desktop metrics; no page-state workaround or alternate browser is required.
- Restored 1440x1000 desktop metrics and opened the live Veo Fast success detail. The dialog renders its persisted metadata and separate open/download actions against the same Adobe CDN result.
- Verified the live media element without fetching through either local service: Adobe URL equality, signed HTTPS host, no local content path, loaded video state, and zero video error all passed.
- Desktop geometry check passed with no horizontal overflow and an in-viewport 980px dialog; the loaded result video has a stable 942x530 box and uses the modal's vertical content flow.
- Final regression started in parallel. The host Adobe test invocation stopped on missing FastAPI/Pydantic, so the authoritative Adobe suite will run in Docker; other build/test sessions remain active.
- new-api full Go tests passed, both frontend production builds passed, and i18n lint reproduced the existing 420-item baseline with no new finding from this task. Adobe Docker-runtime tests remain.
- A follow-up Adobe test command accidentally omitted `docker exec` and repeated the known host dependency failure. The corrected next invocation is explicitly container-scoped.
- The production container correctly omits test sources. Switching to a disposable container that reuses its runtime dependencies while mounting the repository read-only and test data in tmpfs.
- Adobe2API full Docker-runtime suite passed: 100 tests, OK. Final image rebuild and health verification remain.
- Rebuilt and recreated only `adobe2api`; the new container is healthy. Post-rebuild browser reload preserved the task history and direct URL projection, and the task detail still exposes identical preview/open/download links without local proxying.
- Post-rebuild Adobe video metadata loaded successfully in the browser (`readyState=4`, 4.01s, no error).
- Direct capability fetch from the browser evaluation sandbox is unsupported; switching to the Generate Test page, which is the real catalog consumer, for the final model/control audit.
- Post-rebuild Generate Test UI exposes all eight target SKUs and correctly applies the catalog-driven Kling Omni, Veo Standard images, and Veo Fast constraints without submitting a task.
- Kling Omni `images` explicitly reports a three-image maximum. Both worktrees pass whitespace checks, both Docker services are healthy on the shared network, and the Adobe generated-file count remains unchanged at 33.
- Phase 5 verification is complete. Mobile compatibility was intentionally excluded by the user's final scope instruction.
- The planning helper's file-wide check sees two unrelated historical pending phases around the image-pricing Docker review. This Adobe plan itself has no pending item; unrelated task state remains untouched.
- Started Phase 6 after a new new-api-linked retry exposed Kling 3.0's missing one-frame minimum. The zero-image task failed at Adobe submit; the Omni images task continues in progress.
- Located the shared gap: Adobe2API and new-api capability tables encode maximum reference counts only. Started focused source/test inspection while the existing Omni task polls in the background.
- Added the generic minimum-image capability and Kling frame=1 in both services plus the Adobe console. Adobe focused tests/build pass; one new-api fixture needs a valid required image before it can continue testing its intended video-limit branch.
- Corrected the dependent test fixture; new-api Adobe adaptor tests now pass. Billing audit confirms the invalid frame task refunded and the successful Omni task retained exactly one 3-second wallet charge.
- Completed the final new-api Docker rebuild and recreation. The resulting image is healthy alongside the rebuilt Adobe2API container; live precharge and one-image retry checks are next.
- The first read-only PostgreSQL baseline command failed before execution because nested shell quoting stripped SQL literals. Switched the test harness to a direct `psql -c` invocation; no database state changed.
- The direct baseline established admin quota `62,302,740`; its token projection then stopped on obsolete `models_enabled`. No writes occurred, and the next query will use the live schema.
- Confirmed the current token-limit column names. A separate task-count projection then found that `tasks` has no direct `model` column; this was another read-only harness mismatch and will be resolved from the live task/log schemas.
- Identified the intended local token without exposing its key. The JSON task-count query needs an explicit PostgreSQL text cast; no state-changing command has run.
- Final zero-reference live acceptance passed: new-api returned `reference_image_required` before task creation, precharge, billing log creation, or Adobe2API submission. Proceeding with one valid frame image.
- Submitted one valid Picsum frame through new-api. The Kling 3.0 task completed successfully and returned one temporary no-auth direct result; billing, Asset equality, and local-file checks remain.
- Billing and direct-resource acceptance passed: exactly 1,500,000 quota was retained, no success refund was added, task and Asset URLs are identical Adobe CDN links, and no local media file was created.
- Full regression passed: `go test ./... -count=1` and Adobe2API's 101-test suite are green.
- Phase 6 complete. Both Docker services are healthy, both worktrees pass `git diff --check`, and the successful Kling 3.0 frame and Kling Omni images tasks each retain the expected three-second charge.
