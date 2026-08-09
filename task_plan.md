# Leonardo Reference Media Normalization Error Projection (2026-08-08)

## Goal

Keep Leonardo2API's trusted normalization failure code visible through new-api submission and public
task projections without exposing arbitrary upstream errors.

## Status

- [x] Submission validation whitelist already includes `reference_media_normalization_failed`.
- [x] Add the code to public task projection and regression coverage.
- [x] Run focused relay/service tests.

## Errors Encountered

| Error | Attempt | Resolution |
| --- | --- | --- |
| Combined seven-locale patch assumed one stale French translation string and was rejected atomically | 1 | Re-read exact locale values and apply additions using stable key-only anchors per locale. |
| Frontend preview tests compared JavaScript floating-point products with exact decimal literals | 1 | Keep backend decimal billing unchanged and use `toBeCloseTo` for frontend-only preview/display arithmetic. |
| One Docker inspect template assumed every service defines a healthcheck | 1 | Check application/mock health separately and use running state for PostgreSQL/Redis; no container state changed. |
| Combined patch used a stale planning-file title | 1 | Patch validation rejected all changes; split Go source and planning edits. |

---

# Task Plan: Reference-video Per-second Surcharge (2026-08-10)

## Goal
Add one optional per-second surcharge to VideoPricing when and only when a validated asynchronous video request contains at least one `input.reference_videos` item, then verify billing, logs, marketplace descriptions, and comparable upstream pricing.

## Current Phase
Complete

- [x] Phase 1: Audit request normalization, pricing snapshots, logs, public pricing, and Docker test fixtures.
- [x] Phase 2: Add backend configuration, one-time reference-video detection, immutable billing snapshot fields, and focused tests.
- [x] Phase 3: Add the admin setting, preview, log display, marketplace description, translations, and frontend tests.
- [x] Phase 4: Run focused/full Go and frontend verification.
- [x] Phase 5: Rebuild local Docker dev and verify real request billing, persisted logs, and configured-model marketplace text with exact cleanup.
- [x] Phase 6: Research current Jimeng official and LibTV pricing, separating confirmed facts from unavailable information.

## Locked Decisions
- Only `input.reference_videos` triggers the surcharge; `input.video`, images, and audio do not.
- Any non-empty reference-video array adds the surcharge exactly once, regardless of item count.
- Formula: `(base unit price + reference-video surcharge) * output seconds * group ratio`.
- Existing configurations remain valid and default the new surcharge to zero.
- The task billing snapshot and usage log must preserve whether the surcharge applied and the effective unit price.
- Marketplace card, table, and detail views show `(base + reference surcharge) = effective per-second price` only when the configured surcharge is greater than zero; zero or absent surcharge preserves the original display.
- Mobile browser QA is out of scope per the user's follow-up; verify desktop rendering only.

## Errors Encountered

| Error | Attempt | Resolution |
| --- | --- | --- |
| Docker health wait used zsh's read-only `status` variable after the container was recreated | 1 | The application container had already started; rerun the read-only health check with a task-specific variable name. |
| Initial PostgreSQL read-only probe used a nonexistent `newapi` role | 1 | Compose declares the local development role as `root`; rerun schema and fixture inspection with that exact role. |
| Channel inventory probe guessed a `group_` column that does not exist | 1 | Query PostgreSQL's actual reserved column as `"group"`; no data was changed. |
| First live-submit shell was rejected before execution because it planned `rm -f` cleanup for a temporary header file | 1 | Avoid temporary files entirely and parse each response body/status in memory. |
| Existing Leonardo mock requests returned 403 because group `vip` is absent from both current group options | 1 | No tasks or charges were created; temporarily add only the previously absent `vip` ratio/display keys, then remove those exact keys during cleanup. |
| Desktop pricing-formula patch failed the first scoped Prettier check in the table component | 1 | Helper tests and ESLint already pass; run the repository formatter on that file and recheck the scoped set. |
| Frontend route lookup used an unmatched zsh `router*` glob | 1 | No files were touched; enumerate actual route/entry files with `rg --files` before searching them. |
| The existing browser session referenced a deleted ordinary-user row and could not open administrator pages | 1 | Register one disposable local root user, complete desktop settings/log QA, log out, and hard-delete that exact user. |
| The first PostgreSQL cleanup probe used the nonexistent container name `new-api-postgres-dev` | 1 | Read the live container list and use the actual `postgres-dev` container; the failed probe changed no state. |
| The zero-surcharge regression patch used one stale expected test line and was rejected atomically | 1 | Re-read the focused test and apply a smaller exact patch; no partial edit was produced. |

---

# Task Plan: Multi-provider Async Video Resource API (2026-07-23)

# Task Plan: Video Error Diagnostic and Public Projection (2026-08-05)

## Goal
Preserve every upstream video-task error for administrators while exposing the
same safely redacted, provider-neutral error through public task queries and
failure Webhooks without requiring an exhaustive provider error-code list.

## Current Phase
Complete

### Phase 1: Contract and current-path audit
- [x] Trace adaptor parsing, redacted response persistence, administrator DTOs, public projection, and Webhook projection.
- [x] Lock the internal/public error envelope and sanitization fallback behavior.
**Status:** complete

### Phase 2: Regression tests
- [x] Cover unknown safe messages, sensitive messages, account-credit errors, malformed/oversized diagnostics, and known business errors.
- [x] Prove administrator visibility and public Task/Webhook parity.
**Status:** complete

### Phase 3: Backend implementation
- [x] Preserve provider error messages and codes through polling without exposing raw response bodies publicly.
- [x] Add centralized sensitive-data transformation and administrator diagnostic fields.
- [x] Reuse the public projection for failure Webhooks.
**Status:** complete

### Phase 4: Frontend implementation
- [x] Display administrator diagnostics in task details with bounded, non-overlapping content.
- [x] Keep ordinary-user task surfaces limited to the public projection.
**Status:** complete

### Phase 5: Verification
- [x] Run focused Go tests, complete Go tests, frontend checks/build, and Git hygiene checks.
- [x] Review changed files and preserve unrelated untracked content.
**Status:** complete

## Locked Decisions
- The provider message is retained after recursive secret redaction; adapters do not replace unknown failures with a generic string.
- Unknown safe text may be returned publicly after central sanitization; unsafe or unusable text falls back to a provider-neutral message.
- Account IDs, provider balances, credentials, signed URLs, internal channel/provider names, and structured upstream bodies are administrator-only or removed.
- Public task queries and outbound failure Webhooks use the same error builder.
- Administrator diagnostics are bounded and sanitized; no endpoint exposes an unredacted full upstream response.
- No database migration is introduced unless discovery proves existing persisted fields cannot support the contract.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Session catchup reported an unrelated interrupted probe and no synchronized plan updates | 1 | Verified the tracked diff is empty and started a separate dated task section without touching unrelated untracked files. |
| Initial combined planning patch expected stale findings/progress headings | 1 | The patch changed nothing; inspected exact headings and applied file-specific patches. |
| Focused red test failed because `BuildAdminVideoTaskDiagnostic` and `taskToDto` do not exist | 1 | Expected regression-test failure; proceed with the planned backend implementation. |
| Final review found public projection ignored a generic nested diagnostic message when `FailReason` was already generic | 1 | Use the centralized diagnostic message as the public source and add a regression test for nested provider shapes. |

---
- [x] 增加 canvas_config 独立菜单权限和管理员配置接口
- [x] 增加 CanvasGrant 与一次性授权码模型


---

# Task Plan: Infinite Canvas Authorization

## Goal

为 Infinite Canvas 提供管理员可配置、用户独立登录授权、PKCE 兑换并自动生成两枚模型 Token 和一枚 Resource Key 的完整授权链路。

## Current Phase

Complete

### Phase 1: 配置、权限与模型
- [x] 增加 canvas_config 独立菜单权限和管理员配置接口
- [x] 增加 CanvasGrant 与一次性授权码模型
- **Status:** complete

### Phase 2: 授权服务与 API
- [x] 完成用户分组/模型校验、PKCE、事务性凭证创建和重复授权修复
- [x] 完成登录 return_to 和结构化错误
- **Status:** complete

### Phase 3: 管理与授权 UI
- [x] 完成 Canvas 配置页和 GitHub OAuth 风格授权确认页
- **Status:** complete

### Phase 4: Verification
- [x] 完成后端/前端测试、Docker Dev 联调和真实请求验收
- **Status:** complete

## Locked Decisions

- client_id 固定为 infinite-canvas，回调 URI 必须精确命中管理员白名单。
- 图片 Token 为 canvas-images，视频 Token 为 canvas-videos；长期、无限额度、模型白名单开启。
- Resource Key 复用当前有效 Key，无有效 Key时轮换为长期 canvas-resources。
- 凭证仅在授权码兑换事务中生成，缺任一分组权限不产生部分凭证。
- 一期不增加授权用户列表或撤销管理。

## Errors Encountered

| Error | Attempt | Resolution |
|-------|---------|------------|
| 首次动态追加规划文件的补丁格式无效 | 1 | 修正新增行前缀后重试，失败补丁未产生修改 |
| 启动方式检索命令包含未命中的 zsh `compose*.yml` glob | 1 | 前序只读检查已完成；后续直接使用已确认存在的 `docker-compose-dev.yml` |
| 密码登录实测仍跳转 `/console`，首次提前捕获目标后仍被覆盖 | 2 | `AuthRedirect` 的后续 `<Navigate>` 仍会竞争；return-to 改为登录期间幂等保留，授权页确认抵达后再清除 |
| 登录成功后授权 context 仍返回 401，跳转期间触发空 `context` 渲染 | 1 | new-api `UserAuth` 同时要求 Session 与 `New-Api-User`；授权页两次请求补用户头，并在 401 导航期间保持 loading |
| 浏览器截图返回重复拼接的视口画面 | 1 | 使用首个真实视口、DOM 快照和控件布局共同验收，不把采集层拼接误判为页面溢出 |
| abilities 诊断 SQL 误用了不存在的 `group_name` 列 | 1 | 读取列定义后改用实际的 `group` 列，后续查询成功 |
| 清理前模型元数据查询误用了不存在的 `name`/`enabled` 列 | 1 | 读取 `models` schema 后改用 `model_name`/`status` 精确核对，失败查询未修改数据 |


## Goal
Add a provider-neutral asynchronous video create/query contract, per-asset temporary downloads, and video terminal Webhooks while preserving all existing OpenAI/CLIProxyAPI/sub2api compatibility endpoints.

## Current Phase
Complete

### Phase 1: Public contract and persistence
- [x] Add normalized video create/query/result DTOs with explicit optional scalar pointers.
- [x] Persist normalized requests, idempotency, asset type, and operation cross-database.
- [x] Add create/list/get/batch routes with the approved token and `ak_` boundaries.
**Status:** complete

### Phase 2: Provider and asset pipeline
- [x] Add structured video outputs with legacy single-URL fallback.
- [x] Implement xAI normalized generation/edit/extension conversion and validation.
- [x] Add provider-neutral public URL projection and per-asset content proxying.
**Status:** complete

### Phase 3: Webhook and documentation
- [x] Emit `video.task.succeeded` and `video.task.failed` through the existing account Webhook.
- [x] Add the Async Videos OpenAPI and Resource Center documentation, examples, filters, and all locales.
**Status:** complete

### Phase 4: Verification
- [x] Run focused, cross-database, full backend, OpenAPI, frontend, and i18n checks.
- [x] Rebuild Docker dev and verify real Grok generation, query, Asset/legacy Range downloads, idempotent replay, and success/failure Webhooks.
- [x] Exercise normalized edit through the real channel and document that the current sub2api upstream rejects both data URL and provider-file inputs before a successful edit can run.
- [x] Restore the temporary Webhook target, reset the mock, and confirm final container health.
**Status:** complete

### Phase 5: Image-input capability correction
- [x] Keep `image` as one primary image, `reference_images` as an array, and `video` as one source in the public DTO.
- [x] Remove provider-specific image-combination and edit-output restrictions from the public controller.
- [x] Enforce current xAI generation/edit/extension combinations, reference-image count/model/duration limits, and explicit errors in the xAI adaptor.
- [x] Correct OpenAPI, Resource Center examples, and all affected locales; verify backend, spec, Docker dev, and desktop frontend.
**Status:** complete

## Locked Decisions
- Existing `/v1/videos/*` wire formats remain compatible; normalized clients use `/v1/video/tasks`.
- Video create/edit/extension/remix POST routes require ordinary API Tokens; `ak_` is read/download only.
- Public video tasks and Webhooks are provider-neutral and may contain multiple video Assets.
- Private upstream URLs stay internal; public cross-origin HTTPS URLs may bypass new-api, while relative/same-origin/private sources use `/v1/assets/{asset_id}/content`.
- Images and videos share one account Webhook URL and `wk-` Key.
- Video resources remain upstream-backed and temporary; no object-storage archive or retention guarantee is added.
- xAI 1080p is accepted only for `grok-imagine-video-1.5` single-image generation; text and reference-image generation are rejected by the normalized adaptor.
- Public `image` is a singular primary image and `reference_images` is a multi-image array; adaptors, not the public controller, decide which operation/input combinations they support.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Combined Adobe route patch matched the model-list block after later route context and failed atomically | 1 | Split catalog, imports/model list, validation, and worker edits into exact smaller patches. |
| Host Python cannot import FastAPI for Adobe2API tests | 1 | Keep the host environment unchanged and run the repository tests in the existing Adobe2API image with the source mounted read-only and a writable data tmpfs. |
| `VideoJobStore.list` shadowed the built-in `list` in a later runtime annotation | 1 | Quote the later `interrupted` return annotation so Python 3.11 does not resolve the method object as a generic. |
| The first multi-locale patch assumed the English translation of the final `格式` key was `Format` | 1 | The atomic patch changed no locale files; inspect each exact file tail and apply locale patches with their real existing values. |
| `relay_task.go` referenced `ratio_setting` without importing it, and the first snapshot patch matched the image task constructor | 1 | Add the missing import and explicitly add `VideoPricing` to the normalized video task billing context. |
| Seedance completion exposes token usage before the legacy per-call guard | 1 | Add an earlier immutable VideoPricing terminal branch so token usage cannot overwrite request-duration billing; merge returned duration as audit-only metadata. |
| The new terminal billing branch referenced the pricing-mode constant without importing `types` | 1 | Add the missing package import and rerun the scoped compile. |
| The subscription-eligible test used the old minimal subscription fixture, which lacks a plan and preconsume-record table | 1 | Migrate the two required test models, create a valid plan association, and provide the idempotent request ID required by subscription preconsume. |
| Combined Asset DTO/controller patch referenced a controller symbol while applying to the DTO file | 1 | Split the patch by real file context and applied the DTO and controller changes with exact anchors. |
| xAI normalized-model test initialized a promoted RelayInfo field directly | 1 | Put `UpstreamModelName` in the embedded `ChannelMeta`, matching production construction. |
| Scoped Prettier found one mechanical formatting difference in `WebhookTab.jsx` | 1 | Run Prettier on that file and repeat the complete scoped check successfully. |
| Full i18n lint reports the repository's existing 420-item hardcoded-string baseline | 1 | Confirm none of the changed Resource Center/Webhook files appears in the findings; all seven locale catalogs parse and i18n status completes. |
| Real channel 109 rejects normalized video edit for both a downloaded data URL and the generated provider content UUID | 2 | Preserve the provider-neutral edit implementation and record the upstream limitation; current sub2api source exposes generation/status routes but no usable edit route. |
| Combined planning patch referenced a findings heading that does not exist verbatim | 1 | The atomic patch changed nothing; patch the active task section and append findings/progress separately. |
| Desktop acceptance initially showed the previous 1.5 reference-image example | 1 | Source and local build were correct; rebuild and recreate only `new-api-dev`, then repeat the desktop check against the embedded frontend. |
| PostgreSQL cleanup probe used the nonexistent default `postgres` role | 1 | Read the dev Compose configuration and use its configured `root` role for the exact disposable-user cleanup. |
| First Docker capability probes used the token display name as the Bearer value | 1 | Resolve the active Key for the named local token without printing it, add the required `sk-` prefix, and rerun both probes successfully. |

---

# Task Plan: Normalized Video Submit/Poll Race (2026-08-04)

## Goal
Prevent normalized `/v1/video/tasks` rows from being polled with their public
`task_...` ID before the provider task ID has been persisted, while preserving
legacy task-ID fallback, idempotent billing, and terminal error projection.

## Current Phase
Complete

### Phase 1: Contract and race audit
- [x] Reconstruct the remote Adobe failure from persisted timestamps and IDs.
- [x] Trace durable task creation, provider submission, ID persistence, polling, and refund ownership.
**Status:** complete

### Phase 2: Regression tests
- [x] Prove normalized video tasks without an upstream ID are not pollable.
- [x] Prove legacy tasks retain their historical public-ID fallback.
- [x] Prove normalized video 404 responses receive a bounded grace period.
**Status:** complete

### Phase 3: Implementation
- [x] Mark durable normalized video rows as locally submitting until provider acceptance.
- [x] Gate polling on a persisted upstream ID and generalize the new-task 404 grace.
- [x] Preserve explicit submit failure CAS/refund and existing public status compatibility.
**Status:** complete

### Phase 4: Verification
- [x] Run focused relay/service/model tests and race repetitions.
- [x] Run the complete Go suite and formatting/whitespace checks.
**Status:** complete

## Locked Decisions
- This is a new-api fix; Adobe2API, Leonardo2API, and Higgsfield2API contracts do not change.
- Normalized public video tasks must never use their public `task_...` ID for provider polling.
- Historical non-normalized tasks keep `TaskID` fallback compatibility.
- Missing provider IDs are retained until the existing task timeout owns terminalization; they are not resubmitted.
- A valid provider ID is persisted before any provider status request is allowed.
- Preserve all unrelated tracked and untracked workspace changes.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Focused regression tests failed before implementation because normalized tasks fell back to `task_...` and lacked generic 404 grace | 1 | Expected red phase; implement provider-ID readiness gating and shared video 404 grace. |
| Initial status patch matched the earlier async-image initializer | 1 | Restore the image path immediately and apply the status change only inside `createDurableVideoTask`. |
| Relay status assertion compared an untyped constant with `model.TaskStatus` | 1 | Cast the expected constant to `model.TaskStatus`; runtime behavior was already correct. |

---

# Task Plan: Async Task Error Boundaries (2026-08-03)

## Goal
Keep transient Adobe2API polling failures non-terminal, recognize the public
`submitting` state, preserve original upstream diagnostics for administrators,
and expose only structured provider-neutral errors through task queries and
Webhooks.

## Current Phase
Complete

### Phase 1: Contract audit
- [x] Trace video polling, persistence, public task projection, and Webhook creation.
- [x] Lock compatibility constraints for existing providers and historical task rows.
**Status:** complete

### Phase 2: Regression tests
- [x] Cover `submitting`, 408/429/5xx polling, provider-neutral public errors, and matching Webhook payloads.
- [x] Prove transient responses do not refund or create terminal Webhooks.
**Status:** complete

### Phase 3: Implementation
- [x] Add the missing AdobeVideo status mapping and HTTP-aware polling behavior.
- [x] Preserve administrator diagnostics privately while projecting structured public errors.
**Status:** complete

### Phase 4: Verification
- [x] Run focused and full Go regression suites.
- [x] Rebuild local Docker and verify task query plus Webhook behavior.
**Status:** complete

### Phase 5: Timeout terminal Webhooks
- [x] Create timeout failure events and deliveries in the same transaction as the winning status CAS.
- [x] Prove timeout scans do not duplicate Webhooks or refunds and cannot overwrite a concurrent terminal state.
- [x] Run full regression and rebuild local Docker.
**Status:** complete

