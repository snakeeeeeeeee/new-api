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
