# Session: 2026-05-19 聚合分组百分比智能降权

## Scope
- 实施聚合分组百分比智能降权、组级覆盖、旧状态兼容清理、前端策略 UI、单元测试和 Docker dev 真实仿真。

## Progress
- 已确认实施计划和验收要求。
- 已确认当前 dirty worktree 主要是未跟踪 probe/tmp/output 文件，本轮不触碰。
- 已完成后端百分比策略实现：新增全局 option、组级 `smart_strategy_config`、RPM 窗口指标、v2 状态清理、非递归降权。
- 已完成聚合分组 UI：全局策略改为错误率/慢率百分比，编辑弹窗支持跟随全局/自定义覆盖，运行态展示策略来源和窗口指标。
- 已新增 Docker dev 真实仿真脚本 `2dev/script/simulate_aggregate_percentage_strategy.py`，使用临时聚合分组/渠道/token 和 fake Claude upstream 通过真实 `/v1/messages` 网关请求验证百分比策略。

## Verification
- `go test ./service ./controller ./model -run 'Aggregate|Option' -count=1`: passed.
- `go test ./...`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie/chunk-size warnings.
- `python3 -m py_compile 2dev/script/simulate_aggregate_percentage_strategy.py`: passed.
- `docker build -t new-api-local:dev .`: passed.
- `docker compose -f docker-compose-dev.yml up -d --force-recreate new-api-dev postgres-dev redis-dev`: passed; `new-api-dev` healthy.
- `python3 2dev/script/simulate_aggregate_percentage_strategy.py`: passed; verified old state reset, low sample no degrade, 1,000,000-window 1% failure no degrade, 5% failure degrade, 30% slow-rate degrade, group override source/thresholds, runtime API, and Redis state.
- Post-simulation cleanup verified: temporary `agpct%` aggregate groups/users/channels/tokens all `0`, Redis `*agpct*` keys `0`, `new-api-dev` remains healthy.
- `git diff --check`: passed.

## Errors
- 初次 focused Go 测试发现旧连续次数/递归降权测试预期未更新，已改为百分比窗口语义。
- 组级覆盖和流式首字慢 focused 用例第一次失败：测试样本数/阈值设置不匹配新慢率/错误率算法，已修正。
- 第一次 Docker 仿真脚本失败：PostgreSQL `INSERT ... RETURNING` 输出包含 command tag，脚本直接 `int()` 解析失败。已改为 `psql_scalar()` 只取首个非空返回行，并清理首次失败留下的临时 `agpct%` 用户。

---

# Session: 2026-05-18 Relay Error Passthrough Settings

## Scope
- 为 relay 上游错误新增可配置透传策略，默认关闭，启用后默认透传 400/422，运营设置增加管理 UI。

## Progress
- 已新增 `relay_error_setting` 后端配置，默认关闭、状态码 `400,422`、敏感信息脱敏开启。
- 已更新 relay 客户端错误包装逻辑：命中配置的上游 OpenAI/Claude 错误按原协议和原状态码返回，未命中继续包装为通用 service unavailable。
- 已补充 Claude 格式透传时的 `type` 保留逻辑：通用上游错误解析为 OpenAIError 后，Claude 响应仍优先使用上游 `error.type`。
- 已新增运营设置「错误响应设置」卡片。

## Verification
- `go test ./controller -run 'RelayError|RelayErrorSetting|AggregateGroupStrategyOptions' -count=1`: passed.
- `go test ./setting/operation_setting -count=1`: passed.
- `go test ./controller ./service ./setting/operation_setting ./model -count=1`: passed.
- `git diff --check`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie/chunk-size warnings.
- `docker compose -f docker-compose-dev.yml up -d --build`: passed; `new-api-dev` healthy.
- `curl -fsS http://localhost:3001/api/status`: passed.
- Root API `/api/option/` verification passed: defaults visible, updates persisted after refresh, invalid `400,abc` rejected.
- Dev DB cleanup completed after API verification: temporary relay error option rows removed and root access token restored to NULL.

## Errors
- First API verification cleanup used incorrect SQL quoting in a shell `trap`, leaving temporary option rows until manually restored. Fixed by rerunning cleanup with properly quoted SQL and confirming option count is 0.

---

# Session: 2026-05-07 Claude Large Context Relay Profiling

## Scope
- 为 Claude `/v1/messages` 大上下文首字慢问题补分段 profile 日志，并用带真实 cache usage 字段的 fake upstream 做 dev 验证。

## Progress
- 已有代码改动：`relay/profile.go`、`controller/relay.go`、`relay/claude_handler.go`、`docker-compose-dev.yml`。
- 已确认 dev 容器 `new-api-dev` healthy，profile env 已在 compose 中配置。
- 发现之前 fake upstream 只返回 `usage.input_tokens=1`，无法验证用户关心的 cache read/cache creation 计费展示，本轮需要改用真实 Anthropic usage 结构。
- `go test ./dto ./relay/... ./controller -count=1` passed。
- 已重建并重启 `new-api-dev` dev 镜像。
- 端到端 fake Claude cache usage 测试通过：
  - 请求体大小 `3,200,202` bytes。
  - fake upstream 收到请求后约 `12ms` 完成响应。
  - curl `time_starttransfer=0.012744`，`time_total=0.204283`。
  - new-api consume log 记录 `prompt_tokens=1`、`completion_tokens=116`、`cache_tokens=8459`、`cache_creation_tokens=5994`、`cache_creation_tokens_5m=5994`、`usage_semantic=anthropic`。
  - profile 主要阶段：`validate_request=16ms`、`token_count_meta=24ms`、`sensitive_check=32ms`、`estimate_request_token=18ms`、`claude_remove_disabled_fields=21ms`、`claude_do_request=17ms`、`claude_helper_total=50ms`。
