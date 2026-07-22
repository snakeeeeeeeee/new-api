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
  createVideoTask: '创建异步视频任务',
  listVideoTasks: '查询异步视频任务列表',
  getVideoTask: '查询单个异步视频任务',
  queryVideoTasks: '批量查询异步视频任务',
  downloadVideoAsset: '下载视频资源',
  uploadImageInputs: '预上传图片',
  uploadBase64ImageInputs: '预上传 Base64 图片',
};

const ASYNC_CREATE_REQUEST = `curl "$BASE_URL/v1/image/tasks" \\
  -X POST \\
  -H "Authorization: Bearer $MODEL_API_KEY" \\
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
  -H "Authorization: Bearer $MODEL_API_KEY" \\
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

const ASYNC_MULTIPART_EDIT_REQUEST = `curl "$BASE_URL/v1/image/tasks" \\
  -X POST \\
  -H "Authorization: Bearer $MODEL_API_KEY" \\
  -H "Idempotency-Key: edit-order-123" \\
  -F "model=gpt-image-2" \\
  -F "prompt=Combine both products into one studio photo" \\
  -F "image=@input-1.png" \\
  -F "image=@input-2.jpg" \\
  -F "mask=@mask.png" \\
  -F "n=1" \\
  -F "size=1024x1024" \\
  -F "quality=high" \\
  -F "output_format=png"`;

const ASYNC_EDIT_RESPONSE = `HTTP/1.1 202 Accepted
Location: /v1/image/tasks/task_edit_xxx
Retry-After: 2

{
  "id": "task_edit_xxx",
  "object": "image.task",
  "model": "gpt-image-2",
  "operation": "edit",
  "status": "queued",
  "progress": 0,
  "result": null,
  "usage": {},
  "error": null,
  "metadata": {},
  "created_at": 1784250000,
  "started_at": null,
  "completed_at": null,
  "updated_at": 1784250000
}`;

const VIDEO_GENERATION_REQUEST = `curl "$BASE_URL/v1/video/tasks" \\
  -X POST \\
  -H "Authorization: Bearer $MODEL_API_KEY" \\
  -H "Content-Type: application/json" \\
  -H "Idempotency-Key: order-123-video" \\
  -d '{
    "model": "grok-imagine-video-1.5",
    "operation": "generation",
    "input": {"prompt": "A paper boat crossing a rain puddle"},
    "output": {"duration": 5, "aspect_ratio": "16:9", "resolution": "720p"},
    "client_reference_id": "order_video_123",
    "metadata": {"campaign": "spring"}
  }'`;

const VIDEO_IMAGE_GENERATION_REQUEST = `curl "$BASE_URL/v1/video/tasks" \\
  -X POST \\
  -H "Authorization: Bearer $MODEL_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "grok-imagine-video-1.5",
    "operation": "generation",
    "input": {
      "prompt": "Animate the camera moving toward the product",
      "image": {"url": "https://cdn.example.com/product.png"}
    },
    "output": {"duration": 5, "aspect_ratio": "9:16", "resolution": "720p"}
  }'`;

const VIDEO_REFERENCE_GENERATION_REQUEST = `curl "$BASE_URL/v1/video/tasks" \\
  -X POST \\
  -H "Authorization: Bearer $MODEL_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "grok-imagine-video",
    "operation": "generation",
    "input": {
      "prompt": "Use the references to keep the character consistent",
      "reference_images": [
        {"url": "https://cdn.example.com/reference-1.png"},
        {"provider": "xai", "file_id": "file_reference_2"}
      ]
    },
    "output": {"duration": 5, "resolution": "720p"}
  }'`;

const VIDEO_EDIT_REQUEST = `curl "$BASE_URL/v1/video/tasks" \\
  -X POST \\
  -H "Authorization: Bearer $MODEL_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "grok-imagine-video-1.5",
    "operation": "edit",
    "input": {
      "prompt": "Add rain and a moody atmosphere",
      "video": {"url": "https://cdn.example.com/source.mp4"}
    },
    "provider_options": {"xai": {}}
  }'`;

const VIDEO_EXTENSION_REQUEST = `curl "$BASE_URL/v1/video/tasks" \\
  -X POST \\
  -H "Authorization: Bearer $MODEL_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "grok-imagine-video-1.5",
    "operation": "extension",
    "input": {
      "prompt": "Continue as the camera rises above the city",
      "video": {"url": "https://cdn.example.com/source.mp4"}
    },
    "output": {"duration": 5}
  }'`;

