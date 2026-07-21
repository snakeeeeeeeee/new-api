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
