import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const ref = (name) => ({ $ref: `#/components/schemas/${name}` });
const responseRef = (name) => ({ $ref: `#/components/responses/${name}` });
const jsonContent = (schema) => ({
  'application/json': { schema },
});
const jsonResponse = (description, schema, headers) => ({
  description,
  ...(headers ? { headers } : {}),
  content: jsonContent(schema),
});
const pathParameter = (name, description) => ({
  name,
  in: 'path',
  required: true,
  description,
  schema: { type: 'string' },
});
const queryParameter = (name, schema, description) => ({
  name,
  in: 'query',
  required: false,
  description,
  schema,
});
const resourceSecurity = [{ ResourceCenterAuth: [] }];

const webhookSucceededExample = {
  id: 'evt_xxx',
  object: 'event',
  api_version: '2026-07-17',
  type: 'image.task.succeeded',
  created_at: 1784250060,
  data: {
    object: {
      id: 'task_xxx',
      object: 'image.task',
      model: 'gpt-image-2',
      operation: 'generation',
      status: 'succeeded',
      progress: 100,
      result: {
        images: [
          {
            asset_id: 'asset_xxx',
            url: 'https://cdn.example.com/image.png',
            mime_type: 'image/png',
            format: 'png',
            width: 1024,
            height: 1024,
            size_bytes: 245760,
            filename: 'image.png',
          },
        ],
      },
      usage: {},
      error: null,
      client_reference_id: 'order_123',
      metadata: {},
      created_at: 1784250000,
      started_at: 1784250002,
      completed_at: 1784250060,
      updated_at: 1784250060,
    },
  },
};

const webhookFailedExample = {
  id: 'evt_yyy',
  object: 'event',
  api_version: '2026-07-17',
  type: 'image.task.failed',
  created_at: 1784250060,
  data: {
    object: {
      id: 'task_yyy',
      object: 'image.task',
      model: 'gpt-image-2',
      operation: 'edit',
      status: 'failed',
      progress: 100,
      result: null,
      usage: {},
      error: {
        code: 'upstream_error',
        message: 'Image generation failed',
        retryable: false,
      },
      client_reference_id: 'order_456',
      metadata: {},
      created_at: 1784250000,
      started_at: 1784250002,
      completed_at: 1784250060,
      updated_at: 1784250060,
    },
  },
};

const commonErrorResponses = {
  400: responseRef('BadRequest'),
  401: responseRef('Unauthorized'),
  403: responseRef('Forbidden'),
  500: responseRef('ServerError'),
};

const assetFilterParameters = [
  queryParameter('asset_type', ref('AssetType'), 'Filter by asset type.'),
  queryParameter(
    'status',
    { type: 'string', enum: ['available'] },
    'Public APIs return available assets.',
  ),
  queryParameter('task_id', { type: 'string' }, 'Filter by exact task ID.'),
  queryParameter('model', { type: 'string' }, 'Filter by exact model.'),
  queryParameter(
    'platform',
    { type: 'string' },
    'Filter by internal task platform name.',
  ),
  queryParameter('action', { type: 'string' }, 'Filter by task action.'),
  queryParameter('channel_id', { type: 'integer' }, 'Filter by channel ID.'),
  queryParameter(
    'keyword',
    { type: 'string' },
    'Search asset ID, task ID, filename, or URL.',
  ),
  queryParameter(
    'start_timestamp',
    { type: 'integer', format: 'int64', minimum: 0 },
    'Minimum creation time in Unix seconds.',
  ),
  queryParameter(
    'end_timestamp',
    { type: 'integer', format: 'int64', minimum: 0 },
    'Maximum creation time in Unix seconds.',
  ),
];

