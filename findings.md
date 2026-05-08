# Claude Large Context Relay Profiling

## Requirements
- 需要区分上游服务慢与 new-api 本地处理慢。
- 大上下文测试不能只看请求体大小，必须看上游返回的真实 `usage` 字段是否被 new-api 记录。
- 真实 Anthropic cache 口径需要包含：
  - `input_tokens`
  - `cache_read_input_tokens`
  - `cache_creation_input_tokens`
  - `cache_creation.ephemeral_5m_input_tokens`
  - `output_tokens`

## Research Findings
- `relay/channel/claude/relay-claude.go` 的 `FormatClaudeResponseInfo` 会从 `message_start` / `message_delta` 读取 Claude usage。
- DTO `ClaudeUsage` 已包含 `CacheReadInputTokens`、`CacheCreationInputTokens`、`CacheCreation.Ephemeral5mInputTokens`。
- 本轮 fake upstream 应该模拟 SSE `message_start` 和 `message_delta` usage，否则最终消费日志不能代表真实 Claude cache 计费口径。
- 端到端验证确认 new-api 能按 Anthropic usage 记录 cache 字段：`cache_tokens=8459`、`cache_creation_tokens=5994`、`cache_creation_tokens_5m=5994`。
- 在本地 fake upstream 12ms 返回的场景中，3.2MB Claude 请求没有出现秒级 new-api 本地处理耗时；可见阶段主要是 request validate/token meta/sensitive/token estimate/remove disabled fields，各几十毫秒级。

---

# Findings & Decisions

## External Topup API

## Requirements
- 第三方系统调用 new-api 创建充值订单，支持 `user_id` 或 `username` 定位用户。
- 请求包含 `amount`、`payment_method`，可选 `external_order_no`、`callback_url`、`return_url`、`subject`。
- new-api 生成本地 `top_ups` pending 订单，并调用 pay-server 内部 `/api/v1/orders` 获取 checkout / QR 信息。
- pay-server 支付成功后按易支付商户通知格式回调 new-api，new-api 完成入账后再回调外部系统。
- 外部系统成功回调 payload 由 new-api 签名，便于第三方验签。

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| 不复用外部注册鉴权码，新增外部充值独立鉴权码 | 注册和充值权限不同，避免一个外部调用方拿到过大的能力 |
| 下单优先使用 pay-server 内部 `/api/v1/orders` JSON API | 相比易支付兼容 submit.php，能直接拿到 checkout_url、二维码图片/内容等结构化结果 |
| `top_ups` 增加外部元数据字段 | 不新增表即可记录外部订单号、回调地址、回调结果，便于后台账单检索 |
| 入账仍复用现有易支付回调验签 | pay-server 对 new-api 的商户通知已经按易支付格式签名，继续沿用现有信任边界 |
| 外部回调只在订单从 pending 变为 success 后触发 | 保证重复支付通知幂等，不对已成功订单重复通知或重复加额度 |
| `external_order_no` 使用 nullable unique index | 保证外部单号并发幂等，同时普通非外部订单可继续为空 |

## Research Findings
- `pay-server` 文档说明 new-api 当前应配置为 `PayAddress=https://.../newapi-pay`、`EpayId/PAY_COMPAT_PID`、`EpayKey/PAY_COMPAT_KEY`。
- `pay-server` 内部订单接口 `POST /api/v1/orders` 受 `x-service-token` 保护；未设置 `INTERNAL_SERVICE_TOKEN` 时放行。
- `pay-server` 内部订单响应包含 `checkout_url`、`payment_url`、`payment_qrcode_url`、`payment_qrcode_img`、`payment_qrcode_payload`、`qr_code_payload` 等字段。
- `pay-server` 支付成功后 `MerchantNotifyService` 会 POST `application/x-www-form-urlencoded` 到订单 `notifyUrl`，内容为易支付格式，返回 body 精确为 `success` 才认为通知成功。
- new-api 现有 `EpayNotify` 在验签后直接更新 `top_ups` 并给用户加额度，适合抽成可复用的完成函数后挂外部通知。
- 生产 pay-server 如果配置 `INTERNAL_SERVICE_TOKEN`，new-api 需要配置 `PayServerInternalToken`，下单请求会作为 `x-service-token` 发送。
- 外部成功回调使用 JSON POST，Header 带 `X-New-API-Event: topup.success`、`X-New-API-Trade-No`，配置 `ExternalTopupCallbackSecret` 后带 `X-New-API-Signature` HMAC-SHA256。

