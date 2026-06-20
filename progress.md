# Progress

## 2026-06-21
- Confirmed current recharge stats are based on `top_ups`.
- Confirmed successful subscription purchases are synced into `top_ups` with `amount = 0` and non-zero `money`.
- Confirmed admin cancellation marks `subscription_orders.invalidated_at` but does not remove or invalidate the synced `top_ups` row.
- Decided to split subscription package purchase stats from ordinary recharge stats and use `subscription_orders` as the authoritative subscription purchase source.
- Implemented backend stats fields for `subscription_purchase_summary`, `subscription_purchase_ranking`, and `subscription_purchase_details`.
- Updated ordinary recharge stats to exclude `top_ups` rows associated with successful subscription orders.
- Added controller query params for subscription purchase ranking and details pagination.
- Extended usage stats regression coverage for valid subscription purchases and invalidated subscription exclusion.
- Updated UsageStats frontend to show balance recharge stats and subscription package purchase stats as separate summary cards, ranking tables, and detail drawers.
- Reworded recharge labels to "余额充值" on the usage stats page to avoid mixing wallet top-up and subscription purchase semantics.
- Added locale keys for the new balance recharge and subscription purchase labels across all frontend locale files.

## Test Results
| Test | Status | Notes |
| --- | --- | --- |
| `go test ./model -run 'TestGetUsageStats'` | Pass | Covers recharge split and subscription purchase stats. |
| `go test ./model ./controller` | Pass | Broader backend regression check for touched packages. |
| `cd web && bun run build` | Pass | Build succeeds; existing Browserslist/eval/chunk-size warnings remain. |
| Locale JSON parse and new-key check | Pass | All locale files parse and include the new UsageStats keys. |

## Error Log
| Timestamp | Error | Attempt | Resolution |
| --- | --- | --- | --- |
