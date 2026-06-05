# Task Plan: 聚合子分组亲和按模型隔离

## Goal
为聚合分组新增 `route_affinity_scope`，控制 Cluster 亲和缓存是否跨模型共享；默认 `shared` 保持旧逻辑，新增 `model` 用于同一用户/请求标识在不同模型下分别选择和固定子分组。

## Current Phase
Phase 4 complete

## Phases
- [x] Phase 1: 后端字段、API、保存校验和 admin info
- [x] Phase 2: 亲和缓存 key 按 scope 生成，默认保持旧 key 格式
- [x] Phase 3: 前端编辑表单增加亲和范围选择并补 i18n key
- [x] Phase 4: 后端单测、前端构建和 diff 检查

## Verification
- Focused service affinity regression passed:
  - `go test ./service -run 'AggregateCluster(ModelAffinityScope|RequestFirstModelScope|ClaudeCLIPoolModelAffinityScope|RouteAffinityFollowsUserAcrossSupportedModels|RouteAffinitySkipsUserRouteWhenModelUnsupported)|AggregateRouteAffinityTTL|AggregateClusterRequestOnlyAffinity|AggregateClusterRequestFirstFallsBack|AggregateClusterCustomHeaderRouteAffinity' -count=1`
- `go test ./model ./service ./controller -count=1`: passed.
- `go test ./model ./service ./controller ./middleware -count=1`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie/chunk-size warnings.
- `git diff --check`: passed.

## Key Constraints
- 空值或旧数据归一化为 `shared`，不改变老聚合组路由行为。
- `shared` scope 保持旧亲和 key 格式，不包含模型。
- `model` scope 在亲和 key 中加入当前请求模型；route pool 维度仍保持原有隔离。
- `off` 亲和策略下不使用亲和，scope 不参与路由。
- 子分组 RPM 软限制仍按 `aggregate_group + real_group` 总量计算，不随模型拆分。

---

# Task Plan: 聚合子分组软 RPM 总量限制

## Goal
为聚合分组每个真实子分组增加软 RPM 总量限制，默认不限制；路由选择阶段跳过已达到总量 RPM 上限的子分组，并在运行态拓扑中实时展示总 RPM、上限和限制状态。

## Current Phase
Phase 4 complete

## Phases
- [x] Phase 1: 后端字段、API、总量 RPM 统计和路由过滤
- [x] Phase 2: 后端单元测试
- [x] Phase 3: 前端编辑表单、拓扑展示和 5 秒轮询
- [x] Phase 4: Go/frontend/Docker dev 验证

## Verification
- `go test ./model ./service ./controller ./middleware`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie/chunk-size warnings.
- `git diff --check`: passed.
- Docker dev rebuild/health passed:
  - `docker compose -f docker-compose-dev.yml up -d --build new-api-dev`
  - `curl -fsS http://localhost:3001/api/status`: passed.
  - `new-api-dev` healthy on `127.0.0.1:3001`.
- Docker dev business smoke passed with temporary aggregate group:
  - first request selected `rpm_limit=1` primary route.
  - second request within the 60s window skipped primary and selected unlimited secondary route.
  - runtime API returned primary `total_rpm=1`, `rpm_limit=1`, `rpm_limited=true`; secondary `total_rpm=1`, `rpm_limit=0`, `rpm_limited=false`.
  - temporary users/tokens/channels/aggregate group/fake upstream/RPM keys cleaned up.

## Key Constraints
- `rpm_limit <= 0` 表示不限制，老数据默认不限制。
- 限制按 `aggregate_group + route_group` 总量生效，不区分模型和流量池。
- 同一聚合分组内同一真实分组跨流量池重复配置时，RPM 上限必须一致。
- 软限制只跳过候选，不做原子预占用、不主动返回 429。

---

# Task Plan: 邀请统计 v1.1 拆分余额/订阅消费 + 订阅购买

## Goal
扩展邀请统计和用户管理邀请信息：用户管理首屏快速返回并异步补齐余额/订阅消费拆分；邀请统计页同时展示余额消费、订阅额度使用和订阅包成功购买金额。

## Current Phase
Phase 4 complete

## Phases
- [x] Phase 1: 后端聚合模型、接口、路由和索引
- [x] Phase 2: 前端用户管理异步拆分展示
- [x] Phase 3: 前端邀请统计订阅使用/购买展示与 i18n
- [x] Phase 4: 单测、构建和回归验证

## Verification
- `go test ./model ./controller`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie/chunk-size warnings.
- `git diff --check`: passed.

## Key Constraints
- 用户管理列表不能同步等待日志累计拆分。
- 消费拆分基于 `logs.other.billing_source`，历史空值按余额/未标记归类。
- 订阅购买收入只来自成功 `subscription_orders.money`，不把订阅额度使用当真实收入。
- JSON 解析继续使用 `common.*`/`common.StrToMap`，不使用数据库 JSON 函数。

---

# Task Plan: 邀请统计 v1 余额消费报表

## Goal
新增管理员邀请统计页面和 `/api/invite_code/consumption` 接口，按邀请人用户名和日期范围统计直接邀请用户的余额消费，排除订阅包消费。

## Current Phase
Phase 4 complete

## Phases
- [x] Phase 1: 后端接口与聚合逻辑
- [x] Phase 2: 前端页面、路由和侧栏入口
- [x] Phase 3: i18n、格式化和编译修复
- [x] Phase 4: `go test ./model ./controller` 与 `cd web && bun run build`

## Verification
- `go test ./model ./controller`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie/chunk-size warnings.
- `git diff --check`: passed.
- `docker compose -f docker-compose-dev.yml up -d --build`: passed; `new-api-dev` healthy on `127.0.0.1:3001`.
- Docker dev HTTP verification passed against `/api/invite_code/consumption` with seeded invite users/logs:
  - direct invitees counted: 2
  - wallet quota: 6000, wallet requests: 3, model count: 2
  - subscription quota excluded: 9000, excluded requests: 1
  - non-invited and out-of-range logs excluded
  - temporary seed data and temporary root access token cleaned up.