## External Register Auth Code

## Requirements
- 新增 `POST /api/user/external_register`，用 `username + password` 注册，`invite_code` 可选，不返回登录态或默认 token。
- 请求必须带 `Authorization: Bearer <鉴权码>`，并且全局外部注册开关开启、鉴权码已配置。
- 外部注册不受普通注册开关、邮箱验证、Turnstile 和密码注册开关影响。
- 复用 `InsertWithManagedInviteCode`，确保目标分组、邀请归属、奖励额度、新用户初始化和日志保持现有注册语义。
- root 可查看、追加生成、删除单个或全部外部注册鉴权码；生成自动开启，删除全部自动关闭。
- 鉴权码明文可在 root 专用接口查看，但必须从通用 `/api/option/` 列表过滤。
- 前端在系统设置的登录注册区域新增外部注册接口管理区。

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| 多个鉴权码继续存 `options` 表的 `ExternalRegisterAuthKey` JSON 数组 | 符合不新增表要求，简单支持多个调用方 |
| root 管理接口放在现有 `/api/option` 路由组 | 复用既有 RootAuth 策略和系统设置页面权限 |
| `invite_code` 可选且不接受 `aff_code` | 传邀请码时绑定归属人，不传时按普通新用户初始化 |
| 默认 token 生成抽成 helper | 普通注册和外部注册需要保持一致，避免复制两套 token 逻辑 |

## Research Findings
- 当前已有初版改动：`common/constants.go`、`model/option.go`、`controller/option.go`、`controller/user.go`、`router/api-router.go`、`web/src/components/settings/SystemSetting.jsx`。
- 当前已有新增测试文件 `controller/external_register_test.go` 和仿真脚本 `2dev/script/simulate_external_register.py`，尚待审查和运行。
- `task_plan.md` / `findings.md` / `progress.md` 原本仍是旧任务内容，本轮已在文件顶部追加外部注册计划。

---

## Aggregate Group Cluster Smart Strategy v2

## Requirements
- `cluster` 模式的智能策略支持降级窗口内继续触发递减降权。
- 降权比例继续使用全局配置，默认 20%，但不再只降一次。
- `degrade_level=0` 表示健康；`1` 表示首次降级；`2+` 表示降级窗口内重复达到失败/慢请求阈值。
- `failover` 继续把 degraded 子分组作为跳过对象，不参与递减降权。
- 首字慢阈值只对流式请求生效，默认 0 关闭。
- Claude CLI/client 专用池也必须使用同一套 cluster 递减有效权重。
- 拓扑图/运行态需要展示降级层级、降级期间失败/慢请求计数和慢请求原因。

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| 降级层级只存 Redis/内存策略状态，不新增 DB 字段 | 运行态信息，不需要持久迁移，降低上线风险 |
| 旧状态 `DegradedUntil > now` 且无 level 时视为 level 1 | 兼容已经存在的旧 Redis/memory 状态 |
| 有效权重按 `ceil(weight * pct^level)` 计算 | 递减清晰，低权重也能逐步收敛；正权重最低 1，0 权重仍为 0 |
| 降级期间成功但不慢不重置 level | 避免一次成功立刻恢复流量，恢复仍由 degraded 到期控制 |
| 首字慢通过 relay 记录的 first response latency 桥接到 Gin context | `RecordAggregateRouteSuccess` 在中间件层执行，需要从 relay 传递计时信息 |

---

## Aggregate Group Client Route Pool v1

