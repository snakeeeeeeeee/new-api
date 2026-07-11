# GPT Cache-Write Billing Findings

## Current State
- Backend implementation and targeted/full Go tests were reported passing before final independent review.
- Frontend normalization, log formulas, locales, Bun tests, and production build were reported passing.
- Docker dev is available on port 3001; deterministic mock and authorized live sub2api verification remain.
- Unrelated untracked files under `2dev/`, `outputs/`, and `tmp/` are user-owned and must remain untouched.
- Initial review confirms official explicit zero wins over legacy usage, unconfigured/invalid writes remain ordinary input, and an explicit ratio of zero or one remains enabled.
- The generic frontend detail text still has a `缓存写 ...` source key while the expanded-row label uses `缓存写入 ...`; normalize the wording after logic review.
- Frontend normalization keeps unconfigured official writes inside ordinary input (`total - cache read`), while configured writes use `total - cache read - billed write`; legacy Claude rows retain their existing ordinary-prompt semantics.
- Backend validates official writes against prompt input remaining after cache reads for non-Claude usage and logs invalid values without independently billing them.
- `GetCreateCacheRatio` already returns `(defaultRatio, false)` for a missing key, so snapshotting its bool cleanly implements the requested presence switch without changing legacy default ratios.
- Per-call (`UsePrice`) billing remains outside this change as planned; cache-write splitting is applied only in token-ratio billing.
- Responses handlers share one normalizer that copies the official pointer and normalized legacy-compatible value; native Chat relies on the common post-processing hook, so stream and non-stream paths converge before billing.
- The normalizer avoids tagging legacy Claude split usage as OpenAI Responses, preserving its prompt-token semantics and 5m/1h accounting.
- Dedicated handler tests cover Responses, Compact, Responses streaming, Responses-to-Chat streaming, and native Chat streaming/non-streaming, including official nonzero and explicit zero values.
- Other `CachedCreationTokens` producers found by repository search belong to Claude or excluded image/task billing paths; they continue using the legacy field and are not reclassified as official GPT writes.
- Frontend consumers consistently use the log snapshot (`input_tokens_total`, cached/write tokens, stored ratios) rather than current model configuration, and `??` preserves configured zero ratios.
- Fixed-price summaries already render only the per-call amount, but backend must suppress new official-write metadata to avoid a misleading expanded token label.
- Generic price-mode and ratio-mode formulas both keep unconfigured writes in ordinary input and add configured writes at the stored creation ratio; a configured zero ratio still removes the tokens and produces a zero cache-write amount.
- Existing image/audio-specific display branches remain outside the new dedicated cache-write formula, matching the stated scope; their legacy behavior is not changed by configuration handling.
- Independent-review fixes now suppress official cache-write classification entirely for fixed per-call pricing while preserving legacy-only aggregate/split fields.
- New OpenRouter regression fixtures prove a nonzero `usage.cost` would infer creation tokens, then lock official explicit zero and unconfigured official positive values out of that legacy inference path.
- New frontend wording is consistently `缓存写入 ...` in render code and all seven locale files.
- Docker dev uses PostgreSQL plus Redis on `new-api_new-api-dev-network`; the rebuilt application is healthy on host port 3001.
- No dedicated cache-write mock was identified yet; integration can add a temporary network-scoped upstream and temporary DB rows while preserving existing route 85.
- Rebuilt live tests confirm sub2api preserves official `cache_write_tokens` in both streaming and non-streaming Responses; sampled values were explicit zero with 3,840 cached tokens.
- For explicit zero on configured `gpt-5.6-sol`, logs correctly snapshot `reported=0`, `enabled=true`, total input 8,552, no billed creation fields, and quota 12,815.
- Existing options confirm `gpt-5.6-sol` has explicit model/completion/read/write ratios while a normal GPT model lacks the write key, validating the presence-switch data shape.
- Deterministic Docker testing will use unique temporary models/group/channel/token plus JSONB key additions, then remove only those unique rows/keys and restart; existing route 85 remains untouched.
- Token auth requires the group in both `UserUsableGroups` and `GroupRatio`; the mock setup will add a unique group at ratio 1.1 and use an unlimited temporary token owned by the same enabled dev user.
- Current stored model ratios are deliberately narrow, so the mock needs temporary model/completion/read/write ratio keys rather than borrowing a production model name.
- Application startup replaces in-memory ratio maps from the option JSON, so temporary JSONB additions plus a controlled container restart are sufficient and can be cleanly removed afterward.
- Channel abilities can be inserted directly for the unique group/models; token keys are 48 characters and clients authenticate with the usual `sk-` prefix.
- A second temporary Anthropic channel (type 14) can target the same mock server's `/v1/messages` endpoint to verify legacy Claude 5m/1h behavior independently of the new OpenAI field.
- Deterministic Responses non-stream results match the planned quota exactly: configured 1,089 quota (`$0.002178`) and unconfigured 1,034 quota (`$0.002068`).
- Missing and explicit-zero official fields both charge 1,034 quota; zero records `reported=0/enabled=true`, while missing records neither field.
- Negative and oversized writes remain ordinary input, charge 1,034 quota, and persist the expected warning; Responses and Chat both pass the official field in stream and non-stream responses.
- `/v1/responses/compact` distributes using an internal `<model>-openai-compact` name; Docker fixtures must include the suffixed model even though the client sends the base name.
- Compact configured/unconfigured/zero fixtures pass after adding suffixed abilities, with 1,089/1,034/1,034 quota and correct log snapshots.
- The in-app browser has an existing local admin session, so desktop and narrow log rendering can be verified without changing authentication data.
- The apparent local session was stale: the authenticated console returned a login-required error, so real desktop/narrow expanded-log inspection cannot be completed without user login.
- Claude mock billing remained unchanged: ordinary input 800, read 800, creation 400 split as 300/100 at 1.25/2.0, quota 1,130, and no official GPT reported/enabled fields.
- Final residue audit shows no mock listener and zero temporary channels/tokens/abilities/model/group keys; original unrelated untracked files remain untouched.
- Frontend official-zero precedence now also holds for hybrid logs containing stale legacy fields while `reported=0/enabled=true`.

