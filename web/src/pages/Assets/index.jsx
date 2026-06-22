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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Empty,
  Form,
  ImagePreview,
  Pagination,
  Popconfirm,
  Select,
  SideSheet,
  Space,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Copy,
  Download,
  ExternalLink,
  FileSpreadsheet,
  Grid3X3,
  KeyRound,
  Image as ImageIcon,
  Eye,
  EyeOff,
  List,
  Plus,
  Power,
  PowerOff,
  RefreshCcw,
  Trash2,
  Video,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  API,
  copy,
  downloadTextAsFile,
  isAdmin,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { DATE_RANGE_PRESETS } from '../../constants/console.constants';

const { Text, Title } = Typography;
const DIRECT_DOWNLOAD_LIMIT = 20;
const ASSET_KEY_ENABLED = 1;
const ASSET_KEY_DISABLED = 2;

const assetTypeOptions = [
  { value: '', label: '全部' },
  { value: 'image', label: '图片' },
  { value: 'video', label: '视频' },
  { value: 'audio', label: '音频' },
];

const statusOptions = [
  { value: '', label: '全部状态' },
  { value: 'available', label: '可用' },
  { value: 'blocked', label: '已屏蔽' },
  { value: 'deleted', label: '已删除' },
  { value: 'unavailable', label: '不可用' },
];

function assetTypeLabel(type, t) {
  switch (type) {
    case 'image':
      return t('图片');
    case 'video':
      return t('视频');
    case 'audio':
      return t('音频');
    default:
      return t('文件');
  }
}

function assetTypeIcon(type) {
  if (type === 'video') return <Video size={14} />;
  return <ImageIcon size={14} />;
}

function statusTag(status, t) {
  switch (status) {
    case 'available':
      return <Tag color='green'>{t('可用')}</Tag>;
    case 'blocked':
      return <Tag color='red'>{t('已屏蔽')}</Tag>;
    case 'deleted':
      return <Tag color='grey'>{t('已删除')}</Tag>;
    case 'unavailable':
      return <Tag color='orange'>{t('不可用')}</Tag>;
    default:
      return <Tag>{status || t('未知')}</Tag>;
  }
}

function buildDownloadName(asset) {
  if (asset.filename) return asset.filename;
  const ext = asset.asset_type === 'video' ? 'mp4' : 'png';
  return `${asset.asset_id}.${ext}`;
}

function triggerDownload(url, filename) {
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.target = '_blank';
  a.rel = 'noreferrer';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
}

function maskAssetKey(key) {
  if (!key) return '';
  if (key.length <= 14) return `${key.slice(0, 4)}********`;
  return `${key.slice(0, 6)}************${key.slice(-6)}`;
}

function formatExpireTime(expiredAt, t) {
  if (!expiredAt || expiredAt === -1) return t('永不过期');
  return timestamp2string(expiredAt);
}

function CodeBlock({ children, className = '' }) {
  return (
    <pre className={`m-0 whitespace-pre-wrap break-all rounded-md bg-semi-color-fill-0 p-3 text-xs leading-5 text-semi-color-text-0 ${className}`}>
      {children}
    </pre>
  );
}