## Locked Decisions
- Adobe2API continues to own retries for Adobe-originated failures; new-api owns failures while polling Adobe2API.
- A confirmed upstream task must never be resubmitted by this change.
- Public task responses and Webhooks must use the same provider-neutral error object.
- Internal diagnostics retain the original upstream error body for administrators, while credential-bearing fields and oversized binary/Base64 payloads remain protected. No raw body is copied to public payloads.
- Existing synchronous and non-Adobe task adaptors must preserve their behavior.
- Timeout Webhook delivery remains asynchronous; the timeout scan never performs receiver network I/O.
- Legacy tasks without a normalized public request keep their existing timeout behavior and do not gain an invalid public event.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Host Python lacked pytest/FastAPI during the preceding diagnosis | 2 | Reused the Adobe2API runtime image for the authoritative focused tests. |
| First planning-file patch used stale findings context | 1 | Located the exact current lines and applied a narrower patch; no implementation file was changed. |
| New regression suite failed before implementation | 1 | Expected red phase: AdobeVideo returned empty states and the public DTO lacked structured diagnostic fields. Proceed to the minimal implementation. |
| Focused build passed `OpenAIError.Code` (`any`) to a string helper | 1 | Convert the existing polymorphic code to its string representation before transient-code classification. |
| First behavior run masked the legacy safe reason `expired` and lost historical submission translations | 1 | Preserve short provider-neutral legacy reasons and the existing explicit submission translations; keep raw provider/URL/JSON detail internal. |
| Final review found a non-JSON HTTP 200 body could leave an empty parsed status | 1 | Apply the same non-terminal retention fallback after all empty-status parsing paths and add a plain-text regression. |
| Timeout Webhook regression initially failed to compile because the transaction helper did not exist | 1 | Expected red phase; add `commitTimedOutTaskTransition` and rerun the same behavioral tests successfully. |

---

# Task Plan: Adobe2API Seedance 2.0 Fast Integration (2026-07-29)

## Goal
Implement a real Adobe2API asynchronous Seedance video contract, connect it to new-api through a dedicated AdobeVideo task adaptor, and verify one Docker-dev 4-second Fast generation plus per-second wallet billing.

## Current Phase
Complete

### Phase 1: Adobe2API contract discovery
- [x] Confirm the advertised Seedance 2.0 model families and the legality of a 4-second Fast request.
- [x] Verify exact public and upstream model identifiers, payload conversion, duration/resolution bounds, response format, and task-state behavior in code and tests.
- [x] Inspect local Docker topology and credential readiness without printing or mutating secrets.
**Status:** complete

### Phase 2: New-api integration design
- [x] Compare Adobe2API's protocol with the existing Seedance/Doubao and normalized video-task adaptors.
- [x] Evaluate direct Chat Completions proxying, Sora-protocol reuse, and a dedicated AdobeVideo adaptor.
- [x] Select the smallest approach that preserves asynchronous lifecycle semantics, provider-specific fields, and exact-model per-second billing.
**Status:** complete

### Phase 3: Acceptance and cleanup plan
- [x] Define mock-first protocol tests and one separately approved real 4-second paid generation.
- [x] Define task submission, polling, terminal result, asset access, quota, funding-source, idempotency, and refund assertions.
- [x] Define exact Docker rebuild scope, disposable fixtures, secret handling, and cleanup verification.
**Status:** complete

### Phase 4: Adobe2API asynchronous video implementation
- [x] Confirm Seedance 451 policy-error mapping passes with a writable isolated test data directory.
- [x] Add stable resolution-bound Seedance provider SKUs and explicit duration validation.
- [x] Add asynchronous submit/query/content endpoints backed by the existing scheduler, retry, progress, and generated-file services.
- [x] Add route, lifecycle, validation, error, and content tests.
**Status:** complete

### Phase 5: New-api AdobeVideo integration
- [x] Add the AdobeVideo channel type, frontend option, async-task label, and adaptor registration.
- [x] Implement normalized request preparation, exact mapped-model forwarding, per-second estimation, status parsing, and content resolution.
- [x] Add focused adaptor, relay billing, wallet-only, idempotency, and failure-refund tests.
**Status:** complete

### Phase 6: Docker-dev mock acceptance
- [x] Rebuild Adobe2API and new-api Docker dev without restarting PostgreSQL or Redis unnecessarily.
- [x] Configure disposable channel, model mapping, pricing profile, test user/token, and mock lifecycle.
- [x] Verify 202 submission, polling states, content proxy, quota formula, wallet-only behavior, idempotency, and failure refund.
**Status:** complete

### Phase 7: One approved real 4-second call and cleanup
- [x] Submit one text-only 4-second 16:9 480p Fast generation through new-api.
- [x] Verify terminal task, MP4 access/duration, immutable billing snapshot, quota delta, and no subscription consumption.
- [x] Remove disposable database/config fixtures, retain the generated acceptance MP4 for inspection, and confirm both containers remain healthy.
**Status:** complete

## Decisions
- This investigation is read-only for business code and upstream state.
- Do not submit a real generation until the user approves the final plan and identifies an account/credential that may incur cost.
- Treat `seedance_2.0_fast` and `seedance_2.0` as distinct upstream variants; expose resolution through exact public aliases rather than a billing multiplier.
- Use requested duration as the immutable billing quantity; provider-returned duration is audit-only.
- Add a real asynchronous video submit/query/content contract to Adobe2API; do not expose its current blocking `/v1/chat/completions` call as an asynchronous new-api task.
- Add a dedicated AdobeVideo task adaptor in new-api. Reusing Sora would save one adaptor but would either lose Seedance aspect-ratio/audio/reference semantics or add Adobe-only fields to the shared Sora contract.
- Use a public alias such as `seedance-2.0-fast-480p` and exact channel mapping to a stable Adobe2API provider SKU. The provider SKU owns the resolution; `output.duration` remains the explicit integer billing input.
- Scope the first real call to text-to-video, 4 seconds, 16:9, 480p, Fast, and no reference media. Add reference-media support only after the basic async/billing path passes.
- Reuse Adobe2API's existing account scheduler, retry policy, generated-file storage, and internal Adobe submit/poll client. For local acceptance the job store may remain in-memory; production rollout must define restart recovery for nonterminal jobs.
- The user approved one real 4-second paid acceptance call on the currently active local Adobe account.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| The planning skill documents `scripts/session-catchup.py`, but the installed skill package contains no scripts directory | 1 | Read the existing planning files directly, inspect Git state, and append a separate task section without repeating the missing command. |
| Host Python cannot import FastAPI for the Seedance test module | 1 | Do not mutate the host environment; run the test source read-only in an ephemeral container built from Adobe2API's existing image. |
| The running Adobe2API image intentionally excludes `tests/` | 1 | Start an ephemeral network-disabled container with the repository mounted read-only so the image dependencies execute the current test source. |
| A read-only source mount made the 451 route test return 500 while writing `data/request_errors.jsonl` | 1 | Mount a separate writable tmpfs at `/src/data`; all 13 Seedance tests pass and no production error-mapping fix is needed. |
| The execution policy rejected removal of the temporary `/tmp/adobe-models.json` model-list cache | 1 | Leave the non-secret temporary file untouched and complete repository consistency checks without retrying a rejected destructive command. |
| New async route tests initially used a test-only catalog containing only the old short alias | 1 | Add the new stable 480p provider SKU to the isolated router catalog and rerun the lifecycle tests. |
| `compileall` attempted to create `__pycache__` inside the read-only source mount | 1 | Set `PYTHONPYCACHEPREFIX=/tmp/pycache` for the isolated compile check rather than making the source mount writable. |
| The first AdobeVideo HTTP integration test panicked because the package-level default service HTTP client is nil before application initialization | 1 | Add a local `http.DefaultClient` fallback only when `GetHttpClientWithProxy` returns nil; configured proxy clients remain unchanged. |
| The first async mock compile left `snapshot()` with its old direct return before the new last-video-request copy | 1 | Construct a local response value, attach the optional copied request, and return it once. |
| The first Docker success-task query reused the submission `sk-` token and received 401 | 1 | Preserve the established credential boundary and add a disposable `ak_` Resource API Key for task queries and asset downloads. |
| The real-call preflight initially used Adobe's upstream Seedance key as the local service credential and received 401 | 1 | Inspect Adobe2API's `require_service_api_key` path: no `config/config.json` is mounted, so the service uses `config_mgr`'s default `projectx_webapp`; verify that credential against `/v1/models` before creating the disposable real channel. |
| The real Adobe2API content endpoint ignored a valid byte range and returned the full MP4 with HTTP 200 | 1 | Add explicit single-range, suffix-range, `If-Range`, and 416 handling to the authenticated content route; all 72 Adobe2API tests pass, including the new 206/416 contract. |
| The host has no `ffprobe` binary | 1 | Validate the retained MP4 with macOS metadata: H.264, 864x496, 4.042 seconds, 549065 bytes. |


# Task Plan: Per-second Video Billing and Subscription Eligibility (2026-07-29)

## Goal
Determine the minimal cross-layer design needed to bill Seedance/xAI video models by generated seconds and make subscription-quota charging opt-in per video model, defaulting to wallet-only.

## Current Phase
Complete

### Phase 1: Existing billing-chain discovery
- [x] Trace model pricing, task precharge/final settlement, video duration, and subscription quota selection.
- [x] Identify existing reusable settings and database contracts.
**Status:** complete

### Phase 2: Design options and recommendation
- [x] Compare configuration-level, model-metadata, and billing-policy approaches.
- [x] Specify defaults, migration compatibility, API/UI changes, and failure behavior.
**Status:** complete

### Phase 3: Verification and handoff
- [x] Cross-check the recommendation against Seedance and xAI relay flows.
- [x] Deliver concrete file-level change points, test scope, and unresolved product choice.
**Status:** complete

## Key Questions
1. Is authoritative generated duration available before submission, only after task completion, or both?
2. Where does the current code choose subscription quota versus wallet balance?
3. Should subscription eligibility belong to exact models or broader channel/model billing rules?

## Decisions Made
| Decision | Rationale |
| --- | --- |
| Keep this turn read-only for business code | The user requested architecture analysis, not implementation. |
| Use exact model-name per-second pricing | Resolution is encoded in distinct model aliases such as `xxx-720p`; no resolution multiplier is required. |
| Treat validated request duration as the generation billing quantity | It enables full precharge before upstream work and avoids successful-task undercharge from a failing completion supplement. |
| Add model billing metadata beside `ModelPrice` | Preserves the existing numeric price map while adding the missing unit and subscription policy. |
| Default video subscription eligibility to false | Disallowed models bypass all subscription preferences and use wallet only, as requested. |

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Existing planning files contain extensive completed tasks | 1 | Append an isolated task section and preserve all historical records. |

---

# Task Plan: Per-second Video Billing Implementation (2026-07-29)

## Goal
Implement the approved VideoPricing configuration, normalized duration contract, wallet-only-by-default policy, provider adaptors, admin UI, public pricing metadata, and regression coverage.

## Current Phase
Complete

### Phase 1: Configuration and immutable pricing data
- [x] Add validated, atomically updated VideoPricing settings and Option registration.
- [x] Add public pricing DTOs and durable video pricing snapshots.
**Status:** complete

### Phase 2: Relay billing and funding policy
- [x] Resolve exact-model pricing before upstream mapping and precharge the full duration cost.
- [x] Force all video requests to wallet unless the exact model explicitly enables subscription billing.
- [x] Persist and audit immutable billing inputs without completion-time repricing.
**Status:** complete

### Phase 3: Provider duration adaptors
- [x] Add the strong VideoBillingEstimator contract.
- [x] Implement xAI, Seedance/Doubao, Sora, Gemini, and Vertex duration resolution.
- [x] Reject missing duration and unsupported edit operations for bound per-second models before upstream dispatch.
**Status:** complete

### Phase 4: Admin and public pricing UI
- [x] Add the VideoPricing settings editor and ratio-settings tab.
- [x] Render per-second billing in public/admin pricing views and add all locale strings.
**Status:** complete

### Phase 5: Verification
- [x] Run focused backend configuration, adaptor, billing, lifecycle, and funding-source tests.
- [x] Run full backend, frontend build, i18n, mock-provider, and responsive UI checks.
**Status:** complete

### Phase 6: Kling frame minimum reference correction
- [x] Add an explicit one-image minimum for Kling 3.0 frame mode in Adobe2API and new-api capability validation.
- [x] Prove invalid zero-image requests fail before wallet precharge and upstream submission.
- [x] Rebuild Docker dev and retest Kling 3.0 frame plus Kling Omni images through new-api.
**Status:** complete

## Locked Decisions
- Public normalized video requests use `output.duration`; compatibility endpoints retain provider-specific fields.
- Per-second bound models require an explicit positive integer duration and never use provider defaults.
- Resolution is represented only by exact public model aliases; video pricing ignores resolution ratios.
- A pricing binding replaces legacy ModelPrice billing; removing it restores legacy behavior.
- All video requests, including unbound legacy video models, are wallet-only unless an exact binding enables subscriptions.
- xAI edit is unsupported for per-second pricing; extension charges the explicitly requested added seconds.
- No SQL schema migration is introduced; task billing snapshots remain in private JSON data.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| New consumption-log test placed `Action` directly on `RelayInfo`, but it belongs to embedded `TaskRelayInfo` | 1 | Move the field into an explicit `TaskRelayInfo` fixture. |
| New relay pricing test reached wallet preconsume with user ID 0 and panicked in the user lookup path | 1 | Use the existing isolated SQLite relay fixture and a real zero-balance user/token so the test stops deterministically before upstream dispatch. |
| Video duration audit test supplied a log ID but omitted the public task ID used by the log ownership guard | 1 | Populate the same `PublicTaskID` before logging and on the task fixture, matching the real submission lifecycle. |
| Repository i18n lint reported 442 findings, including one new internal `per_second` literal in the VideoPricing editor | 1 | Reuse the exported `VIDEO_PRICING_MODE` constant so this change adds no lint finding; retain the unrelated repository baseline. |
| First ad hoc locale-coverage command over-escaped its JavaScript regular expression | 1 | Replace it with deterministic string splitting and rerun against all seven locale objects. |
| A locale-coverage one-liner used a JavaScript template literal inside a double-quoted shell argument | 1 | Use ordinary string concatenation in a single-quoted script; all seven `Preview group ratio` translations then validated. |
| The first Docker health loop assigned to zsh's read-only `status` parameter | 1 | Rename it to the task-specific `app_health`; the rebuilt application container then reported healthy without restarting PostgreSQL or Redis. |
| A database-test discovery command used an unmatched root-level `*_test.go` glob under zsh | 1 | Use ripgrep's `-g '*_test.go'` filter; confirmed SQLite coverage and the existing optional cross-database test entry points without modifying files. |
| Second locale-coverage check treated locale JSON as flat although keys live under `translation` | 1 | Compare the baseline/current `translation` objects; all 46 new keys are non-empty in all seven locales. |
| Docker UI labeled the non-persisted price calculator input as `分组倍率`, which looked like a VideoPricing setting | 1 | Rename it to `预览分组倍率`; runtime billing continues to resolve the effective user/aggregate group ratio automatically. |
| First disposable Docker QA username exceeded the existing username max validation | 1 | No row was created; register the shorter unique username `cvpqa0729` instead. |
| Browser direct navigation to the logout API was blocked after clearing the stale local session | 1 | Use the resulting unauthenticated login page and a disposable root account through the normal login form. |

---

# Task Plan: Async Image Public Error 524 Masking (2026-07-28)

## Goal
Map internal upstream balance/quota failures to the public business error code `"524"` without exposing balances, channel/subgroup details, provider codes, or upstream request IDs through task polling or account Webhooks.

## Current Phase
Complete

### Phase 1: Public error boundary
- [x] Define the stable `"524"` public error and retryable generic message.
- [x] Detect structured provider quota codes with a narrow legacy-message fallback.
- [x] Persist structured image-handle provider error fields through the callback DTO.
- [x] Preserve existing public behavior for unrelated failures.
**Status:** complete

### Phase 2: Contract documentation
- [x] Update the generated Resource Center OpenAPI examples and semantics.
- [x] Update GPT-Image and Gemini task/Webhook documentation.
- [x] Correct terminal-failure idempotency retry guidance.
**Status:** complete

### Phase 3: Verification
- [x] Run focused Go tests for the public error projection and Webhook.
- [x] Run full Go tests plus OpenAPI/frontend checks.
- [x] Build supertokendoc.
- [x] Rebuild Docker dev and verify polling, Webhook, retained administrator diagnostics, and legacy pending-delivery masking.
**Status:** complete

## Locked Decisions
- `"524"` is a string business code, not an HTTP status code.
- Failed async task polling remains HTTP 200 with `status=failed`.
- Public polling and Webhooks receive only the wrapped error; administrator diagnostics retain the raw provider failure.
- This privacy rule cannot be bypassed by relay passthrough settings.
- No image-handle protocol, database, or environment-variable change is required.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Existing planning files contain extensive completed work | 1 | Append an isolated current-task section and preserve all historical content. |
| A combined progress patch used a heading from another planning file as task-plan context | 1 | The patch changed nothing; split subsequent planning updates by file and exact section. |
| A multi-hunk phase update did not match the duplicated planning headings | 1 | Re-read the isolated current section and apply the status update with its exact local context. |
| Initial Webhook model inspection used nonexistent split filenames | 1 | Read the event-creation service directly; it always persists the event before optionally creating a delivery, so Docker acceptance needs no endpoint fixture. |
| First Docker callback using `image_handle_1` returned `callback secret not found` | 1 | Keep the disposable queued fixture, inspect the runtime secret-ID resolution and retry with the actually configured identifier rather than repeating the request. |
| Background task polling terminalized the disposable queued task before the corrected callback | 1 | Remove its generated event, reset the same fixture, and submit the signed callback immediately in one orchestrated acceptance step. |
| First fixture reset referenced nonexistent `webhook_deliveries.event_id` | 1 | The transaction rolled back; inspect the table and use its actual `event_record_id` foreign key in the corrected cleanup. |

---

# Task Plan: Image-handle Trace Search and Task Table Diagnostics (2026-07-23)

## Goal
Make synchronous image failures traceable across new-api and image-handle, add administrator search by new-api Request ID/client task ID/image-handle provider task ID, prevent the image task table from overflowing, and expose task execution duration.

## Current Phase
Complete

### Phase 1: Trace contract and persistence
- [x] Record new-api `client_task_id` and credential `lease_id` before calling image-handle.
- [x] Record image-handle `provider_task_id` from every structured sync response, including failure and timeout.
- [x] Persist all available trace identifiers in new-api error-log `other`.
- [x] Add an image-handle `request_id` index and exact trace-ID task filtering.
**Status:** complete

### Phase 2: Image-handle administration UI
- [x] Add one exact trace search control for new-api Request ID, new-api task ID, and image-handle provider task ID.
- [x] Keep long parameters, usage, errors, IDs, and URLs inside stable table columns with ellipsis and accessible full-value inspection.
- [x] Display task execution duration using persisted timestamps, including a live elapsed value for active work.
**Status:** complete

### Phase 3: Verification
- [x] Add focused backend tests for trace capture, task filtering, and duration projection; verify the migration contains the request-ID index.
- [x] Run image-handle tests/build and focused new-api tests.
- [x] Start an isolated local service and verify desktop/mobile table layout with screenshots and DOM geometry.
**Status:** complete

## Locked Decisions
- Search is administrator-only and exact-match across `request_id`, `client_task_id`, and `provider_task_id`; public task lookup semantics do not change.
- Existing image-handle task rows already contain all three identifiers, so no data backfill is required.
- Duration is derived from `started_at` and `finished_at`/current time; no redundant duration column is persisted.
- Long values remain inspectable through tooltip/detail presentation instead of expanding table tracks.
- Existing unrelated untracked files and historical planning records remain untouched.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Existing planning files contain completed historical tasks | 1 | Append a separate current task section and preserve all prior records. |
| Mobile loading text disappeared before the browser wait locator was counted | 1 | Refresh the DOM snapshot and continue from the already loaded dashboard; no application failure occurred. |

---

# Task Plan: Task Log Public Video URL Follow-up (2026-07-23)

## Goal
Make task-log video preview, copy, and open actions use the authenticated public `task_.../content` route while preserving the raw upstream result URL internally for proxy retrieval.

## Current Phase
Complete