## Required Invariants
- Missing official cache-write usage is not inferred.
- Explicit official zero overrides legacy cache-creation values.
- Only valid, configured official writes are removed from ordinary input and billed at the create-cache ratio.
- Reported-but-unconfigured writes remain inside ordinary input and are visible in logs.
- Token credentials and request authorization headers are never printed during live verification.

---

# Upstream rc.21 Comparison Findings

## Scope
- Target upstream release: `QuantumNous/new-api` tag `v1.0.0-rc.21`.
- Local baseline: current `main` in fork `snakeeeeeeeee/new-api`, including the previously completed GPT cache-write billing work recorded above.
- This task is analysis-only; no product code changes are planned.

## Initial State
- Local tracked worktree is clean; unrelated untracked files under `2dev/`, `outputs/`, and `tmp/` are user-owned and will remain untouched.
- The upstream tag is not currently present in the local object database.
- Existing records describe local semantics: explicit ratio-key presence enables billing; official explicit zero is authoritative; unconfigured/invalid writes remain ordinary input; fixed-price billing is excluded; legacy Claude/OpenRouter behavior is preserved.

## Version Resolution
- Upstream tag `v1.0.0-rc.21` resolves to commit `bde9b2f44887d34ec54799ae191d50f97914359e`, dated `2026-07-11T22:57:22+08:00`.
- The tag was cloned into `/tmp/new-api-upstream-rc21`; the workspace repository and its refs were not changed.
- Local relevant code is spread across `dto/openai_response.go`, `relay/helper/price.go`, `service/text_quota.go`, OpenAI conversion/relay handlers, `model/log.go`, and `web/src/helpers/promptCacheUsage.js`, with extensive targeted tests.
- Local commit messages do not mention cache-write billing directly, so the introduction commit must be found with pickaxe/history searches rather than message grep.

## First Structural Differences
- The local feature was introduced by commit `614d134cebba4eef4cb9fae2d411612f5252c5e7` (`feat: support configurable OpenAI cache write billing`, 2026-07-11 05:14 +08:00).
- Upstream declares `InputTokenDetails.CacheWriteTokens` as non-pointer `int` with `omitempty`; local declares `*int`. Upstream therefore collapses absent and explicit zero during unmarshal/remarshal, while local preserves them.
- Upstream normalizes creation usage through a `TotalCacheCreationTokens` max-style helper combining legacy `CachedCreationTokens`, split Claude values, and native `CacheWriteTokens`; local separately tracks the official field and gives its presence authoritative precedence.
- Upstream price extraction calls `GetCreateCacheRatio` but discards the returned configuration-presence boolean; local stores that boolean as `CacheCreationRatioConfigured` and uses it as the separate-billing switch.
- Upstream has focused quota tests and conversion propagation tests, but the initial search did not show the local-style `cache_write_tokens_reported` / `cache_write_billing_enabled` log metadata or dedicated frontend normalization helper.

