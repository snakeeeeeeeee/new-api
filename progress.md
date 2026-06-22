# Progress

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

## Error Log
| Timestamp | Error | Attempt | Resolution |
| --- | --- | --- | --- |
| 2026-06-23 | `TestBuildRequestBodyMatchesImageHandleContract` failed after adding mandatory internal secret config | Targeted test run | Added test env vars and callback secret settings, then re-ran targeted tests. |
| 2026-06-23 | Invalid `client_task_id` test returned config error before validation error | Targeted test run | Reordered ImageHandle adaptor validation so request shape errors are returned before deployment config errors. |
| 2026-06-23 | Local token `qArd...` returned 401 | Docker dev test | Token row was soft-deleted; created a local test token `codexasyncimage20260623localtest0000abcdef123456`. |
| 2026-06-23 | Local token could not access `ikun_gpt-image-2` | Docker dev test | Added `ikun_gpt-image-2` to dev `UserUsableGroups`. |
| 2026-06-23 | First callback event stayed pending | Docker dev test | Callback URL was `localhost:3001`, which points to the image-handle container. Changed local callback address to `http://host.docker.internal:3001`. |