## Requirements
- 在聚合分组 `failover` / `cluster` 两种模式下支持客户端专用流量池，并跟随当前聚合分组路由模式。
- 第一版只内置 `claude_code_cli`，默认关闭。
- Claude Code CLI 硬识别条件：请求路径 `/v1/messages`，模型名以 `claude-` 开头，`User-Agent` 包含 `claude-cli/`。
- `Anthropic-Beta` 只作为辅助特征，不把具体日期写死为硬条件。
- 专用池 target 独立于默认池，允许同一真实分组在两个池里配置不同顺序和权重。
- Failover 下专用池按 target 顺序形成独立链路；Cluster 下专用池按 target 权重加权分发。
- 专用池失败后先按当前子分组 `RetryTimes` 组内重试，耗尽后切同池其他 target；同池耗尽后按 `fallback_to_default` 决定是否回退当前模式默认池。

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| `client_route_pools` 使用 `aggregate_groups` 文本 JSON 字段 | 避免新增多张表，配置体积小，旧数据空字符串即可表示关闭 |
| 专用池候选使用 route pool + route group 作为请求内 attempt key | 专用池 target 可不在默认 `aggregate_group_targets`，不能只依赖默认 target index |
| CLI 专用池使用独立 route affinity key | 避免 CLI 粘性污染普通 cluster 的用户级亲和 |
| CLI 专用池使用独立 failover 运行态 key | 避免 CLI 链路切换后污染普通 failover 的链路起点 |
| CLI fallback 到默认池时不写普通 cluster affinity，也不更新默认 failover 运行态 | fallback 是临时兜底，不应改变普通请求后续落点 |
| 智能策略在专用池沿用 cluster 降权语义 | 与默认 cluster 一致，避免短暂波动导致流量全量切走 |

## Research Findings
- 当前 `selectAggregateGroupClusterChannel` 默认候选来自 `aggregateGroup.Targets`，并用 target index 做 retry/fallback 状态。
- `RecordAggregateRouteSuccess` 成功时会在 cluster 模式写默认 route affinity；CLI 专用池必须分流这个写入点。
- 运行态视图目前只遍历默认 targets；专用池需要额外 section，不能伪装成默认池 `weight=0`。
- 原 failover 运行态 key 是 `aggregate_group + model`；专用池支持 failover 后必须增加 pool 级 key，否则 CLI 池切换节点会影响普通默认链路。

---

## Aggregate Group Single-Channel Internal Retry

## Requirements
- `RetryTimes=2` 时，当前子分组首次失败后应继续重试当前子分组 2 次。
- 当前子分组仍失败后，才切换到下一个子分组。
- 该语义应同时适用于 `failover` 和 `cluster`。
- 单渠道 / 单 priority 子分组也要遵守当前子分组内部 retry 预算。

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| `RetryTimes` 直接作为当前真实分组内部预算 | 与用户理解一致：先重试当前子分组，再跨组 fallback |
| 不再用 `priorityRetry >= priorityCount` 判断当前真实分组耗尽 | 单渠道 / 单 priority 场景也需要重试当前真实分组 |
| 保留 `priorityCount <= 0` 作为硬不可用判断 | 没有可用 channel 时仍应跳过该子分组 |
| 依赖底层 `GetRandomSatisfiedChannel` 对超出 priority 的 retry 做 clamp | 既有选渠道逻辑已会收敛到最后一个可用 priority，无需新增重复选择器 |
| failover 与 cluster 同步修正 | 避免两个路由模式在组内 retry 语义上分叉 |

## Research Findings
- 原 cluster 代码在 `prepareAggregateClusterRetry` 中要求 `priorityRetry+1 < priorityCount` 才进入组内 retry。
- 单渠道 / 单 priority 子分组的 `priorityCount=1`，因此第一次失败后直接走跨子分组 fallback。
- failover 代码中也存在同类 priority 数量判断，需要同步修正。

---

## Token And Body Storage Performance Optimization

## Requirements
- 保留正常请求 token 统计、usage fallback 和最终扣费口径。
- OpenAI tokenizer 需要支持 request context cancellation。
- 非 OpenAI completion fallback 继续使用 estimator，但 estimator 热路径需要降 CPU。
- 大请求体仍按磁盘缓存阈值进入磁盘；内存路径减少重复大 slice 分配。
- 增加短压测和 sustained load 工具，观察 p95/p99、503、CPU/RSS/heap/goroutine/GC。

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| 保留 `CountTextToken`，新增 `CountTextTokenContext` | 兼容现有调用点，只在已有 request context 的路径启用取消 |
| `ResponseText2Usage` 使用 request context 调用 estimator | 正常请求输出不变，请求取消后停止无意义本地统计 |
| estimator 添加 ASCII fast path 和查表符号分类 | URL/英文长文本是高频 fallback 输入，避免 `unicode` 和线性扫描开销 |
| `CreateBodyStorageFromReader` 内存路径使用 pooled storage | 保留接口和磁盘阈值语义，同时降低反复读取 body 的分配和 GC 压力 |

