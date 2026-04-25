# 管理员邀请码管理与邀请统计设计

更新时间：2026-04-19

## 目标

新增一套独立于旧 `aff_code` 的管理员邀请码体系：

- 管理员可批量生成指定前缀邀请码
- 每个邀请码必须绑定到具体用户
- 每个邀请码可配置目标分组、单次赠送额度、赠送次数
- 新用户通过邀请码注册后：
  - 绑定归属用户
  - 进入指定分组
  - 在赠送次数未耗尽时获得赠送额度
- 用户侧与管理员侧都能看到统计数据

旧 `aff_code` 邀请体系继续保留，不做破坏性修改。

## 数据结构

### 新表 `invite_codes`

- `id`
- `code`
- `prefix`
- `owner_user_id`
- `target_group`
- `reward_quota_per_use`
- `reward_total_uses`
- `reward_used_uses`
- `status`
- `created_time`
- `updated_time`
- `deleted_at`

### `users` 表新增字段

- `invite_code_id`
- `invite_code_owner_id`

说明：

- 新字段只服务于新邀请码体系
- 不回填历史用户
- 只有后续通过新邀请码注册的用户会写入

### 旧字段保留

- `aff_code`
- `aff_count`
- `aff_quota`
- `aff_history_quota`
- `inviter_id`

这些字段不删除、不改语义，旧逻辑继续可用。

## 迁移时机

完全依赖现有启动时迁移流程：

- `InitDB()`
- `migrateDB()`
- `DB.AutoMigrate(...)`

处理方式：

- 将 `InviteCode` 模型加入现有 `AutoMigrate`
- `User` 模型新增字段也通过现有 `AutoMigrate` 自动补列

不新增手工 SQL，不新增独立迁移脚本。

## 注册生效逻辑

第一版仅接入**密码注册页**。

注册请求新增字段：

- `invite_code`

处理规则：

- 如果传了 `invite_code`，走新邀请码体系
- 如果没传 `invite_code`，继续走旧逻辑

新邀请码注册成功后：

- 写入 `users.invite_code_id`
- 写入 `users.invite_code_owner_id`
- 同时写入 `users.inviter_id = owner_user_id`
- 用户分组改为邀请码指定分组

赠送次数逻辑：

- 若 `reward_used_uses < reward_total_uses`
  - 给新用户赠送 `reward_quota_per_use`
  - `reward_used_uses += 1`
- 若次数耗尽
  - 邀请码仍可继续注册
  - 仍绑定归属用户
  - 仍进入指定分组
  - 仍计入统计
  - 仅不再赠送额度

## 统计口径

### 用户侧

升级现有邀请卡片，显示：

- 邀请人数
- 邀请充值额度
- 邀请实付金额
- 邀请消费额度

统计口径只看新邀请码体系：

- 邀请人数：`users.invite_code_owner_id = 当前用户`
- 邀请充值额度：这些用户成功 `top_ups.amount` 汇总，用于展示站内充值额度
- 邀请实付金额：这些用户成功 `top_ups.money` 汇总，用于展示真实支付金额
- 邀请消费额度：这些用户 `used_quota` 汇总

### 管理员侧

#### 邀请码管理页

每个邀请码展示：

- 邀请码
- 前缀
- 归属用户
- 目标分组
- 单次赠送额度
- 总次数
- 已用次数
- 剩余次数
- 邀请人数
- 邀请充值额度
- 邀请实付金额
- 邀请消费额度
- 状态

#### 用户管理

在现有邀请信息位置升级展示归属用户维度汇总：

- 新邀请码邀请人数
- 新邀请码充值额度
- 新邀请码实付金额
- 新邀请码消费额度
- 新邀请码总消费

## 后台功能

新增管理员菜单：

- `邀请码管理`

支持：

- 列表
- 搜索
- 批量生成
- 编辑
- 启用 / 禁用
- 删除

批量生成是创建效率工具：

- 一次生成多个邀请码
- 生成后的每个邀请码都是独立记录
- 可以单独编辑配置

## 回归要求

必须保证：

- 旧 `aff_code` 注册链路不被破坏
- OAuth / 微信等注册入口行为不变
- 兑换码、充值、订阅、用户管理功能不受影响
- 旧邀请字段仍可正常读取

## 验证要求

- `go test ./controller ./model ./service`
- `go test ./...`
- `cd web && bun run build`
- `docker build -t new-api-local:dev .`
- `docker compose -f docker-compose-dev.yml up -d --force-recreate new-api-dev`

docker dev 集成验证至少覆盖：

- 管理员创建邀请码
- 用户通过新邀请码注册
- 奖励次数递减
- 次数耗尽后仍能注册但不再送额度
- 用户侧汇总统计正确
- 管理员侧邀请码统计正确
- 用户管理页归属用户统计正确
- 旧 `aff_code` 注册链路仍正常
