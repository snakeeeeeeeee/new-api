# Findings

## Current Recharge Stats
- `model/log.go` builds `recharge_summary`, `recharge_ranking`, and `recharge_details` from `top_ups`.
- The filter is currently `top_ups.status = success` and `complete_time` within range.
- Subscription purchases are synced to `top_ups` in `CompleteSubscriptionOrder` through `upsertSubscriptionTopUpTx`.
- Synced subscription `top_ups` rows have `amount = 0`, `money = order.Money`, and `status = success`.
- Admin invalidation marks `subscription_orders.invalidated_at` and related metadata; it does not change the synced `top_ups` row.

## Desired Behavior
- Ordinary recharge stats should represent wallet/top-up orders only.
- Subscription package purchases should be separated into their own stats block.
- Admin-cancelled/deleted subscription orders should be excluded from subscription package purchase stats.

## Proposed Data Contract
- Keep existing fields:
  - `recharge_summary`
  - `recharge_ranking`
  - `recharge_details`
- Add new fields:
  - `subscription_purchase_summary`
  - `subscription_purchase_ranking`
  - `subscription_purchase_details`
- Add query params for detail pagination:
  - `subscription_purchase_user_id`
  - `subscription_purchase_detail_page`
  - `subscription_purchase_detail_page_size`

## Files Of Interest
- `/Users/zhangyu/code/go/new-api/model/log.go`
- `/Users/zhangyu/code/go/new-api/controller/log.go`
- `/Users/zhangyu/code/go/new-api/web/src/pages/UsageStats/index.jsx`
- `/Users/zhangyu/code/go/new-api/model/log_test.go`