## Research Findings
- `ResponseText2Usage` 当前直接调用 `EstimateTokenByModel`，不受 `CountToken=false` 影响。
- `EstimateRequestToken` 已经有 gin request context，适合切换到 `CountTextTokenContext`。
- Horizon tokenizer fork API 是 `Count(context.Context, string)` / `Encode(context.Context, string)`。
- 当前 `CreateBodyStorageFromReader` 未命中磁盘时使用 `io.ReadAll(io.LimitReader(...))`，会为每次请求分配完整 body slice。

---

## Aggregate Group And Channel Group Select UX

## Requirements
- 聚合分组列表需要直接标记当前使用的路由模式。
- 聚合分组“可见用户组”只显示 `default` 和 `UserGroup-*`。
- 聚合分组“添加真实分组”过滤掉 `default` 和 `UserGroup-*`。
- 聚合分组可见用户组、真实分组和渠道新增/编辑里的分组选择都需要支持搜索。
- Failover / Cluster 切换需要确认，避免误操作。

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| 聚合分组列表在名称列增加模式 Tag | 不新增复杂列宽，管理员扫描列表即可看到当前路由模式 |
| 分组选项在聚合分组页面加载后按名称前缀分流 | 不改后端接口，复用 `/api/group/` 数据，风险最低 |
| Select 统一使用 `selectFilter` | 与项目内模型、token 等选择器搜索逻辑保持一致 |
| 模式切换使用 `Modal.confirm` | 切换 tab 前确认，取消时 active tab 保持原模式 |

## Research Findings
- 聚合分组列表和编辑入口都在 `web/src/components/table/aggregate-groups/index.jsx`。
- 聚合分组编辑抽屉已有 `routing_mode` 受控 Tabs，适合在 `onChange` 前增加确认。
- 渠道新增/编辑共用 `web/src/components/table/channels/modals/EditChannelModal.jsx` 的 `Form.Select field='groups'`。

---

## Aggregate Group Edit UI Mode Tabs

## Requirements
- 聚合分组新增/编辑页面需要降低配置混乱感。
- Failover 和 Cluster 的配置项按路由模式分开展示。
- 权重只在 Cluster 模式下可编辑，并在 UI 上说明“相对流量比例”的含义。
- Failover 模式继续表达 `A -> B -> C` 顺序链路，不展示权重输入，避免误以为权重在 failover 生效。

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| 基础信息和路由模式配置拆成两个区块 | 基础字段与路由策略字段职责不同，拆开后保存语义更清楚 |
| 使用 `Tabs` 切换 `routing_mode` | 管理员能直观看到当前正在配置哪种模式，同时减少同屏字段数量 |
| target 列表复用同一份数据，但按模式改变展示 | 保持 payload 和后端结构不变，降低回归风险 |
| Cluster 下显示权重输入，Failover 下隐藏 | 权重只参与 cluster 加权分发，隐藏比禁用更不容易误解 |
| Cluster 说明权重比例与 `weight=0` 语义 | 避免运营看到数字不知道含义 |

## Research Findings
- 当前编辑弹窗集中在 `web/src/components/table/aggregate-groups/EditAggregateGroupModal.jsx`。
- 保存 payload 已同时包含 `routing_mode`、`cluster_affinity_ttl_seconds`、`retry_status_codes` 和 targets 的 `weight`，本次无需改 API。
- 现有 `cluster_affinity_ttl_seconds` 已与 `recovery_interval_seconds` 解耦，适合分别放入 Cluster 和 Failover tab。

---

## Cluster Affinity TTL Config

## Requirements
- 每个聚合分组新增 `Cluster 亲和保持时间（秒）`。
- 默认值为 300 秒。
- 仅 `routing_mode=cluster` 时生效；failover 下不改变现有恢复间隔语义。
- 新增/编辑聚合分组时可配置，但 UI 只有选择 Cluster 模式时允许修改。
- route affinity 按用户级优先：同一聚合组同一用户尽量留在同一子分组，模型只用于校验该子分组是否可用。

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| 新增字段 `cluster_affinity_ttl_seconds` 到 `aggregate_groups` | 与 failover 的 `recovery_interval_seconds` 解耦，避免一个字段承担两种语义 |
| 默认 300 秒，校验必须大于 0 | 满足用户默认要求，并避免禁用 TTL 导致缓存默认值不清晰 |
| route affinity TTL 优先使用新字段 | Cluster 亲和保持时间成为明确配置 |
| failover 下仍保留字段值但不生效 | 方便切换回 cluster 时保留配置，同时不影响旧逻辑 |
| route affinity key 使用 `aggregate_group + user_id` | 让同一用户在 sonnet/opus 等模型间切换时尽量留在同一子分组 |