## DTO and Billing Shape
- Upstream helper `CacheCreationTokensTotal()` chooses the maximum of legacy `CachedCreationTokens` and native `CacheWriteTokens`, then clamps negatives to zero. Local helper `ResolveCacheCreationTokens()` makes a present official value authoritative, including explicit zero, and returns a separate presence flag.
- Consequently, when both fields exist as `cached_creation_tokens=999, cache_write_tokens=0`, upstream keeps 999 cache-write tokens; local resolves to 0. This is a real billing difference, not only a serialization difference.
- Upstream immediately assigns the normalized maximum to `summary.CacheCreationTokens` and includes it in the separate cache-creation term. It has no local-style reported/configured/invalid state machine.
- Upstream clamps negative ordinary-input remainder after subtracting cached reads and cache writes. Local instead validates official write tokens against the available non-cached input; invalid/oversized values remain ordinary input and generate a warning.
- Local fixed-price handling deliberately suppresses new official cache-write classification and metadata. Upstream's fixed-price total is also unaffected by token categories, but its summary still normalizes the field; there is no explicit reported/enabled audit state.
- The full-file diff includes many unrelated fork/upstream differences, so final conclusions will be based on the cache-write-specific hunks and tests rather than treating the entire files as feature diffs.

## Configuration Semantics
- `GetCreateCacheRatio` returns `(1.25, false)` when a model has no entry in both trees.
- Upstream discards `false`, so any positive normalized `cache_write_tokens` is separately billed at the fallback 1.25 ratio even when the model has no explicit `CreateCacheRatio` configuration.
- Local carries the boolean into `PriceData.CacheCreationRatioConfigured`; official writes are only split out when an entry exists. Otherwise those tokens remain ordinary input. This was the central safety switch in the local design.
- Upstream adds built-in `CreateCacheRatio=1.25` entries for `gpt-5.6-sol`, `gpt-5.6-terra`, and `gpt-5.6-luna`. Local source defaults do not include those three names, although the prior Docker/live environment had an explicit database option for `gpt-5.6-sol`.
- For the three official model names, both implementations charge the same ratio when local configuration contains the same entries and the upstream field is valid/nonzero. The major divergence appears for unconfigured models and ambiguous/invalid payloads.

## Tooling Notes
- The temporary upstream clone was deepened successfully for historical inspection.
- `gh` is unavailable locally, so release notes will be read from GitHub's public REST API instead.

## Upstream Feature Commit and Release Claim
- The upstream cache-write feature was introduced by commit `48068ce9236e7bfcf923f8d20ca39fb8e611ef86` at 2026-07-11 21:08 +08:00, about sixteen hours after the local fork commit.
- Its subject is `feat: bill OpenAI cache_write_tokens at cache-creation price with zero clamp`.
- Official rc.21 release notes describe the feature as charging native `cache_write_tokens` at the cache-creation rate with safeguards around cached and uncached prompt accounting.
- The release notes do not claim explicit-zero preservation, explicit configuration gating, invalid-value fallback, or cache-write-specific audit metadata; those are local extensions that must be assessed from code.
- Release URL: https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.21

## Scope Comparison
- Upstream feature commit: 18 files, 158 insertions, 20 deletions. It primarily adds the field, propagates it through conversion paths, changes the billing total, adds 56 quota-test lines, adds GPT-5.6 default ratios, and makes a small mobile-log display adjustment.
- Local feature commit: 28 files, 1,580 insertions, 135 deletions. It adds pointer/presence semantics, price configuration state, validation and audit metadata, comprehensive OpenAI relay/conversion tests, and a dedicated frontend usage-normalization/display layer with locale coverage.
- The size difference reflects materially broader semantics and regression coverage locally, not merely coding style.

## Core Accounting Disagreement
- Upstream explicitly models native `cache_write_tokens` as an unadjusted prefix count that may overlap `cached_tokens`. Its regression fixture uses `prompt=3619`, `cached=2921`, `write=3616`; it bills all 2,921 read-cache tokens and all 3,616 write-cache tokens, while clamping ordinary input to zero.
- Local validates official writes against `prompt - cached` for non-Claude usage. The same upstream fixture has only 698 "available" tokens, so local rejects 3,616 as oversized, bills the 698 remainder as ordinary input, and records a warning.
- Therefore local is more defensive against malformed upstream values but conflicts with the accounting model adopted by rc.21. If rc.21's real OpenAI fixture is representative of GPT-5.6, local will undercharge positive cache writes whenever write and read prefix counts overlap.
- With upstream's test ratios (`input=1`, read=0.1, write=1.25, output=2), upstream quota is 4,884. Local's validation path would be approximately 1,062 for the same payload: `698 + 292.1 + 72`, a large billing divergence.
- Upstream tests only two positive cases: a small write that fits and an overlapping write that requires the zero clamp. Local tests configured/unconfigured, ratio 0/1/1.25, missing/zero/negative/oversized, fixed price, OpenRouter inference suppression, logging, and exact plan amounts.

