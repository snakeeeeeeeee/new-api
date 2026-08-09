# Leonardo normalization error projection (2026-08-08)

- Leonardo submission error projection already trusts `reference_media_normalization_failed`, but
  `service/video_task_public.go` omitted it from code, message, and upstream diagnostic allowlists.
- Without the public projection update, later task queries degraded the trusted error to
  `video_task_failed` even though initial submission retained the specific code.
- The submission boundary now preserves only the trusted Adobe/Leonardo media-validation allowlist;
  arbitrary Leonardo upstream errors remain masked.

---

# Reference-video Per-second Surcharge Findings (2026-08-10)

- Current `VideoPricingProfile` stores only `unit_price`; current billing computes `unit_price * seconds * group ratio` and cannot distinguish reference-video requests.
- The normalized public request explicitly separates `input.reference_videos` from `input.video`, which supports the requested narrow trigger.
- `VideoBillingEstimate` currently carries only seconds and basis; extending this provider-neutral estimate with a reference-video boolean avoids coupling pricing code to provider payload shapes.
- The existing immutable `VideoPricingSnapshot` is the correct audit boundary because configuration changes must not reprice in-flight asynchronous tasks.
- The admin editor and marketplace currently expose only one USD-per-second price and explicitly describe reference media as not affecting billing; both descriptions must change together.
- The term `libtv` is not yet verified as an exact provider identity; research must not silently assume it means LiblibAI.
- The normalized request is available centrally in `RelayTaskSubmit` before pricing and after provider validation, so surcharge detection can remain a single `len(normalizedRequest.Input.ReferenceVideos) > 0` check instead of changing every provider adaptor.
- Public marketplace pricing receives `PublicVideoPricing`; adding the configured surcharge there lets table, card, and model-detail views describe the conditional price without exposing task-private billing data.
- Existing log summaries treat snapshot `unit_price` as the charged rate. New snapshots need an explicit effective rate and frontend fallback to `unit_price` for historical logs.
- Marketplace fixed prices are already multiplied by the selected group ratio and converted through `displayPrice`; the reference surcharge must use the same path rather than showing the raw configured USD value next to a converted base price.
- The existing VideoPricing editor can fit the surcharge as a fourth responsive template field and a preview switch without introducing a new panel or nested card.
- Local Docker already has healthy application and async-video mock containers plus running PostgreSQL/Redis; only `new-api-dev` needs rebuilding for this feature.
- Rebuilt Docker image `sha256:dfdec4add0c...` exposes the configured model through `/api/pricing` with `unit_price=0.03` and `reference_video_unit_price=0.02`.
- Three successful four-second mock tasks produced exact quotas `60000`, `100000`, and `100000` for zero, one, and two `input.reference_videos` entries. Both one and two references persisted `effective_unit_price=0.05` and `reference_video_applied=true`, proving the surcharge is request-level rather than per-item.
- Consumption logs persisted the same immutable snapshots and explicit Chinese text: the reference cases say `$0.050000/秒（基础 $0.030000 + 参考视频附加 $0.020000）`; total user/token quota movement was exactly `260000`.
- The existing Leonardo mock fixture used a deprecated `vip` group because its ratio and display keys were absent. Docker acceptance temporarily restored only those two absent keys with ratio `1`; exact cleanup must remove them again.
- The rebuilt public `/pricing` marketplace loads without authentication in the in-app browser. Its model list is virtualized, so focused filtering is required before asserting the configured Leonardo row/card/detail text.
- Focused card-view QA shows the configured Leonardo model as video per-second pricing with `参考视频附加价 +$0.001 / 秒`. Both base and surcharge are consistently transformed by the marketplace's current recharge-price display conversion, rather than mixing raw `$0.03/$0.02` configuration values with converted output.
- Marketplace table QA renders separate `基础每秒价格` and `参考视频附加价` lines; the model-detail `vip` group row repeats the same converted base/surcharge values and retains the video-per-second and wallet-only labels.
- The in-app browser's `canvas-denied-e2e` state referenced deleted user ID `994213`; protected routes correctly returned `/forbidden`. Desktop administrator QA used disposable root user `994214`, which was logged out and deleted afterward.
- The browser viewport capability clamps a requested 375px override to an observed 560 CSS pixels. At that actual width the marketplace has no document-level horizontal overflow (`scrollWidth == clientWidth == 560`); table price content remains in the table's own horizontal scroll region. This must be reported as a 560px check, not a claimed 375px result.
- After the user's display follow-up, rebuilt desktop card, table, and detail views all render one consistent formula: `(基础价格 $0.002 / 秒 + 参考视频附加价 $0.001 / 秒) = $0.003 / 秒` under the current marketplace conversion. The shared helper computes the effective value before display formatting.
- Mobile QA was explicitly removed from scope by the user; the earlier viewport observation is retained only as an audit note and is not a delivery criterion.
- Desktop administrator QA confirms the new `$0.02/second` input and preview switch. A five-second preview changes from `$0.03 × 5 = $0.15` to `($0.03 + $0.02) × 5 = $0.25` when reference video is enabled.
- Desktop usage-log QA shows the no-reference request as `$0.030000 / second`, while both reference requests show `($0.030000 + $0.020000) / second` plus `已应用参考视频附加价`.
- With the configured surcharge temporarily set to zero, marketplace card, table, and detail views all return to the legacy single-rate text. Backend `reference_video_applied` now also remains false when the configured surcharge is zero, so zero/absent configuration keeps legacy logging behavior.
- Volcengine's official Seedance 2.x LAS operator documentation confirms that requests with input video have a different billable formula, but it is not a fixed surcharge: it combines input duration, output duration, and a resolution coefficient. The documented example is `2 RMB/second * [max(5s, 7s*2/3) + 7s] / 2 * 3.040 = 36.48 RMB` for a five-second input and seven-second 1080p output. This is LAS-specific and does not prove that every Jimeng/Ark surface uses the same formula: https://www.volcengine.com/docs/6492/2595411
- LibTV is now verified as LiblibAI's product. Its official homepage advertises a distinct `720P 参考视频生成` per-second promotional price, currently as low as `0.4 RMB/second`, but neither the homepage nor its public package page exposes evidence that settlement is literally `base + surcharge`; the public evidence supports a separate total reference-video rate only: https://www.liblib.tv/ and https://libtv.gongke.net/pricing/
- Final cleanup removed the three exact Task, Asset, idempotency-request, consumption-log, and Webhook-event rows; restored user `99011`, token `153`, and channel `128` counters; restored the original `$0.01/second` VideoPricing without a surcharge field; removed the temporary `vip` keys and administrator; reset mock metrics; and left Docker healthy with no startup errors.

---

# Multi-provider Async Video Final Findings (2026-07-23)

# Video Error Diagnostic Findings (2026-08-05)

## Requirements
- Administrators must be able to inspect every upstream video-task failure after sensitive-data transformation.
- Public task queries and failure Webhooks must return useful safe messages, including previously unknown error types.
- Sensitive provider account IDs, balances, credentials, signed URLs, platform/channel details, and raw structured responses must not enter public payloads.

## Confirmed Current Behavior
- `updateVideoSingleTask` stores `redactVideoResponseBody(responseBody)` in `task.Data`, bounded to 64 KiB.
- The Leonardo adaptor discards messages for error codes outside a four-code allowlist and stores `Video task failed` as `FailReason`.
- `buildPublicVideoTaskError` and outbound video Webhooks already share the central public projection, but the useful message has already been discarded by the adaptor.
- Existing recursive response redaction masks secret-valued keys but does not comprehensively sanitize sensitive values embedded inside arbitrary strings.
- A provider credit error can contain an internal account UUID plus available/required balances; it must be retained for administrators but projected publicly as provider-neutral capacity unavailability.
- Administrator task responses are produced through `relay.TaskModel2Dto` and `controller/tasksToDto`; the existing DTO already includes `fail_reason` but no structured upstream diagnostic.
- The administrator task-log frontend currently renders `fail_reason`; it has no field for the sanitized provider code/message/status retained in `task.Data`.
- `dto.TaskDto` currently exposes the entire redacted `Data` field to task-log callers, but the UI only shows it through the generic record JSON modal; a bounded structured diagnostic is clearer and avoids making the frontend parse provider payloads.
- The task-log detail column is shared by administrators and ordinary users. Administrator-only diagnostic rendering must be gated by `isAdminUser`, while `fail_reason` remains the existing ordinary-user surface.
- `relay.TaskModel2Dto` is context-free and currently copies `task.Data` for all callers; administrator diagnostics should be attached by the administrator controller path rather than globally in this converter.
- The existing generic content modal can display bounded diagnostic text, so a separate nested modal is unnecessary; the detail column can open a formatted administrator diagnostic while retaining the compact table layout.
- `GetAllTask` is the administrator list (`tasksToDto(..., true)`), while `GetUserTask` uses `false`; this is the correct boundary for adding/removing `upstream_error` without changing public Resource API DTOs.
- `UpdateTaskBlockStatus` is administrator-only but returns `TaskModel2Dto` directly, so it must use the same administrator DTO helper to avoid losing diagnostics after block/unblock updates.
- Existing service-level secret sanitizers (`sanitizeErrorSnapshotText` and diagnostic-map traversal) can be reused inside the `service` package; no duplicate credential regex is needed.
- Public video tests currently assert all unknown provider errors become generic. These tests must be replaced with two cases: unknown safe messages survive, while sensitive/internal messages are transformed or masked.
- Existing Webhook tests already exercise the send-boundary sanitizer for stored legacy failures, providing the correct place to assert public Task/Webhook equality.
- The repository already has a stronger `sanitizeErrorSnapshotText` path that masks URLs/IPs/domains and secret assignments. It is suitable as the first pass for public unknown-error text, but administrator diagnostics need a narrower sanitizer so operational names and context remain useful.
- `VideoTaskPublicError` already carries stable public code/message/retryable plus bounded diagnostic metadata (`upstream_status`, filtered `upstream_error_code`, `request_id`); no public schema field addition is required.
- `common.MaskSensitiveInfo` preserves message structure while masking URL hosts/paths/query values, IPs, and domain names; combined with secret-assignment masking it provides a useful unknown-message public sanitizer.
- Video task detection should accept either `Properties.AssetType == video` or the legacy action mapping so administrator diagnostics also work for older task rows.
- The first implementation review found known business messages still bypassed the new central sanitizer and administrator text did not mask bare Bearer/JWT/email values; both need hardening before broad tests.
- The design document was created under `docs/plans`, but that directory is ignored by the local repository configuration, so it is working documentation unless deliberately force-added later.
- Final data-flow review found that public projection still sourced its message from `task.FailReason` after generic nested extraction. When that field was already generic, administrators saw the extracted detail but public Task/Webhook output did not. Public projection now uses the centralized diagnostic message first and falls back to `FailReason` only when extraction yields no message.

## Technical Decisions
| Decision | Rationale |
| --- | --- |
| Preserve the provider message through adaptor parsing | Central policy cannot evaluate text that an adaptor has already discarded. |
| Keep the redacted diagnostic snapshot in existing `task.Data` | Avoid a cross-database migration and reuse current polling persistence. |
| Build administrator diagnostics from structured fields in `task.Data` | Show useful context without exposing the entire response body. |
| Sanitize unknown messages centrally instead of using a finite code allowlist | Future provider errors remain useful without a release for each new code. |
| Use one public error builder for REST and Webhooks | Prevent divergent disclosure and retryability behavior. |

## Visual Finding
- The supplied new-api task-log screenshot shows a failed Leonardo task whose visible reason is only `Video task failed`; the upstream Leonardo2API task detail contains `originalFilename is required for audio uploads`.

---

- The normalized public contract is provider-neutral: public requests, tasks, Assets, and Webhooks do not expose channel IDs, platform IDs, upstream task IDs, quota, provider cost, raw responses, or internal Asset metadata.
- Current official xAI documentation limits 1080p to `grok-imagine-video-1.5` image-to-video generation. The final adaptor therefore also rejects 1080p text-only and reference-image generation; compatibility `/v1/videos/*` payloads remain unvalidated passthrough.
- The local channel 109 is xAI-compatible sub2api, not direct xAI. Its completed task response exposes `/v1/videos/6d75b77c-fe60-9641-91a2-2f29ad076852/content`, and anonymous access to that upstream endpoint returns 401.
- Because that URL is relative and channel-authenticated, returning it as a public CDN URL would be incorrect and would leak an unusable address. The normalized result correctly projects `/v1/assets/{asset_id}/content` with `url_auth=resource_api_key`; the Asset proxy attaches channel auth server-side.
- This means the public result URL cannot be supplied directly to a remote provider as an edit input. The approved contract explicitly does not promise that `ak_`-protected Asset URLs are upstream-readable.
- A real data-URL edit and a real `{provider:"xai",file_id}` edit both reached channel 109 and were immediately rejected with the same blank sub2api upstream error. The checked local latest sub2api source registers Grok video generation/status routes but no usable edit/extension route.
- Successful edit acceptance therefore requires a channel whose upstream actually implements xAI `/v1/videos/edits` and can consume the chosen input source. Adding new-api-side public unauthenticated media hosting would contradict the approved authenticated temporary-resource design.
- Immediate upstream rejection still creates a durable failed normalized task before returning the synchronous error. Query, Webhook, and idempotent replay all work: each terminal transition creates one event, and same-key replay returns the original task without duplicate delivery.
- Real generation evidence confirms the important external workflow: create with normal Token, poll/query with `ak_`, discover the video Asset, and download either Asset or legacy task content with Range. Both download routes returned HTTP 206 with `video/mp4`.
- Docker UI verification was desktop-only by request. At 1280x720 the document width stayed exactly 1280 pixels and no visible-overflow element widened the page.

---

# Adobe2API Seedance 2.0 Fast Integration Findings (2026-07-29)

- The Adobe2API repository is `/Users/zhangyu/code/myProject/supertoken-projects/adobe2api`; its latest relevant commits are `bb1b37d` (Adobe model compatibility/runtime hardening) and `537b825` (Seedance 2.0 API design).
- The documented variants are `seedance_2.0_fast` and `seedance_2.0`, with expanded aliases `firefly-seedance2-fast-{duration}s-{ratio}-{resolution}` and `firefly-seedance2-{duration}s-{ratio}-{resolution}`.
- Both short aliases default to 4 seconds, 16:9, and 480p. Documented durations are every integer second from 4 through 15.
- Fast documents 480p and 720p support; Standard additionally documents 1080p. These are provider capabilities, not billing multipliers.
- Adobe2API exposes video generation through `/v1/chat/completions`; Seedance uses Adobe Firefly's internal web protocol and Cookie-imported accounts, so a real acceptance call may incur account cost and depends on current entitlement/protocol compatibility.
- The actual upstream payload uses `modelId=seedance`, `modelVersion=seedance_2.0_fast` or `seedance_2.0`, an integer `duration`, a resolution-derived `size`, `generationSettings.aspectRatio`, `generateAudio`, and reference blobs.
- Adobe2API submits to Adobe's async endpoint, polls every three seconds, downloads the completed video, and only then returns the Chat Completions response. Its upstream is asynchronous, but its public video contract is currently blocking.
- The only existing Adobe2API public async submit/query pair is `/api/v1/generate` for images. It uses an in-memory JobStore and background thread; there is no public asynchronous video query or content endpoint yet.
- A direct new-api adaptor over `/v1/chat/completions` would block task creation until completion and leave no valid polling endpoint, so it cannot satisfy the asynchronous acceptance requirement.
- Reusing new-api's Sora adaptor would require Adobe2API to implement `/v1/videos`, status, and content endpoints, but the shared Sora normalized contract currently cannot cleanly carry all Seedance-specific aspect-ratio, audio, and reference semantics. A dedicated AdobeVideo adaptor keeps those provider decisions isolated.
- The local Adobe2API container is healthy on port 6001 and shares the `ai-gateway` Docker network with `new-api-dev`.
- The local account inventory contains one active auto-refresh account with known positive credits, no credit error, and no temporary block. No credential values were read into planning output.
- The live `/v1/models` endpoint returns 362 Seedance entries: 144 Fast combinations, 216 Standard combinations, and two short aliases.
- Local request logs contain multiple successful `seedance_2.0_fast` tasks and a successful Standard 1080p task, each with an upstream job ID and completed video preview. Existing Seedance MP4 artifacts are valid ISO MP4 files; the inspected Fast artifact reports 4.042 seconds.
- A network-disabled container run with read-only source and writable tmpfs data passes all 13 Seedance tests. The earlier 451-to-500 result was a test-harness artifact caused by denying writes to `data/request_errors.jsonl`, not a business-code regression.
- With a temporary test price of `$0.03/second`, four seconds and group ratio `1.0` should charge `60,000` quota at the configured `QuotaPerUnit=500,000`. A mock test at group ratio `1.5` should charge `90,000`, proving the runtime group ratio is inherited rather than configured in VideoPricing.
- Adobe2API's generation router already receives every dependency needed by an asynchronous video worker: account scheduler, retry wrapper, progress callbacks, generated-file URL builder, storage accounting, provider error classes, and the shared Adobe client.
- The existing `JobRecord` and `/api/v1/generate` contract are image-specific. A separate `VideoJobRecord`/`VideoJobStore` avoids changing the legacy image response shape while keeping thread-safe bounded in-memory storage for the first local integration.


# Per-second Video Billing and Subscription Eligibility Findings (2026-07-29)

## Requirements
- Support per-generated-second charging for Seedance and xAI video models.
- Add per-video-model subscription-quota eligibility, default false.
- When eligibility is false, charge wallet balance only rather than consuming subscription quota.
- Determine whether the existing model-pricing subsystem is the right ownership boundary and identify all affected layers.

## Initial Constraints
- Preserve SQLite, MySQL, and PostgreSQL compatibility.
- Preserve explicit zero values in upstream video request DTOs.
- Keep existing provider/channel compatibility routes working.
- No business-code edits are authorized in this investigation turn.

## Confirmed Findings
- Subscription selection is part of the relay billing state, not merely frontend presentation. `relay/common/relay_info.go` records billing source, selected subscription ID, preconsumed amount, and post-settlement delta.
- Async task relay copies the selected subscription ID into task private data, so video task completion already participates in subscription-aware settlement.
- The requested subscription restriction therefore has to be evaluated before billing preconsume selects a source; a model-pricing UI-only flag would not enforce wallet-only charging.
- Current `main` HEAD is `08e1e9732` (`feat: expand Gemini image output controls`); the recent async public-quota sanitization work is adjacent but does not establish video pricing behavior.
- A centralized `service.BillingSession` already owns funding-source preconsume, final delta settlement, and refunds. It delegates to either wallet or subscription funding, making this the likely enforcement boundary for wallet-only models.
- Async task settlement is already source-aware through `service/task_billing.go`; subscription tasks apply their final delta to the persisted `user_subscriptions` row.
- The task adaptor contract already has billing estimation hooks: Gemini/Vertex video adaptors derive `OtherRatios` from video duration and resolution. Per-second billing should reuse this generic task-pricing path rather than add provider-specific deductions.
- xAI normalized requests preserve requested duration, and completed xAI task parsing exposes provider-reported duration as `DurationMS`. This gives both a precharge estimate and a completion-time authoritative value for generation; edit inheritance needs separate handling because request duration may be absent by design.
- `NewBillingSession` currently follows the user's global preference (`subscription_only`, `wallet_only`, `wallet_first`, `subscription_first`) and has no per-model eligibility input. Wallet-only enforcement can be implemented by overriding the allowed source set before this switch, while retaining normal preference behavior for eligible models.
- `TaskAdaptor` explicitly supports three billing phases: estimate from the request, adjust from submit response, and adjust from completed task result. The existing API already documents `{"seconds": N}` as a valid multiplier.
- Durable video tasks snapshot base price, group ratio, `OtherRatios`, billing source, subscription ID, and a `PerCallBilling` boolean. This is sufficient for deterministic completion settlement if the duration multiplier is included in the snapshot.
- xAI is currently hard-coded into per-call classification (`ChannelTypeXai`) both when storing task billing context and when rendering task logs. This does not block an adaptor's completion adjustment (that hook runs first), but it would mislabel per-second pricing and should be replaced by explicit billing-mode metadata.
- Task submission currently computes `base ModelPrice x group ratio`, merges adaptor `OtherRatios`, multiplies all ratios (including seconds), and preconsumes that result before calling upstream. Submit-time adjustments can recalculate the quota, while completion-time adjustments run in the polling service.
- The existing `ModelPrice` setting is only a flat `map[model]float64`; it has no unit or billing-mode metadata. Reusing its numeric value as “USD per second” works arithmetically, but the admin/pricing UI would still mislabel it as a fixed per-call price unless a billing-mode/unit setting is added.
- Video task billing snapshots are already durable and cross-database because they live in task JSON private data; no new SQL column is required merely to preserve requested seconds and base price.
- Completion settlement can adjust wallet or the originally selected subscription consistently through `RecalculateTaskQuota`; the funding source is frozen at submit time, which is desirable for deterministic async settlement.
- Subscription plans currently have only total quota/reset/group fields and no model allowlist. `PreConsumeUserSubscription` accepts `modelName` and `quotaType` but does not use either when selecting a subscription; it chooses by active status, expiry, and remaining total quota.
- Therefore, “subscription disabled for this model” is best enforced before `NewBillingSession` selects a funding source. Implementing it only inside a specific video adaptor would leave other routes/aliases able to consume subscriptions.
- Both providers expose duration data: Seedance/Doubao task results include `duration`; xAI completed video results include `video.duration`. Neither adaptor currently uses that result to return a completion-time actual quota.
- Seedance/Doubao embeds the no-op `BaseBilling`, so it currently has no duration estimate or completion adjustment. Its success parser records total tokens but does not persist a video `DurationMS` output, even though the upstream result provides duration.
- xAI also embeds `BaseBilling`; request validation already normalizes duration into `xai_video_request`, so estimate extraction can be added without reparsing the body. Generation/extension may supply duration, while edit intentionally omits it and must rely on an estimate plus completed result correction.
- No existing generic per-second billing-mode setting was found. Existing `BillingMode` fields are scoped to token-tier or image-pricing snapshots, not async video models.
- A backward-compatible configuration should distinguish legacy models from explicitly configured video models. A nullable/tri-state policy (`unset` = legacy behavior, configured video default `subscription_enabled=false`) avoids accidentally disabling subscriptions for every existing non-video model.
- For configured per-second video models, the numeric `ModelPrice` can remain the one-second base USD price, but a separate task/video billing metadata map is needed to identify the unit and subscription policy and to render it correctly in admin/public pricing surfaces.
- The controller settles submit-time quota and persists the task before polling. On terminal success, the task status/Assets/Webhook transaction commits before completion billing adjustment.
- Video completion settlement currently does not allow debt. If actual duration costs more than the precharge and wallet/subscription remaining quota is insufficient, the funding adjustment fails and the already-successful task remains undercharged, with only an internal error log.
- Therefore, normal generation should precharge the full request-duration cost. For operations with unknown duration (notably xAI edit), a conservative maximum-duration reservation followed by refund-only reconciliation is safer than a low estimate followed by a potentially failing supplement.

## Validated Design Direction
- Resolution is deliberately represented by exact model aliases (for example, `xxx-720p`); there is no resolution multiplier and billing code must not parse resolution from the model name.
- For successful generation, validated request duration is the billable quantity. Provider-reported media duration is retained for audit/anomaly detection rather than automatically changing the charge.
- The formula is `ModelPrice (USD/second) x requested seconds x effective group ratio x QuotaPerUnit`.
- Keep `ModelPrice` as the numeric source and add a parallel exact-model metadata setting such as `ModelBilling`, with `billing_mode=per_second` and `subscription_enabled=false` by default for video.
- When a video model does not explicitly enable subscription billing, set a relay billing policy to wallet-only before `PreConsumeBilling`; do not let `subscription_only` or fallback preferences bypass the model policy.
- Seedance and xAI adaptors should implement `EstimateBilling` by returning only `{"seconds": duration}`. No new provider-specific deduction path is needed.
- Require a valid duration for per-second generation. Operations whose billable seconds are not present in the request should remain fixed-price or unsupported for per-second mode until a safe reservation rule is defined.
- Store `video_per_second` in the existing task `BillingMode`, preserve `seconds` in `OtherRatios`, and expose the unit plus subscription support through the existing pricing DTO `BillingType` surface.

## Final Approved Implementation Corrections
- Use a standalone versioned `VideoPricing` profile/binding setting, parallel to `ImagePricing`, rather than overloading the flat `ModelPrice` map or introducing a generic ModelBilling setting.
- A profile owns the USD-per-second unit price; an exact model binding optionally references a profile and always owns `subscription_enabled` (default false).
- Policy-only bindings are allowed so a legacy fixed-price video model can opt into subscriptions without changing its price.
- Bound per-second requests require an explicit duration. Provider defaults are deliberately not used because routing the same public model to providers with different defaults would make billing channel-dependent.
- Billing uses a strong adaptor result (`Seconds`, `Basis`) and a dedicated immutable `VideoPricingSnapshot`; legacy `EstimateBilling` remains available for unbound compatibility behavior.
- The upstream-reported duration is audit-only and cannot reprice a successful request.
- Existing ImagePricing establishes the reusable implementation pattern: typed config in `types`, atomically replaced state in `setting/ratio_setting`, Option validation/update hooks, a `PriceData` snapshot, durable task private JSON, and a redacted `/api/pricing` projection.
- `PriceData` needs a dedicated `VideoPricing` pointer so bound pricing bypasses the legacy unordered `OtherRatios` multiplier path and cannot accidentally apply resolution or size multipliers.
- Durable normalized video tasks and legacy task rows already serialize `TaskBillingContext` as JSON, so adding a `VideoPricing` pointer remains cross-database and migration-free.
- Funding-source selection is centralized in `NewBillingSession`; a request-scoped billing-preference override on `RelayInfo` can enforce wallet-only before any subscription preconsume while leaving eligible models on the user's existing preference/fallback behavior.
- Terminal task billing checks adaptor/token adjustments before its generic per-call guard. VideoPricing therefore needs an explicit first-priority terminal branch; otherwise Seedance completion tokens would overwrite the approved request-duration price.
- The VideoPricing editor's group-ratio input is a non-persisted preview operand only. Runtime billing obtains the effective ratio from `HandleGroupRatio`, including aggregate-group and exact route/model overrides, then freezes it in `VideoPricingSnapshot`.
- The rebuilt Docker UI now labels the input `预览分组倍率`, while the existing profile, exact binding, policy-only binding, wallet-only policy, and user-preference policy all render correctly from the saved test configuration.
- Responsive Docker QA at 1440, 768, and 375 CSS pixels reports identical document/client widths and no horizontal overflow. The preview label is unique in the DOM and remains within the viewport bounds at each measured breakpoint.
- The live Docker `default` group ratio is `999` while the editor preview remains `1`. For the QA profile (`$0.03/second`) and a 5-second request, runtime precharge must therefore be `0.03 x 5 x 999 x 500000 = 74,925,000 quota`; this provides a strong separation check between the preview operand and effective billing ratio.
- Live wallet-only QA charged exactly `74,925,000` quota from the wallet despite the user's `subscription_only` preference, left subscription usage at zero, and persisted `billing_source=wallet`, `group_ratio=999`, and the immutable per-second snapshot.
- With the exact binding changed to `subscription_enabled=true`, the same user and request left the wallet unchanged, consumed exactly `74,925,000` subscription quota, and persisted `billing_source=subscription` with the policy flag frozen true.
- xAI mock requests received mapped model `grok-imagine-video` and integer durations unchanged. Compatibility missing/zero/fractional durations and normalized missing/fractional/provider-option override attempts all failed with HTTP 400 before any upstream POST.
- `/api/pricing` exposes the test model as `per_video_second` with unit `second`, unit price `0.03`, and the current subscription-enabled state.
- Live log inspection exposed a residual xAI `，按次计费` suffix after the explicit per-second detail. The explicit image/video pricing branches now bypass that legacy xAI suffix and a regression assertion covers the conflict.

