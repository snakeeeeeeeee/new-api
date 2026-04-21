# 日志看板 V1 设计

更新时间：2026-04-21

## 目标

新增一个管理员可见的“日志看板”菜单，面向运行态观测，不替代现有“使用日志”明细页。

第一版目标：

- 提供 `1h / 6h / 24h` 三档窗口切换
- 展示平台最终成功率 / 最终失败率
- 展示各渠道尝试成功率 / 失败率
- 展示各分组最终成功率 / 失败率
- 展示 Top 错误信息与 Top 状态码
- 默认排除健康探针等 `LogConsumeExcludedUserIDs` 用户

## 数据来源

直接复用现有 `logs` 表，不新增表，不做 migration。

仅使用以下日志类型：

- `LogTypeConsume`
- `LogTypeError`

仅统计选定时间窗口内的日志。

## 统计口径

### 平台总览

平台总览按 `request_id` 去重。

规则：

- `request_id` 为空的日志不参与总览和趋势
- 同一 `request_id` 只要存在 `consume` 日志，就记为最终成功
- 同一 `request_id` 没有 `consume`、但存在 `error`，记为最终失败
- 平均耗时只统计最终成功请求，取最终成功那条 `consume.use_time`

这样可以避免聚合分组 fallback、渠道重试、多次 error log 把平台错误率拉高。

### 渠道维度

渠道维度按“渠道尝试”统计，不按 `request_id` 去重。

规则：

- 每条 `consume/error` 渠道日志都算一次尝试
- `consume` 记成功尝试
- `error` 记失败尝试
- 平均耗时按这些尝试日志的 `use_time` 平均

这样更接近运维视角，可以真实反映渠道健康度。

### 分组维度

分组维度按“最终请求”统计，不按尝试数统计。

规则：

- 最终成功请求计入其最终成功日志所在分组
- 最终失败请求计入其最后一条错误日志所在分组
- 成功率 / 失败率按最终请求数计算
- 平均成功耗时只统计最终成功请求
- Top 状态码与 Top 错误信息按该分组内最终失败请求聚合

这样更接近业务分组视角，不会因为组内 fallback 或渠道重试把分组失败率放大。

如果某个分组命中聚合分组表，则在前端分组统计中打 `聚合` 标记，用于与普通真实分组区分。

### 错误信息

总体 Top 错误信息按“最终失败请求”的最后一条错误聚合。

渠道 Top 错误信息按“该渠道失败尝试”聚合。

错误信息归一化规则：

- 去掉 `(request id: xxx)` 这类动态尾巴
- 合并连续空白字符
- trim 首尾空白

### 探针排除

`LogConsumeExcludedUserIDs` 命中的用户，日志看板整体排除：

- 不参与总览
- 不参与趋势
- 不参与渠道统计
- 不参与 Top 错误 / Top 状态码

这样可以避免健康探针账号把失败率和错误分布放大。

## 时间窗口与分桶

- `1h`：5 分钟一桶
- `6h`：15 分钟一桶
- `24h`：1 小时一桶

V1 仅做固定三档，不做自定义时间范围。

## 后端接口

新增接口：

- `GET /api/log/dashboard?window=1h|6h|24h`

权限：

- `AdminAuth`

返回结构包含：

- `window`
- `generated_at`
- `summary`
- `trend`
- `channels`
- `top_error_messages`
- `top_status_codes`

## 前端页面

新增管理员菜单：

- `日志看板`

路由：

- `/console/log-dashboard`

页面布局：

- 顶部操作区：
  - `1h / 6h / 24h`
  - 手动刷新
  - 自动刷新提示
  - 最近更新时间
- 总览卡：
  - 总请求数
  - 最终成功率
  - 最终失败率
  - 平均成功耗时
- 趋势图：
  - 成功数 / 失败数
- 渠道表：
  - 渠道
  - 尝试数
  - 成功数
  - 失败数
  - 成功率
  - 失败率
  - 平均耗时
  - Top 状态码
  - Top 错误信息
- 分组表：
  - 分组
  - 总请求数
  - 成功数
  - 失败数
  - 成功率
  - 失败率
  - 平均成功耗时
  - Top 状态码
  - Top 错误信息
- 错误分布：
  - Top 错误信息
  - Top 状态码

## 回归要求

必须保证：

- 现有“使用日志”列表和统计接口不受影响
- 现有 `LogConsumeExcludedUserIDs` 行为不受影响
- 侧边栏模块设置仍可正常工作
- 聚合分组、邀请码、订阅、充值等既有菜单和路由不受影响

## 验证要求

- `go test ./controller ./model ./service`
- `go test ./...`
- `cd web && bun run build`
- `docker build -t new-api-local:dev .`
- `docker compose -f docker-compose-dev.yml up -d --force-recreate new-api-dev`

docker dev 验证样本：

- 一个请求先在渠道 A 失败，再在渠道 B 成功
- 两个最终失败请求，错误内容仅 request id 不同
- 一个探针账号失败样本
- 一个 `request_id` 为空的渠道失败样本
- 一次空窗口校验