## Key Constraints
- v1 只统计余额/充值额度消费，不统计订阅包内部额度消费。
- 日志 `other` 解析在 Go 侧完成，不使用数据库 JSON 函数。
- 路由 `/api/invite_code/consumption` 必须位于 `/:id` 前。
- 日期范围由前端 DatePicker 日历范围转换为本地日初/日末 Unix 秒。

---

# Task Plan: User Aggregate Group Ratio Overrides

## Goal
Implement per-user aggregate-group ratio overrides with admin UI, billing/display integration, unit tests, frontend build, and Docker dev business regression.

## Current Phase
Phase 6 complete

## Phases
- [x] Discovery and work setup
- [x] Backend implementation
- [x] Frontend implementation
- [x] Go/frontend verification
- [x] Docker dev business regression
- [x] Delivery review
- [x] Optimization: filter override-configurable aggregate groups by target user visibility and improve selector display

## Verification
- `go test ./service ./controller`: passed.
- `go test ./...`: passed.
- `cd web && bun run build`: passed with existing Browserslist/chunk warnings.
- Optimization targeted test `go test ./controller -run 'UserAggregateGroupRatioOverrides|AggregateRatioOverride|GetUserGroupsReturnsAggregateRatioOverrideDetails' -count=1`: passed.
- Optimization full regression `go test ./...`: passed.
- Optimization frontend build `cd web && bun run build`: passed with existing Browserslist/lottie/chunk warnings.
- `git diff --check`: passed.
- Docker dev regression passed with mock upstream:
  - no override aggregate group: quota 300, group ratio 2
  - aggregate override 0.5: quota 75, group ratio 0.5, override fields in log
  - real group default: quota 150, group ratio 1, no override fields
  - log `other` includes `original_group_ratio`, `original_ratio`, `ratio_override`, and `has_ratio_override`

---

# Task Plan: 风险检测与命中拦截 v1

## Goal
新增管理员“风险检测”菜单，支持配置安全风控词和命中策略；relay 请求命中后记录 `violation_logs`，并按策略放行、拦截或累计达到阈值后禁用账号。未命中请求不查库、不写库，默认关闭。

## Current Phase
Phase 4 complete

## Phases
### Phase 1: Discovery & Design Anchoring
- [x] 复核 relay 插入点、错误返回、配置系统、用户缓存和前端菜单/路由。
- [x] 确认检测放在 `GetTokenCountMeta()` 后、预扣费前，复用 `meta.CombineText`。
- **Status:** complete

### Phase 2: Backend
- [x] 新增 `violation_logs` 模型、查询/清理/计数函数和启动迁移。
- [x] 新增 `violation_setting` 配置、规范化和管理员 API。
- [x] 新增检测服务：内存关键词匹配、命中片段、日志写入、阈值封禁、拦截错误。
- [x] 在 relay 预扣费前接入检测，确保命中拦截不扣费、不上游、不重试。
- **Status:** complete

### Phase 3: Frontend
- [x] 新增 `/console/violation` 管理页。
- [x] 新增侧边栏“风险检测”和模块开关。
- **Status:** complete

### Phase 4: Tests & Verification
- [x] 补 model/service/controller 测试。
- [x] `go test ./model ./service ./controller`。
- [x] `go test ./...`。
- [x] `cd web && bun run build`。
- [x] `git diff --check`。
- **Status:** complete

## Key Constraints
- 检测关闭时只做廉价开关判断。
- 开启但未命中时只做内存匹配，不访问数据库。
- 只保存上下文片段，不保存完整 prompt。
- 封禁计数不使用时间窗口，只在命中后按当前保留日志累计计数。
- DB 兼容 SQLite/MySQL/PostgreSQL；JSON 内容存 TEXT。
- JSON marshal/unmarshal 使用 `common.*` 包装。

---

# Task Plan: 数据库原子扣费防超扣

## Goal
以最小改动防止高并发下用户钱包额度和 token 剩余额度被扣成负数。扣费准入以数据库条件原子更新为准；Redis 只做缓存更新；统计、日志和报表不纳入本阶段。

## Current Phase
Phase 4 complete

## Phases
### Phase 1: Discovery
- [x] 确认用户余额、token 余额扣减入口和 BillingSession 信任旁路位置。
- [x] 确认现有测试 DB 初始化方式与可复用 seed/readback helper。
- **Status:** complete

### Phase 2: Implementation
- [x] 用户余额扣减改成 `quota >= amount` 条件原子更新。
- [x] token 扣减改成有限额 token 的 `remain_quota >= amount` 条件原子更新。
- [x] 余额扣减绕过 BatchUpdate，并在 DB 成功后再更新 Redis 缓存。
- [x] 钱包普通请求关闭 trust bypass。
- **Status:** complete

### Phase 3: Tests
- [x] 添加用户/token 不足与并发扣减单测。
- [x] 添加无限额 token 与 BillingSession 额度不足语义单测。
- **Status:** complete

### Phase 4: Verification
- [x] Focused Go tests。
- [x] `go test ./...`。
- [x] `git diff --check`。
- [x] Docker dev build、启动和 health/log 检查。
- **Status:** complete

## Key Constraints
- 不重构统计、日志、消费报表。
- JSON 规则不相关，本轮不新增 JSON 调用。
- DB 逻辑必须兼容 SQLite/MySQL/PostgreSQL。
- 不触碰现有未跟踪 probe/tmp/output 文件。

---

# Task Plan: Relay Error Passthrough Keyword Blocklist

## Goal
在现有「错误响应设置」中新增一行一个的错误透传阻断关键词。状态码允许透传时，如果原始上游错误内容命中任意关键词，则返回通用错误，不向客户透传账号/用量类上游文案；默认空配置，升级后不改变现有行为。

## Current Phase
Phase 3 complete