## Approaches Considered
| Approach | Assessment |
| --- | --- |
| Parallel model billing metadata plus existing `ModelPrice` | Recommended: backward-compatible, exact-model scoped, no database migration, and UI can label `$/second` correctly. |
| Provider-specific hard-coded Seedance/xAI deductions | Rejected: duplicates the generic task billing path and breaks aliases/model mapping. |
| Parse resolution/price semantics from model names | Rejected: model names should remain opaque configuration keys. |
| Add SQL columns to channels or subscription plans | Rejected for this requirement: policy is model-level and existing option storage/cache is the established ownership boundary. |

---

# Image-handle Trace Search and Task Table Findings (2026-07-23)

- new-api generates a synchronous image `client_task_id` and credential `lease_id` before submitting to image-handle, but currently writes the provider task ID and lease ID into Gin context only after success.
- Structured image-handle failed responses already carry `provider_task_id`, `client_task_id`, and `task_id`; new-api stores these under `logs.other.image_handle_sync_error`, but the usage-log UI does not display the nested identifiers.
- A transport failure before an image-handle response loses the locally available `client_task_id`/`lease_id` from the new-api error log.
- image-handle PostgreSQL already persists `request_id`, `client_task_id`, and `provider_task_id`; `provider_task_id` is the primary key and `client_task_id` is indexed, while `request_id` lacks an index.
- The image-handle admin task endpoint currently supports pagination only. Its record projection already returns all three identifiers, task timestamps, attempts, parameters, usage, and error summary.
- The task table overflow shown by the user is caused by unbounded error/content cells competing with timestamp and URL columns. Stable tracks, ellipsis, and full-value inspection are required rather than hiding diagnostic content.
- Task duration can be derived from existing `started_at` and `finished_at`; active tasks can use current time. No schema column is needed.
- UI/UX guidance selected for this operational table: stable data tracks, no page-level horizontal overflow, readable ellipsis with tooltip/detail access, exact search feedback, keyboard-accessible controls, and responsive validation.
- image-handle's admin task API currently calls `getAdminTasksPage(page, pageSize)` with no filter object; the React task table already receives `request_id`, `client_task_id`, `provider_task_id`, `started_at`, `finished_at`, and `updated_at`.
- The existing admin task projection intentionally exposes only safe execution metadata and not prompts or credentials, so trace filtering can stay within the existing redacted response contract.
- The admin UI uses one large React entry file and a shared CSS table system. The task overflow fix should be scoped with a task-table class so other operational tables retain their current behavior.
- The image-handle admin route is session-protected. Extending it is safer than adding alternate ID lookup to the public provider-key task endpoints.
- Final new-api error logs now expose `image_handle_sync_request_id`, `image_handle_sync_client_task_id`, `image_handle_sync_provider_task_id`, and `image_handle_sync_credential_lease_id` whenever available. Transport failures retain the locally generated IDs even without an image-handle response.
- The image-handle administrator query uses parameterized exact matching across the three indexed identifier columns. Public task lookup behavior is unchanged.
- Desktop geometry confirms the 1,348px error text is clipped within a 320px error column; the adjacent 90px duration and 170px updated-time columns do not overlap.
- At a 390px viewport the document width remains 390px, the search form ends at 353px, and the 2,900px task table remains contained in its 314px horizontal scroller.
- The isolated UI fixture produced zero browser console errors and was stopped after acceptance.

---

# Task Log Public Video URL Follow-up Findings (2026-07-23)

- The screenshot shows `/v1/videos/b0c80724-3aee-9916-b53d-c758bc6cacb1/content`, which is the stored upstream UUID path rather than the public new-api task path.
- The previous fix only changes xAI `ConvertToOpenAIVideo`, used by `GET /v1/videos/{task_id}`. The dashboard task log uses `TaskModel2Dto`, a separate response path.
- `TaskModel2Dto` currently assigns successful `result_url` directly from `task.GetResultURL()`, so the frontend faithfully receives and displays the internal relative URL.
- The frontend task-log video modal is not the correct normalization boundary: preview, copy, and open actions all consume the same DTO field, and other clients can consume the task API too.
- Replacing the database value would break `VideoProxy`, which needs the stored upstream relative or absolute URL. The public conversion belongs only in outward task DTO serialization.
- The task-log frontend's `buildVideoResultUrl` already falls back to `/v1/videos/{task_id}/content`, but only when `record.result_url` is empty. Any stored upstream URL therefore overrides the correct fallback.
- Preview, browser-open, and modal copy actions all reuse that same selected URL, so one backend DTO correction fixes every task-log video action without frontend changes.
- `constant.TaskActionAssetType` already defines the same complete video-action set as the task-log frontend, including legacy provider actions and the OpenAI-style generation/edit/extension actions.
- The dashboard DTO should use a relative public path, not `ServerAddress + path`: a relative URL preserves same-origin session cookies when Docker is opened as `localhost` even if the configured server address is `host.docker.internal`.
- The preview modal uses a native `<video src>` element. A same-origin relative proxy path automatically carries the dashboard session cookie; no custom frontend authorization or Blob transport is needed.
- Both administrator `/api/task/` and user `/api/task/self` lists serialize through `TaskModel2Dto`, so the DTO fix covers both task-log views.
- Docker serves the rebuilt page correctly, but the available in-app browser has no local authentication state and redirects `/console/task` to `/login`; no alternate browser session is available.
- The disposable account's administrator role is active, as shown by administrator-only task table columns, but the existing per-admin menu permission layer rejects the task API until its task-log menu key is granted.
- The exact guarded permission for administrator task listing is `async_task`; granting only that key is sufficient for this fixture.
- The rebuilt table displays both xAI task rows without new console errors. Browser automation clicks on the icon-only preview button do not update React modal state in this environment, so endpoint-level browser evidence is more reliable for final acceptance.
- Direct navigation in the same authenticated browser session to `/v1/videos/task_dwEC.../content` creates a native video element with `readyState=4`, no media error, duration 5.041667 seconds, and decoded dimensions 848x480.
- A real authenticated `GET /api/task/` for the existing task returns `result_url=/v1/videos/task_dwECb8BLNtzhUNm8taOUgknsekhWgmk5/content`, exactly matching the public task ID path.
- The disposable fixture cleanup is exact: user, menu permission, token, task, and log counts for user ID 994207 are all zero, and all isolated browser tabs were closed.

---

# xAI Video Provider Compatibility Findings (2026-07-23)

- Local Docker channel 109 reaches a sub2api-compatible xAI video upstream and successfully completes `grok-imagine-video-1.5` tasks.
- The current adaptor normalizes canonical `grok-imagine-video-1.5` to legacy `grok-imagine-video`, despite channel model mapping already being the intended provider-specific translation boundary.
- The upstream completed response stores `/v1/videos/{upstream_uuid}/content` as a relative result URL; the old video proxy passes it directly to `http.Client`, causing `unsupported protocol scheme ""` and a public 502.
- Querying new-api with the upstream UUID is expected to return `Task not found` because public task lookup is keyed by the generated `task_...` ID.
- Direct authenticated retrieval of the same relative upstream content succeeds with HTTP 206 and MP4 bytes, proving generation and upstream storage are healthy.
- The public xAI status representation must override the stored upstream result with `/v1/videos/{public_task_id}/content`; the proxy remains responsible for resolving and authenticating the actual upstream location.
- Absolute cross-origin result URLs are treated as CDN locations and receive no channel key. Relative and absolute same-origin locations use the configured channel Bearer key.
- The first post-fix content download reached the correct upstream URL but exposed the old proxy's fixed 60-second context limit on a larger generated MP4; video transfers need a longer bounded window than ordinary API responses.
- sub2api's completed task representation reports its internal model as `grok-imagine-video` even when new-api sends canonical `grok-imagine-video-1.5`. This upstream response field cannot be used as evidence of new-api request normalization; the outbound adaptor boundary is covered directly by request-body tests.

---

# Async Image Final Usage Log Reconciliation Findings (2026-07-22)

- The screenshot's balance is financially correct: `$0.500000 - $0.458664 = $0.041336` actual charge, but the real `5/196` tokens are attached to the refund delta row.
- Refund rows are excluded from consume-only usage aggregation, so over-precharge tasks can report the precharge instead of the actual quota and omit terminal tokens; under-precharge tasks can create two consume rows and inflate request counts.
- The original consume log already captures the submission Request ID from Gin context, while `RecordTaskBillingLogParams` has no Request ID and creates the blank asynchronous settlement row.
- `TaskBillingContext.ConsumeLogId` provides a direct guarded association to the original row, but callback completion can race with submit-side persistence of that ID.
- The approved user view is one consume row per image task, with financial delta events retained as metadata instead of separate usage-log rows.
- `ApplyTaskResult` wins the terminal status CAS and persists task result data before financial settlement; final log state can therefore be snapshotted afterward without reopening billing ownership.
- A fast callback can settle while `consume_log_id` is still zero. The final log snapshot must be stored in task private data, then replayed after `PersistTaskSubmitResult` attaches the original log ID.
- Image dispatch failures and timeout failures also call `RefundTaskQuota`, so image-specific original-row reconciliation belongs in that shared function while generic task refund logs remain unchanged.
- The final implementation settles balances by delta but never emits a second user-facing async-image settlement row: success overwrites the original consume row with actual quota/usage, and failure overwrites it with quota zero.
- A persisted terminal snapshot plus guarded `consume_log_id` update closes both callback orderings: completion after submit updates immediately, while completion before submit is replayed after the original log association is stored.
- Request ID is captured in the task billing context and retained on finalization; the log updater only falls back to it when the original row is missing a Request ID.
- Real PostgreSQL E2E produced one successful task with exactly one consume log and no refund log: precharge `50000`, final quota `4913`, prompt/completion tokens `5/196`, matching Request ID, and request count `1`.
- No historical migration is required; reconciliation applies to newly settled async image tasks and leaves existing rows unchanged.
- Final review found timeout sweeping still gated reconciliation on nonzero quota; image tasks now enter failure reconciliation even at zero precharge, while the refund helper continues to skip all balance mutations when there is no amount to return.

---

# OpenAI Null Required Tool Schema Compatibility Findings (2026-07-22)

- The upstream 400 `Invalid schema for function 'knowledge_list_documents': None is not of type 'array'` identifies an invalid request-side function schema, most plausibly the JSON Schema keyword `required` carrying JSON `null` instead of an array.
- The upstream/sub2api emits the rejection, while new-api can prevent it by cleaning the outbound request before forwarding; it is unrelated to reserved function names.
- The repository's existing generic tool-schema compatibility can normalize multiple schema defects for Claude/AWS channels, so reusing it directly would violate the approved narrow OpenAI contract.
- OpenAI Chat Completions has two outbound paths in `relay/compatible_handler.go`: normal DTO serialization and raw body passthrough. Both must invoke the same targeted cleaner.
- `GeneralOpenAIRequest.Tools[].Function.Parameters` uses `any`, while legacy `functions` is retained as `json.RawMessage`; a raw structural cleaner applied after marshaling covers both without expanding DTO coercion.
- The compatibility setting is `global.openai_tool_schema_null_required_compat_enabled`; it is hot-reloadable, independent from reserved-function-name aliasing, and defaults to `false` when no option row exists.
- The cleaner walks only recognized JSON Schema child-schema positions. It removes a schema object's own `required` member only when the value is JSON `null`; valid arrays and other invalid types remain unchanged for upstream validation.
- Data-bearing keywords such as `default`, `const`, `enum`, and `examples` are intentionally opaque, so a nested object stored there cannot be mistaken for a child schema. Messages, tool arguments, content, and unrelated request JSON are outside the mutation boundary.
- Focused tests cover modern tools, legacy functions, nested properties/definitions/items/combinators/conditionals/additional-property schemas, raw passthrough, serialized requests, disabled behavior, and preservation of non-null or data-bearing values.
- The full backend suite, frontend production build, scoped ESLint/Prettier, seven-locale i18n status, and whitespace checks pass.
- Docker dev was rebuilt and is healthy at `http://localhost:3001`. Desktop UI verification confirmed the new OpenAI Compatibility switch starts off, toggles on, and toggles back off; mobile QA was intentionally not performed.
- The real disabled/enabled A/B used the same `gpt-5.4` payload, forced `knowledge_list_documents`, and included both top-level and nested `required: null` values through token ID 141 and channel 85.
- Disabled Request ID `20260722094557387149380GU70SYWZ` returned HTTP 400 with `Invalid schema for function 'knowledge_list_documents': None is not of type 'array'.`
- Enabled Request ID `20260722094558315168755H4btF3TY` returned HTTP 200 and a `knowledge_list_documents` tool call, directly confirming the outbound cleanup resolves this validator failure.
- Cleanup restored the original runtime state: the new option has no database row and therefore remains default-disabled, upstream-error passthrough is false, root access token is null, no disposable UI user remains, and Docker dev is healthy.

---

# OpenAI Reserved Python Tool Compatibility Findings (2026-07-22)

- Reproduction Phase 6 found no local match for production Request ID `202607220331435264898266qI0WOpT` in the Docker logs, PostgreSQL `logs` table, `logs-dev`, `tmp`, `outputs`, or `data-dev`. The original client payload is therefore unavailable locally and cannot be replayed byte-for-byte.
- The screenshot's sub2api UUID and the supplied new-api-style Request ID identify different hops of the production request. Local Docker contains an `error_snapshots` table, but the initial Request ID lookup did not find a corresponding local log row.
- The local `error_snapshots` table is empty, and no consume/error log contains the reserved-name message. There is no recoverable local client or upstream request fragment for the production failure.
- The bounded reproduction set is limited to modern `tools` with automatic and forced selection, legacy `functions/function_call`, and a multi-turn request carrying historical tool-call names. Repeating the same successful shape would only add cost without isolating a condition.
- A local sub2api checkout/container is available under `/Users/zhangyu/code/myProject/supertoken-projects/sub2api`, but new-api channel 85 targets remote `http://185.150.190.236:18888`; local container logs cannot reveal that remote deployment's account selection.
- Channel 85 advertises `gpt-5.4` directly and has one gateway key. Its local new-api model mapping is empty, so the model name is not changed before reaching remote sub2api.
- Read-only sub2api source inspection confirms it does not originate the reserved-name error and contains no `python` reservation rule. For OpenAI accounts that support Responses, it converts both modern `tools` and legacy `functions` into Responses tools and forwards the function name unchanged.
- Consequently, a 400 with `param=tools` is consistent with a real OpenAI-side validation collision after sub2api conversion. Whether it reproduces locally can still depend on which remote OpenAI account/type sub2api selects for the gateway key.
- With new-api compatibility explicitly disabled, five distinct Chat Completions shapes all returned HTTP 200 and a real `python` tool call through remote channel 85: modern automatic tools, modern forced strict tools, legacy `functions/function_call`, historical tool-call continuation, and a multi-tool request.
- Those results disprove a request-shape-only rule in the current remote route: neither modern versus legacy definition, tool choice, history, nor multiple tools was sufficient to reproduce the production 400. A single direct Responses request is the remaining bounded isolation check.
- The direct `/v1/responses` isolation probe also returned HTTP 200 with `status=completed` and a function call named `python`. It bypassed the new Chat Completions compatibility scope, so the current remote OpenAI account/model context itself accepted the name.
- Phase 6 Request IDs are `202607220715557642407975Peqpvqw`, `20260722071558727322923Fcs0H9s1`, `20260722071600708053716b6q1QuR0`, `202607220716033247284253YsAmGzw`, `202607220716048952756344wTgDeDQ`, and direct Responses `202607220718061811798023CIvh2Zl`; every record uses channel 85 and model `gpt-5.4`.
- The direct Responses account path injected substantial upstream context (4,430 input tokens), unlike the compact Chat Completions probes. No more paid repetitions are justified without the production client body or a way to pin remote sub2api to the screenshot's `nailong-PRO` account.
- Best-supported diagnosis: the production request did define a custom function named `python`, sub2api passed it unchanged into Responses, and the specific OpenAI account/model context selected at that time treated `python` as reserved. Current remote routing does not reproduce that validator behavior.
- Docker contrast-test preflight found no persisted rows for either new option, so the live starting state is the code default: compatibility enabled with reserved-name list `python`.
- The authorized token name resolves uniquely to token ID 141 in group `gpt-new`; its 48-character secret will remain in shell-local variables and will not be printed or written to artifacts.
- The earlier two successful requests both reached aggregate route `sub2api-gpt`, channel 85, and returned HTTP 200 for model `gpt-5.4`; these are the positive-control baseline, not yet proof of the switch or nonmatching-list behavior.
- The first disabled live control unexpectedly returned HTTP 200 and created consume log 34526 with Request ID `20260722063336944699594mb9wLWI5` on channel 85. This disproves the initial assumption that any forced Chat Completions function named `python` deterministically reproduces the upstream 400.
- The disabled-control result did not bypass sub2api: its log records aggregate route `sub2api-gpt`, channel 85, model `gpt-5.4`, and normal upstream usage. The next diagnostic must distinguish a hot-setting failure from payload-dependent or changed upstream reserved-name behavior.
- Screenshot inspection confirms the original failure used sub2api account `nailong-PRO`, inbound Chat Completions, outbound Responses, and model `gpt-5.4`, but it does not expose the original tools payload. Channel 85 currently points to a remote sub2api deployment with one configured gateway key, so its internal OpenAI-account selection is not locally observable.
- A deterministic black-box contrast can avoid depending on the upstream 400: force a tool call whose required `observed_name` argument must equal the function name shown to the model. Compatibility intentionally restores only structured function-name fields, so the argument reveals `python` versus `run_python` without leaking the alias accidentally.
- The four-request black-box matrix passed: disabled + `python` reported `python`; enabled + nonmatching `not_a_reserved_keyword` reported `python`; enabled + matching `python` reported `run_python` in both non-streaming and streaming tool arguments while the client-visible structured name remained `python`.
- Consume logs 34527 through 34530 provide the authoritative route evidence. Their Request IDs are `20260722063844971541042YsFcz03T`, `20260722063848780488585coz2N5ZT`, `20260722063850406377461JHAgdMx4`, and `20260722063852209810962TUfapqgC`; all used channel 85 and the last is streaming.
- The original upstream reserved-name 400 was not reproducible while compatibility was disabled in the current sub2api routing state. This does not weaken the alias proof, but it means the 400 is likely dependent on sub2api's internal OpenAI account/request context or has changed since the screenshot; claiming a deterministic global OpenAI rejection would be unsupported.
- Post-test restoration is exact: both newly introduced option rows are absent again, admin ID 1's access token is null, Docker is running/healthy, and `/api/status` returns 200. Startup therefore uses the intended default enabled + `python` state.
- Session recovery confirmed there are no existing functional changes for this compatibility feature; only its planning artifacts are modified.
- `GlobalSettings` is already hot-registered under the `global` configuration namespace, so the new switch and text setting can reuse `global.*` option persistence without a schema migration.
- The final outbound JSON is the correct normalization boundary: request DTOs intentionally preserve legacy and unknown fields through `json.RawMessage`/`any`, while `gjson`/`sjson` can update only known name paths without re-decoding numbers or touching arguments/content.
- Compatibility must be gated by the final outbound relay format being OpenAI Chat Completions. Applying it to Claude/Gemini native payloads would introduce aliases where the observed upstream restriction does not apply.
- The captured request entered sub2api through `/v1/chat/completions`, was converted there to `/v1/responses`, and received an OpenAI-style 400 `invalid_request_error` with `param=tools` because custom function name `python` was reserved by the selected model.
- Only the observed model behavior is confirmed; there is no verified complete cross-model reserved-name list in the currently available official documentation sources.
- sub2api is immutable in this deployment, so new-api must alias the Chat Completions request before forwarding and restore the alias after sub2api converts the Responses result back to Chat Completions.
- Model-name gating is brittle and unnecessary. A request-scoped bidirectional alias can activate only when the exact declared custom function name `python` is present, independent of model name.
- One-way channel parameter override is insufficient because downstream clients dispatch tool calls by the original function name.
- Arbitrary byte replacement is unsafe: aliases can appear in JSON arguments or normal text. Only known function-name fields should be changed.
- The user selected a configurable global reserved-name set rather than a hard-coded `python` rule. The default is enabled with `python`, while future names can be added without a deployment.
- Compatibility Management already has an OpenAI tab. The new switch and comma/newline text area fit that existing form hierarchy and avoid a new route or nested card.
- Alias generation uses `run_<original>` and must avoid collisions with all modern and legacy declared function names.
- Alias candidates must also avoid every configured reserved name, not only names declared in the current request; otherwise configuring both `python` and `run_python` could produce another upstream-rejected name.
- Response restoration must depend only on the request-scoped reverse map. Re-reading the hot enable switch during response handling would leak aliases if an administrator changed the setting while a request was in flight.
- Config normalization belongs below the controller as well as in it: direct single-option writes, batch writes, and startup database loads all reach the model option layer and must reject the same invalid boolean/name values.
- The UI/UX workflow confirmed that the existing data-dense admin form is the correct surface. The new switch and disabled-state textarea reuse Semi Design and the existing grid without adding nested cards or a separate page.

---

# Async Image Token Usage Log Backfill Findings (2026-07-22)

- Async image tasks persist the original consume log ID in `private_data.billing_context.consume_log_id`; terminal processing can find the row directly by `logs.id`.
- Image-parameter pricing currently writes real execution usage under `other.image_execution_audit`, but the top-level `prompt_tokens` and `completion_tokens` remain zero.
- `MergeConsumeLogOther` already validates `id + user_id + consume type`, verifies the stored public task ID, and uses the previous `other` value as a CAS fence.
- The lowest-overhead implementation extends that existing guarded update so audit JSON and token columns are written in one database update.
- Missing upstream usage must preserve zero token columns; token backfill is not a billing input for `image_parameter_per_call` tasks.
- A real `total_tokens` value without input/output components remains available in the execution audit but cannot be truthfully assigned to either top-level token column.
- Token-column mapping prefers `input_tokens/output_tokens` and falls back to the equivalent `prompt_tokens/completion_tokens` names; explicit zero values remain valid.
- Concurrent callback and submit-side compensation can target the same consume log; the existing one-shot CAS could discard the richer update, so bounded conflict retries are required for reliable backfill.
- No frontend change is needed because the usage-log table already renders the top-level prompt/completion token columns.

---

# Credential Separation Findings (2026-07-22)

- Final implementation review tightened stored-key normalization: a valid Webhook credential must now be exactly the canonical `wk-` prefix plus 48 generated characters, so a historical short value such as `wk-short` cannot bypass migration or regeneration handling.
- At exact 375 px, the Resource API Key tab remains document-width clean, while the documentation keeps each 560 px credential/flow table inside a 349 px horizontal scroller; all three pages preserve `document.scrollWidth === clientWidth === 375`.
- Exact CDP emulation confirms the Webhook view at `375x812`: the document and main content are both exactly 375 px wide, the revealed 51-character key wraps within a 342 px code line, and no visible button, input, or switch crosses the viewport boundary.
- Regeneration UI acceptance shows an explicit warning that the old key becomes invalid immediately; confirmation produced a new 51-character `wk-` value, changed the database ciphertext digest, preserved the `v1:` encrypted format, and left no plaintext `wk-` substring at rest.
- Live Webhook UI acceptance generated a real `wk-` value, showed it only after explicit visibility state, masked it to 16 bullets when hidden, exposed the expected show/copy/regenerate actions, and displayed a successful copy toast without requiring any Resource API Key.
- Restoring only the previously deleted local user row for the stale session is sufficient for authenticated Webhook UI acceptance; the live account has no pre-existing endpoint, so subsequent key-generation residue can be identified and removed exactly.
- The first Webhook UI mutation failed because the browser session referenced previously deleted user `994203`, not because key generation failed; backend logs showed `GET /api/webhook` returning an empty view and `PUT /api/webhook` correctly returning 404 when the owning user row was absent.
- Desktop browser QA confirms the API Key tab scopes `ak_` to task query, pre-upload, and resource access, while the documentation overview presents separate `sk-`, `ak_`, and `wk-` rows with their exact Bearer formats and explains that only the normal Token selects models, groups, and quota.
- The rebuilt Docker UI loads `/console/assets` under the existing ordinary-user browser session and exposes the expected Resource List, API Key, Webhook, and Documentation tabs; no additional fixture account is required for browser acceptance.
- Current `AssetKeyAuth` forces `ContextKeyUsingGroup` to the account group and disables token model/quota limits; it cannot preserve an `sk_` token's selected group.
- Current `TokenAuth` validates `token.Group`, applies aggregate/auto group semantics, installs model limits and token quota, and is therefore the correct create-path authentication.
- Current Webhook delivery loads the user's active Resource Center key and sends it as Bearer authentication, coupling resource-read privilege to an externally hosted receiver.
- `WebhookEndpoint.AuthKeyEncrypted` already exists and historical commits contain a generated/encrypted Webhook key UX, so the separation can reuse existing storage and prior local patterns without schema expansion.
- UI guidance favors the existing dense Resource Center form: visible field labels, explicit reveal/copy/regenerate actions, stable loading/disabled states, and mobile-safe controls; no new page or decorative treatment is needed.
- The approved breaking migration generates and activates `wk-` for existing configurations; existing receivers must update their expected Bearer value after deployment.
- The pre-unification Resource Center UI already implements password-style reveal, copy, confirmed regeneration, and responsive action wrapping. It can be restored narrowly while changing the misleading historical `sk-...` label to `wk-...` and retaining the current retry copy.
- The current UI blocks Webhook enablement on `resource_key_configured` and links users to the Resource Key tab; both coupling points must be removed when the dedicated key is restored.
- The historical backend already contains AES-GCM encryption, owner-only reveal, regeneration, and decrypt-failure handling. Its old normalization deliberately converted `wk-` into `sk-`; the restored implementation must instead make `wk-` canonical and migrate every legacy/empty value to a new `wk-`.
- Current Webhook tests intentionally assert Resource Key availability, rotation, and delivery headers. These assertions must become dedicated-key lifecycle/header tests while leaving retry, timeout, lease, fencing, SSRF, and endpoint-capacity coverage intact.
- The existing encryption derives an AES-GCM key from `common.CryptoSecret`; if stored ciphertext cannot be decrypted after a secret change, the safe behavior is to disable the Webhook and require owner regeneration rather than silently falling back to `ak_`.
- Resource Center examples currently use one `$API_KEY` for create, upload, query, and assets, and the generated OpenAPI exposes only `ResourceCenterAuth`. The corrected docs need distinct `$MODEL_API_KEY` (`sk_`, create only), `$RESOURCE_API_KEY` (`ak_`, reads/uploads), and outbound `WebhookAuth` (`wk-`).
- Existing router coverage asserts only path registration. Focused authentication tests must prove create requests carry token group/model/quota context and reject `ak_`, while query routes continue to reject `sk_`.
- The Assets page passes `onOpenApiKeys` only because the current Webhook tab links authentication to `ak_`; removing that prop is a narrow parent cleanup and does not affect the documentation tab's separate API-Key navigation.
- All seven locale files retain historical independent Webhook Key strings, so the restored UI can reuse most translations. Only canonical `wk-` labeling and the explicit no-resource-permission explanation need scoped additions.