const VIDEO_CREATE_RESPONSE = `HTTP/1.1 202 Accepted
Location: /v1/video/tasks/task_video_xxx
Retry-After: 2

{
  "id": "task_video_xxx",
  "object": "video.task",
  "model": "grok-imagine-video-1.5",
  "operation": "generation",
  "status": "queued",
  "progress": 0,
  "result": null,
  "error": null,
  "client_reference_id": "order_video_123",
  "metadata": {"campaign": "spring"},
  "created_at": 1784250000,
  "started_at": null,
  "completed_at": null,
  "updated_at": 1784250000
}`;

const VIDEO_TASK_QUERY_REQUEST = `curl "$BASE_URL/v1/video/tasks/task_video_xxx" \\
  -H "Authorization: Bearer $RESOURCE_API_KEY"`;

const VIDEO_TASK_QUERY_RESPONSE = `{
  "id": "task_video_xxx",
  "object": "video.task",
  "model": "grok-imagine-video-1.5",
  "operation": "generation",
  "status": "succeeded",
  "progress": 100,
  "result": {
    "videos": [
      {
        "asset_id": "asset_video_xxx",
        "index": 0,
        "url": "/v1/assets/asset_video_xxx/content",
        "mime_type": "video/mp4",
        "duration_ms": 5000,
        "temporary": true,
        "url_auth": "resource_api_key"
      }
    ]
  },
  "error": null,
  "client_reference_id": "order_video_123",
  "metadata": {"campaign": "spring"},
  "created_at": 1784250000,
  "started_at": 1784250002,
  "completed_at": 1784250060,
  "updated_at": 1784250060
}`;

const VIDEO_TASK_LIST_REQUEST = `curl "$BASE_URL/v1/video/tasks?status=succeeded&operation=generation&limit=20" \\
  -H "Authorization: Bearer $RESOURCE_API_KEY"`;

const VIDEO_TASK_BATCH_REQUEST = `curl "$BASE_URL/v1/video/tasks/query" \\
  -X POST \\
  -H "Authorization: Bearer $RESOURCE_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"task_ids": ["task_video_xxx", "task_not_found"]}'`;

const VIDEO_ASSET_LIST_REQUEST = `curl "$BASE_URL/v1/assets?asset_type=video&page=1&page_size=20" \\
  -H "Authorization: Bearer $RESOURCE_API_KEY"`;

const VIDEO_ASSET_DOWNLOAD_REQUEST = `curl "$BASE_URL/v1/assets/asset_video_xxx/content" \\
  -H "Authorization: Bearer $RESOURCE_API_KEY" \\
  -H "Range: bytes=0-1048575" \\
  --output video.part`;

const IMAGE_UPLOAD_REQUEST = `curl "$BASE_URL/v1/image/uploads" \\
  -X POST \\
  -H "Authorization: Bearer $RESOURCE_API_KEY" \\
  -F "image=@input-1.png" \\
  -F "image=@input-2.jpg" \\
  -F "mask=@mask.png"`;

const IMAGE_UPLOAD_RESPONSE = `{
  "object": "image.upload.list",
  "data": [
    {
      "id": "upload_xxx",
      "object": "image.upload",
      "field": "image",
      "url": "https://cdn.example.com/tmp/input-1.png",
      "mime_type": "image/png",
      "size_bytes": 245760,
      "width": 1024,
      "height": 1024,
      "format": "png",
      "temporary": true
    },
    {
      "id": "upload_yyy",
      "object": "image.upload",
      "field": "image",
      "url": "https://cdn.example.com/tmp/input-2.jpg",
      "mime_type": "image/jpeg",
      "size_bytes": 198400,
      "width": 1024,
      "height": 1024,
      "format": "jpeg",
      "temporary": true
    },
    {
      "id": "upload_mask_xxx",
      "object": "image.upload",
      "field": "mask",
      "url": "https://cdn.example.com/tmp/mask.png",
      "mime_type": "image/png",
      "temporary": true
    }
  ],
  "images": [
    "https://cdn.example.com/tmp/input-1.png",
    "https://cdn.example.com/tmp/input-2.jpg"
  ],
  "mask": "https://cdn.example.com/tmp/mask.png"
}`;