## Phases
### Phase 1: Discovery
- [x] 重新读取当前提交后的错误响应配置、controller 判断、option 更新和前端设置页。
- **Status:** complete

### Phase 2: Implementation
- [x] 新增 `relay_error_setting.passthrough_block_keywords` 配置字段。
- [x] 在状态码透传判断后增加大小写不敏感的包含匹配阻断。
- [x] 后台设置页新增多行输入框，一行一个关键词。
- **Status:** complete

### Phase 3: Verification
- [x] Focused Go tests。
- [x] 相关 Go package tests。
- [x] Frontend build。
- [x] Docker dev build、health 和日志检查。
- **Status:** complete

## Key Constraints
- 默认值为空，不能改变现有错误透传行为。
- 不新增数据库表或字段，只增加现有 `options` 表中的 key/value 配置项。
- 关键词只用于阻断透传，不影响渠道禁用、重试、计费和请求转发。
- 匹配原始上游错误内容，先匹配再按旧逻辑决定是否脱敏展示。

---

# Task Plan: Dump 分析与内置 Console

## Goal
新增管理员“Dump 分析”页面和 admin-only API，临时抓取指定用户请求并输出到服务日志和内存 Console；支持 grep 式关键词过滤和日志级别选择。Dump 必须旁路失败，不能改变用户请求、鉴权、计费、路由、重试、上游转发或业务响应。

## Current Phase
Phase 4 complete

## Phases
### Phase 1: Backend Dump Service
- [x] 新增进程内 Dump rule/status/events 服务、ring buffer、过期/max_count 自动停用。
- [x] 新增 admin-only status/start/stop/events/clear API。
- [x] 过滤敏感 Header，跳过 multipart/audio/image/video/octet-stream body。
- [x] 新增关键词匹配、大小写选项和日志级别。
- **Status:** complete

### Phase 2: Relay Hooks
- [x] relay 分发后记录 raw_request。
- [x] 上游转换后可选记录 upstream_request。
- [x] relay_error 记录状态码、错误码、渠道和必要 raw body。
- [x] Dump 入口 recover，失败只写诊断日志。
- **Status:** complete

### Phase 3: Frontend
- [x] 新增 `/console/request-dump` 管理员页面和侧边栏入口。
- [x] 规则表单支持用户 ID、模型、路径、聚合分组、关键词、日志级别和打印开关。
- [x] Token ID 放入高级可选过滤。
- [x] Console 支持轮询、暂停、跟随、清空、复制和本地搜索。
- **Status:** complete

### Phase 4: Verification
- [x] `go test ./...`。
- [x] `cd web && bun run build`。
- [x] `docker build -t new-api-local:dev .`。
- [x] `docker compose -f docker-compose-dev.yml up -d --force-recreate new-api-dev postgres-dev redis-dev`。
- [x] `python3 2dev/script/simulate_request_dump.py` 真实网关仿真通过。
- [x] 清理临时 DB/Redis 数据，确认 `new-api-dev` healthy。
- **Status:** complete

## Key Constraints
- Dump 不落 DB，不写 Redis；Console 只保留当前进程内 ring buffer。
- `user_ids` 必填，其他过滤均可选。
- 关键词是后端过滤：未命中不写服务日志、不进 Console、不消耗 `max_count`。
- `debug` 日志级别由管理员显式选择时强制输出 request_dump，不受全局 Debug 开关影响。
- Token ID 是内部 API Token 记录 ID，仅用于高级精确过滤。
- 关闭或重新启动规则后，旧规则构建中的事件不会写入新规则 Console。

---

# Task Plan: 聚合分组百分比智能降权

## Goal
把聚合分组智能降权从连续失败/慢请求次数触发改为滑动窗口百分比触发，并支持聚合分组级覆盖全局策略。旧连续次数配置保留兼容但不再参与判断；旧运行态降权状态升级后首次读取即清空。完成后跑单元测试、前端构建、Docker dev 构建和真实网关仿真。

## Current Phase
Phase 4 complete

## Phases
### Phase 1: Backend Strategy & Compatibility
- [x] 新增全局百分比策略 option 和规范化逻辑。
- [x] 给聚合分组增加 `smart_strategy_config` JSON 字段与 API 透传。
- [x] 扩展 RPM 指标为可配置窗口，并新增策略失败/慢成功指标。
- [x] 改造降权触发为错误率/慢率，旧状态按 `strategy_version=2` 清空。
- **Status:** complete

### Phase 2: Frontend
- [x] 聚合分组全局策略 UI 替换为百分比配置。
- [x] 编辑弹窗增加跟随全局/自定义策略。
- [x] 运行态抽屉展示策略来源、窗口指标、错误率、慢率和阈值。
- **Status:** complete

### Phase 3: Tests
- [x] 补 service/model/controller 单元测试。
- [x] 运行 `go test ./...`。
- [x] 运行 `cd web && bun run build`。
- **Status:** complete

### Phase 4: Docker Dev Simulation
- [x] 构建 `new-api-local:dev`。
- [x] 重建 `docker-compose-dev.yml` 服务并确认健康。
- [x] 通过真实网关请求验证低样本、1% 错误率、5% 错误率、慢率和组级覆盖。
- [x] 清理临时 DB/Redis 数据。
- **Status:** complete

## Key Constraints
- 本版不做多档降权。
- 本版不做真实子分组 route-level 覆盖。
- 旧连续次数配置只保留兼容，不再影响新策略。
- JSON 操作使用 `common.*` 包装。
- DB 变更兼容 SQLite、MySQL、PostgreSQL。

---

# Task Plan: Relay Error Passthrough Settings

## Goal
新增全局错误响应设置，让上游 400/422 等调用方可修复的错误透传给下游，同时继续隐藏 429/5xx 和权限/渠道类上游细节。

## Current Phase
Phase 4 complete

## Phases
### Phase 1: Backend Settings
- [x] 新增 `relay_error_setting` 配置模块。
- [x] 接入 option 导出、更新和状态码校验。
- **Status:** complete