---

# Async Worker Operations Findings (2026-07-21)

- Final source-scope review found no embedded production credentials. The only broad secret-pattern matches are existing Docker dev placeholders and the image-handle API-key form field; the opt-in async mock adds no authentication material.
- `TestImageTaskAndWebhookSchemaAcrossDatabases` passes with all three enabled targets (SQLite, MySQL, PostgreSQL), including migrations, uniqueness, Chinese payloads, queue stats, and admin delivery queries. The available live containers are newer than the documented minimum versions, so this is real engine coverage but not literal MySQL 5.7/PostgreSQL 9.6 execution.
- The final sequential frontend checks distinguish product scope from repository baselines: production build and i18n status pass, changed AsyncTask files are format/i18n clean, while full Prettier reports 113 files after build-generated `dist` assets and full i18n lint reports the unchanged 420 issues.
- Live disposable cross-database validation can use dedicated temporary databases on `postgres-db-test` and `business-ai-mysql`; their existing application databases must not be used because the schema test migrates tables and writes fixed fixture IDs.
- The final rebuilt Docker image containing the SIGTERM ordering, endpoint URL redaction, time filter, and mobile filter fixes is `sha256:975b7acfe7ca...`; both the application and opt-in mock report healthy.
- The existing in-app browser identity is a non-admin account and is correctly denied access to the admin operations page, so visual acceptance must use the disposable root account rather than treating `/forbidden` as a page regression.
- The rebuilt operations overview renders cleanly at `1139x1204` with `documentElement.scrollWidth === clientWidth`; worker capacity, queue state, and refresh controls remain readable in the dense admin shell.
- The available in-app browser backend does not open the application's hover-triggered Semi Dropdowns, including the account and theme menus; ordinary clicks only focus the trigger. Dark-theme QA therefore requires supported media emulation or must remain explicitly environment-limited rather than being claimed from a forced DOM mutation.
- The Webhook detail response is reflected safely in the UI: no authorization material is present, the endpoint contains only scheme/host/path, payload is bounded, and the attempt timeline retains prior failures.
- Browser retry acceptance exposed and fixed a live-consistency gap: after CAS retry returned `pending`, the worker immediately discarded the delivery because its endpoint was disabled; the list refreshed while the open SideSheet stayed stale. The Webhook tab now quietly reloads an open detail on each active-tab refresh token, and rebuilt Docker QA confirms row/detail convergence within five seconds.
- The repository-wide i18n lint currently has a large pre-existing hardcoded-string baseline; changed AsyncTask modules must be checked separately so this feature contributes no new findings.
- Admin endpoint URLs must omit query strings because user-supplied query parameters can contain credentials even though URL userinfo is already forbidden.
- Graceful shutdown must signal worker stop before waiting on HTTP shutdown; otherwise workers can continue claiming throughout the 30-second HTTP drain window.
- Both image dispatch and Webhook delivery currently claim 20 records and process them serially, so increasing only the claim limit would not improve throughput.
- Image dispatch already reclaims expired processing leases and fences completion with a lock token; Webhook delivery currently claims pending records only and can strand processing records after a crash.
- The shared image submission client can have no timeout when `RELAY_TIMEOUT=0`; dispatch requests need their own context deadline.
- Webhook delivery currently creates a new HTTP transport for every request. A shared transport can retain per-connection DNS/IP validation and redirect blocking while reusing safe keep-alive connections.
- `async_task_setting` is registered in the live option system and applies normalization after updates, so new worker settings can take effect without restart.
- Existing tables contain the required queue, event, payload, endpoint, and attempt state; no new business table is needed.
- The existing page loads all settings and stats once. The approved replacement uses active-tab polling and leaves unsaved settings untouched.
- UI guidance confirms a data-dense operational layout, semantic theme tokens, accessible icon labels, mobile table containment, and stable loading dimensions are the correct fit.
- `docker-compose-dev.yml` has PostgreSQL, Redis, and new-api but no deterministic Webhook/image submission receiver; the test service will be opt-in through a profile.
- The new standard-library mock can vary image defaults through `/control`, vary individual image jobs through metadata, expose distinct Webhook paths for endpoint-cap tests, and reports only whether authorization was present rather than credential values.

---

# Multipart Async Image Editing Findings (2026-07-18)

- Docker E2E created `task_fzZfMwkWiFiGVzRKJyxyByw3E9ZOdR8h` from a local PNG multipart request, returned HTTP 202, and reached `succeeded` through image-handle and the local mock provider.
- The retry E2E forced HTTP 500 twice and HTTP 204 on the third attempt. All three requests used valid Bearer authentication and one stable `evt_hViePhxooTbn19AF094BC9XdvuYlAikb`; the delivery finished as `delivered` with three persisted attempt rows.
- A same-file multipart replay returned HTTP 202 with `Idempotent-Replayed: true`; changing the file contents under the same key returned HTTP 409.
- Browser QA found and corrected one stale overview sentence that still required pre-upload. The final page describes both direct multipart upload and the URL/pre-upload flow.
- `PrepareImageTaskRequest` currently strictly decodes JSON, validates it, computes a canonical request fingerprint, resolves sequential idempotent replay, and rewrites the body for the existing relay task handler.
- `ProxyImageTaskUpload` already validates multipart/base64 upload limits and proxies bytes to image-handle, but response parsing is embedded in the HTTP handler.
- Multipart task creation must preflight idempotency before image-handle upload because temporary object URLs change on every upload.
- The durable relay path consumes the public request JSON and fingerprint from Gin context, so multipart can join it after URL materialization without changing task persistence or dispatch models.
- Synchronous image edit uses `model`, `prompt`, repeated `image`, optional `mask`, `n`, `size`, `quality`, `output_format`, `output_compression`, and `background`; the async multipart surface will match those names.
- The distributor's generic branch intentionally skipped multipart requests; `/v1/image/tasks` now has a narrowly scoped multipart model extraction branch using the existing reusable form parser.
- The internal upload body is rebuilt with only `image` and `mask` file parts. Task fields are neither sent to image-handle nor included in generated temporary object names.
- Focused controller and middleware tests pass with multipart mapping, upload failure propagation, and pre-upload idempotency replay/conflict coverage.
- Current Webhook delivery marks every HTTP response, including 500, as delivered; only transport errors become final failures, and no retries are scheduled.
- `WebhookDelivery` already has attempts, `next_attempt_at`, leases, and pending status. `CompleteWebhookDeliveryAttempt` already supports returning a failed attempt to pending, so configurable retries require no schema migration.
- The existing Async Task Management page and `async_task_setting` option group are the correct narrow configuration surface for attempt count and a fixed retry interval.
- Webhook failures now return the delivery to `pending` with `next_attempt_at`; the third failed attempt becomes terminal under defaults, while any 2xx response becomes delivered regardless of body.
- OpenAPI now exposes both JSON and multipart request bodies for task creation and documents the configurable Webhook retry contract.

---

# Image-handle Channel Override and Signed URL Findings (2026-07-15)

- Channel-level `response_format=url` must reach Adobe when execution is delegated to image-handle.
- A signed URL returned by image-handle must be emitted with literal `&` separators in the raw client JSON response.
- `relay.ImageHelper` maps the model and takes the image-handle sync branch before adapter conversion and the normal channel parameter-override block.
- The distributor has already selected the concrete channel and populated `RelayInfo.ParamOverride`; image-handle does not need to identify Adobe.
- `imageHandleSyncParameters` only includes `response_format` when it exists on the normalized request, so the final `result_data_format=url` policy does not force the provider request format.
- image-handle already normalizes residual literal `\\u0026`, validates HTTP(S), and directly passes URL sources without downloading or uploading them.
- new-api unmarshals the image-handle response correctly, but `common.Marshal` HTML-escapes `&` when rebuilding the OpenAI-compatible client response.
- Reuse the established channel override engine before image-handle payload construction; do not add Adobe names, domains, or model heuristics to image-handle.
- Add no-HTML-escape encoding to `common/json.go` and limit its use to the image-handle sync client response.
- Preserve Base64-to-R2 fallback for providers that do not return URL data.
- The admin screenshot shows `跟随请求参数` with default `URL`; this currently governs the final image-handle result, not the upstream provider's `response_format`.
- A second screenshot placed `response_format` under request-header override, which is the wrong protocol location; Adobe expects it in the JSON body.
- Payload-level tests prove the same selected-channel override reaches both generation and edit requests after a public alias in the `aggregate` group maps to upstream `gpt-image-2`.
- image-handle receives only the upstream model, normalized parameters, and a credential lease; it has no need to recognize Adobe by provider name, URL, or model heuristic.
- The full Go suite passes with the selected-channel override and signed-URL serialization changes.
- Local channels 89 and 90 now both persist `{"response_format":"url"}` as request parameter overrides; the temporary upstream debug option was restored to `false`.
- A count-mode request that omitted `response_format` returned HTTP 200 from `pre-signed-firefly-prod.s3-accelerate.amazonaws.com`, with no Base64 data and no `img.supertoken.cc` reference.
- The count-mode raw client JSON contains six literal ampersands and zero `\\u0026` sequences; a one-byte range request to the signed URL returned HTTP 206 with `image/png`.
- The successful count log records channel 90, upstream model `gpt-image-2`, `image_handle_sync`, and the expected low-tier per-image charge of 20,000 quota.
- Token-mode image-handle tasks also persisted `response_format=url`, the mapped `gpt-image-2`, and the correct channel-89 lease, proving the new contract is applied there too.
- The token upstream disconnected before any HTTP response on two image-handle attempts (`fetch failed` after roughly 97 and 113 seconds). A separate host-direct request failed with curl HTTP status 000 and an HTTP/2 framing-layer error, isolating the remaining failure outside the new-api override/serialization changes.
- Integration artifacts and container logs are retained under the four new `tmp/image-handle-channel-override-*` and `tmp/adobe-token-direct-upstream-*` directories.

---

# Aggregate Group Categories Findings (2026-07-17)

- UI review confirms the existing Semi Design data-dense admin language should be preserved; category management needs keyboard-labelled icon actions, touch-sized mobile selection, explicit loading states, and no new decorative styling.
- The aggregate-group page already owns client-side search and renders all filtered rows through `CardTable`, so category filtering and selection can be added without pagination or API contract changes.
- The token modal's existing custom group renderer can be reused inside explicit `Select.OptGroup` children; the flat `optionList` is the only piece that needs replacing there.
- Category API rows already include custom-category usage counts; the virtual Other count must be derived from aggregate groups whose `category_id` is 0.
- Historical token groups should be stored separately from the current selectable options so group/token request ordering cannot incorrectly expose or hide an unavailable value.
- Aggregate groups currently have no category field; full and fast migrations both explicitly register aggregate-group models.
- The aggregate-group admin list loads all groups and filters client-side, so category filtering and select-all-filtered can stay local.
- `CardTable` forwards desktop `rowSelection` but currently ignores it in mobile card mode; mobile bulk assignment needs shared selection rendering.
- `/api/user/self/groups` already distinguishes `auto`, `aggregate`, and `real`, making category metadata additive without changing token persistence.
- The existing token selector uses one searchable Semi Select and a custom option renderer; Semi supports `Select.OptGroup` for category sections.
- Configurable categories require stable IDs and ordering; virtual category ID 0 gives old groups and deleted-category groups a cross-database-safe fallback.
- The approved UI remains a data-dense operational surface with explicit filters, loading states, confirmations, accessible labels, and responsive controls.
- User administration also reuses the aggregate-group response builder for per-user ratio overrides, so category metadata must be loaded there as well as on the main aggregate-group endpoints.
- Existing controller tests use an isolated AutoMigrate list; new persistence models must be registered in both production and test migrations.
- Docker browser QA passed at effective 1440px, 768px, and 375px widths with no document-level horizontal overflow; the mobile card checkbox stays above the card title and the batch bar stacks vertically at 375px.
- Light and dark theme checks passed for the category drawer and token editor. The 375px drawer fills the viewport without clipping, and the dark token dropdown keeps category headers, HA markers, names, and ratios readable.
- The token selector shows ordered custom categories before Other, keeps real and uncategorized aggregate groups in Other, applies HA only to aggregate groups, excludes `auto` for creation, and searches by both stored and display names.
- A disposable local `auto` token verified the historical section and one-way behavior: the existing value can be retained, but after choosing a current group the historical option disappears.
- Browser QA exposed a real interaction defect: a `Tooltip` wrapped inside Semi `Popconfirm` prevented the category delete trigger from opening. Making the button the direct trigger fixed the confirmation; deleting a category with two assignments restored both groups to Other.
- All temporary categories, assignments, token, administrator, and build artifacts were removed after QA.
- Follow-up visual feedback showed the stock Semi OptGroup label was too low-contrast and depended mostly on whitespace. The token selector now uses a semantic fill header band, stronger text, a primary-color left rule, top/bottom boundaries, and subtle option dividers.
- Docker browser QA confirmed the stronger grouping at 1440px and 375px in light/dark themes. A freshly opened mobile popup matched the 375px viewport, kept ratios visible, and produced no document-level horizontal overflow.


# Image Parameter Pricing Findings (2026-07-14)

- Current implementation spans direct image relay, synchronous image-handle execution, and asynchronous `/v1/image/tasks` with one shared single-dimension pricing resolver.
- Public-model pricing is resolved before model mapping; this allows `adobe-gpt-image-2-count` and `adobe-gpt-image-2-token` to both map to upstream `gpt-image-2` while retaining different billing modes.
- Count snapshots use `image_parameter_per_call`; token aliases continue `async_image_usage_billing` and must not contain an image-pricing snapshot.
- The local tokens named/groups `adobe-image-2-count` and `adobe-image-2-token` exist, but the current channels still expose only `gpt-image-2` in unrelated groups and have no alias mapping or abilities for those token groups.
- `ImagePricing` is absent locally. Count acceptance therefore requires a configured Adobe quality profile (`low=0.04`, `medium=0.07`, `high=0.15`, default `low`) bound to the public count alias.
- Local integration must never print token values or upstream keys. Resolve secrets inside the request command/container and report only HTTP/task/billing evidence.
- Async submit returns the public/client task ID. Polling terminal state is `data.status == SUCCESS|FAILURE`; the image-handle provider task ID stays in task private data.
- For a count request with `quality=high`, `n=2`, group ratio `1`, and default `QuotaPerUnit=500000`, expected snapshot subtotal is `0.30` and expected final quota is `150000`.
- The resolver writes a missing profile parameter's default `upstream_value` back into the request, normalizes `n`, and calculates final quota with `shopspring/decimal` plus the repository quota-rounding helper.
- Local Compose currently has healthy `new-api-dev`, PostgreSQL, and Redis services; the app container predates the final review and must be rebuilt before acceptance.
- The adjacent image-handle repository contains in-progress parameter-forwarding and audit changes in its worker, runner, server, and contract tests; those changes must be preserved and verified rather than recreated.
- Local image-handle API, worker, notifier, PostgreSQL, and Redis containers are already running; API is exposed on port `8787`. New-api remains exposed on port `3001`.
- Both repository diffs currently pass `git diff --check`; image-handle's product diff is limited to three source files and three contract-test files.
- Runtime configuration is now present: both aliases map to upstream `gpt-image-2`; `adobe-quality-v1` binds only the count alias with low/medium/high prices, default low, and `max_n=10`.
- Live synchronous image-handle evidence exists for both billing modes on the current container: count defaulted to `low` and charged `20000` quota for one image; token mode used returned usage and charged `2958` quota. Both logs retain the public alias and upstream mapped model.
- Async count task `task_codex_count_async_1783981346` froze `quality=high`, `n=2`, unit price `0.15`, subtotal `0.30`, group ratio `1`, and final quota `150000`; its lease resolved model `gpt-image-2`.
- That async count execution ended `FAILURE` with image-handle `fetch failed`; callback delivery succeeded and new-api ran the failure refund path. The evidence does not identify whether the failed fetch was the upstream POST or a returned image URL download.
- Async token task `task_codex_token_async_1783981671` was still `IN_PROGRESS` at audit time with legacy usage billing, `50000` precharge, no image-pricing snapshot, and a resolved `gpt-image-2` lease.
- The token async task later reached `SUCCESS` with one asset and usage `8 input + 196 output`; `(8 + 196 * 6) * 2.5 = 2960` exactly matches task quota. Its `50000` precharge produced a `47040` refund log.
- `/api/pricing` exposes the count alias as `quota_type=1`, `model_price=0.04`, `billing_type=per_image_parameter`, and redacted tier data; the token alias remains `quota_type=0`, ratio `2.5`, and has no image-pricing field.
- Authenticated polling returns uppercase terminal states and a result URL for the successful token task; the count failure returns `FAILURE`, `100%`, and `fetch failed` without a result URL.
- Runtime accounting audit found refund deltas restore spendable wallet/subscription/token quota but do not decrement cumulative `users.used_quota` or `channels.used_quota`. This needs an explicit compatibility decision against the plan's full-refund/net-settlement language.
- Standard synchronous `BillingSession` accounting records used quota only after final settlement. Async task accounting records the precharge at submission, then updates used counters only for positive deltas; therefore a failed/over-precharged image-handle task leaves gross precharge in cumulative counters even though logs and spendable balances are net-correct.
- A safe correction, if adopted, should be image-handle billing-mode scoped, decrement only used-quota counters (not request count), and remain behind the existing CAS-owned terminal settlement so duplicate callbacks cannot double-adjust.
- Frontend focused verification passed: image-pricing helper tests `10/10` (21 assertions), targeted ESLint/Prettier, production build, and frontend whitespace checks. Full i18n lint still reports the repository's existing 421 hardcoded-string findings; none point at the new image-pricing files.
- Browser interaction coverage remains outstanding. Static review covers profile CRUD/copy, bulk binding, `max_n`, preview, marketplace rendering, and log snapshot helpers, but there are no React component tests for the settings editor.
- image-handle product code only extends the existing parameter/audit allowlists with `resolution`; `quality`, `size`, and `n` remain existing passthrough fields. No pricing or model-mapping logic was added there.
- image-handle tests explicitly prove public alias task input is replaced by lease model `gpt-image-2` before upstream generation/edit calls, with all four normalized parameters present in JSON and multipart contracts.
- `response_format` had two independent contract gaps: sync image-handle dropped it for `gpt-image-*` and derived it from result policy for other models, while async image-handle lacked a top-level DTO field and parameter allowlist entry.
- Async top-level image fields also exposed a context-copy bug: when metadata was initially absent, the adaptor created a new map without storing the updated request back into Gin context. Persisting the normalized request fixes top-level `quality`, `resolution`, `n`, and `response_format` together.
- The corrected contract forwards only an explicit client `response_format`; `result_data_format` force/default policy remains independent and omission never synthesizes an upstream parameter.

---

# Multi-level Token Tier Pricing Findings (2026-07-13)

- Official GPT-5.6 Standard pricing uses a whole-request threshold: requests above 272,000 total input tokens charge all input at 2x and all output at 1.5x; this is not marginal pricing on only the excess tokens.
- Threshold selection uses total input usage only. Cached-read and cache-write usage are component details within total input and must not be added again.
- Existing billing snapshots live in `types/price_data.go`; final token settlement and log text live in `service/text_quota.go`.
- The feature must preserve the legacy formula byte-for-byte for models without an effective enabled rule and must exclude per-call fixed pricing.
- Docker dev is currently healthy on port 3001. `test-gpt` is a database token name, not a literal key; validation must retrieve the secret without printing it.
- UI guidance favors inline row errors, stable column widths, and stacked mobile tier rows rather than requiring horizontal scrolling.
- Existing quota units convert to absolute prices as `price_per_million = ratio * 1_000_000 / QuotaPerUnit`; resolving this at request start preserves custom `QuotaPerUnit` behavior.
- The final text quota function already separates Token quota from fixed tool quotas before summing. The tier branch replaces only the Token subtotal, so Web Search, File Search, image-generation calls, and audio fixed charges are not multiplied by the context tier.
- Marketplace group/currency conversion can reuse `calculateModelPrice` with a synthetic per-tier ratio record, avoiding a second frontend currency implementation.
- The final live report contains seven passed scenarios. The official long-context case used `gpt-5.6-luna` with 285,016 input and 18 output tokens, selected tier 2, and matched 285,097 expected/actual quota plus identical user, token, and channel deltas.
- The repeatable validator stores its evidence in `tmp/token-tier-pricing-report-1783875609.json`, guards the costly long-context case behind `--allow-real-long-context`, and never prints the `test-gpt` secret.
- Final Docker image `e1c0d1bdf24c...` is running at port 3001. `new-api-dev` and `sub2api-dev` are healthy; PostgreSQL and Redis are running and pass the validator's readiness probes despite having no Compose healthcheck metadata.
- Final database audit confirms the temporary override and visual user were deleted, the original two usable groups remain, and the temporary root access token was restored to null.
- The disabled marketplace badge was a display-cache issue, not a billing-state issue: settlement used the current disabled rule immediately, while `/api/pricing` copied tier metadata from its one-minute cached model row.
- Recomputing only `token_tier_pricing` while cloning the pricing response gives immediate enable/disable behavior without forcing the expensive abilities, model metadata, vendor, and endpoint pricing cache to rebuild.
- Docker browser verification confirms the disabled Luna card contains only its normal prices and usage-billing tag; restoring the system default immediately restores the localized base-price suffix and two-tier badge.

---

# Model marketplace dynamic-route label findings (2026-07-12)

- The screenshot's orange label is emitted by `formatPriceInfo`; the same wording also appears in table and pricing-detail views.
- `/api/pricing` returns per-model aggregate details with `ratio`, `max_ratio`, and `dynamic_route`.
- `max_ratio` covers configured ratios across reachable child routes. Removing that calculation would risk showing a lower price than a route can actually charge.
- The scoped fix keeps price and ratio values unchanged and removes only the "动态路由最高价/最高倍率" labels from all model-marketplace views.

---

# Usage Statistics Split Findings (2026-07-12)

## Follow-up table audit
- The reported issue is visual table fill, not API completeness. Primary suspects are unconstrained Semi Table column allocation, hidden mobile columns, and pagination/container width behavior.
- The preferred correction is one elastic identity column plus explicit widths for numeric/time columns, avoiding fixed total table widths and whole-page horizontal scrolling.
- Docker dev is currently healthy but predates commit `424d5e02`; it must be rebuilt before visual conclusions are valid.
- Docker image `1aa4938c...` was rebuilt from current `main` and the app was recreated successfully on port 3001.
- Both desktop ranking tables currently use `scroll={{ x: 'max-content' }}` without explicit column widths; this is a strong source-level explanation for unfilled right-side space when cell content is short.
- The first automated in-app browser session redirects the rebuilt page to `/login`, so it cannot yet provide authenticated table measurements.

## Requirements and current state
- Current `GetUsageStats` loads filtered consume logs and builds one total summary, trend, model ranking, user ranking, and user-model details in memory.
- Consumption logs can be classified from `other.billing_source`; exact `wallet` and `subscription` are known, while missing/invalid values must remain unknown.
- Text/audio/realtime log helpers already append billing metadata. `LogTaskConsumption`, `GenerateMjOtherInfo`, and violation-fee log construction omit it.
- Subscription billing launched with the metadata field on 2026-02-03; historical gaps are not safely backfillable for all special paths.
- The current page is a single large component with a filter card, eight equal KPI cards, two charts, three vertically stacked ranking tables, and three side sheets.
- The redesign will use overview/ranking/funding tabs, lazy section loading, a sticky compact filter surface, four primary KPIs, and responsive reduced-column tables.

## Technical decisions
| Decision | Rationale |
| --- | --- |
| Add flat additive fields to existing response structs | Keeps current clients compatible and minimizes frontend normalization. |
| Add `subscription_ranking` beside `ranking` | Satisfies the requested separate leaderboard without changing the existing total ranking. |
| Add `section=usage|recharge|subscription_purchase|all` | Hidden tabs should not trigger irrelevant log/order scans. |
| Add `billing_source=all|wallet|subscription|unknown` for usage | Reuses the existing detail response shape for source-specific drill-down. |
| Use Semi theme tokens and Lucide icons | Matches the established frontend and supports dark mode. |

## Implementation findings
- `section=usage` can bypass both recharge queries and subscription-order queries without changing the default response.
- Source filtering must occur after parsing `other`, so it remains database-neutral.
- Subscription zero-quota requests are retained in source request counts but excluded from active-user counting and the subscription ranking.
- Existing model and controller tests pass after the additive response changes.
- Frontend already provides `useIsMobile`; responsive columns can use the established breakpoint rather than adding a new media-query hook.
- `useIsMobile` is a named export; new page modules must follow the existing import convention.
- The locale extractor is not scoped to changed files and currently rewrites hundreds of pre-existing missing keys; it is unsuitable for a narrow feature diff without cleanup.
- Current `UsageStats` mixes request orchestration, chart specs, table definitions, and three detail sheets in one file; component extraction removes real complexity rather than adding a cosmetic abstraction.

## Visual/browser findings
- Local Docker UI is available on port 3001, but the current in-app browser session redirects the protected usage page to login.
- Source inspection confirms the long-scroll problem is structural, not caused only by row count.
- Final implementation keeps usage, recharge, and subscription-purchase queries independently addressable through `section`; omitted `section` remains backward-compatible `all`.
- The frontend cache key includes applied filters, section, and funding pagination. Query/reset clears the cache, while refresh reloads only the visible section.
- All seven locale files contain the 29 new dashboard strings. The broad repository i18n lint retains unrelated baseline warnings, but none remain under `src/pages/UsageStats`.
- Full Go tests and the frontend production build pass. Authenticated multi-viewport screenshots remain blocked by the login redirect.