function DocsTable({ columns, rows }) {
  return (
    <div className='overflow-x-auto rounded-md border border-solid border-semi-color-border'>
      <table className='w-full border-collapse text-sm'>
        <thead>
          <tr className='bg-semi-color-fill-0'>
            {columns.map((column) => (
              <th key={column} className='px-3 py-2 text-left font-medium text-semi-color-text-1 border-0 border-b border-solid border-semi-color-border'>
                {column}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr key={`${row[0]}-${index}`} className='border-0 border-b border-solid border-semi-color-border last:border-b-0'>
              {row.map((cell, cellIndex) => (
                <td key={`${row[0]}-${cellIndex}`} className='px-3 py-2 align-top text-semi-color-text-0'>
                  {cellIndex === 0 ? <Text code>{cell}</Text> : cell}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function MethodTag({ method }) {
  const colorMap = {
    GET: 'blue',
    POST: 'green',
    PUT: 'orange',
    DELETE: 'red',
  };
  return <Tag color={colorMap[method] || 'grey'}>{method}</Tag>;
}

const ASSET_API_FIELD_ROWS = [
  ['object', 'string', '固定为 asset'],
  ['id', 'string', '资源 ID，格式 asset_xxx'],
  ['task_id', 'string', 'new-api 任务 ID，格式 task_xxx'],
  ['index', 'integer', '同一任务内的资源序号，从 0 开始'],
  ['type', 'string', '资源类型：image、video、audio、file'],
  ['url', 'string', '资源直链，用于预览、打开、下载'],
  ['thumbnail_url', 'string', '可选，缩略图 URL'],
  ['mime_type', 'string', '可选，资源 MIME 类型'],
  ['filename', 'string', '可选，建议文件名'],
  ['size_bytes', 'integer', '可选，资源大小，单位 byte'],
  ['width', 'integer', '可选，图片/视频宽度'],
  ['height', 'integer', '可选，图片/视频高度'],
  ['duration_ms', 'integer', '可选，视频/音频时长，单位毫秒'],
  ['model', 'string', '生成资源使用的模型'],
  ['platform', 'string', '任务平台，例如 58'],
  ['action', 'string', '任务动作，例如 imageGeneration、imageEdit'],
  ['status', 'string', '资源状态；外部 API 默认只返回 available'],
  ['metadata', 'object', '可选，非敏感元数据'],
  ['created_at', 'integer', 'Unix 秒级创建时间'],
  ['updated_at', 'integer', 'Unix 秒级更新时间'],
];

const ASSET_URL_FIELD_ROWS = [
  ['asset_id', 'string', '资源 ID，格式 asset_xxx'],
  ['task_id', 'string', 'new-api 任务 ID，格式 task_xxx'],
  ['asset_type', 'string', '资源类型：image、video、audio、file'],
  ['url', 'string', '资源直链'],
];

const ASSET_API_STATUS_ROWS = [
  ['200', '请求成功；列表、详情和 URL 批量接口返回 JSON，导出接口返回 text/csv'],
  ['400', '请求参数或 JSON body 无效，例如 asset_type 不在允许枚举内'],
  ['401', '缺少 Authorization、Authorization 不是 Bearer ak_...，或 Key 不存在'],
  ['403', 'Key 已禁用、已过期、IP 不允许，或 Key 所属用户被禁用'],
  ['404', '资源不存在，或资源不属于当前 Key 对应用户'],
  ['500', '服务端错误'],
];

const ASSET_API_ERROR_CODE_ROWS = [
  ['invalid_request', '请求参数或 JSON body 无效'],
  ['access_denied', '认证或授权失败'],
  ['not_found', '资源不存在或不可见'],
  ['server_error', '服务端错误'],
];

const ASSET_API_AUTH_ROWS = [
  ['Header', 'Authorization', '是', 'Bearer ak_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'],
];

const ASSET_API_COMMON_QUERY_ROWS = [
  ['asset_type', 'string', '否', '资源类型：image、video、audio、file'],
  ['status', 'string', '否', '资源状态；外部 API 只会返回当前用户可见资源，通常使用 available'],
  ['task_id', 'string', '否', '按任务 ID 精确筛选'],
  ['model', 'string', '否', '按模型名称精确筛选'],
  ['platform', 'string', '否', '按任务平台筛选，例如 58'],
  ['action', 'string', '否', '按任务动作筛选，例如 imageGeneration、imageEdit'],
  ['channel_id', 'integer', '否', '按渠道 ID 筛选'],
  ['keyword', 'string', '否', '按 asset_id、task_id、filename、url 模糊搜索'],
  ['start_timestamp', 'integer', '否', '创建时间下限，Unix 秒'],
  ['end_timestamp', 'integer', '否', '创建时间上限，Unix 秒'],
];

const ASSET_API_LIST_QUERY_ROWS = [
  ...ASSET_API_COMMON_QUERY_ROWS,
  ['page', 'integer', '否', '页码，默认 1；兼容 p'],
  ['page_size', 'integer', '否', '每页数量，默认 20，最大 100；兼容 ps、size'],
];

const ASSET_API_FILTER_BODY_ROWS = [
  ['asset_ids', 'string[]', '否', '资源 ID 数组，最多 100 个；传入后优先按 ID 批量查询'],
  ['asset_type', 'string', '否', '资源类型：image、video、audio、file'],
  ['task_id', 'string', '否', '按任务 ID 精确筛选'],
  ['model', 'string', '否', '按模型名称精确筛选'],
  ['platform', 'string', '否', '按任务平台筛选，例如 58'],
  ['action', 'string', '否', '按任务动作筛选，例如 imageGeneration、imageEdit'],
  ['start_timestamp', 'integer', '否', '创建时间下限，Unix 秒'],
  ['end_timestamp', 'integer', '否', '创建时间上限，Unix 秒'],
  ['page', 'integer', '否', '页码，默认 1'],
  ['page_size', 'integer', '否', '每页数量，默认 20，最大 100'],
];

const ASSET_API_ENDPOINTS = [
  {
    method: 'GET',
    path: '/v1/assets',
    title: '查询资源列表',
    description: '按资源类型、任务 ID、模型、渠道、关键词、时间范围等条件分页查询当前 Key 所属用户的可用资源。',
    query: ASSET_API_LIST_QUERY_ROWS,
    responseFields: ASSET_API_FIELD_ROWS,
    curl: `curl "$BASE_URL/v1/assets?asset_type=image&page=1&page_size=20" \\
  -H "Authorization: Bearer $ASSET_KEY"`,
    response: `{
  "object": "list",
  "data": [
    {
      "object": "asset",
      "id": "asset_xxx",
      "task_id": "task_xxx",
      "index": 0,
      "type": "image",
      "url": "https://cdn.example.com/image.webp",
      "thumbnail_url": "https://cdn.example.com/thumb.webp",
      "mime_type": "image/webp",
      "filename": "image.webp",
      "width": 2048,
      "height": 2048,
      "model": "gpt-image-2",
      "platform": "58",
      "action": "imageGeneration",
      "status": "available",
      "created_at": 1782152450,
      "updated_at": 1782152450
    }
  ],
  "page": 1,
  "page_size": 20,
  "total": 1,
  "has_more": false
}`,
  },
  {
    method: 'GET',
    path: '/v1/assets/{asset_id}',
    title: '查询单个资源',
    description: '按资源 ID 查询资源详情。只能查询当前 Key 所属用户的可用资源。',
    pathParams: [['asset_id', 'string', '是', '资源 ID，格式 asset_xxx']],
    responseFields: ASSET_API_FIELD_ROWS,
    curl: `curl "$BASE_URL/v1/assets/asset_xxx" \\
  -H "Authorization: Bearer $ASSET_KEY"`,
    response: `{
  "object": "asset",
  "id": "asset_xxx",
  "task_id": "task_xxx",
  "index": 0,
  "type": "image",
  "url": "https://cdn.example.com/image.webp",
  "filename": "image.webp",
  "model": "gpt-image-2",
  "platform": "58",
  "action": "imageGeneration",
  "status": "available",
  "created_at": 1782152450,
  "updated_at": 1782152450
}`,
  },
  {
    method: 'POST',
    path: '/v1/assets/query',
    title: '批量查询资源',
    description: '支持按 asset_ids 批量查询，或用 JSON body 提交筛选条件查询资源列表。asset_ids 为空时走筛选分页。',
    body: ASSET_API_FILTER_BODY_ROWS,
    responseFields: ASSET_API_FIELD_ROWS,
    curl: `curl "$BASE_URL/v1/assets/query" \\
  -H "Authorization: Bearer $ASSET_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"asset_ids":["asset_xxx","asset_yyy"]}'`,
    response: `{
  "object": "list",
  "data": [
    {
      "object": "asset",
      "id": "asset_xxx",
      "task_id": "task_xxx",
      "index": 0,
      "type": "image",
      "url": "https://cdn.example.com/image.webp",
      "status": "available",
      "created_at": 1782152450,
      "updated_at": 1782152450
    }
  ],
  "page": 1,
  "page_size": 2,
  "total": 1,
  "has_more": false
}`,
  },
  {
    method: 'POST',
    path: '/v1/assets/batch/urls',
    title: '批量获取 URL',
    description: '只返回资源 ID、任务 ID、资源类型和 URL，适合下载器或脚本快速获取直链。asset_ids 最多处理 100 个，重复和空值会被忽略。',
    body: [['asset_ids', 'string[]', '是', '资源 ID 数组，最多 100 个']],
    responseFields: ASSET_URL_FIELD_ROWS,
    curl: `curl "$BASE_URL/v1/assets/batch/urls" \\
  -H "Authorization: Bearer $ASSET_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{"asset_ids":["asset_xxx","asset_yyy"]}'`,
    response: `{
  "object": "list",
  "data": [
    {
      "asset_id": "asset_xxx",
      "task_id": "task_xxx",
      "asset_type": "image",
      "url": "https://cdn.example.com/image.webp"
    }
  ]
}`,
  },
  {
    method: 'GET',
    path: '/v1/assets/export',
    title: '导出 CSV',
    description: '按筛选条件导出最多 10000 条 URL 清单。响应类型为 text/csv，适合交给下载器、脚本或表格工具处理。',
    query: ASSET_API_COMMON_QUERY_ROWS,
    responseFields: [
      ['asset_id', 'string', '资源 ID'],
      ['task_id', 'string', '任务 ID'],
      ['asset_type', 'string', '资源类型'],
      ['url', 'string', '资源直链'],
      ['filename', 'string', '文件名'],
      ['model', 'string', '模型'],
      ['platform', 'string', '平台'],
      ['action', 'string', '动作'],
      ['created_at', 'integer', 'Unix 秒级创建时间'],
    ],
    curl: `curl "$BASE_URL/v1/assets/export?asset_type=video" \\
  -H "Authorization: Bearer $ASSET_KEY" \\
  -o assets.csv`,
    response: `asset_id,task_id,asset_type,url,filename,model,platform,action,created_at
asset_xxx,task_xxx,image,https://cdn.example.com/image.webp,image.webp,gpt-image-2,58,imageGeneration,1782152450`,
  },
];

const ASSET_API_OPENAPI_SPEC = {
  openapi: '3.0.3',
  info: {
    title: 'new-api Assets API',
    version: '1.0.0',
    description: '资源管理中心只读 API，用于通过资源 API Key 查询、导出和获取异步图片、视频、音频、文件资源直链。',
  },
  servers: [
    {
      url: '{base_url}',
      variables: {
        base_url: {
          default: 'https://new-api.example.com',
          description: '替换为你的 new-api 访问地址',
        },
      },
    },
  ],
  tags: [{ name: 'Assets', description: '资源查询、批量 URL 和 CSV 导出' }],
  security: [{ AssetKeyAuth: [] }],
  components: {
    securitySchemes: {
      AssetKeyAuth: {
        type: 'http',
        scheme: 'bearer',
        bearerFormat: 'ak_*',
        description: '在资源管理中心的 API Key Tab 创建，格式为 Authorization: Bearer ak_xxx',
      },
    },
    parameters: {
      AssetTypeQuery: {
        name: 'asset_type',
        in: 'query',
        required: false,
        schema: { $ref: '#/components/schemas/AssetType' },
        description: '资源类型',
      },
      StatusQuery: {
        name: 'status',
        in: 'query',
        required: false,
        schema: { type: 'string', enum: ['available'] },
        description: '资源状态；外部 API 只返回当前用户可见资源，通常使用 available',
      },
      TaskIdQuery: {
        name: 'task_id',
        in: 'query',
        required: false,
        schema: { type: 'string' },
        description: '按任务 ID 精确筛选',
      },
      ModelQuery: {
        name: 'model',
        in: 'query',
        required: false,
        schema: { type: 'string' },
        description: '按模型名称精确筛选',
      },
      PlatformQuery: {
        name: 'platform',
        in: 'query',
        required: false,
        schema: { type: 'string', example: '58' },
        description: '按任务平台筛选，例如 58',
      },
      ActionQuery: {
        name: 'action',
        in: 'query',
        required: false,
        schema: { type: 'string', example: 'imageGeneration' },
        description: '按任务动作筛选，例如 imageGeneration、imageEdit',
      },
      ChannelIdQuery: {
        name: 'channel_id',
        in: 'query',
        required: false,
        schema: { type: 'integer' },
        description: '按渠道 ID 筛选',
      },
      KeywordQuery: {
        name: 'keyword',
        in: 'query',
        required: false,
        schema: { type: 'string' },
        description: '按 asset_id、task_id、filename、url 模糊搜索',
      },
      StartTimestampQuery: {
        name: 'start_timestamp',
        in: 'query',
        required: false,
        schema: { type: 'integer', format: 'int64' },
        description: '创建时间下限，Unix 秒',
      },
      EndTimestampQuery: {
        name: 'end_timestamp',
        in: 'query',
        required: false,
        schema: { type: 'integer', format: 'int64' },
        description: '创建时间上限，Unix 秒',
      },
      PageQuery: {
        name: 'page',
        in: 'query',
        required: false,
        schema: { type: 'integer', minimum: 1, default: 1 },
        description: '页码，默认 1；兼容参数 p',
      },
      PageSizeQuery: {
        name: 'page_size',
        in: 'query',
        required: false,
        schema: { type: 'integer', minimum: 1, maximum: 100, default: 20 },
        description: '每页数量，默认 20，最大 100；兼容参数 ps、size',
      },
    },
    schemas: {
      AssetType: {
        type: 'string',
        enum: ['image', 'video', 'audio', 'file'],
        example: 'image',
      },
      Asset: {
        type: 'object',
        required: ['object', 'id', 'task_id', 'index', 'type', 'url', 'status', 'created_at', 'updated_at'],
        properties: {
          object: { type: 'string', enum: ['asset'], example: 'asset' },
          id: { type: 'string', example: 'asset_xxx', description: '资源 ID' },
          task_id: { type: 'string', example: 'task_xxx', description: 'new-api 任务 ID' },
          index: { type: 'integer', example: 0, description: '同一任务内的资源序号，从 0 开始' },
          type: { $ref: '#/components/schemas/AssetType' },
          url: { type: 'string', format: 'uri', example: 'https://cdn.example.com/image.webp' },
          thumbnail_url: { type: 'string', format: 'uri', nullable: true, example: 'https://cdn.example.com/thumb.webp' },
          mime_type: { type: 'string', nullable: true, example: 'image/webp' },
          filename: { type: 'string', nullable: true, example: 'image.webp' },
          size_bytes: { type: 'integer', format: 'int64', nullable: true, example: 1024000 },
          width: { type: 'integer', nullable: true, example: 2048 },
          height: { type: 'integer', nullable: true, example: 2048 },
          duration_ms: { type: 'integer', format: 'int64', nullable: true, example: 12000 },
          model: { type: 'string', nullable: true, example: 'gpt-image-2' },
          platform: { type: 'string', nullable: true, example: '58' },
          action: { type: 'string', nullable: true, example: 'imageGeneration' },
          status: { type: 'string', enum: ['available'], example: 'available' },
          metadata: { type: 'object', nullable: true, additionalProperties: true },
          created_at: { type: 'integer', format: 'int64', example: 1782152450 },
          updated_at: { type: 'integer', format: 'int64', example: 1782152450 },
        },
      },
      AssetListResponse: {
        type: 'object',
        required: ['object', 'data', 'page', 'page_size', 'total', 'has_more'],
        properties: {
          object: { type: 'string', enum: ['list'], example: 'list' },
          data: { type: 'array', items: { $ref: '#/components/schemas/Asset' } },
          page: { type: 'integer', example: 1 },
          page_size: { type: 'integer', example: 20 },
          total: { type: 'integer', format: 'int64', example: 1 },
          has_more: { type: 'boolean', example: false },
        },
      },
      AssetQueryRequest: {
        type: 'object',
        properties: {
          asset_ids: {
            type: 'array',
            maxItems: 100,
            items: { type: 'string' },
            description: '资源 ID 数组；传入后优先按 ID 批量查询',
            example: ['asset_xxx', 'asset_yyy'],
          },
          asset_type: { $ref: '#/components/schemas/AssetType' },
          task_id: { type: 'string', example: 'task_xxx' },
          model: { type: 'string', example: 'gpt-image-2' },
          platform: { type: 'string', example: '58' },
          action: { type: 'string', example: 'imageGeneration' },
          start_timestamp: { type: 'integer', format: 'int64', example: 1782150000 },
          end_timestamp: { type: 'integer', format: 'int64', example: 1782160000 },
          page: { type: 'integer', minimum: 1, default: 1 },
          page_size: { type: 'integer', minimum: 1, maximum: 100, default: 20 },
        },
      },
      AssetBatchURLRequest: {
        type: 'object',
        required: ['asset_ids'],
        properties: {
          asset_ids: {
            type: 'array',
            maxItems: 100,
            items: { type: 'string' },
            example: ['asset_xxx', 'asset_yyy'],
          },
        },
      },
      AssetURLItem: {
        type: 'object',
        required: ['asset_id', 'task_id', 'asset_type', 'url'],
        properties: {
          asset_id: { type: 'string', example: 'asset_xxx' },
          task_id: { type: 'string', example: 'task_xxx' },
          asset_type: { $ref: '#/components/schemas/AssetType' },
          url: { type: 'string', format: 'uri', example: 'https://cdn.example.com/image.webp' },
        },
      },
      AssetURLListResponse: {
        type: 'object',
        required: ['object', 'data'],
        properties: {
          object: { type: 'string', enum: ['list'], example: 'list' },
          data: { type: 'array', items: { $ref: '#/components/schemas/AssetURLItem' } },
        },
      },
      ErrorResponse: {
        type: 'object',
        required: ['error'],
        properties: {
          error: {
            type: 'object',
            required: ['message', 'type', 'code'],
            properties: {
              message: { type: 'string', example: '资源 API Key 已禁用 (request id: ...)' },
              type: { type: 'string', example: 'new_api_error' },
              code: { type: 'string', example: 'access_denied' },
            },
          },
        },
      },
    },
    responses: {
      BadRequest: {
        description: '请求参数或 JSON body 无效',
        content: { 'application/json': { schema: { $ref: '#/components/schemas/ErrorResponse' } } },
      },
      Unauthorized: {
        description: '缺少 Authorization 或 Key 无效',
        content: { 'application/json': { schema: { $ref: '#/components/schemas/ErrorResponse' } } },
      },
      Forbidden: {
        description: 'Key 已禁用、已过期、IP 不允许，或用户不可用',
        content: { 'application/json': { schema: { $ref: '#/components/schemas/ErrorResponse' } } },
      },
      NotFound: {
        description: '资源不存在或不属于当前用户',
        content: { 'application/json': { schema: { $ref: '#/components/schemas/ErrorResponse' } } },
      },
      ServerError: {
        description: '服务端错误',
        content: { 'application/json': { schema: { $ref: '#/components/schemas/ErrorResponse' } } },
      },
    },
  },
  paths: {
    '/v1/assets': {
      get: {
        tags: ['Assets'],
        operationId: 'listAssets',
        summary: '查询资源列表',
        description: '分页查询当前资源 API Key 所属用户的可用资源。',
        security: [{ AssetKeyAuth: [] }],
        parameters: [
          { $ref: '#/components/parameters/AssetTypeQuery' },
          { $ref: '#/components/parameters/StatusQuery' },
          { $ref: '#/components/parameters/TaskIdQuery' },
          { $ref: '#/components/parameters/ModelQuery' },
          { $ref: '#/components/parameters/PlatformQuery' },
          { $ref: '#/components/parameters/ActionQuery' },
          { $ref: '#/components/parameters/ChannelIdQuery' },
          { $ref: '#/components/parameters/KeywordQuery' },
          { $ref: '#/components/parameters/StartTimestampQuery' },
          { $ref: '#/components/parameters/EndTimestampQuery' },
          { $ref: '#/components/parameters/PageQuery' },
          { $ref: '#/components/parameters/PageSizeQuery' },
        ],
        responses: {
          200: {
            description: '资源列表',
            content: { 'application/json': { schema: { $ref: '#/components/schemas/AssetListResponse' } } },
          },
          401: { $ref: '#/components/responses/Unauthorized' },
          403: { $ref: '#/components/responses/Forbidden' },
          500: { $ref: '#/components/responses/ServerError' },
        },
      },
    },
    '/v1/assets/{asset_id}': {
      get: {
        tags: ['Assets'],
        operationId: 'getAsset',
        summary: '查询单个资源',
        description: '按资源 ID 查询资源详情。',
        security: [{ AssetKeyAuth: [] }],
        parameters: [
          {
            name: 'asset_id',
            in: 'path',
            required: true,
            schema: { type: 'string' },
            description: '资源 ID，格式 asset_xxx',
          },
        ],
        responses: {
          200: {
            description: '资源详情',
            content: { 'application/json': { schema: { $ref: '#/components/schemas/Asset' } } },
          },
          401: { $ref: '#/components/responses/Unauthorized' },
          403: { $ref: '#/components/responses/Forbidden' },
          404: { $ref: '#/components/responses/NotFound' },
          500: { $ref: '#/components/responses/ServerError' },
        },
      },
    },
    '/v1/assets/query': {
      post: {
        tags: ['Assets'],
        operationId: 'queryAssets',
        summary: '批量查询资源',
        description: '按 asset_ids 批量查询，或用 JSON body 中的筛选条件分页查询。',
        security: [{ AssetKeyAuth: [] }],
        requestBody: {
          required: true,
          content: {
            'application/json': {
              schema: { $ref: '#/components/schemas/AssetQueryRequest' },
            },
          },
        },
        responses: {
          200: {
            description: '资源列表',
            content: { 'application/json': { schema: { $ref: '#/components/schemas/AssetListResponse' } } },
          },
          400: { $ref: '#/components/responses/BadRequest' },
          401: { $ref: '#/components/responses/Unauthorized' },
          403: { $ref: '#/components/responses/Forbidden' },
          500: { $ref: '#/components/responses/ServerError' },
        },
      },
    },
    '/v1/assets/batch/urls': {
      post: {
        tags: ['Assets'],
        operationId: 'getAssetBatchURLs',
        summary: '批量获取资源 URL',
        description: '按资源 ID 批量返回直链信息。',
        security: [{ AssetKeyAuth: [] }],
        requestBody: {
          required: true,
          content: {
            'application/json': {
              schema: { $ref: '#/components/schemas/AssetBatchURLRequest' },
            },
          },
        },
        responses: {
          200: {
            description: '资源 URL 列表',
            content: { 'application/json': { schema: { $ref: '#/components/schemas/AssetURLListResponse' } } },
          },
          400: { $ref: '#/components/responses/BadRequest' },
          401: { $ref: '#/components/responses/Unauthorized' },
          403: { $ref: '#/components/responses/Forbidden' },
          500: { $ref: '#/components/responses/ServerError' },
        },
      },
    },
    '/v1/assets/export': {
      get: {
        tags: ['Assets'],
        operationId: 'exportAssets',
        summary: '导出资源 CSV',
        description: '按筛选条件导出最多 10000 条资源 URL 清单。',
        security: [{ AssetKeyAuth: [] }],
        parameters: [
          { $ref: '#/components/parameters/AssetTypeQuery' },
          { $ref: '#/components/parameters/StatusQuery' },
          { $ref: '#/components/parameters/TaskIdQuery' },
          { $ref: '#/components/parameters/ModelQuery' },
          { $ref: '#/components/parameters/PlatformQuery' },
          { $ref: '#/components/parameters/ActionQuery' },
          { $ref: '#/components/parameters/ChannelIdQuery' },
          { $ref: '#/components/parameters/KeywordQuery' },
          { $ref: '#/components/parameters/StartTimestampQuery' },
          { $ref: '#/components/parameters/EndTimestampQuery' },
        ],
        responses: {
          200: {
            description: 'CSV 文件，字段为 asset_id,task_id,asset_type,url,filename,model,platform,action,created_at',
            content: {
              'text/csv': {
                schema: {
                  type: 'string',
                  example:
                    'asset_id,task_id,asset_type,url,filename,model,platform,action,created_at\\nasset_xxx,task_xxx,image,https://cdn.example.com/image.webp,image.webp,gpt-image-2,58,imageGeneration,1782152450',
                },
              },
            },
          },
          401: { $ref: '#/components/responses/Unauthorized' },
          403: { $ref: '#/components/responses/Forbidden' },
          500: { $ref: '#/components/responses/ServerError' },
        },
      },
    },
  },
};

function AssetPreview({ asset, className = '' }) {
  if (!asset) return null;
  if (asset.asset_type === 'video') {
    return (
      <video
        src={asset.url}
        poster={asset.thumbnail_url || undefined}
        className={`w-full h-full object-cover bg-black ${className}`}
        controls
        preload='metadata'
      />
    );
  }
  if (asset.asset_type === 'image') {
    return (
      <img
        src={asset.thumbnail_url || asset.url}
        alt={asset.filename || asset.asset_id}
        className={`w-full h-full object-cover ${className}`}
        loading='lazy'
      />
    );
  }
  return (
    <div className={`flex items-center justify-center bg-gray-50 ${className}`}>
      <ImageIcon size={32} />
    </div>
  );
}

export default function AssetsPage() {
  const { t } = useTranslation();
  const adminUser = isAdmin();
  const [formApi, setFormApi] = useState(null);
  const [keyFormApi, setKeyFormApi] = useState(null);
  const [activeMainTab, setActiveMainTab] = useState('assets');
  const [assets, setAssets] = useState([]);
  const [loading, setLoading] = useState(false);
  const [assetKeys, setAssetKeys] = useState([]);
  const [keysLoading, setKeysLoading] = useState(false);
  const [keyActivePage, setKeyActivePage] = useState(1);
  const [keyPageSize, setKeyPageSize] = useState(ITEMS_PER_PAGE);
  const [keyTotal, setKeyTotal] = useState(0);
  const [revealedKeyIds, setRevealedKeyIds] = useState([]);
  const [showCreateKeySheet, setShowCreateKeySheet] = useState(false);
  const [createKeyLoading, setCreateKeyLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [total, setTotal] = useState(0);
  const [assetType, setAssetType] = useState('');
  const [viewMode, setViewMode] = useState('grid');
  const [selectedRowKeys, setSelectedRowKeys] = useState([]);
  const [detailAsset, setDetailAsset] = useState(null);
  const [previewImage, setPreviewImage] = useState('');
  const [showOpenAPISheet, setShowOpenAPISheet] = useState(false);

  const now = new Date();
  const zeroNow = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const formInitValues = {
    asset_type: '',
    status: '',
    task_id: '',
    model: '',
    channel_id: '',
    user_id: '',
    keyword: '',
    dateRange: [
      timestamp2string(zeroNow.getTime() / 1000),
      timestamp2string(now.getTime() / 1000 + 3600),
    ],
  };

  const selectedAssets = useMemo(
    () => assets.filter((asset) => selectedRowKeys.includes(asset.asset_id)),
    [assets, selectedRowKeys],
  );

  const getQueryParams = () => {
    const values = formApi ? formApi.getValues() : {};
    let startTimestamp = '';
    let endTimestamp = '';
    if (
      values.dateRange &&
      Array.isArray(values.dateRange) &&
      values.dateRange.length === 2
    ) {
      startTimestamp = parseInt(Date.parse(values.dateRange[0]) / 1000);
      endTimestamp = parseInt(Date.parse(values.dateRange[1]) / 1000);
    }
    return {
      asset_type: assetType || values.asset_type || '',
      status: values.status || '',
      task_id: values.task_id || '',
      model: values.model || '',
      channel_id: values.channel_id || '',
      user_id: values.user_id || '',
      keyword: values.keyword || '',
      start_timestamp: startTimestamp || '',
      end_timestamp: endTimestamp || '',
    };
  };

  const loadAssets = async (page = activePage, size = pageSize) => {
    setLoading(true);
    try {
      const endpoint = adminUser ? '/api/assets/' : '/api/assets/self';
      const params = new URLSearchParams({
        p: String(page),
        page_size: String(size),
      });
      const queryParams = getQueryParams();
      Object.entries(queryParams).forEach(([key, value]) => {
        if (value !== undefined && value !== null && String(value) !== '') {
          params.set(key, String(value));
        }
      });
      const res = await API.get(`${endpoint}?${params.toString()}`);
      const { success, data, message } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setAssets(data.items || []);
      setTotal(data.total || 0);
      setActivePage(data.page || page);
      setPageSize(data.page_size || size);
      setSelectedRowKeys([]);
    } catch (error) {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  const loadAssetKeys = async (page = keyActivePage, size = keyPageSize) => {
    setKeysLoading(true);
    try {
      const res = await API.get(`/api/assets/keys?p=${page}&page_size=${size}`);
      const { success, data, message } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setAssetKeys(data.items || []);
      setKeyTotal(data.total || 0);
      setKeyActivePage(data.page || page);
      setKeyPageSize(data.page_size || size);
    } catch (error) {
      showError(t('加载失败'));
    } finally {
      setKeysLoading(false);
    }
  };

  useEffect(() => {
    loadAssets(1, pageSize);
  }, [assetType]);

  useEffect(() => {
    if (activeMainTab === 'keys') {
      loadAssetKeys(1, keyPageSize);
    }
  }, [activeMainTab]);

  const exportCsv = async () => {
    const endpoint = adminUser ? '/api/assets/export' : '/api/assets/self/export';
    const params = new URLSearchParams();
    const queryParams = getQueryParams();
    Object.entries(queryParams).forEach(([key, value]) => {
      if (value !== undefined && value !== null && String(value) !== '') {
        params.set(key, String(value));
      }
    });
    const res = await API.get(`${endpoint}?${params.toString()}`);
    downloadTextAsFile(res.data, `assets-${Date.now()}.csv`);
    showSuccess(t('已导出 CSV'));
  };

  const copySelectedUrls = async () => {
    const urls = selectedAssets.map((asset) => asset.url).filter(Boolean);
    if (urls.length === 0) return;
    if (await copy(urls.join('\n'))) {
      showSuccess(t('已复制链接'));
    }
  };

  const downloadSelected = () => {
    if (selectedAssets.length === 0) return;
    if (selectedAssets.length > DIRECT_DOWNLOAD_LIMIT) {
      showError(t('选中资源过多，请导出 CSV 后使用下载工具处理'));
      return;
    }
    selectedAssets.forEach((asset) => {
      triggerDownload(asset.url, buildDownloadName(asset));
    });
  };

  const createAssetKey = async () => {
    const values = keyFormApi?.getValues() || {};
    const name = (values.name || '').trim();
    if (!name) {
      showError(t('请输入名称'));
      return;
    }
    let expiredAt = -1;
    if (values.expired_at && values.expired_at !== -1) {
      const parsed = Date.parse(values.expired_at);
      if (Number.isNaN(parsed)) {
        showError(t('过期时间格式错误！'));
        return;
      }
      expiredAt = Math.ceil(parsed / 1000);
      if (expiredAt <= Math.floor(Date.now() / 1000)) {
        showError(t('过期时间不能早于当前时间！'));
        return;
      }
    }
    setCreateKeyLoading(true);
    try {
      const res = await API.post('/api/assets/keys', {
        name,
        expired_at: expiredAt,
        allow_ips: values.allow_ips || '',
      });
      const { success, message } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      showSuccess(t('创建成功'));
      setShowCreateKeySheet(false);
      keyFormApi?.reset();
      loadAssetKeys(1, keyPageSize);
    } catch (error) {
      showError(t('创建失败'));
    } finally {
      setCreateKeyLoading(false);
    }
  };

  const updateAssetKeyStatus = async (record, status) => {
    try {
      const res = await API.put(`/api/assets/keys/${record.id}/status`, { status });
      const { success, message } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      showSuccess(t('更新成功'));
      loadAssetKeys(keyActivePage, keyPageSize);
    } catch (error) {
      showError(t('更新失败'));
    }
  };

  const deleteAssetKey = async (record) => {
    try {
      const res = await API.delete(`/api/assets/keys/${record.id}`);
      const { success, message } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      showSuccess(t('删除成功'));
      setRevealedKeyIds((current) => current.filter((id) => id !== record.id));
      loadAssetKeys(keyActivePage, keyPageSize);
    } catch (error) {
      showError(t('删除失败'));
    }
  };

  const toggleRevealKey = (id) => {
    setRevealedKeyIds((current) =>
      current.includes(id)
        ? current.filter((item) => item !== id)
        : [...current, id],
    );
  };

  const getAssetOpenAPIJSON = () => JSON.stringify(ASSET_API_OPENAPI_SPEC, null, 2);

  const copyAssetOpenAPIJSON = async () => {
    if (await copy(getAssetOpenAPIJSON())) {
      showSuccess(t('已复制 OpenAPI JSON'));
    }
  };

  const downloadAssetOpenAPIJSON = () => {
    downloadTextAsFile(getAssetOpenAPIJSON(), 'new-api-assets-openapi.json');
    showSuccess(t('已导出 OpenAPI JSON'));
  };

  const columns = [
    {
      title: t('资源'),
      dataIndex: 'url',
      render: (_, record) => (
        <Space>
          <div className='w-14 h-14 rounded-md overflow-hidden border border-solid border-semi-color-border'>
            <AssetPreview asset={record} />
          </div>
          <div className='min-w-0'>
            <div className='flex items-center gap-2'>
              <Tag prefixIcon={assetTypeIcon(record.asset_type)}>
                {assetTypeLabel(record.asset_type, t)}
              </Tag>
              {statusTag(record.status, t)}
            </div>
            <Text size='small' ellipsis={{ showTooltip: true }}>
              {record.asset_id}
            </Text>
          </div>
        </Space>
      ),
    },
    {
      title: t('模型'),
      dataIndex: 'model',
      render: (text) => text || '-',
    },
    {
      title: t('任务 ID'),
      dataIndex: 'task_id',
      render: (text) => <Text copyable>{text}</Text>,
    },
    {
      title: t('时间'),
      dataIndex: 'created_at',
      render: (time) => timestamp2string(time),
    },
    adminUser && {
      title: t('用户'),
      dataIndex: 'username',
      render: (_, record) => record.username || record.user_id || '-',
    },
    {
      title: t('操作'),
      dataIndex: 'actions',
      render: (_, record) => (
        <Space>
          <Tooltip content={t('预览')}>
            <Button icon={<Grid3X3 size={14} />} size='small' onClick={() => setDetailAsset(record)} />
          </Tooltip>
          <Tooltip content={t('复制链接')}>
            <Button
              icon={<Copy size={14} />}
              size='small'
              onClick={() => copy(record.url).then((ok) => ok && showSuccess(t('已复制链接')))}
            />
          </Tooltip>
          <Tooltip content={t('打开')}>
            <Button
              icon={<ExternalLink size={14} />}
              size='small'
              onClick={() => window.open(record.url, '_blank', 'noreferrer')}
            />
          </Tooltip>
        </Space>
      ),
    },
  ].filter(Boolean);

  const keyColumns = [
    {
      title: t('名称'),
      dataIndex: 'name',
      render: (text) => <Text ellipsis={{ showTooltip: true }}>{text}</Text>,
    },
    {
      title: t('API Key'),
      dataIndex: 'key',
      render: (text, record) => {
        const revealed = revealedKeyIds.includes(record.id);
        return (
          <Space>
            <Text code ellipsis={{ showTooltip: true }}>
              {revealed ? text : maskAssetKey(text)}
            </Text>
            <Tooltip content={revealed ? t('隐藏') : t('显示')}>
              <Button
                size='small'
                icon={revealed ? <EyeOff size={14} /> : <Eye size={14} />}
                onClick={() => toggleRevealKey(record.id)}
              />
            </Tooltip>
            <Tooltip content={t('复制')}>
              <Button
                size='small'
                icon={<Copy size={14} />}
                onClick={() => copy(text).then((ok) => ok && showSuccess(t('已复制')))}
              />
            </Tooltip>
          </Space>
        );
      },
    },
    {
      title: t('状态'),
      dataIndex: 'status',
      render: (status) =>
        status === ASSET_KEY_ENABLED ? (
          <Tag color='green'>{t('启用')}</Tag>
        ) : (
          <Tag color='grey'>{t('禁用')}</Tag>
        ),
    },
    {
      title: t('过期时间'),
      dataIndex: 'expired_at',
      render: (expiredAt) => formatExpireTime(expiredAt, t),
    },
    {
      title: t('最后使用'),
      dataIndex: 'last_used_at',
      render: (lastUsedAt) => (lastUsedAt ? timestamp2string(lastUsedAt) : '-'),
    },
    {
      title: t('创建时间'),
      dataIndex: 'created_at',
      render: (createdAt) => timestamp2string(createdAt),
    },
    {
      title: t('操作'),
      dataIndex: 'actions',
      render: (_, record) => (
        <Space>
          {record.status === ASSET_KEY_ENABLED ? (
            <Tooltip content={t('禁用')}>
              <Button
                size='small'
                icon={<PowerOff size={14} />}
                onClick={() => updateAssetKeyStatus(record, ASSET_KEY_DISABLED)}
              />
            </Tooltip>
          ) : (
            <Tooltip content={t('启用')}>
              <Button
                size='small'
                icon={<Power size={14} />}
                onClick={() => updateAssetKeyStatus(record, ASSET_KEY_ENABLED)}
              />
            </Tooltip>
          )}
          <Popconfirm
            title={t('确定删除该 API Key？')}
            content={t('删除后使用该 Key 的程序将无法继续访问资源接口')}
            onConfirm={() => deleteAssetKey(record)}
          >
            <Button size='small' type='danger' icon={<Trash2 size={14} />} />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const renderAssetsTab = () => (
    <div className='flex flex-col gap-3'>
      <div className='flex flex-col md:flex-row md:items-center md:justify-between gap-2'>
        <div>
          <Title heading={5} className='!mb-1'>
            {t('资源列表')}
          </Title>
          <Text type='tertiary'>
            {t('集中查看、筛选和导出异步生成的图片与视频资源')}
          </Text>
        </div>
        <Space wrap>
          <Button icon={<RefreshCcw size={14} />} onClick={() => loadAssets(1, pageSize)} loading={loading}>
            {t('刷新')}
          </Button>
          <Button icon={<FileSpreadsheet size={14} />} onClick={exportCsv}>
            {t('导出 CSV')}
          </Button>
        </Space>
      </div>

      <Form
        initValues={formInitValues}
        getFormApi={setFormApi}
        onSubmit={() => loadAssets(1, pageSize)}
        allowEmpty
        autoComplete='off'
        layout='vertical'
      >
        <div className='grid grid-cols-1 md:grid-cols-2 xl:grid-cols-6 gap-2'>
          <div className='xl:col-span-2'>
            <Form.DatePicker
              field='dateRange'
              className='w-full'
              type='dateTimeRange'
              placeholder={[t('开始时间'), t('结束时间')]}
              showClear
              pure
              size='small'
              presets={DATE_RANGE_PRESETS.map((preset) => ({
                text: t(preset.text),
                start: preset.start(),
                end: preset.end(),
              }))}
            />
          </div>
          <Form.Input field='task_id' placeholder={t('任务 ID')} showClear pure size='small' />
          <Form.Input field='model' placeholder={t('模型')} showClear pure size='small' />
          <Form.Input field='keyword' placeholder={t('关键词')} showClear pure size='small' />
          <Form.Select field='status' placeholder={t('状态')} showClear pure size='small'>
            {statusOptions.map((option) => (
              <Select.Option key={option.value} value={option.value}>
                {t(option.label)}
              </Select.Option>
            ))}
          </Form.Select>
          {adminUser && (
            <>
              <Form.Input field='user_id' placeholder={t('用户 ID')} showClear pure size='small' />
              <Form.Input field='channel_id' placeholder={t('渠道 ID')} showClear pure size='small' />
            </>
          )}
        </div>
        <div className='flex justify-between items-center mt-2 gap-2 flex-wrap'>
          <Tabs
            type='button'
            activeKey={assetType}
            onChange={(key) => setAssetType(key)}
            tabList={assetTypeOptions.slice(0, 3).map((option) => ({
              tab: t(option.label),
              itemKey: option.value,
            }))}
          />
          <Space wrap>
            <Button htmlType='submit' type='tertiary' loading={loading}>
              {t('查询')}
            </Button>
            <Button
              type='tertiary'
              onClick={() => {
                formApi?.reset();
                setAssetType('');
                setTimeout(() => loadAssets(1, pageSize), 100);
              }}
            >
              {t('重置')}
            </Button>
            <Button
              icon={viewMode === 'grid' ? <List size={14} /> : <Grid3X3 size={14} />}
              onClick={() => setViewMode(viewMode === 'grid' ? 'table' : 'grid')}
            >
              {viewMode === 'grid' ? t('表格') : t('网格')}
            </Button>
          </Space>
        </div>
      </Form>

      <div className='flex items-center justify-between gap-2 min-h-[36px]'>
        <Text type='tertiary'>
          {selectedAssets.length > 0
            ? t('已选择 {{count}} 个资源', { count: selectedAssets.length })
            : t('共 {{count}} 个资源', { count: total })}
        </Text>
        <Space wrap>
          <Button icon={<Copy size={14} />} disabled={selectedAssets.length === 0} onClick={copySelectedUrls}>
            {t('复制链接')}
          </Button>
          <Button icon={<Download size={14} />} disabled={selectedAssets.length === 0} onClick={downloadSelected}>
            {t('下载选中')}
          </Button>
        </Space>
      </div>

      {viewMode === 'grid' ? (
        <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3'>
          {assets.map((asset) => (
            <div
              key={asset.asset_id}
              className={`border border-solid rounded-lg overflow-hidden bg-white dark:bg-semi-color-bg-1 ${
                selectedRowKeys.includes(asset.asset_id)
                  ? 'border-semi-color-primary'
                  : 'border-semi-color-border'
              }`}
            >
              <button
                type='button'
                className='w-full aspect-[4/3] border-0 p-0 cursor-pointer bg-transparent'
                onClick={() => setDetailAsset(asset)}
              >
                <AssetPreview asset={asset} />
              </button>
              <div className='p-3 flex flex-col gap-2'>
                <div className='flex items-center justify-between gap-2'>
                  <Tag prefixIcon={assetTypeIcon(asset.asset_type)}>
                    {assetTypeLabel(asset.asset_type, t)}
                  </Tag>
                  {statusTag(asset.status, t)}
                </div>
                <Text ellipsis={{ showTooltip: true }} strong>
                  {asset.model || asset.asset_id}
                </Text>
                <Text size='small' type='tertiary' ellipsis={{ showTooltip: true }}>
                  {asset.task_id}
                </Text>
                <Text size='small' type='tertiary'>
                  {timestamp2string(asset.created_at)}
                </Text>
                <div className='flex items-center justify-between'>
                  <label className='flex items-center gap-2 text-sm'>
                    <input
                      type='checkbox'
                      checked={selectedRowKeys.includes(asset.asset_id)}
                      onChange={(e) => {
                        setSelectedRowKeys((current) =>
                          e.target.checked
                            ? [...current, asset.asset_id]
                            : current.filter((key) => key !== asset.asset_id),
                        );
                      }}
                    />
                    {t('选择')}
                  </label>
                  <Space>
                    <Button
                      icon={<Copy size={14} />}
                      size='small'
                      onClick={() => copy(asset.url).then((ok) => ok && showSuccess(t('已复制链接')))}
                    />
                    <Button
                      icon={<ExternalLink size={14} />}
                      size='small'
                      onClick={() => window.open(asset.url, '_blank', 'noreferrer')}
                    />
                  </Space>
                </div>
              </div>
            </div>
          ))}
          {!loading && assets.length === 0 && (
            <div className='col-span-full py-12'>
              <Empty description={t('暂无资源')} />
            </div>
          )}
        </div>
      ) : (
        <Table
          columns={columns}
          dataSource={assets}
          rowKey='asset_id'
          loading={loading}
          pagination={false}
          rowSelection={{
            selectedRowKeys,
            onChange: setSelectedRowKeys,
          }}
        />
      )}

      <div className='flex justify-end'>
        <Pagination
          currentPage={activePage}
          pageSize={pageSize}
          total={total}
          showSizeChanger
          pageSizeOptions={[10, 20, 50, 100]}
          onPageChange={(page) => loadAssets(page, pageSize)}
          onPageSizeChange={(size) => loadAssets(1, size)}
        />
      </div>
    </div>
  );

  const renderKeysTab = () => (
    <div className='flex flex-col gap-3'>
      <div className='flex flex-col md:flex-row md:items-center md:justify-between gap-2'>
        <div>
          <Title heading={5} className='!mb-1'>
            {t('API Key')}
          </Title>
          <Text type='tertiary'>
            {t('生成专用于资源查询的只读 Key，可随时查看、复制、禁用或删除')}
          </Text>
        </div>
        <Space wrap>
          <Button icon={<RefreshCcw size={14} />} onClick={() => loadAssetKeys(1, keyPageSize)} loading={keysLoading}>
            {t('刷新')}
          </Button>
          <Button icon={<Plus size={14} />} type='primary' onClick={() => setShowCreateKeySheet(true)}>
            {t('创建 Key')}
          </Button>
        </Space>
      </div>
      <Table
        columns={keyColumns}
        dataSource={assetKeys}
        rowKey='id'
        loading={keysLoading}
        pagination={false}
      />
      <div className='flex justify-end'>
        <Pagination
          currentPage={keyActivePage}
          pageSize={keyPageSize}
          total={keyTotal}
          showSizeChanger
          pageSizeOptions={[10, 20, 50, 100]}
          onPageChange={(page) => loadAssetKeys(page, keyPageSize)}
          onPageSizeChange={(size) => loadAssetKeys(1, size)}
        />
      </div>
    </div>
  );

  const renderDocsTab = () => (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-col md:flex-row md:items-center md:justify-between gap-2'>
        <div>
          <Title heading={5} className='!mb-1'>
            {t('Assets API Reference')}
          </Title>
          <Text type='tertiary'>
            {t('资源查询 API 的请求参数、响应参数、状态码和 OpenAPI 文档。')}
          </Text>
        </div>
        <Space wrap>
          <Button icon={<Eye size={14} />} onClick={() => setShowOpenAPISheet(true)}>
            {t('查看 OpenAPI')}
          </Button>
          <Button icon={<Copy size={14} />} onClick={copyAssetOpenAPIJSON}>
            {t('复制 OpenAPI')}
          </Button>
          <Button icon={<Download size={14} />} onClick={downloadAssetOpenAPIJSON}>
            {t('下载 OpenAPI')}
          </Button>
        </Space>
      </div>

      <div className='grid grid-cols-1 xl:grid-cols-3 gap-3'>
        <div className='flex flex-col gap-2 rounded-md border border-solid border-semi-color-border p-3'>
          <Title heading={6}>{t('认证')}</Title>
          <Text type='tertiary'>{t('所有 /v1/assets 接口都只接受资源管理中心生成的 ak_ 开头 Key。')}</Text>
          <DocsTable
            columns={[t('位置'), t('名称'), t('必填'), t('说明')]}
            rows={ASSET_API_AUTH_ROWS}
          />
          <CodeBlock>{`Authorization: Bearer ak_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`}</CodeBlock>
        </div>

        <div className='flex flex-col gap-2 rounded-md border border-solid border-semi-color-border p-3'>
          <Title heading={6}>{t('状态码')}</Title>
          <DocsTable
            columns={[t('状态码'), t('说明')]}
            rows={ASSET_API_STATUS_ROWS}
          />
        </div>

        <div className='flex flex-col gap-2 rounded-md border border-solid border-semi-color-border p-3'>
          <Title heading={6}>{t('错误格式')}</Title>
          <DocsTable
            columns={[t('错误码'), t('说明')]}
            rows={ASSET_API_ERROR_CODE_ROWS}
          />
          <CodeBlock>{`{
  "error": {
    "message": "资源 API Key 已禁用 (request id: ...)",
    "type": "new_api_error",
    "code": "access_denied"
  }
}`}</CodeBlock>
        </div>
      </div>

      <div className='flex flex-col gap-2 rounded-md border border-solid border-semi-color-border p-3'>
        <div className='flex flex-col md:flex-row md:items-center md:justify-between gap-2'>
          <div>
            <Title heading={6}>{t('OpenAPI 文档')}</Title>
            <Text type='tertiary'>
              {t('已按 OpenAPI 3.0.3 生成完整 spec，可导入 Apifox、Postman、Swagger Editor 或其它文档工具。')}
            </Text>
          </div>
          <Space wrap>
            <Tag color='blue'>OpenAPI 3.0.3</Tag>
            <Tag>{t('{{count}} 个接口', { count: ASSET_API_ENDPOINTS.length })}</Tag>
          </Space>
        </div>
        <DocsTable
          columns={[t('内容'), t('说明')]}
          rows={[
            ['securitySchemes', 'Bearer 资源 API Key，格式 ak_xxx'],
            ['schemas', 'Asset、AssetListResponse、AssetQueryRequest、AssetURLListResponse、ErrorResponse'],
            ['responses', '200、400、401、403、404、500 的 JSON/CSV 响应结构'],
          ]}
        />
        <Space wrap>
          <Button icon={<Eye size={14} />} onClick={() => setShowOpenAPISheet(true)}>
            {t('查看 JSON')}
          </Button>
          <Button icon={<Copy size={14} />} onClick={copyAssetOpenAPIJSON}>
            {t('复制 JSON')}
          </Button>
          <Button icon={<Download size={14} />} onClick={downloadAssetOpenAPIJSON}>
            {t('下载 JSON')}
          </Button>
        </Space>
      </div>

      <div className='flex flex-col gap-2 rounded-md border border-solid border-semi-color-border p-3'>
        <Title heading={6}>{t('Asset JSON 响应字段')}</Title>
        <DocsTable
          columns={[t('字段'), t('类型'), t('说明')]}
          rows={ASSET_API_FIELD_ROWS}
        />
      </div>

      {ASSET_API_ENDPOINTS.map((endpoint) => (
        <div key={`${endpoint.method}-${endpoint.path}`} className='flex flex-col gap-3 rounded-md border border-solid border-semi-color-border p-3'>
          <div className='flex flex-col md:flex-row md:items-center md:justify-between gap-2'>
            <div className='flex items-center gap-2 flex-wrap'>
              <MethodTag method={endpoint.method} />
              <Text code>{endpoint.path}</Text>
              <Title heading={6} className='!mb-0'>
                {t(endpoint.title)}
              </Title>
            </div>
          </div>
          <Text type='tertiary'>{t(endpoint.description)}</Text>

          {endpoint.pathParams && (
            <div className='flex flex-col gap-2'>
              <Text strong>{t('路径参数')}</Text>
              <DocsTable
                columns={[t('名称'), t('类型'), t('必填'), t('说明')]}
                rows={endpoint.pathParams}
              />
            </div>
          )}

          {endpoint.query && (
            <div className='flex flex-col gap-2'>
              <Text strong>{t('Query 参数')}</Text>
              <DocsTable
                columns={[t('名称'), t('类型'), t('必填'), t('说明')]}
                rows={endpoint.query}
              />
            </div>
          )}

          {endpoint.body && (
            <div className='flex flex-col gap-2'>
              <Text strong>{t('请求 Body')}</Text>
              <DocsTable
                columns={[t('字段'), t('类型'), t('必填'), t('说明')]}
                rows={endpoint.body}
              />
            </div>
          )}

          {endpoint.responseFields && (
            <div className='flex flex-col gap-2'>
              <Text strong>{endpoint.path === '/v1/assets/export' ? t('响应 CSV 字段') : t('响应字段')}</Text>
              <DocsTable
                columns={[t('字段'), t('类型'), t('说明')]}
                rows={endpoint.responseFields}
              />
            </div>
          )}

          <div className='grid grid-cols-1 xl:grid-cols-2 gap-3'>
            <div className='flex flex-col gap-2'>
              <Text strong>{t('请求示例')}</Text>
              <CodeBlock>{endpoint.curl}</CodeBlock>
            </div>
            <div className='flex flex-col gap-2'>
              <Text strong>{endpoint.path === '/v1/assets/export' ? t('响应示例（CSV）') : t('响应示例')}</Text>
              <CodeBlock>{endpoint.response}</CodeBlock>
            </div>
          </div>
        </div>
      ))}
    </div>
  );

  return (
    <div className='p-2'>
      <div className='flex flex-col gap-3'>
        <div>
          <Title heading={4} className='!mb-1'>
            {t('资源管理中心')}
          </Title>
          <Text type='tertiary'>
            {t('管理异步任务生成的图片、视频和资源访问 Key')}
          </Text>
        </div>
        <Tabs
          type='line'
          activeKey={activeMainTab}
          onChange={setActiveMainTab}
          tabList={[
            { tab: t('资源列表'), itemKey: 'assets' },
            { tab: t('API Key'), itemKey: 'keys' },
            { tab: t('使用文档'), itemKey: 'docs' },
          ]}
        />
        {activeMainTab === 'assets' && renderAssetsTab()}
        {activeMainTab === 'keys' && renderKeysTab()}
        {activeMainTab === 'docs' && renderDocsTab()}
      </div>

      <SideSheet
        placement='right'
        title={t('资源详情')}
        visible={!!detailAsset}
        onCancel={() => setDetailAsset(null)}
        width={560}
        footer={null}
      >
        {detailAsset && (
          <div className='flex flex-col gap-3'>
            <div className='w-full aspect-video rounded-lg overflow-hidden border border-solid border-semi-color-border'>
              {detailAsset.asset_type === 'image' ? (
                <button
                  type='button'
                  className='w-full h-full border-0 p-0 cursor-pointer bg-transparent'
                  onClick={() => setPreviewImage(detailAsset.url)}
                >
                  <AssetPreview asset={detailAsset} />
                </button>
              ) : (
                <AssetPreview asset={detailAsset} />
              )}
            </div>
            <Space wrap>
              <Button icon={<Copy size={14} />} onClick={() => copy(detailAsset.url).then((ok) => ok && showSuccess(t('已复制链接')))}>
                {t('复制链接')}
              </Button>
              <Button icon={<ExternalLink size={14} />} onClick={() => window.open(detailAsset.url, '_blank', 'noreferrer')}>
                {t('打开资源')}
              </Button>
              <Button icon={<Download size={14} />} onClick={() => triggerDownload(detailAsset.url, buildDownloadName(detailAsset))}>
                {t('下载')}
              </Button>
            </Space>
            {[
              [t('资源 ID'), detailAsset.asset_id],
              [t('任务 ID'), detailAsset.task_id],
              [t('类型'), assetTypeLabel(detailAsset.asset_type, t)],
              [t('状态'), detailAsset.status],
              [t('模型'), detailAsset.model || '-'],
              [t('平台'), detailAsset.platform || '-'],
              [t('渠道 ID'), detailAsset.channel_id || '-'],
              [t('生成时间'), timestamp2string(detailAsset.created_at)],
              [t('URL'), detailAsset.url],
            ].map(([label, value]) => (
              <div key={label} className='flex gap-3 border-0 border-b border-solid border-semi-color-border pb-2'>
                <Text type='tertiary' className='w-24 shrink-0'>
                  {label}
                </Text>
                <Text copyable={label === t('URL') || label === t('任务 ID')} ellipsis={{ showTooltip: true }}>
                  {value}
                </Text>
              </div>
            ))}
            {detailAsset.metadata && Object.keys(detailAsset.metadata).length > 0 && (
              <pre className='text-xs p-3 rounded-md bg-semi-color-fill-0 overflow-auto'>
                {JSON.stringify(detailAsset.metadata, null, 2)}
              </pre>
            )}
          </div>
        )}
      </SideSheet>

      <SideSheet
        placement='right'
        title={t('OpenAPI JSON')}
        visible={showOpenAPISheet}
        onCancel={() => setShowOpenAPISheet(false)}
        width={720}
        footer={
          <Space>
            <Button onClick={() => setShowOpenAPISheet(false)}>{t('关闭')}</Button>
            <Button icon={<Copy size={14} />} onClick={copyAssetOpenAPIJSON}>
              {t('复制')}
            </Button>
            <Button type='primary' icon={<Download size={14} />} onClick={downloadAssetOpenAPIJSON}>
              {t('下载')}
            </Button>
          </Space>
        }
      >
        <CodeBlock className='max-h-[calc(100vh-180px)] overflow-auto'>{getAssetOpenAPIJSON()}</CodeBlock>
      </SideSheet>

      <SideSheet
        placement='right'
        title={t('创建资源 API Key')}
        visible={showCreateKeySheet}
        onCancel={() => setShowCreateKeySheet(false)}
        width={520}
        footer={
          <Space>
            <Button onClick={() => setShowCreateKeySheet(false)}>{t('取消')}</Button>
            <Button type='primary' loading={createKeyLoading} onClick={createAssetKey}>
              {t('创建')}
            </Button>
          </Space>
        }
      >
        <Form
          getFormApi={setKeyFormApi}
          initValues={{ name: '', expired_at: -1, allow_ips: '' }}
          layout='vertical'
          allowEmpty
          autoComplete='off'
        >
          <Form.Input field='name' label={t('名称')} placeholder={t('例如：本地下载脚本')} showClear />
          <Form.DatePicker
            field='expired_at'
            label={t('过期时间')}
            type='dateTime'
            placeholder={t('留空或设为永不过期')}
            showClear
            className='w-full'
          />
          <Form.Slot label={t('快捷设置')}>
            <Space wrap>
              <Button type='tertiary' onClick={() => keyFormApi?.setValue('expired_at', -1)}>
                {t('永不过期')}
              </Button>
              <Button type='tertiary' onClick={() => keyFormApi?.setValue('expired_at', timestamp2string(Date.now() / 1000 + 86400 * 30))}>
                {t('一个月')}
              </Button>
              <Button type='tertiary' onClick={() => keyFormApi?.setValue('expired_at', timestamp2string(Date.now() / 1000 + 86400))}>
                {t('一天')}
              </Button>
            </Space>
          </Form.Slot>
          <Form.TextArea
            field='allow_ips'
            label={t('允许 IP')}
            placeholder={t('每行一个 IP 或 CIDR，留空表示不限制')}
            autosize
            showClear
          />
        </Form>
      </SideSheet>

      <ImagePreview
        src={previewImage}
        visible={!!previewImage}
        onVisibleChange={(visible) => {
          if (!visible) setPreviewImage('');
        }}
      />
    </div>
  );
}
