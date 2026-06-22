# image-handle 对接 new-api 内部执行模式

## 1. 背景

new-api 里已经存在真实图片渠道，例如 `gpt-image-2`。这些渠道已经维护了真实上游的 `base_url`、`api_key`、模型映射、分组、倍率和计费规则。

异步生图时，我们希望继续复用这些现有渠道，而不是在 new-api 里额外创建一个 `ImageHandle` 模型渠道，也不希望在 image-handle 里再维护一套 OpenAI、xAI 或其他 provider key。

因此，image-handle 的定位调整为：

- 异步队列
- Worker 执行调度
- R2 上传
- 状态库
- 终态 callback

真实上游图片模型调用由 new-api 的内部执行接口完成。

## 2. 目标流程

```text
用户
  -> new-api POST /v1/image/tasks
  -> new-api 按 model 选择已有图片渠道，例如 gpt-image-2
  -> new-api 预扣费、创建任务、锁定 channel_id
  -> new-api 提交异步任务到 image-handle
  -> image-handle worker 消费任务
  -> image-handle 调用 new-api internal execute
  -> new-api 使用已锁定渠道调用真实上游
  -> new-api 返回图片结果给 image-handle
  -> image-handle 上传 R2
  -> image-handle callback new-api
  -> new-api 更新任务、结算或退款、写 assets
```

## 3. 核心变化

### 原逻辑

```text
image-handle 自己维护 provider key
image-handle 自己根据 provider/model 调 OpenAI、xAI 等上游
new-api 需要把 ImageHandle 当成一个独立模型渠道
```

### 新逻辑

```text
new-api 继续维护真实上游渠道
new-api 根据 model 选择已有图片渠道
image-handle 只负责异步编排、上传和 callback
image-handle 通过 new-api internal execute 间接完成真实生图
```

## 4. image-handle 执行模式

image-handle 新模式只保留 executor 类型：

```text
new_api_internal
```

含义：

```text
该任务不由 image-handle 直接调用 provider。
image-handle worker 需要调用 new-api 的 internal execute 接口完成真实生图。
```

执行要求：

| 场景 | 行为 |
| --- | --- |
| `executor.type = "new_api_internal"` | 调用 new-api internal execute |
| 请求没有 `executor` | 返回失败，标记不可重试 |
| `executor.type` 未知 | 返回失败，标记不可重试 |

provider 直连模式废弃。image-handle 不再自行选择 provider，也不再保存 OpenAI、xAI 或其他真实上游 key。

## 5. new-api 配置

new-api 侧需要配置 image-handle 执行器地址和签名密钥。推荐在后台：

```text
异步任务管理 -> 异步图片执行器
```

填写并保存，配置会落到 `options` 表。下面这些环境变量仍然保留为启动兜底，适合 Docker 首次启动或无后台权限的部署场景。

| 变量 | 必填 | 示例 | 说明 |
| --- | --- | --- | --- |
| `IMAGE_HANDLE_BASE_URL` | 是 | `http://image-handle:8787` | new-api 提交任务到 image-handle 的内网地址 |
| `IMAGE_HANDLE_API_KEY` | 是 | `test-api-key` | new-api 调用 image-handle submit/query 接口的 key |
| `IMAGE_HANDLE_INTERNAL_BASE_URL` | 是 | `http://new-api:3000` | image-handle 容器访问 new-api internal execute 的内网 base URL |
| `IMAGE_HANDLE_INTERNAL_SECRET_ID` | 是 | `image_handle_1` | internal execute 签名密钥 ID |
| `IMAGE_HANDLE_INTERNAL_SECRET` | 是 | `internal-secret-xxx` | internal execute HMAC secret |
| `IMAGE_HANDLE_CALLBACK_SECRET` | 否 | `callback-secret-xxx` | 旧配置兜底 callback secret；正式建议用真实图片渠道的 `settings.callback_secret` |

注意：

- `IMAGE_HANDLE_INTERNAL_SECRET` 必须和真实图片渠道的 `settings.callback_secret` 分开。
- `/v1/image/tasks` 选中的真实图片渠道必须配置 `settings.callback_secret`，new-api 会用 `channel_<channel_id>` 作为 `callback.secret_id`。
- `IMAGE_HANDLE_INTERNAL_BASE_URL` 必须是 image-handle 容器或生产 worker 可以访问的地址，不一定是公网地址。
- 真实上游 OpenAI/xAI key 仍然只配置在 new-api 原有图片渠道里。

## 6. new-api 提交给 image-handle 的任务格式

### 请求

```http
POST /v1/image/tasks
Authorization: Bearer <image_handle_provider_key>
Content-Type: application/json
```