---

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
- The running new-api and image-handle containers predate the shared-network compose changes; `ai-gateway` currently exists with no attached containers, so both stacks must be recreated before E2E DNS checks are meaningful.
- After rebuilding/recreating both stacks, `new-api-dev`, image API/worker/notifier, `mock-provider`, and `webhook-receiver` all attach to `ai-gateway`. Bidirectional probes new-api -> image-handle/mock/receiver and image-handle -> new-api/mock return HTTP 200.
- Local third-party Webhook E2E requires the explicit dev-only `WEBHOOK_ALLOW_INSECURE_LOCAL=true`; production keeps HTTPS/public-IP enforcement by default.
- Persisted image-handle options still point to `host.docker.internal`, so they override the new shared-network Compose defaults until updated/restarted.
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
# UsageStats Table Layout Findings (2026-07-12)
- Semi Table writes `scroll.x` directly to the internal `<table>` width. `max-content` therefore shrank short-content tables and left unused space on the right.
- `scroll.x='100%'` fixes desktop fill but auto table layout still lets long usernames expand the first column and compress other headers.
- In the installed Semi version, the public `tableLayout` prop is not consulted by `getTableLayout`; fixed layout is selected when a column has `ellipsis` or `fixed`.
- The stable combination is explicit column widths, `ellipsis` on the first text-heavy column, `100%`/`max(100%, min-width)` desktop width, and a bounded 580px mobile width for the four visible columns.
- Internal horizontal scrolling at narrow widths is intentional; the document itself has no horizontal overflow.
# Wallet Usage Ranking Direction (2026-07-12)
- The new tab must use an independently aggregated and sorted wallet ranking; client-side sorting of the total ranking would use the wrong ordering and may omit users.
- Display order is `总消耗 / 按量消耗 / 订阅包消耗`.
- Wallet ranking excludes subscription and unknown-source quota, and its user detail drill-down uses the wallet billing source.
- `populateUsageStatsUsage` already maintains a dedicated subscription accumulator during the single log scan; a wallet accumulator can follow the same path without another database query.
- The response contract currently exposes `ranking` and `subscription_ranking`; `wallet_ranking` belongs beside them in `UsageStatsData`.
- Both source-specific rankings should include only positive-quota consumption, matching the current subscription ranking semantics.
- The existing mixed-source model test is the right regression point; wallet fixtures should make wallet ordering differ from total ordering so independent sorting is proven.
- `UsageStatsPage.loadUserDetail` already accepts an arbitrary billing source and sends it as `billing_source`; wallet mode only needs to pass `wallet` from the ranking row click.
- Frontend mode-specific copy must cover the panel title, quota column, empty state, tag color, and detail sheet title, not only the new tab label.
- No component-level UsageRanking test exists; backend aggregation tests plus frontend static/build/i18n checks and Docker browser verification provide proportional coverage.
- Locale coverage is seven files: en, fr, ja, ru, vi, zh-CN, and zh-TW.
- Wallet detail trend copy needs its own `仅统计按量计费额度` key so the displayed scope matches the filtered API response.
- Final static trace confirms `wallet_ranking` is initialized as an empty array, populated only from positive wallet consumption, sorted independently, and consumed by wallet mode.
- The tab order in code is total, wallet, subscription, matching the requested placement.
- Docker data for 2026-04-27 contains wallet quota `$0.83` across 22 requests, sufficient for an authenticated wallet-ranking and drill-down audit.
- Authenticated UI shows the requested order `总消耗 / 按量消耗 / 订阅包消耗`.
- Wallet ranking is demonstrably independent: its first user has `$0.26` wallet usage, while the total ranking first user has `$0.53` combined usage.
- Wallet drill-down title is `按量消耗明细`; the selected user shows `$0.26` wallet, `$0.00` subscription, and one wallet-only model row.
- At 375px all three secondary tabs remain visible, document width stays 375px, and the table uses its existing bounded 580px internal scroll width.
# Claude `Content block not found` Analysis Findings (2026-07-14)

## Requirements
- Analyze the frequent `API Error: Content block not found` failure observed with `claude-fable-5`.
- Ground the answer in this repository's code, public web material, and official documentation.
- Diagnose only; do not change product code unless separately requested.

## Initial Context
- The worktree contains unrelated user changes; this investigation is read-only except for these planning records.
- Primary questions are the error's emitter, protocol invariant being violated, alias/provider routing, and practical confirmation steps.

## Repository Discovery
- A repository-wide case-insensitive search found no product-code emitter for the literal `Content block not found`; matches were only this investigation's planning text. The visible error therefore enters from an upstream body/SSE error event or is produced by a downstream client consuming the relayed stream.
- Claude SSE handling and format conversion are concentrated in `relay/channel/claude/relay-claude.go` and `service/convert.go`. The latter explicitly tracks open block indices and documents the required `content_block_start -> content_block_delta* -> content_block_stop` ordering.
- `claude-fable-5` occurs in compatibility tests and model-family predicates in `relay/common/claude_compat.go`; no local provider implementation or official model declaration was found in the first search. Its repository presence alone indicates an accepted alias/family string, not an official Anthropic model ID.
- Existing conversion tests focus on emitting exactly one block start/stop and keeping deltas aligned to a block index, which confirms that content-block lifecycle mismatches are a known interoperability risk in this codebase.
- The native Anthropic adaptor sends Claude-format input to `{channel_base_url}/v1/messages`, defaults `anthropic-version` to `2023-06-01`, and returns native streams through `ClaudeStreamHandler`. OpenAI-format requests routed to a Claude channel are converted by `RequestOpenAI2ClaudeMessage` first.
- `relay/channel/claude/constants.go` does not list `claude-fable-5`; its newest listed public Claude model is `claude-sonnet-4-6`. A configured channel can still expose arbitrary models, but `fable` is not built into this adaptor's official/default list.
- The OpenAI-to-Claude stream converter maintains per-index start/stop maps. It starts tool blocks only when a tool name is present, then emits argument deltas by index. A nonstandard upstream chunk that sends tool arguments before the name can therefore create a delta without a prior start; this is a concrete local path capable of causing a consumer-side “block not found” state error.
- The same converter explicitly contains prior hardening for “Mismatched content block type,” showing that nonstandard interleaving of thinking, text, and tool calls has previously required lifecycle repairs.
- Native Claude SSE data is unmarshaled and checked with `GetClaudeError`; an upstream `type:error` event is turned into a new-api error. For native Claude-format clients, ordinary non-error events are otherwise relayed after usage-field patching, so new-api does not itself validate that every delta references an open block.
- Git history for `service/convert.go` contains repeated protocol repairs, including `fix: Claude stream block index/type transitions` and the 2026-05-19 commit `d9c1dfcaf` (`fix: 修复 Claude 流式工具调用转换状态异常`). This supports treating stream-shape interoperability as the leading code-level area, not a generic HTTP/network failure.
- The native Claude-to-OpenAI converter maps block indices into OpenAI tool-call indices but does not maintain a downstream block table; therefore “block not found” is more naturally emitted by an Anthropic-style stream consumer or upstream proxy state machine than by this conversion function.

## External Research
- Correction to the initial alias hypothesis: Anthropic's current official model overview documents `claude-fable-5` as an official Claude API ID/alias, generally available beginning 2026-06-09. The local adaptor's static model list is stale/incomplete, while the compatibility predicate added Fable support on 2026-06-11.
- Anthropic documents Fable 5 as always using adaptive thinking, with up to 128k output. Manual extended-thinking budgets, `thinking: {type:"disabled"}`, and assistant prefill are unsupported. It may therefore exercise thinking-to-tool stream transitions much more often than older/non-thinking models.
- The exact string is documented in the official Claude Code changelog: a client bug caused streaming requests to fail with `Content block not found` (or JSON parse errors) after a machine woke from sleep. This establishes at least one independent client-side cause unrelated to model validity or new-api request conversion.
- Ollama issue `ollama/ollama#14816` provides a concrete reproduction and SSE trace: a converter reused block index 0 when transitioning directly from thinking to tool use, then emitted a stop for index 1 that had never started. Claude Code logged `Error streaming, falling back to non-streaming mode: Content block not found`. The fix was to close thinking and advance the index before starting `tool_use`.
- new-api issue `QuantumNous/new-api#4389` reports the exact symptom with Claude Code v2.1.117 and OpenAI-compatible non-Anthropic models. Reports say older Claude Code worked and changing the channel from OpenAI type to Anthropic type avoided the failure, directly implicating the OpenAI-to-Anthropic stream conversion path rather than the official Anthropic model itself.
- Public reports also show the same symptom with local/open models (Kimi, llama.cpp), reinforcing that the phrase is a Claude Code stream-assembly error surfaced when a provider emits an invalid or unexpected content-block lifecycle.
- Anthropic's official streaming spec is explicit: after `message_start`, each content block has `content_block_start`, zero or more deltas, and `content_block_stop`; its `index` is the position in the final message `content` array. A delta or stop for an index with no live start, an index reuse, or a block-type change without close/advance violates this contract.
- Official Fable 5 migration guidance says adaptive thinking is always on and, with default `thinking.display:"omitted"`, a thinking block can contain only a `signature_delta` before closing. Proxies must preserve even these visually empty thinking blocks and their lifecycle; dropping them or treating “no text” as “no block” can desynchronize subsequent tool indices.
- The official spec permits `ping`, mid-stream `error`, and future unknown event types. Consumers should tolerate the former/unknown events, but this does not relax content-block index ordering.
- The public new-api search did not yet identify a single upstream PR tied directly to issue #4389; local history is more useful because the checkout already includes later block-state fixes through May 2026.
- Official Claude Code `CHANGELOG.md` identifies the sleep/wake fix specifically in version `2.1.186`. Any affected client older than 2.1.186 should be upgraded before attributing every occurrence to the relay.
- new-api issue #4389 was closed for insufficient reproduction details, not as definitively fixed. The only configuration-specific report says `/v1/messages` through an OpenAI-type channel failed while an Anthropic-type channel worked. This is useful corroboration but not maintainer-confirmed root-cause proof.
- Issue #5126 is adjacent but distinct: newer Claude Code may send `document` request blocks that non-Anthropic upstreams reject or lose during conversion. That produces request-side 400/invalid request behavior, not the stream accumulator's `Content block not found`; it should not be presented as the primary cause of this exact error.
- The OpenAI channel response path calls `StreamResponseOpenAI2Claude` for every upstream chat-completion chunk and emits every synthesized Claude event immediately. There is no later validator/repair layer, so any missing start, invalid stop, or reused index produced there reaches Claude Code unchanged.
- Current stream tests cover repeated tool names, final arguments before stop, dense parallel indices 0/1, and text/thinking transitions. They do not cover sparse tool indices, a tool fragment whose arguments arrive before its name/start, or asserting that a stop is emitted only for indices that actually started.
- `helper.ClaudeData` serializes each synthesized event with `event: <resp.Type>` and its JSON data immediately. The native path similarly rebuilds the event name from the JSON `type`; it does not preserve an independent upstream SSE event name, but official events require those values to match anyway.
- `StreamScannerHandler` adds SSE comment pings (`: PING`) during idle periods. These are valid keepalive comments and are not content blocks, so they are not a primary explanation for a block lookup failure.
- The February block-transition fix introduced `stopOpenBlocks`, but its tools branch closes every offset from zero through the maximum seen offset. The May deduplication fix prevents duplicate stops but still does not require `ContentBlockStartSent[idx]` before stopping. Sparse/missing-start tool indices therefore remain a plausible current defect.
- A temporary diagnostic program invoked the current `StreamResponseOpenAI2Claude` with this legal-to-parse but nonstandard OpenAI chunk order: empty initial chunk -> thinking -> tool arguments without name -> tool name/final arguments. The emitted Claude sequence was `start(0 thinking), delta(0), stop(0), delta(1), start(1 tool), delta(1), stop(1)`. The first delta for index 1 has no preceding start and deterministically violates Anthropic's official state machine. This is a confirmed current code defect, not just a hypothesis.
- The temporary diagnostic file was deleted after execution; no product code was changed.
- The local Docker app is running and healthy. The OpenAI adaptor's `ConvertClaudeRequest` explicitly performs Claude -> OpenAI request conversion, and its response helper then performs OpenAI -> Claude SSE synthesis. This is the exact risky round trip when an OpenAI-type channel serves `/v1/messages`.
- `claude-fable-5` remains absent from product model lists/default configuration in this checkout outside the compatibility predicates/tests; actual availability is therefore supplied by runtime channel/model configuration or sync data.

## Issues Encountered
- A scoped `git status` command included the already-deleted `/tmp` diagnostic path, which Git rejected because it is outside the repository. This had no filesystem effect; repository paths were inspected separately.
- The first PostgreSQL metadata commands failed because nested shell quoting stripped SQL string literals. No query executed and no data changed; the next attempt uses quote-free metadata output plus external filtering.

## Local Runtime Evidence
- The last 48 hours of `new-api-dev` container stdout contain no match for `Content block not found`, `claude-fable-5`, or related block-mismatch phrases. Runtime attribution therefore needs database logs/channel configuration or a fresh captured reproduction.
- Runtime configuration exposes Fable only on channel 63, type 14 (`Anthropic`), whose base URL is the third-party endpoint `https://www.doubingo.com`; it is not a direct `api.anthropic.com` connection. Native `/v1/messages` traffic therefore trusts that provider's Anthropic-compatible SSE implementation.
- Six stored Fable calls on 2026-06-11 were successful consume logs, not errors. They were non-streaming test calls over both `/v1/messages` and `/v1/chat/completions`, so they do not validate Claude Code's streaming/tool-use path.
- No error logs were stored for Fable/channel 63 in the queried local data. A client-side accumulator failure after HTTP 200 can leave only a server-side success/consume log, so absence of a new-api error record does not disprove malformed SSE.
- Correction: the suspected local ping/data write race is not present. `StreamScannerHandler` acquires the same `writeMutex` around both the ping writer and the entire data handler, so `event:`/`data:` pairs emitted by one handler call cannot be split by its synthetic ping.
- Public Fable-specific reports confirm its common leading shape: `content_block_start(index 0, thinking)` -> `signature_delta` only -> stop -> visible text/tool at the next index. Clients/proxies that discard an empty-looking thinking block lose index alignment. new-api's native Claude path relays this block unchanged; the third-party endpoint and Claude Code version remain the two likely points for the local type-14 route.
- The installed local Claude Code is `2.1.209`, newer than the official `2.1.186` sleep/wake fix. If the reported failures are occurring on this same installation, the known post-sleep client bug is unlikely to be the primary remaining cause.
- No `Content block not found` match exists in the currently available `~/.claude/debug` files, and no active shell environment override identified a different base URL/model. There is therefore no captured client event trace to compare against the server for this specific occurrence.
- An exact web search for `claude-fable-5` plus the error found no public Fable-specific reproduction beyond the general Claude Code sleep bug. This argues against claiming an Anthropic-confirmed Fable service defect.
- Anthropic's official Fable/extended-thinking docs precisely confirm the high-risk stream shape: default `display:"omitted"` still opens a `thinking` block, sends a `signature_delta`, closes it, then starts text at index 1. Empty thinking blocks must be passed back unchanged, including their signature.
- A low `max_tokens` can yield a successful response ending with `stop_reason:"max_tokens"`, potentially before visible text, but the official protocol still requires well-formed block start/stop events. Raising `max_tokens` may reduce empty/truncated turns; it is not a protocol-level explanation for the specific block lookup error.
- Request-side `thinking:disabled`, manual budgets, assistant prefill, ZDR eligibility, or malformed preserved thinking blocks produce documented 400/refusal/history problems. They are separate from a client accumulator saying a streamed block index does not exist.

## User-Supplied Request Evidence (2026-07-14)
- Request ID `20260714085229927488967BnZHA7jW` used channel 132 (`gemini91_claude-max`), path `/v1/chat/completions`, and conversion `OpenAI Compatible -> Claude Messages`. The call was billed for 2,476 uncached input, 2,788 cached input, and 338 completion tokens.
- This path sends an OpenAI-style request through `RequestOpenAI2ClaudeMessage`, receives a Claude Messages response, and converts it back with `StreamResponseClaude2OpenAI`. It does not use `StreamResponseOpenAI2Claude`, so the earlier confirmed delta-before-start defect is real but not the direct response path for this request.
- Channel 132 and this request ID do not exist in the local dev database/container logs; they belong to another deployment, so the raw upstream SSE cannot be recovered from this workspace.
- A code audit found a path-specific round-trip defect: Claude -> OpenAI response conversion emits Fable's thinking signature as `reasoning_signature`/`signature`, and the OpenAI request DTO accepts both fields, but `RequestOpenAI2ClaudeMessage` ignores all per-message `ReasoningContent`, `ReasoningSignature`, `Thinking`, and `Signature` fields when rebuilding Claude history.
- A temporary diagnostic passed an OpenAI assistant message containing `opaque-signature` plus a tool call and following tool result through the current converter. The resulting Claude history contained only `tool_use` and `tool_result`; the required preceding `thinking` block and signature were absent. The diagnostic file was then deleted.
- Anthropic officially requires the complete, unmodified thinking block (including signature) to be returned with tool-use cycles. Since Fable 5 always uses adaptive thinking and defaults to signature-only omitted thinking, this one-way mapping is a strong explanation for why `/v1/chat/completions` agent/tool workflows fail frequently after otherwise successful turns.
- The literal error is still not generated locally. The likely chain is: new-api drops Fable thinking state during OpenAI -> Claude history conversion; channel 132's upstream/proxy rejects or mishandles the resulting tool continuation and returns `Content block not found` (possibly mid-stream); new-api preserves that upstream error message.

## Ranked Conclusion
1. For the supplied Request ID, the literal error came from channel 132's upstream/proxy or its streamed error event; new-api has no emitter for it. The nonzero completion usage makes a mid-stream upstream error especially plausible.
2. The exact `/v1/chat/completions -> Claude Messages` path has a confirmed new-api compatibility defect: Fable thinking signatures are exposed on the response but discarded on the next request. This can trigger the upstream failure during tool-use continuations and explains the model-specific frequency.
3. Independently, third-party Anthropic-compatible providers have documented block-index bugs around thinking -> tool transitions. Raw channel-132 SSE is required to distinguish a malformed response sequence from a request-history rejection with certainty.
4. The Claude Code post-sleep bug is real but secondary here: this request uses Chat Completions, and the local Claude Code version already contains the official fix.

## Verification Results
- `go test ./service -run '^TestStreamResponseOpenAI2Claude' -count=1` passed.
- `go test ./relay/channel/claude -run 'Test(RequestOpenAI2ClaudeMessage|StreamResponseClaude2OpenAI|ResponseClaude2OpenAI)' -count=1` passed.
- The passing tests do not cover OpenAI assistant thinking/signature -> Claude thinking-block reconstruction; the temporary diagnostic proves that gap.
- `git diff --check` passed for the planning records. Product code was not modified.

---
# Async Image Open API and Webhook Findings (2026-07-17)

- Product code is clean at implementation start; existing modified planning files and unrelated untracked diagnostics belong to prior work and must be preserved.
- Current async submit synchronously calls image-handle before returning, while terminal callbacks already use a CAS transaction for task state and asset creation.
- Current public task query reuses the internal TaskDto and HTTP-200 dashboard envelopes, so a dedicated public DTO/error boundary is required.
- AssetKey already stores a scopes string with assets:read, making scoped webhooks:read/webhooks:write an extension rather than a new credential type.
- The task table cannot safely gain a global task_id unique constraint because it is shared with legacy provider task types; a one-to-one ImageTaskRequest table can own public image-task uniqueness and nullable idempotency keys.
- image-handle already has PostgreSQL task facts, BullMQ, stale recovery, and a callback outbox. Its required changes are limited to semantic fingerprints, provider_options persistence, and image URL download security.
- Existing new-api and image-handle Docker dev stacks are isolated; image-handle already ships an optional external gateway-network overlay.
- RelayTask currently performs channel selection, precharge, task/lease creation, synchronous image-handle submission, settlement, and consumption logging in one request flow. Durable dispatch should preserve its pricing snapshot and credential-lease helpers while moving only the internal HTTP submission behind an outbox.
- AssetKey.Scopes is already persisted as a comma-compatible string and middleware exposes asset_key_scopes in Gin context; adding scope validation does not require a new key model.
- Route distribution may cache the original public request body before the image-task normalizer runs. The normalizer must call common.CleanupBodyStorage, replace Request.Body/ContentLength, and let relay validation build a fresh reusable body store.
- Asset records already have a stable unique task_id + asset_index key and contain all public result-image metadata needed by the normalized task DTO.
- Durable dispatch can persist the exact signed internal image-handle request body after creating the credential lease; the worker only needs the global image-handle base URL/API key and does not store credentials in the dispatch row.
- Service already exposes an injected TaskPollingAdaptor factory, so dispatch exhaustion can reuse ApplyTaskResult for the same CAS/refund path without creating a service-to-relay import cycle.
- The durable create transaction currently inserts Task, credential lease, ImageTaskRequest, and ImageTaskDispatch together, but `PreConsumeBilling` still runs immediately before that transaction. A failed create transaction relies on the outer relay error path for refund rather than sharing one database transaction with the reservation.
- ImageTaskDispatch claims use conditional updates and expiring leases but have no per-claim lock token. A worker that outlives its lease could update a row after another worker reclaimed it; WebhookDelivery already demonstrates the safer lock-token pattern.
- Resource Center locale files retain 66-69 pre-existing missing keys depending on language, but all Webhook and scope keys introduced by this implementation are present in zh-CN/zh-TW/en/fr/ru/ja/vi.
- Resource Center currently embeds an Assets-only OpenAPI 3.0.3 object directly in `Assets/index.jsx`. The normalized task, upload, and Webhook routes are absent, so the new OpenAPI 3.1 document should be a standalone JSON source imported by the UI instead of expanding the page further.
- The frontend Docker stage currently copies only `web/`, so a direct import from repository `docs/openapi` also requires copying that canonical document into the frontend builder context/path. Keeping one imported JSON avoids a second drifting frontend copy.
- The unified public spec has 21 operations: 5 Assets, 4 async task, 2 pre-upload, and 10 Webhook management/delivery operations. Tasks/uploads use normal bearer tokens; Assets and Webhooks use `ak_` keys with `assets:read`, `webhooks:read`, or `webhooks:write` scopes.
- Durable task creation currently precharges before inserting the `(user_id, idempotency_key)` unique row, so concurrent same-key requests can both precharge before one loses the insert race. The durable branch should insert its claim/task/outbox first inside the transaction and precharge only after the unique claim is held.
- Permanent dispatch failure currently marks the outbox failed before `ApplyTaskResult` runs. If the terminal CAS/Webhook transaction fails, the task stays queued while the failed outbox is no longer retryable; outbox failure must be committed only after terminal transition succeeds, otherwise it should be rescheduled.
- BillingSession precharge mutates wallet/subscription SQL state and token quota/cache state through global model APIs. Calling it inside the current GORM task transaction would self-lock SQLite and still could not atomically commit Redis with SQL; literal cross-store atomicity requires a separate durable billing-reservation ledger/state machine, not just moving the existing function call.
- Docker failure acceptance task `task_GjiTaMXd4J1HCnXdwiUIyL6FjhwJuyBW` reached the normalized `failed` terminal state after the mock provider returned a permanent 404. It created no asset, emitted a valid signed `image.task.failed` event, and restored disposable user 994189 exactly to `quota=999900000` and `used_quota=100000`.
- The task row intentionally retains its request-time quota snapshot (`50000`) for audit even after the user's balance is refunded; user balance, not the task snapshot, is the refund source of truth.
- Channel 91 was restored from the deliberate `/missing` URL to `http://mock-provider:3999`, and `new-api-dev` returned healthy after restart.
- Local Docker now has healthy new-api and image-handle application containers attached to `ai-gateway`; real `new-api-dev` handles leases/callbacks while the mock service is used only as the image provider and the receiver only as a third-party Webhook target.
- The schema contract now has a reusable integration test. SQLite runs by default; disposable PostgreSQL/MySQL DSNs enable the same migration, TEXT/Unicode round-trip, idempotency, event, and delivery unique-index assertions. Local acceptance passed SQLite, PostgreSQL 15, and MySQL 5.7 with the project's required `utf8mb4` charset.
- Browser QA found fixed-width Resource Center SideSheets clipped inputs and footer actions at a true 375px viewport. All Resource Center/Webhook sheets now use `min(design-width, 100vw)`; rebuilt Docker QA confirmed no page overflow and full visibility of Webhook and API Key scope controls at 375x812.
- Cleanup removed the disposable user, token, asset key, channel, ability, model-ratio key, tasks, dispatches, leases, resources, quota summary, Webhook records/attempts, four image-handle facts/outbox rows, and four BullMQ jobs. Receiver events and its in-memory signing secret are empty. R2 objects remain subject to the configured one-day lifecycle.
- Repository-wide `i18n:lint` retains its existing 422 hardcoded-string baseline. The production build, i18n status, targeted ESLint/Prettier, and an explicit check of 63 new Webhook/scope keys across zh-CN/zh-TW/en/fr/ru/ja/vi all pass.

# Simplified Async Image Webhook Findings (2026-07-17)

- The current implementation exposes five endpoints per user, event filters, derived HMAC secrets, 24-hour dual-sign rotation, delivery inspection/retry APIs, and `webhooks:read/write` asset-key scopes.
- The repository already has a separate URL/Bearer-key Webhook for quota notifications in user settings. It cannot be reused silently because changing the notification channel clears those fields and its payload contract is unrelated to async image events.
- The simplification will therefore keep an independent account-level task configuration while reusing the established Bearer-key user mental model; current delivery remains image-only and future video events can share the same target.
- The approved future-video boundary is deliberately narrow: endpoint ownership, encrypted Bearer credentials, event/delivery persistence, retries, and the UI stay account-level; only the current terminal-event producer and public OpenAPI callback examples remain image-specific. No speculative video schemas or event producers are added now.
- Docker retry verification received the same `webhook.test` event twice after a configured first-attempt 500. Both requests passed Bearer validation and retained an identical stable event ID and payload.
- PostgreSQL confirms the retry delivery has exactly two attempts (500 then 204), final status `delivered`, and attempt count 2. The saved credential is a 74-character `v1:` encrypted envelope and contains no plaintext Key substring.
- Docker 410 verification passed: one Bearer-authenticated `webhook.test` received 410 and disabled the account configuration. Saving only the URL with an omitted Key re-enabled it, and a following 204 test still authenticated with the unchanged Key.
- The final cleanup found no generic-task blocker. One local-state issue was corrected: decryption-failure disable persisted a new timestamp but did not copy it into the returned object; the regression now asserts API/database equality against a forced stale timestamp.
- The normal self-delete endpoint soft-deletes only the user row and does not cascade durable Webhook logs. Local E2E cleanup therefore needs an exact user-scoped transaction after the normal API call so no receiver credential or test event remains.