- 已清理临时 channel/token/log/quota_data，停止 fake upstream，恢复 root quota 到 `92545367`，并重启 dev 服务刷新缓存。
- gofmt 后复跑 `go test ./dto ./relay/... ./controller -count=1` passed。
- 按用户要求改用真实渠道 `16 - 测试自建windsurf` 和真实令牌 `测试自建的windsurf` 跑真实上游，不清理消费日志：
  - 第一条 cache create：log id `33796`，request id `20260507154634692989505meEnMzCO`，请求体 `1,645,268` bytes，`frt=7521ms`，`claude_do_request=7461ms`，`claude_helper_total=7480ms`，`cache_creation_tokens_5m=7850`。
  - 第二条 cache read + create：log id `33797`，request id `20260507154720486527554nof0t7SA`，请求体 `2,174,081` bytes，`frt=5513ms`，`claude_do_request=5424ms`，`claude_helper_total=5447ms`，`cache_tokens=407753`，`cache_creation_tokens_5m=7921`。
  - 本地阶段仍是毫秒级：`validate_request` 10-15ms、`token_count_meta` 10-20ms、`sensitive_check` 15-21ms、`claude_remove_disabled_fields` 10-14ms。
  - 直连渠道背后的 `127.0.0.1:3003` 发同一大请求总耗时 `6.922706s`，说明该真实链路本身就是秒级。

## Errors
- 健康检查 shell 第一次使用变量名 `status`，zsh 下该变量只读导致命令失败；改用其他变量名重跑。

---

# Progress Log

## Session: 2026-05-06 External Topup API

### Scope
- 为第三方系统新增公开扫码充值下单接口，new-api 通过 pay-server 创建支付订单，支付成功后完成本地入账并通知第三方回调 URL。

### Progress
- 已确认现有充值链路：`RequestEpay` 创建 `top_ups` pending，`EpayNotify` 验签后置 success 并给用户加额度。
- 已确认 pay-server 既支持易支付兼容层 `/submit.php`，也支持内部 JSON 下单接口 `/api/v1/orders`。
- 已决定本轮优先使用 pay-server 内部 JSON 下单，保留易支付商户通知作为 new-api 入账回调。
- 已新增 `POST /api/user/external_topup`，支持 `user_id` / `username`、`amount`、`payment_method`、`external_order_no`、`callback_url`。
- 已扩展 `top_ups` 外部充值元数据：pay-server 单号、外部单号、回调 URL/状态/响应和支付信息 JSON。
- 已新增外部充值专用鉴权码 root 管理接口，并在支付设置页补 `PayServerInternalToken`。
- 已将易支付成功入账抽成 `model.CompleteEpayTopUp`，支付成功后异步通知外部 callback。

### Verification
- `go test ./controller -run 'ExternalTopup|ExternalRegister|Epay' -count=1`: passed.
- `go test ./controller ./model -count=1`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie/chunk-size warnings.
- `git diff --check`: passed.

---

## Session: 2026-05-05 External Register Auth Code

### Scope
- 新增外部注册公开接口、全局鉴权码 root 管理接口、系统设置 UI 和测试/仿真验证。

### Progress
- 已接续已有工作树，确认当前已改动后端 option、user、router 和前端 SystemSetting。
- 已补充本轮 `task_plan.md` / `findings.md` 顶部计划和发现，避免继续沿用旧任务计划。
- 已完成后端接口、鉴权码管理、通用 options 过滤与直接更新保护、外部注册输入校验和普通注册默认 token helper 复用。
- 已完成系统设置页外部注册接口管理区，并同步生成/删除后的 checkbox 状态。
- 已将仿真脚本改为通过 root 管理接口生成/重生成/删除鉴权码，只用 DB 准备邀请人/邀请码和校验注册结果。

### Verification
- `go test ./controller ./model -count=1`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie eval/chunk size warnings.
- `git diff --check`: passed.
- `python3 -m py_compile 2dev/script/simulate_external_register.py`: passed.
- `docker build -t new-api-local:dev .`: passed.
- `docker compose -f docker-compose-dev.yml up -d --force-recreate new-api-dev postgres-dev redis-dev`: passed; `new-api-dev` became healthy.
- `python3 2dev/script/simulate_external_register.py`: passed; verified missing/wrong auth rejection, successful registration, DB invite binding/quota, regenerated old key failure/new key success, and delete disables endpoint.
- After replacing `controller/user.go` direct JSON calls with `common.*` wrappers, reran:
  - `go test ./controller ./model -count=1`: passed.
  - `git diff --check`: passed.
  - Rebuilt `docker build -t new-api-local:dev .`, recreated dev compose services, and reran `python3 2dev/script/simulate_external_register.py`: passed.
- Follow-up change: switched external register auth from one global code to multiple codes stored as a JSON array in `ExternalRegisterAuthKey`, added single-code/all-code deletion, and made `invite_code` optional.
- Cleanup note: `rm -rf ... web/vite.config.js.timestamp-*.mjs` failed once under zsh because the glob had no match. Resolved with `find web -maxdepth 1 -name 'vite.config.js.timestamp-*.mjs' -delete`.
- Follow-up Docker simulation first run failed after optional-invite registration because `psql` output trimming omitted the final empty left-join field. Resolved by selecting `coalesce(ic.code, '-')`.

### Known Issue From Previous Attempt
- 上一轮仿真命令因 zsh 只读变量 `status` 命名冲突失败；需要用不同变量名重跑脚本或修脚本后再验证。
- 本轮第一次 Docker 仿真失败：默认前缀生成的用户名超过 `User.Username max=20`，外部注册返回 username max 校验错误。处理方式：改脚本使用短命名空间生成临时用户名。
- 第二次 Docker 仿真失败：`psql -At` 默认用 `|` 分隔，脚本按 tab 拆字段导致 “expected 7, got 1”。处理方式：给 psql 增加 `-F '\\t'`。

---

## Session: 2026-05-03 Aggregate Group Cluster Smart Strategy v2

