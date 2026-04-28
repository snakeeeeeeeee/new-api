# 邀请返佣报表设计说明

## 背景

邀请关系已经可以通过新邀请码体系和后台手动绑定功能维护。运营侧需要按时间范围查看某个邀请人名下两级用户带来的充值、兑换、消费和预估返佣，用于人工周结或月结。

## 范围

本功能是“返佣报表”和“预估返佣”，不是自动结算系统。

- 不自动发放余额。
- 不生成返佣流水。
- 不记录已结算 / 未结算状态。
- 不做历史关系快照，按查询时的当前邀请关系实时统计。
- 最多统计两级邀请链：`A -> B -> C`。

## 统计关系

报表按新邀请码字段识别归属：

- 一级：`users.invite_code_owner_id = 查询用户 ID` 且 `invite_code_id > 0`
- 二级：`users.invite_code_owner_id IN 一级用户 ID` 且 `invite_code_id > 0`
- 三级及更深不计入

后台重绑邀请归属后，历史时间范围的报表也会按新的当前关系展示。

## 统计口径

充值、兑换和订阅分开展示：

- 钱包充值：成功 `top_ups`，排除成功订阅订单对应的 `trade_no`。
- 兑换码兑换：已使用 `redemptions`，按 `used_user_id` 和 `redeemed_time` 统计 `quota`。
- 邀请码注册送额度：不统计，不读取系统日志，也不读取 `invite_codes.reward_quota_per_use`。
- 订阅购买金额：成功 `subscription_orders.money`，单独展示。
- 钱包消费：`logs.type = consume`、`quota > 0`，且不是 `billing_source=subscription`。
- 订阅消费：`logs.type = consume`、`quota > 0`，且 `billing_source=subscription`。

后台赠送订阅 `user_subscriptions.source=admin` 的用量会展示为订阅消费，但预估返佣为 0。

## 返佣规则

全局规则存储在 `options.InviteCommissionSettings`，后台“邀请返佣管理 -> 规则配置”可编辑：

- 新增代理配置时默认带入的一级 / 二级返佣比例。
- 服务分类字典，例如 `gpt / GPT`、`claude / Claude`、`gemini / Gemini`，运营可继续新增 DeepSeek、Qwen 等服务分类。
- 订阅包用量档位，例如 `0-33%=15%`、`33-66%=7.5%`、`66-100%=0%`。
- 分组利润规则：
  - `group`：普通渠道分组或聚合分组名称。
  - `service`：从服务分类字典中选择，用于报表服务维度汇总。
  - `profit_rate_bps`：运营配置的毛利率。
  - `max_commission_rate_bps`：对外最高返佣比例。
  - `profit_share_rate_bps`：毛利可分出上限。
  - `profit_protection_enabled`：是否启用利润保护。

服务归类以分组利润规则为准，不再维护模型名通配归类规则，也不再维护旧版服务基准返佣比例。需要新增服务厂商时，先在服务分类字典中维护，再在分组利润规则中下拉选择。缺失快照或未配置分组的日志统一归入 `Other`，只展示消费，不产生 v2 预估返佣。

分组利润规则来源：

- 倍率分组来自系统设置里的 `GroupRatio`，也就是“分组与模型定价设置 -> 分组相关设置 -> 分组倍率”。
- 普通分组来自现有渠道 `channels.group`，支持逗号拆分。
- 聚合分组来自 `aggregate_groups.name`。
- 默认过滤 `default` 和 `UserGroup-*`，避免把用户分组当成渠道利润规则配置。

用户单独配置存储在 `invite_commission_user_configs`：

- 只有被添加到用户配置且 `enabled=true` 的用户，才被视为可返佣代理并计算预估返佣。
- 没有用户配置时，邀请、充值和消费统计继续展示，但预估返佣为 0。
- 用户配置关闭后，统计继续展示，预估返佣为 0。
- 全局默认一级 / 二级比例用于新增代理配置时预填，不代表所有邀请人默认都有返佣资格。

返佣公式：

- 理论返佣：`消费额度 * 分组最高返佣比例 * 层级比例`
- 订阅理论返佣：`订阅消费额度 * 订阅用量档位比例 * 层级比例`
- 毛利额度：`消费额度 * 分组毛利率`
- 利润保护上限：`毛利额度 * 利润分成上限 * 层级比例`
- 最终返佣：`min(理论返佣, 利润保护上限)`

订阅档位按每条订阅消费日志发生时的 `subscription_used / subscription_total` 匹配，不按查询截止时间动态重算。

## 利润快照

v2 从新消费日志开始写入利润快照，位置为 `logs.other.admin_info.commission`。

快照字段包括：

- `group`：用于返佣计算的分组；聚合分组请求使用聚合分组名。
- `route_group`：聚合分组实际命中的子分组，仅用于审计。
- `service`：按分组利润规则固化的服务类型。
- `configured`：写入时该分组是否已配置利润规则。
- `profit_rate_bps / cost_rate_bps`：毛利率和成本率。
- `max_commission_rate_bps`：最高返佣比例。
- `profit_share_rate_bps`：利润分成上限。
- `profit_protection_enabled`：是否启用利润保护。
- `revenue_quota / upstream_cost_quota / gross_profit_quota`：消费、成本和毛利额度。
- `source=snapshot`：标识来源为日志快照。

说明：

- v2 不处理老数据，不做历史回填。
- 没有利润快照的老日志只展示消费，不参与 v2 预估返佣，并计入缺失快照消费额度。
- 利润率是后台运营配置口径，不自动读取上游账单。
- 分组利润规则后续变化不会影响已经写入快照的新日志。
- `admin_info.commission` 只管理员可见，用户侧使用日志会过滤 `admin_info`。
- 未配置利润规则的分组仍写 `configured=false` 快照，但毛利和预估返佣按 0 处理。

## 接口

- `GET /api/invite_commission/self/report`
- `GET /api/invite_commission/admin/report`
- `GET /api/invite_commission/settings`
- `PUT /api/invite_commission/settings`
- `GET /api/invite_commission/user_configs`
- `PUT /api/invite_commission/user_configs/:user_id`
- `GET /api/invite_commission/group_profit_rules?keyword=`
- `PUT /api/invite_commission/group_profit_rules`
- `DELETE /api/invite_commission/group_profit_rules?group=`

管理员报表返回模型明细、被邀请用户明细、分组利润汇总、成本、毛利、理论返佣、利润保护上限、压低返佣和缺失快照消费额度；用户侧报表只返回简化汇总，不展示利润率、成本、毛利和利润保护细节。
