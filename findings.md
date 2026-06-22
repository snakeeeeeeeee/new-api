# Findings

## Current Direction
- The previous ImageHandle implementation used `ChannelTypeImageHandle = 58` as a task adaptor selected by normal model distribution.
- That makes `/v1/image/tasks` compete with existing image-generation channels for the same model name, which caused `metadata` to be forwarded to a normal OpenAI image endpoint.
- The desired architecture is different: new-api should first select the existing real image channel, lock it on the task, and submit only an executor job to image-handle.
- image-handle confirmed it will support only `executor.type = "new_api_internal"` and will not choose providers directly.

## Key Constraints
- Existing synchronous `/v1/images/generations` and `/v1/images/edits` must keep working.
- Existing tasks, callback settlement/refund, polling fallback, and assets creation should remain compatible.
- Internal execute must HMAC verify requests from image-handle and be idempotent.
- Internal execute secret must be separate from callback secret.
- `execute_url` must be configurable so it can point at a Docker/cluster-internal new-api address.
- ImageHandle task rows still use `platform=58`, but `channel_id` is now the selected real image channel.
- The executor is configured with environment variables: `IMAGE_HANDLE_BASE_URL`, `IMAGE_HANDLE_API_KEY`, `IMAGE_HANDLE_INTERNAL_BASE_URL`, `IMAGE_HANDLE_INTERNAL_SECRET_ID`, `IMAGE_HANDLE_INTERNAL_SECRET`; `IMAGE_HANDLE_CALLBACK_SECRET` is only a legacy fallback.
- The formal callback secret should come from the selected real image channel settings as `callback_secret`, using `channel_<channel_id>` as the callback secret id.
- The image-handle executor settings now live under `异步任务管理 -> 异步图片执行器`, are persisted as `image_handle_setting.*` options, and environment variables remain startup fallback values.
- Admins can view saved image-handle API/internal/callback secrets in the async task page and can view saved channel `callback_secret` in the channel edit modal.
- Internal execute parses standard OpenAI-compatible image responses with `data[].url` or `data[].b64_json`; this covers the current `gpt-image-2` path.
- Async edit tasks are supported by storing `input.images`/`input.mask` URLs and rebuilding a multipart `/v1/images/edits` request during internal execute. Downloads go through the existing `service.DoDownloadRequest` SSRF and size checks.
- Non-retry internal execute results are cached in task private data so repeated execute calls do not call the real upstream twice.

## Files Of Interest
- `/Users/zhangyu/code/go/new-api/router/video-router.go`
- `/Users/zhangyu/code/go/new-api/controller/relay.go`
- `/Users/zhangyu/code/go/new-api/relay/relay_task.go`
- `/Users/zhangyu/code/go/new-api/relay/channel/task/imagehandle/adaptor.go`
- `/Users/zhangyu/code/go/new-api/controller/task_callback.go`
- `/Users/zhangyu/code/go/new-api/service/task_polling.go`
- `/Users/zhangyu/code/go/new-api/model/task.go`
- `/Users/zhangyu/code/go/new-api/controller/channel.go`
- `/Users/zhangyu/code/go/new-api/dto/channel.go`