### Scope
- 实现 cluster 模式递减降权和流式首字慢阈值，覆盖默认 cluster 池与 Claude CLI/client 专用池；保持 failover 旧行为不变。

### Progress
- 已记录实施计划和关键设计决策。
- 已完成后端递减降权、降级窗口内重复触发、首字慢阈值配置、relay 首字时间桥接和运行态字段。
- 已完成前端首字慢阈值配置、拓扑节点权重/level 展示、节点详情降级期间计数和慢请求原因展示。
- 已更新 `2dev/doc` 三份文档。
- 已扩展 Docker dev 场景脚本，新增 level 2 递减降权验证。

### Verification
- `go test ./service ./controller ./model`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie eval/chunk size warnings.
- `docker build -t new-api-local:dev .`: passed.
- `docker compose -f docker-compose-dev.yml up -d --no-deps --force-recreate new-api-dev`: passed.
- `curl -fsS http://localhost:3001/api/status`: passed.
- `new-api-dev` health: healthy.
- `python3 2dev/script/simulate_aggregate_cluster_scenarios.py --attempts-per-scenario 1 --report-json 2dev/script/aggregate_cluster_scenario_report.json`: passed, 10/10.
- `python3 2dev/script/simulate_aggregate_client_pool_scenarios.py --attempts-per-scenario 1 --strict`: passed.

### Live Demo Script
- Added `2dev/script/live_aggregate_degrade_demo.py`.
- The script starts a local fake Claude upstream, temporarily points the selected dev channel pair at healthy/failing fake routes, sends real gateway requests, and prints the live Redis smart-state after each round.
- Defaults restore DB/options/channel/Redis state on exit; `--keep-degrade-state` can keep generated Redis degradation state visible after DB/options restore.
- Smoke verification:
  - `python3 -m py_compile 2dev/script/live_aggregate_degrade_demo.py`: passed.
  - `python3 2dev/script/live_aggregate_degrade_demo.py --target-level 1 --rounds 6 --pause 1 --hold-seconds 0`: passed; observed `200/200 -> 200/40`.
  - `python3 2dev/script/live_aggregate_degrade_demo.py --target-level 2 --rounds 10 --pause 0 --hold-seconds 0`: passed; observed `200/200 -> 200/40 -> 200/8`.

---

## Session: 2026-04-30 Aggregate Group Client Route Pool v1

### Scope
- 新增默认关闭的客户端专用流量池，第一版定向 Claude Code CLI；该能力同时支持 `failover` 和 `cluster`，并跟随当前聚合分组路由模式。

### Progress
- 已确认 `client_route_pools` 需要独立于默认 `aggregate_group_targets`。
- 已确认现有 cluster retry 使用默认 target index，专用池需要补 route pool + route group 上下文。
- 已确认 route affinity 当前是 `aggregate_group + user_id`，专用池需要独立 key。
- 已完成后端 `client_route_pools` 配置、Claude Code CLI 识别、专用池选路、专用池 retry/fallback、独立 affinity / 运行态、日志和运行态字段。
- 已补充 failover 专用池：按专用池 target 顺序故障转移，并使用 pool 级运行态避免污染默认 failover 链路。
- 已完成前端路由模式配置区的客户端专用流量池配置，以及运行态专用池展示。
- 已新增 `2dev/script/simulate_aggregate_client_pool_scenarios.py` 并更新 `2dev/doc`。

### Verification
- `go test ./service ./controller ./model`: passed.
- `go test ./...`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie eval/chunk size warnings.
- `python3 -m py_compile 2dev/script/simulate_aggregate_client_pool_scenarios.py 2dev/script/simulate_aggregate_cluster_scenarios.py`: passed.
- `docker build -t new-api-local:dev .`: passed.
- `docker compose -f docker-compose-dev.yml up -d --no-deps --force-recreate new-api-dev`: passed; `new-api-dev` healthy.
- `curl -fsS http://localhost:3001/api/status`: passed.
- `python3 2dev/script/simulate_aggregate_client_pool_scenarios.py --attempts-per-scenario 1 --strict`: passed.
- `python3 2dev/script/simulate_aggregate_cluster_scenarios.py --attempts-per-scenario 1`: passed, 9/9.

---

## Session: 2026-04-29 Aggregate Group Single-Channel Internal Retry

### Scope
- 修正聚合分组在单渠道 / 单 priority 子分组场景下的组内 retry 语义。

### Progress
- 已确认原实现依赖 `priorityCount` 判断组内 retry，导致单渠道子分组失败后直接跨组 fallback。
- 已修正 failover 选路：当前真实分组有可用 channel 时，内部 retry 不因 priority 数量耗尽而提前切组。
- 已修正 cluster 选路：当前 route group 内部 retry 预算未耗尽时继续尝试当前 route group。
- 已补 failover 单渠道 / 单 priority 测试，验证 `RetryTimes=2` 时当前子分组先被尝试 3 次。
- 已补 cluster 单渠道 / 单 priority 测试，验证 `RetryTimes=2` 时当前子分组先被尝试 3 次，再切下一个子分组。
- 已更新 `2dev/doc/aggregate-group-design.md`、里程碑和迭代日志。

### Verification
- `go test ./service -run 'Aggregate.*Retry|CacheGetRandomSatisfiedChannelUsesAggregate'`: passed.
- `go test ./service ./controller ./model`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie eval/chunk size warnings.
- `docker build -t new-api-local:dev .`: passed.
- `docker compose -f docker-compose-dev.yml up -d --no-deps --force-recreate new-api-dev`: passed.

---

## Session: 2026-04-29 15m Sustained Fallback Load Test

### Scope
- 验证 token/body storage 性能优化后，在本地 fake upstream 缺失 usage、强制触发 `ResponseText2Usage` fallback 的持续流式场景下，是否仍出现 `system cpu overloaded` 或资源单调增长。