### Phase 1: Diagnose the remaining UI path
- [x] Confirm the task log receives `TaskDto.result_url`, not the OpenAI video status representation.
- [x] Confirm `TaskModel2Dto` currently exposes `task.GetResultURL()` unchanged.
- [x] Reject database replacement because the proxy still needs the original upstream URL.
**Status:** complete

### Phase 2: Public DTO conversion
- [x] Convert successful video task DTO result URLs to the public proxy route.
- [x] Preserve non-video task result URLs and internal task storage unchanged.
- [x] Add focused regression coverage.
**Status:** complete

### Phase 3: Docker task-log acceptance
- [x] Rebuild Docker dev and verify the task API returns a public URL for existing xAI tasks.
- [x] Verify authenticated browser playback through the same public route used by the task-log modal.
- [x] Run final backend/frontend checks and confirm container health.
**Status:** complete

## Locked Decisions
- Do not rewrite `Task.PrivateData.ResultURL`; it remains the upstream fetch location.
- Apply the public URL at the task DTO boundary so every task-log consumer receives the same safe value.
- Scope conversion to video actions/platforms so image and audio result behavior is unchanged.
- Use a same-origin relative path for dashboard/session compatibility; retain absolute proxy URLs in external OpenAI video status responses.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| The only controllable browser has no local login session and redirects task logs to `/login` | 1 | Create an isolated disposable local administrator for UI playback acceptance, then remove it without touching existing accounts. |
| The first disposable username exceeded the existing username length limit | 1 | Registration rejected it before creating data; use a shorter unique fixture name. |
| The disposable administrator lacks task-log menu permission | 1 | Grant only the task-log menu key to the fixture, then remove it with the account. |
| Playwright clicks on the unique preview button did not open the modal | 2 | Inspect the bound modal component and browser logs, then use the stable visible DOM node or direct media-state evidence instead of repeating the same click path. |
| Visible DOM click on the same icon button also did not change React modal state | 1 | Stop click retries; validate the task DTO and authenticated media URL directly in tabs sharing the same browser session. |
| Browser client blocks direct navigation to the JSON API with `ERR_BLOCKED_BY_CLIENT` | 1 | Issue the same read-only same-origin GET from the authenticated task page instead. |
| The browser's read-only page sandbox does not expose `fetch` | 1 | Use the already-rendered task rows plus direct authenticated browser navigation to the public media route for playback evidence. |
| CLI session request omitted the frontend's `New-Api-User` header | 1 | Login succeeded but task API rejected the request; repeat with the fixture user ID header and log out in the same command. |
| Browser logout API navigation and physical account-menu click are blocked by the in-app browser | 3 | Stop UI retries, close all isolated tabs, and delete the disposable account so its remaining browser session no longer identifies an existing user. |

---

# Task Plan: xAI Video Provider Compatibility (2026-07-23)

## Goal
Keep the xAI video relay compatible with both CLIProxyAPI and sub2api by preserving configured upstream model names and proxying completed video content through the public task ID.

## Current Phase
Complete

### Phase 1: Contract diagnosis
- [x] Reproduce a real local xAI video task through Docker dev.
- [x] Confirm that model normalization changes `grok-imagine-video-1.5` incorrectly.
- [x] Confirm that sub2api returns a relative content URL which the current proxy cannot request.
**Status:** complete

### Phase 2: Generic compatibility implementation
- [x] Preserve `UpstreamModelName` exactly and leave provider aliases to channel model mapping.
- [x] Return the public `task_...` content URL from xAI status responses.
- [x] Resolve relative result URLs against the channel base URL and attach Bearer auth only for same-origin targets.
- [x] Preserve absolute CDN URLs without channel auth and strip auth across cross-origin redirects.
- [x] Allow bounded long-running video downloads instead of truncating them at 60 seconds.
**Status:** complete

### Phase 3: Regression verification
- [x] Add focused adaptor and proxy URL/auth tests.
- [x] Run the full Go test suite and diff validation.
**Status:** complete

### Phase 4: Docker dev acceptance
- [x] Rebuild and recreate Docker dev from the final source.
- [x] Verify canonical model handling with a real xAI video task.
- [x] Verify status and public content download end to end.
**Status:** complete

## Locked Decisions
- The shared xAI adaptor does not normalize or infer model aliases.
- CLIProxyAPI/sub2api model differences are expressed only through channel `model_mapping`.
- Both compatible upstreams use the generic create and status routes under `/v1/videos`.
- Clients receive only the public new-api task content URL, never an upstream task ID path.
- Relative result URLs may use channel credentials; absolute cross-origin CDN URLs never receive them.

---

# Task Plan: Async Image Final Usage Log Reconciliation (2026-07-22)

## Goal
Keep one user-facing consume log per async image task: precharge at submission, then reconcile that same row to the real terminal quota and usage while preserving the original Request ID.

## Current Phase
Complete

### Phase 1: Settlement contract and persistence ordering
- [x] Confirm the current delta log is financially correct but produces incorrect usage-log semantics and aggregates.
- [x] Map callback, task persistence, consume-log ID persistence, and early-completion compensation ordering.
- **Status:** complete

### Phase 2: Original-log reconciliation
- [x] Persist the submission Request ID in the task billing snapshot for fallback correlation.
- [x] On successful usage billing, update the original log with final quota, usage, duration, content, and settlement metadata.
- [x] On async image failure, reconcile the original row to zero and mark the precharge refund instead of adding a refund row.
- [x] Preserve generic non-image async billing log behavior.
- **Status:** complete

### Phase 3: Tests and Docker dev
- [x] Cover over-precharge refund, under-precharge supplement, exact charge, failure refund, Request ID, early completion, and usage statistics.
- [x] Run focused and full Go checks plus diff validation.
- [x] Rebuild Docker dev and verify one real usage-billed async image record end to end.
- **Status:** complete

## Locked Decisions
- Financial balance, subscription, token-quota, task-quota, and channel/user counters continue to settle by delta.
- The user-facing log row stores the final actual quota, not the delta; it remains a consume row so one task counts as one request.
- Precharge, actual quota, delta, settlement direction, and terminal state remain auditable in `other`.
- Real usage is never estimated, and image-parameter pricing success remains anchored to its request snapshot.
- Do not add a historical data migration in this change.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Combined planning patch used task-plan hunks out of file order | 1 | The atomic patch changed nothing; reorder the hunks and reapply. |
| Initial Docker callback fixture read the channel callback secret from `other` instead of `settings` | 1 | Correct the test harness lookup, rerun the callback flow, and remove the disposable fixture; production code was unaffected. |
| One Docker validation SQL query omitted a closing parenthesis | 1 | Correct the read-only validation query and rerun it successfully; no database state changed. |
| Final manual mock health probe used `/health` instead of the configured `/healthz` | 1 | Compose already reported the mock healthy; use the configured endpoint for the explicit probe. |

---

# Task Plan: OpenAI Null Required Tool Schema Compatibility (2026-07-22)

## Goal
Allow administrators to opt into a narrowly scoped OpenAI Chat Completions compatibility cleanup that removes JSON Schema keyword `required` only when its value is `null`, including nested schemas and raw passthrough requests.

## Current Phase
Complete

### Phase 1: Setting and bounded schema cleaner
- [x] Add an independent hot global switch that defaults disabled.
- [x] Recursively remove only schema-keyword `required: null` from recognized child-schema locations.
- [x] Cover both `tools[].function.parameters` and legacy `functions[].parameters`.
**Status:** complete

### Phase 2: Relay and desktop UI integration
- [x] Apply the cleaner to serialized and raw passthrough OpenAI Chat Completions requests.
- [x] Add the switch to Compatibility Management -> OpenAI Compatibility and all seven locales.
- [x] Run backend tests and scoped frontend build/lint/format/i18n checks without mobile QA.
**Status:** complete

### Phase 3: Docker live A/B
- [x] Rebuild Docker dev and snapshot the original setting.
- [x] Send an identical real payload through `test-gpt兼容`: disabled must reproduce upstream 400, enabled must return a successful response/tool call.
- [x] Restore the setting to disabled and confirm Docker health and fixture cleanup.
**Status:** complete

## Locked Decisions
- This feature is independent from reserved-function-name compatibility and defaults disabled.
- Only `required: null` is removed; other invalid schema types are preserved for upstream validation.
- Recursion follows JSON Schema child-schema keywords and never enters data-bearing `default`, `const`, `enum`, or `examples` values.
- Messages, tool-call arguments, content, and unrelated JSON remain byte-semantically untouched.
- Mobile UI compatibility testing is explicitly excluded.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| The first Docker A/B SQL helper used an incompatible placeholder form | 1 | PostgreSQL rejected the helper before any option mutation or API request; replace it with the correct parameter handling and continue. |
| The first disabled request surfaced as client HTTP 500 because local upstream-error passthrough was disabled | 1 | Logs confirmed the target upstream 400; temporarily enable passthrough for the identical-payload A/B, then restore its original disabled state. |
| Initial disposable UI fixture values exceeded existing validation limits | 1 | Shorten only the fixture fields and rerun desktop UI validation; production validation was unchanged. |
| Initial UI fixture cleanup observed a cached root role after database mutation | 1 | Delete the fixture through the normal management endpoint, then verify the database count is zero. |

---

# Task Plan: OpenAI Reserved Python Tool Compatibility (2026-07-22)

## Goal
Transparently support OpenAI Chat Completions clients whose custom function names collide with configurable upstream reserved names when an upstream or intermediate bridge converts the request to Responses.

## Current Phase
Complete

### Phase 1: Settings and request-scoped alias contract
- [x] Add hot global compatibility settings with an enable switch and configurable reserved-name list.
- [x] Add a collision-safe `run_<name>` alias mapping to relay request state.
- [x] Rewrite structured Chat Completions function-name references in normal and raw passthrough requests.
**Status:** complete

### Phase 2: Response restoration
- [x] Restore aliases in non-streaming Chat Completions responses.
- [x] Restore aliases in streaming Chat Completions chunks without changing unrelated JSON fields.
**Status:** complete

### Phase 3: Verification
- [x] Cover model scope, alias collisions, request history, tool choice, normal responses, and streaming responses.
- [x] Run focused and package-level Go tests plus formatting and diff checks.
**Status:** complete

### Phase 4: Compatibility management UI
- [x] Add the switch and reserved-name input to the existing OpenAI compatibility tab.
- [x] Add all frontend translations and verify desktop form behavior; mobile testing was excluded by user request.
**Status:** complete

### Phase 5: Docker live contrast matrix
- [x] Snapshot the live compatibility settings and resolve the authorized `test-gpt兼容` token without exposing its secret.
- [x] Verify disabled + `python` forwards the original name using a model-reported tool-schema observation.
- [x] Verify enabled + nonmatching keyword + `python` still forwards the original name.
- [x] Verify enabled + matching `python` exposes `run_python` upstream while restoring the client-visible structured name to `python`, for non-streaming and streaming requests.
- [x] Restore the original compatibility settings and confirm Docker dev remains healthy.
**Status:** complete

### Phase 6: Original 400 reproduction and identical-payload A/B
- [x] Trace Request ID `202607220331435264898266qI0WOpT` through local logs, consume/error records, and request snapshots without exposing credentials.
- [x] Test bounded request-shape variants for a custom function named `python`, stopping once the exact upstream reserved-name 400 is reproduced.
- [x] Confirm the identical-payload 400/200 branch is not applicable because no disabled request reproduced the 400; retain the completed upstream-visible alias A/B as compatibility proof.
- [x] Document every attempted shape and the missing external condition instead of claiming a successful 400/200 contrast.
- [x] Restore the exact pre-test configuration and verify Docker dev health.
**Status:** complete

## Locked Decisions
- Compatibility is implemented entirely in new-api because sub2api is an immutable bridge.
- Automatic rewriting is model-independent and activates only for configured reserved names declared by an OpenAI Chat Completions request.
- The global setting defaults to enabled with `python`; administrators may enter comma- or newline-separated names in Compatibility Management.
- Aliases default to `run_<original>` and add a numeric suffix on collision.
- Rewriting is bidirectional and request-scoped; clients continue to see `python`.
- JSON function-name fields are rewritten structurally. Arbitrary byte replacement is forbidden because aliases may occur in tool arguments or ordinary content.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| OpenAI documentation MCP was registered but is not loaded in the current task; direct official-doc fetches returned HTTP 403 | 1 | Treat the captured OpenAI 400 response as runtime evidence and avoid claiming an unverified complete reserved-name list. |
| Initial compatibility tests over-escaped nested `arguments` JSON inside raw string literals, making the fixtures invalid | 1 | Keep a single JSON escape layer inside `arguments` and rerun the same focused tests. |
| Full frontend i18n lint reports 420 repository-wide hardcoded-string issues | 1 | Confirm this matches the existing baseline and that `src/pages/Compatibility` has no findings; scoped locale status passes. |
| A planning-only patch matched the first generic `Current Phase` heading instead of the OpenAI section | 1 | Restore the unrelated completed phase and scope the update by the OpenAI task heading; no product code or runtime configuration changed. |
| The first inline live-test command collided with JavaScript template interpolation before shell execution | 1 | No request or mutation occurred; create the secret-free harness under `tmp/` with `apply_patch`, execute it directly, then remove only that file. |
| The temporary root access token was generated as 64 characters but the local column is `char(32)` | 1 | PostgreSQL rejected the update before any option write or upstream call; cleanup confirmed the exact original state, then reduce the process-local token to 32 characters. |
| Disabled + `python` returned 200 instead of the assumed reserved-name 400 | 1 | Confirmed it still used channel 85/sub2api; inspect live configuration observability and reproduce the original request shape before interpreting the compatibility result. |
| The live harness attempted to iterate an empty request-header ID array after all four assertions passed | 1 | Cleanup still restored the exact state; obtain authoritative Request IDs from the four new token logs instead of resending paid requests. |

---

# Task Plan: Async Image Token Usage Log Backfill (2026-07-22)

## Goal
Show real upstream token usage in the original async image consume-log token columns without changing image-parameter billing or creating estimated usage.

## Current Phase
Complete

### Phase 1: Log update contract
- [x] Confirm the task stores the original consume-log ID and current audit merge already uses it.
- [x] Select a single CAS update that merges audit metadata and token columns together.
- **Status:** complete

### Phase 2: Implementation and regression coverage
- [x] Extend the consume-log merge API with optional real prompt/completion token values.
- [x] Backfill token columns only when image execution audit contains real upstream usage.
- [x] Cover successful backfill, missing usage, stale associations, and guarded update behavior.
- **Status:** complete

### Phase 3: Verification
- [x] Run focused model/service tests and formatting checks.
- [x] Run broader affected-package tests and inspect the final diff.
- **Status:** complete

## Locked Decisions
- Token backfill is observability only and never changes the request-time image pricing snapshot or final charge.
- Never estimate tokens from prompt length or image count.
- Reuse `TaskBillingContext.ConsumeLogId`; do not scan logs by JSON fields.
- Merge audit JSON and token columns in one guarded update.
- Retry rare CAS conflicts by re-reading the latest metadata; do not add retries to the normal path.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |

---

# Task Plan: Separate Async Image and Webhook Credentials (2026-07-22)

## Goal
Restore token-scoped routing for asynchronous image submission and replace Resource Center API Key reuse in outbound Webhooks with an independent system-generated `wk-` credential, including Resource Center UI, documentation, tests, and Docker dev acceptance.

## Current Phase
Complete

### Phase 1: Authentication contract and migration
- [x] Require normal `sk_` Token authentication for `POST /v1/image/tasks` while preserving token group, model limits, quota, and audit context.
- [x] Keep task/resource reads on `ak_` Resource Center authentication.
- [x] Generate, encrypt, reveal, rotate, and migrate an independent `wk-` Webhook credential without adding a business table.
- **Status:** complete

### Phase 2: Webhook delivery, UI, and docs
- [x] Deliver Webhooks with `Authorization: Bearer wk-...` and never expose `ak_` to callback receivers.
- [x] Update the Resource Center Webhook saved/edit states with reveal, copy, and regenerate controls using existing Semi Design patterns.
- [x] Update all seven locales, curl examples, receiver examples, and OpenAPI security contracts.
- **Status:** complete

### Phase 3: Verification and Docker dev
- [x] Add focused route, group-selection, secret lifecycle, migration, delivery, redaction, and UI checks.
- [x] Rebuild `docker-compose-dev.yml`, verify `sk_` submission plus `ak_` query and `wk_` callback end to end, and clean fixtures.
- [x] Run affected/full Go and Bun checks, responsive browser QA, and final diff/sensitive-data audit.
- **Status:** complete

## Locked Decisions
- The create request carries exactly one credential: `sk_`; no `ak_`, callback URL, or Webhook key is supplied per task.
- The terminal task's `user_id` resolves the account Webhook configuration and its encrypted `wk-` key.
- `ak_` remains the Resource Center read credential for tasks, assets, and uploads, but is never sent to callback receivers.
- Webhook receivers authenticate `wk-`, return any 2xx on success, and deduplicate at-least-once delivery by event ID.
- Existing configured Webhooks receive a generated `wk-` during migration and switch immediately; operators retrieve it from the UI and update receivers.
- Reuse `WebhookEndpoint.AuthKeyEncrypted`; do not add a new business table.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Cleanup preflight used inferred `image_tasks` instead of the live generic `tasks` table | 1 | The read-only query changed nothing; verify the live task table and rerun against `tasks.user_id`. |
| Cleanup preflight used the inferred table name `asset_api_keys` instead of GORM's actual `asset_keys` | 1 | The read-only query changed nothing; verify the live table list and rerun with `asset_keys`. |
| Cleanup preflight assumed `webhook_deliveries.user_id`, but deliveries link users through their event | 1 | The read-only query changed nothing; inspect the actual table relationship and join `delivery -> event -> user` for exact counts and deletion. |
| Browser viewport capability mapped the requested `375x812` override to an effective `560x1212` CSS viewport | 1 | Do not claim this as mobile acceptance; read the advertised CDP capability and use exact device-metrics emulation, then reset both overrides. |
| A combined planning/findings/progress patch contained an extra empty hunk marker | 1 | The atomic patch changed nothing; remove the malformed marker and reapply against the same verified headings. |
| The browser backend logged an unrelated Statsig telemetry timeout while confirming key regeneration | 1 | The local action completed and authoritative UI/database checks passed; record it as tooling noise and do not retry the successful mutation. |
| Initial PostgreSQL schema inspection used the nonexistent default `postgres` role | 1 | Read `docker-compose-dev.yml` and rerun with the configured `root` role; no data was changed. |
| Browser QA waited for the post-generation button name `隐藏密钥`, but the expected control did not appear within 10 seconds | 1 | Backend logs proved the browser retained a stale session for already-cleaned user `994203`; the PUT correctly returned 404. Use a disposable live local account, then clean it precisely. |
| A combined notation/findings patch expected findings in a different order | 1 | The atomic patch changed nothing; split updates by file and normalize the canonical prefix to the user-requested `wk-`. |

---

# Task Plan: Async Worker Operations and Webhook Delivery Management (2026-07-21)

## Goal
Replace the serial fixed-20 image dispatch and Webhook loops with independently bounded, dynamically configurable workers, and turn Async Task Management into a live operations surface with Docker dev acceptance coverage.

## Current Phase
Complete

### Phase 1: Worker runtime and leases
- [x] Add normalized concurrency and request-timeout settings.
- [x] Implement independent capacity-aware schedulers, endpoint limits, transport reuse, telemetry, and stale Webhook lease recovery.
- [x] Add cancellation and bounded shutdown behavior.

### Phase 2: Admin API
- [x] Extend compatible stats with image/Webhook queue and worker runtime data.
- [x] Add paginated async task and Webhook delivery administration APIs.
- [x] Add safe detail payloads and CAS-protected manual retry.

### Phase 3: Admin UI
- [x] Rework the page into Overview, Async Tasks, Webhook Deliveries, and Settings tabs.
- [x] Add active-tab polling, filters, pagination, detail SideSheet, and responsive behavior.
- [x] Update scoped frontend locale keys.

### Phase 4: Docker dev and verification
- [x] Add an opt-in `async-test` mock service with deterministic delay/failure modes and concurrency counters.
- [x] Run focused/full Go and Bun checks; repository-wide Prettier/i18n baselines remain documented separately from clean changed-file checks.
- [x] Rebuild Docker dev, verify concurrency/recovery, and inspect desktop/mobile UI.

