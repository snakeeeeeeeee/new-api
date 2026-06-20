# Task Plan: Usage Stats Subscription Purchase Split

## Goal
Update usage statistics so normal recharge stats exclude subscription package purchases, invalidated subscription orders are excluded from paid subscription statistics, and subscription package purchases are shown as a separate stats section with summary, ranking, and user-level details.

## Current Phase
Complete

## Phases
- [complete] Confirm current recharge stats source and subscription completion behavior.
- [complete] Add backend subscription purchase stats and remove subscription purchases from recharge stats.
- [complete] Update usage stats controller query parameters.
- [complete] Update frontend UsageStats page to show separate subscription package stats.
- [complete] Add/update regression tests for normal recharge, subscription purchases, and invalidated subscriptions.
- [complete] Run targeted Go tests and frontend checks as feasible.

## Decisions Made
| Decision | Rationale |
| --- | --- |
| Use `subscription_orders` as the source for subscription package stats | This includes historical paid subscription orders even if older code did not sync a `top_ups` row. |
| Exclude `subscription_orders.invalidated_at > 0` | Admin-cancelled/deleted subscriptions should not count in the new paid subscription stats per user request. |
| Exclude `top_ups` whose `trade_no` matches a successful subscription order | Prevents subscription purchases from being mixed into ordinary recharge stats. |
| Keep normal recharge fields stable | Reduces API and UI breakage for existing consumers. |

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