### Setup
- Docker dev service: `new-api-dev` on `http://127.0.0.1:3001`.
- Temporary test data: `perf15m_20260429182647` user/channel/token, channel pointed to `http://host.docker.internal:19081`.
- Load command shape: `duration=900s`, `concurrency=20`, `body-size=50KB`, `response-size=200KB`, `stream=true`, fake upstream omitted usage.
- Added `--stats-user-id` to `2dev/scripts/perf_sustained_load.py` so the script can call `/api/performance/stats` with root auth headers.

### Results
- Smoke test: `2083 / 2083` success, `system_cpu_overloaded=0`.
- 15m total: `17803 / 17803` success, `errors=0`, `status_503=0`, `system_cpu_overloaded=0`.
- Formal 15m consume logs: `17803`, all had `prompt_tokens=10`, `completion_tokens=100836`, confirming local fallback token estimation was exercised.
- Runtime stats windows:
  - `heap_alloc_mb`: `13.68 -> 30.35` max, last sampled window `17.15`, idle after run `10.98`.
  - `heap_sys_mb`: `68.49 -> 68.74`, no linear growth.
  - `goroutines`: `80 -> 136` max during bursts, final total `22`, idle after run `22`.
  - `memory_buffers`: max `20`, final/idle `0`.
  - `disk_files`: always `0`.
- Docker/proc samples:
  - RSS: first `71.82MB`, max `96.37MB`, last sample `76.77MB`, idle after run about `66.6MB`.
  - OS threads: `15 -> 18`, no continuous growth.
  - Docker CPU sampled max `247.72%`, last sample `0.02%`.
- Tail latency caveat: p99 had periodic `~68s` tails and 10 zero-completion windows because all 20 workers were waiting on long-tail stream requests. This did not correlate with heap/RSS/goroutine growth or 503s.

### Cleanup
- Deleted temporary logs, quota data, ability, token, channel, and user.
- Removed temporary group from `GroupRatio` and `UserUsableGroups`.
- Restored root `access_token` to empty and restarted `new-api-dev`; service returned healthy.

## Session: 2026-04-29 Token And Body Storage Performance Optimization

### Scope
- 在不改变业务逻辑和计费语义的前提下优化 token estimator、tokenizer cancellation、response usage fallback 和 body storage。

### Progress
- 已确认目标文件无已有本地 diff。
- 已读取 Horizon/PR 3428 相关记忆和本地 hot path。
- 已记录旧实现 golden 样本：
  - `gpt-4o` 英文 estimate `12`, CountTextToken `10`
  - `claude-sonnet-4-6` 中文 estimate/count `19`
  - `gemini-1.5-pro` URL estimate/count `26`
  - `claude-3.5` 数学符号 estimate/count `34`
  - `gpt-4o` emoji 混合 estimate `14`, CountTextToken `11`
  - `gemini-2.0-flash` 换行制表 estimate/count `12`
- 已将 tokenizer 依赖切换到 `github.com/Calcium-Ion/tokenizer v0.0.1`。
- 已新增 `CountTextTokenContext`，并将 request token counting 和 response fallback 接入 request context。
- 已优化 estimator：移除只读锁、数学符号/URL 分隔符查表、ASCII fast path。
- 已新增 `pooledMemoryStorage`，reader 内存路径复用 buffer，未知长度超过阈值时仍转磁盘缓存。
- 已新增 `2dev/scripts/perf_sustained_load.py`，支持 fake upstream、短压测/持续压测、窗口指标、进程指标、stats URL 和 pprof 抓取。

### Verification
- `go test ./service -run 'TestEstimateTokenByModelGolden|TestCountTextTokenGolden|TestCountTextTokenContextCancellation|TestResponseText2UsageGolden'`: passed.
- `go test ./common -run 'TestCreateBodyStorageFromReader'`: passed.
- `go test ./service ./common ./relay/channel/claude ./relay/channel/gemini ./controller`: passed.
- `go test ./service -bench 'EstimateToken|CountTextToken|ResponseText2Usage' -benchmem -count=5`: passed.
- `go test ./common -bench 'BodyStorage|CreateBodyStorageFromReader' -benchmem -count=5`: passed.
- `go test ./...`: passed.
- `git diff --check`: passed.
- `python3 -m py_compile 2dev/scripts/perf_sustained_load.py`: passed.
- `2dev/scripts/perf_sustained_load.py --start-fake-upstream --target http://127.0.0.1:19081/v1/chat/completions --duration 1 --concurrency 1 --body-size 1KB --response-size 2KB --sample-interval 1`: passed, 0 `system_cpu_overloaded`.

---

## Session: 2026-04-29 Aggregate Group And Channel Group Select UX

### Scope
- 优化聚合分组列表、聚合分组编辑抽屉和渠道新增/编辑中的分组选择体验。

### Progress
- 已在聚合分组列表名称列增加当前路由模式标签：`Failover 故障转移` / `Cluster 集群`。
- 已将聚合分组“可见用户组”候选过滤为 `default` 和 `UserGroup-*`。
- 已将聚合分组“添加真实分组”候选过滤掉 `default` 和 `UserGroup-*`。
- 已为聚合分组可见用户组和真实分组 Select 增加搜索过滤。
- 已为渠道新增/编辑弹窗里的分组 Select 增加搜索过滤。
- 已为 Failover / Cluster tab 切换增加确认弹窗；取消时不会修改 `routing_mode`。

### Verification
- `git diff --check`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie eval/chunk size warnings.

---

## Session: 2026-04-29 Aggregate Group Edit UI Mode Tabs

### Scope
- 重构聚合分组新增/编辑 SideSheet 的信息层级，让 Failover 和 Cluster 的配置项按模式分开展示。