**Status:** complete

## Locked Decisions
- Image dispatch and Webhook delivery have independent concurrency limits; Webhook also has a per-endpoint cap.
- Claims are based on available capacity rather than a separately configurable batch size.
- Settings apply to new claims without restarting or cancelling requests already in flight.
- Existing async stats fields remain compatible; monitoring responses never expose authorization material or task private credentials.
- Delivery semantics remain at-least-once and stale completions are fenced by lock tokens.
- The UI remains a dense Semi Design operations page with no historical time-series charts in this version.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Focused service tests retained an assertion that expired Webhook processing leases are never reclaimed | 1 | Replace it with the approved recovery contract: reclaim after expiry and reject completion from the stale lock token. |
| Queue-stat fallback logging used a nil context when legacy tests intentionally omitted the new queue tables | 1 | Use `context.Background()` so legacy task-only fixtures preserve the compatible top-level response and new queue sections remain zero-valued. |
| The first frontend format command ran from `web/` but still prefixed files with `web/` | 1 | No file matched and no product file changed; rerun with `src/pages/AsyncTask/...` paths. |
| Targeted ESLint rejected the new files' semantically equivalent AGPL line wrapping | 1 | Copy the repository's exact protected header text and wrapping into every new AsyncTask module. |
| Full i18n lint reported four new AsyncTask technical literals (`HTTP`/`ms`) alongside the repository baseline | 1 | Route the labels through scoped translations and keep the AsyncTask directory at zero new lint findings. |
| `bun run lint` was first launched concurrently with Vite build and observed transient missing files under generated `dist/` | 1 | Wait for the build to finish, then treat the sequential full lint result separately from targeted changed-file formatting. |
| First Docker image-concurrency acceptance inserted zero fixtures because `docker exec` did not keep stdin open for the SQL heredoc | 1 | Add `-i`, verify fixture count before polling, and reuse the already-confirmed hot worker configuration. |
| Lease-recovery script assumed `restart: unless-stopped` would restart a container after an explicit `docker kill` | 1 | Preserve the expired locked record and explicitly start `new-api-dev` through Compose before verifying reclaim. |
| First reclaimed lease was discarded because startup migration disables non-primary account Webhook endpoints | 1 | Repeat against the test user's primary configuration, then restore its original URL/status; reclaim completed successfully after lease expiry. |
| Browser QA temporary-user insert used a `psql -c` variable form that was not expanded | 1 | No row was written; expand the locally generated bcrypt hash in the shell and retry with its quote-safe character set. |
| Browser role locator did not expose a `hover()` method for the hover-triggered Semi account dropdown | 1 | Use the application's same-origin logout GET in the current authenticated tab, then continue through the normal login page. |
| In-app browser blocked direct top-level navigation to `/api/user/logout`, and its documentation object has no partial `lookup()` helper | 1 | Keep the session in the UI and use the available coordinate mouse movement to trigger the hover-only dropdown. |
| Coordinate mouse movement used screenshot-space coordinates that did not map to the browser's interaction viewport | 1 | Stop guessing coordinates; issue the normal same-origin logout request through the page execution API and resume visible-form interaction. |
| Browser page execution sandbox exposed neither `fetch` nor the `MouseEvent` constructor | 2 | Use DOM `createEvent` once to trigger the existing hover handler; if unavailable, switch browser profiles instead of retrying the isolated-script path. |
| Chrome extension browser was unavailable on the initial connection and one prescribed retry | 1 | Do not install or repair browser integrations; return to the available in-app browser and derive exact CUA coordinates from the target element bounds. |
| Exact DOM-derived CUA coordinates still did not open the Semi hover dropdown | 2 | Treat this as browser-backend/overlay incompatibility and navigate to the normal `/login` application route to re-authenticate directly. |
| AsyncTasks mobile-filter patch needed Prettier line wrapping | 1 | ESLint already passed; run the repository formatter on only the changed component and recheck both AsyncTask tabs. |
| Combined mobile navigation/filter browser call exceeded the execution timeout and reset the browser-control kernel | 1 | Reconnect to the existing browser, split navigation and assertions into shorter calls, then reset emulation explicitly. |
| Mock `/reset` and `/control` were first invoked in parallel, so reset restored the default 500ms image delay after the control write | 1 | Metrics were cleared; apply `/control` once more sequentially after reset and verify the final config. |
| The generic planning completion script exited nonzero after this task was complete | 1 | It scans the entire long-lived planning file and found pending checkboxes in the older image-pricing plan; leave unrelated historical task state untouched and verify this plan's four phases directly. |

---

# Task Plan: Multipart Async Image Editing and Webhook Retries (2026-07-18)

## Goal
Allow `POST /v1/image/tasks` to accept synchronous-style multipart image edit requests while preserving the existing JSON URL contract, durable task flow, Resource Center Key authorization, and idempotency semantics. Restore bounded Webhook retries with administrator-configurable attempt count and fixed interval.

## Current Phase
Complete

### Phase 1: Contract and discovery
- [x] Confirm one route with content-type dispatch: JSON remains unchanged; multipart defaults to edit.
- [x] Reuse the existing image-handle upload proxy and current upload limits.
- [x] Define multipart idempotency fingerprints from normalized scalar fields and file content hashes before upload.
- **Status:** complete

### Phase 2: Backend implementation
- [x] Parse and strictly validate multipart fields/files.
- [x] Resolve idempotent replays before uploading and map upload URLs into the normalized task DTO.
- [x] Share upload proxy response parsing with the standalone upload endpoint.
- **Status:** complete

### Phase 3: Tests and documentation
- [x] Cover field mapping, upload errors, file validation, and same/conflicting idempotency retries.
- [x] Document curl usage and add multipart requestBody to OpenAPI 3.1.
- **Status:** complete

### Phase 4: Configurable Webhook retry
- [x] Treat only HTTP 2xx as delivery success and ignore the response body.
- [x] Retry network and non-2xx failures up to the configured total attempt count.
- [x] Add default `3` attempts and `30` seconds fixed interval to Async Task Management.
- [x] Replace the one-shot tests with success, retry, exhaustion, and option normalization coverage.
- **Status:** complete

### Phase 5: Verification and delivery
- [x] Run focused/full tests, frontend/OpenAPI checks, and diff checks.
- [x] Rebuild Docker dev and run local multipart and Webhook retry E2E coverage.
- [x] Commit and push directly to main.
- **Status:** complete

## Locked Decisions
- Multipart accepts `model`, `prompt`, repeated `image`, optional `mask`, `n`, `size`, `quality`, `output_format`, `output_compression`, `background`, optional `client_reference_id`, and optional JSON-object `metadata`.
- Multipart defaults to `operation=edit`; an explicit operation must also be `edit`.
- Multipart files are uploaded internally to image-handle, then execution continues through the existing normalized durable task path.
- The request fingerprint excludes generated temporary URLs and includes normalized fields plus ordered file content hashes.
- No video multipart or additional provider-option surface is added.
- Webhook success means any HTTP 2xx response; the receiver body is ignored and no business acknowledgement schema is required.
- Webhook maximum attempts include the initial request. Defaults are 3 total attempts and a fixed 30-second interval, configurable by administrators in Async Task Management.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Initial focused test compile found a branch-local marshal error variable plus stale/new test imports | 1 | Declare the shared marshal error in `PrepareImageTaskRequest`, remove unused imports, and add the required `mime` import. |
| A locale-tail discovery command referenced the nonexistent legacy `zh.json` filename | 1 | Use the repository's actual `zh-CN.json` and `zh-TW.json` locale files; no product file was affected. |
| The first combined docs/OpenAPI/i18n patch used a context line that did not match the JSX template literal | 1 | The patch was atomic and changed nothing; split the documentation changes into smaller exact-context patches. |
| The first combined planning update expected an error row that was part of the failed atomic docs patch | 1 | Re-read the planning-file header and apply the new scope against its actual content. |

---

# Task Plan: Image-handle channel overrides and signed URL output

## Goal
Make image-handle sync execution honor channel request-parameter overrides and return signed image URLs with literal query separators, while preserving provider compatibility and existing R2 fallback behavior.

## Current Phase
Complete with external token-upstream failure documented

### Phase 1: Design and discovery
- [x] Trace the effective result-format policy, upstream request construction, URL passthrough, and final JSON serialization.
- [x] Confirm channel parameter overrides are currently bypassed by the early image-handle sync branch.
- **Status:** complete

### Phase 2: Implementation
- [x] Apply the selected channel's existing parameter override before building the image-handle sync payload.
- [x] Add a JSON wrapper that disables HTML escaping and use it only for the image-handle client response.
- **Status:** complete

### Phase 3: Verification
- [x] Cover channel override, pricing-owned parameters, signed URL passthrough, and literal ampersand output with focused unit tests.
- [x] Add payload-level coverage and confirm generation/edit compatibility.
- [x] Run focused Go tests, broader affected-package tests, formatting, and diff review.
- **Status:** complete

### Phase 4: Docker and live local integration
- [x] Rebuild local new-api Docker from the verified source.
- [x] Confirm both selected Adobe channels have `response_format=url` in request parameter overrides.
- [x] Call the count Adobe model without client `response_format` and verify Adobe URL passthrough plus literal `&` output.
- [x] Confirm the token Adobe model also receives `response_format=url`; its upstream currently disconnects before returning an HTTP response.
- [x] Retain generated request/application/image-handle logs.
- **Status:** complete_with_external_token_upstream_failure

## Locked Decisions
| Decision | Rationale |
| --- | --- |
| Reuse channel parameter overrides instead of a global GPT-image default | Adobe and official providers can expose the same upstream model name but differ in accepted parameters. |
| Let new-api identify and lock the channel; keep image-handle provider-agnostic | The selected channel already owns its base URL, credentials, model mapping, and parameter override. |
| Preserve the existing override semantics | Operators can force Adobe URL output without introducing a second provider-specific configuration surface. |
| Disable HTML escaping only for the image-handle sync client response | Makes signed URLs copyable without changing unrelated API JSON behavior. |
| Keep Base64-to-R2 conversion as fallback | Providers that do not return URLs remain compatible. |

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| UI design-system script path under the short `r0` skill path resolved to a non-directory | 1 | Inspect the installed skill link/root and rerun the required design-system query from the real script location. |
| Combined planning-file patch expected a template heading that the existing findings file did not use | 1 | Re-read each file header and apply independent insertions using its actual first line. |
| Health polling used zsh's read-only `status` variable | 1 | Use a direct health endpoint check and a non-reserved variable on subsequent checks. |
| Initial image-handle PostgreSQL inspection assumed a `postgres` role | 1 | Read the container connection configuration and use the actual `image_handle` role. |
| A diagnostic query treated the new-api log `other` TEXT column as JSONB | 1 | Cast `other::jsonb` before extracting structured error fields. |
| Adobe token upstream disconnected twice through image-handle and once directly | 3 | Stop paid retries; retain the `fetch failed` and HTTP/2 framing evidence for upstream investigation. |

---

# Task Plan: Simplify Account Webhooks (2026-07-17)

## Goal
Replace the multi-endpoint public management surface with one account-level callback URL and Bearer key while preserving durable delivery, retries, terminal event creation, SSRF protection, and local Docker verification. The current event set remains image-task success/failure, but the configuration and UI must be reusable for future video events.

## Current Phase
Complete

### Phase 1: Backend contract and migration
- [x] Add one-config DTOs/controllers and encrypted user-supplied key storage.
- [x] Collapse legacy endpoints to one active account task config per user.
- [x] Send Bearer authentication and remove signing, rotation, public management routes, and Webhook asset-key scopes.
- **Status:** complete

### Phase 2: Resource Center and documentation
- [x] Replace endpoint/delivery management UI with URL/Key save, test, and disable controls.
- [x] Remove Webhook management operations from OpenAPI while retaining outbound event definitions.
- [x] Update all seven locales and remove obsolete scope controls.
- **Status:** complete

### Phase 3: Verification and local integration
- [x] Add focused backend/migration/route/frontend receiver coverage.
- [x] Run final full Go, Bun, image-handle, OpenAPI, i18n, Compose, and diff checks; i18n retains the documented repository-wide 422-item baseline while change-scoped Webhook files are clean.
- [x] Finish Docker Bearer retry/410 verification; responsive UI passes at 1440px, 560px, and 375x812.
- **Status:** complete

## Locked Decisions
- One independent account-level task Webhook configuration per user; quota-warning Webhooks remain separate.
- new-api generates the account Key with a `wk-` prefix; the owner can reveal, copy, or explicitly regenerate it, while storage remains encrypted and delivery uses `Authorization: Bearer <key>`.
- Both terminal image events are always enabled; no names, event filters, manual retry, secret rotation, or public Webhook management API.
- The reliable event/delivery/attempt tables, automatic retries, retention, leases, and SSRF protections remain internal.
- The storage, console API, UI, delivery worker, and event envelope are task-generic. Video events are not added in this change; future terminal event producers will emit `video.task.*` through the same account configuration.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Planning status patch omitted the existing list marker before `**Status:**` | 1 | Re-read the exact task section and applied a context-correct planning-only patch. |
| Combined 410 E2E script was rejected because its temp-cookie cleanup used `rm -f` | 1 | No request ran; switch to an in-memory `Set-Cookie` value and avoid filesystem cleanup entirely. |
| Focused Webhook test search used a zsh glob with no matches | 1 | No code was affected; search the known service test and discovered test filenames directly with `rg`. |

---

# Task Plan: Webhook Saved View and Generated Key UX (2026-07-17)

## Goal
Make a saved Webhook read as a configuration detail instead of a permanently open form, and replace user-entered credentials with a system-generated `wk-...` Key that the account owner can reveal, copy, or regenerate at any time.

## Current Phase
Complete

### Phase 1: Contract and existing-pattern discovery
- [x] Reuse the Resource Center's existing generated-token action hierarchy where practical.
- [x] Define server-generated Key create/reveal/regenerate semantics with encrypted-at-rest storage.
- **Status:** complete

### Phase 2: Backend and frontend implementation
- [x] Add server-side Key generation, authenticated reveal/copy, and regeneration behavior with focused tests.
- [x] Add saved detail, explicit edit mode, create/regenerate flow, copy affordance, and all locale strings.
- **Status:** complete

### Phase 3: Verification and Docker handoff
- [x] Run focused/full backend and frontend checks.
- [x] Rebuild Docker dev and inspect desktop/560px/375px create, saved, edit, reveal/copy, and regeneration states.
- **Status:** complete

## Locked Decisions
- Saved configuration is read-only until the user clicks Edit.
- New Keys are generated by new-api with a `wk-` prefix and are never accepted as user-entered values in the Resource Center UI.
- Plaintext Key is available through the authenticated account configuration API for reveal/copy at any time; it remains encrypted at rest and is never logged.
- Editing URL keeps the current Key. Regenerating replaces it explicitly and does not change event or delivery semantics.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Combined seven-locale patch assumed a Traditional Chinese value that differed from the file | 1 | The patch was atomic and changed nothing; inspect locale tails with JSON tooling and apply exact per-file additions. |
| Created a browser tab without passing the target URL, so the returned object was not a navigable page | 1 | Create a fresh tab with the URL argument, then navigate the resulting page object. |
| Requested the viewport capability documentation using the short `viewport` name | 1 | Read the capability document under `capabilities/browser/viewport` before applying responsive overrides. |
| Full i18n lint reported 423 repository findings, including one new `spacing='tight'` literal in the Webhook component | 1 | Remove the optional spacing prop; rerun to restore the existing 422-item repository baseline and keep the Webhook component clean. |

---

# Aggregate Group Categories and Token Group UX (2026-07-17)

## Goal
Add configurable aggregate-group categories, category filtering and batch assignment in the admin UI, and category-grouped token options without exposing `auto` for new selection.

## Current Phase
Complete

### Phase 1: Backend model, migration, and APIs
- [x] Add category persistence and aggregate-group assignment.
- [x] Add category CRUD, ordering, deletion fallback, and batch assignment APIs.
- [x] Add category metadata to aggregate-group and user-group responses.
- **Status:** complete

### Phase 2: Aggregate-group admin UI
- [x] Add category manager side sheet and category field to group editing.
- [x] Add filtering, row selection, and batch category assignment.
- [x] Support selection in mobile CardTable cards.
- **Status:** complete

### Phase 3: Token group selector
- [x] Group aggregate options by configured category.
- [x] Put real and uncategorized aggregate groups under Other.
- [x] Hide auto for new selection and preserve historical values on edit.
- **Status:** complete

### Phase 4: Verification
- [x] Add focused backend and frontend tests.
- [x] Run Go tests, Bun checks/build, i18n checks, and responsive browser QA.
- **Status:** complete

## Locked Decisions
- Categories are admin-configurable and single-select.
- Category ID 0 is the virtual, non-deletable Other category.
- Category behavior is presentation-only and never changes routing or billing.
- The token UI hides auto for new selection; backend compatibility remains.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Existing planning files contain prior completed tasks | 1 | Append a separately titled task section and preserve all prior records. |
| Controller compile failed after response signature change | 1 | Add the missing common import and update user-management callers to load category metadata explicitly. |
| Focused controller tests lacked the new category table | 1 | Register AggregateGroupCategory in the shared controller test database migration. |
| Category controller test used a nonexistent real group | 1 | Use the configured default group so the test reaches category preservation behavior. |
| Combined test/log patch targeted the wrong file context | 1 | Reapply the test and planning-record edits under their correct file headers. |
| Bun helper test imported browser-bound `api.js` and failed because `document` is unavailable | 1 | Move new group-option normalization/grouping into a pure helper module and test that module directly. |
| `i18n:extract` rewrote thousands of unrelated existing locale entries | 1 | Revert only the extractor-generated locale diff, then add this feature's keys with a targeted structured JSON update. |
| Full frontend lint scans generated `dist` and existing source baselines | 1 | Record the existing 116-file Prettier and 68-file header baseline; keep targeted changed-file lint green. |
| Docker BuildKit spent over four minutes resolving pinned base-image metadata | 1 | Stop before touching the running container, then retry as a separate no-pull image build followed by service recreation. |
| No-pull Docker build repeated registry metadata waits and pinned bases were absent locally | 2 | Cross-build the current embedded-web Linux binary and layer it over the existing matching dev runtime image for local UI verification. |
| Temporary Docker binary under `tmp/` was excluded from the build context | 1 | Emit the generated binary at a temporary non-ignored root path, then remove it after building the dev image. |
| Category delete confirmation did not open in browser QA | 1 | Make the delete button the direct `Popconfirm` trigger instead of wrapping it in `Tooltip`; preserve a native title and verify the confirmation and fallback flow after rebuilding Docker dev. |
| Recreating Docker used the container name instead of the Compose service name | 1 | Read `docker-compose-dev.yml` and recreate the `new-api-dev` service. |
| A test-category insert collided with the administrator's existing `生图` category | 1 | Stop creating category fixtures and reuse the existing category plus its two assigned Adobe aggregate groups for visual QA; no data was changed. |
| Resizing while the Select popup was already open retained its desktop popup width | 1 | Close and reopen the popup after applying the 375px viewport, matching the real mobile interaction; the reopened popup fit the viewport with zero overflow. |


# Task Plan: Image parameter per-call pricing and image-handle compatibility

## Goal
Add configurable single-dimension per-image pricing for public models, preserve legacy/token billing for unbound models, keep pricing snapshots stable across sync/async execution, and validate the count/token aliases end to end with local new-api and image-handle.

## Current Phase
Implementation review and local integration

### Phase 1: Configuration and billing core
- [x] Add atomic `ImagePricing` option, profile/binding validation, normalization, immutable snapshots, decimal quota calculation, and legacy fallback.
- [x] Resolve public-model pricing before existing model mapping and make `n` the shared multiplier.
- **Status:** complete

### Phase 2: Direct, sync image-handle, and async task paths
- [x] Apply shared normalization to generations, edits, sync image-handle, and `/v1/image/tasks`.
- [x] Persist async snapshots, bypass usage repricing for count mode, and preserve token-mode usage settlement.
- [x] Add targeted image-handle polling/refund fixes and async submit race protection.
- **Status:** complete

