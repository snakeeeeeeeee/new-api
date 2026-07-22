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