### Phase 2: Relay Error Behavior
- [x] 按配置决定是否包装上游 OpenAI/Claude 错误。
- [x] 透传时保持原 HTTP 状态和响应协议，并按配置脱敏。
- **Status:** complete

### Phase 3: Admin UI
- [x] 运营设置新增「错误响应设置」卡片。
- [x] 复用状态码规则输入组件。
- **Status:** complete

### Phase 4: Verification
- [x] Focused Go tests passed.
- [x] Planned Go package tests passed.
- [x] Frontend build.
- [x] Docker dev build and smoke verification.
- [x] Root API option read/update/refresh verification.
- **Status:** complete

## Key Constraints
- 默认关闭透传；启用后默认状态码为 `400,422`。
- 不做渠道级覆盖。
- 不暴露敏感信息；默认启用脱敏。
- 不修改无关未跟踪 probe/tmp 文件。

---

# Task Plan: Claude Large Context Relay Profiling

## Goal
给 Claude `/v1/messages` 大上下文链路增加低开销分段耗时日志，并用接近真实 Anthropic usage 的 fake upstream 验证 new-api 是否正确记录 cache read/cache creation 计费字段，以及本地处理是否造成明显首字延迟。

## Current Phase
Phase 3 complete

## Phases
### Phase 1: Instrumentation
- [x] 增加 `CLAUDE_RELAY_PROFILE` 和阶段耗时日志。
- [x] 在 controller 与 Claude relay handler 的关键阶段打点。
- **Status:** complete

### Phase 2: Baseline Verification
- [x] 跑 focused Go tests。
- [x] Docker dev 启用 profile 环境变量并确认服务健康。
- **Status:** complete

### Phase 3: Realistic Cache Usage Simulation
- [x] 用 fake Claude upstream 返回真实 `usage` cache 字段。
- [x] 通过 dev new-api 发 `/v1/messages` 流式请求。
- [x] 对比 consume log 与 profile log，确认 cache 字段和本地耗时。
- **Status:** complete

## Key Constraints
- fake upstream 必须返回 `cache_read_input_tokens`、`cache_creation_input_tokens`、`cache_creation.ephemeral_5m_input_tokens`，不能只用 `input_tokens=1` 证明链路。
- 测试完成后清理临时 channel/token/ability。
- 不修改无关工作树改动。

---

# Task Plan: 外部扫码充值接口

## Goal
新增公开外部充值下单接口：第三方系统通过鉴权码调用 new-api，为指定用户创建扫码充值订单；new-api 调用本地 pay-server 生成收银台/二维码支付信息；pay-server 支付成功后回调 new-api 完成入账；new-api 再按订单内保存的外部 `callback_url` 通知第三方系统。

## Current Phase
Phase 3 complete

## Phases
### Phase 1: Discovery
- [x] 梳理 new-api 当前 `top_ups` 下单、易支付回调和入账逻辑。
- [x] 梳理 pay-server 的易支付兼容层与内部 `/api/v1/orders` JSON 下单接口。
- **Status:** complete

### Phase 2: Backend
- [x] 新增外部充值开关和多鉴权码 option。
- [x] 扩展 `top_ups` 保存外部订单号、回调 URL 和回调状态。
- [x] 新增外部充值下单接口和 pay-server 内部订单客户端。
- [x] 在支付成功入账后触发外部成功回调。
- [x] 新增 root 管理外部充值鉴权码接口。
- **Status:** complete

### Phase 3: Tests
- [x] 覆盖鉴权、用户定位、订单创建、重复外部单号幂等和成功回调。
- [x] 运行 focused Go tests。
- [x] 运行 `git diff --check`。
- [x] 运行前端 `bun run build`。
- **Status:** complete

## Key Constraints
- 不绕过 new-api 订单和入账逻辑；new-api 仍是商户、订单和额度一致性的来源。
- 外部 `callback_url` 只在 new-api 已成功入账后通知。
- 支付成功回调必须保持幂等，重复支付通知不能重复加额度或重复制造错误。
- JSON 操作走 `common.*` 包装。
- DB 变更必须兼容 SQLite、MySQL、PostgreSQL。

---

# Task Plan: 外部注册接口与全局鉴权码

## Goal
新增独立外部注册入口，第三方系统用 `username + password` 注册用户，`invite_code` 可选且传入时绑定邀请码归属人。入口只受外部注册开关和全局鉴权码控制；多个鉴权码存现有 `options` 表，root 可查看/生成/删除，通用 options 列表不泄漏明文。

## Current Phase
Phase 5 complete

## Phases
### Phase 1: Discovery
- [x] 接续现有工作树，确认已有后端、前端、测试和仿真脚本改动。
- [x] 确认 root `/api/option` 路由组权限可复用。
- **Status:** complete

### Phase 2: Backend
- [x] 新增 `ExternalRegisterEnabled` / `ExternalRegisterAuthKey` option。
- [x] 通用 `/api/option/` 过滤鉴权码。
- [x] 新增 root 查看/生成/删除鉴权码接口。
- [x] 新增公开 `/api/user/external_register`。
- [x] 审核普通注册重构是否保持原行为和错误口径。
- [x] 补齐鉴权码启停与注册路径边界处理。
- **Status:** complete

### Phase 3: Frontend
- [x] 系统设置登录注册区增加外部注册接口区域。
- [x] 检查开关、查看、复制、生成、重新生成、删除交互和布局。
- **Status:** complete

### Phase 4: Tests
- [x] 补/修控制器测试覆盖计划列出的边界。
- [x] 运行 `go test ./controller ./model`。
- [x] 运行 `cd web && bun run build`。
- [x] 运行 `git diff --check`。
- **Status:** complete

### Phase 5: Docker Dev Simulation
- [x] 构建 `new-api-local:dev`。
- [x] 启动 `docker-compose-dev.yml` dev 服务。
- [x] 执行 HTTP 仿真：缺码、错码、正确码、重生成旧码失效、删除后失败，并校验 DB 绑定字段。
- **Status:** complete

