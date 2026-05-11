# 二级代理邀请与统计规划

## Summary

实现严格两级邀请代理：

- 管理员创建给 A 的邀请码默认赋予 A `agent_level=1`，A 可以邀请用户，也可以给自己通过非手动邀请码邀请来的 B 一键开启邀请功能。
- A 给 B 开启后，系统为 B 自动生成一个启用的邀请码，B 的 `agent_level=2`；B 可以邀请用户 C，但不能再给 C 开启邀请功能。
- 统计分开展示：A 看到每个 B 本人的充值/消费总额，也看到每个 B 邀请用户的充值/消费总额；B 只看到自己直接邀请用户的充值/消费总额和趋势。

## Key Changes

- 扩展 `InviteCode`：
  - 新增 `agent_level`：邀请码归属用户的代理等级，现有和管理员新建邀请码默认为 `1`，A 给 B 开通的邀请码为 `2`。
  - 新增 `granted_by_user_id`：二级邀请码由谁开通；管理员创建的邀请码为 `0`。
  - 手动绑定码 `MANUAL-*` 不参与代理等级判定，也不能用于注册。
- 新增用户侧接口：
  - `POST /api/user/self/invitees/:id/enable_invitation`
    - 仅 `agent_level=1` 用户可调用。
    - 目标用户必须是调用者通过非手动邀请码直接邀请来的用户。
    - 自动为目标用户生成一个二级邀请码：目标分组继承 B 当前分组，注册赠送额度为 `0`，赠送次数为 `0`，状态启用。
    - 如果目标已有代理能力，返回已开启错误，避免重复创建或降级。
  - `GET /api/user/self/invite_agent_stats?period=day|month&start_time=&end_time=`
    - 返回当前用户代理等级、直接邀请统计、趋势数据。
    - 对一级用户额外返回二级代理列表：每个 B 的本人充值/消费汇总、B 直接邀请用户充值/消费汇总，以及所有 B 的聚合趋势。
- 扩展现有 `/api/user/self` 和 `/api/user/self/invitees` 返回：
  - `invite_agent_level`
  - `can_grant_invitation`
  - 被邀请人是否已开通邀请能力、对应二级邀请码信息。
- 前端在充值/邀请页增强现有 `InvitationCard`：
  - A 的被邀请人列表增加“开启邀请功能”按钮和“已开启”状态。
  - A 增加“二级代理统计”区域，展示 B 本人流水和 B 下级流水。
  - B 只展示自己的邀请码、直接邀请用户列表、充值/消费汇总和日/月趋势图。
  - 图表使用现有 `@visactor/react-vchart`。

## Statistics Rules

- 充值总额：只统计 `top_ups.status = success`，金额用 `amount` 和 `money`，时间用 `complete_time`。
- 消费总额：总量继续使用 `users.used_quota`；趋势图使用 `logs` 中 `LogTypeConsume` 的 `quota` 和 `created_at`。
- 如果站点关闭消费日志，消费趋势为空，但累计消费总额仍正常显示。
- 聚合在 Go 层完成，避免使用数据库专属日期函数，保证 SQLite、MySQL、PostgreSQL 兼容。
- 默认查询范围：日维度近 30 天，月维度近 12 个月；日维度最多 366 天，月维度最多 36 个月。

## Test Plan

- Model tests：
  - 现有邀请码迁移后默认是一级；手动码不授予代理能力。
  - 一级 A 可给直属 B 开通二级邀请码。
  - 二级 B 不能给 C 开通邀请功能。
  - 非直属用户、手动绑定来源、管理员/更高角色用户均不能被普通用户开通。
  - A 的统计分离 B 本人流水和 B 下级流水；B 只看到直接邀请用户流水。
  - 日/月趋势只统计成功充值和消费日志。
- Controller tests：
  - 新增开通接口成功和拒绝场景。
  - 新统计接口在一级、二级、无代理权限用户下返回正确结构。
- Regression：
  - 运行相关 Go 测试：`go test ./model ./controller`
  - 前端构建：`cd web && bun run build`
  - i18n 同步/检查：`cd web && bun run i18n:extract && bun run i18n:sync && bun run i18n:lint`

## Assumptions

- 旧版 `aff_code` 邀请逻辑保持不变，不纳入新二级代理统计。
- 管理员仍保留完整邀请码管理能力；普通用户只获得一键给直属用户开通二级邀请码的能力。
- 本期不做 A 对 B 邀请码的停用/删除管理，停用和删除仍由管理员处理。