const imageTaskListParameters = [
  queryParameter(
    'status',
    ref('ImageTaskStatus'),
    'Filter by one public task status.',
  ),
  queryParameter(
    'operation',
    ref('ImageTaskOperation'),
    'Filter by generation or edit.',
  ),
  queryParameter(
    'client_reference_id',
    { type: 'string' },
    'Filter by the caller business reference.',
  ),
  queryParameter(
    'created_after',
    { type: 'integer', format: 'int64', minimum: 0 },
    'Exclusive lower creation-time bound in Unix seconds.',
  ),
  queryParameter(
    'created_before',
    { type: 'integer', format: 'int64', minimum: 0 },
    'Exclusive upper creation-time bound in Unix seconds.',
  ),
  queryParameter(
    'after',
    { type: 'string' },
    'Task ID cursor returned by the previous page.',
  ),
  queryParameter(
    'limit',
    { type: 'integer', minimum: 1, maximum: 100, default: 20 },
    'Page size.',
  ),
];

const spec = {
  openapi: '3.1.0',
  info: {
    title: 'new-api Resource Center API',
    version: '2026-07-17',
    summary: 'Assets, asynchronous image tasks, uploads, and Webhooks.',
    description:
      'Public Resource Center API. Async image tasks, existing video task APIs, uploads, assets, and outbound Webhook verification use the same Resource Center API Key. This document focuses on normalized image and asset operations.',
  },
  jsonSchemaDialect: 'https://json-schema.org/draft/2020-12/schema',
  servers: [
    {
      url: '{base_url}',
      variables: {
        base_url: {
          default: 'https://new-api.example.com',
          description: 'Your new-api origin.',
        },
      },
    },
  ],
  tags: [
    {
      name: 'Assets',
      description: 'Read generated assets with a Resource Center API Key.',
    },
    {
      name: 'Async Images',
      description: 'Create and query durable image generation/edit tasks.',
    },
    {
      name: 'Image Uploads',
      description: 'Upload temporary URL inputs before creating an edit task.',
    },
  ],
  paths: {
    '/v1/assets': {
      get: {
        tags: ['Assets'],
        operationId: 'listAssets',
        summary: 'List assets',
        description: 'Lists only assets owned by the API token user.',
        security: resourceSecurity,
        parameters: [
          ...assetFilterParameters,
          queryParameter(
            'page',
            { type: 'integer', minimum: 1, default: 1 },
            'Page number. Alias: p.',
          ),
          queryParameter(
            'page_size',
            { type: 'integer', minimum: 1, maximum: 100, default: 20 },
            'Page size. Aliases: ps and size.',
          ),
        ],
        responses: {
          200: jsonResponse('Asset list.', ref('AssetListResponse')),
          ...commonErrorResponses,
        },
      },
    },
    '/v1/assets/{asset_id}': {
      get: {
        tags: ['Assets'],
        operationId: 'getAsset',
        summary: 'Get an asset',
        security: resourceSecurity,
        parameters: [pathParameter('asset_id', 'Asset ID such as asset_xxx.')],
        responses: {
          200: jsonResponse('Asset.', ref('Asset')),
          404: responseRef('NotFound'),
          ...commonErrorResponses,
        },
      },
    },
    '/v1/assets/query': {
      post: {
        tags: ['Assets'],
        operationId: 'queryAssets',
        summary: 'Query assets in a JSON body',
        security: resourceSecurity,
        requestBody: {
          required: true,
          content: jsonContent(ref('AssetQueryRequest')),
        },
        responses: {
          200: jsonResponse('Asset list.', ref('AssetListResponse')),
          ...commonErrorResponses,
        },
      },
    },
    '/v1/assets/batch/urls': {
      post: {
        tags: ['Assets'],
        operationId: 'getAssetURLs',
        summary: 'Get asset URLs in bulk',
        security: resourceSecurity,
        requestBody: {
          required: true,
          content: jsonContent(ref('AssetBatchURLRequest')),
        },
        responses: {
          200: jsonResponse('Asset URL list.', ref('AssetURLListResponse')),
          ...commonErrorResponses,
        },
      },
    },
    '/v1/assets/export': {
      get: {
        tags: ['Assets'],
        operationId: 'exportAssets',
        summary: 'Export asset URLs as CSV',
        description: 'Exports at most 10,000 matching assets.',
        security: resourceSecurity,
        parameters: assetFilterParameters,
        responses: {
          200: {
            description:
              'CSV with asset_id, task_id, asset_type, url, filename, model, platform, action, and created_at.',
            content: { 'text/csv': { schema: { type: 'string' } } },
          },
          401: responseRef('Unauthorized'),
          403: responseRef('Forbidden'),
          500: responseRef('ServerError'),
        },
      },
    },
    '/v1/image/tasks': {
      post: {
        tags: ['Async Images'],
        operationId: 'createImageTask',
        summary: 'Create an asynchronous image task',
        description:
          'Accepts the normalized JSON contract for generation or edit tasks, or synchronous-style multipart local files for edit tasks. Returns immediately after the task, billing reservation, credential lease, and durable dispatch are stored. Reusing an Idempotency-Key with the same normalized fields and file contents returns the original task.',
        security: resourceSecurity,
        parameters: [
          {
            name: 'Idempotency-Key',
            in: 'header',
            required: false,
            description:
              'Optional caller key, unique per user. Maximum 128 characters.',
            schema: { type: 'string', maxLength: 128 },
          },
        ],
        requestBody: {
          required: true,
          content: {
            ...jsonContent(ref('ImageTaskCreateRequest')),
            'multipart/form-data': {
              schema: ref('ImageTaskMultipartCreateRequest'),
              encoding: { image: { style: 'form', explode: true } },
            },
          },
        },
        responses: {
          202: jsonResponse('Task accepted.', ref('ImageTask'), {
            Location: {
              description: 'Canonical task URL.',
              schema: { type: 'string' },
            },
            'Retry-After': {
              description: 'Suggested polling delay in seconds.',
              schema: { type: 'integer' },
            },
            'Idempotent-Replayed': {
              description: 'true when the original task is replayed.',
              schema: { type: 'boolean' },
            },
          }),
          409: responseRef('Conflict'),
          413: responseRef('PayloadTooLarge'),
          502: responseRef('BadGateway'),
          503: responseRef('ServiceUnavailable'),
          ...commonErrorResponses,
        },
      },
      get: {
        tags: ['Async Images'],
        operationId: 'listImageTasks',
        summary: 'List asynchronous image tasks',
        security: resourceSecurity,
        parameters: imageTaskListParameters,
        responses: {
          200: jsonResponse(
            'Cursor-paginated task list.',
            ref('ImageTaskListResponse'),
          ),
          ...commonErrorResponses,
        },
      },
    },
    '/v1/image/tasks/{task_id}': {
      get: {
        tags: ['Async Images'],
        operationId: 'getImageTask',
        summary: 'Get an asynchronous image task',
        security: resourceSecurity,
        parameters: [
          pathParameter('task_id', 'Public task ID such as task_xxx.'),
        ],
        responses: {
          200: jsonResponse('Task.', ref('ImageTask')),
          404: responseRef('NotFound'),
          ...commonErrorResponses,
        },
      },
    },
    '/v1/image/tasks/query': {
      post: {
        tags: ['Async Images'],
        operationId: 'queryImageTasks',
        summary: 'Query up to 100 image tasks',
        description:
          'Returned tasks preserve request order. Unknown or unauthorized IDs are listed in missing.',
        security: resourceSecurity,
        requestBody: {
          required: true,
          content: jsonContent(ref('ImageTaskBatchQueryRequest')),
        },
        responses: {
          200: jsonResponse(
            'Ordered task list with missing IDs.',
            ref('ImageTaskBatchResponse'),
          ),
          ...commonErrorResponses,
        },
      },
    },
    '/v1/image/uploads': {
      post: {
        tags: ['Image Uploads'],
        operationId: 'uploadImageInputs',
        summary: 'Upload image inputs as multipart data',
        description:
          'Accepts up to 10 repeated image files and one mask. Each PNG, JPEG, or WebP file is limited to 20 MiB and the request to 100 MiB. Temporary URLs should be submitted to a task promptly.',
        security: resourceSecurity,
        requestBody: {
          required: true,
          content: {
            'multipart/form-data': {
              schema: ref('ImageMultipartUploadRequest'),
              encoding: { image: { style: 'form', explode: true } },
            },
          },
        },
        responses: {
          200: jsonResponse(
            'Temporary uploaded URLs.',
            ref('ImageUploadListResponse'),
          ),
          413: responseRef('PayloadTooLarge'),
          502: responseRef('BadGateway'),
          503: responseRef('ServiceUnavailable'),
          ...commonErrorResponses,
        },
      },
    },
    '/v1/image/uploads/base64': {
      post: {
        tags: ['Image Uploads'],
        operationId: 'uploadBase64ImageInputs',
        summary: 'Upload base64 image inputs',
        description:
          'Accepts plain base64 or data URLs. Use images plus optional mask, or uploads with an optional field value of image or mask.',
        security: resourceSecurity,
        requestBody: {
          required: true,
          content: jsonContent(ref('ImageBase64UploadRequest')),
        },
        responses: {
          200: jsonResponse(
            'Temporary uploaded URLs.',
            ref('ImageUploadListResponse'),
          ),
          413: responseRef('PayloadTooLarge'),
          502: responseRef('BadGateway'),
          503: responseRef('ServiceUnavailable'),
          ...commonErrorResponses,
        },
      },
    },
  },
  webhooks: {
    imageTaskSucceeded: {
      post: {
        operationId: 'receiveImageTaskSucceededWebhook',
        summary: 'image.task.succeeded callback',
        description:
          'Sent with Authorization: Bearer ak_.... This is the same Resource Center API Key used to create and query tasks. Any 2xx response succeeds and its body is ignored. Network errors and non-2xx responses are retried using the administrator-configured fixed interval and maximum attempts (defaults: 3 total attempts, 30 seconds).',
        security: resourceSecurity,
        parameters: [],
        requestBody: {
          required: true,
          content: {
            'application/json': {
              schema: ref('WebhookEvent'),
              example: webhookSucceededExample,
            },
          },
        },
        responses: {
          '2XX': {
            description: 'Delivery accepted. The response body is ignored.',
          },
          default: {
            description:
              'Delivery failed and is retried until the configured maximum attempt count is reached.',
          },
        },
      },
    },
    imageTaskFailed: {
      post: {
        operationId: 'receiveImageTaskFailedWebhook',
        summary: 'image.task.failed callback',
        description:
          'Sent with Authorization: Bearer ak_.... This is the same Resource Center API Key used to create and query tasks. Any 2xx response succeeds and its body is ignored. Network errors and non-2xx responses are retried using the administrator-configured fixed interval and maximum attempts (defaults: 3 total attempts, 30 seconds).',
        security: resourceSecurity,
        parameters: [],
        requestBody: {
          required: true,
          content: {
            'application/json': {
              schema: ref('WebhookEvent'),
              example: webhookFailedExample,
            },
          },
        },
        responses: {
          '2XX': {
            description: 'Delivery accepted. The response body is ignored.',
          },
          default: {
            description:
              'Delivery failed and is retried until the configured maximum attempt count is reached.',
          },
        },
      },
    },
  },
  components: {
    securitySchemes: {
      ResourceCenterAuth: {
        type: 'http',
        scheme: 'bearer',
        bearerFormat: 'ak_...',
        description:
          'A Resource Center API Key generated on the Resource Center API Key tab. The same key authorizes async image and video tasks, uploads, asset reads, and outbound Webhook verification.',
      },
    },
    schemas: {
      Error: {
        type: 'object',
        additionalProperties: false,
        required: ['type', 'code', 'message'],
        properties: {
          type: { type: 'string', example: 'invalid_request_error' },
          code: { type: 'string', example: 'invalid_request' },
          message: { type: 'string' },
          param: { type: 'string' },
          request_id: { type: 'string' },
        },
      },
      ErrorResponse: {
        type: 'object',
        additionalProperties: false,
        required: ['error'],
        properties: { error: ref('Error') },
      },
      AssetType: { type: 'string', enum: ['image', 'video', 'audio', 'file'] },
      Asset: {
        type: 'object',
        additionalProperties: false,
        required: [
          'object',
          'id',
          'task_id',
          'index',
          'type',
          'url',
          'status',
          'created_at',
          'updated_at',
        ],
        properties: {
          object: { const: 'asset' },
          id: { type: 'string', examples: ['asset_xxx'] },
          task_id: { type: 'string', examples: ['task_xxx'] },
          index: { type: 'integer', minimum: 0 },
          type: ref('AssetType'),
          url: { type: 'string', format: 'uri' },
          thumbnail_url: { type: 'string', format: 'uri' },
          mime_type: { type: 'string' },
          filename: { type: 'string' },
          size_bytes: { type: 'integer', format: 'int64', minimum: 0 },
          width: { type: 'integer', minimum: 0 },
          height: { type: 'integer', minimum: 0 },
          duration_ms: { type: 'integer', format: 'int64', minimum: 0 },
          model: { type: 'string' },
          platform: { type: 'string' },
          action: { type: 'string' },
          status: { const: 'available' },
          metadata: { type: 'object', additionalProperties: true },
          created_at: { type: 'integer', format: 'int64', minimum: 0 },
          updated_at: { type: 'integer', format: 'int64', minimum: 0 },
        },
      },
      AssetListResponse: {
        type: 'object',
        additionalProperties: false,
        required: ['object', 'data', 'page', 'page_size', 'total', 'has_more'],
        properties: {
          object: { const: 'list' },
          data: { type: 'array', items: ref('Asset') },
          page: { type: 'integer', minimum: 1 },
          page_size: { type: 'integer', minimum: 1, maximum: 100 },
          total: { type: 'integer', format: 'int64', minimum: 0 },
          has_more: { type: 'boolean' },
        },
      },
      AssetQueryRequest: {
        type: 'object',
        additionalProperties: false,
        properties: {
          asset_ids: {
            type: 'array',
            maxItems: 100,
            items: { type: 'string' },
          },
          asset_type: ref('AssetType'),
          task_id: { type: 'string' },
          model: { type: 'string' },
          platform: { type: 'string' },
          action: { type: 'string' },
          start_timestamp: { type: 'integer', format: 'int64', minimum: 0 },
          end_timestamp: { type: 'integer', format: 'int64', minimum: 0 },
          page: { type: 'integer', minimum: 1, default: 1 },
          page_size: { type: 'integer', minimum: 1, maximum: 100, default: 20 },
        },
      },
      AssetBatchURLRequest: {
        type: 'object',
        additionalProperties: false,
        required: ['asset_ids'],
        properties: {
          asset_ids: {
            type: 'array',
            minItems: 1,
            maxItems: 100,
            items: { type: 'string' },
          },
        },
      },
      AssetURLItem: {
        type: 'object',
        additionalProperties: false,
        required: ['asset_id', 'task_id', 'asset_type', 'url'],
        properties: {
          asset_id: { type: 'string' },
          task_id: { type: 'string' },
          asset_type: ref('AssetType'),
          url: { type: 'string', format: 'uri' },
        },
      },
      AssetURLListResponse: {
        type: 'object',
        additionalProperties: false,
        required: ['object', 'data'],
        properties: {
          object: { const: 'list' },
          data: { type: 'array', items: ref('AssetURLItem') },
        },
      },
      ImageTaskOperation: { type: 'string', enum: ['generation', 'edit'] },
      ImageTaskStatus: {
        type: 'string',
        enum: ['queued', 'in_progress', 'succeeded', 'failed'],
      },
      ImageTaskSource: {
        type: 'object',
        additionalProperties: false,
        required: ['url'],
        properties: {
          url: { type: 'string', format: 'uri', pattern: '^https?://' },
        },
      },
      ImageTaskInput: {
        type: 'object',
        additionalProperties: false,
        required: ['prompt'],
        properties: {
          prompt: { type: 'string', minLength: 1 },
          images: { type: 'array', minItems: 1, items: ref('ImageTaskSource') },
          mask: ref('ImageTaskSource'),
        },
      },
      ImageTaskOutput: {
        type: 'object',
        additionalProperties: false,
        properties: {
          count: { type: 'integer', minimum: 1, maximum: 10 },
          size: { type: 'string', examples: ['1024x1024'] },
          quality: { type: 'string', examples: ['high'] },
          format: { type: 'string', examples: ['png'] },
          compression: { type: 'integer', minimum: 0, maximum: 100 },
          background: { type: 'string', examples: ['auto'] },
        },
      },
      ImageTaskCreateRequest: {
        type: 'object',
        additionalProperties: false,
        required: ['model', 'operation', 'input'],
        properties: {
          model: { type: 'string', minLength: 1 },
          operation: ref('ImageTaskOperation'),
          input: ref('ImageTaskInput'),
          output: ref('ImageTaskOutput'),
          client_reference_id: { type: 'string', maxLength: 191 },
          metadata: { type: 'object', additionalProperties: true },
        },
        allOf: [
          {
            if: {
              properties: { operation: { const: 'edit' } },
              required: ['operation'],
            },
            then: {
              properties: {
                input: {
                  type: 'object',
                  properties: {
                    images: {
                      type: 'array',
                      minItems: 1,
                      items: ref('ImageTaskSource'),
                    },
                  },
                  required: ['images'],
                },
              },
            },
          },
        ],
      },
      ImageTaskMultipartCreateRequest: {
        type: 'object',
        additionalProperties: false,
        required: ['model', 'prompt', 'image'],
        properties: {
          model: { type: 'string', minLength: 1 },
          operation: {
            type: 'string',
            const: 'edit',
            default: 'edit',
            description:
              'Optional. Multipart requests always create edit tasks.',
          },
          prompt: { type: 'string', minLength: 1 },
          image: {
            type: 'array',
            minItems: 1,
            maxItems: 10,
            items: { type: 'string', format: 'binary' },
          },
          mask: { type: 'string', format: 'binary' },
          n: { type: 'integer', minimum: 1, maximum: 10 },
          size: { type: 'string', examples: ['1024x1024'] },
          quality: { type: 'string', examples: ['high'] },
          output_format: { type: 'string', examples: ['png'] },
          output_compression: { type: 'integer', minimum: 0, maximum: 100 },
          background: { type: 'string', examples: ['auto'] },
          client_reference_id: { type: 'string', maxLength: 191 },
          metadata: {
            type: 'string',
            description: 'A JSON-encoded object.',
            examples: ['{"order_id":"order_123"}'],
          },
        },
      },
      ImageTaskResultImage: {
        type: 'object',
        additionalProperties: false,
        required: ['asset_id', 'url'],
        properties: {
          asset_id: { type: 'string' },
          url: { type: 'string', format: 'uri' },
          mime_type: { type: 'string' },
          format: { type: 'string' },
          width: { type: 'integer', minimum: 0 },
          height: { type: 'integer', minimum: 0 },
          size_bytes: { type: 'integer', format: 'int64', minimum: 0 },
          filename: { type: 'string' },
          revised_prompt: { type: 'string' },
        },
      },
      ImageTaskResult: {
        type: 'object',
        additionalProperties: false,
        required: ['images'],
        properties: {
          images: { type: 'array', items: ref('ImageTaskResultImage') },
          output: { type: 'object', additionalProperties: true },
        },
      },
      ImageTaskError: {
        type: 'object',
        additionalProperties: false,
        required: ['code', 'message', 'retryable'],
        properties: {
          code: { type: 'string' },
          message: { type: 'string' },
          retryable: { type: 'boolean' },
        },
      },
      ImageTask: {
        type: 'object',
        additionalProperties: false,
        required: [
          'id',
          'object',
          'model',
          'operation',
          'status',
          'progress',
          'result',
          'usage',
          'error',
          'created_at',
          'started_at',
          'completed_at',
          'updated_at',
        ],
        properties: {
          id: { type: 'string', pattern: '^task_' },
          object: { const: 'image.task' },
          model: { type: 'string' },
          operation: ref('ImageTaskOperation'),
          status: ref('ImageTaskStatus'),
          progress: { type: 'integer', minimum: 0, maximum: 100 },
          result: { oneOf: [ref('ImageTaskResult'), { type: 'null' }] },
          usage: { type: 'object', additionalProperties: true },
          error: { oneOf: [ref('ImageTaskError'), { type: 'null' }] },
          client_reference_id: { type: 'string' },
          metadata: { type: 'object', additionalProperties: true },
          created_at: { type: 'integer', format: 'int64', minimum: 0 },
          started_at: {
            type: ['integer', 'null'],
            format: 'int64',
            minimum: 0,
          },
          completed_at: {
            type: ['integer', 'null'],
            format: 'int64',
            minimum: 0,
          },
          updated_at: { type: 'integer', format: 'int64', minimum: 0 },
        },
      },
      ImageTaskListResponse: {
        type: 'object',
        additionalProperties: false,
        required: ['object', 'data', 'has_more'],
        properties: {
          object: { const: 'list' },
          data: { type: 'array', items: ref('ImageTask') },
          first_id: { type: 'string' },
          last_id: { type: 'string' },
          has_more: { type: 'boolean' },
        },
      },
      ImageTaskBatchQueryRequest: {
        type: 'object',
        additionalProperties: false,
        required: ['task_ids'],
        properties: {
          task_ids: {
            type: 'array',
            minItems: 1,
            maxItems: 100,
            items: { type: 'string' },
          },
        },
      },
      ImageTaskBatchResponse: {
        type: 'object',
        additionalProperties: false,
        required: ['object', 'data'],
        properties: {
          object: { const: 'list' },
          data: { type: 'array', items: ref('ImageTask') },
          missing: { type: 'array', items: { type: 'string' } },
        },
      },
      ImageMultipartUploadRequest: {
        type: 'object',
        additionalProperties: false,
        anyOf: [{ required: ['image'] }, { required: ['mask'] }],
        properties: {
          image: {
            type: 'array',
            maxItems: 10,
            items: { type: 'string', format: 'binary' },
          },
          mask: { type: 'string', format: 'binary' },
        },
      },
      ImageBase64UploadItem: {
        oneOf: [
          { type: 'string', description: 'Plain base64 or an image data URL.' },
          {
            type: 'object',
            additionalProperties: false,
            properties: {
              field: { type: 'string', enum: ['image', 'mask'] },
              b64_json: { type: 'string' },
              base64: { type: 'string' },
              data: { type: 'string' },
            },
            anyOf: [
              {
                properties: { b64_json: { type: 'string' } },
                required: ['b64_json'],
              },
              {
                properties: { base64: { type: 'string' } },
                required: ['base64'],
              },
              {
                properties: { data: { type: 'string' } },
                required: ['data'],
              },
            ],
          },
        ],
      },
      ImageBase64UploadRequest: {
        type: 'object',
        additionalProperties: false,
        properties: {
          uploads: {
            type: 'array',
            minItems: 1,
            maxItems: 11,
            items: ref('ImageBase64UploadItem'),
          },
          images: {
            type: 'array',
            minItems: 1,
            maxItems: 10,
            items: ref('ImageBase64UploadItem'),
          },
          mask: ref('ImageBase64UploadItem'),
        },
        anyOf: [
          { required: ['uploads'] },
          { required: ['images'] },
          { required: ['mask'] },
        ],
      },
      ImageUpload: {
        type: 'object',
        additionalProperties: false,
        required: ['id', 'object', 'field', 'url', 'temporary'],
        properties: {
          id: { type: 'string' },
          object: { const: 'image.upload' },
          field: { type: 'string', enum: ['image', 'mask'] },
          url: { type: 'string', format: 'uri' },
          mime_type: { type: 'string' },
          size_bytes: { type: 'integer', format: 'int64', minimum: 0 },
          width: { type: 'integer', minimum: 0 },
          height: { type: 'integer', minimum: 0 },
          format: { type: 'string' },
          temporary: { const: true },
        },
      },
      ImageUploadListResponse: {
        type: 'object',
        additionalProperties: false,
        required: ['object', 'data', 'images', 'mask'],
        properties: {
          object: { const: 'image.upload.list' },
          data: { type: 'array', items: ref('ImageUpload') },
          images: { type: 'array', items: { type: 'string', format: 'uri' } },
          mask: { type: ['string', 'null'], format: 'uri' },
        },
      },
      WebhookEventType: {
        type: 'string',
        enum: ['image.task.succeeded', 'image.task.failed'],
      },
      WebhookEvent: {
        type: 'object',
        additionalProperties: false,
        required: ['id', 'object', 'api_version', 'type', 'created_at', 'data'],
        properties: {
          id: { type: 'string', pattern: '^evt_' },
          object: { const: 'event' },
          api_version: { const: '2026-07-17' },
          type: ref('WebhookEventType'),
          created_at: { type: 'integer', format: 'int64' },
          data: {
            type: 'object',
            additionalProperties: false,
            required: ['object'],
            properties: { object: ref('ImageTask') },
          },
        },
      },
    },
    responses: {
      BadRequest: {
        description: 'Invalid request.',
        content: jsonContent(ref('ErrorResponse')),
      },
      Unauthorized: {
        description: 'Missing or invalid credential.',
        content: jsonContent(ref('ErrorResponse')),
      },
      Forbidden: {
        description: 'API Key, IP, expiry, or user policy denied access.',
        content: jsonContent(ref('ErrorResponse')),
      },
      NotFound: {
        description: 'The resource was not found for the authenticated user.',
        content: jsonContent(ref('ErrorResponse')),
      },
      Conflict: {
        description: 'Idempotency conflict or endpoint limit.',
        content: jsonContent(ref('ErrorResponse')),
      },
      PayloadTooLarge: {
        description: 'Upload request or file is too large.',
        content: jsonContent(ref('ErrorResponse')),
      },
      BadGateway: {
        description: 'The internal upload service failed.',
        content: jsonContent(ref('ErrorResponse')),
      },
      ServiceUnavailable: {
        description: 'The internal upload service is not configured.',
        content: jsonContent(ref('ErrorResponse')),
      },
      ServerError: {
        description: 'Internal server error.',
        content: jsonContent(ref('ErrorResponse')),
      },
    },
  },
};

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const outputPath = path.resolve(
  scriptDirectory,
  '../../docs/openapi/resource-center.json',
);
const output = `${JSON.stringify(spec, null, 2)}\n`;

if (process.argv.includes('--check')) {
  const current = fs.existsSync(outputPath)
    ? fs.readFileSync(outputPath, 'utf8')
    : '';
  if (current !== output) {
    console.error(
      'docs/openapi/resource-center.json is out of date. Run bun run openapi:generate.',
    );
    process.exit(1);
  }
} else {
  fs.mkdirSync(path.dirname(outputPath), { recursive: true });
  fs.writeFileSync(outputPath, output);
}