## Key Constraints
- 不新增数据库表。
- 外部注册邀请码可选，且不受普通注册、邮箱验证、Turnstile、密码注册开关影响。
- 成功响应不返回登录态和默认 API token。
- 如果系统配置开启默认 token，注册路径仍按普通注册一致生成默认 token。
- 明文鉴权码只允许 root 专用接口查看，不进入通用 options 列表。

---

# Task Plan: 聚合分组 Cluster 智能策略 v2

## Goal
在 `cluster` 模式下把智能策略从单次固定降权升级为降级窗口内可递减降权，并新增仅流式请求生效的首字慢阈值。`failover` 保持旧跳过语义；普通 cluster 池和 Claude CLI/client 专用池都使用同一套递减有效权重。完成后补后端/前端/文档/测试，并构建 Docker dev 做本地仿真验证。

## Current Phase
Phase 5 complete

## Phases
### Phase 1: Discovery
- [x] 确认现有智能策略状态、选路候选和运行态展示结构。
- [x] 确认流式首字时间从 relay 写入 Gin context 的合适位置。
- **Status:** complete

### Phase 2: Backend
- [x] 扩展智能策略运行态字段和兼容旧状态读取。
- [x] 实现 cluster 递减有效权重和降级期间重复触发降级。
- [x] 新增流式首字慢阈值配置和校验。
- [x] 将 relay 首字时间桥接给聚合分组成功记录逻辑。
- **Status:** complete

### Phase 3: Frontend
- [x] 智能策略设置增加首字慢阈值输入。
- [x] 运行态拓扑/详情展示降级层级、降级期间计数和慢请求原因。
- **Status:** complete

### Phase 4: Tests & Docs
- [x] 补 service/controller/model 单元测试。
- [x] 更新 `2dev/doc` 三份文档。
- **Status:** complete

### Phase 5: Verification
- [x] `go test ./service ./controller ./model`。
- [x] `cd web && bun run build`。
- [x] Docker dev build/recreate/status。
- [x] 本地仿真 cluster 降权、禁用、failover 回归、CLI 专用池。
- **Status:** complete

## Key Constraints
- 不新增 DB 字段，降级层级只作为运行态状态。
- `failover` 行为不变。
- `weight=0` 永远不参与加权选择。
- 首字慢阈值只对流式请求生效，默认 `0` 关闭。
- 客户端专用池必须同步使用递减降权。

---

# Task Plan: Token And Body Storage Performance Optimization

## Goal
在不改变 token 统计、计费口径、API 行为的前提下，降低 response fallback token 估算、OpenAI tokenizer、本地 body storage 在大请求/高并发下的 CPU、内存分配和 GC 压力，并补充持续压测工具验证 `system cpu overloaded` 不再持续出现。

## Current Phase
Phase 5 complete

## Phases
### Phase 1: Discovery
- [x] 确认 `system cpu overloaded` 是本地性能监控中间件触发，不是上游返回。
- [x] 确认 `ResponseText2Usage` 在上游 usage 缺失时仍会本地估算 completion tokens。
- [x] 确认现有 body storage 对未知长度内存路径使用 `io.ReadAll`。
- **Status:** complete

### Phase 2: Backend Optimization
- [x] 替换 context-aware tokenizer 并保留旧 `CountTextToken` API。
- [x] 优化 estimator 热路径并保持 golden 输出一致。
- [x] 将 response usage fallback 改为 request context 可取消。
- [x] 新增 pooled memory body storage，降低 reader 内存路径分配。
- **Status:** complete

### Phase 3: Tests And Benchmarks
- [x] 增加 token estimator / tokenizer / usage fallback 一致性测试。
- [x] 增加 body storage 行为测试。
- [x] 增加 service/common benchmarks。
- **Status:** complete

### Phase 4: Sustained Load Tooling
- [x] 增加本地 fake upstream + sustained load 压测脚本。
- [x] 支持短压测、持续压测、窗口指标、503 统计和进程/pprof 采集。
- **Status:** complete

### Phase 5: Verification
- [x] 运行 targeted tests。
- [x] 运行全量或可承受范围内回归。
- [x] 运行 benchmark 命令并记录结果。
- **Status:** complete

## Key Constraints
- 不关闭 `CountToken`。
- 不移除 `ResponseText2Usage` usage fallback。
- 正常完成请求的 token 数、usage 和最终扣费必须保持一致。
- tokenizer 取消只影响请求取消/超时路径，不能影响正常成功路径。
- 不修改受保护项目标识。

---

# Task Plan: 聚合分组客户端专用流量池 v1

## Goal
新增默认关闭的客户端专用流量池，第一版只识别 Claude Code CLI。启用后 CLI 请求优先进入独立配置的 CLI 专用池；专用池跟随当前聚合分组路由模式，`failover` 下按专用池顺序故障转移，`cluster` 下按专用池权重分发；专用池不可用时按配置回退当前模式默认池。未配置或关闭时，现有 failover / cluster / 权重 / 亲和 / 智能降权 / retry 行为不变。

## Current Phase
Phase 6 complete

## Phases
### Phase 1: Discovery
- [x] 确认默认 cluster 选路依赖 `aggregate_group_targets` 和 target index。
- [x] 确认专用池 target 可能不在默认池中，需要 route pool + route group 级上下文。
- [x] 确认前端路由模式配置区和运行态抽屉的数据结构接入点。
- **Status:** complete

### Phase 2: Backend Config And Detection
- [x] 新增 `aggregate_groups.client_route_pools` 文本 JSON 字段和模型读写方法。
- [x] 创建/编辑/列表/详情 API round-trip `client_route_pools`。
- [x] 增加配置校验：负权重拒绝、重复 target 拒绝、真实分组校验。
- [x] 增加 Claude Code CLI 识别。
- **Status:** complete