### Progress
- 已确认当前编辑弹窗中路由模式、恢复、亲和、重试状态码、target 权重混在同一组表单里。
- 已将基础信息区收敛为名称、显示名、描述、倍率、启用状态、智能策略和可见用户组。
- 已新增 `Failover 故障转移` / `Cluster 集群分发` Tabs，切换 tab 会同步更新 `routing_mode`。
- 已将懒恢复和恢复间隔移动到 Failover tab。
- 已将 Cluster 亲和保持时间移动到 Cluster tab。
- 已将 target 列表改为按当前模式展示：Failover 显示链路顺序，Cluster 显示权重输入和权重含义说明。

### Verification
- `git diff --check`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie eval/chunk size warnings.

---

## Session: 2026-04-29 Cluster Affinity TTL Config

### Scope
- 新增每个聚合分组独立的 Cluster 亲和保持时间配置，默认 300 秒。

### Progress
- 已确认当前 route affinity TTL 复用 `recovery_interval_seconds`。
- 已确认 `RecordAggregateRouteAffinity` 只在 cluster 成功路径调用。
- 已新增 `aggregate_groups.cluster_affinity_ttl_seconds`，默认 300。
- 已将 route affinity TTL 改为使用 `cluster_affinity_ttl_seconds`。
- 已将 route affinity key 改为 `aggregate_group + user_id`，模型只做目标子分组能力校验。
- 已在编辑抽屉增加 `Cluster 亲和保持时间（秒）`，仅 Cluster 模式可编辑。
- 已更新 `2dev/doc/aggregate-group-design.md`、里程碑和迭代日志。

### Verification
- `go test -count=1 ./service ./controller ./model`: passed.
- `go test -count=1 ./...`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie eval/chunk size warnings.
- `docker build -t new-api-local:dev .`: passed.
- `docker compose -f docker-compose-dev.yml up -d --no-deps --force-recreate new-api-dev`: passed.
- `new-api-dev` health: healthy.
- Dev PostgreSQL column check: `cluster_affinity_ttl_seconds` exists with default 300.

---

## Session: 2026-04-27 Invite Commission Rule Simplification

### Scope
- 收敛返佣规则配置，删除未上线的 v1 服务基准和模型名归类规则。

### Changes
- 删除后端 `InviteCommissionSettings.service_rates` 和 `InviteCommissionSettings.model_rules` 字段及相关类型。
- 删除模型名通配归类函数，报表服务分类只读取已配置利润快照里的 `service`。
- 缺失快照、未配置分组或未命中利润规则的日志归入 `Other`，只展示消费，不参与 v2 返佣。
- 删除前端“服务返佣基准”和“模型归类规则”配置卡片，服务汇总表不再展示“服务基准”列。
- 更新设计文档、迭代开发日志和里程碑事件。

### Verification
- `go test ./model ./controller`: passed.
- `go test ./...`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie eval/chunk size warnings.
- `docker build -t new-api-local:dev . && docker compose -f docker-compose-dev.yml up -d --no-deps --force-recreate new-api-dev`: passed.
- `curl -fsS http://localhost:3001/api/status`: success.
- `GET /api/invite_commission/settings`: no `service_rates`, no `model_rules`.
- `new-api-dev`: healthy.

### Follow-up UI
- 新增 `InviteCommissionSettings.service_categories` 服务分类字典。
- `邀请返佣管理 -> 规则配置` 新增“服务分类”模块，可提前维护 GPT、Claude、Gemini、DeepSeek、Qwen 等服务分类。
- 分组利润规则弹窗里的“服务归类”改为从服务分类字典下拉选择。
- 自定义服务分类会规范成小写标识保存，后续报表按该服务单独汇总并使用字典里的显示名称。
- `cd web && bun run build`: passed.
- `docker build -t new-api-local:dev . && docker compose -f docker-compose-dev.yml up -d --no-deps --force-recreate new-api-dev`: passed.
- `GET /api/invite_commission/settings`: returns `service_categories` with GPT / Claude / Gemini / Other defaults.
- `new-api-dev`: healthy.

## Session: 2026-04-27 Invite Commission Profit Protection v2

### Scope
- 实现分组利润规则、消费日志利润快照、利润保护返佣报表、管理员 UI、文档、测试和 Docker dev 演示验证。

### Progress
- 已读取当前返佣、日志写入、分组/聚合分组和 Docker dev 配置。
- 已更新 `task_plan.md` 和 `findings.md` 到 v2 工作范围。
- 已完成后端分组利润规则、消费日志利润快照、报表利润保护字段和管理员接口。
- 已补模型测试：分组列表过滤、规则保存/清除、日志快照、用户日志过滤、利润保护、订阅保护、缺失快照、复杂邀请链。
- 已补控制器测试：分组利润规则查询/保存/清除和普通用户权限拒绝。
- 已完成前端 `邀请返佣管理 -> 规则配置 -> 分组利润规则`，以及管理员报表成本/毛利/利润保护展示。
- 已更新设计文档、迭代开发日志和里程碑事件。

### Verification So Far
- `go test ./model`: passed.
- `go test ./controller`: passed.
- `go test ./model ./controller`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie eval/chunk size warnings.
- `go test ./...`: passed.
- `docker build -t new-api-local:dev . && docker compose -f docker-compose-dev.yml up -d --no-deps --force-recreate new-api-dev`: passed.
- `curl -fsS http://localhost:3001/api/status`: success.
- `new-api-dev`: healthy.

### Docker Runtime Verification
- Run id: `20260427v2a`
- Prefix: `commission_profit_20260427v2a`
- Demo users:
  - A owner: `994100` / `commission_profit_20260427v2a_a`
  - B level 1: `994101` / `commission_profit_20260427v2a_b`
  - C level 2: `994102` / `commission_profit_20260427v2a_c`
  - D third-level: `994103` / `commission_profit_20260427v2a_d_third`