## Research Findings
- 当前 route affinity TTL 由 `ContextKeyAggregateRecoveryInterval` 推导，实际来源是聚合分组 `recovery_interval_seconds`。
- `RecordAggregateRouteAffinity` 只在 `routing_mode=cluster` 的成功路径调用。
- 聚合分组新增/编辑前端集中在 `EditAggregateGroupModal.jsx`。

---

## Invite Commission Profit Protection v2

## Requirements
- 新增分组利润规则，默认过滤 `default` 和 `UserGroup-*`。
- 普通分组来自 `channels.group` 拆分去重，聚合分组来自 `aggregate_groups.name`。
- 新消费日志写入 `logs.other.admin_info.commission` 利润快照。
- 报表按快照计算成本、毛利、理论返佣、利润保护上限和最终返佣。
- 老日志缺失快照不参与 v2 返佣，不做历史回填。
- 用户侧不能看到利润率、成本、毛利等内部经营字段。

## Research Findings
- 当前返佣规则存储在 `options.InviteCommissionSettings`，已有 settings 接口和前端规则配置页。
- 当前消费日志 `logs.other` 已有 `admin_info`，用户侧 `formatUserLogs` 会删除 `admin_info`。
- 文本、音频、WSS、任务和 MJ 消费日志都通过 `RecordConsumeLog` / `RecordTaskBillingLog` 写入，适合在日志写入层统一补快照。
- 聚合分组场景下 `relayInfo.UsingGroup` 可能是最终子分组，但上下文 `ContextKeyAggregateGroup` 能拿到聚合分组名，`ContextKeyRouteGroup` 能拿到实际子分组。

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| 利润规则继续存入 `InviteCommissionSettings` | 避免新增表和迁移，符合现有全局规则配置方式 |
| 快照放入 `admin_info.commission` | 用户日志已有过滤，减少隐私泄漏风险 |
| 缺失快照的老日志不计算返佣 | 用户明确暂不考虑老数据，避免历史估算漂移 |
| 聚合分组按聚合分组规则计算，route group 只记录审计 | 与计划一致，运营配置简单 |
| 老日志缺失快照时返佣为 0 | v2 不回填老数据，避免规则变化导致历史估算漂移 |
| 未配置分组仍写 `configured=false` 快照 | 便于管理员看到消费发生在未配置分组，但不产生预估返佣 |
| 删除 `service_rates` 和 `model_rules` | v2 已由分组利润规则明确服务和返佣上限，继续保留模型名归类和服务基准会造成配置膨胀和口径混乱 |
| 新增 `service_categories` 字典 | 服务厂商需要提前维护并在分组利润规则中下拉选择，避免在每条分组规则里临时输入导致命名不一致 |

## Implementation Findings
- `RecordConsumeLog` 和 `RecordTaskBillingLog` 是统一写入利润快照的主入口。
- 任务计费、违规扣费和 MJ 代理消费需要先把聚合分组信息写入 `other.admin_info`，快照才能拿到聚合分组和实际 `route_group`。
- 原 v1 单测直接插入 `logs`，这些日志没有快照；v2 下应作为老日志处理，只统计消费不返佣。
- 用户侧日志格式化已经会删除 `admin_info`，新增利润快照不会暴露到普通用户日志接口。

## Requirements
- 管理员可在用户管理中手动将某个用户归属到目标邀请人名下。
- 支持选择目标邀请人的已有邀请码；未选择时自动创建或复用 `MANUAL-<owner_user_id>`。
- 只做统计归属：不补发奖励、不增加 `reward_used_uses`、不改 `aff_quota` / `aff_history` / `aff_count`。
- 支持重绑覆盖和解绑清零。
- 每次绑定、重绑、解绑必须写管理日志，记录旧/新邀请人和旧/新邀请码。