### Phase 3: Routing Runtime
- [x] failover / cluster 选路优先尝试 CLI 专用池。
- [x] 专用池支持模型过滤、手动禁用过滤和当前模式智能策略。
- [x] 专用池内部按 `RetryTimes` 重试，耗尽后切同池其他 target。
- [x] 专用池耗尽后按 `fallback_to_default` 回退当前模式默认池或失败。
- [x] 专用池 route affinity / failover 运行态独立于默认池。
- [x] 日志/runtime 增加 client route 字段。
- **Status:** complete

### Phase 4: Frontend
- [x] 路由模式配置区增加客户端专用流量池区域。
- [x] 默认池与 Claude CLI 专用池分开展示和保存。
- [x] 运行态展示专用池目标和状态。
- **Status:** complete

### Phase 5: Tests, Script, Docs
- [x] 补模型、service、controller 单元测试。
- [x] 新增 `2dev/script/simulate_aggregate_client_pool_scenarios.py`。
- [x] 更新 `2dev/doc/aggregate-group-design.md`、里程碑和迭代日志。
- **Status:** complete

### Phase 6: Verification
- [x] `go test ./service ./controller ./model`。
- [x] `cd web && bun run build`。
- [x] Docker dev build/recreate/status。
- [x] 复跑聚合分组场景脚本。
- **Status:** complete

## Key Constraints
- 只在总开关、Claude CLI 池开关都开启且请求识别为 Claude Code CLI 时生效。
- 空配置等价关闭，升级后不开启专用池不改变生产行为。
- 专用池跟随当前聚合分组路由模式，不新增专用池自己的路由模式开关。
- Claude CLI 识别硬条件不依赖固定 Anthropic-Beta 日期。
- 专用池独立配置，不用默认池 `weight=0` 表达 CLI pool。

---

# Task Plan: 聚合分组单渠道子分组组内 retry 修正

## Goal
修正聚合分组在 `failover` / `cluster` 下的组内 retry 语义：`RetryTimes` 表示当前真实分组内部重试预算。即使某个子分组只有一个渠道 / 一个 priority，也应先按 `RetryTimes` 重试当前子分组，预算耗尽后才切换到下一个子分组。

## Current Phase
Phase 3 complete

## Phases
### Phase 1: Discovery
- [x] 确认现有实现以 `priorityCount` 判断是否可以组内 retry。
- [x] 确认单渠道 / 单 priority 子分组会被判断为组内无下一档，从而直接跨组 fallback。
- **Status:** complete

### Phase 2: Backend
- [x] 修正 failover 选路：当前真实分组有可用 channel 时，不因 retry 超过 priority 数量而直接跳到下一个真实分组。
- [x] 修正 cluster 选路：当前 route group 内部 retry 预算未耗尽时，继续选择当前 route group。
- [x] 保持跨子分组 fallback 的 attempted route 记录，避免预算耗尽后反复回到失败子分组。
- **Status:** complete

### Phase 3: Tests, Docs & Verification
- [x] 补 failover 单渠道 / 单 priority 组内 retry 测试。
- [x] 补 cluster 单渠道 / 单 priority 组内 retry 测试。
- [x] 更新 `2dev/doc` 设计、里程碑和迭代日志。
- [x] 运行聚合分组 retry 单测。
- [x] 运行 `go test ./service ./controller ./model`。
- [x] 运行 `cd web && bun run build`。
- [x] 构建并重启 Docker dev。
- **Status:** complete

## Key Constraints
- 不改变非聚合分组选路逻辑。
- `RetryTimes=2` 的语义为当前真实分组最多尝试 `1 + 2` 次。
- 跨子分组 fallback 仍受 retryable 状态码 / skip-retry 规则控制。

---

# Previous Plan: 聚合分组与渠道分组选择体验优化

## Goal
补齐聚合分组列表和相关表单的可操作性：列表直接标记当前路由模式；聚合分组可见用户组只显示 `default` / `UserGroup-*`，真实分组只显示实际路由分组；相关 Select 都支持搜索；Failover / Cluster 切换必须确认后才生效。

## Current Phase
Phase 3 complete

## Phases
### Phase 1: Discovery
- [x] 定位聚合分组列表列定义、编辑抽屉和渠道新增/编辑分组选择。
- [x] 确认渠道新增和编辑共用 `EditChannelModal.jsx`。
- **Status:** complete

### Phase 2: Frontend
- [x] 聚合分组列表名称列增加路由模式标签。
- [x] 聚合分组可见用户组选项过滤为 `default` / `UserGroup-*`。
- [x] 聚合分组真实分组选项过滤掉 `default` / `UserGroup-*`。
- [x] 聚合分组可见用户组和真实分组 Select 启用搜索过滤。
- [x] 渠道新增/编辑分组 Select 启用搜索过滤。
- [x] Failover / Cluster tab 切换前增加确认弹窗。
- **Status:** complete

### Phase 3: Docs & Verification
- [x] 更新 `2dev/doc` 设计、里程碑和迭代日志。
- [x] 运行 `git diff --check`。
- [x] 运行 `cd web && bun run build`。
- **Status:** complete

## Key Constraints
- 不改变后端保存结构和聚合分组选路逻辑。
- `default` 和 `UserGroup-*` 是用户可见分组，不作为聚合分组 target。
- 真实分组搜索只影响前端候选展示，不改变已有数据。

---

# Previous Plan: 聚合分组编辑面板模式化改造

## Goal
将聚合分组新增/编辑 SideSheet 从“所有字段混在一起”调整为“基础信息 + 路由模式 Tabs”。Failover 只展示链式故障转移相关配置，Cluster 只展示亲和时间和权重分发相关配置；权重含义必须在 UI 上直接说明。后端保存结构不变。

## Current Phase
Phase 3 complete

## Phases
### Phase 1: Discovery
- [x] 确认当前编辑弹窗字段、保存 payload 和 target 权重逻辑。
- [x] 确认本次只做前端布局调整，不改变聚合分组选路语义。
- **Status:** complete

