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
| Combined planning-file patch expected a template heading that the existing findings file did not use | 1 | Re-read each file header and apply independent insertions using its actual first line. |
| Health polling used zsh's read-only `status` variable | 1 | Use a direct health endpoint check and a non-reserved variable on subsequent checks. |
| Initial image-handle PostgreSQL inspection assumed a `postgres` role | 1 | Read the container connection configuration and use the actual `image_handle` role. |
| A diagnostic query treated the new-api log `other` TEXT column as JSONB | 1 | Cast `other::jsonb` before extracting structured error fields. |
| Adobe token upstream disconnected twice through image-handle and once directly | 3 | Stop paid retries; retain the `fetch failed` and HTTP/2 framing evidence for upstream investigation. |

---

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