### Phase 3: Management UI, marketplace, and logs
- [x] Add profile CRUD/copy, tier editing, bulk model binding, `max_n`, preview, takeover hints, marketplace metadata/filter/details, and log snapshot display.
- [x] Complete independent frontend review and focused i18n/lint/build verification.
- **Status:** complete

### Phase 4: Automated regression
- [ ] Complete independent backend review; focused and full Go tests plus current diff checks already pass.
- [x] Complete image-handle contract tests for `quality`, `size`, `resolution`, `n`, and leased upstream model.
- **Status:** pending

### Phase 5: Docker and live local integration
- [ ] Rebuild local Docker dev from the final source.
- [ ] Configure isolated count/token public aliases, channel mappings/groups, group ratios, and the Adobe quality profile without exposing secrets.
- [ ] Run count and token async tasks through local image-handle; poll terminal state and verify task, lease, wallet/subscription, log, and snapshot behavior.
- [ ] Restore or retain only the explicitly requested durable local configuration and document any environment blocker.
- **Status:** pending

## Locked Decisions
| Decision | Rationale |
| --- | --- |
| V1 has one pricing dimension: `quality`, `size`, or `resolution` | Avoids a combinatorial matrix while covering the stated providers. |
| `n` is a universal multiplier and normalized before mapping | Ensures aliases can map to one upstream model without losing count billing. |
| Pricing belongs to model settings in new-api | Channels execute requests; image-handle owns neither prices nor aliases. |
| Request-time snapshot is authoritative | Responses and config hot updates are audit data only and cannot reprice an in-flight task. |
| Unbound and snapshot-less legacy tasks stay on old billing | Maintains backward compatibility and makes unbinding restore the old configuration. |

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Planning files did not contain the active image-pricing task after context recovery | 1 | Added this task section and resumed from the existing diff and independent reviews. |
| Local count/token token groups currently have no matching abilities/mappings/group ratios or `ImagePricing` option | Runtime audit | Treat as an environment/configuration phase after code review; configure atomically before sending any paid generation request. |
| Initial combined response-format patch used stale test assertion context | 1 | Split the patch into current-file hunks; no partial write occurred, then focused tests passed. |
| Pointer conversion exposed image-pricing resolver and assertion type mismatches | 1 | Updated fallback handling, normalized writeback pointers, and pointer-aware tests; all focused packages pass. |

---

# Task Plan: Multi-level token tier pricing

## Goal
Add configurable whole-request token tier pricing, preserve legacy billing for all unconfigured models, expose auditable calculations in logs and the marketplace, and verify the implementation with unit, Docker, and authorized live upstream tests.

## Current Phase
Complete

### Phase 1: Configuration and billing core
- [x] Add rule types, built-in GPT-5.6 defaults, validation, merging, and atomic snapshots.
- [x] Add Decimal whole-request tier selection and settlement without changing legacy or fixed-price billing.
- [x] Add structured and human-readable billing audit details.
- **Status:** complete

### Phase 2: API and frontend
- [x] Add option metadata and optional marketplace pricing payloads.
- [x] Add admin tier editor with inline validation and responsive layouts.
- [x] Add marketplace badges and full tier details.
- **Status:** complete

### Phase 3: Automated verification
- [x] Add boundary, component, protocol, configuration, snapshot, and legacy regression tests.
- [x] Run focused and full Go tests, frontend build, formatting, and i18n checks.
- **Status:** complete

### Phase 4: Docker and live upstream verification
- [x] Add the repeatable secure validation script and report output.
- [x] Rebuild Docker dev and run disabled, official short, synthetic multi-tier, authorized real long-context, and streaming scenarios.
- [x] Restore configuration, audit quota deltas, and retain reports/logs.
- **Status:** complete

### Phase 5: Disabled marketplace visibility regression
- [x] Reconcile cached marketplace pricing rows with the current effective tier rule on every response.
- [x] Add enabled, disabled, re-enabled, and fixed-price response regression coverage.
- [x] Rebuild Docker dev and verify disabled rules disappear from the marketplace API and UI immediately.
- **Status:** complete

## Locked Decisions
| Decision | Rationale |
| --- | --- |
| V1 supports only `whole_request` selected by total input tokens | Matches the current OpenAI GPT-5.6 pricing rule and avoids marginal-tier misbilling. |
| Rules match exact model names and support arbitrary ordered tiers | Prevents accidental rollout while allowing future 500K+ tiers without code changes. |
| Tier pricing is additive and opt-in per effective rule | Unconfigured, disabled, and fixed-price models retain their existing behavior. |
| Actual usage selects the final tier; request-start data is immutable | Supports correct reconciliation and prevents mid-request configuration changes from altering charges. |

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Combined defaults patch used stale map alignment context | 1 | Re-read exact locations and apply narrowly scoped per-file hunks. |
| Targeted frontend lint found the edited hook lacked the repository license header | 1 | Add the same protected project header used by the adjacent editor component, then rerun checks. |
| Full i18n lint retained 424 repository-wide findings, including three new technical labels | 1 | Translate the three labels and verify feature files no longer appear; retain unrelated baseline findings. |
| Docker compose rebuild used the container name instead of the compose service name | 1 | Read `config --services` and rebuild the `new-api-dev` service. |
| Initial live validator required a Docker healthcheck on Postgres/Redis, which these containers do not define | 1 | Accept running containers without health metadata and add real `psql`/`redis-cli` readiness probes. |
| Temporary administrator access token exceeded the database `char(32)` limit | 1 | Generate a 32-character token with `token_hex(16)`; the rejected write changed no state. |
| Non-Chinese admin UI exposed empty/missing cache component labels | Visual QA | Fill existing empty cache-read translations and add cache-write translations in all seven locales. |
| Disabled tier rules remained visible in the marketplace for up to one minute | User report | Reconcile each cloned pricing response with the current effective rule instead of copying stale cached tier metadata. |

---

# Task Plan: Hide dynamic-route maximum pricing labels

## Goal
Keep aggregate child-route model pricing calculations intact while showing users only the configured price and ratio values, without "dynamic route maximum" labels.

## Current Phase
Complete

### Phase 1: Discovery, implementation, and verification
- [x] Locate all model-marketplace dynamic-route maximum labels.
- [x] Confirm `max_ratio` remains necessary for price coverage across reachable child routes.
- [x] Remove the labels from card, table, and pricing-detail views.
- [x] Run focused frontend checks and review the final diff.
- **Status:** complete

## Decision
| Decision | Rationale |
| --- | --- |
| Preserve existing `max_ratio` calculation and hide only user-facing labels | Avoids understating a dynamically routed price while removing confusing "highest price" wording. |

---

# Task Plan: Usage statistics billing split and dashboard redesign

## Goal
Separate subscription, wallet, and unknown usage accounting; add a subscription usage ranking; and redesign `/console/usage-stats` into a compact, responsive, lazily loaded tabbed dashboard.

## Current Phase
Complete

### Phase 1: Backend contract and attribution
- [x] Add section and billing-source query contracts with validation and backward-compatible defaults.
- [x] Add billing-source summary, trend, model, and subscription-ranking response fields.
- [x] Complete billing-source metadata for task, Midjourney, and violation-fee logs.
- **Status:** complete

### Phase 2: Aggregation and backend verification
- [x] Aggregate total, wallet, subscription, and unknown usage in one pass.
- [x] Implement subscription ranking and source-filtered user drill-down.
- [x] Add model/controller/log-generation regression tests.
- **Status:** complete

### Phase 3: Frontend redesign
- [x] Split the oversized page into filter, overview, ranking, funding, and detail modules.
- [x] Implement applied filters, three primary tabs, secondary tabs, and per-section request caching.
- [x] Implement responsive charts, compact metrics, unknown-source warning, and mobile table/card behavior.
- **Status:** complete

### Phase 4: Verification and delivery
- [x] Run focused/full Go tests and frontend formatting/build/i18n checks.
- [x] Run authenticated responsive browser checks against Docker dev.
- [x] Review diff scope, update planning records, and deliver.
- **Status:** complete

### Phase 5: Docker table layout audit
- [x] Rebuild and recreate Docker dev from current `main`.
- [x] Inspect every usage-statistics table at desktop and narrow viewports.
- [x] Correct incomplete table fill and unstable mobile column allocation.
- [x] Run targeted frontend checks, production build, and authenticated browser verification.
- **Status:** complete

### Phase 6: Wallet usage ranking
- [x] Add an independently sorted `wallet_ranking` to the usage aggregation response.
- [x] Add backend coverage for wallet-only membership, values, and ordering.
- [x] Add `按量消耗` after `总消耗` and preserve wallet-scoped detail drill-down.
- [x] Run focused backend/frontend checks and rebuild Docker dev.
- **Status:** complete

## Decisions
| Decision | Rationale |
| --- | --- |
| Subscription-active users require positive subscription quota in the selected period | Matches actual subscription usage rather than ownership. |
| Missing or invalid `billing_source` is `unknown` | Prevents silent wallet overstatement. |
| `section` defaults to `all` | Preserves existing API behavior while enabling lazy frontend loading. |
| No schema migration or history backfill | Existing log metadata supports one-pass classification across all databases. |
| Keep one page with overview, ranking, and funding tabs | Removes long scrolling without adding routes. |

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Initial planning-file patch was a no-op | 1 | Apply an explicit insertion before the existing first heading. |
| Used cell wait for a terminal session ID | 1 | Poll the running terminal with `write_stdin` instead. |
| Task-log test set promoted `Action` directly on `RelayInfo` | 1 | Initialize the embedded `TaskRelayInfo` fixture instead. |
| First fixture-fix patch used pre-gofmt spacing and did not apply | 1 | Read the exact formatted block and use a one-line replacement. |
| Task-log integration fixture omitted embedded channel metadata and panicked | 1 | Initialize an empty `ChannelMeta`, matching production RelayInfo initialization. |
| Frontend build imported `useIsMobile` as default | 1 | Match the existing named-export convention. |
| Full i18n lint reports 426 repository-wide hardcoded strings | 1 | Remove the five new-page findings; retain unrelated existing warnings and run targeted checks. |
| `i18n:extract` rewrote hundreds of unrelated locale entries | 1 | Mechanically reverse only the locale diff, then add feature keys with scoped patches. |
| In-app browser redirects `/console/usage-stats` to `/login` | 1 | Preserve authentication boundaries and report responsive screenshot QA as blocked until an admin signs in. |
| Cached Browser skill version path no longer existed | 1 | Locate and load the current installed browser skill version before browser work. |
| Semi Table ignored its declared `tableLayout` prop | 1 | Follow the installed implementation and enable `ellipsis` on a bounded first column, which activates fixed layout. |
| Docker health loop used zsh's read-only `status` name | 1 | Rename the loop variable to `health_status`. |
| A usage-file search used unmatched zsh globs | 1 | Use the confirmed `model/log.go` and explicit test paths instead of optional shell globs. |
| Locale lookup assumed `zh.json` existed | 1 | Enumerate the locale directory and patch the repository's actual Chinese locale filename. |
| Extended wallet fixture changed the existing `gpt-4o` model total | 1 | Update the expected wallet model quota from 100 to 450; the new aggregate was correct. |
| Full i18n lint reports 421 repository-wide hardcoded strings | 1 | Confirm no UsageStats finding is present and retain targeted locale/prettier checks. |
| Python environment has no bcrypt module | 1 | Use the project's existing Go password-hash implementation for the temporary browser account. |
| Phase 4 status patch omitted the Markdown list prefix | 1 | Read the exact line and patch `- **Status:**` with its prefix. |

---

# Task Plan: GPT cache-write configurable billing

## Goal
Recognize official OpenAI `cache_write_tokens`, bill it separately only when the model has an explicit `CreateCacheRatio` entry, preserve all legacy cache accounting, and verify the result with unit, Docker mock, and authorized live sub2api tests.

## Current Phase
Complete

## Phases
- [x] Implement usage DTO normalization and preserve explicit zero values across OpenAI relay/conversion paths.
- [x] Implement configuration-presence gating, billing split, validation, and log snapshots.
- [x] Implement frontend normalization and generic non-Claude billing display.
- [x] Independently review the complete diff and run backend/frontend regression tests.
- [x] Rebuild Docker dev and run deterministic configured/unconfigured usage scenarios.
- [x] Run authorized streaming and non-streaming live sub2api tests without exposing token secrets.
- [x] Review the final diff and report compatibility and any upstream limitations.

## Completion Audit

### Phase 1: Usage normalization
**Status:** complete

### Phase 2: Billing and logging
**Status:** complete

### Phase 3: Frontend display
**Status:** complete

### Phase 4: Automated regression
**Status:** complete

### Phase 5: Docker mock integration
**Status:** complete

### Phase 6: Live sub2api integration
**Status:** complete

### Phase 7: Final review and cleanup
**Status:** complete

## Current Decisions
| Decision | Rationale |
| --- | --- |
| Configuration-key presence is the only billing switch | Ratios `0`, `1`, and `1.25` must all count as explicitly enabled. |
| Official `cache_write_tokens` takes precedence over legacy usage | A present value, including explicit `0`, is authoritative. |
| Unconfigured or invalid official writes remain ordinary input | Prevents free input and preserves old billing behavior. |
| Legacy, Claude split-cache, and OpenRouter logic is unchanged | Limits the compatibility surface of this feature. |

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Official writes on fixed per-call pricing were labeled as ordinary-input billing even though token classes do not affect the fixed price | Independent review | Skip all new official cache-write billing/log state for `UsePrice` requests and add a regression test. |
| OpenRouter official-field precedence had no direct regression test | Independent review | Add explicit-zero and unconfigured-positive cases with nonzero upstream cost. |
| First wording patch assumed an incorrect Russian translation value and did not apply | Frontend wording cleanup | Inspect exact locale entries, then apply a narrower key-only patch. |
| A plan status patch used hunks in reverse file order and did not apply | Phase update | Reordered hunks to match file order and re-applied. |
| Initial mock search used an unmatched zsh glob (`test*`) | Docker mock discovery | Re-run with explicit `rg` include/exclude globs only. |
| First mock setup transaction passed literal `\\n` characters to `psql -c` and failed before `BEGIN` | Docker mock setup | Re-run with shell-safe single-quote escaping and verify all temporary rows/keys after commit. |
| Compact requests add the internal `-openai-compact` model suffix before channel distribution, so the first fixtures had no matching abilities | Docker Compact test | Add temporary suffixed abilities/models and matching ratios, restart, and retry. |
| In-app browser load-state helper does not support `networkidle` | UI verification | Use a fresh DOM snapshot and targeted element checks instead of repeating the unsupported wait mode. |
| Local browser session is expired when entering the authenticated console | Desktop/narrow log verification | Do not alter credentials; retain automated frontend checks and report the visual verification limitation. |
| Final phase patch again placed a later file hunk before an earlier one | Phase update | Reordered the phase-title hunk before the phase-list hunk. |
| Frontend explicit-zero fallback could prefer a stale legacy field when `reported=0` and `enabled=true` coexisted | Final frontend review | When the new reported key exists, use it exclusively for billed tokens and add a realistic enabled-zero regression test. |
| Planning completion checker did not recognize list-style phase statuses | Final plan audit | Add the required `### Phase` and `**Status:** complete` audit entries, then re-run the checker. |

---

# Task Plan: Compare GPT-5.6 cache-write billing with upstream rc.21

## Goal
Compare the fork's completed GPT-5.6 cache-write billing implementation with `QuantumNous/new-api` tag `v1.0.0-rc.21`, and explain behavioral and implementation differences without changing product code.

## Current Phase
Complete

### Phase 1: Resolve versions and change scope
- [x] Fetch and identify the upstream tag commit.
- [x] Identify the fork's relevant commits and files.
- **Status:** complete

### Phase 2: Compare implementations
- [x] Compare usage DTO normalization, billing, logs, frontend display, and tests.
- [x] Trace concrete behavioral differences and edge cases.
- **Status:** complete

### Phase 3: Verify conclusions
- [x] Run focused tests or static checks where needed.
- [x] Produce an evidence-backed Chinese summary with file references.
- **Status:** complete

## Comparison Questions
1. Does upstream preserve absent versus explicit-zero `cache_write_tokens`?
2. What enables separate cache-write billing upstream: model family, ratio presence, or another switch?
3. How do invalid/unconfigured write-token values affect ordinary input billing?
4. Are streaming, non-streaming, Responses, Chat, Compact, OpenRouter, logs, and frontend display covered equally?

## Comparison Errors
| Error | Attempt | Resolution |
| --- | --- | --- |
| GitHub connector tools are unavailable in this session | 1 | Use read-only local `git` fetch/show/diff against the public tag, as allowed by the GitHub skill fallback. |
| `gh` CLI is not installed | 1 | Use the public GitHub REST endpoint with `curl` for release metadata; continue using the deepened temporary clone for commit history. |

---

# Task Plan: Merge upstream GPT-5.6 cache-write accounting

## Goal
Retain the fork's explicit-zero semantics, configuration gating, audit data, and frontend compatibility while adopting upstream overlap-aware GPT-5.6 cache-write accounting; hide missing and zero writes in visible logs and verify with unit plus Docker dev tests.

## Current Phase
Complete

### Phase 1: Design and baseline
- [x] Confirm desired visible-log behavior for missing and zero writes.
- [x] Inspect current billing, frontend, and Docker fixtures.
- **Status:** complete

### Phase 2: Implementation
- [x] Replace the non-cached-input rejection with overlap-aware accounting and bounded malformed-value protection.
- [x] Add GPT-5.6 default creation ratios where compatible with local configuration semantics.
- [x] Hide zero/missing cache writes from visible log summaries while preserving backend explicit-zero precedence.
- **Status:** complete

### Phase 3: Automated verification
- [x] Add/update backend regression tests for official overlapping-prefix fixtures and boundary cases.
- [x] Add/update frontend tests for hidden missing/zero visible logs.
- [x] Run focused and broad Go/Bun checks.
- **Status:** complete

### Phase 4: Docker dev verification
- [x] Rebuild Docker dev.
- [x] Run deterministic configured overlap, missing, zero, unconfigured, and malformed scenarios.
- [x] Audit logs, quota totals, cleanup, and final diff.
- **Status:** complete

## Decisions
| Decision | Rationale |
| --- | --- |
| Preserve `*int` and raw explicit-zero state | Explicit zero must override stale legacy creation fields and suppress inference. |
| Hide missing and zero only in visible log UI | Meets the user-facing requirement without weakening billing semantics. |
| Keep explicit ratio-key gating | Retains operator control and prevents new token classes from changing unconfigured-model prices. |
| Adopt overlap-aware base calculation | GPT-5.6 read and write prefix counts can legitimately overlap. |

## Errors
| Error | Attempt | Resolution |
| --- | --- | --- |
| `rg` included nonexistent root `package.json` and exited 2 after still finding the frontend scripts | 1 | Use the confirmed `web/package.json` scripts directly; do not repeat the invalid root path. |
| Full `bun run lint` scans generated `web/dist` and 111 pre-existing unformatted source files; concurrent build also replaced dist files during the scan | 1 | Do not modify unrelated files. Run Prettier `--check` only on the four touched frontend files after the build completes. |
| Docker configured-overlap, unconfigured-overlap, and oversized requests returned HTTP 403 while zero/missing succeeded | 1 | Inspect response bodies, user quota, model access, and pre-consumption state before changing fixtures; do not repeat the same requests blindly. |

---

# Task Plan: ImageHandle edit upload compatibility

## Goal
Support image-handle's upload-before-edit contract for gray-enabled synchronous image edits. `/v1/images/edits` must keep old direct-upstream behavior when the switch is off; when sync image-handle execution is enabled, URL inputs should submit directly, multipart file inputs should upload through `/v1/image/uploads`, and JSON base64/data-URI inputs should upload through `/v1/image/uploads/base64` before submitting the edit task.

## Current Phase
Complete