## Research Findings
- `model/invite_code.go` 中 `GetInviteStatsByOwnerUserIDs` 和 `GetInviteeSummariesByOwnerUserID` 都过滤 `invite_code_id > 0`，只改旧字段 `inviter_id` 不会进入新统计。
- 邀请码软删除使用 GORM `DeletedAt`，`GetInviteCodesByOwnerUserID` 会 `Unscoped()` 返回软删除码以便用户端展示历史状态；手动绑定应使用普通 scoped 查询，默认排除已删除码。
- 注册路径 `InsertWithManagedInviteCode` 会设置 `InviterId`、`InviteCodeId`、`InviteCodeOwnerId` 并调用 `ApplyInviteCodeRewardTx`；手动绑定不能复用该奖励路径。
- 管理日志现有入口是 `model.RecordLog(userId, model.LogTypeManage, content)`，用户管理中修改额度和清理绑定已有类似用法。
- 用户管理表格操作区在 `web/src/components/table/users/UsersColumnDefs.jsx`，表格容器在 `UsersTable.jsx`，已有 `refresh` 可用于操作成功后刷新当前列表。
- 前端已有 `/api/user/search` 可按 ID / 用户名等搜索用户；邀请码管理只有全局搜索接口，缺少按 owner 查询的后台接口。

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| 使用 GORM 事务实现绑定/解绑和 MANUAL 码查找/创建 | 保持跨 SQLite/MySQL/PostgreSQL 兼容，避免部分更新 |
| 指定邀请码用 scoped `First` 校验 | 默认拒绝已删除邀请码，符合手动绑定不指向删除码的建议 |
| 补充 `/api/user/:id/invite_codes` 管理接口 | 弹窗选择目标邀请人后只需要该 owner 的邀请码列表，放在 user admin 路由便于用户管理功能调用 |

## Issues Encountered
| Issue | Resolution |
|-------|------------|
| 复杂模拟恢复脚本中 GPT token 写短，导致 401 `无效的令牌` | 终止脚本，改为从 `tokens.key` 查询实际 48 位 key；失败请求没有写入消费日志 |
| 订阅生图请求可能预扣较大额度 | 仅将本轮演示订阅包和订阅实例额度调到 `10000000`，用于覆盖低/中/高用量档位 |

## Invite Commission Runtime Findings
- `commission_complex_20260427a` 数据已保留在 Docker dev 数据库。
- 45 条真实消费日志均成功写入，ID 范围为 `541`-`585`。
- 报表服务归类按模型名生效：
  - `gpt-image-2` -> GPT
  - `claude-sonnet-4-6` -> Claude
  - `gemini-3.1-flash-image` -> Gemini
- A owner `993100` 只统计 B/C1/C2，不统计 D1/D2，证明第三级被排除。
- B owner `993101` 使用用户自定义比例 `800`/`300` bps，覆盖全局默认 `500`/`150` bps。
- 钱包充值、兑换码、订阅购买展示值和独立重算一致；订阅购买对应 `top_ups.trade_no` 没有混入普通钱包充值。
- 订阅消费按日志里的 `subscription_used / subscription_total` 匹配档位，接口返回和按日志逐条四位小数舍入后的重算一致。

## Resources
- `/Users/zhangyu/code/go/new-api/AGENTS.md`

## Visual/Browser Findings
- 尚未使用浏览器或图像查看。

## Horizon Investigation

## 15m Sustained Load Findings
- Local sustained run used fake OpenAI-compatible streaming upstream with no usage, `concurrency=20`, `body-size=50KB`, `response-size=200KB`, duration `900s`.
- Result: `17803 / 17803` succeeded, with `status_503=0`, `system_cpu_overloaded=0`, and `errors=0`.
- Consume logs for the formal 15m window all had `completion_tokens=100836`, proving the response-side usage fallback path was hit rather than bypassed.
- Heap/RSS/goroutine did not grow monotonically:
  - heap alloc peaked around `30.35MB`, final total `18.83MB`, idle-after-run `10.98MB`;
  - RSS peaked around `96.37MB`, last sampled `76.77MB`, idle-after-run about `66.6MB`;
  - goroutines peaked around `136`, then returned to `22`.