## Conversion, Logging, and UI
- Upstream propagates its integer field through Responses, Chat, Compact, OpenAI-to-Claude, Claude-to-OpenAI, and tiered expression billing. Its Claude-to-OpenAI converter deliberately fills both the legacy field and the new native-looking `cache_write_tokens` field.
- Local avoids reclassifying legacy Claude split-cache usage as official OpenAI reporting. That preserves the local distinction between legacy cache creation and a natively reported OpenAI field, including explicit-zero authority and OpenRouter cost-inference suppression.
- Upstream's OpenAI-to-Claude conversion subtracts cached reads and the normalized cache-write maximum from OpenAI prompt tokens, then clamps Claude input tokens to zero. This matches its overlapping-prefix model end to end.
- Upstream includes `cache_write_tokens` in tiered-billing variable `cc`; the local fork's current billing architecture does not contain the same upstream tiered-settlement path, so this is an upstream capability addition rather than a direct regression in the local patch.
- Local logs `cache_write_tokens_reported`, `cache_write_billing_enabled`, and a reliable `input_tokens_total`, and can tell administrators that a reported write was charged as ordinary input. Upstream logs only the normalized billed creation total through its existing fields; it does not preserve "reported but not separately billed" because that state does not exist upstream.
- Local frontend reconstructs ordinary/read/write amounts from the new log snapshot, displays configured-zero and unconfigured cases correctly, and supports old logs plus Claude split-cache logs. Upstream's feature commit only makes a small existing log-card adjustment and has no comparable cache-write normalization/audit UI layer.

## Coverage Confirmation
- Upstream tag tests cover positive write propagation and the two quota cases, including overlapping prefixes. Search found no cache-write test for explicit zero, field absence, unconfigured ratios, negative native values, or reported-vs-billed log state.
- Current local line anchors for the main design are: DTO pointer/presence helper at `dto/openai_response.go:262`, configuration flag at `types/price_data.go:30`, configuration capture at `relay/helper/price.go:67`, billing validation at `service/text_quota.go:198`, and frontend normalization at `web/src/helpers/promptCacheUsage.js:45`.

## Final Assessment
- Local is stronger on provenance, explicit-zero correctness, configuration control, malformed-data defense, auditability, frontend explanation, and regression breadth.
- Upstream is better aligned with the newly documented GPT-5.6 overlapping-prefix accounting model. Its zero clamp avoids negative ordinary-input charges without rejecting a legitimate cache-write prefix larger than `prompt-cached`.
- A sound merge strategy is hybrid rather than a direct cherry-pick: retain local pointer/presence semantics, explicit ratio gating, audit logs, UI, and tests; adopt upstream's overlap-aware ordinary-input calculation for valid nonnegative official writes; add the three GPT-5.6 default ratios if those names should work out of the box.
- The business decision that remains is whether unconfigured native writes should use upstream's fallback 1.25 or local's ordinary-input fallback. Current local policy intentionally chooses the latter and provides clearer operator control.

---

# GPT-5.6 Overlap Merge Findings

## Confirmed Requirements
- Keep local pointer/presence semantics, configuration gating, audit metadata, legacy Claude behavior, and frontend compatibility.
- Adopt upstream overlap-aware accounting for configured official cache writes.
- Missing and explicit-zero writes must not appear as visible cache-write entries in logs.
- Explicit zero remains meaningful internally and must still override stale/legacy write values.
- Complete automated tests and Docker dev integration tests before delivery.

## Selected Design
- Backend raw log metadata may retain `cache_write_tokens_reported=0` and `cache_write_billing_enabled=true`; visible frontend summaries must treat zero exactly like missing and omit the cache-write row/segment.
- Configured nonnegative native writes use `max(prompt-cached-write, 0)` for ordinary input, allowing read/write prefix overlap.
- Unconfigured native writes remain ordinary input under the existing local policy.
- Malformed-value protection will be based on native write validity rather than the obsolete `prompt-cached` bound.

