# image-handle provider_direct_lease 对接说明

## 1. 背景

new-api 已经维护真实图片渠道，例如 OpenAI 兼容的 `gpt-image-2` 或 xAI 视频/图片渠道。这些渠道包含真实上游的 `base_url`、`api_key`、模型映射、分组、倍率和计费规则。

异步图片任务不再把 image-handle 当成一个模型渠道使用。new-api 负责选择并锁定真实渠道，image-handle 只负责异步队列、worker 执行、R2 上传、任务状态和终态 callback。

真实上游凭证不放进提交任务 payload。image-handle worker 开始执行前，通过短期 lease 向 new-api resolve 本次任务锁定的真实渠道凭证。

## 2. 总体流程

```text
用户
  -> new-api POST /v1/image/tasks
  -> new-api 鉴权、分组、模型映射、选择真实图片渠道
  -> new-api 预扣费、创建 tasks 记录、创建 credential lease
  -> new-api 提交 provider_direct_lease 任务给 image-handle
  -> image-handle worker 消费任务
  -> image-handle worker 调用 lease resolve 接口领取短期凭证
  -> image-handle worker 直连真实上游生图/编辑图
  -> image-handle 上传结果到 R2
  -> image-handle callback new-api
  -> new-api CAS 更新任务终态、结算或退款、写 assets
```

## 3. new-api 侧职责

- 用户鉴权和权限校验。
- 聚合分组最终选中真实子渠道。
- 预扣费和任务记录。
- 创建 `image_credential_leases`，只保存真实 `channel_id`，不保存明文 API key。
- 提供 HMAC 保护的 resolve 接口。
- 接收终态 callback，成功结算并写 assets，失败退款。

## 4. image-handle 侧职责

- 接收 new-api 提交的异步任务。
- 入队和 worker 调度。
- worker resolve lease 后直连真实上游。
- 上传 R2。
- 写 image-handle 自己的任务状态。
- 终态 callback new-api。

image-handle 不应该持久化、打印或展示 resolve 得到的 `api_key`。

## 5. new-api 配置

配置入口：

```text
异步任务管理 -> 异步图片执行器
```

配置会保存到 `options.image_handle_setting`。环境变量仍作为启动兜底：

| 配置 | 必填 | 示例 | 说明 |
| --- | --- | --- | --- |
| image-handle 服务地址 | 是 | `http://image-handle:8787` | new-api 提交任务到 image-handle 的地址 |
| image-handle API Key | 是 | `test-api-key` | new-api 调 image-handle submit/query 接口的 key |
| internal resolve 访问地址 | 是 | `http://new-api:3000` | image-handle 容器可访问的 new-api 内网地址 |
| internal resolve Secret ID | 是 | `image_handle_1` | resolve HMAC secret id |
| internal resolve Secret | 是 | `internal-secret-xxx` | resolve HMAC secret |
| Callback Secret | 否 | `callback-secret-xxx` | 兜底 callback secret；正式建议配置到真实图片渠道 |

对应环境变量：

| 环境变量 | 说明 |
| --- | --- |
| `IMAGE_HANDLE_BASE_URL` | image-handle 服务地址 |
| `IMAGE_HANDLE_API_KEY` | image-handle API Key |
| `IMAGE_HANDLE_INTERNAL_BASE_URL` | internal resolve 访问地址 |
| `IMAGE_HANDLE_INTERNAL_SECRET_ID` | internal resolve Secret ID |
| `IMAGE_HANDLE_INTERNAL_SECRET` | internal resolve Secret |
| `IMAGE_HANDLE_CALLBACK_SECRET` | callback 兜底 Secret |

注意：

- `internal resolve Secret` 必须和 callback secret 分开。
- 真实图片渠道建议配置 `settings.callback_secret`，new-api 会用 `channel_<channel_id>` 作为 callback secret id。
- `internal resolve 访问地址` 必须是 image-handle worker 可访问的地址。Docker 内通常不能填只对浏览器有意义的 `localhost`。

## 6. new-api 提交给 image-handle 的任务

```http
POST /v1/image/tasks
Authorization: Bearer <image_handle_api_key>
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
    "output_format": "png"
  },
  "executor": {
    "type": "provider_direct_lease",
    "lease_id": "lease_xxx",
    "resolve_url": "http://new-api:3000/api/internal/image/credential-leases/lease_xxx/resolve",
    "secret_id": "image_handle_1"
  },
  "callback": {
    "url": "http://new-api:3000/api/task/callback/external-image/task_xxx",
    "batch_url": "http://new-api:3000/api/task/callback/external-image/batch",
    "secret_id": "channel_123"
  },
  "metadata": {
    "tenant_id": "user_123",
    "channel_id": "channel_123"
  }
}
```