```json
{
  "request_id": "req_xxx",
  "client_task_id": "task_xxx",
  "model": "gpt-image-2",
  "operation": "generation",
  "input": {
    "text": "吃铜锣烧的机器猫",
    "images": [],
    "mask": null
  },
  "parameters": {
    "size": "2560x1440",
    "quality": "auto",
    "n": 1,
    "output_format": "webp",
    "output_compression": 85
  },
  "executor": {
    "type": "new_api_internal",
    "execute_url": "https://new-api.example.com/api/internal/image/tasks/task_xxx/execute",
    "secret_id": "image_handle_1"
  },
  "callback": {
    "url": "https://new-api.example.com/api/task/callback/external-image/task_xxx",
    "batch_url": "https://new-api.example.com/api/task/callback/external-image/batch",
    "secret_id": "channel_123"
  },
  "metadata": {
    "tenant_id": "user_123",
    "channel_id": "channel_123",
    "new_api_task_id": "task_xxx"
  }
}
```

### 字段说明

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `request_id` | string | 是 | new-api 请求 ID，用于链路追踪 |
| `client_task_id` | string | 是 | new-api 对外任务 ID |
| `model` | string | 是 | 用户请求的模型，例如 `gpt-image-2` |
| `operation` | string | 是 | `generation` 或 `edit` |
| `input.text` | string | 是 | prompt |
| `input.images` | string[] | 否 | 图生图或编辑输入图 |
| `input.mask` | string/null | 否 | 图片编辑 mask |
| `parameters` | object | 否 | 生图参数 |
| `executor.type` | string | 是 | 新模式固定为 `new_api_internal` |
| `executor.execute_url` | string | 是 | image-handle worker 调用的 new-api 内部执行地址 |
| `executor.secret_id` | string | 是 | internal execute 签名密钥 ID |
| `callback.url` | string | 是 | 单任务 callback 地址 |
| `callback.batch_url` | string | 否 | 批量 callback 地址 |
| `callback.secret_id` | string | 是 | callback 签名密钥 ID |
| `metadata` | object | 否 | 非敏感追踪信息 |

注意：

- `new_api_internal` 模式下，image-handle 不应该根据 `model` 自己选择 provider。
- `provider`、`provider_options` 在该模式下可以忽略。
- 真实上游调用由 new-api 内部执行接口完成。
- `operation=edit` 时，new-api 会保存 `input.images` 和 `input.mask` URL，并在 internal execute 中通过现有下载安全策略转成 `/v1/images/edits` multipart 请求。

## 7. image-handle worker 执行逻辑

```text
if task.executor.type == "new_api_internal":
    result = call_new_api_internal_execute(task)
else:
    result = call_provider_directly(task)

if result.status == "succeeded":
    upload result images to R2
    mark task succeeded
    callback new-api succeeded
else if result.status == "failed" and result.error.retryable == true:
    retry with backoff
else:
    mark task failed
    callback new-api failed
```

## 8. new-api internal execute 协议

### 请求

```http
POST /api/internal/image/tasks/{task_id}/execute
Content-Type: application/json
X-ImageHandle-Timestamp: 1782140000
X-ImageHandle-Signature: <signature>
X-ImageHandle-Event-Id: evt_xxx
X-ImageHandle-Secret-Id: image_handle_1
```

```json
{
  "provider_task_id": "imgtask_xxx",
  "attempt": 1
}
```

### 请求字段说明

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `provider_task_id` | string | 是 | image-handle 内部任务 ID |
| `attempt` | number | 是 | 当前执行次数 |

### 签名规则

```text
signature = HMAC-SHA256(timestamp + "." + raw_body, internal_secret)
```

要求：

- `timestamp` 建议 5 分钟有效期。
- `signature` 使用 constant-time compare。
- `secret_id` 用于 new-api 查找对应 internal secret。
- `provider_task_id` 必须匹配 new-api 保存的 image-handle 任务 ID。
- `event_id` 用于日志追踪；new-api 会缓存非重试执行结果，重复 execute 不会重复调用真实上游。

## 9. new-api internal execute 响应