const IMAGE_BASE64_UPLOAD_REQUEST = `curl "$BASE_URL/v1/image/uploads/base64" \\
  -X POST \\
  -H "Authorization: Bearer $RESOURCE_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "images": [
      "data:image/png;base64,iVBORw0KGgoAAA...",
      {"base64": "/9j/4AAQSkZJRgABAQ..."}
    ],
    "mask": {"b64_json": "iVBORw0KGgoAAA..."}
  }'`;

const TASK_QUERY_REQUEST = `curl "$BASE_URL/v1/image/tasks/task_xxx" \\
  -H "Authorization: Bearer $RESOURCE_API_KEY"`;

const TASK_QUERY_RESPONSE = `{
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
}`;

const TASK_LIST_REQUEST = `curl "$BASE_URL/v1/image/tasks?status=succeeded&operation=generation&limit=20" \\
  -H "Authorization: Bearer $RESOURCE_API_KEY"`;

const TASK_LIST_RESPONSE = `{
  "object": "list",
  "data": [
    {
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
  ],
  "first_id": "task_xxx",
  "last_id": "task_xxx",
  "has_more": false
}`;

const TASK_BATCH_QUERY_REQUEST = `curl "$BASE_URL/v1/image/tasks/query" \\
  -X POST \\
  -H "Authorization: Bearer $RESOURCE_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "task_ids": ["task_xxx", "task_not_found"]
  }'`;

const TASK_BATCH_QUERY_RESPONSE = `{
  "object": "list",
  "data": [
    {
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
            "url": "https://cdn.example.com/image.png"
          }
        ]
      },
      "usage": {},
      "error": null,
      "metadata": {},
      "created_at": 1784250000,
      "started_at": 1784250002,
      "completed_at": 1784250060,
      "updated_at": 1784250060
    }
  ],
  "missing": ["task_not_found"]
}`;

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

const ASSET_GET_REQUEST = `curl "$BASE_URL/v1/assets/asset_xxx" \\
  -H "Authorization: Bearer $RESOURCE_API_KEY"`;

const ASSET_GET_RESPONSE = `{
  "object": "asset",
  "id": "asset_xxx",
  "task_id": "task_xxx",
  "index": 0,
  "type": "image",
  "url": "https://cdn.example.com/image.png",
  "mime_type": "image/png",
  "filename": "image.png",
  "size_bytes": 245760,
  "width": 1024,
  "height": 1024,
  "model": "gpt-image-2",
  "platform": "image_handle",
  "action": "image_generation",
  "status": "available",
  "metadata": {},
  "created_at": 1784250060,
  "updated_at": 1784250060
}`;

const ASSET_QUERY_REQUEST = `curl "$BASE_URL/v1/assets/query" \\
  -X POST \\
  -H "Authorization: Bearer $RESOURCE_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "asset_ids": ["asset_xxx", "asset_yyy"],
    "asset_type": "image",
    "page": 1,
    "page_size": 100
  }'`;

const ASSET_URLS_REQUEST = `curl "$BASE_URL/v1/assets/batch/urls" \\
  -X POST \\
  -H "Authorization: Bearer $RESOURCE_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "asset_ids": ["asset_xxx", "asset_yyy"]
  }'`;

const ASSET_URLS_RESPONSE = `{
  "object": "list",
  "data": [
    {
      "asset_id": "asset_xxx",
      "task_id": "task_xxx",
      "asset_type": "image",
      "url": "https://cdn.example.com/image-1.png"
    },
    {
      "asset_id": "asset_yyy",
      "task_id": "task_yyy",
      "asset_type": "image",
      "url": "https://cdn.example.com/image-2.png"
    }
  ]
}`;

const ASSET_EXPORT_REQUEST = `curl "$BASE_URL/v1/assets/export?asset_type=image&start_timestamp=1784160000" \\
  -H "Authorization: Bearer $RESOURCE_API_KEY" \\
  --output assets.csv`;

const ASSET_EXPORT_RESPONSE = `HTTP/1.1 200 OK
Content-Type: text/csv; charset=utf-8
Content-Disposition: attachment; filename=assets.csv

asset_id,task_id,asset_type,url,filename,model,created_at
asset_xxx,task_xxx,image,https://cdn.example.com/image.png,image.png,gpt-image-2,1784250060`;

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

const VIDEO_WEBHOOK_SUCCEEDED_PAYLOAD = `{
  "id": "evt_video_xxx",
  "object": "event",
  "api_version": "2026-07-17",
  "type": "video.task.succeeded",
  "created_at": 1784250060,
  "data": {
    "object": ${VIDEO_TASK_QUERY_RESPONSE}
  }
}`;

