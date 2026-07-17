/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useMemo, useState } from 'react';
import {
  Button,
  Collapse,
  SideSheet,
  Space,
  Tabs,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { Copy as CopyIcon, Download, ExternalLink, Eye } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { copy, downloadTextAsFile, showSuccess } from '../../../helpers';
import RESOURCE_CENTER_OPENAPI_SPEC from '../../../../../docs/openapi/resource-center.json';

const { Text, Title } = Typography;
const OPENAPI_HTTP_METHODS = new Set(['get', 'post', 'put', 'patch', 'delete']);
const OPENAPI_OPERATION_COUNT = Object.values(
  RESOURCE_CENTER_OPENAPI_SPEC.paths,
).reduce(
  (total, pathItem) =>
    total +
    Object.keys(pathItem).filter((method) => OPENAPI_HTTP_METHODS.has(method))
      .length,
  0,
);
const OPERATION_TITLES = {
  listAssets: '查询资源列表',
  getAsset: '查询单个资源',
  queryAssets: '批量查询资源',
  getAssetURLs: '批量获取资源 URL',
  exportAssets: '导出资源 CSV',
  createImageTask: '创建异步图片任务',
  listImageTasks: '查询异步图片任务列表',
  getImageTask: '查询单个异步图片任务',
  queryImageTasks: '批量查询异步图片任务',
  uploadImageInputs: '预上传图片',
  uploadBase64ImageInputs: '预上传 Base64 图片',
};

const ASYNC_CREATE_REQUEST = `curl "$BASE_URL/v1/image/tasks" \\
  -X POST \\
  -H "Authorization: Bearer $API_KEY" \\
  -H "Content-Type: application/json" \\
  -H "Idempotency-Key: order-123-image" \\
  -d '{
    "model": "gpt-image-2",
    "operation": "generation",
    "input": {"prompt": "A product photo on a white background"},
    "output": {"count": 1, "size": "1024x1024", "format": "png"},
    "client_reference_id": "order_123",
    "metadata": {}
  }'`;

const ASYNC_CREATE_RESPONSE = `HTTP/1.1 202 Accepted
Location: /v1/image/tasks/task_xxx
Retry-After: 2

{
  "id": "task_xxx",
  "object": "image.task",
  "model": "gpt-image-2",
  "operation": "generation",
  "status": "queued",
  "progress": 0,
  "result": null,
  "usage": {},
  "error": null,
  "client_reference_id": "order_123",
  "metadata": {},
  "created_at": 1784250000,
  "started_at": null,
  "completed_at": null,
  "updated_at": 1784250000
}`;

const ASYNC_EDIT_REQUEST = `curl "$BASE_URL/v1/image/tasks" \\
  -X POST \\
  -H "Authorization: Bearer $API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "gpt-image-2",
    "operation": "edit",
    "input": {
      "prompt": "Replace the background with a studio wall",
      "images": [{"url": "https://cdn.example.com/input.png"}],
      "mask": {"url": "https://cdn.example.com/mask.png"}
    },
    "output": {"size": "1024x1024", "format": "png"}
  }'`;

const IMAGE_UPLOAD_REQUEST = `curl "$BASE_URL/v1/image/uploads" \\
  -X POST \\
  -H "Authorization: Bearer $API_KEY" \\
  -F "image=@input.png" \\
  -F "mask=@mask.png"`;

const IMAGE_UPLOAD_RESPONSE = `{
  "object": "image.upload.list",
  "data": [
    {
      "id": "upload_xxx",
      "object": "image.upload",
      "field": "image",
      "url": "https://cdn.example.com/tmp/input.png",
      "mime_type": "image/png",
      "temporary": true
    }
  ],
  "images": ["https://cdn.example.com/tmp/input.png"],
  "mask": "https://cdn.example.com/tmp/mask.png"
}`;

const TASK_QUERY_REQUEST = `curl "$BASE_URL/v1/image/tasks/task_xxx" \\
  -H "Authorization: Bearer $API_KEY"`;

const ASSET_LIST_REQUEST = `curl "$BASE_URL/v1/assets?asset_type=image&page=1&page_size=20" \\
  -H "Authorization: Bearer $RESOURCE_API_KEY"`;

const ASSET_LIST_RESPONSE = `{
  "object": "list",
  "data": [
    {
      "object": "asset",
      "id": "asset_xxx",
      "task_id": "task_xxx",
      "index": 0,
      "type": "image",
      "url": "https://cdn.example.com/image.png",
      "mime_type": "image/png",
      "filename": "image.png",
      "width": 1024,
      "height": 1024,
      "model": "gpt-image-2",
      "status": "available",
      "created_at": 1784250060,
      "updated_at": 1784250060
    }
  ],
  "page": 1,
  "page_size": 20,
  "total": 1,
  "has_more": false
}`;

const WEBHOOK_HEADERS = `POST https://your-service.example.com/webhooks/new-api HTTP/1.1
Authorization: Bearer wk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
Content-Type: application/json`;

const WEBHOOK_SUCCEEDED_PAYLOAD = `{
  "id": "evt_xxx",
  "object": "event",
  "api_version": "2026-07-17",
  "type": "image.task.succeeded",
  "created_at": 1784250060,
  "data": {
    "object": {
      "id": "task_xxx",
      "object": "image.task",
      "model": "gpt-image-2",
      "operation": "generation",
      "status": "succeeded",
      "progress": 100,
      "result": {
        "images": [
          {
            "asset_id": "asset_xxx",
            "url": "https://cdn.example.com/image.png",
            "mime_type": "image/png",
            "format": "png",
            "width": 1024,
            "height": 1024,
            "size_bytes": 245760,
            "filename": "image.png"
          }
        ]
      },
      "usage": {},
      "error": null,
      "client_reference_id": "order_123",
      "metadata": {},
      "created_at": 1784250000,
      "started_at": 1784250002,
      "completed_at": 1784250060,
      "updated_at": 1784250060
    }
  }
}`;

const WEBHOOK_FAILED_PAYLOAD = `{
  "id": "evt_yyy",
  "object": "event",
  "api_version": "2026-07-17",
  "type": "image.task.failed",
  "created_at": 1784250060,
  "data": {
    "object": {
      "id": "task_yyy",
      "object": "image.task",
      "model": "gpt-image-2",
      "operation": "edit",
      "status": "failed",
      "progress": 100,
      "result": null,
      "usage": {},
      "error": {
        "code": "upstream_error",
        "message": "Image generation failed",
        "retryable": false
      },
      "client_reference_id": "order_456",
      "metadata": {},
      "created_at": 1784250000,
      "started_at": 1784250002,
      "completed_at": 1784250060,
      "updated_at": 1784250060
    }
  }
}`;

const WEBHOOK_TEST_PAYLOAD = `{
  "id": "evt_test_xxx",
  "object": "event",
  "api_version": "2026-07-17",
  "type": "webhook.test",
  "created_at": 1784250000,
  "data": {
    "object": {
      "object": "webhook.test",
      "created_at": 1784250000
    }
  }
}`;

function MethodTag({ method }) {
  const colorMap = { GET: 'green', POST: 'blue', PUT: 'orange', DELETE: 'red' };
  return <Tag color={colorMap[method] || 'grey'}>{method}</Tag>;
}

function DocsTable({ columns, rows }) {
  return (
    <div className='max-w-full overflow-x-auto border-y border-solid border-semi-color-border'>
      <table className='w-full min-w-[560px] border-collapse text-sm'>
        <thead>
          <tr className='bg-semi-color-fill-0'>
            {columns.map((column) => (
              <th
                key={column}
                className='border-0 border-b border-solid border-semi-color-border px-3 py-2 text-left font-medium text-semi-color-text-1'
              >
                {column}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, rowIndex) => (
            <tr
              key={`${rowIndex}-${String(row[0])}`}
              className='border-0 border-b border-solid border-semi-color-border last:border-b-0'
            >
              {row.map((cell, cellIndex) => (
                <td
                  key={`${rowIndex}-${cellIndex}`}
                  className='px-3 py-2 align-top text-semi-color-text-0'
                >
                  {cell}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function CodeExample({ title, children }) {
  const { t } = useTranslation();

  const copyCode = async () => {
    if (await copy(children)) showSuccess(t('已复制示例'));
  };

  return (
    <div className='min-w-0 overflow-hidden rounded-md border border-solid border-semi-color-border'>
      <div className='flex min-h-11 items-center justify-between gap-3 border-0 border-b border-solid border-semi-color-border bg-semi-color-fill-0 px-3 py-2'>
        <Text strong>{title}</Text>
        <Button
          theme='borderless'
          type='tertiary'
          icon={<CopyIcon size={16} />}
          aria-label={t('复制示例')}
          onClick={copyCode}
        />
      </div>
      <pre className='m-0 max-w-full overflow-x-auto p-3 text-xs leading-5 text-semi-color-text-0'>
        <code>{children}</code>
      </pre>
    </div>
  );
}

function DocumentationSection({ title, description, children }) {
  return (
    <section className='flex flex-col gap-3 border-0 border-b border-solid border-semi-color-border py-5 first:pt-1 last:border-b-0 last:pb-0'>
      <div className='max-w-3xl'>
        <Title heading={6} className='!mb-1'>
          {title}
        </Title>
        {description && <Text type='tertiary'>{description}</Text>}
      </div>
      {children}
    </section>
  );
}

function EndpointTable({ tags, apiKey, t }) {
  const rows = Object.entries(RESOURCE_CENTER_OPENAPI_SPEC.paths).flatMap(
    ([path, pathItem]) =>
      Object.entries(pathItem)
        .filter(
          ([method, operation]) =>
            OPENAPI_HTTP_METHODS.has(method) &&
            operation.tags?.some((tag) => tags.includes(tag)),
        )
        .map(([method, operation]) => [
          <MethodTag key={`${method}-${path}`} method={method.toUpperCase()} />,
          <Text key={path} code>
            {path}
          </Text>,
          t(OPERATION_TITLES[operation.operationId] || operation.summary),
          apiKey,
        ]),
  );

  return (
    <DocsTable
      columns={[t('方法'), t('路径'), t('用途'), t('API Key')]}
      rows={rows}
    />
  );
}

function OverviewDocs({ t }) {
  return (
    <div>
      <DocumentationSection
        title={t('选择正确的 API Key')}
        description={t('三类 Key 用途不同，不能互相替换。')}
      >
        <DocsTable
          columns={[t('使用场景'), t('API Key'), t('Authorization')]}
          rows={[
            [
              t('创建和查询异步图片任务、预上传图片'),
              t('普通 API Key（sk-...）'),
              'Bearer sk-...',
            ],
            [
              t('查询和导出已生成资源'),
              t('资源 API Key（ak_...）'),
              'Bearer ak_...',
            ],
            [
              t('接收任务完成回调'),
              t('Webhook Key（wk-...）'),
              t('由 new-api 发给接收端'),
            ],
          ]}
        />
      </DocumentationSection>

      <DocumentationSection
        title={t('异步图片调用流程')}
        description={t(
          '编辑任务需要图片 URL；本地文件可先通过预上传换成临时 URL。',
        )}
      >
        <DocsTable
          columns={[t('步骤'), t('接口'), t('说明')]}
          rows={[
            ['1', '/v1/image/uploads', t('可选：预上传编辑所需的图片或遮罩')],
            ['2', '/v1/image/tasks', t('创建生成或编辑任务，立即获得 task ID')],
            [
              '3',
              '/v1/image/tasks/{task_id}',
              t('轮询任务；也可以配置 Webhook 等待主动通知'),
            ],
            ['4', '/v1/assets/{asset_id}', t('按需查询或下载任务生成的资源')],
          ]}
        />
      </DocumentationSection>

      <DocumentationSection
        title={t('通用错误格式')}
        description={t('接口使用真实 HTTP 状态码，request_id 可用于排查请求。')}
      >
        <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
          <DocsTable
            columns={[t('状态码'), t('说明')]}
            rows={[
              ['400', t('请求参数或 JSON 无效')],
              ['401', t('API Key 缺失或无效')],
              ['403', t('API Key 被禁用、过期或无权访问')],
              ['404', t('对象不存在或不属于当前用户')],
              ['409', t('幂等 Key 已用于不同请求')],
              ['429', t('请求过于频繁')],
              ['500', t('服务端错误')],
            ]}
          />
          <CodeExample title={t('错误响应')}>{`{
  "error": {
    "type": "invalid_request_error",
    "code": "invalid_request",
    "message": "model is required",
    "param": "model",
    "request_id": "req_xxx"
  }
}`}</CodeExample>
        </div>
      </DocumentationSection>
    </div>
  );
}

function AsyncImageDocs({ t }) {
  return (
    <div>
      <DocumentationSection
        title={t('接口列表')}
        description={t('这些接口统一使用普通 API Key（sk-...）。')}
      >
        <EndpointTable
          tags={['Async Images', 'Image Uploads']}
          apiKey={t('普通 API Key')}
          t={t}
        />
      </DocumentationSection>

      <DocumentationSection
        title={t('创建生成任务')}
        description={t(
          '成功创建后返回 202；使用 Location 查询任务，或等待 Webhook 通知。',
        )}
      >
        <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
          <CodeExample title={t('请求示例')}>
            {ASYNC_CREATE_REQUEST}
          </CodeExample>
          <CodeExample title={t('202 响应')}>
            {ASYNC_CREATE_RESPONSE}
          </CodeExample>
        </div>
        <Text type='tertiary'>
          {t(
            '建议为可重试的创建请求设置 Idempotency-Key；同一个 Key 只能对应同一份请求。',
          )}
        </Text>
      </DocumentationSection>

      <DocumentationSection
        title={t('创建编辑任务')}
        description={t(
          '编辑任务必须提供 prompt 和至少一个图片 URL，mask 可选。',
        )}
      >
        <CodeExample title={t('请求示例')}>{ASYNC_EDIT_REQUEST}</CodeExample>
      </DocumentationSection>

      <DocumentationSection
        title={t('预上传与查询')}
        description={t(
          '上传结果中的 images 和 mask 可直接放入编辑任务；临时 URL 应尽快使用。',
        )}
      >
        <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
          <CodeExample title={t('multipart 预上传')}>
            {IMAGE_UPLOAD_REQUEST}
          </CodeExample>
          <CodeExample title={t('上传响应')}>
            {IMAGE_UPLOAD_RESPONSE}
          </CodeExample>
        </div>
        <CodeExample title={t('查询单个任务')}>
          {TASK_QUERY_REQUEST}
        </CodeExample>
      </DocumentationSection>
    </div>
  );
}

function AssetApiDocs({ t }) {
  return (
    <div>
      <DocumentationSection
        title={t('接口列表')}
        description={t('资源 API 只接受资源中心生成的 ak_ API Key。')}
      >
        <EndpointTable tags={['Assets']} apiKey={t('资源 API Key')} t={t} />
      </DocumentationSection>

      <DocumentationSection
        title={t('查询资源')}
        description={t(
          '资源接口严格按 API Key 所属用户隔离，只返回当前用户可见的数据。',
        )}
      >
        <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
          <CodeExample title={t('请求示例')}>{ASSET_LIST_REQUEST}</CodeExample>
          <CodeExample title={t('响应示例')}>{ASSET_LIST_RESPONSE}</CodeExample>
        </div>
      </DocumentationSection>

      <DocumentationSection
        title={t('常用查询参数')}
        description={t('完整字段、枚举和响应 Schema 以 OpenAPI 文档为准。')}
      >
        <DocsTable
          columns={[t('参数'), t('类型'), t('说明')]}
          rows={[
            ['asset_type', 'string', t('资源类型：image、video、audio、file')],
            ['task_id', 'string', t('按任务 ID 精确筛选')],
            ['model', 'string', t('按模型名称精确筛选')],
            ['keyword', 'string', t('搜索资源 ID、任务 ID、文件名或 URL')],
            ['start_timestamp', 'integer', t('创建时间下限，Unix 秒')],
            ['end_timestamp', 'integer', t('创建时间上限，Unix 秒')],
            ['page', 'integer', t('页码，默认 1')],
            ['page_size', 'integer', t('每页数量，默认 20，最大 100')],
          ]}
        />
      </DocumentationSection>
    </div>
  );
}

function WebhookDocs({ onOpenWebhook, t }) {
  return (
    <div>
      <DocumentationSection
        title={t('接收方式')}
        description={t(
          '任务进入成功或失败终态后，new-api 会向账号配置的地址发送 POST 请求。',
        )}
      >
        <div className='flex flex-wrap items-center gap-2'>
          <Button
            type='primary'
            icon={<ExternalLink size={16} />}
            onClick={onOpenWebhook}
          >
            {t('打开 Webhook 配置')}
          </Button>
          <Text type='tertiary'>
            {t('在资源中心生成 wk- Key，并在接收端校验 Authorization 请求头。')}
          </Text>
        </div>
        <CodeExample title={t('回调请求头')}>{WEBHOOK_HEADERS}</CodeExample>
      </DocumentationSection>

      <DocumentationSection
        title={t('回调 Payload')}
        description={t(
          'data.object 与任务查询接口返回的对象一致；使用稳定的 event id 去重。',
        )}
      >
        <Collapse defaultActiveKey={['succeeded']}>
          <Collapse.Panel
            header={t('image.task.succeeded')}
            itemKey='succeeded'
          >
            <CodeExample title={t('成功事件')}>
              {WEBHOOK_SUCCEEDED_PAYLOAD}
            </CodeExample>
          </Collapse.Panel>
          <Collapse.Panel header={t('image.task.failed')} itemKey='failed'>
            <CodeExample title={t('失败事件')}>
              {WEBHOOK_FAILED_PAYLOAD}
            </CodeExample>
          </Collapse.Panel>
          <Collapse.Panel header={t('webhook.test')} itemKey='test'>
            <Text type='tertiary' className='mb-3 block'>
              {t('点击“发送测试”后会收到该事件，用于验证地址和 wk- Key。')}
            </Text>
            <CodeExample title={t('测试事件')}>
              {WEBHOOK_TEST_PAYLOAD}
            </CodeExample>
          </Collapse.Panel>
        </Collapse>
      </DocumentationSection>

      <DocumentationSection
        title={t('确认、重试与去重')}
        description={t('Webhook 采用至少一次投递，接收端必须支持重复事件。')}
      >
        <DocsTable
          columns={[t('行为'), t('处理方式')]}
          rows={[
            [t('确认成功'), t('在 10 秒内返回任意 2xx；建议直接返回 204')],
            [
              t('自动重试'),
              t('网络错误、429 或非 2xx 会自动重试，event id 保持不变'),
            ],
            [t('停用'), t('接收端返回 410 时自动停用配置')],
            [t('去重'), t('保存已处理的 event id，重复收到时直接返回 2xx')],
            [
              t('安全'),
              t('校验 Authorization: Bearer wk-...，不要记录完整 Key'),
            ],
          ]}
        />
        <CodeExample
          title={t('推荐响应')}
        >{`HTTP/1.1 204 No Content`}</CodeExample>
      </DocumentationSection>
    </div>
  );
}

export default function ResourceCenterDocs({ onOpenWebhook }) {
  const { t } = useTranslation();
  const [activeSection, setActiveSection] = useState('overview');
  const [showOpenAPI, setShowOpenAPI] = useState(false);
  const openAPIJSON = useMemo(
    () => JSON.stringify(RESOURCE_CENTER_OPENAPI_SPEC, null, 2),
    [],
  );

  const copyOpenAPI = async () => {
    if (await copy(openAPIJSON)) showSuccess(t('已复制 OpenAPI JSON'));
  };

  const downloadOpenAPI = () => {
    downloadTextAsFile(openAPIJSON, 'new-api-resource-center-openapi.json');
    showSuccess(t('已导出 OpenAPI JSON'));
  };

  return (
    <div className='flex max-w-6xl flex-col gap-4'>
      <div className='flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between'>
        <div className='max-w-3xl'>
          <Title heading={5} className='!mb-1'>
            {t('API 使用文档')}
          </Title>
          <Text type='tertiary'>
            {t('按调用场景查看 API Key、请求示例、响应和 Webhook 回调。')}
          </Text>
        </div>
        <Space wrap>
          <Tag color='blue'>{t('OpenAPI 3.1')}</Tag>
          <Tag>
            {OPENAPI_OPERATION_COUNT} {t('个接口')}
          </Tag>
          <Button icon={<Eye size={16} />} onClick={() => setShowOpenAPI(true)}>
            {t('查看 OpenAPI')}
          </Button>
          <Button icon={<Download size={16} />} onClick={downloadOpenAPI}>
            {t('下载')}
          </Button>
        </Space>
      </div>

      <Tabs
        type='button'
        activeKey={activeSection}
        onChange={setActiveSection}
        tabList={[
          { tab: t('API 概览'), itemKey: 'overview' },
          { tab: t('异步图片'), itemKey: 'async-images' },
          { tab: t('资源 API'), itemKey: 'assets' },
          { tab: 'Webhook', itemKey: 'webhook' },
        ]}
      />

      {activeSection === 'overview' && <OverviewDocs t={t} />}
      {activeSection === 'async-images' && <AsyncImageDocs t={t} />}
      {activeSection === 'assets' && <AssetApiDocs t={t} />}
      {activeSection === 'webhook' && (
        <WebhookDocs onOpenWebhook={onOpenWebhook} t={t} />
      )}

      <SideSheet
        placement='right'
        title={t('OpenAPI JSON')}
        visible={showOpenAPI}
        onCancel={() => setShowOpenAPI(false)}
        width='min(760px, 100vw)'
        footer={
          <Space wrap>
            <Button onClick={() => setShowOpenAPI(false)}>{t('关闭')}</Button>
            <Button icon={<CopyIcon size={16} />} onClick={copyOpenAPI}>
              {t('复制')}
            </Button>
            <Button
              type='primary'
              icon={<Download size={16} />}
              onClick={downloadOpenAPI}
            >
              {t('下载')}
            </Button>
          </Space>
        }
      >
        <pre className='m-0 max-h-[calc(100vh-180px)] max-w-full overflow-auto rounded-md bg-semi-color-fill-0 p-3 text-xs leading-5 text-semi-color-text-0'>
          <code>{openAPIJSON}</code>
        </pre>
      </SideSheet>
    </div>
  );
}
