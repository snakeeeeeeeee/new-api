# Progress

## 2026-06-23
- Started Assets Resource Center implementation on top of the completed ImageHandle async task integration.
- Confirmed existing worktree contains ImageHandle changes and unrelated untracked local files; this implementation will not revert them.
- Created the assets/resource-center task plan and findings files.
- Added `assets` model, migration, query/export APIs, task success asset extraction, and Resource Center frontend page.
- Added task-log `asset_type` filtering and resource/admin menu wiring.
- Tightened consistency: first task `SUCCESS` transition and asset row inserts now commit in one DB transaction; asset insert failure rolls back task success and skips billing.
- Added dedicated asset API keys (`ak_`), `/v1/assets` read APIs, Resource Center API Key tab, and Resource Center usage documentation tab.

## Test Results
| Test | Status | Notes |
| --- | --- | --- |
| `go test ./model ./service ./controller` | passed | Backend model/service/controller checks after assets API and task success write path. |
| `cd web && bun run build` | passed | Resource Center frontend compiles; existing Browserslist/eval/chunk-size warnings remain. |
| `go test ./...` | passed | Full backend regression after assets/resource center changes. |
| `docker compose -f docker-compose-dev.yml build new-api-dev` | passed | Docker image rebuilt with backend and frontend changes. |
| Docker dev image-handle smoke test | passed | `task_assets_dev_20260623012923` reached `SUCCESS`, created `asset_KrChLj9O62FLNpZ2T78wgoDT9ZK7Vcgh`, and `/api/assets/self`, batch URLs, CSV export returned it with `Bearer testapikey`. |
| `go test ./service ./controller ./model && go test ./...` | passed | Regression after transactional task success + asset insert refactor. |
| `cd web && bun run build` | passed | Frontend still compiles after consistency refactor; same existing warnings. |
| `go test ./...` | passed | Full backend regression after asset API key implementation. |
| `cd web && bun run build` | passed | Resource Center three-tab UI compiles; same existing Browserslist/eval/chunk-size warnings remain. |
| `docker compose -f docker-compose-dev.yml build new-api-dev` | passed | Dev Docker image rebuilt after asset API key implementation. |

## Error Log
| Timestamp | Error | Attempt | Resolution |
| --- | --- | --- | --- |
| 2026-06-23 | DTO imported `model`, causing model -> dto -> model import cycle | `go test ./model ./service ./controller` | Changed DTO metadata type to `map[string]any`. |
| 2026-06-23 | Semi UI does not export `Drawer` in this version | `cd web && bun run build` | Replaced the detail drawer with existing `SideSheet`. |
| 2026-06-23 | `/api/assets/self` rejected OpenAI API token during curl smoke test | Local Docker API validation | Switched user assets routes from `UserAuth` to `TokenOrUserAuth`; admin routes remain `AdminAuth`. |
| 2026-06-23 | Callback tests failed after making asset insert part of task success transaction because shared test DB had no `assets` table | `go test ./service ./controller` | Added `model.Asset` to shared controller test migration and added rollback coverage. |