## Baseline Code Findings
- Current backend rejects `write > prompt-cached`; the existing test case uses `prompt=2000`, `cached=800`, `write=1201` and expects ordinary-input fallback. Under the upstream overlap model this value is valid and must be billed separately with ordinary input clamped to zero.
- A safer local bound is `write <= prompt` for non-Claude native OpenAI usage. This accepts the official overlap fixture while still rejecting obviously oversized positive values such as `write=2001` for `prompt=2000`.
- Current frontend normalizer correctly retains explicit-zero presence internally, but visible table and expanded-row conditions use `wasReported && valid`, which renders a zero cache-write entry. Visible conditions must instead require a positive reported count.
- Existing backend and frontend test suites already isolate all affected semantics, so changes can remain focused in billing validation/clamping, default ratios, visible-log predicates, and their tests.
- Current quota calculation subtracts cached reads and cache writes from `baseTokens` but does not clamp `baseTokens` before adding priced components. Adopting overlap semantics therefore requires an explicit zero clamp at the end of input-category subtraction, matching upstream.
- Visible zero currently leaks through two separate paths: the compact input-column helper treats any valid reported field as a write, and the expanded usage row accepts `wasReported && valid`. Billing-detail rendering itself already uses positive-token guards in the render helpers.
- The local default create-cache map currently starts with Claude entries; adding the three GPT-5.6 entries will make the existing presence gate true after default settings are loaded.
- With the existing unit-test ratios, `prompt=2000`, `cached=800`, `write=1201`, `output=100` should become a valid overlap case with quota 2,181 and no warning. A new `write=2001` case preserves malformed-value fallback at quota 1,880.
- A separate regression should reproduce upstream's exact fixture and ratios so `prompt=3619`, `cached=2921`, `write=3616`, `output=36` asserts quota 4,884.
- Adding a normalized `hasVisibleCacheWrite` flag centralizes the positive-only UI rule and lets helper tests cover both compact-column and expanded-row consumers without discarding raw zero state.

## Configuration and Docker Findings
- Default ratio initialization adds `defaultCreateCacheRatio` before option loading. Existing deployments subsequently load their persisted `CreateCacheRatio` JSON, so Docker verification must inspect the live option rather than assuming new source defaults were merged into an existing database.
- No reusable cache-write Docker simulation script exists under `2dev`; the prior deterministic verification was performed with temporary DB/model/channel/token fixtures. This run will use the same isolated-fixture pattern and clean up only uniquely named rows/options.
- Ratio JSON loading replaces the runtime map rather than merging defaults. The new GPT-5.6 defaults therefore affect fresh/default configurations, while existing deployments retain their persisted operator choices. This is consistent with local explicit-presence policy and avoids silently re-enabling billing on upgrades.
- The ratio package has no cache-ratio-specific test file; a focused package-local test can lock the three new default entries without mutating runtime option state.

## Implementation Audit
- All remaining render-helper uses already require positive reported tokens before emitting cache-write text. After switching the compact input summary and expanded row to `hasVisibleCacheWrite`, no visible path renders missing, zero, or invalid-negative writes.
- `git diff --check` passes after Go and frontend formatting.
- Final product-diff review found no remaining cache-write logic, visibility, compatibility, or cleanup gaps. The change keeps explicit-zero backend precedence while applying positive-only visible rendering.

## Docker Baseline
- Docker dev uses PostgreSQL and Redis on `new-api-dev-network`, with the app exposed at `http://localhost:3001` and source-built image `new-api-local:dev`.
- All three services are already running; the existing app container is healthy but predates this change and must be rebuilt/recreated.
- The rebuilt image is `sha256:361ae238808b35378ee29750fa0551dd37da8dd1e621a4a1239b9050e8f94980`; the recreated app immediately served a successful `/api/status` response while the healthcheck was still in its initial `starting` interval.
- The Compose network is `new-api_new-api-dev-network`. PostgreSQL schemas confirm temporary fixtures can be isolated with unique channel/token/ability/model names and validated through persisted `logs.quota`, token counts, and `logs.other`.
- Enabled user `temp_default` (id 2) and group `default` are suitable for a temporary unlimited test token. An OpenAI channel uses type 1. Existing ratio options are persisted, so tests will add and later delete only unique model keys.
- The live `default` group ratio is intentionally 999 and is not listed in `UserUsableGroups`, so using it would distort expected quotas. Docker fixtures will instead add unique group `codex_cache_write_20260711` with ratio 1 and a matching usable-group label, then remove both keys.
- The unique mock container is running on port 8080 inside the Compose network and returns model-selected overlap, zero, missing, and oversized native usage payloads without logging authorization headers.

