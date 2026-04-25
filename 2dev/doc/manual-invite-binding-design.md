# 后台手动绑定邀请归属设计说明

## 背景

部分用户没有通过邀请链接或邀请码注册，但业务上需要归属到某个邀请人名下，便于后台邀请统计和邀请人明细查看。旧邀请字段 `inviter_id` 不能单独满足新邀请码统计，因为当前统计逻辑以 `invite_code_owner_id` 为归属字段，并要求 `invite_code_id > 0`。

## 范围

本功能是“统计归属绑定”，不是奖励补发，也不是历史分账。

- 绑定、重绑时只更新被绑定用户的 `inviter_id`、`invite_code_owner_id`、`invite_code_id`。
- 解绑时将上述三个字段清零。
- 不补发邀请奖励。
- 不增加 `invite_codes.reward_used_uses`。
- 不修改 `aff_quota`、`aff_history`、`aff_count`。
- 不回算历史充值、历史消费或历史分账，仅让现有统计查询按新的归属字段聚合。

## 接口

- `PUT /api/user/:id/invite_binding`
  - `owner_user_id`：目标邀请人 ID。
  - `invite_code_id`：可选，指定目标邀请人的邀请码；为 0 或不传时自动使用 `MANUAL-<owner_user_id>`。
- `DELETE /api/user/:id/invite_binding`
  - 解绑用户邀请统计归属。
- `GET /api/user/:id/invite_codes`
  - 给后台弹窗加载目标邀请人的可绑定邀请码，默认不返回已删除邀请码。

这些接口放在 user admin 路由下，因为操作对象是“被绑定用户”，入口也在用户管理表格。

## 自动手动码

未指定邀请码时，系统在事务中创建或复用手动码：

- `prefix = MANUAL`
- `code = MANUAL-<owner_user_id>`
- `owner_user_id = 目标邀请人 ID`
- `target_group = 目标邀请人当前 group`，为空时使用 `default`
- `reward_quota_per_use = 0`
- `reward_total_uses = 0`
- `reward_used_uses = 0`
- `status = enabled`

手动码只是统计归属载体，不参与本次绑定奖励。

## 审计

每次绑定、重绑、解绑都会写 `LogTypeManage` 管理日志，记录管理员 ID、旧邀请人、旧统计归属、旧邀请码、新邀请人、新统计归属、新邀请码。