- Configured group profit rules:
  - `test_gpt-self`: GPT, profit `3000`, max commission `1500`, share `6000`
  - `claude-re-kiro` / `claude-re-kiro-002`: Claude, profit `2000`, max `1000`, share `6000`
  - `test-gemini-image`: Gemini, profit `5000`, max `2000`, share `6000`
  - `ikun_gpt-image-2`: GPT, profit `3000`, max `1500`, share `6000`
  - `test-juhe-gpt`: aggregate GPT, profit `3000`, max `1500`, share `6000`
  - `test-juhe-claude`: aggregate Claude, profit `2000`, max `1000`, share `6000`
- Runtime logs retained:
  - Controlled logs: `586`-`592`
  - Real relay logs: `593`-`598`
  - GPT / Claude / Gemini real calls all produced consume logs after moving B/C user group to `test-gemini-image` for Gemini access.
- A report `owner_user_id=994100`, `start_timestamp=1777287000`, `end_timestamp=1777290000`:
  - Invitees `2` (`B` level 1, `C` level 2), D excluded as third level.
  - Wallet recharge amount `1300000`, money `130`.
  - Redemption `700000`, subscription purchase `48`.
  - Consumption `509216`, cost `376029`, gross `125410`.
  - Theoretical `1864.3531`, cap/final commission `1856.6881`, reduced `7.665`, missing snapshot `7777`.
  - Independent SQL recomputation matched the API result exactly.
- B report `owner_user_id=994101`:
  - Invitees `2` (`C` level 1, `D` level 2).
  - Wallet recharge amount `1700000`, money `170`.
  - Redemption `900000`, subscription purchase `29`.
  - Consumption `375716`, cost `261512`, gross `114204`.
  - Theoretical/final commission `2494.296`.
  - Independent SQL recomputation matched the API result exactly.
- Self report for A:
  - Returned commission `1856.6881`.
  - Did not expose `groups`, `profit_rate_bps`, `upstream_cost_quota`, or `gross_profit_quota`.

## Session: 2026-04-27 Invite Commission Agent Enablement

### Changes
- 将返佣语义收敛为“邀请关系负责统计，用户配置负责代理返佣资格”。
- 未添加到 `invite_commission_user_configs` 或已关闭返佣的用户，报表仍展示邀请人数、充值、兑换和消费，但预估返佣为 0。
- 管理员 `邀请返佣管理 -> 用户配置` 的新增 / 编辑改为弹窗。
- 报表页在未启用代理返佣时展示提示，并将层级计算比例显示为 0，避免误认为默认比例仍在生效。
- 规则配置中的层级比例文案调整为“新增代理默认层级比例”。
- 更新邀请返佣设计文档、迭代开发日志和里程碑事件。

### Verification
- `go test ./model ./controller`: passed.
- `go test ./...`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie eval/chunk size warnings.
- `docker build -t new-api-local:dev .`: passed.
- `docker compose -f docker-compose-dev.yml up -d --no-deps --force-recreate new-api-dev`: passed.
- `new-api-dev` health: healthy.
- `curl -fsS http://localhost:3001/api/status`: success.

## Session: 2026-04-27 Invite Commission Complex Simulation

### Current Scope
- 使用 `commission_complex_20260427a` 前缀保留复杂业务演示数据。
- 目标是继续上一轮中断的 Docker runtime 验证，不删除已插入的用户、邀请码、订阅、订单、令牌和配置。

### Completed Before Resume
- 已补充复杂链路单元测试 `TestInviteCommissionComplexChainWithCustomConfigAndSubscriptions`。
- 已补充服务归类测试，锁定只按模型名或 `upstream_model_name` 归类。
- 已运行并通过：
  - `go test ./model ./controller`
  - `go test ./...`
  - `cd web && bun run build`

### Resume Notes
- 上一次 Docker 请求脚本在发真实请求前失败，错误为 zsh readonly 变量 `status`。
- 已创建复杂演示数据的基础对象，后续需要确认状态后从真实请求阶段继续。

### Docker Runtime Verification
- Run id: `20260427a`
- Prefix: `commission_complex_20260427a`
- 演示用户：
  - A: `993100` / `commission_complex_20260427a_a`
  - B: `993101` / `commission_complex_20260427a_b`
  - C1: `993102` / `commission_complex_20260427a_c1`
  - C2: `993103` / `commission_complex_20260427a_c2`
  - D1: `993104` / `commission_complex_20260427a_d1`
  - D2: `993105` / `commission_complex_20260427a_d2`
- 邀请链：A -> B -> C1/C2 -> D1/D2。
- 真实请求成功写入 45 条消费日志，日志 ID `541`-`585`。
- 每个被邀请用户均有 GPT / Claude / Gemini 各 3 条消费日志。
- B 已配置自定义返佣比例：一级 `800` bps，二级 `300` bps。
- 返佣规则保留在 dev 环境：默认一级/二级 `500`/`150` bps；GPT `2000` bps；Claude/Gemini `1000` bps；订阅档位 `0-33=1500`、`33-66=750`、`66-100=0`。

### Report Reconciliation
- A owner `993100` 报表：
  - 邀请人数 `3`，只包含 B/C1/C2，排除 D1/D2。
  - 钱包充值额度 `600`，真实充值金额 `60`。
  - 兑换码额度 `6000`，订阅购买金额 `36`。
  - 钱包消费额度 `43702`，订阅消费额度 `69455`。
  - 预估返佣 `459.2364`，与按日志独立重算一致。
- B owner `993101` 报表：
  - 邀请人数 `4`，包含 C1/C2/D1/D2。
  - 使用用户自定义比例 `800`/`300` bps。
  - 钱包充值额度 `1400`，真实充值金额 `140`。
  - 兑换码额度 `14000`，订阅购买金额 `54`。
  - 钱包消费额度 `24057`，订阅消费额度 `97217`。
  - 预估返佣 `991.4708`，与按日志独立重算一致。