## Saved-view and generated-Key follow-up
- The screenshot confirms the configured state still renders active URL/Key inputs and Save as the primary action, so it visually reads as an edit form despite the enabled status.
- The requested mental model is the existing API-token flow: new-api generates a prefixed credential, reveals it exactly once for copying, and never returns the stored plaintext afterward.
- Keep future task-type support unchanged: this follow-up changes only configuration presentation and credential issuance, not image/video event contracts.
- Current `WebhookTab` always renders both inputs and Save, exactly matching the reported ambiguity. Its hook already centralizes load/save state, so view/edit mode and one-time Key state can stay isolated in the existing Webhook component folder.
- The current PUT response only returns public configuration metadata. Token-style issuance requires a one-time plaintext field on create/regenerate responses while GET must remain unchanged.
- The real UI/UX skill scripts live under `/Users/zhangyu/.agents/skills/ui-ux-pro-max`; the Codex skill copy contains instructions but no runnable script at the attempted path.
- Resource API Keys are already server-generated through `GenerateAssetKey`, and the Resource Center has an established create-Key flow to inspect and mirror rather than inventing a foreign interaction pattern.
- The generic design-system search suggested marketing-style motion that conflicts with this operational settings surface, so existing Resource Center/Semi conventions take precedence. The applicable guidance is limited to clear state separation, focused components, accessible copy/edit actions, and 375px responsive verification.
- The Resource API Key tab uses a create SideSheet and system-generated credential returned after creation. The Webhook interaction should reuse its explicit action hierarchy while tightening security to one-time plaintext reveal.
- Selected contract: `PUT /api/webhook` creates a server-generated Key automatically, preserves it on URL-only updates, and generates a replacement only with explicit `regenerate_key`. Only create/regenerate PUT responses contain one-time plaintext; GET stays redacted.
- Selected UI: unconfigured create form, configured read-only detail rows, explicit URL edit, independent confirmed regenerate action, and a one-time copy modal. This avoids credential changes being coupled to ordinary address edits.
- The shared `copy()` helper is available from the same frontend helper barrel already used by the Resource Center. Existing locale files cluster the simplified Webhook strings near their end, allowing a tightly scoped seven-locale update.
- Existing service tests inject a fixed Key through the PUT request. They can be strengthened by consuming the one-time generated response instead, then asserting prefix/rotation/redaction while preserving the Bearer retry and 410 coverage.
- Backend implementation now enforces the token-style contract through the strict DTO: console callers cannot submit arbitrary plaintext Keys, create always generates, and only `regenerate_key` rotates an existing credential.
- User clarified that Webhook Keys are not one-time credentials: the owner must be able to reveal and copy them after creation at any time. Keep system generation/regeneration, but return the decrypted Key from the authenticated console configuration API while preserving encrypted-at-rest storage and log redaction.
- The final saved view now uses text/detail rows rather than disabled inputs: URL has an icon-only copy action; Key has masked/revealed text, status, eye/copy actions, and confirmed system regeneration. At widths below 640px the grid stacks to protect long Keys and translations.
- The outbound OpenAPI security description still says the user supplies the Key and must be updated to the generated `wk-...` contract before regenerating the checked-in spec.
- The locale tails contain both current simplified strings and older unused multi-endpoint strings. This follow-up will add only the new saved-view/generated-Key text and remove the two now-obsolete manual-Key prompts, avoiding unrelated translation churn.
- Reliable event, delivery, attempt, retry, lease, retention, 410, and SSRF behavior can remain unchanged behind a single internal endpoint record.
- User-supplied keys require reversible encryption for delivery; versioned AES-GCM keyed from stable `CRYPTO_SECRET` avoids storing or returning plaintext.
# Webhook Saved View and Generated Key Findings (2026-07-17)

- The supplied screenshot shows the saved URL inside an active input with Save/Refresh controls, so a configured Webhook still reads as an edit form.
- The accepted interaction is a read-only saved detail state with URL and Key rows; URL editing is explicit, while Key reveal, copy, and system regeneration remain available.
- The latest requirement intentionally allows authenticated account owners to reveal and copy the generated `wk-...` Key repeatedly; encrypted-at-rest storage and no plaintext logging remain required.
- The implementation keeps the account-level configuration task-generic for future video events without adding speculative video event behavior now.
- The in-app browser has no active local session and redirects `/console/assets` to `/login?expired=true`; use an isolated disposable local account for mutation-heavy UI checks.
- With a disposable account, the rebuilt unconfigured state shows only the Webhook URL input and `创建并生成密钥`; it does not ask for a user-entered Key.
- Creating the disposable configuration produces a 51-character Key (`wk-` plus 48 random characters) and immediately switches to the read-only detail state.
- The Key is masked by default after reload, can be revealed repeatedly, matches the original value, and both Key/URL copy actions show success feedback.
- The desktop saved-state Key row is functionally correct but visually crowded: the long Key wraps and the `已安全保存` tag is truncated beside reveal/copy/regenerate actions.
- Removing the redundant saved tag and using icon-only regenerate control makes the desktop Key row stable without losing reveal, copy, or confirmation behavior.
- URL edit/cancel/save works and preserves the Key. A normal URL save should leave the Key masked; only first creation and explicit regeneration should reveal it.
- Explicit regeneration requires confirmation, returns another 51-character `wk-...` Key, and invalidates/replaces the prior value.
- The final saved detail view has zero horizontal overflow at 560px and 375x812. Revealed Keys wrap within the page, and all icon/action controls remain separate and usable.
- Full validation passes: `go test ./...`, image-handle's 72 tests, frontend build, OpenAPI check, targeted Prettier/ESLint, and both repository diff checks.
- Full i18n lint is back to the known 422-item repository baseline; the Webhook component contributes no finding.
- The disposable user and its single Webhook endpoint were deleted after QA; there were no related events, deliveries, attempts, or tokens.
# Resource Center API Documentation Findings (2026-07-18)

- The UI advertises 11 operations: six async image/upload operations and five asset operations.
- The current async tab documents task creation well, but combines upload/query into one short section and omits list, batch query, and Base64 upload examples.
- The current asset tab only demonstrates list querying; single lookup, batch query, batch URL lookup, and CSV export have no executable examples.
- The clearest low-complexity structure is one collapsible operation entry per endpoint, each containing a curl request and representative success response; the endpoint table remains a navigation overview.
- Async task listing uses cursor pagination (`after`, `limit`), while task batch query accepts 1-100 IDs and separately reports unauthorized or unknown IDs in `missing`.
- Multipart upload accepts up to 10 repeated `image` fields and one optional `mask`; Base64 upload supports the simpler `images` plus optional `mask` shape.
- Asset list and conditional batch query return the same paginated list shape; single lookup returns one asset, batch URL lookup returns compact URL items, and export returns a CSV attachment.
- CSV export sets `Content-Disposition: attachment; filename=assets.csv` and returns the columns `asset_id,task_id,asset_type,url,filename,model,platform,action,created_at`.
- Existing locale catalogs already include the generic request/response labels, so the expanded examples can avoid introducing a large set of redundant translation keys.
- The frontend exposes dedicated `openapi:check`, `i18n:lint`, build, Prettier, and ESLint commands; the changed-file whitespace audit is currently clean.
- Docker desktop QA renders all async operation sections and 15 request/response example cards with no document-level horizontal overflow; the repeated `image` fields, list, batch query, and Base64 examples are present.
- Japanese QA exposed one pre-existing dynamic operation label, `创建异步图片任务`, missing from locale catalogs while the other newly added operation labels translate correctly.
- The in-app browser enforces a 560 CSS-pixel minimum for a requested 375x812 viewport; at both requested mobile sizes every example card stays within the document and only long code content scrolls internally.
- Asset API QA confirms all five operations have request/response pairs: list, single get, body query, batch URL lookup, and CSV export. The page has no document-level overflow.

---
# Automatic Error Snapshot Findings (2026-07-20)

- Backend settings, GORM index, bounded gzip storage, cleanup/reconciliation, admin APIs, and relay capture hooks are implemented in the current worktree.
- Focused backend tests pass across settings, model, service, controller, router, Claude, and relay packages.
- The existing Request Dump page is one large temporary-console component; a thin tab shell plus a separate `ErrorSnapshots` component keeps the old polling path isolated.
- The UI should remain a compact Semi Design operations surface. Existing theme tokens and typography take precedence over generic external dashboard palette recommendations.
- The remaining high-risk areas are destructive-action confirmation, responsive filters/table details, bounded cleanup tests, and end-to-end runtime verification.
- Status returns `settings`, `storage_path`, storage file/byte/oldest metrics, dropped/write-error counters, and cleanup/error diagnostics.
- List filtering accepts timestamps, exact request ID, user ID/username, channel ID, and an error keyword; responses use the repository's standard page envelope.
- Detail returns `{ snapshot, payload }`, while download returns raw gzip bytes and therefore needs an explicit blob response in the browser.
- Summary capture intentionally omits client/upstream request bodies; the detail view must distinguish this policy from an API read failure.
- The production frontend build passes after adding the new tab, status/settings surface, paginated responsive table, and four-section detail SideSheet.
- All translation keys used by the complete Request Dump page now exist in all seven locale files. Repository-wide i18n lint still reports its unrelated hardcoded-string baseline.
- Full `go test ./...` passes. The Claude integrity benchmark is faster and allocates less than the legacy switch-off path on the current Apple M2 Max run; first-block p95 is also lower.
- Generic stream handlers that return errors already pass through the shared attempt-level capture hook. Claude integrity additionally emits a bounded post-commit state-machine summary.
- The first full-page screenshot at a fractional browser scale misleadingly appeared to collapse the mobile content into a narrow column. Direct DOM geometry and a viewport screenshot disproved that assumption: at 375x812 CSS pixels the page/main/content widths are about 375/365/349 px, headings and banners use the full content width, and the document has no horizontal overflow. Full-page stitched screenshots at this scale are not reliable acceptance evidence, so the remaining mobile checks use viewport screenshots plus DOM bounds while scrolling.
- Mobile settings controls, priority selectors, read-only storage path, save/cleanup/clear actions, and the expandable filter surface all render inside the 375 px viewport. Expanding the action area exposes date, Request ID, user, channel, error keyword, search/reset, and the empty state without losing any control.
- Final security/boundary review found and fixed two plan-level gaps: unsupported multipart/binary priority requests now retain MIME, original size, SHA-256, and skip reason without persisting the body; structured and embedded-text masking now covers channel/API key, token, credential, password, and secret naming variants while preserving ordinary fields such as `max_tokens`.
- Final Docker fault injection passed all 15 scenarios against the rebuilt image, including disabled-path compatibility, hot 30/45/60-second settings, priority redaction, aggregate fallback, retry exclusion, pre-first-block EOF/timeout, in-flight snapshots, real-time valid streaming, post-commit interruption, final 502, and management APIs. Cleanup restored default error-snapshot settings, removed all snapshots, and left the app healthy at port 3001.

---

# Per-user Aggregate Route Model Ratio Findings (2026-07-21)

- Existing per-user aggregate defaults are stored as `map[aggregateGroup]ratio` in the user setting JSON; structured child-route rules can be added without a schema migration.
- Global child-route ratios are exact `(aggregate group, real group, model name)` rules and currently override the per-user aggregate default.
- The request context already carries the complete cached user setting and the selected real route group, so per-user exact lookup needs no database query in the relay hot path.
- User updates refresh the user cache, but the self-setting rebuild must explicitly preserve every new user-setting field.
- `/api/pricing`, `/api/user/self/groups`, token/Playground group renderers, and user log responses currently expose override identity or original values; UI-only removal would not satisfy the chosen privacy requirement.
- The existing aggregate model lookup endpoint requires aggregate-group menu permission; the user management SideSheet needs a user-menu-scoped lookup route.
- Docker dev is running PostgreSQL, Redis, and a healthy `new-api-dev` at `http://localhost:3001`.
- The UI should remain the existing data-dense Semi Design operations surface, with visible labels, Tabs for progressive disclosure, loading states, accessible icon actions, and mobile-safe layouts.
- Docker verification confirmed the complete precedence with real quota changes: `0.5` user exact, `3` global exact, `0.8` user aggregate default, and `1.2` aggregate default.
- Administrator logs retain route-model source and original-ratio audit data, while ordinary user logs and expanded detail views retain only the final `group_ratio`.
- Pricing must evaluate every reachable real group before selecting the maximum effective ratio; applying only the chosen route or aggregate default would understate dynamic-route pricing.
- The final responsive UI exposes inherited and effective values only to administrators; ordinary-user ratio surfaces render one final value with no comparison styling or override identity.
- The isolated Docker fixture and all matching PostgreSQL/Redis records were removed after acceptance, leaving the application healthy.

---
# Temporary Video Resource and Webhook Findings (2026-07-23)
- The user explicitly accepts temporary video access backed by the upstream provider; expiration does not require archival, repair, or retention guarantees.
- Existing `ak_` authentication already covers `/v1/assets*`, video status, and video content proxy routes.
- Successful video tasks already create `video` assets, but current asset URLs can preserve sub2api's internal UUID content path and therefore fail when resolved against new-api.
- The current durable outbound Webhook infrastructure is reusable, but event creation is image-specific and skips video tasks because it depends on normalized image request metadata.
- The preferred API shape is to reuse `/v1/assets?asset_type=video` for resource collection queries and retain `/v1/videos/{task_id}` for a single task status, avoiding a second task-list abstraction unless a concrete consumer requires it.
- `TokenOrUserAuth` delegates non-session requests to `AssetOrTokenAuth`, so the existing `/v1/videos/{task_id}/content` route already accepts `ak_` credentials and enforces the owning user through the task lookup.
- Resource Center video downloads therefore need no new route. The missing contract is a correct public URL projection in Asset/Webhook output; a relative sub2api UUID path must never be returned as though it belonged to new-api.
- Webhook fetch authentication remains intentionally split: outbound delivery uses the account `wk-` Bearer Key, while a receiver follows an authenticated new-api video URL with its separate Resource Center `ak_` Key.
- A normalized `video.task` Webhook object can be built from the generic task plus its video assets; unlike normalized image tasks, it must not require an `ImageTaskRequest` row.
- Final approved scope adds normalized creation, not only query: `POST /v1/video/tasks` covers generation, edit, extension, and remix while compatibility POST routes remain unchanged.
- xAI official contracts use separate generation, edit, and extension endpoints. Edit inherits source duration/aspect/resolution; extension duration is the added segment; input sources can be URLs, data URLs, or provider file IDs.
- The current task relay is single-output (`TaskInfo.Url`) and identifies video mainly from action names. The provider-neutral implementation needs persisted asset type/operation plus structured video outputs with a single-URL fallback.
- Per-asset content routes are required for multi-output tasks. The existing task content route remains a first-result compatibility alias.

---
# Multi-provider Async Video Implementation Findings (2026-07-23)

- The approved design is already specific enough to satisfy the brainstorming workflow; implementation can proceed without reopening product choices.
- Existing `TokenOrUserAuth` and Asset APIs already provide most read/download authentication primitives, while mutation routes need to be split away from `AssetOrTokenAuth` so `ak_` cannot submit video work.
- `model.Asset` already supports multiple records per task and ordered `asset_index`; the missing contract is structured provider output plus a public projection that hides internal metadata and selects direct CDN versus authenticated Asset proxy URLs.
- `service.ApplyTaskResult` already performs terminal task persistence, Asset creation, and image Webhook outbox creation in one transaction; video events should extend this transaction rather than introduce another worker.
- The normalized video request should follow the existing image request/idempotency persistence pattern but remain a separate `VideoTaskRequest`, because its operation/input/output/provider namespace contract is materially different.
- Existing `/v1/videos/*` compatibility routes and raw provider DTO parsing must remain untouched by normalized validation. The new `/v1/video/tasks` controller should translate its DTO before entering the established relay path.
- Ordinary video adaptors currently call upstream before the Task row is inserted. A normalized idempotency contract cannot be race-safe if it merely records the request after that call; it needs a database-unique reservation before the upstream mutation, with replay/conflict decisions made from the persisted fingerprint.
- `model.Asset` already has a `(task_id, asset_index)` unique key and ordered task lookup, so multi-output persistence does not require a schema change to Asset itself.
- `Task.Properties` is the correct durable place for provider-neutral `asset_type` and `operation`; query compatibility can fall back to existing task actions for historical rows.
- Normalized creation must remain independent of the image-handle-only platform override in `RelayTaskSubmit`; it should preserve normal channel distribution/model mapping and only inject a normalized request context plus provider conversion behavior.
- xAI already selects generation/edit/extension from the compatibility request path. For the normalized route it must instead consume the standard operation from request context, while the compatibility paths retain their existing raw-payload logic and response shape.
- `TaskAdaptor` can gain an optional normalized-video interface instead of forcing every existing adaptor to implement new methods immediately. Unsupported operations then produce a provider capability error, and a non-xAI mock can prove the common layer has no xAI assumptions.
- Existing video content proxy logic already handles Range, same-origin channel auth, redirect credential stripping, data URLs, Gemini, Vertex, and provider content endpoints. It should be extracted behind a shared task/asset resolver rather than duplicated in a second handler.
- Video terminal persistence currently sets `PrivateData.ResultURL` from the legacy single `TaskInfo.Url` and only then parses raw task data for Assets. Structured outputs should be added to `TaskInfo`, copied into Asset inputs first, with the current `Url`/raw-data behavior preserved as fallback.
- Current official xAI generation schema confirms duration `[1,15]`, aspect ratios `1:1`, `16:9`, `9:16`, `4:3`, `3:4`, `3:2`, `2:3`, and resolutions `480p`, `720p`, `1080p`.
- Current xAI documentation states that 1080p is supported only by `grok-imagine-video-1.5` for image-to-video generation; normalized validation rejects non-1.5 models plus text-only and reference-image modes while compatibility endpoints remain passthrough.
- 2026-07-23 video input follow-up: xAI `image` is one primary/start-frame source, while `reference_images` is an array of up to seven sources. Current xAI edit and extension accept a video source but do not document auxiliary images.
- The public controller currently rejects images for every non-generation operation and globally rejects `image` plus `reference_images`; these are provider capability decisions and prevent future adaptors from supporting video-plus-image workflows.
- The current Resource Center reference-to-video example incorrectly uses `grok-imagine-video-1.5`; current xAI documentation limits reference-to-video to `grok-imagine-video`.

# Async Image Public Error 524 Findings (2026-07-28)

- Ordinary self-log responses already pass through a separate sanitizer; the remaining leak path is the public async image task projection reused by polling and outbound Webhooks.
- image-handle already preserves structured provider diagnostics in callback error fields, so no cross-service protocol change is needed.
- A terminal failed task is immutable under its original idempotency key. Retrying requires a new `Idempotency-Key`; replaying the old key returns the original failed task.
- The public contract should describe `"524"` as a retryable business code while keeping the existing HTTP status behavior unchanged.
- The generated Resource Center OpenAPI still used `upstream_error` as its image failure example, and the GPT reference incorrectly advised reusing the original idempotency key after a terminal failure.
- Gemini already shares the same image-task and Webhook DTOs, so the `"524"` contract must be documented identically for both providers rather than as a Gemini-specific endpoint.
- The local Docker dev stack is already healthy with `new-api-dev`, PostgreSQL, Redis, image-handle, and the async test mock; acceptance can rebuild only `new-api-dev` and use disposable database fixtures.
- Resource API keys are stored as `ak_` values in the local `asset_keys` table and scope task reads to one user, allowing a disposable user/key/task fixture to exercise the real HTTP task endpoint without exposing any existing credential.
- The Docker dev service shares its PostgreSQL volume with the current healthy container, so a targeted rebuild/recreate preserves existing local state; acceptance fixtures must use unique IDs and be deleted afterward.
- image-handle already emits `upstream_status`, `provider_error_code`, `provider_error_type`, and `provider_error_message`, but new-api's `controller.imageCallbackError` previously declared only `code`, `message`, and `retryable`. JSON unmarshalling therefore dropped the structured fields before task persistence.
- The callback DTO must retain those fields so public masking can prefer stable machine codes while administrator task data keeps the raw diagnostic. This remains a new-api-only change because the sender contract already exists.
- Terminal image callback handling always creates the durable Webhook event payload when an image-task request record exists; an enabled account endpoint is needed only for delivery creation. Docker privacy acceptance can therefore avoid external delivery side effects.
- The callback signing ID is intentionally `image_handle_callback` (or a channel-specific `channel_<id>`), while `image_handle_1` is the credential-lease internal secret ID. The first Docker probe mixed these two independent IDs; the configured callback secret itself was correct.
- The master process polls all nonterminal image tasks, so a manually inserted queued fixture with channel ID zero can be failed by the background poller. Docker callback acceptance must reset and callback atomically enough to precede the next poll, then inspect the resulting terminal row.
- Durable Webhook events store their public payload at terminal transition time. Pending retries created before this release could therefore retain legacy sensitive messages even after the task-query code is upgraded.
- A send-boundary sanitizer now rewrites only quota-related `image.task.failed` payloads in memory before HTTP delivery, fails closed on malformed failed-image JSON, leaves the stored administrator event unchanged, and preserves every unrelated event payload byte-for-byte.
# Resource Center DTO Documentation Findings (2026-07-24)

- The current Resource Center already has executable request/response examples, but users cannot reliably infer required fields, scalar types, nested shapes, constraints, or enums from examples alone.
- The generated OpenAPI document should remain authoritative; a reusable schema renderer can expose the same contract in the dashboard without creating duplicate field metadata in JSX.
- This task is documentation-only. Authentication remains split between ordinary API Token submission, Resource API Key query/upload/asset access, and the independent outbound Webhook Key.
- The page currently advertises 16 operations: four image-task operations, four video-task operations, two upload operations, five Asset operations, and one video-content download operation.
- `ResourceCenterDocs.jsx` already imports the generated JSON document, so schema lookup/rendering can be added without a new runtime dependency or network request.
- The existing UI uses operation IDs as stable identifiers and keeps hand-written examples in the same component; the new renderer should bind definitions by operation ID and leave those examples intact.
- Existing schemas already encode many structural rules (`required`, `enum`, numeric bounds, item limits, conditionals), but most properties have no human-readable `description`, so a renderer alone would still leave important semantics unclear.
- Operation inputs are split across path/query/header parameters and JSON or multipart request bodies. The UI must render both groups, not only `requestBody`, otherwise list/get/export operations would still lack their DTO-equivalent contract.
- Successful outputs include JSON task/asset objects, binary video content, and CSV export. The definition component needs a concise non-JSON representation for binary/text responses instead of pretending all responses are object schemas.
- Backend DTOs confirm optional image/video output scalars are pointers, while `model`, `operation`, `input`, and `input.prompt` are required. Public task timestamps are Unix seconds and terminal/start times are nullable.
- Image task creation supports both `application/json` generation/edit requests and `multipart/form-data` edit requests on the same operation; multipart `image` is repeatable (1-10 files), `mask` is optional, the total body limit is 100 MiB, and each file is capped at 20 MiB.
- Asset query DTOs also contain `platform` and `action`, even though the existing hand-written “常用查询参数” table omits them. Rendering directly from the OpenAPI operation will prevent this sort of partial coverage.
- Public video task source is a union: either `url`, or `provider + file_id`. Non-generation operations require `input.video`; selected provider adaptors may impose tighter capability constraints.
- A generated-schema audit shows nearly every request/response property currently lacks `description`; the useful metadata already present is mostly structural. Key semantics must therefore be added to the generator before the UI is considered complete.
- Image JSON validation accepts only absolute HTTP(S) input URLs; multipart upload accepts PNG/JPEG/WebP, up to 10 images plus one mask, 20 MiB per file and 100 MiB total. Base64 accepts plain base64 or data URLs with the same file/count/type limits.
- Asset public list/export parsing internally still accepts `platform` and `action`, but the public Asset DTO and existing OpenAPI intentionally omit those routing details. They remain outside the public contract; the incorrect hand-written single-Asset example that exposed them must be corrected instead.
- The Asset error envelope currently omits `param` and `request_id` at runtime even though task APIs include them. The documentation must not falsely mark those fields as universally required; all five inner error fields should remain optional except `code/message/type` only where guaranteed by the shared response schema.
- Chinese dashboard descriptions are sourced from generated `x-description-zh-CN` values. Chinese locales deliberately do not fall back to the standard English OpenAPI `description`, so a missing translation renders no English sentence.
- The displayed-contract audit covers schemas, operation parameters, JSON/multipart request bodies, success responses, and response headers; every displayed description has a Chinese extension (`missingCount: 0`).
- `oneOf` requiredness must distinguish fields shared by every object variant from fields required by only one variant. Browser acceptance confirms video source `url` and `provider + file_id` are conditionally required, while the image/video task fields shared by every Webhook variant remain required.
- At 375x812, the schema renderer hides its desktop table and exposes stacked field definitions. The final document, body, and main scroll widths all equal 375 CSS pixels, and the inspected long field names remain within the viewport.
- The first implementation technically used tables, but an outer `Collapse` hid every definition behind “请求与返回参数”; this explains why users could not discover the field contract. Field definitions should be visible by default, with Name, Type, Required, Description, and Notes as distinct information columns.
- Rebuilt desktop acceptance confirms the Async Images page contains the visible five-column header and real `model` field row immediately after selecting the tab; there is no remaining schema-collapse button.
- The rebuilt 375x812 view renders the same information as labeled stacked rows (`类型`, `描述`, `备注`), including `model`, requiredness, Chinese description, request-body location, and constraints. Document, body, and main widths remain exactly 375 CSS pixels.

---
# Adobe2API Seedance 2.0 Fast Integration Findings (2026-07-29, implementation)

- The new-api task adaptor contract already exposes all three capabilities needed by AdobeVideo: normalized request preparation, standardized video billing estimation, and provider-specific content resolution. No billing-core interface change is required.
- Adobe2API's pending worktree is limited to the six expected implementation/test files and contains the completed asynchronous route, catalog, store, and test changes.
- The frontend channel constants path differs from the earlier planning note and must be discovered from the current source tree before editing.
- Existing xAI/Sora task adaptors do not implement `VideoContentResolver`; AdobeVideo must provide the first provider-specific resolver that authenticates and proxies Adobe2API's private content endpoint.
- Adobe2API submit and poll responses share the task shape (`id`, `task_id`, `status`, `progress`, `duration`, `aspect_ratio`, `resolution`, optional `video_url`, optional structured `error`). Its content endpoint is authenticated `GET /v1/videos/{task_id}/content` and returns `FileResponse`, so the stable provider reference is the upstream task ID rather than the returned deployment-specific URL.
- A completed AdobeVideo result should emit a `VideoOutput` with `ProviderReference=<upstream task id>` and a non-empty resolver marker. The existing asset pipeline will persist both fields and force content through the adaptor resolver with Range/If-Range forwarding.
- `ChannelType2APIType` must remain unchanged for AdobeVideo. Existing task-only Sora and DoubaoVideo channel types are intentionally absent from that synchronous adaptor mapping; mapping AdobeVideo to OpenAI would pollute its advertised model list and permit accidental protocol misuse.
- Docker dev already connects new-api and Adobe2API through the external `ai-gateway` network, while the opt-in async mock shares new-api's private dev network. Mock acceptance can target `http://async-test-mock:8080` without exposing or changing the real Adobe2API channel.
- The local `VideoPricing` Option is currently absent and `GroupRatio` exists. Acceptance cleanup must therefore delete the temporary VideoPricing Option rather than restoring an invented empty value, while restoring the exact prior GroupRatio text.
- A meaningful wallet-only check requires an active subscription and `subscription_only` user preference. The disposable fixture will include both; wallet movement and unchanged subscription usage will be asserted independently.
## 2026-07-29 — Claude 渠道“空任务回复”

- 当前只确认了用户提供的材料：测试脚本路径、两张截图、仓库当前源码。
- 截图声称 channel 70 为原生 Claude 协议，`new-api` 只透传上游 SSE；该说法尚需对当前源码和脚本验证。
- 截图中的三类现象不能预先视为同一根因：
  - 9/12 正常；
  - 2/12 HTTP 200 且只有 ping、无 usage；
  - 1/12 HTTP 200 返回空任务式文本，但有完整 input/cache usage。
- 工作树已有大量与本诊断无关的未跟踪文件；本次不触碰。
- 客户脚本的已确认行为：
  - `burst` 并发复用一个 `http.Client`，每条请求重新构造 body，只改唯一 tag。
  - body 并非“形态完全一致、只改哨兵”：默认还会覆盖 `max_tokens=256`，并在第一个 system 文本块前注入唯一 marker；指定 `-model` 时还可能删除 `thinking`。
  - `setLastUserText` 是从尾部寻找最后一个 `role=user`，只替换其最后一个 text block；不会删除同一 user 消息内的其他非 text 内容，也不会删除该 user 后面的任何 assistant/system 消息。
  - `GREET` 实际口径包含任何“既无哨兵、又非 tool、又非空”的输出，不只是真正问候语。
  - `EMPTY` 只表示脚本没有收集到 `content_block_delta.delta.text`；它会忽略 thinking、JSON delta 等非 text 内容，因此不等价于“上游 SSE 只有 ping”。
  - usage 只取 `message_start.message.usage`，不是最终 `message_delta` 的完整 usage。