const VIDEO_WEBHOOK_FAILED_PAYLOAD = `{
  "id": "evt_video_yyy",
  "object": "event",
  "api_version": "2026-07-17",
  "type": "video.task.failed",
  "created_at": 1784250060,
  "data": {
    "object": {
      "id": "task_video_yyy",
      "object": "video.task",
      "model": "video-model",
      "operation": "extension",
      "status": "failed",
      "progress": 100,
      "result": null,
      "error": {
        "code": "video_task_failed",
        "message": "Video task failed",
        "retryable": false
      },
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

function RequestResponseExamples({ request, response, responseTitle }) {
  const { t } = useTranslation();

  return (
    <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
      <CodeExample title={t('请求示例')}>{request}</CodeExample>
      <CodeExample title={responseTitle || t('响应示例')}>
        {response}
      </CodeExample>
    </div>
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
          typeof apiKey === 'function' ? apiKey(operation) : apiKey,
        ]),
  );

  return (
    <DocsTable
      columns={[t('方法'), t('路径'), t('用途'), t('API Key')]}
      rows={rows}
    />
  );
}

function OverviewDocs({ onOpenApiKeys, onOpenWebhook, t }) {
  return (
    <div>
      <DocumentationSection
        title={t('三类凭据，各自负责一件事')}
        description={t(
          '异步图片/视频任务创建、资源访问和 Webhook 回调验证使用相互独立的凭据。',
        )}
      >
        <Space wrap className='mb-3'>
          <Button
            type='primary'
            icon={<ExternalLink size={16} />}
            onClick={onOpenApiKeys}
          >
            {t('生成 API Key')}
          </Button>
          <Button icon={<ExternalLink size={16} />} onClick={onOpenWebhook}>
            {t('配置 Webhook')}
          </Button>
        </Space>
        <DocsTable
          columns={[t('使用场景'), t('API Key'), t('Authorization')]}
          rows={[
            [
              t('创建异步图片或视频任务'),
              t('普通 API Token（sk-...）'),
              'Bearer sk-...',
            ],
            [
              t(
                '查询异步图片/视频任务、预上传图片，以及查询、下载和导出生成资源',
              ),
              t('资源 API Key（ak_...）'),
              'Bearer ak_...',
            ],
            [
              t('验证 new-api 发出的 Webhook 回调'),
              t('Webhook Key（wk-...）'),
              'Bearer wk-...',
            ],
          ]}
        />
        <Text type='tertiary'>
          {t(
            '创建任务使用的 Token 会决定模型权限、分组和额度；wk- Key 仅用于接收端验证回调来源。',
          )}
        </Text>
        <Text type='tertiary'>
          {t(
            '现有 /v1/videos 兼容接口保持不变；新集成建议使用规范化 /v1/video/tasks。',
          )}
        </Text>
      </DocumentationSection>

      <DocumentationSection
        title={t('异步任务调用流程')}
        description={t(
          '创建使用普通 API Token；轮询、资源查询和代理下载使用资源 API Key。',
        )}
      >
        <DocsTable
          columns={[t('步骤'), t('接口'), t('说明')]}
          rows={[
            [
              '1',
              '/v1/image/uploads',
              t('可选：预上传图片编辑所需的图片或遮罩'),
            ],
            [
              '2',
              '/v1/image/tasks 或 /v1/video/tasks',
              t('创建任务并立即获得 task ID'),
            ],
            [
              '3',
              '/v1/{image|video}/tasks/{task_id}',
              t('轮询任务；也可以配置 Webhook 等待主动通知'),
            ],
            [
              '4',
              '/v1/assets/{asset_id}',
              t('查询结果；视频代理地址使用 /content 下载'),
            ],
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
        description={t(
          '创建任务使用普通 API Token（sk-...）；查询和预上传使用资源 API Key（ak_...）。',
        )}
      >
        <EndpointTable
          tags={['Async Images', 'Image Uploads']}
          apiKey={(operation) =>
            operation.operationId === 'createImageTask'
              ? t('普通 API Token')
              : t('资源 API Key')
          }
          t={t}
        />
      </DocumentationSection>

      <DocumentationSection
        title={`${t('创建生成任务')} · POST /v1/image/tasks`}
        description={t(
          '成功创建后返回 202；使用 Location 查询任务，或等待 Webhook 通知。',
        )}
      >
        <RequestResponseExamples
          request={ASYNC_CREATE_REQUEST}
          response={ASYNC_CREATE_RESPONSE}
          responseTitle={t('202 响应')}
        />
        <Text type='tertiary'>
          {t(
            '建议为可重试的创建请求设置 Idempotency-Key；同一个 Key 只能对应同一份请求。',
          )}
        </Text>
      </DocumentationSection>

      <DocumentationSection
        title={`${t('创建编辑任务')} · POST /v1/image/tasks`}
        description={t(
          '编辑任务必须提供 prompt 和至少一张图片；可直接上传本地文件，也可使用 URL JSON，mask 可选。',
        )}
      >
        <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
          <CodeExample title={t('JSON 图片 URL')}>
            {ASYNC_EDIT_REQUEST}
          </CodeExample>
          <CodeExample title={t('multipart 本地文件')}>
            {ASYNC_MULTIPART_EDIT_REQUEST}
          </CodeExample>
          <CodeExample title={t('202 响应')}>{ASYNC_EDIT_RESPONSE}</CodeExample>
        </div>
      </DocumentationSection>

      <DocumentationSection
        title={`${t('查询异步图片任务列表')} · GET /v1/image/tasks`}
      >
        <RequestResponseExamples
          request={TASK_LIST_REQUEST}
          response={TASK_LIST_RESPONSE}
        />
        <Text type='tertiary'>
          {t(
            '下一页把上页返回的 last_id 作为 after 参数，并保持其他筛选条件不变。',
          )}
        </Text>
      </DocumentationSection>

      <DocumentationSection
        title={`${t('查询单个异步图片任务')} · GET /v1/image/tasks/{task_id}`}
      >
        <RequestResponseExamples
          request={TASK_QUERY_REQUEST}
          response={TASK_QUERY_RESPONSE}
        />
      </DocumentationSection>

      <DocumentationSection
        title={`${t('批量查询异步图片任务')} · POST /v1/image/tasks/query`}
        description={t('最多提交 100 个 task ID，返回顺序与请求顺序一致。')}
      >
        <RequestResponseExamples
          request={TASK_BATCH_QUERY_REQUEST}
          response={TASK_BATCH_QUERY_RESPONSE}
        />
      </DocumentationSection>

      <DocumentationSection
        title={`${t('预上传图片')} · POST /v1/image/uploads`}
        description={t(
          '可重复提交 image，最多 10 张；mask 最多 1 张。临时 URL 应尽快用于编辑任务。',
        )}
      >
        <RequestResponseExamples
          request={IMAGE_UPLOAD_REQUEST}
          response={IMAGE_UPLOAD_RESPONSE}
        />
      </DocumentationSection>

      <DocumentationSection
        title={`${t('预上传 Base64 图片')} · POST /v1/image/uploads/base64`}
        description={t(
          '支持纯 Base64 和 data URL；响应中的 images 与 mask 可直接用于编辑任务。',
        )}
      >
        <RequestResponseExamples
          request={IMAGE_BASE64_UPLOAD_REQUEST}
          response={IMAGE_UPLOAD_RESPONSE}
        />
      </DocumentationSection>
    </div>
  );
}

function AsyncVideoDocs({ t }) {
  return (
    <div>
      <DocumentationSection
        title={t('接口列表')}
        description={t(
          '创建任务使用普通 API Token（sk-...）；任务查询和视频代理下载使用资源 API Key（ak_...）。',
        )}
      >
        <EndpointTable
          tags={['Async Videos']}
          apiKey={(operation) =>
            operation.operationId === 'createVideoTask'
              ? t('普通 API Token')
              : t('资源 API Key')
          }
          t={t}
        />
      </DocumentationSection>

      <DocumentationSection
        title={`${t('文本或图片生成')} · POST /v1/video/tasks`}
        description={t(
          'image 是单个主图对象，reference_images 是多图数组；具体操作允许的组合与数量由供应商 adaptor 校验。',
        )}
      >
        <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
          <CodeExample title={t('文生视频')}>
            {VIDEO_GENERATION_REQUEST}
          </CodeExample>
          <CodeExample title={t('图生视频')}>
            {VIDEO_IMAGE_GENERATION_REQUEST}
          </CodeExample>
          <CodeExample title={t('202 响应')}>
            {VIDEO_CREATE_RESPONSE}
          </CodeExample>
        </div>
        <Text type='tertiary'>
          {t(
            '建议为可重试的创建请求设置 Idempotency-Key；同一个 Key 与同一请求会返回原任务，不同请求返回 409。',
          )}
        </Text>
      </DocumentationSection>

      <DocumentationSection
        title={`${t('参考图生成')} · POST /v1/video/tasks`}
        description={t(
          '输入源支持公共 URL、供应商支持的 data URL，以及 provider + file_id 文件引用。',
        )}
      >
        <CodeExample title={t('请求示例')}>
          {VIDEO_REFERENCE_GENERATION_REQUEST}
        </CodeExample>
        <Text type='tertiary'>
          {t(
            '需要 ak_ 鉴权的 Asset URL 不保证上游能够读取；应使用公开 URL 或供应商文件引用。',
          )}
        </Text>
        <Text type='tertiary'>
          {t(
            'xAI 参考图生成最多支持 7 张图片，当前应使用 grok-imagine-video；grok-imagine-video-1.5 不支持 reference_images。',
          )}
        </Text>
      </DocumentationSection>

      <DocumentationSection
        title={t('编辑、扩展与 Remix')}
        description={t(
          '三种操作都必须提供 input.video；公共协议允许附加 image 或 reference_images，实际支持情况由供应商 adaptor 决定，当前 xAI 编辑与扩展不接受图片输入。',
        )}
      >
        <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
          <CodeExample title={t('视频编辑')}>{VIDEO_EDIT_REQUEST}</CodeExample>
          <CodeExample title={t('视频扩展')}>
            {VIDEO_EXTENSION_REQUEST}
          </CodeExample>
        </div>
        <DocsTable
          columns={[t('操作'), t('关键语义')]}
          rows={[
            [
              'edit',
              t('继承输入视频的时长、宽高比和分辨率，不能在 output 中覆盖'),
            ],
            [
              'extension',
              t('output.duration 表示新增片段长度，不是最终视频总时长'),
            ],
            [
              'remix',
              t('仅在所选模型与供应商声明支持时可用，否则返回能力错误'),
            ],
          ]}
        />
        <Text type='tertiary'>
          {t(
            '供应商专属参数只能放在 provider_options 的对应命名空间中，例如 provider_options.xai。',
          )}
        </Text>
      </DocumentationSection>

      <DocumentationSection
        title={t('查询视频任务')}
        description={t(
          '列表使用 last_id 作为下一页 after 游标；批量查询最多 100 个 ID，并保持请求顺序。',
        )}
      >
        <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
          <CodeExample title={t('查询单个任务')}>
            {VIDEO_TASK_QUERY_REQUEST}
          </CodeExample>
          <CodeExample title={t('任务结果')}>
            {VIDEO_TASK_QUERY_RESPONSE}
          </CodeExample>
          <CodeExample title={t('查询任务列表')}>
            {VIDEO_TASK_LIST_REQUEST}
          </CodeExample>
          <CodeExample title={t('批量查询任务')}>
            {VIDEO_TASK_BATCH_REQUEST}
          </CodeExample>
        </div>
      </DocumentationSection>

      <DocumentationSection
        title={t('查询与下载视频资源')}
        description={t(
          '先检查 url_auth：none 可直接访问；resource_api_key 必须携带 ak_ 访问返回的 Asset /content 地址。',
        )}
      >
        <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
          <CodeExample title={t('查询视频资源')}>
            {VIDEO_ASSET_LIST_REQUEST}
          </CodeExample>
          <CodeExample title={t('Range 下载')}>
            {VIDEO_ASSET_DOWNLOAD_REQUEST}
          </CodeExample>
        </div>
        <Text type='tertiary'>
          {t(
            '视频只依赖上游临时资源，不会归档到对象存储；上游过期后代理下载返回 410。',
          )}
        </Text>
      </DocumentationSection>
    </div>
  );
}

function AssetApiDocs({ t }) {
  return (
    <div>
      <DocumentationSection
        title={t('接口列表')}
        description={t('资源 API 使用同一个资源 API Key（ak_...）。')}
      >
        <EndpointTable tags={['Assets']} apiKey={t('资源 API Key')} t={t} />
      </DocumentationSection>

      <DocumentationSection
        title={`${t('查询资源列表')} · GET /v1/assets`}
        description={t(
          '资源接口严格按 API Key 所属用户隔离，只返回当前用户可见的数据。',
        )}
      >
        <RequestResponseExamples
          request={ASSET_LIST_REQUEST}
          response={ASSET_LIST_RESPONSE}
        />
      </DocumentationSection>

      <DocumentationSection
        title={`${t('查询单个资源')} · GET /v1/assets/{asset_id}`}
      >
        <RequestResponseExamples
          request={ASSET_GET_REQUEST}
          response={ASSET_GET_RESPONSE}
        />
      </DocumentationSection>

      <DocumentationSection
        title={`${t('下载视频资源')} · GET /v1/assets/{asset_id}/content`}
        description={t(
          '仅用于 url_auth=resource_api_key 的视频；支持 Range，请求上游已过期时返回 410。',
        )}
      >
        <CodeExample title={t('Range 下载')}>
          {VIDEO_ASSET_DOWNLOAD_REQUEST}
        </CodeExample>
      </DocumentationSection>

      <DocumentationSection
        title={`${t('批量查询资源')} · POST /v1/assets/query`}
      >
        <RequestResponseExamples
          request={ASSET_QUERY_REQUEST}
          response={ASSET_LIST_RESPONSE}
        />
      </DocumentationSection>

      <DocumentationSection
        title={`${t('批量获取资源 URL')} · POST /v1/assets/batch/urls`}
      >
        <RequestResponseExamples
          request={ASSET_URLS_REQUEST}
          response={ASSET_URLS_RESPONSE}
        />
      </DocumentationSection>

      <DocumentationSection
        title={`${t('导出资源 CSV')} · GET /v1/assets/export`}
        description={t('最多导出 10000 条符合筛选条件的资源。')}
      >
        <RequestResponseExamples
          request={ASSET_EXPORT_REQUEST}
          response={ASSET_EXPORT_RESPONSE}
        />
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
            {t(
              'Webhook 配置页会生成独立的 wk- Key；接收端只需校验 Authorization 请求头。',
            )}
          </Text>
        </div>
        <CodeExample title={t('回调请求头')}>{WEBHOOK_HEADERS}</CodeExample>
      </DocumentationSection>

      <DocumentationSection
        title={t('回调 Payload')}
        description={t(
          'data.object 与对应的图片或视频任务查询 DTO 完全一致，event id 可用于幂等去重和排查。',
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
          <Collapse.Panel
            header={t('video.task.succeeded')}
            itemKey='video-succeeded'
          >
            <Text type='tertiary' className='mb-3 block'>
              {t(
                '视频 URL 是上游临时资源；根据 url_auth 决定直接访问或携带 ak_ 下载。',
              )}
            </Text>
            <CodeExample title={t('视频成功事件')}>
              {VIDEO_WEBHOOK_SUCCEEDED_PAYLOAD}
            </CodeExample>
          </Collapse.Panel>
          <Collapse.Panel
            header={t('video.task.failed')}
            itemKey='video-failed'
          >
            <CodeExample title={t('视频失败事件')}>
              {VIDEO_WEBHOOK_FAILED_PAYLOAD}
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
        title={t('投递与重试')}
        description={t(
          'HTTP 2xx 即投递成功；网络错误或非 2xx 默认最多尝试 3 次，次数和固定间隔由管理员在异步任务管理中配置。',
        )}
      >
        <DocsTable
          columns={[t('行为'), t('处理方式')]}
          rows={[
            [t('成功'), t('接收端返回任意 2xx 即停止投递，响应体内容会被忽略')],
            [
              t('发送失败'),
              t('网络错误、超时或非 2xx 按固定间隔重试，达到最大次数后结束'),
            ],
            [
              t('安全'),
              t('校验 Authorization: Bearer wk-...，不要记录完整 Key'),
            ],
          ]}
        />
      </DocumentationSection>
    </div>
  );
}

export default function ResourceCenterDocs({ onOpenApiKeys, onOpenWebhook }) {
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
          { tab: t('异步视频'), itemKey: 'async-videos' },
          { tab: t('资源 API'), itemKey: 'assets' },
          { tab: 'Webhook', itemKey: 'webhook' },
        ]}
      />

      {activeSection === 'overview' && (
        <OverviewDocs
          onOpenApiKeys={onOpenApiKeys}
          onOpenWebhook={onOpenWebhook}
          t={t}
        />
      )}
      {activeSection === 'async-images' && <AsyncImageDocs t={t} />}
      {activeSection === 'async-videos' && <AsyncVideoDocs t={t} />}
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