### Phase 2: Frontend
- [x] 基础信息区保留名称、显示名、描述、倍率、启用状态、智能策略和可见用户组。
- [x] 新增 `Failover 故障转移` / `Cluster 集群分发` Tabs，并用 tab 切换 `routing_mode`。
- [x] Failover tab 只展示懒恢复、恢复间隔、重试状态码和链式 target 列表。
- [x] Cluster tab 展示亲和保持时间、重试状态码和带权重输入的子分组列表。
- [x] 在 target 列表中说明权重是相对流量比例，`100/200` 约等于 `1:2`，`0` 不参与普通加权随机。
- **Status:** complete

### Phase 3: Docs & Verification
- [x] 更新 `2dev/doc` 设计、里程碑和迭代日志。
- [x] 运行 `cd web && bun run build`。
- **Status:** complete

## Key Constraints
- 不改 `handleSubmit` payload，不影响已有后端兼容逻辑。
- 未切换到 `Cluster` 时不显示权重编辑，避免 Failover 操作误解。
- `cluster_affinity_ttl_seconds` 只在 Cluster tab 中配置。

---

# Previous Plan: Cluster 亲和保持时间配置

## Goal
为每个聚合分组新增 `cluster_affinity_ttl_seconds` 配置，默认 300 秒；仅在 `routing_mode=cluster` 时生效，并且新增/编辑 UI 只有 Cluster 模式下允许修改。同步将 route affinity 调整为用户级优先，模型只作为能力校验条件。保持已有 failover 行为不变。

## Current Phase
Phase 4

## Phases
### Phase 1: Discovery
- [x] 确认现有 route affinity TTL 来源和聚合分组 create/edit 数据流。
- [x] 确认运行态/测试需要覆盖的兼容行为。
- **Status:** complete

### Phase 2: Backend
- [x] 模型新增 `cluster_affinity_ttl_seconds` 默认 300。
- [x] 创建/编辑接口读写并校验该字段。
- [x] route affinity TTL 使用新字段；无效值 fallback 到 300。
- [x] route affinity key 从 `aggregate_group + model + user_id` 调整为 `aggregate_group + user_id`，选中前仍校验模型支持。
- **Status:** complete

### Phase 3: Frontend & Docs
- [x] 编辑弹窗增加 Cluster 亲和保持时间输入，仅 Cluster 模式可编辑。
- [x] 更新 `2dev/doc` 设计、里程碑和迭代日志。
- **Status:** complete

### Phase 4: Verification
- [x] 补充单元测试。
- [x] 运行 `go test ./service ./controller ./model`。
- [x] 运行 `go test ./...`。
- [x] 运行 `cd web && bun run build`。
- [x] 运行 Docker dev 构建和重建。
- **Status:** complete

## Key Constraints
- `failover` 模式不使用该字段，不改变旧行为。
- 老数据迁移后默认 300 秒。
- 亲和 key 按用户级语义演进：同一用户切换 sonnet/opus 时，原子分组支持目标模型则继续复用。

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|

---

# 违禁关键词检测记录菜单可行性 Plan

## Goal
评估是否可以新增管理员菜单，用于配置/查看违禁关键词检测命中记录，并判断最佳接入点、数据存储方式和对 relay 主流程的影响。

## Current Phase
Phase 2 in progress

## Phases
### Phase 1: Discovery
- [x] 阅读管理员菜单、路由、设置页、日志页、request dump、敏感词检测相关代码。
- [x] 确认现有日志结构是否足够承载命中记录。

### Phase 2: Design Options
- [x] 比较复用 error log、复用 request dump、新增 violation log 表三种方案。
- [x] 判断推荐方案和迁移/测试成本。

### Phase 3: Answer
- [ ] 给出可行性、推荐方案、需要确认的问题。

## Constraints
- 先只做排查和设计，不改业务代码。
- 不修改受保护项目标识。
- JSON marshal/unmarshal 遵循 `common.*`。

---

# 违规用途拦截能力排查 Plan

## Goal
确认当前系统是否已经对“逆向、安全、破限”等疑似违规用途做请求侧或响应侧拦截，并指出可配置位置、覆盖范围和缺口。

## Current Phase
Phase 3 complete

## Phases
### Phase 1: Static Discovery
- [x] 搜索 moderation、内容过滤、敏感词、prompt/key blocking、模型映射和 relay 中断逻辑。
- [x] 阅读相关 controller/service/middleware/relay 代码。

### Phase 2: Behavior Assessment
- [x] 判断拦截发生在用户输入、上游响应、日志/告警、还是只做额度/鉴权限制。
- [x] 确认是否默认启用、是否 admin 可配置、是否支持“逆向/安全/破限”这类场景。

### Phase 3: Answer
- [x] 给出结论、证据文件和建议。

## Constraints
- 不做代码修改，除本排查记录追加外不触碰业务文件。
- 遵守项目 JSON 和数据库兼容规则。
| ui-ux skill bundled path lacks scripts directory | tried `/Users/zhangyu/.codex/skills/ui-ux-pro-max/scripts/search.py` | used `/Users/zhangyu/.agents/skills/ui-ux-pro-max/scripts/search.py` |

---

# Previous Plan: 邀请返佣利润保护 v2

## Goal
在现有邀请返佣报表基础上增加“分组利润规则”和“消费日志利润快照”，使新日志可以按分组毛利率计算成本、毛利、理论返佣、利润保护上限和最终预估返佣；完成单元测试、前端构建、Docker dev 回归，并保留一套 `commission_profit_<run_id>` 演示数据给用户查看。

## Current Phase
Phase 5

## Phases
### Phase 1: Backend Profit Rules
- [x] 扩展 `InviteCommissionSettings` 和分组利润规则结构。
- [x] 增加系统分组平铺列表、规则保存、规则清除逻辑。
- [x] 增加后台接口和路由。
- **Status:** complete