- Memory buffer pool drained after run (`memory_buffers=0`), and disk cache files stayed `0` for this 50KB request-body scenario.
- Important caveat: p99 showed periodic `~68s` stream tails and several zero-completion windows where all workers were waiting. Since resource metrics stayed flat and no 503 occurred, this looks separate from the original CPU-overload leak/pressure issue, but it should be investigated with a Go fake upstream or real incident sample before making a tail-latency SLA claim.

## Research Findings
- `Calcium-Ion/new-api-horizon` README states Horizon is not open source and points users to Docker images only.
- Git refs for `Calcium-Ion/new-api-horizon` show tags and a `main` branch but do not expose application source code in this repository.
- Horizon release notes advertise performance-oriented changes, including streaming request/response optimizations and `0.4.0-alpha.1` claiming at least 50% CPU and memory savings under large request-body high-concurrency scenarios.
- Docker Hub API to `hub.docker.com` timed out from this environment; next step is using registry tooling (`crane`) against the image directly.
- Horizon Docker image `calciumion/new-api-horizon:latest` exposes build labels for version `0.5.5`, revision `f0c193c45034a56d4760c74769500b3499f03537`, and source `https://github.com/Calcium-Ion/new-api-horizon-repo`; the image contains a stripped Go binary, not application source.
- `go version -m /tmp/horizon-image/new-api` shows Horizon replaces `github.com/tiktoken-go/tokenizer v0.6.2` with `github.com/Calcium-Ion/tokenizer v0.0.1`.
- The `github.com/Calcium-Ion/tokenizer` fork changes tokenizer APIs to `Count(context.Context, string)` and `Encode(context.Context, string)`, adding `ctx.Done()` cancellation checks inside `tokenize` and `mergePairs`.
- Horizon binary strings include `StreamingPerformanceOptimization`, `json:"streaming_performance_optimization"`, `performance.streaming_performance_optimization`, disk cache settings, and UI text `Content is large, performance optimization mode enabled`; the local binary does not include `StreamingPerformanceOptimization`.
- Local `CountToken=false` only disables request-side `EstimateRequestToken`; response fallback still calls `ResponseText2Usage` and `EstimateTokenByModel`, including Gemini/Claude stream fallback paths.
- GoReSym/redress can recover package/function/type metadata from Horizon despite stripped symbols, but not original Go source. This is enough to infer implementation shape, not exact source code.
- Redress recovered `performance_setting.PerformanceSetting` with `StreamingPerformanceOptimization`, `DiskCacheEnabled`, `DiskCacheThresholdMB`, `DiskCacheMaxSizeMB`, `DiskCachePath`, and monitor fields.
- Horizon adds `common.pooledMemoryStorage` with `Read`, `Seek`, `Close`, `Bytes`, `Size`, and `IsDisk` methods. Local inspected binary only has `memoryStorage`/`diskStorage`.
- Horizon `common.CreateBodyStorageFromReader` is much larger than local inspected binary (`2032` bytes vs `848` bytes by GoReSym function span), supporting a more complex streaming/body-storage path.
- Horizon `service.getTokenNum` is recovered as a distinct 832-byte function with nested closures and a closure type containing `tokenizer.Codec`, `context.Context`, input string, `chan int`, and `chan error`, consistent with async/context-cancellable tokenizer counting.

---

## Image Generation Large Base64 Response Investigation

## Requirements
- 判断 `gpt-image-2` 等图片生成模型返回大体积 `b64_json` 时，new-api 是否可能导致 HTTP 响应截断。
- 用户观察：4K 成功生成图片约 8MB，返回 base64 JSON 后客户端加载不出图。

