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