### Phase 2: Log Snapshot & Report Calculation
- [x] 写消费日志时固化 `admin_info.commission` 利润快照。
- [x] 返佣报表读取快照并计算成本、毛利、理论返佣、利润保护和分组汇总。
- [x] 老日志缺失快照只统计消费，不参与 v2 返佣。
- **Status:** complete

### Phase 3: Tests
- [x] 补模型层分组规则、日志快照、利润保护和报表测试。
- [x] 补控制器接口权限和返回字段测试。
- **Status:** complete

### Phase 4: Frontend
- [x] 在规则配置中增加“分组利润规则”平铺表和编辑弹窗。
- [x] 管理员报表展示分组、成本、毛利和利润保护字段。
- [x] 用户侧不展示利润率、成本和毛利。
- **Status:** complete

### Phase 5: Docs & Regression
- [x] 更新设计文档、迭代开发日志和里程碑事件。
- [x] 运行 `go test ./model ./controller`、`go test ./...`、`cd web && bun run build`。
- [x] Docker dev 单服务重建并保留演示数据验证。
- **Status:** complete

## Key Constraints
- 不处理老数据，不做历史回填；缺失快照的老日志只统计消费。
- 利润率是运营配置口径，不自动读取上游账单。
- `admin_info.commission` 只管理员可见。
- Docker 回归只重建 `new-api-dev`：`docker build -t new-api-local:dev . && docker compose -f docker-compose-dev.yml up -d --no-deps --force-recreate new-api-dev`。
- Runtime 演示数据使用 `commission_profit_<run_id>` 前缀并保留，不清理。

## Run Data
- `run_id`: `20260427v2a`
- `prefix`: `commission_profit_20260427v2a`
- A owner user id: `994100`
- B user id: `994101`
- C user id: `994102`
- D third-level user id: `994103`
- runtime log ids: `586`-`598`

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
| zsh `read-only variable: status` | 上一轮请求脚本第一条请求前失败 | 后续脚本避免使用变量名 `status`，改用 `http_code` |
| GPT 请求 401 无效令牌 | 恢复脚本使用了上一轮记录的错误长度 key | 终止脚本，改为每次从 `tokens.key` 读取实际 48 位 key 后继续；401 未写入消费日志 |
| v1 返佣单测期望非 0，v2 改为缺失快照不返佣 | `go test ./model` | 将需要返佣的测试日志改为带 `admin_info.commission` 快照，并补缺失快照测试 |

## Follow-up Simplification
- [x] 删除 `InviteCommissionSettings.service_rates` 和 `InviteCommissionSettings.model_rules` 旧配置字段。
- [x] 新增 `InviteCommissionSettings.service_categories` 服务分类字典。
- [x] 报表服务分类只从已配置的日志利润快照读取；缺失快照或未配置分组归 `Other`。
- [x] 前端规则配置移除“服务返佣基准”和“模型归类规则”，新增“服务分类”。
- [x] 文档同步说明 v2 以分组利润规则为准。

---

# Task Plan: 日志看板耗时分析

## Goal
在现有日志看板新增独立耗时分析区域，基于最终成功请求展示渠道、分组、渠道模型维度的平均/P50/P90/P95/最大耗时。

## Current Phase
Phase 3 complete

## Phases
### Phase 1: Backend
- [x] 扩展 dashboard 日志查询字段。
- [x] 实现 latency 聚合结构与 nearest-rank 百分位。
- [x] 补 service 测试。

### Phase 2: Frontend
- [x] 新增耗时分析 Card。
- [x] 增加维度 Tabs、筛选、P95 Top 图表与 Top 20 表格。

### Phase 3: Verification
- [x] go test ./service
- [x] go test ./controller ./model
- [x] cd web && bun run build

## Constraints
- 不新增接口、不迁移数据库。
- 不修改受保护项目标识。
- JSON marshal/unmarshal 遵循 common/json.go。

---

# Task Plan: Dump 分析与内置 Console

## Goal
实现管理员临时请求 Dump：按用户/令牌等条件旁路打印请求 URL、header、JSON 原文和可选上游 body 到服务日志与页面 Console，且不破坏用户原始请求、计费、路由、重试或业务响应。

## Current Phase
Phase 5 in progress

## Phases
### Phase 1: Discovery & Design Anchoring
- [x] 读取 AGENTS.md 约束、relay 路由、body storage、admin 菜单和日志方式。
- [x] 记录实现入口与安全约束。

### Phase 2: Backend
- [x] 新增 request dump service：内存规则、计数、过期、ring buffer、header 过滤、content-type 跳过。
- [x] 新增 admin-only controller/API/route。
- [x] 在 relay raw/upstream/error 阶段接入旁路 Dump，确保失败不影响主流程。

### Phase 3: Frontend
- [x] 新增 `/console/request-dump` 页面。
- [x] 新增管理员菜单“Dump 分析”。
- [x] Console 支持轮询、暂停、清空、复制、搜索、自动滚动。

### Phase 4: Tests
- [x] 补服务与控制器单元测试。
- [x] 补请求体可复读/不破坏业务的测试。

### Phase 5: Verification
- [x] `go test ./...`
- [x] `cd web && bun run build`
- [ ] `docker build -t new-api-local:dev .`
- [ ] `docker compose -f docker-compose-dev.yml up -d --force-recreate new-api-dev postgres-dev redis-dev`
- [ ] Docker dev 创建临时数据并真实验证 Dump 开启/关闭/错误/body 跳过/业务不变。

## Key Constraints
- Dump 只用当前进程内存，不落 DB，不进 Redis。
- JSON marshal/unmarshal 必须用 `common.*`。
- 不直接读空 `c.Request.Body`；只通过 `common.GetBodyStorage(c)`。
- Dump 任何异常必须 recover 并旁路失败，不能改变业务响应。
- Header 默认排除凭证类字段。

## Errors Encountered
| Error | Attempt | Resolution |
|-------|---------|------------|
