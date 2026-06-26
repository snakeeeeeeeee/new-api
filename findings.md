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
