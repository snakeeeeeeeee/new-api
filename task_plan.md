# Task Plan: Assets Resource Center

## Goal
Add a first-class assets table and Resource Center UI for async image/video task results. Tasks remain the execution and billing facts; assets become the searchable, previewable, exportable resource index. Add dedicated asset API keys so users can query generated assets programmatically without granting model-calling token permissions.

## Current Phase
Complete

## Phases
- [complete] Add backend asset model, migration, extraction, query/export APIs, and route wiring.
- [complete] Hook asset creation into shared task success handling for callback and polling paths.
- [complete] Add task-log resource type filtering.
- [complete] Build Resource Center frontend page, menu entries, permissions, filters, grid/table views, detail drawer, and batch actions.
- [complete] Add targeted backend/frontend tests and run regression checks.
- [complete] Add asset-specific API keys, `/v1/assets` read APIs, and Resource Center API Key/documentation tabs.
- [complete] Run backend and frontend regression checks after asset key work.

## Decisions Made
| Decision | Rationale |
| --- | --- |
| Use a separate `assets` table | Avoids overloading task logs and supports one task producing multiple resources. |
| Store direct stable URLs in `assets.url` | image-handle already returns usable R2/CDN URLs; first version should not add object storage SDKs or signed URL logic. |
| Do not implement server-side ZIP in v1 | Prevents new-api from becoming a large-file proxy/packager. CSV/URL export and small direct downloads cover the first workflow. |
| Create assets only on first task `SUCCESS` transition | Reuses existing CAS terminal update semantics and prevents duplicate rows from callback/polling races. |
| Use `asset_type` filters in both assets and task logs | Lets users separate image/video/audio task results without knowing internal task actions. |
| Let `/api/assets/self` accept session auth or API token auth | Users and API clients can both retrieve their own generated resources; admin endpoints remain admin-only. |
| Wrap first task `SUCCESS` transition and asset inserts in one DB transaction | Prevents task success from committing when resource rows fail to persist. Billing still runs only after the transaction commits. |
| Use dedicated `ak_` asset keys for `/v1/assets` | Gives external asset consumers least-privilege read access and prevents normal model tokens from becoming resource-library credentials. |
| Store asset keys in plaintext | The user explicitly requires generated keys to be viewable after creation, not only once. UI will mask by default. |

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| DTO imported `model`, causing model -> dto -> model import cycle | `go test ./model ./service ./controller` | Changed DTO metadata type to `map[string]any`. |
| Semi UI does not export `Drawer` in this version | `cd web && bun run build` | Replaced the detail drawer with existing `SideSheet`. |
| `/api/assets/self` rejected OpenAI API token during curl smoke test | Local Docker API validation | Switched user assets routes from `UserAuth` to `TokenOrUserAuth`; admin routes remain `AdminAuth`. |
| Shared controller callback test DB missed the new `assets` table | Transactional success update test | Added `model.Asset` to the shared test migration. |