## Phases
- [complete] Read updated image-handle docs for `/v1/image/uploads` and `/v1/image/uploads/base64`.
- [complete] Map current new-api image edit multipart/base64 parsing and sync image-handle decision point.
- [complete] Implement upload-to-URL normalization for sync image-handle edit requests.
- [complete] Add unit tests for URL, multipart, base64/data-URI, and switch-off behavior.
- [complete] Run Go/frontend regression checks.
- [complete] Build Docker dev and联调 switch-off, sync URL edit, sync multipart edit, sync base64 edit, and URL/base64 output formats.

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
| Edit sync should now normalize multipart/base64 through image-handle uploads | image-handle added upload endpoints so non-URL edit inputs can still execute in image-worker without queueing large images. |
| Channel override lives in `channels.settings` | `channels.other` is legacy; UI and backend read/write `settings` for `image_handle_sync_mode` and `callback_secret`. |

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Old docs and plan still described `new_api_internal execute` | Code review | Rewrote the integration doc and plan to `provider_direct_lease`. |
| Local image-handle mock edit route returned 415 for multipart edits | Docker联调 | Verified URL edit input reached image-handle sync and new-api refunded on failed terminal status; non-URL multipart edit correctly fell back to direct upstream. The 415 is a mock-new-api multipart parser limitation, not a new-api routing issue. |
| Channel `force_on` did not appear to work during first SQL test | Docker联调 | Test SQL wrote `image_handle_sync_mode` to legacy `channels.other`; corrected to `channels.settings`, matching frontend/backend field usage. |
| image-handle 202 processing could not be triggered safely in Docker | Docker联调 | Current local `SYNC_TASK_TIMEOUT_MS` is 300s. Added unit coverage for HTTP 202 -> `image_handle_sync_timeout`; did not wait 300s in Docker. |
| Local mock upstream does not support multipart `/v1/images/edits` | Docker联调 | Verified new-api upload-to-URL and image-handle sync task submission; final edit result fails in worker with 415 because mock-new-api Fastify lacks multipart content parser for edits. |
# Task Plan: Claude `Content block not found` analysis

## Goal
Determine why requests using `claude-fable-5` frequently fail with `API Error: Content block not found`, correlate the failure with this repository's Claude relay implementation and public/official protocol evidence, and give evidence-backed diagnosis and mitigation guidance without changing product code.

## Current Phase
Complete

### Phase 1: Repository discovery
- [x] Find the exact error string and all Claude request/stream conversion paths.
- [x] Trace `claude-fable-5` model mapping, channel selection, retries, and tool/content-block handling.
- **Status:** complete

### Phase 2: Protocol and public-source research
- [x] Compare the implementation with Anthropic official Messages and streaming event invariants.
- [x] Check public reports for the same error and identify provider/proxy-specific patterns.
- **Status:** complete

### Phase 3: Synthesis and verification
- [x] Rank likely root causes and distinguish upstream/provider errors from local conversion errors.
- [x] Identify concrete logs or request/response evidence that can confirm each hypothesis.
- [x] Deliver mitigations and, if warranted, scoped code-fix suggestions without modifying code.
- **Status:** complete

## Key Questions
1. Which component emits the literal `Content block not found` message?
2. What invalid event/content-block sequence can produce it?
3. Is `claude-fable-5` an official Anthropic model name or a mapped/provider alias?
4. Which repository behaviors can make the issue frequent rather than random?

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Existing planning files contain unrelated active work | 1 | Append a separate scoped task section and preserve all prior content. |
| `git status` was passed a deleted `/tmp` path outside the repository | 1 | Treat as a harmless diagnostic command error and inspect only repository paths thereafter. |
| Initial read-only PostgreSQL metadata queries lost SQL string quoting through nested shell quoting | 1 | Log the error and retry with quote-free metadata queries, filtering safe output outside `psql`. |

---
# Task Plan: Async Image Open API and Webhook (2026-07-17)

## Goal
Normalize the public async image task API, make new-api dispatch to image-handle durably, add user-configured outbound Webhooks, expose scoped management APIs, and verify the complete local Docker workflow.

## Current Phase
Complete with one documented billing-atomicity follow-up

### Phase 1: Persistence and public task contracts
- [x] Add cross-database image request/dispatch and Webhook models and migrations.
- [x] Add normalized create/get/list/batch/upload APIs with user isolation and idempotency.
- [x] Add durable new-api to image-handle dispatch and unified terminal transitions.
- **Status:** complete

### Phase 2: Webhook management and delivery
- [x] Add endpoint/event/delivery/attempt services, signed delivery worker, retries, cleanup, and SSRF controls.
- [x] Add session and scoped ak_ management APIs, secret rotation, test events, logs, and manual retry.
- **Status:** complete

### Phase 3: Resource Center and documentation
- [x] Add Webhook UI, API key scopes, delivery logs, one-time secrets, and OpenAPI documentation.
- [x] Update all frontend locales.
- [x] Verify responsive behavior in the rebuilt Docker dev UI.
- **Status:** complete

### Phase 4: image-handle contract and local integration
- [x] Add request fingerprint/provider_options/URL security changes in image-handle.
- [x] Join both Docker dev stacks through ai-gateway and run end-to-end scenarios.
- **Status:** complete

### Phase 5: Verification
- [x] Run focused and full Go, image-handle, frontend, i18n, compose, and cross-database checks.
- [x] Review final diffs and document any environmental limitations.
- **Status:** complete

## Locked Decisions
- new-api is the only public task and third-party Webhook boundary; image-handle remains internal.
- Public task IDs are server-generated; Idempotency-Key is optional and request-fingerprint protected.
- Async edits use URL inputs with new-api multipart/base64 pre-upload endpoints.
- Public statuses are queued, in_progress, succeeded, and failed; cancellation is out of scope.
- Webhooks broadcast terminal task events to up to five subscribed endpoints and retain delivery logs for seven days.
- Webhook management Open API uses scoped ak_ keys; existing keys remain assets:read only.
- Endpoint secret rotation emits old and new signatures for 24 hours.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Existing planning files contain several prior task records | 1 | Append a separate task section and preserve all prior content and status. |
| Used the yielded exec session id with the cell-wait helper | 1 | Resume the PTY session with write_stdin; focused model/service tests passed. |
| Controller build retained an unused errors import after standardizing task-not-found responses | 1 | Remove the obsolete import and rerun the affected packages before continuing. |
| Focused controller tests did not migrate `image_task_requests` and still asserted legacy HTTP-200 error envelopes | 1 | Update the test schema/fixtures and assertions to the normalized public API contract before rerunning controller tests. |
| Adding terminal Webhook creation rolled back legacy test tasks whose intentionally minimal schemas lacked the new tables | 1 | Treat absent extension tables as a rolling-upgrade compatibility case and skip event creation until migrations are present. |
| Secret-rotation test found the returned one-time secret used a version one higher than the persisted endpoint after GORM synchronized map updates into the struct | 1 | Snapshot old salt/version before the update and assign the exact persisted new version when constructing the response secret. |
| Durable image API tests had been added but not formatted or executed before the context handoff | 1 | Format the test file and run controller/relay/service/model/middleware/router packages; all passed. |
| Temporary locale merge script used unquoted Chinese keys containing punctuation, then an initial repair malformed the Russian secret row | 2 | Quote every punctuated key, restore the exact Russian key/value, syntax-check before executing, and remove the temporary script after the structured merge. |
| Phase 3 status patch expected the status marker without its Markdown list prefix | 1 | Read the current section and reapply the script deletion/status update with the exact `- **Status:**` context. |
| Initial Redocly OpenAPI 3.1 validation was valid but reported nine schema/operation warnings | 1 | Define conditional required properties in their local schemas and add operation IDs plus explicit unauthenticated security to outgoing Webhook operations. |
| Dispatch terminal tests left their new ImageTaskRequest row behind, causing the next SQLite fixture to reuse task record ID 1 and hit its unique index | 1 | Delete dispatch request/event rows before deleting the task in the fixture cleanup, then rerun the real terminal path. |
| Current Docker Compose CLI rejected `config --networks` as an unknown flag | 1 | Use validated compose config plus `docker network inspect` and container-level network/DNS probes; do not repeat the unsupported flag. |
| Docker build could not copy `docs/openapi` because `.dockerignore` excluded the entire docs directory | 1 | Keep docs excluded by default but explicitly include `docs/openapi/**`, then rebuild from the corrected context. |
| Local channel inventory query assumed a nonexistent `group_name` column | 1 | Use the actual reserved `group` column with PostgreSQL quoting, matching the repository's cross-database convention. |
| A combined source read looked for image-handle's mock script from the new-api repository | 1 | Read the script from the image-handle workdir on the next inspection; no product command was affected. |
| Inline Node E2E script passed escaped newlines as literal `\\n` and failed before making requests | 1 | Pass the same structured-fetch script as a single line, avoiding shell newline interpretation. |
| First Docker task fixture used an unregistered custom group and was rejected with HTTP 403 before task creation | 1 | Use the valid `default` group with a unique temporary public model alias, model mapping, ratio, and ability so no real channel can match. |
| Manual retry was requested while the first failed HTTP attempt correctly left delivery in scheduled `pending` state | 1 | Verify manual retry from an explicit terminal `failed` fixture state; verify automatic retry separately by advancing the due time. |
| A repository-root diagnostic used the unmatched glob `deploy/README*` under zsh | 1 | Read the confirmed Compose files directly from each repository; no product command or file was affected. |
| Two planning-record patches used either summary wording or a missing file boundary instead of exact local context | 2 | Re-read the active section and apply correctly separated, narrowly anchored patches; neither failed attempt made a partial write. |
| Full `bun run i18n:lint` reports 422 pre-existing hardcoded-string findings across the frontend | 1 | Preserve the existing baseline, verify all introduced locale keys across seven locales, and run targeted formatting/lint checks on the changed Resource Center files. |
| The first PostgreSQL integration DSN targeted Docker-only port 5432 from the host | 1 | Use an isolated temporary PostgreSQL container with an explicit loopback port; the test passed and the container was removed. |
| Official MySQL 5.7 has no arm64 manifest and its first emulated readiness check raced host port forwarding | 2 | Pull/run the amd64 image, wait for container health plus a real TCP query, then execute the integration test. |
| MySQL 5.7's default `latin1` test schema rejected the Unicode payload fixture | 1 | Start MySQL with `utf8mb4`, matching new-api's existing `checkMySQLChineseSupport` startup requirement; the Unicode/TEXT contract passed. |
| The 375px browser screenshot capture timed out after viewport calibration | 1 | Do not repeat the capture; verify the exact 375x812 DOM geometry and overflow metrics, using the successful 560px mobile screenshot for visual inspection. |
| The first locale completeness script read keys from the JSON root instead of its `translation` object | 1 | Inspect the actual locale structure and rerun; all 63 Webhook/scope keys exist in all seven locales. |
| The initial image-handle cleanup query referenced nonexistent `image_tasks.task_id` | 1 | Query and delete by the real `client_task_id`; no mutation occurred in the failed read-only query. |
| Final changed-file JSON scan used a temporary-file cleanup command rejected by the command safety policy | 1 | Replace it with a read-only Git file list piped directly to `rg`; only the approved `common/json.go` wrapper contains direct JSON calls. |
| Final receiver verification requested unsupported `GET /config` and received 404 | 1 | Use the successful `POST /config` response (`secret_configured:false`) plus `GET /events` (`attempts:0`, `received:0`) as the supported verification contract. |
| The generic planning completion script reported two pending phases | 1 | Confirm they belong to an older image-pricing plan in the shared planning file; all five phases of the active async-image/Webhook plan are complete. |

---
# Task Plan: Complete Resource Center API Examples (2026-07-18)

## Goal
Give every Resource Center endpoint a complete, copyable request example and representative success response, while keeping the documentation easy to scan on desktop and mobile.

## Current Phase
Complete

### Phase 1: Contract audit
- [x] Map all 11 documented operations to their real OpenAPI request and response shapes.
- [x] Identify missing examples and reusable presentation patterns.
- **Status:** complete

### Phase 2: Documentation implementation
- [x] Add complete curl and response examples for all async image and asset operations.
- [x] Keep generation JSON, edit URL JSON, and multi-file multipart edit examples distinct.
- [x] Add concise parameter guidance where query behavior is not obvious.
- **Status:** complete

### Phase 3: Verification
- [x] Run formatting/lint checks, OpenAPI validation, frontend build, and diff checks.
- [x] Rebuild Docker dev and visually inspect the documentation at desktop and mobile widths.
- **Status:** complete

## Locked Decisions
- The endpoint table remains a compact overview; executable examples live in per-operation sections below it.
- Every operation gets both a request and representative success response, including bodyless GET/export operations.
- Examples use the same Resource API Key environment variable and are copyable without placeholder restructuring.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| The first combined locale patch assumed ASCII French text and mismatched the existing accented translation | 1 | The patch was atomic and changed no locale; inspect each locale tail and reapply with exact existing context. |
| The OpenAPI script discovery search included root-level `package.json` and `scripts` paths that do not exist | 1 | Read the actual `web/package.json`; use its `openapi:check` script from the `web` directory. |
| The first Compose discovery command used a zsh glob with no `compose*.yml` match | 1 | Use `rg --files` with compose filename patterns before selecting the dev Compose file. |
| Full i18n lint reported the repository's existing 421-item hardcoded-string baseline | 1 | Confirm the changed documentation file is absent from findings and all new keys exist in every locale; keep targeted ESLint green. |
| Browser QA completed, then the browser runtime's unrelated telemetry POST timed out | 1 | Treat as browser-tool telemetry noise; the local DOM assertions had already completed successfully and the app reported no relevant error. |
| The generic planning completion script reported two pending phases | 1 | Confirm they belong to an older image-pricing task at lines 273-283; all three phases of the active Resource Center documentation task are complete. |

---
# Task Plan: Automatic Error Snapshots and Dump Management (2026-07-20)

## Goal
Add bounded, non-blocking automatic relay error snapshots with runtime settings and a dedicated management experience inside the existing Dump page, while preserving relay, fallback, billing, and temporary Dump behavior.

## Current Phase
Complete

### Phase 1: Backend storage, capture, and APIs
- [x] Add hot settings, bounded gzip storage, cleanup, reconciliation, and GORM index.
- [x] Capture failed relay attempts, including fallback-hidden and Claude stream integrity failures.
- [x] Add permission-protected status, settings, list, detail, download, delete, cleanup, and clear APIs.
- **Status:** complete

### Phase 2: Dump management UI and translations
- [x] Preserve the existing temporary Dump experience under a top-level tab.
- [x] Add error snapshot status, settings, filters, paginated attempts, detail SideSheet, download, cleanup, and deletion controls.
- [x] Add all new strings to zh-CN, zh-TW, en, fr, ru, ja, and vi locales.
- **Status:** complete

### Phase 3: Boundary and regression coverage
- [x] Add capacity/file-count cleanup and queue-full nonblocking tests.
- [x] Review fallback outcome ordering and post-commit stream capture coverage.
- [x] Run full backend and frontend verification.
- **Status:** complete

### Phase 4: Docker and responsive acceptance
- [x] Rebuild Docker dev and validate runtime setting changes and failure capture at port 3001.
- [x] Inspect desktop and mobile layouts and restore test configuration.
- **Status:** complete

## Locked Decisions
- Automatic capture is disabled by default and adds only a fast settings check while disabled.
- Summary snapshots never persist prompts; priority user/channel matches add sanitized client and upstream request bodies.
- Snapshot payloads are capped at 128 KiB before gzip and writes use a fixed 32-item non-blocking queue.
- Local disk storage and a bounded database index are intentionally single-instance for this change.
- The existing temporary Dump implementation and API behavior remain unchanged.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Existing planning files contain completed historical tasks | 1 | Prepend a separate current task section and preserve all prior records. |
| Full i18n lint reports a large repository baseline | 1 | Remove the three new-component hardcoded labels, retain the unrelated baseline, and verify all page keys exist in all seven locales. |
| Full frontend ESLint scans generated `dist` and existing source files without required headers | 1 | Record the 68-item repository baseline; targeted ESLint for both changed Request Dump components passes. |
| Scoped Go vet reaches an existing self-assignment in `model/invite_code.go` | 1 | Record the unrelated existing finding; focused and full Go tests pass. |
| Mobile browser exact-name selector did not match the filter toggle because the icon name is included in its accessible label | 1 | Use a partial accessible-name match for the same visible button; no application failure occurred. |
| `go test ./service` failed after adding multipart metadata and secret-assignment tests, while verbose unrelated package logs hid the failing assertion | 1 | JSON-filtered output identified an assertion against escaped envelope bytes; decode the envelope before checking the inner body. The full service package then passed. |
| Generic planning completion check reports two pending phases | 1 | Confirmed both are from the older image-pricing task at lines 274-283; all four phases of the active Error Snapshot task are complete, so preserve the historical task state. |

---

# Task Plan: Per-user Aggregate Route Model Ratios (2026-07-21)

## Goal
Add per-user aggregate child-route exact-model ratios that override global child-route rules, while exposing only final effective ratios to ordinary users.

## Current Phase
Complete

### Phase 1: Data contract and resolver
- [x] Add backward-compatible user-setting rule storage and validation.
- [x] Apply user exact rules before global exact rules and preserve source in task snapshots.
- [x] Update pricing aggregation to use the same precedence.
**Status:** complete

### Phase 2: Admin APIs and user-facing privacy
- [x] Extend the existing user ratio GET/PUT contract and add scoped model lookup.
- [x] Remove original/override metadata from ordinary user group, pricing, and log responses.
**Status:** complete

### Phase 3: Frontend administration and display
- [x] Add default-ratio and child-route-model tabs to the existing user SideSheet.
- [x] Render only final ratios in pricing, token, and Playground surfaces.
**Status:** complete

### Phase 4: Verification
- [x] Run focused and full backend/frontend test suites.
- [x] Rebuild Docker dev, verify precedence and privacy, perform responsive browser QA, and clean fixtures.
**Status:** complete

## Locked Decisions
- Precedence is user exact route/model, global exact route/model, user aggregate default, aggregate default.
- Exact model names remain case-sensitive; zero is valid; disabled rules fall through.
- User rules remain in the existing user setting JSON, so no database migration is added.
- Ordinary user APIs and logs expose only final effective ratios; administrator management and logs retain audit metadata.
- Existing planning sections and unrelated untracked diagnostics remain untouched.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Full i18n lint still reports the repository's pre-existing hardcoded-string baseline | 1 | Confirm the feature adds no new finding, validate all new locale keys, and keep changed frontend files clean with targeted ESLint and Prettier. |
| Browser telemetry to Statsig timed out during local UI acceptance | 1 | Treat it as external browser-tool telemetry; local page DOM, network contracts, console timing, and application health checks completed successfully. |

---
# Task Plan: Resource Center DTO Documentation (2026-07-24)

## Goal
Make every Resource Center API operation self-contained by documenting request and response fields, types, requiredness, constraints, enums, and nested objects from the OpenAPI 3.1 contract without changing runtime API behavior.

## Current Phase
Complete

### Phase 1: Contract audit
- [x] Inventory every operation presented in API Overview, Async Images, Async Videos, and Resource API.
- [x] Compare the generated OpenAPI schemas with backend DTOs/controllers and record missing field metadata.
**Status:** complete

### Phase 2: OpenAPI and UI implementation
- [x] Complete request, success response, and shared error schemas in the OpenAPI generator.
- [x] Add reusable responsive schema-definition rendering beside each operation's examples.
- [x] Add only the presentation translations required across all seven locales.
**Status:** complete

### Phase 3: Verification
- [x] Run OpenAPI drift/validation, targeted frontend formatting/lint, production build, i18n checks, and diff checks.
- [x] Rebuild Docker dev and inspect desktop, 560px, and 375x812 documentation layouts for overflow and completeness.
**Status:** complete

### Phase 4: Field-table visibility follow-up
- [x] Remove the outer schema collapse so every operation shows its field definitions directly.
- [x] Split the table into Name, Type, Required, Description, and Notes while retaining responsive mobile rows.
- [x] Rebuild Docker dev and verify the visible tables on desktop and 375x812.
**Status:** complete

