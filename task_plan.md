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