## Research Findings
- `/v1/images/generations` 路由进入 `relay.ImageHelper`，OpenAI channel 的图片生成响应默认走 `OpenaiHandlerWithUsage`。
- `OpenaiHandlerWithUsage` 当前使用 `io.ReadAll(resp.Body)` 完整读取上游响应，再 `common.Unmarshal` 解析 usage，最后 `service.IOCopyBytesGracefully` 一次性写回客户端。
- `service.IOCopyBytesGracefully` 会复制上游响应头但跳过上游 `Content-Length`，然后按 `len(responseBody)` 设置新的 `Content-Length` 并 `io.Copy` 到 Gin writer。
- 代码中没有发现针对图片生成响应的 8MB 或 10MB 固定响应体限制；Gin `server.Run` 也未设置 `WriteTimeout`。
- `/v1` relay 路由没有 gzip 中间件，base64 JSON 会原样返回；8MB 二进制图片变成 base64 后约 10.7MB，再加 JSON 字段。
- 新增本地单元测试用 11MiB `b64_json` 模拟响应，`adaptor.DoResponse` 在 `httptest` 下可完整写回并设置正确 `Content-Length`。
- 若客户端、反向代理或 CDN 在写回阶段断开，`IOCopyBytesGracefully` 只记录 `failed to copy response body`；外层业务仍会继续按已收到的 usage 扣费。
- OpenAI 官方 Images API 文档显示，GPT image 模型默认返回 `b64_json`，`url` 返回只适用于 DALL-E；图片 streaming 会发送 partial/completed 事件，usage 信息出现在 GPT image 模型的流式完成事件里。
- 当前 `ImageRequest.Stream` 原先被注释，导致 `stream:true` 不会让 relay 标记为流式；本轮已改为 `*bool`，保留显式 false/true 语义。
- OpenAI 图片 SSE 响应现在由 `OpenaiImageStreamHandler` 透传，并从 `image_generation.completed` / `image_edit.completed` 提取 usage 用于后续扣费。
- Docker dev live test against channel 9 (`gpt-image-2自建`) with `size=3840x2160` succeeded once through `POST /v1/images/generations`; returned `Content-Length=5106057`, curl downloaded exactly `5106057` bytes, JSON parse succeeded, `b64_json` length was `5105432`, base64 decode succeeded, and decoded PNG dimensions were `3840x2160`.
- The successful live response did not show HTTP truncation in new-api logs: no `bad_response_body`, `failed to copy response body`, `broken pipe`, `connection reset`, or `unexpected EOF` for request id `20260506110157446567795PjGDgsGR`.
- Two attempts to force a larger 4K response with high-detail prompts failed upstream with `502 Upstream request failed` after about 63-70s; new-api returned complete 143-byte error JSON. These failures were not evidence of response truncation.
- Live caveat: the successful 4K image was a highly compressible PNG (`3829074` decoded bytes), producing about `5.1MB` JSON rather than the user's observed `~8MB` decoded image / `~10.7MB+` base64 JSON. The current live test verifies the local app path for ~5MB JSON, while an 8MB decoded-image case still needs a successful upstream generation or a controlled upstream fixture in Docker dev.

## Technical Decisions
| Decision | Rationale |
|----------|-----------|
| 保留大响应单元测试 | 证明应用层没有固定 8MB 截断，并防止以后引入响应大小回归 |
| 支持 OpenAI 图片流式透传 | 官方流式 partial image 是缓解大图一次性 base64 JSON 等待/断链风险的最小改动，不引入文件存储服务 |

## Issues Encountered
| Issue | Resolution |
|-------|------------|
| `rg` 搜索命令误包含未转义 `*_test.go`，zsh 报 `no matches found` | 后续使用明确路径或引号/rg glob |

## Resources
- `/Users/zhangyu/code/go/new-api/relay/image_handler.go`
- `/Users/zhangyu/code/go/new-api/relay/channel/openai/relay-openai.go`
- `/Users/zhangyu/code/go/new-api/service/http.go`
- `/Users/zhangyu/code/go/new-api/relay/channel/openai/adaptor_image_test.go`
- OpenAI Images guide: https://platform.openai.com/docs/guides/image-generation
- OpenAI Images API reference: https://platform.openai.com/docs/api-reference/images
- OpenAI image streaming reference: https://platform.openai.com/docs/api-reference/images-streaming/image_generation

---

# 日志看板耗时分析 Findings

- logs 已有 use_time、channel_id、group、model_name。
- /api/log/dashboard 当前由 service/log_dashboard.go 内存聚合近 1h/6h/24h 窗口。
- 现有 dashboard 查询缺 model_name，需补字段。
- 前端日志看板使用 Semi UI Card/Tabs、VChart 和 CardTable，耗时区应复用这些模式。
- `use_time` 是秒级字段；本轮只统计最终成功请求，不采集 TTFT。
- service 测试环境此前未初始化 logGroupCol，dashboard 查询入口现在会确保日志列名已初始化。