- 因此脚本注释中“回问候语即证明模型没拿到任务正文”和“空流即整条 SSE 无任何内容事件”均超出了代码能证明的范围。
- 用户已澄清：截图中的 `channel: 70` 只是客户接入后记录到的渠道 ID，`supertoken` 即己方服务层。本诊断不再把 channel 70 当作协议或供应商线索。
- 当前仓库存在 `relay/channel/claude/response_integrity.go` 及覆盖 ping、提前 EOF、`message_stop` 的测试；需确认客户部署 commit 是否包含该逻辑。
- 在本机常见临时路径未找到 `gwprobe-sample.json`；目前没有真实 capture body 可核验消息顺序和 content block 结构。
- 已确认旧 Claude 原生流路径的行为：
  - `ClaudeStreamHandler` 使用通用 `StreamScannerHandler`，该 helper 会先设置下游 SSE headers，并可由独立 goroutine 定时调用 `PingData(c)`。
  - scanner EOF、scanner error、idle timeout 只写日志/context stop reason，不作为错误返回给 `ClaudeStreamHandler`。
  - 之后 `HandleStreamFinalResponse` 会在 usage 不完整时本地估算 usage，handler 最终仍返回成功。因此“HTTP 200 且只有 ping”可以由 `new-api` 自己造成，不要求 supertoken 发过 ping。
- 当前完整性保护路径会在首个 `content_block_start` 前缓冲所有事件；如果只有 ping 后 EOF，则返回 `claude_content_block_missing` / 502，允许 controller 在响应尚未提交时重试。该能力受 `ClaudeResponseIntegrityEnabled` 开关控制。
- `I'm ready. What would you like me to work on?` 不存在于当前仓库源码。旧原生 Claude relay 对 `content_block_delta` 文本没有生成逻辑，只做解析、usage patch 和 SSE 重写；该文本不是 `new-api` 内置兜底文案，但仍可能由己方 supertoken 后面的模型调用链响应，不能据此把责任推给外部渠道。
- 完整 usage 只能证明上游对某份约 27k token 的输入做了计费；不能证明客户期望的最后任务文本包含在上游实际处理的那份输入中。
- Claude 响应完整性修复 commit 为 `148a2c626`，提交时间 2026-07-20 05:21:23 +0800。
- 当前 `main` 已包含修复，但默认设置仍是 `response_integrity_fallback_enabled=false`，首块超时默认 30 秒。若 supertoken 未显式开启该配置，请求仍走旧的“先提交 200 + 本地 ping + EOF/scanner error 不上抛”路径。
- 仓库已有两组与此问题直接相关的既有诊断/验证材料：
  - `tmp/claude-fable-5-content-block-diagnosis`
  - `tmp/claude-response-integrity-validation`
  后续将读取其结论和复现记录，不重新制造生产请求。
- 既有 fault-injection 验证已确认：
  - 开启完整性保护后，首块缺失的坏路由返回内部 502 并可切换到下一真实路由；
  - 坏尝试不产生 consume log，只有最终成功路由计费；
  - 关闭开关时仍走 legacy handler；
  - 测试完成后运行时配置恢复为“关闭、30 秒”。
- “空任务问候”包含正常文本 content block，响应完整性状态机不会把它判为坏流。因此它与“只有 ping”可能同源于上游不稳定，但不是同一个可观测协议错误，不能用首块完整性修复直接识别。
- 既有 `Content block not found` 诊断针对 HTTP 200 + `content=[]`/refusal，证明旧 non-stream handler 同样会把空内容成功响应原样交给客户端；这与本次有文本问候的样本不同，但共同说明旧路径把 HTTP 200 当成功，缺少语义/结构完整性门槛。
- 当前 native Claude 请求处理会为每次请求 `DeepCopy` 出独立 DTO，Anthropic adaptor 的 `ConvertClaudeRequest` 本身不改消息；后续 compat/param override 可能改 JSON，需继续核对 body storage 与实际配置。
- 对 native Claude relay，OpenAI-style role 归一化只在 `RelayFormatOpenAI -> FinalRequestRelayFormatClaude` 时启用；原生 `/v1/messages` 不会自动把中途/尾部 `role=system` 转成 user。是否原样放行取决于 pass-through 与 schema validation 配置。
- 客户脚本的 `keep-system` 对照是关键但当前截图没有给出两组结果；没有 capture body 也无法确认最后 user 后是否仍有 assistant/system。
- 异常条目的大额 `input/cache_creation` 是“超大请求被计费”的证据，不足以证明哨兵是上游最后一个有效任务或确实进入了最终模型上下文。
- `BodyStorage` 存在于单个 Gin context 内；pooled buffer 只在 storage close 后归还，request 间没有共享 reader。每次 relay/retry 都通过 `GetBodyStorage` seek 到 0。现有代码不支持“并发 12 条随机共用/串写请求 body”的猜测。
- pass-through 路径用 `ReaderOnly(storage)` 避免 `http.NewRequest` 提前关闭 storage；非 pass-through 路径为每条请求创建独立 marshaled buffer。未发现会按概率丢掉最后 message 的本地代码分支。
- capture 阶段只抓第一个 body >1000 字节的 `/v1/messages`，没有断言 body 包含 `GWPROBE_CAPTURE`，也没有断言最后一条 message 是 user；这是测试样本有效性的缺口。
- 本机文件中查不到截图两条 Request ID，无法从当前 workspace 还原它们的 request dump 或 raw SSE。
- 普通相关包测试通过：`go test ./relay/channel/claude ./relay/helper ./controller ./common -count=1`。
- race run 失败并发现两类竞争：
  - legacy `StreamScannerHandler` cleanup 的 `stopChan` send/close race，出现在 `TestClaudeStreamHandlerMessageStopOpenUpstreamReturns` 和关闭完整性开关的 legacy 测试；与流收尾可靠性直接相关，但不修改请求正文。
  - `GetClaudeSettings().NormalizeValidationModes()` 对全局 settings 的原地写入与 Claude 包内并行测试的 settings 访问竞争；需隔离测试夹具因素，当前不能归因到客户 greeting。
- settings race 的触发测试本身不修改全局配置，只是 `t.Parallel()` 同时调用请求转换；因此 `GetClaudeSettings()` 并发同值写入是实际生产代码竞争，不只是测试 cleanup 干扰。竞争字段为 validation modes 等全局配置，不直接持有/改写 request `messages`。
- legacy stream 单例 race 测试首次隔离运行通过，stop-channel race 具有时序性；继续多次单例确认。它即使存在也位于流 cleanup，而非请求 upload/body mutation。
- `go test -race ./relay/channel/claude -run '^TestClaudeStreamHandlerMessageStopOpenUpstreamReturns$' -count=30` 稳定复现 stop-channel send/close race 并失败。该 race 是确认的 legacy 流收尾缺陷。
- stop-channel race 与已确认的“EOF/scanner error 只记日志、不返回 handler error”共同支持 ping-only 的网关根因；两者都不能生成语义完整的英文 greeting。
- 测试负载并非普通重复请求：默认 `cold=true` 在第一个 system block 注入唯一 marker，且每条 sentinel 不同；截图 `cache_read=0`、约 27k cache creation 表明 12 路都是大冷前缀。按异常条目约 30,198 输入 token 估算，12 路同时约 36 万输入 token，可能主动放大后端排队/超时问题。
- 延迟分层（正常 4.5–7.3s、ping-only 约 22s、greeting 58.9s）与并发冷启动下的排队/超时或后端异常分支相符；这是合理推测，不是仅凭时间即可证明的根因。
- `1/12` 的 Wilson 95% 区间约为 1.5%–35.4%，生产 `39/500` 约为 5.8%–10.5%；点估计接近不是强证据。若合并 2 条 ping-only，本轮静默失败为 `3/12=25%`（区间约 8.9%–53.2%）。
- 现有 request dump 支持 `raw_request` 和 `upstream_request` 两阶段，后者记录最终转换 JSON；对 pass-through 且未启用 compat 的路径，raw body 本身就是最终 upstream body。greeting 要定根因仍需这两阶段 body/tag 和出口 raw SSE。

### Final assessment

- 已确认根因（ping-only）：legacy Claude stream handler 在收到首个 content block 前就允许本地 ping 提交 HTTP 200；EOF/scanner error/timeout 不上抛，最后仍按成功结束。该路径另有可复现的 stop-channel send/close race。
- 已有但默认未启用的保护：2026-07-20 的 Claude response integrity fallback 会缓冲至首个 content block，并把首块前 EOF 变为 retryable 502；当前默认开关是 false。
- 不能确认的根因（greeting）：英文问候不是 new-api 固定文本，且它是有正常 content block/usage 的协议完整响应；当前代码未发现并发 BodyStorage 串包或随机丢 message 的路径。
- greeting 的高优先级候选：
  1. capture/replay 没保证 sentinel 是最终有效 message（可能有尾部 assistant/system 或同 message 其他 block）；
  2. 12 路约 3 万 token 冷前缀并发触发己方后端排队、超时恢复或语义降级；
  3. 更下游模型服务处理了完整计费输入但未遵从指令。
- 排除/低优先级：new-api 内置问候兜底、prompt output cache 命中、当前代码中的跨请求 body reader 复用。
- 最小闭环证据：异常 tag 对应的 raw request、final upstream request、出口 raw SSE；并验证 `messages[len-1].role=user` 且最后 user 只有一个 sentinel text block。
## 2026-07-29 — Claude 空流/空任务回复公开讨论检索

- 待确认官方仓库 remote 后再开始 GitHub issue/PR 定向检索。
- 目标症状拆为四组关键词：
  - `empty stream`, `empty response`, `ping only`, `EOF`, `scanner error`
  - `content block not found`, `missing content block`, `content=[]`
  - `I'm ready`, `What would you like me to work on`, `no task`
  - `model mapping`, `wrong model`, `model spoofing`, `fallback model`
- 本地 remote 关系：
  - 当前 new-api checkout 的 origin 是 `snakeeeeeeeee/new-api` fork；代码 module/项目身份指向官方候选 `QuantumNous/new-api`。
  - sub2api checkout 的 origin 是 `snakeeeeeeeee/sub2api` fork，upstream 明确为 `Wei-Shaw/sub2api`。
- 已加载 GitHub repository/issue search 与 Exa web search/fetch；后续优先读取原始页面。
- GitHub connector 已确认 `QuantumNous/new-api` 为公开、未归档仓库，默认分支 `main`，repo id `717197250`。
- GitHub connector 已确认 `Wei-Shaw/sub2api` 为公开、未归档仓库，默认分支 `main`，repo id `1118601518`。
- new-api 初步高相关 Issues：
  - `#5411`：Claude Code ultracode 尾部 `role: system` 经 DeepSeek Anthropic-compatible relay 后返回合法但空的 assistant turn；SSE 有 `message_start(content:[]) -> message_delta(output_tokens:1,end_turn) -> message_stop`，没有 content block。与当前“最后 user 不一定是最终 message”的假设高度吻合，但上游模型不同，且需核对作者/日期确认是否为独立报告。https://github.com/QuantumNous/new-api/issues/5411
  - `#6429`：AgentRouter + Claude Code/Trae 间歇性 `API returned an empty or malformed response (HTTP 200)`，另伴随 new-api panic。相似症状，但部署与根因未确认。https://github.com/QuantumNous/new-api/issues/6429
  - `#5483`：OpenAI→Claude 流转换产生非连续 content block index，官方 Anthropic SDK 崩溃。证明 Claude SSE 转换路径有公开协议完整性问题，但与 ping-only 不是同一缺陷。https://github.com/QuantumNous/new-api/issues/5483
  - `#5402`：Claude native relay 额外空行，属于 SSE frame formatting 差异，不会直接解释无正文。https://github.com/QuantumNous/new-api/issues/5402
- sub2api 初步高相关 Issues：
  - `#4493`：Grok free 账户经 Claude Code 偶发 `API returned an empty or malformed response (HTTP 200)`。与空回复症状直接相似，根因尚未给出。https://github.com/Wei-Shaw/sub2api/issues/4493
  - `#1528`：Anthropic content block 缺 `text:""`，usage 有 output tokens 但 SDK 得到 `text=None`；是“计费/usage 正常但正文丢失”的另一个已报告兼容层缺陷。https://github.com/Wei-Shaw/sub2api/issues/1528
- Issue 元数据核验：
  - new-api `#5411` 由独立用户 `chensunny` 于 2026-06-10 创建，标签 `bug`，2 条评论，同日关闭；早于本次排查，可视为独立相似报告，仍需读取关闭理由。
  - new-api `#6429` 由 `RAJAT-KHANDELWAL-PROFILE` 于 2026-07-23 创建，30 秒内关闭并标记 `invalid`；证据权重低。
  - sub2api `#4493` 由 `liushunqiu` 于 2026-07-17 创建，当前 open、0 评论。
  - sub2api `#1528` 由 `ycjcl868` 于 2026-04-09 创建，当前 open、0 评论。
- new-api `#5411` 评论/关闭理由：
  - reviewer bot 仅要求补充精确版本/commit，没有否定复现。
  - 维护者 `seefs001` 回复：“原生 claude -> deepseek官方 路径的话，这种事情应由Provider去做而不是NewAPI”。
  - 因此该 Issue 确认了公开相似症状与尾部 system 触发链，但官方把修复责任归给 Provider；它不是 new-api 官方承认自身 bug。
- new-api `#6429` 被标 invalid 是因为缺失必填模板/版本/复现步骤，bot 建议重新提交；不是对间歇性 HTTP 200 空/畸形响应真实性的反驳。
- sub2api `#2377` 是目前与客户 greeting 最接近的公开报告：2026-05-11，独立用户 `selfancy` 报告 OpenAI 转发给 Claude Code 后只返回 `Great — thanks. I’m ready. What would you like me to do next?`，并明确称“丢失了上下文信息”；issue 当前仍 open。
- sub2api `#2377` 下另一位独立用户 `realzolo` 明确回复“同样遇到了这个问题”；但该 issue 没有维护者定因、原始请求体、上游响应或修复链接，因此只能证明同症状不止一人，不能证明与客户样本同一根因。
- sub2api `#4077` 报告 DeepSeek V4 Pro 经 `/v1/messages` 转换时频繁出现 `API Error: Content block not found`；另一名用户在评论中给出 `#4114` 的代码级根因。
- sub2api `#4114` 已确认一类具体转换 bug：Read 工具参数被缓冲但依赖上游 `.done` 事件 flush；GLM/DeepSeek Chat Completions 上游不发该事件时，参数和对应 delta 被吞，Claude Code 合成 `Content block not found`。修复 PR `#4138` 于 2026-07-13 合并。
- sub2api `#4193` 确认另一类独立转换 bug：并行 tool use 时 block index 错位，向从未 start 的 block 发送 delta；两名其他用户在评论中确认遇到同错误。精准修复 PR `#4294` 与直转重构 PR `#4295` 均于 2026-07-15 合并。
- sub2api `#4177` 证明模型映射实现确实可能出错，但方向与“暗中换模”不同：配置 `claude-fable-5 -> gpt-5.6-sol` 后，转发层反而丢弃已解析映射，最终仍把原始 `claude-fable-5` 发给上游。关联修复 PR 为 `#4179`，需继续核验合并状态。
- new-api `#4067`（2026-04-03）报告 Claude Code 卡在思考状态、后台已扣费但无回复；维护者认为客户端一次对话可能对应多次 API 调用，并要求本地抓包。issue 最终因信息不足关闭，因此是相似症状证据，不是已确认的 new-api 根因。
- new-api `#4697`（2026-05-08，当前 open）报告 OpenAI→Claude SSE 缺少尾部 `content_block_stop/message_delta/message_stop`，已有另一用户确认相同问题；维护者在官方 Qwen 渠道无法复现，后续报告者提供的是“与阿里云格式一致”的企业上游而非维护者要求的同一官方测试条件。因此该问题存在公开争议，不能写成官方确认。
- new-api `#4139`（2026-04-08，当前 open）确认另一种 HTTP 200 空输出路径：阿里百炼流式审核错误在 HTTP 200 流内返回，new-api 已提交成功响应且不会重试其他渠道；维护者当时回复“目前无法修复”，报告者随后询问是否支持空回复重试。它证明“HTTP 200 但无正常正文、不能重试”是公开已知问题，但供应商和事件形态与客户 ping-only 不同。
- new-api `#2999` 报告高并发/高负载时 stream scanner 的硬编码 10 秒 channel 写入超时可能中断正常长思考请求，当前 open；这是客户并发冷请求会放大流中断的旁证，不是 ping-only 的直接复现。
- new-api `#3511/#4129` 与已合并 PR `#4128` 证明 Claude 原生流在中断路径曾存在 usage fallback/计费字段丢失问题；这解释“断流后本地如何结算”，但不负责生成 greeting。
- new-api `#1502` 报告 Claude SSE 事件顺序异常，维护者仅回复升级新版后关闭；new-api `#4697` 报告结尾事件缺失。这些均说明 Claude Code 对 SSE 生命周期严格，而兼容层曾有多种公开协议缺陷，但不能都合并为同一故障。
- new-api `#4389` 是较大规模的 `Content block not found` 集中报告：2026-04-22 创建后至少十余名用户回复同症状，部分用户称新 Claude Code 版本更容易触发，另有人称改用旧版可用。维护者因缺少可控官方上游复现于 2026-06-16 关闭；后续关联到 OpenAI→Claude 并行 tool block 修复 PR `#6394`。它证明现象广泛，但每个回复未必同一根因。
- LiteLLM `#24765` 公开了相同协议级错误：OpenAI SSE 转 Anthropic SSE 时偶发丢 `content_block_start`，后续 delta 指向不存在的 block，Claude Code 报 `Content block not found` 并回退到非流。该报告提供了转换链和响应指纹，是强相关的跨项目证据。
- `ik_llama.cpp #1524` 也报告 Claude Code 流式 Anthropic API 的 `Content block not found`，non-stream 正常，并由 PR `#1543` 修复；再次说明该错误通常是 SSE block 生命周期/转换问题，不是模型“不会回答”。
- `free-claude-code #137` 有多位用户报告 `API returned an empty or malformed response (HTTP 200)`；该案例最终通过把 llama.cpp context size 从较小值提升到 262k 后消失。它不能解释客户约 30k 输入为何失败，但证明同一客户端错误也可能由上下文容量/代理后端产生。
- `9router #1025` 报告 GLM 经过 Claude→OpenAI 转换后，代理日志显示约 98k input、291 output、上游“成功”，Claude Code 仍显示 HTTP 200 空/畸形响应。该 issue 当前 open，根因未闭环。
- Claude Code 官方 issue `#68582` 报告多个后台 agent 输出同时注入导致上下文瞬间溢出，随后 API 返回 HTTP 200 empty/malformed；这是“大并发/大上下文可触发空成功”的官方客户端侧相似案例，但触发点与客户 12 个独立请求不同。
- LINUX DO 可检索到至少三条相似主题：
  - `2037625`：`求助: sub2api, 模型经常返回空值`；
  - `1996734`：`cc这个报错是什么原因：API Error: Content block not found`；
  - `2408402`：CLIProxyAPI + Grok 在 Claude Code 中经常 `Content block not found`。
  其中后两条可确认正文主题；`2037625` 页面正文未能通过公开抓取稳定提取，因此只将标题视作弱证据。
- V2EX 定向搜索只返回标签/节点页，没有找到可核验的具体同症状帖子。X 定向搜索也没有返回稳定可访问的结果；这只能说明本次检索未命中，不能写成“X 上无人遇到”。
- sub2api `#1493` 是明确的 HTTP 200 + 正常 usage + `output=[]`：非流式 `/v1/responses` 未从上游 SSE delta 重建最终 output。修复 PR `#1501` 于 2026-04-08 合并；issue 本身仍 open，但 PR 明确写明 fixes `#1493`。
- sub2api `#2972` 是另一条高度相关的已合并修复：兼容层曾把上游 `response.failed` 错误转换成 `HTTP 200 + 空 assistant message + finish_reason=stop`。2026-06-05 合并后会在客户端输出提交前触发 failover/502。
- sub2api `#1661` 报告接入 new-api 的账号测试会在无任何正文时仍显示测试成功；说明“只以 HTTP/流程完成判断成功”也出现在管理测试路径。
- sub2api `#2064` 报告大输入跨过特定边界后稳定返回 HTTP 200、`content:null`、`finish_reason=stop`、无 usage；同样请求较小或换模型正常。它支持“上下文/路由容量边界可能表现为空成功”，但模型与 token 规模都不同于客户案例。
- 关于“换模/掺假”的公开证据需要区分能力与事实：
  - new-api 本身明确支持 `model_mapping`，因此网关从技术上能把对外模型 A 映射到上游模型 B。
  - new-api `#4465/#5868` 显示当前设计会把上游实际响应的 model 名返回给客户端，维护者明确拒绝增加“隐藏原始模型名”的功能。因此在上游如实回显 model 且未再嵌套代理时，响应 model 字段是可用线索；但它不是密码学证明，上游仍可自行回显别名。
  - sub2api `#4177` 及已合并 PR `#4179` 证明 Fable 精确映射曾有实现 bug：期望 `claude-fable-5 -> gpt-5.6-sol`，实际却继续发送原始 Fable 名。方向是“映射没生效”，不是“暗换成别的模型”。
  - sub2api `#1651` 报告 `/v1/chat/completions` 曾稳定把 `gpt-5.4-mini/gpt-5.3-codex` 静默改写为 `gpt-5.4`，并有多位用户确认类似 fallback；issue 当前 open。sub2api `#3915` 也有用户报告 5.6 降到 5.4，但信息不足。
  - 这些资料证明代理层确实可能因显式 mapping、fallback 或 bug 改变实际模型；它们不能证明本次客户的 Fable 5 请求被换模，更不能仅凭 greeting 或空回复识别替代模型。

## 2026-07-29 — Claude 可疑成功响应被动采集

- 用户已确认覆盖所有 Claude 成功响应，仅命中时采集，不重试；目标是保留定因证据。
- 当前 Request Dump 关键词 haystack 不包含 Claude 成功响应正文；Responses API 流事件追踪不能直接覆盖 `/v1/messages`。
- `ClaudeResponseInfo.ResponseText` 同时拼接 `delta.text` 与 `delta.thinking`，不能直接作为客户可见回复匹配源。
- 现有自动错误快照默认只由 relay error 或 stream incomplete 触发；需要新增不会参与错误/fallback outcome 的成功快照入口。
- 成功快照必须继续服从 `error_snapshot.enabled`。关闭被动快照时不得额外采集或保留响应。
- 三个采集挂载点已经明确：
  - 非流式：完整响应成功解析、完整性校验通过之后；
  - legacy 流式：scanner 返回且最终 usage 处理之前；
  - 完整性流式：收到合法 `message_stop`、正常向客户完成写入之后。
- 非流式 `HandleClaudeResponseData` 当前不会写入 `ClaudeResponseInfo.ResponseText`；匹配器应直接遍历全部 `content[type=text]`，不能复用 OpenAI 转换里最后文本块覆盖前一文本块的行为。
- 成功快照不能使用现有 `FinalizeErrorSnapshotRequest`，否则 `err == nil` 会把它改成 `fallback_succeeded`；应在创建时直接使用稳定 outcome，且不设置失败 attempt 的 context 标记。
- 当前最终上游请求只为重点用户/渠道预留。为满足“所有 Claude 请求命中后可定因”，需在自动快照开启时对 Claude 请求临时保留有界上游 JSON；未命中的副本只存在于单请求 context，结束即释放。
- UI outcome 映射目前只有失败/fallback/流不完整。新增 `suspicious_success` 后需要单独颜色和中文标签，并避免列表“错误”列把 HTTP 200 渲染成红色失败。
- `ErrorSnapshot` 可直接复用现有索引表，不需要新增数据库字段；分类可用既有 `error_type`、`error_code` 和 `final_outcome` 表达。
- 流式匹配只能在末尾判定，因此 raw SSE 必须在请求期间有界缓存；仅 `error_snapshot.enabled=true` 时启用，命中后写盘，正常请求直接随局部对象释放。
- 可见文本累加器只收 `content_block_delta.delta.text`；thinking、tool JSON、signature 不参与问候语匹配。
- 匹配器应要求短回复且同时出现 ready 语义与 “what would you like me to work on/do” 结构，全文匹配而不是任意 substring，降低包含引用或正常长回答的误报。
- 写流 helper 不提供已写 frame 的回读接口。诊断 trace 应在 handler 写出前保存 upstream/downstream data payload，并记录 relay format、event type 和序号；页面可据此比较转换前后，无需包裹全局 ResponseWriter。
- legacy handler 已有 `message_stop` 单测，可扩展完整合法文本流测试；完整性 handler 的测试夹具可直接验证命中后排队的快照 work，不必启动后台 worker或真实落盘。
- controller 仅根据 Claude handler 返回的 `NewAPIError` 决定 retry。成功快照函数不返回错误且不设置 `relayInfo.LastError`，因此不会进入 `shouldRetry` 或渠道失败记录。
- 非流式下游是一次 JSON body 写入；诊断可以把完整脱敏响应作为 `upstream_response` 片段保存，同时在 summary 中保存 content block 类型、模型、stop reason 和 usage。
- 现有 `preserveBodyHeadTail` 正好适合可疑响应证据：长请求保留前后文和原始 SHA-256，尾部可覆盖客户最终 user task；该行为只用于 diagnostic capture，不改变普通错误快照既有的大正文仅元数据策略。
- pass-through 且无 compat 的 Claude 请求不经过 `DumpUpstreamRequestIfNeeded`；需显式标记 upstream 与 client body 相同，快照构建时复用客户请求片段，避免额外读取/复制整个 body。
- 后端实现已通过聚焦测试：matcher 覆盖原始短语、弯引号和公开变体；长请求摘要保留最终 client/upstream task；嵌套 SSE JSON 字符串中的 secret 会脱敏；成功快照不会设置任何失败 context。
- Claude 侧测试确认 visible text 不含 thinking；native 非流式拼接全部 text block，OpenAI 非流式只使用实际下发的最后一个 text block；双向 trace 严格受事件数和 16 KiB/侧限制。
- UI 仅需新增三个语义标签和两处说明文案；现有 ErrorSnapshot API、筛选、下载和详情读取协议可直接承载 `response` 字段，不需要 controller 改动。
- Claude 兼容上游可能直接在 `content_block_start.content_block.text` 放首段文本；现有转发会下发它，诊断现已与客户可见语义一致地计入。
- OpenAI `/v1/chat/completions` 转 Claude 的 converted 与 compat-passthrough 路径也已加入最终上游 JSON 临时留存；AWS 等无同形 JSON 的 Claude 响应仍可保存客户请求、响应和 SSE。
- 生产启动路径在 `main.go` 始终调用 `StartErrorSnapshotManager()`；启用自动错误快照后，命中的成功快照队列有常驻 worker 消费。
- 流式诊断在构造时冻结 requested upstream model，避免后续 `message_start.model` 覆盖后丢失对比证据；快照同时保存 requested model 与 response-reported model。
- 最终验证：`go test ./... -count=1`、相关包聚焦 race、前端 `bun run build`、Prettier 和 `git diff --check` 全部通过；i18n lint 保持既有 441 条，无本次页面新增项。