### Regression Results
- `go test ./model ./controller`: passed.
- `go test ./...`: passed.
- `cd web && bun run build`: passed with existing Browserslist/lottie eval/chunk size warnings.
- `docker build -t new-api-local:dev .`: passed.
- `docker compose -f docker-compose-dev.yml up -d --no-deps --force-recreate new-api-dev`: passed.
- `new-api-dev` health: healthy.

## Session: 2026-04-27 Invite Commission Formula UI

### Changes
- 在管理员 `邀请返佣管理 -> 返佣报表` 中新增 `计算说明` 按钮。
- `计算说明` 打开 SideSheet，展示返佣流向图、核心公式、当前报表代入值、层级比例、订阅档位、层级拆分和服务拆分。
- 说明中明确充值、兑换码、订阅购买金额只展示，当前预估返佣默认只按消费计算。

### Verification
- `cd web && bun run build`: passed with existing Browserslist/lottie eval/chunk size warnings.
- `docker build -t new-api-local:dev .`: passed.

## Session: 2026-04-28 Horizon Source/Fix Investigation

### Scope
- Determine whether the closed Horizon Docker image can reveal performance fixes relevant to local `new-api`.

### Progress
- Confirmed public GitHub repository is documentation/tag oriented and does not expose application source code.
- Confirmed release notes mention performance improvements, especially request/response stream handling and large body high-concurrency CPU/memory savings.
- Docker Hub HTTP API timed out; switching to registry tooling for image metadata/content inspection.
- Inspected `calciumion/new-api-horizon:latest` image metadata and copied binary from `/new-api` for local analysis.
- Confirmed binary is stripped, so full source cannot be recovered from symbols.
- Extracted Go build info: Horizon is version `0.5.5`, built with `GOEXPERIMENT=greenteagc`, and uses `github.com/Calcium-Ion/tokenizer v0.0.1` instead of local `github.com/tiktoken-go/tokenizer v0.6.2`.
- Compared tokenizer fork against upstream: Horizon's tokenizer API accepts `context.Context` and checks cancellation during regex tokenization and BPE merge loops.
- Searched binary strings and found Horizon-only `StreamingPerformanceOptimization` / `performance.streaming_performance_optimization` plus disk cache and large-content performance UI strings.
- Installed temporary Go reverse-engineering helpers in `/tmp/go-bin`: GoReSym and redress.
- Ran GoReSym/redress against `/tmp/horizon-image/new-api`; recovered package/source projection, function spans, and type metadata despite stripped ELF.
- Confirmed Horizon has `common.pooledMemoryStorage` methods and larger `CreateBodyStorageFromReader`, indicating body reuse/pooling/streaming optimizations beyond the local binary.
- Confirmed Horizon `service.getTokenNum` has nested closure metadata carrying `context.Context`, `tokenizer.Codec`, input text, and result/error channels, indicating a cancellable/asynchronous token count wrapper.
- `docker compose -f docker-compose-dev.yml up -d --no-deps --force-recreate new-api-dev`: passed.
- `/api/status`: success.
- `new-api-dev` health: healthy.

## Session: 2026-04-27 Invite Commission User Config Edit

### Changes
- 在管理员 `邀请返佣管理 -> 用户配置` 的已配置用户表格中新增 `操作 / 编辑` 列。
- 点击 `编辑` 后自动把该用户、启用状态、一级比例、二级比例和备注带回上方表单。
- 保存仍使用现有用户配置接口覆盖该用户配置，并刷新配置列表。

### Verification
- `cd web && bun run build`: passed with existing Browserslist/lottie eval/chunk size warnings.
- `docker build -t new-api-local:dev .`: passed.
- `docker compose -f docker-compose-dev.yml up -d --no-deps --force-recreate new-api-dev`: passed.
- `/api/status`: success.
- `new-api-dev` health: healthy.

## Session: 2026-04-25

### Phase 1: Discovery
- **Status:** complete
- Actions taken:
  - 读取 `AGENTS.md` 和技能说明。
  - 确认本次任务目标、边界和测试要求。
  - 将 planning files 切换为本次任务。
  - 阅读 `model/user.go`、`model/invite_code.go`、`controller/user.go`、`controller/invite_code.go`、`router/api-router.go`。
  - 阅读用户管理和邀请码管理前端表格、hooks、弹窗代码。

### Phase 2: Backend Implementation
- **Status:** complete
- Actions taken:
  - 新增模型层绑定、重绑、解绑事务逻辑。
  - 新增后台用户邀请绑定接口和 owner 邀请码列表接口。
  - 新增管理日志内容，记录旧/新邀请人与旧/新邀请码。
- Files created/modified:
  - `model/invite_code.go`
  - `controller/user.go`
  - `router/api-router.go`

### Phase 3: Backend Tests
- **Status:** complete
- Actions taken:
  - 补充模型层统计归属测试。
  - 补充控制器管理日志和 bindable 邀请码列表测试。
- Files created/modified:
  - `model/invite_code_test.go`
  - `controller/user_invite_binding_test.go`

### Phase 4: Frontend Implementation
- **Status:** complete
- Actions taken:
  - 开始接入用户管理表格操作和绑定弹窗。
  - 用户操作菜单新增“绑定邀请人”。
  - 新增绑定弹窗，支持当前绑定展示、目标邀请人搜索、邀请码选择、自动 MANUAL 码和解绑。
- Files created/modified:
  - `web/src/components/table/users/UsersTable.jsx`
  - `web/src/components/table/users/UsersColumnDefs.jsx`
  - `web/src/components/table/users/modals/InviteBindingModal.jsx`

### Phase 5: Docs & Verification
- **Status:** complete
- Actions taken:
  - 完成前端生产构建验证。
  - 更新迭代开发日志和里程碑事件。
  - 新增手动绑定邀请归属设计说明。
  - 完成整仓 Go 测试。
  - 完成 Docker dev 镜像构建、compose 单服务重建和状态检查。