## Locked Decisions
- OpenAPI 3.1 is the single source of truth for field definitions; the React page must not maintain a second hand-written DTO catalog.
- Preserve existing curl and response examples and place request/response definitions close to each operation.
- Do not change API behavior or authentication boundaries in this documentation-only task.
- Keep ordinary API Token (`sk-`), Resource API Key (`ak_`), and Webhook Key (`wk-`) terminology distinct.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Existing planning files contain extensive completed and unrelated work | 1 | Append a separate current task section and preserve every existing section and workspace artifact. |
| Docker health loop used zsh's read-only `status` variable | 1 | Rename the task-local variable to `health_state` before repeating the health check. |
| Browser section text filter matched ancestor sections, and the supported locator `has` filter failed inside the browser client | 2 | Stop refining text/role locators; resolve the exact section index with one read-only DOM query, confirm the section count, then use the permitted indexed locator. |
| An initial Chinese-description audit checked unused OpenAPI top-level metadata and outbound operation prose, reporting 30 irrelevant missing extensions | 1 | Restrict the assertion to every schema, parameter, request body, success response, and header the dashboard renderer can display; the resulting missing count is zero. |
| Reused browser capability bindings targeted a finalized older tab, so the first responsive override and one scoped locator evaluation did not apply to the active page | 2 | Obtain fresh uniquely named bindings, use the tab-scoped CDP fallback for the exact mobile viewport, refresh the DOM state, and complete the 375x812 geometry check without retrying stale locators. |
| The first UI/UX recommendation command used the short skill alias as if it were a real scripts directory | 1 | Resolve and run the installed skill from `/Users/zhangyu/.agents/skills/ui-ux-pro-max`; the data-dense documentation-table guidance completed successfully. |

---
## 2026-07-29 — Claude 渠道“空任务回复”诊断

### Goal

基于客户测试脚本、两张截图和当前 `new-api` 代码，定位“完整计费但回复 `I'm ready. What would you like me to work on?`”以及“整条 SSE 只有 ping”的成因；明确已确认事实、合理推测和仍缺失的上游证据。当前任务仅诊断，不修改业务代码。

### Phases

- [completed] 读取客户脚本并还原请求、并发和响应判定逻辑
- [completed] 追踪 Claude 请求转换、SSE 转发和 usage 处理链路
- [completed] 用截图统计与代码行为验证候选原因
- [completed] 汇总结论、证据边界及下一步取证方式

### Scope / constraints

- 保留工作树内所有既有未跟踪文件。
- 遵守项目跨数据库、JSON wrapper 和受保护标识规则。
- 不在未获修复授权的情况下修改业务代码。

### Errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| 在 macOS 临时目录递归查找样本时遇到多个系统目录 `Operation not permitted` | 1 | 不扩大权限；后续只检查 `TMPDIR` 根目录及已知精确路径 |
| `go test -race ./common ./relay/channel/claude` 失败 | 1 | race detector 发现 legacy stream `stopChan` send/close 竞争，以及并行 Claude settings 读写竞争；改用单测隔离并按与客户现象的相关性分别判断 |
| 通用 planning completion check 报 2 个 pending phase | 1 | 两项均属于共享文件内较早的 image-pricing 任务（约 1012–1021 行）；保留其历史状态，本次诊断四阶段均已完成 |
## 2026-07-29 — Claude 空流/空任务回复公开讨论检索

### Goal

检索 new-api 官方仓库、sub2api 官方仓库，以及 X/技术论坛中是否存在与以下现象相似的公开报告：Claude SSE HTTP 200 但只有 ping/空输出、`Content block not found`/缺失 content block、模型返回 `I'm ready. What would you like me to work on?`、疑似模型映射或换模。区分“相似症状”“同一代码路径”和“已确认同一根因”。

### Phases

- [completed] 确认 new-api 与 sub2api 官方仓库及检索关键词
- [completed] 检索 new-api Issues/PRs/Discussions 与相关提交
- [completed] 检索 sub2api Issues/PRs/Discussions
- [completed] 检索 X、Reddit、V2EX、技术论坛及其他网关项目
- [completed] 汇总可核验链接、时间、相似度与证据边界

### Constraints

- 只做公开信息的只读检索。
- 不把标题相似或单条用户描述直接认定为同一根因。
- 优先引用原始 Issue/PR/帖子，搜索摘要仅用于发现线索。

### Errors

| Error | Attempt | Resolution |
|---|---:|---|
| 本机无 `gh` CLI，四个 `gh api` 元数据查询均返回 `command not found` | 1 | 不重复；改用 GitHub connector 读取评论、GitHub 公共 REST API 读取 Issue 元数据 |
| LINUX DO Discourse JSON 端点返回非 JSON 拦截页，无法直接读取两个主题正文 | 1 | 不绕过站点限制；只引用 Exa 可公开提取的标题/正文片段，并降低标题-only 结果的证据等级 |
| X 和 V2EX 定向搜索未返回可核验的具体同症状帖子 | 1 | 明确记录为“本次未命中”，不将其解释为平台上不存在相关讨论 |

## 2026-07-29 — Claude 可疑成功响应被动采集

### Goal

对所有 Claude 成功响应被动检测空任务式问候，仅在命中时通过现有自动错误快照存储诊断证据；不得重试、改写客户响应、改变计费或计入渠道失败。

### Phases

- [completed] 梳理 Claude 非流式、legacy 流式和完整性保护流式路径
- [completed] 实现可疑回复匹配、成功快照构建和有界证据采集
- [completed] 在 Dump 分析页面清晰展示“可疑成功”
- [completed] 添加聚焦测试并完成后端与前端验证

### Locked decisions

- 覆盖所有 Claude 成功响应；只在命中规则时落盘。
- 第一版只匹配空任务式待命回复，不将命中视为错误。
- 不触发重试，不修改 HTTP/SSE，不记录渠道 RPM 失败。
- 复用现有错误快照的脱敏、TTL、容量、文件数和下载能力。
- 匹配客户可见文本，不使用包含 thinking 的聚合文本。

### Errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| 当前工作树含既有规划文件改动和大量无关文件 | 1 | 仅向规划文件追加本任务小节，业务修改严格限定在相关模块 |
| 首次跨三个共享规划文件的追加补丁因末尾上下文混淆而未应用 | 1 | 分别读取各文件真实末尾，再使用独立上下文追加 |
| 首次业务大补丁因 `error_snapshot.go` 常量对齐上下文不匹配而未应用 | 1 | 拆成模型、快照基础结构、新文件和 handler 四组小补丁 |
| 首次页面大补丁因 JSX 长文案换行上下文不匹配而未应用 | 1 | 按标签映射、列表列、详情区和文案拆分应用 |
| 首次七语言 Banner 批量替换因现有繁体译文不匹配而未应用 | 1 | 读取七个精确现值后重新应用 |
| `bun run i18n:lint` 返回 441 条 | 1 | 与仓库既有基线完全一致；本次 ErrorSnapshots 页面没有新增 lint finding |
| 相关 Go 包首次完整回归有一个新测试失败 | 1 | 热路径优化后测试夹具未启用 diagnostics；修正夹具，不回退“开关关闭时不复制回复”行为 |

## 2026-07-29 — Claude 客户令牌实流量复现与诊断快照验收

### Goal

使用客户提供的 `gwprobe` 与 6 个临时令牌，对目标网关执行受控 Claude 并发重放，尝试复现空任务问候或 HTTP 200 空流；在本地 Docker dev 中验证本次“可疑成功”采集能否保存足以分析渠道、模型映射、请求、回复和 SSE 的证据。不得把凭据写入仓库、规划文件或持久日志。

### Phases

- [completed] 审计脚本、样本、目标端点和 Docker dev 拓扑，设计不泄露凭据的实验矩阵
- [completed] 建立基线并使用 6 个临时令牌进行受控 burst 复现
- [completed] 在本地 Docker dev 中启用自动错误快照并验证命中采集链路
- [completed] 分析请求、渠道、模型与 SSE 证据，并修正完整性失败快照的 SSE/流式元数据缺口
- [completed] 完成回归验证、清理临时凭据与实验夹具并汇总结论

### Locked decisions

- 临时令牌仅通过进程环境或权限为 `0600` 的仓库外临时文件注入，任何输出都不得包含完整令牌。
- 六个上游渠道统一使用用户确认的 `https://hk.supertoken.cc`，本地客户端只接触隔离的本地测试令牌。
- 第一阶段只复现和取证；不因可疑回复自动重试，不改写客户响应，不改变计费和渠道失败状态。
- 不能仅凭问候语断言换模；需结合 requested upstream model、response-reported model、渠道和原始 SSE 判断。
- 控制并发和轮数，先小样本确认端点与脚本行为，再扩大实验，避免不必要计费。

### Errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| 通用 planning completion check 在上一任务结束时仍报告 2 个 pending phase | 1 | 已确认属于共享文件内较早的 image-pricing 历史任务；本轮新增独立阶段，不篡改历史状态 |
| 首次跨三个规划文件追加本轮计划时 `progress.md` 锚点不匹配 | 1 | 补丁未产生部分写入；分别读取三个文件真实末尾并改用独立追加 |
| 默认样本 `/tmp/gwprobe-sample.json` 不存在，且当前环境未设置目标端点 | 1 | 不发出盲目请求；先从现有诊断材料或 Claude CLI 生成脱敏样本，并只解析可核验的目标端点配置 |
| 客户脚本 `capture` 在 60 秒内未捕获 Claude CLI 请求 | 1 | 不原样重试；脚本丢弃 CLI 输出，改为使用同一无凭据本地端点直接运行 CLI，读取实际错误后选择现有样本或兼容的抓样方式 |
| Claude Code `2.1.220` 使用 dummy 本地 capture 配置时在请求发送前持续 `401 authentication_failed` | 1 | 第 6 次自动重试时人工终止，确认费用与 usage 均为 0；停止此路径，改用仓库现有脱敏 Claude 请求样本 |
| 错把生成本机 Claude CLI capture 样本当成实流量验收前置步骤 | 1 | 用户澄清后纠正：6 个临时令牌应作为本地 Docker dev 的上游渠道凭据，调用本地 `/v1/messages`，使本次诊断代码位于实际请求链路中 |
| 用户纠正测试拓扑后的首次规划补丁锚点不匹配 | 1 | 补丁未产生部分写入；读取真实末尾后追加正确拓扑和 `hk.supertoken.cc` |
| 令牌元数据只读查询误用了不存在的 `models_enabled` 列 | 1 | 根据 `\\d tokens` 改用 `model_limits_enabled`；查询未产生任何数据库修改 |
| error_snapshots 只读查询误用了不存在的 `file_name` 列 | 1 | 根据真实 schema 改用 payload_path 等字段；快照写入本身未受影响 |
| 12 并发保留中途 `role:system` 全部被本地 Claude DTO 400 拒绝 | 1 | 该非标准 role 未到上游且未计费；按客户脚本的 `keep-system=false` 对照继续，记录生产/入口差异 |
| 12 并发大输入在本地预扣费阶段有 11 条因管理员组倍率 99 返回 403 | 1 | 只有 1 条到达上游且成功；不改全局倍率，改把隔离探针 token 切到 `default` 组并刷新精确 token 缓存 |
| 探针 token 切到 `ccmax-yimo` 后因管理员无该组权限返回 403 | 1 | 未到上游；改用 `UserUsableGroups` 明确允许且倍率为 1 的 `local-adobe2api`，精确删除该 token Redis 缓存 |

## 2026-07-30 — AdobeVideo 异步参考图片

### Goal

让 Adobe2API `/v1/videos` 与 new-api `/v1/video/tasks` 的 AdobeVideo 渠道支持 Seedance 异步参考图片，同时保持现有纯文本请求、按秒计费、异步轮询和 Asset 下载契约不变。

### Phases

- [completed] 核对 Adobe2API Chat 参考媒体能力、异步 DTO/worker 和 new-api adaptor 限制
- [completed] 扩展 Adobe2API 异步请求、校验、worker 参考图加载与测试
- [completed] 扩展 new-api AdobeVideo 标准化映射、provider options、mock 与测试
- [completed] 运行两仓库全量回归、Docker mock 验收和差异检查

### Locked decisions

- V1 只桥接参考图片，不同时扩展视频/音频参考。
- 统一接口继续使用 `input.image` 和 `reference_images`；顺序为主图在前，其余参考图随后。
- `provider_options.adobe_video.reference_mode` 显式支持 `frame` 与 `media`，默认 `frame`。
- `frame` 最多两张，分别为首帧和尾帧；`media` 最多九张普通参考图。
- V1 只接受 URL/Data URL，不接受 `provider + file_id`，不得静默丢弃不支持的来源。
- 参考图片不改变按秒计费数量，不增加分辨率倍率。
- Adobe2API 在异步 worker 内下载、验证和上传素材，提交接口本身保持快速返回 202。

### Errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| session catch-up 只发现上一轮已中断的 `GWPROBE_CAPTURE` 指令 | 1 | 判定与本任务无关；以当前用户请求和现有规划文件为准继续，不恢复旧请求 |
| 首次跨三个共享规划文件追加补丁因末尾上下文已变化而失败 | 1 | 补丁原子失败且无部分写入；读取三个文件真实末尾后分别追加 |
| 本机 Python 环境缺少 FastAPI，无法直接导入 Seedance 测试 | 1 | 重建 Adobe2API Docker 镜像并在镜像依赖环境中运行完整测试 |
| 首次验收查询把公开字符串 `task_id` 当成 tasks 主键 `id` | 1 | 根据真实表结构改查 `tasks.task_id`，任务本身和后台轮询未受影响 |
| 首次用创建任务的 `sk-` token 查询 `/v1/video/tasks/{id}` 返回 401 | 1 | 核对后确认查询按公开契约使用资源 API Key `ak_`；不放宽鉴权边界，本轮通过任务表、mock 指标和 token 内容代理验证生命周期 |

## 2026-07-30 — Seedance URL-only 多媒体异步链路

### Goal

在不让图片、视频、音频文件内容进入 new-api 视频任务请求的前提下，统一 Adobe2API、Higgsfield2API 和 new-api 的 `frame|media` 参考素材语义，并通过 image-handle 提供 R2 预签名直传控制面。

### Phases

- [completed] 审计四个服务现状并锁定最终公共契约
- [completed] 在 image-handle 与 new-api 增加媒体预签名上传控制面
- [completed] 扩展 new-api URL-only 多媒体 DTO、AdobeVideo 与 HiggsfieldVideo adaptor
- [completed] 补齐 Adobe2API、Higgsfield2API 的 9/3/3、15 秒探测和上游映射
- [completed] 更新 Seedance 异步文档与 Resource Center OpenAPI
- [completed] 完成定向、全量和 Docker dev 联动验收

## 2026-07-30 — 四服务 main 分支发布

### Goal

仅提交并推送 Seedance URL-only 多媒体异步链路在 new-api、image-handle、Adobe2API、Higgsfield2API 的改动，保留各仓库所有无关工作树内容。

### Phases

- [completed] fetch 四个远程并审计提交边界
- [completed] 分仓库精确暂存并复核 staged diff
- [completed] 在四个 main 分支分别创建提交
- [in_progress] 推送 origin/main 并验证远程 SHA

### Exclusions

- new-api 的 Claude 响应诊断、请求转储脚本、临时输出和其他历史任务改动不进入本次提交。
- Adobe2API 未跟踪的管理员凭据脱敏设计文档不进入本次提交。
- supertokendoc 不属于用户本次指定的“四个服务”，不提交或推送。

### Errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| staged 临时 worktree 先运行 Go 测试时缺少被忽略的 `web/dist` | 1 | 临时 worktree 已自动清理；改为先执行前端 build 生成 embed 目录，再运行 `go test ./...` |
| 长命令经嵌套 exec 返回后台 session，首次未取得最终输出且留下临时 worktree | 1 | 不重复创建快照；复用同一 worktree直接取得全量测试退出码 0，并精确移除两个临时 worktree |

### Locked decisions

- 最新 URL-only 决策覆盖旧计划中 new-api 接收 multipart、Base64、Data URL和任务级文件暂存的部分。
- `frame` 最多两张图；`media` 最多 9 图、3 视频、3 音频，总计最多 12。
- 客户端通过 new-api 小型控制请求获取 R2 预签名 PUT，文件字节直接进入 R2；new-api 不代理 PUT。
- Adobe2API、Higgsfield2API 仍为直连兼容接口支持 JSON/multipart，并统一基于实际内容探测 15 秒限制。
- 参考素材不改变 `output.duration` 的按秒计费；分辨率仍由精确模型 SKU 固定。
- 不新增 new-api 或 Higgsfield 业务数据库字段；持久化内容必须脱敏。

### Errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| URL-only DTO 首次聚焦测试仍按旧契约期待 provider file reference 与 Data URL 合法 | 1 | 更新旧断言为明确拒绝，并增加 media/名称/URL 脱敏覆盖 |
| AdobeVideo adaptor 首次大补丁因底部 helper 的实际错误码与上下文不同未应用 | 1 | 确认补丁原子失败，读取精确片段后拆为结构、计数和 URL helper 三个小补丁 |
| 首次注册 HiggsfieldVideo 的跨前后端补丁因前端 Adobe 颜色上下文不同未应用 | 1 | 确认没有部分写入，拆分后端注册与三个前端精确片段 |
| 本机 Higgsfield Python 环境没有安装 pytest | 1 | 不修改全局 Python；后续使用项目已有 `uv run pytest` 或现有 Docker 镜像执行 |
| 首版 HiggsfieldVideo 测试按简化签名调用嵌入的 Adobe adaptor | 1 | 对齐真实 `BuildRequestBody(c, info)`、`DoRequest(c, info, body)` 与 `ChannelMeta.ApiKey` 后重跑 |
| Higgsfield 旧双图测试仍模拟每张图单独申请 `/media/batch` | 1 | 按已确认契约更新为一次传两个 MIME；实现保持单 batch、并行 PUT、串行 confirm |
| HiggsfieldVideo 继承 Adobe adaptor 时仍使用 Adobe 私有 SKU 集合 | 1 | 抽出 provider 参数化请求体构建，并在 Higgsfield adaptor 覆盖精确模型校验 |
| i18n 提取与同步把七个 locale 扩大为数千行无关重排和空翻译 | 1 | 以各 locale 的 HEAD 为基线，机械叠加既有 Claude 诊断键和本次 13 个媒体文档键；键级审计确认无删除、无空新增 |
| TypeScript 格式化首次在 new-api 目录运行，找不到 image-handle 文件 | 1 | Go 格式化已成功；切换到 image-handle 真实目录后执行 Prettier |
| HiggsfieldVideo 新测试漏导入 Adobe 命名空间常量 | 1 | 增加精确 import；controller、router、HiggsfieldVideo 聚焦测试全部通过 |
| image-handle 返回 `media.upload_session.list`，OpenAPI 声明 `media.upload.session.list` | 1 | 统一实现、测试和文档为 `media.upload.session.list` |
| HiggsfieldVideo 会接受或覆盖错误的 `provider_options.adobe_video` | 1 | 在命名空间桥接前明确拒绝 Adobe 命名空间，并增加回归测试 |
| 首次 Docker 综合验收在 Adobe 成功任务后退出，终端只输出成功任务 JSON | 1 | 不重复整条流程；已确认实际扣费 60000、钱包来源和 URL 脱敏持久化，按真实 Mock/数据库 schema 从断点核查 Range、Webhook 与后续 Higgsfield 流程 |
| 异步失败退款断言要求 `used_quota` 回到 120000，但实际为 180000 | 1 | 可用钱包已恢复 60000 且存在退款日志；`used_quota` 是累计消费审计值而非净支出，验收改为断言可用额度、退款日志和退款幂等性 |
| 15.001 秒 Docker 探测脚本使用 zsh 只读变量 `status` | 1 | Adobe 任务已创建且不重复提交；改用 `task_state` 从已有任务 ID继续轮询，再执行 Higgsfield 同步拒绝 |
| 首次 PostgreSQL fixture 清理通过未带 `-i` 的 `docker exec` 发送 here-doc | 1 | 对象与 Redis 已精确删除且验证；数据库计数确认未变化，补执行一次带 stdin 的同一精确事务后再重启核验 |
| Adobe2API 清理重启脚本探测不存在的 `/health` 路径 | 1 | 数据库和 Mock 清理已完成；中止无效循环，改用已验证的鉴权 `/v1/models` 与容器 healthcheck 检查，再删除本地媒体 fixture |
| 仓库级 i18n lint 从既有 420 项增至 441 项 | 1 | 新增 21 项全部是 Resource Center 的 OpenAPI `operationId`/`schemaName` 属性；将这两个非展示协议属性加入 ignoredAttributes 后恢复 420 基线，本次文件为 0 项 |
| 最终并行健康检查脚本引用了未在新 isolate 声明的 `repos` | 1 | 无测试动作被执行；在同一脚本内声明 `repoPaths` 后运行，所有健康、清理和 diff 检查通过 |