## 2026-07-29 — Claude 客户令牌实流量复现与诊断快照验收

- 客户脚本不会连接业务库或任务队列；`burst` 通过 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN` 调用 `/v1/messages`。
- 探针要求模型只返回每请求唯一哨兵；问候回复代表任务正文疑似未生效，HTTP 200 且没有内容事件则归为空流。
- 6 个临时令牌属于敏感凭据，只能在运行时注入；实验记录只使用用户提供的别名。
- 需要分别验证“目标网关能否复现”和“本地修改能否捕获”。若生产目标尚未部署本次代码，本地 Docker 无法被动观察直接发往生产的请求，必须通过本地 new-api 转发或构造等价故障注入完成采集验收。
- `gwprobe` 默认样本路径是 `/tmp/gwprobe-sample.json`，当前不存在；当前 shell 也没有 `ANTHROPIC_BASE_URL`。
- 脚本的真正问候正则包括 `what would you like me to work on`、`i'm ready`、`don't see a task` 等，但最终 `default` 分支也会把任何非空、非哨兵、非工具回复计为 `GREET`。实验结论必须复核每条截断文本，不能把汇总中的全部 `GREET` 自动解释为同一种根因。
- 客户脚本只在 `content_block_delta.delta.text` 中累加正文，不读取 `content_block_start.content_block.text`；如果上游把首段文本放在 block start，探针可能把实际有文本的响应误判为空流。本次 new-api 采集代码已覆盖 block-start 文本，二者结果需要分别解释。
- 本机存在 Claude Code `2.1.220`，可以用客户脚本的本地固定-400 capture 代理生成真实请求样本而不产生上游调用。
- `new-api-dev` 已运行约 8 小时且健康；当前镜像早于本次可疑成功采集补丁，验收前必须重新 build/recreate。
- `2dev/scripts/README.md` 和现有探针命令均使用 `https://supertoken.cc` 作为 Anthropic 端点。这支持但不能绝对证明 6 个新令牌的目标地址；先用 1 条请求确认授权与模型可用性。
- 实测 Claude Code `2.1.220` 未进入客户脚本的固定-400 capture handler，而是报告 `401 authentication_failed` 并自动重试；本次运行费用和 token 均为 0。附件的 capture 子命令在当前 CLI 环境不可直接使用，但不影响 burst 对已有样本的重放能力。
- 三个既有 Claude 诊断目录只保存报告、假上游事件和复现代码，没有 `messages` 请求体；全仓库临时 JSON/JSONL 也未命中可重放的 Claude Messages 样本。
- 正确测试拓扑是 client probe -> 本地 `new-api-dev:3001` -> 6 个隔离 Claude 渠道 -> `https://hk.supertoken.cc`。这样本地代码才能记录本次快照；直接调用远端端点无法验证未部署的本地修改。
- 本地数据库已有大量互不相关渠道，因此本轮必须使用独立 group，而不是把新渠道放入 `default` 随机路由。
- `claude.response_integrity_fallback_enabled=true` 已在本地启用；可疑成功快照开关尚未从简单 option 名称查询中出现，需要定位聚合设置键。
- `options` 中没有任何 `error_snapshot.*` 行，等价于保持默认关闭；只有重建包含新代码的容器并启用该配置后，命中才会落盘。
- Claude 原生渠道类型为 `14`；channels 表支持独立 base URL、group 和精确模型列表，无需修改业务代码即可创建六个隔离实验渠道。
- `middleware/auth.go` 在管理员 token 被解析成多个部分时把第二部分写入 `specific_channel_id`；普通用户会被明确拒绝。这提供了比独立 group 更直接的一对一渠道选择能力。
- token group 若走普通路由必须同时满足用户可用组与 `GroupRatio`/聚合组校验；因此不应仅靠数据库插入任意 token group。
- 指定渠道路径会校验渠道存在且启用，但不会走普通 abilities/group 随机选择；`shouldRetry` 也对存在 `specific_channel_id` 的请求直接返回 false。因此它既能固定渠道，也满足“不重试”的实验约束。
- `error_snapshot` 默认 TTL 60 分钟、256 MiB、1000 文件；本轮可保留这些上限，仅开启 enabled 并把六个测试渠道列为 priority，确保保存完整上游请求证据。
- 实验渠道 ID 固定为：dra=115、vll=116、doubv5=117、maidoucoding=118、9527=119、yimo=120；模型均为 `claude-fable-5`，base URL 均为 `https://hk.supertoken.cc`，auto-ban 关闭。
- 本地快照 TTL 临时设为 120 分钟，priority user 为管理员 ID 1，priority channels 为 115–120；这只影响本地 dev。
- 首条端到端基线证明 `sk-<local-key>-115` 能固定渠道且不会被其他本地渠道分流；HK 上游接受 `claude-fable-5` 和 synthetic Claude Code 形态请求。
- 基线移除中途 system role 后正确返回 `SENTINEL_*`，说明最小 Claude Code 形态、2 个工具和约 2094 input tokens 本身不会触发客户现象。
- 六渠道低成本基线结果为 3/6 正常、3/6 上游流不完整：vll 提前出现非法序列，doubv5 与 9527 在首内容块超时。由于本地启用了 response integrity，这三条返回明确 502；在旧 handler 下，它们可能表现为 HTTP 200 + 本地 ping/空 EOF。
- 客户截图中的 12 并发实验：9 条 4.5–7.3 秒正常哨兵，2 条约 22 秒 HTTP 200 ping-only 且无 usage，1 条 58.9 秒返回精确问候；问候 usage 为 input=2715、cache_creation=27483、cache_read=0，正常样本 cache_creation 约 27930。总输入量约 30k，而当前本地基线只有 2094 input tokens。
- 客户声称生产 39/500（约 8%）问候与网关 1/12（约 8.3%）吻合；样本量下只能称“点估计接近”，不能据此证明同一概率或同一根因。
- 第二张截图列出两个具体请求在网关侧 `stream=eof/ok`、output 224/219，且声称无 scanner error/客户端断开；最近 24 小时 supertoken channel 共 1836 请求、83 条零输出、其中 35 条正常 EOF 但零输出。该统计支持异常集中在上游链路，但“问候不是 AI 生成”这一表述逻辑不成立：new-api 不生成文本，只能说明文本来自 supertoken 后续模型/代理链，而非 new-api 本地兜底。
- 客户的 ping-only 与本地完整性 502 在症状上高度对应；`I'm ready` 是结构完整的成功 SSE，必须由可疑成功快照而非完整性失败快照捕获，两者需要分开取证。
- 当前三份错误快照大小约 1.6–1.8 KiB compressed、11.7–12.2 KiB original，均保存 request ID、channel、retry=0、完整 5051-byte client/upstream JSON 和错误/timing，证明现有错误快照基本链路工作。
- 但三份 payload 顶层均无 `stream`：vll 仅保存 new-api 合成的 502 JSON，doubv5/9527 连 upstream_response 也为空。说明 response-integrity 失败尚未把已观察 SSE/ping trace 附加到 `NewAPIError.Diagnostic`，这是分析 ping-only 根因的实际可观测性缺口。
- 192,208 system characters 在 dra/115 的实际 Anthropic usage 中产生 input=1259、cache_creation=47685、cache_read=0；客户问候样本为 2715+27483，因此并发复现样本应按约 0.58 比例缩减，不能仅按字符/token 通用估算。
- 111,408 system characters 实测产生 cache_creation=27953，和客户正常样本约 27930 的差值仅 23 tokens；该样本足以用于对齐输入规模的并发实验。
- 当前 native `/v1/messages` 校验会拒绝 `messages[].role=system`，即使带 `claude-code` beta；客户实验若成功到达上游，说明其实际 burst 可能使用了 `keep-system=false`，或走不同入口/版本。不能把 role:system 直接认定为问候根因。
- 管理员本地用户的 `UserGroup-admin` group ratio=99，使约 30k 输入单条预扣费约 $25；这只是本地计费配置，不代表上游实际成本或客户环境。
- `local-adobe2api` 是当前 `UserUsableGroups` 明确允许且 ratio=1 的隔离本地计费组；指定渠道仍固定为 115–120，不会真的路由到 Adobe 渠道。
- dra/115 在相同约 29k input 形态下既有 4.807 秒正确哨兵，也有 30.064 秒首块超时，证明空流不是固定令牌失效或固定请求格式错误，而是间歇性上游/代理行为。
- dra/115 在 12 并发冷缓存大输入下 100% 首块超时，显著高于客户 2/12 ping-only；这说明该 HK/令牌组合的并发容量或上游状态更差，不能直接把 100% 当作生产故障率。
- 响应完整性 first-block timeout=30s 与客户 58.9s 问候存在观察偏差：启用保护后会在问候到达前返回 502。要验收 `suspicious_success` 必须临时延长本地 timeout，但生产取值仍需在“快速失败”和“保留晚到诊断”之间权衡。
- 延长到 90 秒后 dra/115 仍没有任何请求出现首内容块，说明本轮不是单纯慢到 30–60 秒，而是至少 90 秒的上游无首块/排队挂起。`I'm ready` 与这种空流仍可能共享容量触发条件，但不是同一个响应形态。
- maidoucoding/118 在相同并发下不是空流，而是上游明确返回 400/502；client 统一看到 500 “Service temporarily unavailable”，说明还有一处状态/错误正文抽象，需通过快照读取真实上游错误。
- yimo/120 证明相同 local new-api、HK 域名、模型、请求形态和并发可以 12/12 正常；因此 dra/maidoucoding 的失败不能归因于本地通用 Claude 转换或探针哨兵逻辑，更可能与令牌后端路由、账号池或上游容量有关。
- yimo 的 38.7–62.9 秒首结果显著慢于客户正常 4.5–7.3 秒，但跨过客户 58.9 秒问候窗口仍能正确执行最终 user 指令；延迟本身不是问候的充分条件。
- maidoucoding 快照中的 400/502 `upstream_response.body` 已经是上游代理生成的二次错误 JSON，不含原始 provider body；多个嵌套 request ID 表明 `hk.supertoken.cc` 后面还有至少一层请求转发/错误包装。
- 目前自动快照已覆盖所有 40 条失败，但没有任何 `suspicious_success`，因为尚未真实命中问候；成功哨兵按设计不落盘。
- yimo/120 两轮 24/24 正常；按客户点估计 8% 独立概率，24 条零命中的概率约 13.5%，因此未命中并不反证客户现象，但不宜继续无界真实采样。
- `ClaudeIntegrityStreamHandler` 已创建 `ClaudeResponseInfo.Diagnostics`，并在事件处理函数写 downstream trace；修正方向应复用该有界结构，不新增第二套 SSE 缓冲。
- 已确认缺失 `stream` 的直接原因：diagnostics 已采集事件，但首块前错误没有写入 `NewAPIError.Diagnostic.StreamSummary`，已提交后的 `CaptureStreamErrorSnapshot` 也只收到计数摘要。
- 修复后每个方向仍严格限制为 256 events、16 KiB，额外保存单事件原始字节数、SHA-256 和截断标记；不会形成无界响应复制。
- ping-only 故障注入证明旧式“HTTP 200 只有 ping”可在当前保护下稳定转为 commit 前的 502，快照精确记录唯一上游 ping、0 个下游事件、retry=0、channel/request/timing 和完整请求证据。
- 完整 SSE 的 `I'm ready. What would you like me to work on?` 故障注入证明 matcher 不会干预响应：客户仍收到原始 HTTP 200；旁路快照标为 `suspicious_success/claude_idle_greeting`，包含模型、usage、stop reason、客户/上游请求和上下游 SSE。
- “问候不是 AI 生成”仍不是可成立结论。已确认的边界是 new-api 没有该固定文本生成逻辑，文本来自 supertoken 后续模型或代理链；仅凭问候和 model 回显无法证明 Fable 5 被换模。
- 客户实流量与本地受控结果共同支持“令牌后端路由/账号池/上游容量差异”优先级高于“new-api 通用转换错误”：同一代码、域名、模型和请求下，yimo 24/24 正常，而 dra、vll、doubv5、9527 和 maidoucoding 呈现不同失败形态。
- 当前证据不足以断言 ping-only 和问候是同一根因。前者是 SSE 完整性失败；后者是结构完整但语义异常的成功响应，只能通过两类独立快照在生产中继续关联 channel、request、usage、延迟和原始 SSE。

## 2026-07-30 — AdobeVideo 异步参考图片

- Adobe2API `/v1/chat/completions` 已支持 Seedance `frame` 与 `media` 两种参考语义，能够加载 HTTP(S)/Data URL 图片并复用现有 Adobe UGS 上传和 `referenceBlobs` 构造。
- 当前异步 `VideoGenerateRequest` 没有参考字段，worker 固定传 `source_media=[]` 与 `reference_mode="frame"`，因此底层能力没有暴露到 `/v1/videos`。
- new-api 统一视频 DTO 已有 `input.image` 与 `reference_images`，无需修改公共请求结构。
- 当前 AdobeVideo adaptor 明确拒绝任何图片输入；应在 adaptor 内完成精确 provider 映射，而不是放宽共享 controller。
- 首版只桥接图片，避免把 Adobe2API 的视频/音频 Media 能力提前扩大为新的统一公共契约。
- 主图与参考图按 `input.image`、`reference_images...` 的稳定顺序发送；默认 `frame`，`media` 必须显式声明。
- 异步参考素材应在 worker 内加载，URL/MIME/大小错误进入异步失败与既有退款路径，而不是让任务提交阻塞等待远端下载。
- Adobe2API 异步 DTO 现支持 `reference_mode` 与 `{url,name?}` 参考图；提交阶段只校验模式、数量和 URL scheme，worker 复用 `_load_seedance_media` 下载并校验 MIME/大小。
- worker 加载失败会写入 `failed / invalid_reference_media`，不会占用 Adobe 账号或调用 `generate_video`；上游生成错误继续保留原有错误类型。
- new-api AdobeVideo adaptor 仅放开图片：主图先于 `reference_images`，拒绝视频输入和 `provider + file_id`，并禁止 provider options 覆盖公共图片字段。
- 参考图不会改变 `ResolveVideoBilling` 的有效秒数。Docker 验收的 4 秒请求保持 `$0.03/秒 * 4 * 分组倍率 1 = 60000` 额度，资金来源为 wallet。
- Docker mock 收到 `seedance_2.0_fast_480p`、`duration=4`、`reference_mode=media` 和两张顺序正确的图片；后台 3 次轮询后成功，Range 内容代理返回 206。
- 异步失败 mock 在 1 次轮询后进入 `FAILURE`，预扣的 60000 额度按现有终态流程退款。
- 所有临时渠道、令牌、VideoPricing Option、任务和日志均已删除；管理员 quota 与 used_quota 恢复到验收前数值。

## 2026-07-30 — Seedance URL-only 多媒体异步链路

- 现有 image-handle 已提供 `/v1/image/uploads` 和 `/v1/image/uploads/base64`，会写入 R2 并返回临时公开 URL。
- 现有 image-handle multipart 路径使用 `part.toBuffer()`，new-api 图片上传代理使用 `storage.Bytes()`；两层都会完整缓冲文件，不能原样扩展为视频/音频入口。
- 选定 R2 预签名 PUT：new-api 仅承担鉴权和上传会话元数据，文件直接由客户端传到 R2。
- new-api `/v1/video/tasks` 最终只接收 HTTP(S) 参考 URL；旧计划中的 multipart/Base64/Data URL 仅保留给 Adobe2API、Higgsfield2API 的直连接口兼容。
- Adobe2API 当前提交已包含异步 Seedance media references；需要审计后复用，避免重复改写。
- Higgsfield2API 当前仓库只有初始实现，需要依据已确认的 `/media/batch`、预签名 PUT、upload confirm 与 `params.medias` 角色契约扩展。
- Adobe2API `995b6fd` 已扩展异步 `reference_videos`、`reference_audios`，并在提交层校验 frame/media、9/3/3 和总计 12；不应重复实现这部分。
- Adobe2API 当前仍缺统一 multipart 入口、Pydantic `extra="forbid"` 和基于 ffprobe 的参考视频/音频 15 秒真实时长校验。
- Higgsfield2API 公共 schema 当前把 `reference_mode` 固定为 `frame`、参考图最多 2；submission 服务只调用图片专用 `upload_reference_image`。
- Higgsfield2API upstream 已存在 `/media/batch`、预签名 PUT、upload confirm 和 `params.medias` 构造，可泛化而无需重写任务生命周期。
- new-api 当前 AdobeVideo adaptor 只桥接图片参考，统一 DTO 尚无 `ReferenceMode`、`ReferenceVideos`、`ReferenceAudios`；当前没有 HiggsfieldVideo 渠道。
- Higgsfield 公共 `VideoReferenceImage.url` 当前允许约 28 MB 字符串，说明 Data URL 会直接进入 Pydantic 对象；URL-only 合并后应把统一入口收紧到普通 HTTP(S) URL。
- Higgsfield 的 BaseModel 当前未统一设置 `extra="forbid"`，未知的 `reference_videos`/`reference_audios` 会被静默丢弃。
- 2026-07-30 恢复实施审计确认：Higgsfield 已有 `/media/batch`、预签名 PUT、`/media/{id}/upload` 和 `params.medias` 构造，缺口集中在公共 schema、通用媒体准备、批量上传编排和 account retry 重传，不需要重写异步任务生命周期。
- Higgsfield 当前 `PublicVideoCreateRequest` 仍把 `reference_mode` 固定为 `frame`，仅允许两张 `VideoReferenceImage`；`ReferenceImageService` 仍以内存 `bytes` 保存图片，未实现视频/音频、ffprobe 和任务级临时文件。
- Adobe2API 当前测试已覆盖 `reference_videos`、`reference_audios` 和 media 计数，但 Docker 镜像只安装 `tzdata`，尚无 ffmpeg/ffprobe runtime。
- Adobe2API Docker 目前仅安装 `tzdata`，没有 ffmpeg/ffprobe；15 秒真实探测需要补充固定 Debian runtime 包。
- image-handle 已依赖 S3 client 与 Redis，但未依赖 `@aws-sdk/s3-request-presigner`；预签名实现需要新增该依赖并复用现有 Redis connection。
- Adobe2API 与 Higgsfield2API 的边界测试均覆盖 14.999、15.000 和 15.001 秒；实现使用真实 ffprobe duration，前两者接受，15.001 拒绝。
- Higgsfield2API 的 `params_json` 仅保存素材 kind/source/name/MIME/size/SHA-256/duration，不保存 bytes、Base64、临时路径或预签名 URL；账号切换会在新 workspace 重新上传。
- Higgsfield 公共模型 preset 会覆盖外部 `resolution`，new-api adaptor 又拒绝 `output.resolution` 并只允许 480p/720p provider SKU，因此 480p 定价模型无法通过请求参数提升到更高分辨率。
- new-api 公开视频失败原先统一投影为 `video_task_failed`；本次只允许 `invalid_reference_media_duration` 与 `reference_media_duration_exceeded` 从已脱敏的 provider task JSON 穿透，其他未知上游码继续隐藏。
- Resource Center locale 自动提取会把存量缺失键一并同步，产生大量无关空翻译；最终 locale 仅保留 HEAD、既有 Claude 诊断键和本次媒体文档键。
- 首次 Docker 综合验收已生成 Adobe Mock 成功任务 `task_wzLkRwoB7Y7JJPJTf3moALyEBO03kzVg`，返回 4 秒 MP4 Asset；消费日志记录钱包扣费 60000，快照为 `$0.03/秒 × 4 秒 × 分组倍率 1`。
- 该任务的 `video_task_requests.request_json` 长度为 590，三类参考 URL 均被替换为 `sha256:<digest>`，不含 Base64 或文件内容。
- Mock `/metrics` 证明 Adobe 上游收到 `seedance_2.0_fast_480p`、`media`、4 秒及 1 图/1 视频/1 音频；素材名称分别为 character/motion/music。
- Adobe 终态审计记录 `reported_duration_ms=4000`、`requested_seconds=4`、`matches_request=true`；成功 Webhook 一次投递即返回 204。
- 首次验收退出前 Mock content 计数仍为 0；结合 `/control` 只返回配置、`/metrics` 才返回请求指标，失败原因是验收脚本读取了错误的 Mock endpoint，不是产品链路失败。
- Adobe 公开 Asset `asset_yMf6bCRW5QcEZTtxCcFOrkVWWveRgE0R` 使用 `mvqa-resource` 执行 4 字节 Range 下载返回 HTTP 206、`video/mp4` 和正文 `mock`。
- HiggsfieldVideo Docker 联动任务 `task_nbtuhrLJmzp3KjK56Frp23pPzYDisQWe` 成功；上游精确模型为 `seedance-2.0-480p`，4 秒和三类 media 数量/名称均正确，Asset Range 返回 206。
- Higgsfield 成功后用户余额为 880000、累计 used_quota 为 120000，证明两个 4 秒任务分别按 60000 额度结算。
- Higgsfield 失败任务 `task_1z49broMpbvFAnaUsGD5aLRGrfxX93Ps` 先预扣 60000，终态失败后可用钱包从 820000 恢复到 880000，并生成 quota=60000 的退款日志。
- `users.used_quota` 在退款后仍为 180000，这是现有累计消费审计口径，不代表钱包未退款；净钱包支出应由初始与当前 `quota` 差额计算，仍为 120000。
- 同一失败任务重复查询三次后退款日志仍只有 1 条、退款额度仍为 60000，证明退款幂等。
- 三个 new-api 任务请求快照均不含 HTTP(S) URL、Base64 或 Data URL；三个任务均持久化 `video_pricing` 与 `subscription_enabled=false` 快照。
- 两个成功事件和一个失败事件均一次投递成功，HTTP 状态均为 204。
- Docker ffprobe 确认测试 WAV 为 15.001000 秒；Adobe 任务在提交真实上游前异步失败，Higgsfield 同步返回 400，错误码均为 `reference_media_duration_exceeded`。

## 2026-07-30 — 四服务 main 分支发布

- 四个仓库本地 main 与 fetch 后的 origin/main 完全一致。
- new-api 的业务候选文件与多媒体实现一致；Claude 响应诊断、RequestDump、临时文件和规划日志必须留在工作树。
- locale 的完整 JSON diff 会额外带入 6 个 Claude 诊断键及既有重复键规范化；必须仅暂存 13 个媒体文档键。

## 2026-07-30 — Adobe Fast 480p 真实多媒体联调

- supertokendoc 要求先通过 `/v1/media/uploads` 创建上传会话、直接 PUT 对象存储、再调用 `/v1/media/uploads/complete`，视频任务本身只引用完成响应中的 URL。
- 六个测试素材已上传：3 张 PNG、2 个约 4.04 秒 MP4、1 个约 4.09 秒 WAV。
- Docker dev 的完成响应把对象 URL 主机写成 `localhost:9000`；Adobe2API 容器无法通过该主机访问 MinIO。同一路径改为 `minio:9000` 后六个对象均返回 200 和正确 MIME。
- `/v1/models` 已公开五个 Adobe Seedance SKU，其中包括 `adobe-seedance-2.0-fast-480p`。
- 渠道 124 当前 Key 和 Adobe2API 实际服务 Key 不匹配；前者访问 `/v1/models` 为 401，配置文件中的 Key 为 200。Adobe2API 的 `config/config.json` 会覆盖容器 `ADOBE_API_KEY` 环境变量。
- frame 任务 `task_nor0WTAJF4qZGz39chA1POO27z46FO6R` 返回 202，经历 queued/in_progress 后 failed；Adobe 上游错误为 `Unauthorized to perform request.`。
- frame 的不可变计费快照为 `unit_price=1`、`seconds=4`、`group_ratio=1`、`final_quota=2000000`。代码常量 `QuotaPerUnit=500000`，因此额度换算正确。
- frame 失败后钱包恢复到提交前余额；消费日志和退款日志各一条、额度均为 2000000。`used_quota` 仍增加是现有累计审计口径。
- media 任务 `task_3LYiSCIXOFX9khx8QNQJDK3GAYrKoNmR` 返回 202，经历 queued/in_progress 后同样因 Adobe `Unauthorized to perform request.` 失败；钱包净变化为 0。
- media 请求快照记录 `reference_mode=media`、3 图、2 视频、1 音频；不含 HTTP URL、Base64 或 Data URL。frame 快照记录主图存在并有 1 个追加参考图。
- Adobe 管理端账号刷新返回 200，刷新结果包含新的有效期和 credit 状态；刷新后 frame 任务 `task_vwznBeIVW6z5O0RK7CMZXV0CSrIG70XG` 仍得到相同错误。
- 三个真实任务各有一条 2000000 消费日志和一条 2000000 退款日志；最终钱包回到测试前的 73802740。生成 Asset、Range 下载和输出时长无法验证，因为上游未生成视频。
- 已确认事实是 Adobe 上游授权失败；“账号缺少 Seedance entitlement 或 Firefly 媒体/生成端点权限”是合理推测，当前响应没有提供更细的权限码，无法进一步确认。

## 2026-07-30 — Adobe 新账号 10000 积分复测

- Adobe2API 当前账号池只有 1 个 active 账号，积分为 `10000/10000`、used=0，已绑定 auto refresh；这是用户新导入的账号。
- new-api、Adobe2API、PostgreSQL 和 MinIO 均健康。
- 六个参考素材从 Adobe2API 容器访问均为 HTTP 200，MIME 为 3 个 image/png、2 个 video/mp4、1 个 audio/wav。
- frame 任务 `task_KXqYhgQOg5CT3E9nByCC1kE4kkGHVTZS` 成功，公开 Asset 为 `asset_ydbAMwlMf4Qp2sDu5cSzL9dndN58EFS2`。
- frame Asset Range 返回 206、`Content-Range: bytes 0-3/1441463`、4 字节；完整文件 1441463 字节，ffprobe 为 H.264、864x496、4.042 秒。
- frame 固定按请求 4 秒结算：`$1/秒 x 4 x group 1 x 500000 = 2000000`，资金来源 wallet；请求快照不含 HTTP URL、Base64 或 Data URL。
- media 任务 `task_NsDG9fOflSKh2e3DfvY7uckpdtJnpp3q` 成功，Asset 为 `asset_BMNce7IEmJVWqWOwXmp4Ix03tXCCANG9`。
- media Asset Range 返回 206、`Content-Range: bytes 0-3/894202`；完整文件 894202 字节，ffprobe 为 H.264 864x496、AAC 44.1kHz 双声道、4.087 秒。
- media 快照精确记录 3 图、2 视频、1 音频，仍只按请求 4 秒、group ratio 1、wallet 结算 2000000；不含原始 URL 或 inline data。
- 两个成功任务共有 2 条消费日志、总额度 4000000、没有退款日志；管理员钱包从 73802740 降到 69802740，净变化完全一致。

## 2026-07-30 — VideoPricing 编辑与删除缺陷

