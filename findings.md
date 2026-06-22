# Findings

## Existing Task and UI Extension Points
- `model.Task` stores task status, platform/action, user/channel, private result URL, and raw `Data`.
- `service.ApplyTaskResult` is the shared polling/callback terminal update path from the ImageHandle integration.
- Task logs already expose platform/action/status/time filters in backend APIs; frontend currently lacks a user-friendly resource type filter.
- Sidebar/admin permissions use keyed modules; adding a new admin resource menu requires a new menu key and default config entry.
- Frontend task-log rendering already recognizes image/video actions and can be reused for action-to-type mappings.

## Assets Resource Center Decisions
- The first version will persist direct URLs only, with optional metadata and thumbnail fields.
- One task can create multiple assets, keyed by `task_id + asset_index`.
- User assets endpoints must hide blocked/deleted/unavailable assets unless admin APIs explicitly request them.
- CSV export should stream or write a simple response directly from query results and include the agreed fields.
- Frontend should present a dense operational UI: filters, tabs, grid/table toggle, batch actions, and a detail drawer.
- Asset API keys should be separate from normal model tokens. They will use the `ak_` prefix, fixed `assets:read` scope, optional IP restrictions, and optional expiration.
- Because keys must remain viewable later, the first version stores the full key in `asset_keys.key`; the UI should mask by default and reveal on demand.

## Files Of Interest
- `/Users/zhangyu/code/go/new-api/model/task.go`
- `/Users/zhangyu/code/go/new-api/service/task_polling.go`
- `/Users/zhangyu/code/go/new-api/controller/task.go`
- `/Users/zhangyu/code/go/new-api/router/api-router.go`
- `/Users/zhangyu/code/go/new-api/web/src/components/table/task-logs`
- `/Users/zhangyu/code/go/new-api/web/src/components/layout/SiderBar.jsx`
- `/Users/zhangyu/code/go/new-api/web/src/App.jsx`