## Docker Fixture Diagnostics
- The 403 failures are `insufficient_user_quota`, not routing, ratio, or billing failures. User id 2 currently has quota 0; token, abilities, channel, and all configured option keys are correct.
- The test token is unlimited, but positive cache-write scenarios require a larger pre-consumption amount than the zero/missing scenarios and still hit the user-quota guard. The user quota must be temporarily raised and restored during cleanup.
- The two successful rows each charged quota 1,062. Explicit zero persisted `reported=0/enabled=true/input_total=3619`; missing persisted no cache-write keys. This already confirms backend distinction while both remain eligible for frontend hiding.
- Rather than continue touching an existing user's balance, the fixture will transactionally reverse those two token-scoped charges on user id 2, delete their temporary logs, create a uniquely named disposable user with sufficient quota, and rebind the temporary token. This makes final cleanup exact.

## Docker Results
- All five final requests succeeded through the rebuilt app and network-scoped mock.
- Configured overlap: quota 4,884, reported/billed/creation tokens 3,616, enabled true, input total 3,619.
- Unconfigured overlap: quota 1,062, reported 3,616, enabled false, no billed creation fields, input total 3,619.
- Explicit zero: quota 1,062, raw reported 0 and enabled true, no billed creation fields; frontend positive-only flag hides it.
- Missing field: quota 1,062 and no cache-write metadata.
- Oversized write (`3620 > prompt 3619`): quota 1,062, enabled false, no billed creation fields, and the expected ordinary-input warning was persisted and emitted once.
- Disposable user accounting exactly totals 9,132 quota across five requests. Application logs match the persisted rows and show the configured overlap formula path.
- Cleanup audit is clean: zero temporary logs/tokens/channels/abilities/users, no fixture option keys, no mock container, and the app is healthy.
- Existing user id 2 was restored exactly to quota 2,124 and used quota 999,000, confirming the initial two successful charges were fully reversed.
- Final `/api/status` returns `success=true`; `git diff --check` passes and no generated Docker/frontend artifacts are tracked in the product diff.

---

# Findings

## Current Direction
- Async image tasks now use `provider_direct_lease`.
- new-api selects the real image channel through the normal distribution path, including aggregate group child-channel selection.
- The task keeps `platform=58` to identify the image-handle async protocol, while `channel_id` is the real selected provider channel.
- image-handle no longer chooses provider from task metadata and no longer receives real `api_key` in submit payload.
- image-handle worker resolves a short-lived lease before execution and directly calls the real upstream.

## Key Constraints
- Existing synchronous `/v1/images/generations` and `/v1/images/edits` must keep working.
- Existing video tasks, Suno/MJ tasks, task logs, asset center, and asset API key flow must not regress.
- `api_key/base_url/model` come from the locked real new-api channel.
- The lease table stores only `channel_id`, not plaintext credentials.
- Resolve HMAC secret and callback HMAC secret must be separate.
- Callback `raw_response` is small JSON only; large base64 fields must be scrubbed by image-handle and are capped by new-api.
- Callback and polling both flow through `service.ApplyTaskResult`, whose success path uses a DB transaction for terminal task update + assets creation.
- image-handle edit task payload still only accepts `input.images` and `input.mask` as HTTP(S) URLs.
- For image-handle sync edits, multipart files must be uploaded to `/v1/image/uploads`; JSON base64/data URI inputs must be uploaded to `/v1/image/uploads/base64`.
- Upload responses expose `images []string` and optional `mask string`; new-api should feed those URLs into the later edit task and should not submit edit if upload fails.

## Files Of Interest
- `/Users/zhangyu/code/go/new-api/model/image_credential_lease.go`
- `/Users/zhangyu/code/go/new-api/controller/image_credential_lease.go`
- `/Users/zhangyu/code/go/new-api/controller/task_callback.go`
- `/Users/zhangyu/code/go/new-api/relay/relay_task.go`
- `/Users/zhangyu/code/go/new-api/relay/channel/task/imagehandle/adaptor.go`
- `/Users/zhangyu/code/go/new-api/relay/image_handle_sync.go`
- `/Users/zhangyu/code/go/new-api/service/image_handle_executor.go`
- `/Users/zhangyu/code/go/new-api/docs/image-handle-new-api-internal-executor.md`