- 页面已经引用 `deleteVideoPricingProfile`，helper 也能修改 profile 和 binding，初步判断不是后端 Option 缺少删除协议，而是管理页交互或保存状态处理不完整。
- 截图中的绑定表提供价格模板下拉、订阅开关和仅图标解绑操作，但缺少明确的编辑/删除反馈；需要检查事件是否调用统一持久化函数。
- 采用保守删除语义：仍被模型绑定的模板禁止删除；先改绑或解绑后才能删除，避免静默改变线上计费。
- `VideoPricingSettings` 的 `updateProfile`、`updateBindingProfile`、订阅开关和 `removeBinding` 都只调用 `setConfig`，没有独立请求 `/api/option/`；只有页面顶部全局保存会持久化整个配置。
- 当前删除模板 helper 会级联把允许订阅的绑定转为 policy-only，并删除 wallet-only 绑定；这与保守删除语义冲突，需要改成由页面阻止仍被引用模板的删除。
- 应用内浏览器的新会话未登录本地 new-api，无法在不获取管理员凭据的情况下复现受保护设置页；先通过组件/helper 测试实现，Docker 重建后再使用已有认证浏览器或 API 验收。
- 图片计价页使用相同的“本地草稿 + 顶部全局保存”模式，不能直接作为本次单项 CRUD 的正确范例。
- `RatioSetting` 的 `refresh()` 会重新加载整个 Option 列表，因此单项操作成功后可以通过统一 PUT + refresh 保持父子状态一致，无需新增后端接口。
- 前端没有现成的 VideoPricing 组件测试环境；应把绑定改绑、订阅策略、解绑和被引用模板删除保护沉到纯 helper，并用 Bun 定向测试覆盖，UI 只负责调用统一持久化函数。
- 用户补充 `adobe-veo-3.1-480p` 在模型广场落入未分类，而 Kling 已正常分类；需要检查模型系列规则是否只覆盖 `veo-*` 或只覆盖现有 720p/1080p SKU。
- 已确认模型广场不使用前端 `getModelCategories` 决定供应商，而使用 `/api/pricing` 中的 `vendor_id`；该值来自 `model/pricing_default.go` 的默认供应商规则。
- `defaultVendorRules` 已包含 `kling -> 快手`，但缺少 `veo -> Google` 和 `seedance -> 即梦`，因此没有显式模型元数据的 Self-Adobe Veo/Seedance 会得到 `vendor_id=0` 并落入未分类。
- 本地 `models` 表没有这些 Adobe 包装模型的显式行，证明当前显示确实依赖默认规则；Google vendor 已存在，Veo 修复不会创建重复供应商。
- 修复后的 `/api/pricing` 和桌面模型广场均确认：`adobe-veo-*` 归 Google，`adobe-seedance-*` 归即梦；渠道显示名 Self-Adobe 不参与供应商分类。
- 默认供应商规则必须有稳定顺序。旧 map 遍历无序，类似 `adobe-kling-o3-*` 的模型可能被通用 `o3 -> OpenAI` 抢先匹配；改为有序规则后先匹配视频系列。
- VideoPricing 单项操作应在服务端保存成功后再更新本地配置。Docker 桌面实测证明模板和绑定的修改、删除、解绑均能跨刷新持久化，且临时验收数据已清理。
# Adobe Multi-model and Console Upgrade Findings (2026-07-30)

- new-api already has an AdobeVideo adaptor, normalized video tasks, VideoPricing, public assets, and uncommitted model visibility/Self provider fixes that must be extended rather than replaced.
- Adobe2API currently uses static HTML/CSS/JS, an in-memory video job store, and one configuration API key; account scheduling, balance refresh, health checks, concurrency, and import/export APIs already exist.
- The approved UI direction is a dense operations console with a dark persistent sidebar, light work area, data tables, status colors, Lucide icons, visible focus states, and responsive drawer navigation.
- New Adobe result URLs must be persisted privately with signatures intact, but every normal log surface must redact query strings.
- Adobe2API already contains legacy duration-specific Kling 3.0, Kling O3, Veo 3.1 Standard/Fast catalog entries and payload branches. The new stable SKUs should reuse these upstream shapes while replacing scattered engine conditionals with explicit capabilities.
- Both Seedance and generic video completion branches currently extract `presignedUrl` and then download it. Direct-URL support must change both branches and keep the old content route only for already persisted local paths.
- Higgsfield2API already provides the target React/Vite navigation and reusable operational interaction patterns. Adobe can reuse the layout/component structure, but its pages must call Adobe's existing account/config/log APIs plus new task/key/capability APIs.
- new-api already projects cross-origin HTTPS video Asset URLs as `url_auth=none`; Adobe currently misses that path because its adaptor always persists an internal provider reference and resolver. New completions should persist `VideoOutput.URL`, with resolver fallback only for legacy responses lacking `video_url`.
- The gateway DTO already carries reference videos and audios. Adding `images` is a validation/capability change, not a storage or database change.
- Backend model fetching already has generic `/v1/models` support after provider-specific branches. The visible limitation comes from two frontend fetchable-type sets and the absence of a backend capability/reason response.
- A real Adobe Kling request proves the published plan's `1-15s` assumption is false for the current upstream: Adobe validation requires a minimum of 3 seconds. Both gateway and provider catalogs now use `3-15s`.
- A real Kling 3.0 `3s frame` request submitted successfully and persisted one upstream job ID, but Adobe's downstream `fal-ai-video` timed out with HTTP 408. new-api refunded the full 1,500,000 quota and Adobe2API created no local media file.
- A real Kling Omni `images` request proves the image role must be `style`, not `asset`; the upstream error explicitly allows `frame` and `style`. The adapter mapping and contract test were corrected before retrying.
- The corrected Kling Omni request passed Adobe validation and received an upstream job ID, but the same `fal-ai-video` service later returned HTTP 408. This separates the corrected payload contract from the provider execution outage.
- Veo Standard `8s/16:9 images` and Veo Fast `4s frame` both completed successfully. Their Assets contain the original signed Adobe S3 URL, `temporary=true`, and `url_auth=none`.
- The two successful Veo tasks charged exactly 6,000,000 quota at the local `$1/second` test price. All three failed attempts refunded 4,500,000 quota; the wallet moved from 69,802,740 to 63,802,740 with no subscription use.
- Adobe2API persisted two full signed result URLs, while SQLite request logs and both service logs contain zero signed query parameters. The local generated-file count stayed at 33 with zero files created by these tasks.
- The Adobe task detail implementation already renders completed direct results through a `<video>` whose `src` is the persisted Adobe URL and provides separate open and download actions using that same URL; the compact task-list cell intentionally exposes only one Adobe URL entry.
- The final Veo Standard `images` capability is also reflected in the console form: only 8 seconds and 16:9 are selectable in that mode, preventing the UI from advertising combinations rejected by the shared catalog.
- The UI/UX validation classifies this as a data-dense operational dashboard; the existing dark sidebar, light work area, compact tables, Lucide actions, status badges, and direct task controls fit that use case. No decorative redesign is needed for final acceptance.
- Mobile compatibility is no longer an acceptance requirement for this task; desktop behavior remains required.
- Desktop browser verification of the live Adobe task list shows the two successful Veo rows expose cross-origin Adobe S3 URLs directly, while failed Kling rows expose no fabricated result link. The list also renders the persisted model, duration, aspect ratio, reference mode, account, status, progress, and upstream error state.
- The active browser tab inherited a narrow viewport emulation but no longer retained the previous session's viewport capability handle. The tab itself exposes a capability registry, so desktop restoration must use that supported capability rather than coordinate workarounds.
- After restoring 1440x1000 desktop metrics, the live Veo Fast task detail opens correctly and shows completed status, exact 4s/16:9/720p/frame parameters, account/upstream identifiers, the redacted reference summary, a temporary-resource label, and distinct open/download links that resolve to the same signed Adobe CDN URL.
- Runtime DOM inspection confirms the task detail contains one video and exactly two actions (`打开`, `下载`); all three use the identical signed HTTPS Adobe S3 URL, none uses `/content`, and the video is fully loaded (`readyState=4`) with no media error.
- Desktop geometry is 1440x1000 with document width exactly 1440 and no horizontal overflow. The task dialog stays within the viewport at 980px wide; its long metadata/media content is vertically scrollable, while the video preserves a stable 942x530 content box.
- Final regression confirms `go test ./... -count=1`, the Adobe React production build, and the new-api production build pass. new-api i18n lint still reports the established 420-item repository baseline and does not introduce a new finding in the modified video/model-discovery UI files.
- Adobe2API's complete suite passes all 100 tests in a disposable container built from the production runtime image, with the repository mounted read-only and `/src/data` isolated on tmpfs.
- Rebuilding and recreating only the Adobe Compose service preserves the authenticated console session and all SQLite task rows. After reload, both successful Veo tasks still expose their direct Adobe URLs; the reopened detail again has matching preview/open/download URLs and no `/content` fallback.
- The post-rebuild preview loads its signed Adobe media successfully after metadata fetch: ready state reaches 4, duration is 4.01 seconds, and no media error is present.
- The live Generate Test page lists all eight Kling/Veo provider SKUs alongside preserved Seedance models. Capability-driven controls show Kling Omni `frame|images` with 3-15s and both aspect ratios, Veo Standard `frame|images` with images locked to 8s/16:9/three images, and Veo Fast frame-only with 4/6/8s.
- Explicitly selecting Kling Omni `images` changes the reference control to a three-image maximum while preserving 3-15s and both aspect ratios.
- Final workspace checks pass in both repositories (`git diff --check`). The rebuilt Adobe and existing new-api containers are healthy on the shared `ai-gateway` network, and `data/generated` remains at 33 files, confirming the direct-URL acceptance work did not persist new media files locally.
- A new end-to-end request through new-api proves Adobe Kling 3.0 frame requires at least one frame image. The zero-image request passed current gateway validation, was precharged, and then failed at Adobe submit with a 400; this is a capability-contract gap rather than an upstream execution outage.
- The concurrent corrected Kling Omni images request passed new-api validation and reached `in_progress`, confirming its three-image `usage=style` mapping remains accepted.
- Both capability implementations currently store only maximum reference-image counts. Neither exposes a per-mode minimum, which explains why zero-image Kling frame reached precharge and upstream submission.
- The corrected Omni new-api task `task_v0x6857Vgl5ZuUzEN9ud4ec7jJiCJnMc` succeeded. Its exact 3-second charge remains, while the concurrent zero-image Kling frame charge was refunded; the wallet net change is exactly 1,500,000 quota.
- The final new-api image containing the minimum-reference validation rebuilt as `sha256:2b9148e6274d...`; both `new-api-dev` and `adobe2api` report healthy. Adobe2API does not expose `/health` even though its configured Docker healthcheck passes, so that route's 404 is not a service failure.
- The live token schema uses `model_limits_enabled` and `model_limits`; no legacy `models_enabled` column exists. The admin wallet baseline before final Kling retries is `62,302,740` quota.
- Local acceptance token ID 150 belongs to admin, uses aggregate group `video-generation`, and has model limits disabled. Its token-local quota is not the billing balance; the authoritative wallet baseline remains the user quota.
- The rebuilt gateway rejects a zero-image `adobe-kling-3.0-720p` frame request synchronously with HTTP 400 and `reference_image_required`. Admin quota, matching task count, and matching log count remain exactly `62,302,740`, 6, and 11; Adobe2API received no submit request.
- The corrected one-image Kling request was accepted through new-api as `task_zasvWwokheIl0fCMrWX56o20PQcXhJ5G`, progressed from queued to in-progress, and succeeded. Its public result contains one Adobe-backed URL with `temporary=true` and `url_auth=none`.
- Final billing moved the admin wallet from `62,302,740` to `60,802,740`, exactly one 1,500,000-quota three-second charge. The task and Asset expose the same signed Adobe HTTPS URL, the Asset uses `url_auth=none`, and Adobe's local generated-file count remains 33.
- Post-fix regression passes the complete new-api Go suite and all 101 Adobe2API tests.

# Video Contract Alignment Findings (2026-07-31)

# Async Task Error Boundary Findings (2026-08-03)

- Adobe2API already retries Adobe polling 408/429/451/5xx and network errors, preserves poll state, and terminates only after the configurable continuous-unavailability budget.
- Adobe2API submit retries are bounded separately and never resubmit `submission_unknown` tasks.
- new-api's AdobeVideo adaptor does not map the valid `submitting` status, so parsing returns an empty status.
- `updateVideoSingleTask` does not branch on HTTP status before parsing. An empty parsed status falls into `upstream returned unrecognized message`, which is applied as a terminal failure and can refund a task that Adobe2API later completes.
- Terminal task transitions persist `FailReason` and atomically create a public task Webhook event. Task query and Webhook payloads share the same public task projection.
- A real local historical failure proves current task queries and Webhooks expose provider implementation details verbatim and mark a polling 408 as non-retryable.
- Querying `/v1/video/tasks/{id}` requires the account resource key (`ak_...`), while task creation uses the ordinary token (`sk-...`).
- `tasks.data` is the existing internal diagnostic payload and is excluded from normalized public task objects except for explicit safe-field projection. This avoids a schema migration.
- The current caller mutates `task.Data` before `ApplyTaskResult` takes its snapshot, so same-status polls can skip persisting a changed response. Passing the redacted body through `TaskInfo.Data` lets the state transition layer persist it atomically.
- `updateVideoSingleTask` can preserve a transient response without inventing a new status by applying the task's current non-terminal status and current progress together with the diagnostic data.
- Video public errors currently expose only code/message/retryable. Adding optional `upstream_status`, `upstream_error_code`, and `request_id` is backward-compatible JSON.
- Image public errors already normalize callback details to code/message/retryable; provider-specific callback fields remain internal in `tasks.data` and are not part of the public DTO.
- The user explicitly chose administrator-visible original upstream error bodies. Internal diagnostics therefore do not mask provider names, URLs, status text, or error detail; credential fields and oversized binary/Base64 payloads remain the narrow safety exception.
- The existing video response redactor already removes large Base64 payloads. It remains suitable for ordinary success bodies; error diagnostics preserve original provider detail except credentials and database-size hazards.
- Focused polling integration tests can reuse the package-level SQLite setup and `mockAdaptor` in `task_billing_test.go`, so transient HTTP behavior can be proven without live provider calls.
- `TaskInfo.Data` is explicitly intended for a small raw task-result JSON. Moving sanitized poll bodies into this field fixes same-status persistence while preserving existing terminal CAS, billing, and Webhook behavior.
- The HTTP-aware branch is intentionally in the common single-task polling boundary: every async provider benefits from retaining 408/429/5xx, while provider-specific status mapping remains inside each adaptor. Existing adaptors that reject unknown HTTP-200 statuses with an error keep their behavior.
- Provider-neutral fallback text must preserve short safe legacy reasons such as `expired`, but suppress provider names, URLs, embedded JSON, credential markers, and long diagnostics.
- The optional public `upstream_status` needs transport metadata that a raw JSON body may not contain. Persisting the last poll HTTP status in `TaskPrivateData` avoids wrapping or altering the administrator-visible response body and requires no column migration.
- Historical failed-video Webhook rows can contain the old raw error object. Sanitizing them only in `outboundWebhookPayload` preserves administrator evidence at rest and prevents detail leakage on retries or delayed delivery.
- Unknown HTTP 200 response handling needs a final empty-status fallback outside JSON decoding; otherwise a plain-text body can bypass the retention branch and persist an invalid empty task status.
- Local interface verification against an existing failed Adobe task returns the expected public `upstream_timeout` object with HTTP 408 and no provider name, while internal task data remains available to administrators.
- The global timeout sweep currently wins a guarded status CAS and refunds afterward, but unlike `ApplyTaskResult` it does not call `CreateTaskWebhookEventTx`; outer-timeout failures are therefore visible by query but silent to Webhook-only consumers.
- `CreateTaskWebhookEventTx` already skips legacy rows without normalized image/video request records, and creates delivery rows only for enabled endpoints. Reusing it inside the timeout CAS transaction preserves existing compatibility without a schema change.
- The outbound Webhook worker scans pending deliveries every second, so timeout finalization only adds local database writes and never waits on receiver I/O.
- The winning timeout transaction can reuse `Task.UpdateWithStatusTx`; a stale timeout scanner gets `won=false`, creates no event, and leaves a concurrently completed task untouched.
- Timeout finalization now produces the same provider-neutral public failure envelope as ordinary terminal polling because both paths call `CreateTaskWebhookEventTx` after persisting the terminal task inside one transaction.

# Normalized Video Submit/Poll Race Findings (2026-08-04)

- The remote Adobe2API row was created at 04:03:21, retained local ID `5507a80922024cfbbac82de849b92b70`, and reached the real `reference_image_privacy_error` terminal state at 04:03:58.
- The corresponding new-api row was created at 04:03:20 and failed after about four seconds with `Video task not found`, before Adobe2API reached any terminal state.
- `createDurableVideoTask` inserts the normalized task as pollable before `DoRequest` starts; `PersistTaskSubmitResult` writes the provider ID only after `DoResponse` completes.
- `resolveTaskPollingUpstreamID` protects only ImageHandle when `PrivateData.UpstreamTaskID` is empty. Other tasks call `GetUpstreamTaskID`, which falls back to the public `task_...` ID.
- A global polling tick during provider submission can therefore query `/v1/videos/task_...`, receive 404, mark the task terminal, and refund before the correct provider ID is persisted.
- The existing 60-second 404 grace is limited to OpenAI Video compatibility metadata and does not cover normalized `/v1/video/tasks` rows.
- Durable normalized video rows are already distinguishable without a schema migration through `Properties.AssetType == video` plus a non-empty `Properties.Operation`.
- The safest minimal fix is readiness gating, not a longer retry alone: a normalized row with no provider ID remains locally submitting and is skipped by polling; existing timeout handling owns abandoned rows.
- `PersistTaskSubmitResult` merges the provider ID under a row lock without requiring a specific non-terminal status, so changing the initial internal state to `SUBMITTED` does not break provider-ID persistence.
- Explicit submit failures snapshot the current status and use `UpdateWithStatusTx`; they therefore retain their existing single-winner terminal transition when the initial state is `SUBMITTED`.
- Both `SUBMITTED` and `QUEUED` project to public `queued`, so the internal readiness distinction is backward-compatible for `/v1/video/tasks` clients.


- The local `/v1/models` response exposes five Adobe Seedance SKUs with the
  `adobe-seedance-*` prefix; the Seedance docs use bare names and omit fast/1080p.
- HiggsfieldVideo promotes AdobeVideo's `ResolveVideoBilling`, but Higgsfield provider
  SKUs use hyphens while the Adobe capability map uses underscores. A VideoPricing-bound
  Higgsfield request is therefore rejected as an unsupported Adobe provider SKU.
- Higgsfield's model validation checks only exact model membership and does not apply
  the shared Seedance duration, aspect ratio, or reference limits when legacy pricing is used.
- Adobe2API caps video prompts at 1200 Unicode characters. new-api currently checks only
  for a non-empty prompt, so the upstream limit is reached after precharge.
- The checked-in OpenAPI contains Kling/Veo and `images`, but its generator does not.
  `bun run openapi:check` fails and regeneration would remove the new contract.
- Current Adobe Seedance completions use direct signed URLs with `url_auth=none`; the
  Seedance task and Webhook pages still show only the historical protected Asset URL.
- Normalized reference images, videos, and audios are URL-only HTTP(S) sources.
  `input.video` is intentionally a separate source contract and also accepts a data URL
  or an exact `provider` plus `file_id` pair for edit/extension/remix.
- The Adobe and new-api Kling capability catalogs currently agree on 3-15 seconds, not
  the earlier proposed 1-15 seconds. Documentation and OpenAPI must describe executable
  runtime behavior unless both services are deliberately changed together.
- The repaired OpenAPI generator now owns the Adobe Kling/Veo capability extension,
  four request examples, `images` mode, direct Adobe result semantics, and the separate
  source-video schema. Regeneration and drift checking both succeed.
- Adobe normalized task `duration_ms` currently echoes the validated request duration;
  neither Adobe2API nor new-api downloads the result media body to probe a final duration.
- Docker public-endpoint probes show global reference-count and frame/images media-type
  violations return `invalid_request` in the controller before provider validation.
  Provider-level `reference_*_limit_exceeded` codes are not externally reachable for the
  current Adobe limits, so the public docs now describe the observed API response.


---

## Infinite Canvas Authorization Findings

- TokenAuth 会再次校验用户对 Token Group 的可用权限，授权前和兑换时都必须使用用户当前可用分组集合校验。
- AssetKey 当前只有 assets:read scope，并且每个用户只保留最新一枚当前 Key；Canvas 应复用有效 Key，无效时使用事务兼容 helper 轮换。
- 管理菜单 grantable 集合当前由默认权限派生；canvas_config 需要可授予但不属于普通管理员默认权限，必须拆分默认与可授予集合。
- 模型白名单应从聚合分组可用模型与 endpoint type 交集生成：图片为 image-generation，视频为 video-task/openai-video。
- Session Cookie SameSite=Strict 不影响顶层授权弹窗；公开兑换接口不依赖 Cookie，并使用 PKCE 防止授权码被截获兑换。
- AutoMigrate 同时存在普通和 fast 两套模型清单，CanvasGrant 与 CanvasAuthorizationCode 必须加入两处。
- 配置使用现有 Option 表可以避免单例配置表；管理员接口仍提供强类型校验和聚合分组/模型预览。
- 现有 CreateAssetKeyWithScopes 内部开启事务，不能直接嵌套到授权兑换原子事务，需要抽取接收 *gorm.DB 的内部实现。
- `service.GetModelsForGroup` 是聚合分组模型目录的现有事实来源，会展开聚合目标分组并去重。
- 登录页所有本地成功分支集中在 LoginForm，可抽 same-origin return_to helper；第三方 OAuth 回调需读取同一 sessionStorage 目标。
- 实测发现登录处理器与 `AuthRedirect` 会先后导航；只把 return-to 提前消费仍会让后续 `<Navigate>` 用默认 `/console` 覆盖。目标必须在登录跳转期间保持幂等，直到授权页确认抵达后清除；普通无参数登录页则清除陈旧目标。
- new-api 的 `UserAuth` 不只校验 Session Cookie，还要求 `New-Api-User` 与 Session 用户一致；授权确认页使用原生 fetch 时必须显式携带该头，否则登录成功后仍返回 401。
- 本地 UI 保存配置后，图片聚合分组识别 1 个 `gpt-image-2`，视频聚合分组识别 13 个视频任务模型；桌面与 390px 窄屏下本功能表单保持可读、单列换行且无控件重叠。
- Canvas 专项服务测试已覆盖配置、回调、PKCE、授权码重放、分组权限、Token 修复/上限、Resource Key 复用与并发兑换；`go test ./service ./controller ./model` 通过。
- 管理菜单权限测试与前端生产构建通过；剩余验证集中在本地 Docker 登录/授权交互和真实媒体链路。
- `/api/canvas/oauth/token` 的 OPTIONS 响应并不能保证实际 POST 带 CORS；兑换路由必须显式挂载现有 CORS 中间件，路由测试需要同时断言 POST 响应头。
- 真实浏览器已验证同步预开授权窗可避免异步 PKCE 丢失用户激活，完整授权会自动关闭弹窗并立即持久化三类凭证与模型缓存。
- 本地真实图片和视频请求均通过自动授权凭证完成；重复授权修复与缺分组零副作用已由数据库状态和 UI 共同验证。
- 联调测试数据已精确清理，临时倍率与模型元数据不存在，Canvas 管理配置保持分组/回调但 `enabled=false`。
# Leonardo Seedance 2.5 Runtime Findings (2026-08-09)

- Model discovery now returns Leonardo2API `/v1/models` without filtering, but the runtime adaptor still rejects provider SKUs absent from `supportedModels`.
- The production `unsupported_video_model` response is emitted before upstream submission and billing.
- Seedance 2.5 needs more than a list entry: the current non-H3 path caps duration at 15 seconds, permits only `media`, and omits `reference_mode` from the Leonardo2API request body.
- Adobe model discovery was already unfiltered and is outside this runtime fix.
- The Leonardo adaptor performs duration and reference-mode validation twice: request normalization and pre-billing model validation; both must classify 2.5 consistently.
- `BuildRequestBody` currently forwards `reference_mode` only for H3, so Seedance 2.5 frame requests require an explicit payload change.
- The OpenAPI generator owns the public model catalog and currently documents only five Seedance 2.0 SKUs; the generated JSON must be refreshed after adding the two 2.5 SKUs.
- Seedance 2.5 can be represented without a new public DTO: its only normalized contract differences are a 30-second maximum and `frame` support with one or two ordered images.
- Local Docker has an existing isolated Leonardo mock channel (`128`) pointing to `http://async-test-mock:8080`; the real local Leonardo channel is separate (`125`). A temporary 2.5 mapping can therefore be tested without paid generation if exact database state is restored afterward.
- Existing unlimited local token `153` belongs to the mock channel's `vip` group, and the mock exposes reset/metrics endpoints plus the last submitted Leonardo payload. No new credential or real channel is required for the zero-paid probe.
- Disposable Docker records can be removed exactly by returned public `tasks.task_id`, matching `logs.request_id`, and the temporary channel-128 ability/model mapping; no broad cleanup is needed.
- Successful video polling may create `assets` rows keyed by the public task ID; cleanup must remove those before deleting the task and restore token/user/channel quota counters captured before the probe.
- The second Docker probe reached channel submission and was refunded, but async-test-mock rejected the valid 30-second 2.5 payload because its own fixture still globally capped video duration at 15 seconds and treated non-H3 `reference_mode` as legacy. The production adaptor is no longer the blocker.
- The rebuilt async-test-mock now mirrors Seedance 2.5's 4-30 second range and `frame`/`media` rules while leaving Seedance 2.0 and H3 behavior unchanged.
- The public task query route requires an active Resource Key. The only pre-existing local `ak_` row was soft-deleted, so the final probe used a temporary read-only key and removed it afterward.
- The completed new-api probe returned HTTP 202, reached `succeeded`, selected only mock channel 128, and created one Asset. Its upstream snapshot contained model `seedance-2.5-720p`, duration 30, `generate_audio=false`, and `seed=-1` with exactly one submit.
- Final cleanup restored channel 128's original group/models/mapping, VideoPricing, user/token/channel quota counters, and removed the temporary ability, Task, Asset, log, idempotency, Webhook, and Resource Key records. Both application and mock containers are healthy.

---

## Broad Leonardo reference admission findings (2026-08-10)

- The public controller currently caps media requests at 9 images, 3 videos, 3 audios, and 12 total,
  rejecting valid Leonardo requests before channel selection.
- The Leonardo adaptor separately duplicates H3, Seedance 2.0/Fast, and Seedance 2.5 reference
  counts/combinations. Seedance 2.5 media currently falls through to the stale 4/3/1 branch.
- The normalized reference DTO carries URL/provider/file ID/name but no trustworthy media duration.
  Downloading and `ffprobe` validation correctly remain Leonardo2API responsibilities.
- Known local validation runs before VideoPricing and precharge. Leonardo2API validation responses
  are already accepted only through a bounded status/body/code/message sanitizer.
- The agreed boundary is a 30-image/10-video/10-audio/50-total safety envelope plus common request
  checks in new-api; exact per-model rules remain solely in Leonardo2API.
- The generated OpenAPI and Resource Center page still advertised the former 9/3/3/12 common
  envelope and copied concrete Leonardo reference rules, so they must be regenerated from wording
  that distinguishes the gateway safety envelope from downstream model validation.