### 成功响应

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "status": "succeeded",
  "images": [
    {
      "url": "https://example.com/original-image.png",
      "b64_json": null,
      "mime_type": "image/png"
    }
  ],
  "usage": {
    "actual_quota": 1234
  }
}
```

说明：

- `images[].url` 或 `images[].b64_json` 至少有一个。
- image-handle 拿到结果后继续上传 R2。
- image-handle callback 给 new-api 时，返回 R2 后的最终 URL。

### 失败响应

```http
HTTP/1.1 200 OK
Content-Type: application/json
```

```json
{
  "status": "failed",
  "error": {
    "code": "upstream_error",
    "message": "upstream provider error message",
    "retryable": true
  }
}
```

### 失败字段说明

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `error.code` | string | 错误码 |
| `error.message` | string | 错误描述 |
| `error.retryable` | boolean | image-handle 是否应该重试 |

## 10. internal execute HTTP 状态处理建议

| HTTP 状态 | image-handle 行为 |
| --- | --- |
| `200` 且 `status=succeeded` | 上传 R2，然后 callback 成功 |
| `200` 且 `status=failed`，`retryable=true` | 退避重试 |
| `200` 且 `status=failed`，`retryable=false` | callback 失败 |
| `401` / `403` | 不可重试，internal secret 或权限错误 |
| `404` | 不可重试，任务不存在或已失效 |
| `409` | 通常不可重试，任务状态冲突 |
| `5xx` / timeout | 可重试 |

补充：

- internal execute 返回 `200 + status=failed + retryable=true` 时，new-api 会释放执行占用，允许 image-handle 后续重试。
- internal execute 返回成功或不可重试失败后，new-api 会缓存该响应；同一任务重复 execute 时直接返回缓存结果，避免重复生图。

## 11. callback 协议保持不变

image-handle 终态 callback new-api 的地址保持：

```http
POST /api/task/callback/external-image/{task_id}
POST /api/task/callback/external-image/batch
```

### 成功 callback 示例

```json
{
  "event_id": "evt_xxx",
  "client_task_id": "task_xxx",
  "provider_task_id": "imgtask_xxx",
  "status": "succeeded",
  "progress": "100%",
  "result": {
    "images": [
      {
        "url": "https://r2.example.com/path/to/image.webp"
      }
    ]
  },
  "usage": {
    "actual_quota": 1234
  }
}
```

### 失败 callback 示例

```json
{
  "event_id": "evt_xxx",
  "client_task_id": "task_xxx",
  "provider_task_id": "imgtask_xxx",
  "status": "failed",
  "progress": "100%",
  "error": {
    "code": "upstream_error",
    "message": "upstream provider error message",
    "retryable": false
  }
}
```

## 12. 幂等要求

image-handle 侧建议保证：

- `client_task_id` 唯一。
- 重复提交同一个 `client_task_id` 返回同一个 `provider_task_id`。
- `new_api_internal` 执行失败重试时，不重复创建 image-handle 任务。
- callback 可以重复发送，new-api 会做终态 CAS，但 image-handle 也应保留 callback event 日志便于排查。

new-api internal execute 侧建议保证：

- 同一个 `task_id` 可以被 image-handle 重试调用。
- new-api 根据任务状态决定是否允许再次执行。
- 如果任务已经终态，返回明确错误，避免重复调用上游。

## 13. 安全要求

- image-handle 不保存真实 provider key。
- image-handle 不接收 new-api 渠道 API key。
- internal execute 必须 HMAC 签名。
- callback 仍然 HMAC 签名。
- `metadata` 只保存非敏感追踪信息。
- 不在日志里打印完整密钥、Authorization header 或 callback secret。

## 14. 这样做的好处

- 不需要在 new-api 里新增 `gpt-image-2-async` 这种重复模型。
- 不需要在 image-handle 里维护 OpenAI、xAI 或其他 provider key。
- 同步生图和异步生图复用同一套渠道配置。
- 同步和异步复用同一套模型倍率、分组权限、模型映射。
- 任务日志里的 `channel_id` 指向真实生图渠道，而不是 ImageHandle 壳渠道。
- 后续 xAI、OpenAI、其他图片渠道都可以自然复用。

## 15. 验收标准

- new-api 调用 `/v1/image/tasks` 时，可以使用现有模型名 `gpt-image-2`。
- image-handle 收到任务后，识别 `executor.type = "new_api_internal"`。
- image-handle worker 调用 `executor.execute_url`，不直接调用 provider。
- new-api internal execute 使用任务锁定的真实渠道完成上游生图。
- image-handle 上传 R2 后 callback new-api。
- new-api 成功更新任务状态、写入 assets。
- 失败场景可以按 `retryable` 正确重试或失败 callback。

## 16. 用户侧 new-api 调用示例

最终用户只调用 new-api，不需要传 image-handle 的 `provider_task_id`，也不需要知道 image-handle 服务鉴权 key。

```bash
curl --location 'https://api.example.com/v1/image/tasks' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer sk-xxx' \
  -d '{
    "model": "gpt-image-2",
    "prompt": "吃铜锣烧的机器猫",
    "size": "2560x1440",
    "metadata": {
      "quality": "auto",
      "n": 1,
      "output_format": "png"
    }
  }'
```

返回：

```json
{
  "task_id": "task_xxx",
  "status": "queued"
}
```

查询：

```bash
curl --location 'https://api.example.com/v1/image/tasks/task_xxx' \
  -H 'Authorization: Bearer sk-xxx'
```

注意：`IMAGE_HANDLE_API_KEY` 是 new-api 调 image-handle 的服务 key，不是最终用户的 `sk-xxx`。