字段说明：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `request_id` | string | 是 | new-api 请求 ID |
| `client_task_id` | string | 是 | new-api 对外任务 ID |
| `model` | string | 是 | 锁定渠道后的上游模型名 |
| `operation` | string | 是 | `generation` 或 `edit` |
| `input.text` | string | 是 | prompt |
| `input.images` | string[] | 否 | 编辑图输入图 URL |
| `input.mask` | string/null | 否 | 编辑图 mask URL |
| `parameters` | object | 否 | 图片参数，例如 size、quality、n |
| `executor.type` | string | 是 | 固定 `provider_direct_lease` |
| `executor.lease_id` | string | 是 | new-api 创建的短期凭证 lease |
| `executor.resolve_url` | string | 是 | image-handle worker 领取凭证的 URL |
| `executor.secret_id` | string | 是 | resolve HMAC Secret ID |
| `callback.url` | string | 是 | 单任务 callback 地址 |
| `callback.batch_url` | string | 否 | 批量 callback 地址 |
| `callback.secret_id` | string | 是 | callback HMAC Secret ID |
| `metadata` | object | 否 | 非敏感追踪信息 |

提交任务 payload 禁止包含真实上游 `api_key`。

## 7. resolve 接口

```http
POST /api/internal/image/credential-leases/{lease_id}/resolve
Content-Type: application/json
X-ImageHandle-Timestamp: 1782140000
X-ImageHandle-Signature: <signature>
X-ImageHandle-Event-Id: evt_xxx
X-ImageHandle-Secret-Id: image_handle_1
```

请求体：

```json
{
  "provider_task_id": "imgtask_xxx",
  "client_task_id": "task_xxx",
  "attempt": 1,
  "operation": "generation",
  "model": "gpt-image-2"
}
```

签名：

```text
signature = HMAC-SHA256(timestamp + "." + raw_body, internal_secret)
```

校验规则：

- timestamp 在 5 分钟窗口内。
- signature constant-time compare。
- `lease_id` 存在且未过期。
- `client_task_id` 匹配 lease 关联任务。
- 如果 new-api 已保存 `provider_task_id`，则必须匹配。
- 任务未进入终态。
- 渠道仍启用且 key 可用。
- 第一版只返回 OpenAI Images 兼容格式。

成功响应：

```json
{
  "provider": "openai_compatible",
  "request_format": "openai_images",
  "base_url": "https://api.xxx.com/v1",
  "api_key": "sk-xxx",
  "model": "gpt-image-2",
  "channel_id": "channel_123",
  "expires_at": "2026-06-24T12:30:00Z"
}
```

错误响应：

```json
{
  "error": {
    "code": "lease_expired",
    "message": "credential lease expired",
    "retryable": false
  }
}
```

常见错误码：

| code | retryable | 说明 |
| --- | --- | --- |
| `invalid_signature` | false | 签名错误 |
| `lease_not_found` | false | lease 不存在 |
| `lease_expired` | false | lease 过期或已失效 |
| `task_cancelled` | false | 任务已取消 |
| `task_already_finished` | false | 任务已终态 |
| `channel_disabled` | false | 渠道禁用 |
| `credential_unavailable` | true | 渠道 key 暂不可用 |
| `model_not_supported` | false | 非 OpenAI Images 兼容格式 |

## 8. image-handle 执行要求

- `base_url` 去掉尾部 `/` 后拼接 `/images/generations` 或 `/images/edits`。
- 如果 `base_url` 已经是 `.../v1`，不要重复拼 `/v1/v1`。
- 编辑图任务中，new-api 只传图片 URL；worker 可以自行下载 URL 后转 multipart/file 调上游。
- `api_key` 只允许在 worker 内存短暂使用，不落库、不进 Redis、不进日志。
- 上游返回大 base64 时，不要 callback 给 new-api；应上传 R2 后 callback URL。

## 9. callback 协议

callback 地址：

```http
POST /api/task/callback/external-image/{task_id}
POST /api/task/callback/external-image/batch
```

签名 header：

```text
X-Callback-Timestamp
X-Callback-Signature
X-Callback-Secret-Id
X-Callback-Event-Id
```

成功事件：

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
    "total_tokens": 1234,
    "input_tokens": 100,
    "output_tokens": 1134,
    "actual_quota": 0
  },
  "raw_response": {
    "usage": {
      "total_tokens": 1234
    }
  },
  "raw_response_truncated": false,
  "raw_response_omitted_fields": ["data.0.b64_json"]
}
```

失败事件：

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

raw response 约束：

- `raw_response` 是剔除大字段后的安全 JSON。
- 递归删除或替换 `b64_json`、`base64`、`image_base64`、`data:image/...` 等大字段。
- 建议 image-handle 限制 `RAW_RESPONSE_MAX_BYTES=64KB/256KB`。
- new-api 默认按 256KB 上限处理，超过会截断并标记 `raw_response_truncated=true`。

## 10. 计费和幂等

- 提交任务时 new-api 预扣费。
- callback 成功时，只有 CAS 首次进入 `SUCCESS` 的进程会结算和写 assets。
- callback 失败时，只有 CAS 首次进入 `FAILURE` 的进程会退款。
- token 计价模型使用 callback `usage` 和 new-api 价格规则重算差额。
- 按次或固定价模型保持现有按次逻辑。
- 重复 callback 会返回 `ignored_terminal`，不会重复结算或退款。

## 11. 用户侧调用示例

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

最终用户不需要传 `client_task_id`；不传时 new-api 自动生成。`IMAGE_HANDLE_API_KEY` 是 new-api 调 image-handle 的服务 key，不是用户的 `sk-xxx`。