## 2026-07-30 — Adobe Fast 480p 真实多媒体联调

### Goal

按照 supertokendoc 的媒体上传与异步视频任务契约，通过本地 Docker dev 的 new-api 和 Adobe2API，真实验证 `adobe-seedance-2.0-fast-480p` 的 frame、media、轮询、Asset 下载和按秒计费。

### Phases

- [completed] 核对文档、容器、模型可见性、测试凭据和六个已上传素材
- [completed] 提交并轮询 4 秒 frame 双图任务
- [completed] 提交并轮询 4 秒 media 三图、双视频、单音频任务
- [completed] 验证任务终态、计费快照、钱包退款和请求脱敏；生成结果、Range 下载因上游授权失败不可执行
- [completed] 汇总真实问题与本地配置修复建议

### Locked decisions

- 任务请求只传 image-handle 完成接口返回的 HTTP URL，不传 Base64、Data URL 或 multipart 文件。
- 本地 image-handle 错误返回 `localhost:9000`；仅在本次容器联调请求中改写为 `minio:9000`，不改变公开契约或业务代码。
- 测试凭据只从本地 PostgreSQL 读入 shell 变量，不输出、不持久化。
- frame 使用首图加尾图；media 使用 3 图、2 个约 4 秒视频、1 个约 4 秒 WAV，均低于 15 秒限制。
- 两个任务都请求 4 秒；参考素材数量和时长不得改变有效计费秒数。

### Errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| image-handle Docker dev 完成接口返回 `http://localhost:9000/...`，Adobe2API 容器无法访问 | 1 | 已验证同一对象经 `http://minio:9000/...` 返回 200 和正确 MIME；本次仅改写测试 URL 并保留为待修配置问题 |
| 只读 preflight 按旧字段名查询 abilities，PostgreSQL 报 `model_name does not exist` | 1 | 使用 `\d abilities` 核对当前字段为 `model`、`group`、`channel_id`、`enabled`；不涉及任何数据修改 |
| 将 psql 元命令 `\d` 与 SQL 写在同一个 `-c` 参数中导致语法错误 | 1 | 元命令必须独立执行；拆成四个只读 psql 调用，不重试错误命令 |
| 首次 frame 创建由 Adobe2API 返回 401 `Invalid API key` | 1 | 渠道 124 的 Key、容器环境 Key 均与 Adobe2API `config/config.json` 的实际服务 Key 不一致；任务未创建且未扣费，准备对齐本地渠道 Key并刷新缓存 |
| `psql -v` 变量在单条 `-c` SQL 中未展开，Key 更新语句语法错误 | 1 | 数据未修改、容器未重启；改用 PostgreSQL 自定义 session setting 传入值，SQL 只读取 `current_setting` |
| 渠道 Key 更新语句假设 channels 有 `updated_at` 字段 | 1 | PostgreSQL 在执行前拒绝整条语句，Key 仍未修改；去掉不存在的审计列，仅更新已确认存在的 `key` |
| frame 任务进入真实 Adobe Seedance 后失败：`Unauthorized to perform request.` | 1 | 任务已正确异步终止；new-api 预扣 2000000 后钱包全额退款且只有一条退款日志。当前 Adobe 账号可 mint Firefly token，但无权完成 Seedance submit |
| 账号刷新脚本用 `trap rm -rf` 清理临时 cookie 目录，被执行策略拒绝 | 1 | 未执行登录或刷新；改为只在进程内存中解析 Session cookie，不创建含凭据的临时文件 |
| 内存刷新脚本的 awk HTTP 正则多写了一层反斜杠 | 1 | 登录请求已发出但未继续调用刷新接口；修正为 awk 字面量 `/^HTTP\//` 后重新登录 |
| media 任务与账号刷新后的 frame 重试均返回 `Unauthorized to perform request.` | 3 | Adobe 账号刷新接口返回 200，但错误不变；按三次失败协议停止继续提交，判定为当前 Adobe 账号/Firefly Seedance 权限阻塞，不再消耗调用 |

## 2026-07-30 — Adobe 新账号 10000 积分复测

### Goal

使用新导入的 10000 积分 Adobe 账号重新执行 4 秒 Fast 480p 的 frame 与 media 真实生成，完成异步、Asset、时长和计费闭环。

### Phases

- [completed] 核对新账号、渠道、模型和六个参考素材
- [completed] 运行 frame 双图任务并验证结果
- [completed] 运行 media 三图、双视频、单音频任务并验证结果
- [completed] 核对两次按秒钱包扣费、快照与任务脱敏

### Errors encountered

| Error | Attempt | Resolution |
|---|---:|---|
| 素材预检尝试在 Adobe2API 容器内调用 `curl`，镜像没有该命令 | 1 | 没有发出素材请求；改用镜像现有 Python `requests` 执行只读 HEAD/GET 检查 |

# Task Plan: VideoPricing 编辑与删除修复 (2026-07-30)

## Goal
补齐视频按秒计价页面中模型绑定和价格模板的修改、解绑与删除能力，并确保保存失败不会让页面显示出未持久化的假状态。

## Current Phase
Complete

### Phase 1: 现状与根因
- [x] 检查页面事件、保存流程和 helper 删除语义
- [x] 复现绑定修改、解绑、模板修改和删除问题
**Status:** complete

### Phase 2: 实现
- [x] 补齐模板编辑/删除和绑定修改/解绑交互
- [x] 增加确认、引用保护、保存中状态和错误回滚
- [x] 修复 `adobe-veo-*` 与 `adobe-seedance-*` 在模型广场的系列分类
**Status:** complete

### Phase 3: 验证
- [x] 增加 helper 定向测试
- [x] 增加 Self-Adobe Veo/Seedance 分类回归测试
- [x] 运行前端测试、桌面构建和 Docker dev 浏览器验收
**Status:** complete

## Locked Decisions
- 删除仍被模型引用的价格模板时阻止操作，并提示先解绑或改绑。
- 不修改 VideoPricing JSON 协议或后端计费计算。
- 模型分类按模型系列判断；Self-Adobe 只表示渠道接入方式，不改变 Veo 的 Google 分类。
- 用户明确排除多语言补齐与移动端兼容。
- 不触碰工作区中无关的未跟踪诊断文件。

## Errors Encountered
| Error | Attempt | Resolution |
| --- | ---: | --- |
| VideoPricing 多文件补丁因页面上下文不完全匹配而校验失败 | 1 | 未产生部分修改；拆成 helper、测试、页面小补丁并按精确行段应用 |
| 从 `web/` 工作目录执行 `gofmt model/...` 找不到 Go 文件 | 1 | 前端 Prettier 已完成；改为从仓库根目录单独执行 Go 格式化 |
# Task Plan: Adobe Multi-model Direct URLs and Console Upgrade (2026-07-30)

## Goal
Add the approved Kling/Veo Adobe video SKUs and capability validation, pass Adobe CDN result URLs through without downloading media, make new-api assets expose those URLs safely, add general channel model discovery, and replace the Adobe2API static admin page with a persisted React operations console.

## Current Phase
Complete

### Phase 1: Baseline and contracts
- [x] Audit both worktrees, existing Adobe video flow, asset projection, channel discovery, and Higgsfield console reference.
- [x] Lock capability, direct-URL, persistence, and backward-compatibility contracts in focused tests.
**Status:** complete

### Phase 2: Adobe2API backend
- [x] Add table-driven Kling/Veo capabilities, payload conversion, strict pre-submit validation, and direct Adobe result URLs.
- [x] Add SQLite task/API-key/request-log persistence, restart recovery, and compatible legacy task/content behavior.
**Status:** complete

### Phase 3: Adobe2API console
- [x] Build the React/Vite overview, account pool, generation test, tasks, API keys, request logs, and settings pages.
- [x] Integrate the existing account/config/admin operations with responsive and accessible interaction states.
**Status:** complete

### Phase 4: new-api integration
- [x] Add `images` mode and the eight public Adobe model capabilities before billing/upstream submission.
- [x] Pass direct Adobe URLs into tasks/assets, fix historical internal-reference projection, and generalize channel model discovery.
**Status:** complete

### Phase 5: Verification
- [x] Run focused/full backend and frontend checks in both repositories.
- [x] Rebuild Docker dev, verify lifecycle/billing/direct URLs, and run the four approved real 720p tasks against the available Adobe account.
- [x] Verify desktop console task preview/open/download behavior without routing media bytes through either service.
**Status:** complete

## Locked Decisions
- Existing uncommitted VideoPricing, model-visibility, and Self-Adobe/Self-Higgsfield work must be preserved.
- New Adobe results remain temporary provider URLs; neither service downloads, refreshes, archives, or guarantees their lifetime.
- Historical local Adobe results retain the authenticated `/content` path and existing stored files.
- Resolution is fixed by exact SKU; production VideoPricing values remain administrator configuration.
- Mobile compatibility and mobile browser acceptance are explicitly out of scope per the user's final instruction.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| PostgreSQL baseline query used the nonexistent `tokens.quota` column | 1 | Read the live schema and use `tokens.remain_quota` plus `users.quota`; no data was changed. |
| Adobe request-log verification queried nonexistent `request_body`/`response_body` columns | 1 | Read the SQLite schema and inspect the single redacted `payload` column instead. |
| Real Kling 3.0 1-second acceptance request was rejected because Adobe requires at least 3 seconds | 1 | Correct both capability catalogs and tests to the verified `3-15s` range; use 3 seconds for real acceptance. |
| Real Kling 3.0 frame task reached Adobe but failed with downstream `fal-ai-video` 408 timeout | 1 | Confirm the immutable 3-second charge was fully refunded and no local result file was created; do not silently treat it as a contract failure. |
| Real Kling Omni images submit rejected `usage=asset`; Adobe reported allowed roles `frame` and `style` | 1 | Map Omni `images` references to `usage=style`, add a payload regression assertion, rebuild, and retry once. |
| First desktop task-row click inherited the previous mobile viewport override and targeted a point outside the visible width | 1 | Reset the existing browser tab to a desktop viewport before continuing desktop-only QA; no request or task state was changed. |
| Adobe2API full suite on the host could not import FastAPI/Pydantic; the next command accidentally omitted `docker exec` and repeated the host failure | 2 | Use an explicit `docker exec adobe2api ...` invocation for the authoritative suite; do not mutate the host Python environment or repeat the host command. |
| The running Adobe production image does not contain the repository `tests/` directory | 1 | Launch a disposable container from the same runtime image with the source mounted read-only and an isolated writable test-data tmpfs. |
| Browser evaluate sandbox does not expose `fetch` for a direct capability API probe | 1 | Validate the same catalog through the live Generate Test UI that consumes it, avoiding unsupported page-sandbox networking. |
| The planning completion helper reports two pending phases from an unrelated historical image-pricing plan in the shared planning file | 1 | Confirm this Adobe plan has no unchecked phase and leave unrelated historical work unchanged instead of falsely marking it complete. |
| Corrected Kling 3.0 `3s frame` request without an image reached Adobe before failing with `at least 1 item(s) with usage='frame' required` | 1 | Add a minimum-reference capability contract in both services so zero-image Kling frame requests fail before precharge; resubmit with one valid image. |
| Combined validation-test search ran from new-api and therefore could not resolve Adobe2API's `tests/` paths | 1 | Inspect each repository's tests from its own workdir and avoid cross-repository relative paths. |
| Existing `stable media rejected` test omitted the newly required Kling frame image, so validation correctly stopped at `reference_image_required` before its intended video-limit assertion | 1 | Add one valid image to that fixture so it reaches and continues covering `reference_video_limit_exceeded`. |
| Live PostgreSQL baseline query lost SQL string quotes through nested `sh -lc` parsing | 1 | No query ran and no data changed; invoke `psql` directly with one quoted `-c` argument for the retry. |
| Revised baseline query referenced obsolete `tokens.models_enabled` | 1 | The first read-only user row returned and later statements stopped; inspect the live schema and query only existing token columns. |
| Baseline task count assumed a `tasks.model` column | 1 | Current task model data is stored under a different schema; inspect `tasks` and `logs` columns before the final read-only baseline query. |
| PostgreSQL rejected `LIKE` directly on JSON `tasks.properties` | 1 | Cast the JSON value to text for the diagnostic count; this affects only the read-only test query, not application code. |

---

# Task Plan: Video Contract Alignment (2026-07-31)

## Goal
Make the normalized video implementation, generated OpenAPI, and SuperToken
Seedance/Kling/Veo documentation describe the same executable contract.

## Current Phase
Complete

### Phase 1: Contract tests
- [x] Add Higgsfield Seedance validation and per-second billing regression coverage.
- [x] Add Adobe prompt-length precharge-boundary coverage.
- [x] Lock generated OpenAPI support for `images` and provider-specific capabilities.
**Status:** complete

### Phase 2: Implementation
- [x] Share Seedance validation and billing with Higgsfield without model-name parsing.
- [x] Enforce Adobe's 1200-character prompt limit before pricing.
- [x] Repair the OpenAPI generator and regenerate the checked-in artifact.
**Status:** complete

### Phase 3: Documentation
- [x] Correct Seedance public model names and complete the Adobe model list.
- [x] Correct direct result URLs, Webhook examples, error codes, prompt limits, and duration semantics.
**Status:** complete

### Phase 4: Verification
- [x] Run focused/full backend tests, OpenAPI checks, JSON example parsing, and docs build.
- [x] Rebuild Docker dev and verify rejection before task creation and billing.
**Status:** complete

## Locked Decisions
- Do not parse public model names to infer resolution or pricing.
- Do not change database schemas, channel IDs, or existing VideoPricing values.
- Keep URL-only reference sources; do not reintroduce Base64 or multipart into `/v1/video/tasks`.
- Preserve all unrelated tracked and untracked workspace changes.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| The first local token probe selected a nonexistent `tokens.enabled` column | 1 | Inspect the live PostgreSQL schema and use the actual `status` column. |
| The Docker health loop assigned zsh's read-only `status` variable | 1 | Keep the already recreated container and repeat the bounded health check with `health_state`. |
| The first live rejection harness used `rm -f` to clean an exact `mktemp` response file and was rejected before execution | 1 | Avoid filesystem state entirely and capture the response body plus HTTP code in shell memory. |
| The Adobe2API runtime image does not include `pytest` | 1 | Run the repository's `unittest`-based suites with Python's standard-library test runner. |

---
# Task Plan: Leonardo Seedance 2.5 Runtime Support (2026-08-09)

## Goal
Allow discovered `seedance-2.5-480p` and `seedance-2.5-720p` Leonardo mappings to run through `/v1/video/tasks` with correct pre-billing validation and upstream payloads.

## Current Phase
Complete

- [x] Confirm discovery is already unfiltered and isolate the remaining runtime whitelist failure.
- [x] Add Seedance 2.5 model, duration, reference-mode, and payload support.
- [x] Update focused adaptor and public catalog tests/document generation sources as required.
- [x] Run focused/full backend checks, frontend/OpenAPI checks if touched, and mock-only integration.
- [x] Commit only task implementation files and push `main`.

## Locked Decisions
- Seedance 2.0 and MiniMax H3 behavior remains unchanged.
- Seedance 2.5 supports exact 480p/720p mappings, 4-30 seconds, media mode, and 1-2 image frame mode.
- Validation must happen before billing; unknown future model capabilities are not guessed.
- No paid Leonardo Generate is executed.

## Errors Encountered

| Error | Attempt | Resolution |
| --- | --- | --- |
| Focused red tests reject Seedance 2.5 as `unsupported_video_model` and reject frame mode as `unsupported_reference_mode` | 1 | Expected pre-implementation failure; add exact SKUs and separate 2.5 capability handling. |
| Full Go suite retained the old six-model expectation in the shared relay registration test | 1 | Update the shared contract assertion with both exact Seedance 2.5 SKUs. |
| Prettier warns on generated OpenAPI JSON even though generator drift check passes | 1 | Keep generated JSON in the generator's canonical `JSON.stringify` format and scope Prettier to the JavaScript source, matching repository behavior. |
| Used the exec-cell wait helper with a nested PTY session ID during Docker build | 1 | Poll the returned PTY with `write_stdin`; the build continued normally and completed successfully. |
| Initial residue count queried nonexistent `tasks.model` | 1 | Inspect the task schema before the next exact cleanup/residue query; no data was changed. |
| First Docker API probe returned 403 because local `vip` group is deprecated | 1 | Cleanup restored all state; retry with existing active admin token/group and temporarily add only that group to mock channel 128. |
| Second Docker API probe returned masked 502 after successful precharge/refund | 1 | Inspect mock validation: it still caps all non-H3 videos at 15 seconds and rejects 2.5 `reference_mode`; update the mock contract and rerun. |
| First completed mock submission could not be queried with the create Bearer token | 1 | The normalized query route intentionally requires a Resource Key; preserve the successful create cleanup and switch the query probe to `ak_`. |
| The selected local `ak_` row still returned 401 | 1 | Inspect the row and find it was soft-deleted; create one temporary active read-only Resource Key for the final probe and physically delete it during cleanup. |

---

# Task Plan: Broad Leonardo Reference Admission (2026-08-10)

## Goal

Keep only a broad, provider-neutral reference safety envelope in new-api's Leonardo channel and
delegate concrete model capabilities, combinations, and media-duration limits to Leonardo2API.

## Current Phase

Complete

### Phase 1: Contract tests

- [x] Lock the public envelope at 30 images, 10 videos, 10 audios, and 50 total references.
- [x] Lock the Leonardo adaptor to the same broad envelope for all supported Leonardo models.
- [x] Prove frame/images public semantics, model mapping, duration, URL, and name checks remain.
**Status:** complete

### Phase 2: Implementation

- [x] Remove Leonardo model-specific reference counts and combinations.
- [x] Preserve H3 native-audio, model duration, aspect-ratio, and transport validation.
- [x] Keep actual media probing and detailed validation authoritative in Leonardo2API.
- [x] Align generated OpenAPI and the Resource Center reference copy with the new responsibility boundary.
**Status:** complete

### Phase 3: Verification

- [x] Run focused controller/adaptor/relay tests and formatting.
- [x] Run broader affected-package tests and `git diff --check`.
- [x] Regenerate/check OpenAPI, validate i18n status, and build the frontend production bundle.
**Status:** complete

## Locked Decisions

- The broad envelope is a request-abuse bound, not a claim that every Leonardo model supports it.
- new-api does not download reference URLs or add client-supplied duration metadata.
- Leonardo2API's safe validation code/message is returned to the caller; arbitrary upstream errors
  remain sanitized.
- Adobe, Higgsfield, xAI, billing, databases, and Leonardo2API are unchanged in this task.
- Generated OpenAPI and Resource Center copy describe the broad gateway envelope without exposing internal channel names.
- Preserve all unrelated tracked and untracked workspace changes.

## Errors Encountered

| Error | Attempt | Resolution |
| --- | ---: | --- |
| Initial planning patch targeted a heading present only in progress/findings, not task_plan | 1 | No file changed; inspect exact file tails and append this scoped plan using unique context. |
| Focused contract tests do not compile because the shared broad-envelope constants do not exist | 1 | Expected red phase; add the constants and use them in both common and Leonardo validation. |
| Combined OpenAPI/UI localization patch used translations that differed from the checked-in locale text | 1 | Patch was atomic and changed nothing; inspect exact locale entries and split generator/UI edits from precise locale replacements. |