- Files created/modified:
  - `2dev/doc/迭代开发日志.md`
  - `2dev/doc/二次开发里程碑事件.md`
  - `2dev/doc/manual-invite-binding-design.md`
- Files created/modified:
  - `task_plan.md`
  - `findings.md`
  - `progress.md`

## Test Results
| Test | Input | Expected | Actual | Status |
|------|-------|----------|--------|--------|
| 后端局部测试 | `go test ./model ./controller` | 通过 | 通过 | ✓ |
| 前端构建 | `cd web && bun run build` | 通过 | 通过（存在既有 Browserslist/eval/chunk 警告） | ✓ |
| 整仓后端测试 | `go test ./...` | 通过 | 通过 | ✓ |
| Docker 镜像构建 | `docker build -t new-api-local:dev .` | 通过 | 通过 | ✓ |
| Docker dev 重建 | `docker compose -f docker-compose-dev.yml up -d --no-deps --force-recreate new-api-dev` | 服务启动 | `new-api-dev` started / healthy | ✓ |
| Docker 健康检查 | `curl -fsS http://localhost:3001/api/status` | `success=true` | `success=true` | ✓ |
| 图片大 base64 响应局部测试 | `go test ./relay/channel/openai -run 'TestDoResponseWithoutImageAdapterKeepsLargeBase64ImageResponse|TestDoResponseWithImageStreamPassesThroughSSEAndExtractsUsage' -count=1` | 通过 | 通过 | ✓ |
| 图片 relay 回归测试 | `go test ./dto ./relay/... -count=1` | 通过 | 通过 | ✓ |
| Docker dev 渠道 9 4K 非流式生图 | `POST /v1/images/generations`，`model=gpt-image-2`，`size=3840x2160` | 响应完整可解析 | HTTP 200，`Content-Length=size_download=file_bytes=5106057`，`b64_json=5105432`，base64 decode 成功，PNG `3840x2160` | ✓ |
| Docker dev 渠道 9 高复杂度 4K 补测 | 两个更高细节 prompt，`size=3840x2160` | 尽量获得更大响应 | 均为上游 `502 Upstream request failed`，new-api 返回完整 143B 错误 JSON；未出现截断迹象 | ⚠ |

## Error Log
| Timestamp | Error | Attempt | Resolution |
|-----------|-------|---------|------------|
| 2026-05-06 18:58 CST | 使用 token id 16 请求 4K 生图返回 `401 无效的令牌` | 从 DB 查询到 token 但未过滤 `deleted_at` | 确认 token id 16 已软删除，改用临时测试用户/token |
| 2026-05-06 18:59 CST | 未删除的 `ikun_gpt-image-2` token 返回 `403 无权访问 ikun_gpt-image-2 分组` | 用历史 commission token 请求 `/v1/models` | 确认用户自身分组不匹配，临时创建用户分组和 token 分组均为 `ikun_gpt-image-2` 的测试凭证，测试后已清理 |
| 2026-05-06 19:08 CST | 高复杂度 4K prompt 返回 `500 service_unavailable` | 密集微图案 prompt | 日志显示渠道 9 上游 `502 Upstream request failed`，不是响应截断 |
| 2026-05-06 19:10 CST | 自然高细节 4K prompt 返回 `500 service_unavailable` | 植物园高细节 prompt | 日志显示渠道 9 上游 `502 Upstream request failed`，不是响应截断 |

## Docker Dev 4K Image Response Test
- Environment: `new-api-dev` image `new-api-local:dev`, `http://localhost:3001`, channel 9 `gpt-image-2自建`, base URL `http://152.53.177.60:18080`.
- Created temporary local test user/token in group `ikun_gpt-image-2` because existing usable image tokens were either soft-deleted or blocked by user usable group checks; cleanup removed the temp user, token, logs, and quota data.
- Successful request id: `20260506110157446567795PjGDgsGR`.
- Successful response metrics: HTTP `200`, `Content-Length: 5106057`, curl `size_download=5106057`, file bytes `5106057`, total time `281.208537s`, first byte `281.188455s`.
- Successful response validation: JSON parsed, top-level keys `background/created/data/model/output_format/quality/size/usage`, `data[0].b64_json` length `5105432`, base64 length mod 4 is `0`, decoded PNG bytes `3829074`, dimensions `3840x2160`, decoded SHA-256 `09caeee6eeadea6ed38b70813307683b728d6e20963c04f85f3ff3fd712e7d3e`.
- Log scan for the successful request showed normal consume log and `POST /v1/images/generations` 200; no `bad_response_body`, `failed to copy response body`, `broken pipe`, `connection reset`, or `unexpected EOF`.
- Caveat: the successful 4K PNG compressed to about `3.83MB`, so its base64 JSON was about `5.1MB`; two attempts to force a larger, more complex 4K response failed upstream with `502` before returning a large body.

## 5-Question Reboot Check
| Question | Answer |
|----------|--------|
| Where am I? | Phase 1 discovery |
| Where am I going? | 实现后端接口、测试、前端弹窗和文档 |
| What's the goal? | 管理员可手动绑定/重绑/解绑用户邀请统计归属 |
| What have I learned? | 任务边界是不补奖励、不做历史分账，只改统计归属字段 |
| What have I done? | 读取项目约束并建立任务计划 |

---

# Session: 日志看板耗时分析

## Scope
- 按用户给定计划实现后端 latency 聚合、前端展示和测试验证。

## Progress
- 已确认旧 planning 文件存在，采用追加方式记录本任务，避免覆盖旧内容。
- 已完成后端 latency 聚合：渠道、分组、渠道模型维度，nearest-rank P50/P90/P95/最大/平均。
- 已完成日志看板耗时分析 Card：维度 Tabs、筛选、P95 Top 10 横向柱状图、Top 20 表格和样本较少标记。
- 验证通过：
  - `go test ./service -count=1`
  - `go test ./controller ./model -count=1`
  - `cd web && bun run build`
  - `git diff --check`
