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
const modelTokenSecurity = [{ ModelTokenAuth: [] }];
const resourceSecurity = [{ ResourceCenterAuth: [] }];
const webhookSecurity = [{ WebhookAuth: [] }];

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

const videoWebhookSucceededExample = {
  id: 'evt_video_xxx',
  object: 'event',
  api_version: '2026-07-17',
  type: 'video.task.succeeded',
  created_at: 1784250060,
  data: {
    object: {
      id: 'task_video_xxx',
      object: 'video.task',
      model: 'grok-imagine-video-1.5',
      operation: 'generation',
      status: 'succeeded',
      progress: 100,
      result: {
        videos: [
          {
            asset_id: 'asset_video_xxx',
            index: 0,
            url: 'https://cdn.example.com/video.mp4',
            mime_type: 'video/mp4',
            duration_ms: 5000,
            temporary: true,
            url_auth: 'none',
          },
        ],
      },
      error: null,
      client_reference_id: 'order_video_123',
      metadata: {},
      created_at: 1784250000,
      started_at: 1784250002,
      completed_at: 1784250060,
      updated_at: 1784250060,
    },
  },
};

const videoWebhookFailedExample = {
  id: 'evt_video_yyy',
  object: 'event',
  api_version: '2026-07-17',
  type: 'video.task.failed',
  created_at: 1784250060,
  data: {
    object: {
      id: 'task_video_yyy',
      object: 'video.task',
      model: 'video-model',
      operation: 'extension',
      status: 'failed',
      progress: 100,
      result: null,
      error: {
        code: 'video_task_failed',
        message: 'Video task failed',
        retryable: false,
      },
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

const videoTaskListParameters = [
  queryParameter(
    'status',
    ref('VideoTaskStatus'),
    'Filter by one public task status.',
  ),
  queryParameter(
    'operation',
    ref('VideoTaskOperation'),
    'Filter by generation, edit, extension, or remix.',
  ),
  queryParameter(
    'client_reference_id',
    { type: 'string' },
    'Filter by the caller business reference.',
  ),
  queryParameter(
    'created_after',
    { type: 'integer', format: 'int64', minimum: 0 },
    'Inclusive lower creation-time bound in Unix seconds.',
  ),
  queryParameter(
    'created_before',
    { type: 'integer', format: 'int64', minimum: 0 },
    'Inclusive upper creation-time bound in Unix seconds.',
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
    version: '2026-07-23',
    summary:
      'Assets, asynchronous image and video tasks, uploads, and Webhooks.',
    description:
      'Public Resource Center API. Creating asynchronous image or video tasks uses a standard API Token; task queries, uploads, assets, and proxied video downloads use a Resource Center API Key; outbound Webhook verification uses a dedicated Webhook Key.',
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
      name: 'Async Videos',
      description:
        'Create and query provider-neutral video generation, edit, extension, and remix tasks.',
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
    '/v1/assets/{asset_id}/content': {
      get: {
        tags: ['Assets'],
        operationId: 'downloadVideoAsset',
        summary: 'Download video asset content',
        description:
          'Streams a temporary upstream-backed video. Range requests are supported. A 410 response means the upstream resource expired. Public CDN URLs may be returned directly by Asset and task responses instead of using this proxy.',
        security: [{ ResourceCenterAuth: [] }, { ModelTokenAuth: [] }],
        parameters: [
          pathParameter('asset_id', 'Video Asset ID such as asset_xxx.'),
          {
            name: 'Range',
            in: 'header',
            required: false,
            description: 'Optional RFC 9110 byte range.',
            schema: { type: 'string', examples: ['bytes=0-1048575'] },
          },
        ],
        responses: {
          200: {
            description: 'Complete video content.',
            headers: {
              'Accept-Ranges': {
                schema: { type: 'string', const: 'bytes' },
              },
              'Cache-Control': { schema: { type: 'string' } },
            },
            content: {
              'video/mp4': { schema: { type: 'string', format: 'binary' } },
              'application/octet-stream': {
                schema: { type: 'string', format: 'binary' },
              },
            },
          },
          206: {
            description: 'Partial video content.',
            headers: {
              'Content-Range': { schema: { type: 'string' } },
              'Accept-Ranges': {
                schema: { type: 'string', const: 'bytes' },
              },
            },
            content: {
              'video/mp4': { schema: { type: 'string', format: 'binary' } },
              'application/octet-stream': {
                schema: { type: 'string', format: 'binary' },
              },
            },
          },
          404: responseRef('NotFound'),
          410: responseRef('ResourceExpired'),
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
              'CSV with asset_id, task_id, asset_type, url, filename, model, and created_at.',
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
        security: modelTokenSecurity,
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
    '/v1/video/tasks': {
      post: {
        tags: ['Async Videos'],
        operationId: 'createVideoTask',
        summary: 'Create an asynchronous video task',
        description:
          'Creates a normalized generation, edit, extension, or remix task. Provider capability and parameter limits are validated by the selected provider adaptor. Compatibility endpoints under /v1/videos remain unchanged.',
        security: modelTokenSecurity,
        parameters: [
          {
            name: 'Idempotency-Key',
            in: 'header',
            required: false,
            description:
              'Optional caller key, unique per user. Maximum 128 characters. The same key and request replay the original task; a different request returns 409.',
            schema: { type: 'string', maxLength: 128 },
          },
        ],
        requestBody: {
          required: true,
          content: jsonContent(ref('VideoTaskCreateRequest')),
        },
        responses: {
          202: jsonResponse('Task accepted.', ref('VideoTask'), {
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
          502: responseRef('BadGateway'),
          ...commonErrorResponses,
        },
      },
      get: {
        tags: ['Async Videos'],
        operationId: 'listVideoTasks',
        summary: 'List asynchronous video tasks',
        security: resourceSecurity,
        parameters: videoTaskListParameters,
        responses: {
          200: jsonResponse(
            'Cursor-paginated task list.',
            ref('VideoTaskListResponse'),
          ),
          ...commonErrorResponses,
        },
      },
    },
    '/v1/video/tasks/{task_id}': {
      get: {
        tags: ['Async Videos'],
        operationId: 'getVideoTask',
        summary: 'Get an asynchronous video task',
        security: resourceSecurity,
        parameters: [
          pathParameter('task_id', 'Public task ID such as task_xxx.'),
        ],
        responses: {
          200: jsonResponse('Task.', ref('VideoTask')),
          404: responseRef('NotFound'),
          ...commonErrorResponses,
        },
      },
    },
    '/v1/video/tasks/query': {
      post: {
        tags: ['Async Videos'],
        operationId: 'queryVideoTasks',
        summary: 'Query up to 100 video tasks',
        description:
          'Returned tasks preserve request order. Unknown or unauthorized IDs are listed in missing.',
        security: resourceSecurity,
        requestBody: {
          required: true,
          content: jsonContent(ref('VideoTaskBatchQueryRequest')),
        },
        responses: {
          200: jsonResponse(
            'Ordered task list with missing IDs.',
            ref('VideoTaskBatchResponse'),
          ),
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
          'Sent with Authorization: Bearer wk-.... The Webhook Key is generated with the account Webhook configuration and has no API permissions. Any 2xx response succeeds and its body is ignored. Network errors and non-2xx responses are retried using the administrator-configured fixed interval and maximum attempts (defaults: 3 total attempts, 30 seconds).',
        security: webhookSecurity,
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
          'Sent with Authorization: Bearer wk-.... The Webhook Key is generated with the account Webhook configuration and has no API permissions. Any 2xx response succeeds and its body is ignored. Network errors and non-2xx responses are retried using the administrator-configured fixed interval and maximum attempts (defaults: 3 total attempts, 30 seconds).',
        security: webhookSecurity,
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
    videoTaskSucceeded: {
      post: {
        operationId: 'receiveVideoTaskSucceededWebhook',
        summary: 'video.task.succeeded callback',
        description:
          'Sent for every normalized video operation with Authorization: Bearer wk-.... data.object is identical to GET /v1/video/tasks/{task_id}. Video URLs are temporary and may require an ak_ Resource Center API Key when url_auth is resource_api_key.',
        security: webhookSecurity,
        parameters: [],
        requestBody: {
          required: true,
          content: {
            'application/json': {
              schema: ref('WebhookEvent'),
              example: videoWebhookSucceededExample,
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
    videoTaskFailed: {
      post: {
        operationId: 'receiveVideoTaskFailedWebhook',
        summary: 'video.task.failed callback',
        description:
          'Sent for failed generation, edit, extension, or remix tasks with Authorization: Bearer wk-.... No Asset is required for a failure event.',
        security: webhookSecurity,
        parameters: [],
        requestBody: {
          required: true,
          content: {
            'application/json': {
              schema: ref('WebhookEvent'),
              example: videoWebhookFailedExample,
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
      ModelTokenAuth: {
        type: 'http',
        scheme: 'bearer',
        bearerFormat: 'sk-...',
        description:
          'A standard API Token from Token Management. Its model limits, group, quota, and audit identity apply when creating asynchronous image or video tasks. It may also download an owned proxied video Asset.',
      },
      ResourceCenterAuth: {
        type: 'http',
        scheme: 'bearer',
        bearerFormat: 'ak_...',
        description:
          'A Resource Center API Key generated on the Resource Center API Key tab. It authorizes image/video task queries, uploads, asset reads, and proxied video downloads, but cannot create tasks or authenticate Webhooks.',
      },
      WebhookAuth: {
        type: 'http',
        scheme: 'bearer',
        bearerFormat: 'wk-...',
        description:
          'A dedicated Webhook Key generated with the account Webhook configuration. Receivers use it only to verify outbound callbacks; it grants no API access.',
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
          url: { type: 'string', format: 'uri-reference' },
          temporary: { type: 'boolean' },
          url_auth: {
            type: 'string',
            enum: ['none', 'resource_api_key'],
          },
          thumbnail_url: { type: 'string', format: 'uri' },
          mime_type: { type: 'string' },
          filename: { type: 'string' },
          size_bytes: { type: 'integer', format: 'int64', minimum: 0 },
          width: { type: 'integer', minimum: 0 },
          height: { type: 'integer', minimum: 0 },
          duration_ms: { type: 'integer', format: 'int64', minimum: 0 },
          model: { type: 'string' },
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
          url: { type: 'string', format: 'uri-reference' },
          temporary: { type: 'boolean' },
          url_auth: {
            type: 'string',
            enum: ['none', 'resource_api_key'],
          },
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
      VideoTaskOperation: {
        type: 'string',
        enum: ['generation', 'edit', 'extension', 'remix'],
      },
      VideoTaskStatus: {
        type: 'string',
        enum: ['queued', 'in_progress', 'succeeded', 'failed'],
      },
      VideoTaskSource: {
        oneOf: [
          {
            type: 'object',
            additionalProperties: false,
            required: ['url'],
            properties: {
              url: {
                type: 'string',
                format: 'uri',
                pattern: '^(https?://|data:)',
                description:
                  'A public HTTP(S) URL or a provider-supported data URL. Asset URLs that require ak_ authentication are not guaranteed to be readable by an upstream provider.',
              },
            },
          },
          {
            type: 'object',
            additionalProperties: false,
            required: ['provider', 'file_id'],
            properties: {
              provider: { type: 'string', minLength: 1 },
              file_id: { type: 'string', minLength: 1 },
            },
          },
        ],
      },
      VideoTaskInput: {
        type: 'object',
        additionalProperties: false,
        required: ['prompt'],
        properties: {
          prompt: { type: 'string', minLength: 1 },
          image: {
            ...ref('VideoTaskSource'),
            description:
              'One primary image source. For xAI generation this is the starting frame. Support in other operations is provider-specific.',
          },
          reference_images: {
            type: 'array',
            minItems: 1,
            items: ref('VideoTaskSource'),
            description:
              'Multiple reference image sources. Allowed combinations and provider-specific limits are validated by the selected adaptor.',
          },
          video: {
            ...ref('VideoTaskSource'),
            description:
              'One source video for edit, extension, or remix operations.',
          },
        },
      },
      VideoTaskOutput: {
        type: 'object',
        additionalProperties: false,
        properties: {
          duration: {
            type: 'integer',
            description:
              'Requested output duration. For extension this is the new segment duration. Provider-specific bounds are validated by the selected adaptor.',
          },
          aspect_ratio: { type: 'string' },
          resolution: { type: 'string' },
        },
      },
      VideoTaskCreateRequest: {
        type: 'object',
        additionalProperties: false,
        required: ['model', 'operation', 'input'],
        properties: {
          model: { type: 'string', minLength: 1 },
          operation: ref('VideoTaskOperation'),
          input: ref('VideoTaskInput'),
          output: ref('VideoTaskOutput'),
          client_reference_id: { type: 'string', maxLength: 191 },
          metadata: { type: 'object', additionalProperties: true },
          provider_options: {
            type: 'object',
            description:
              'Provider-specific options keyed by provider namespace, for example provider_options.xai.',
            additionalProperties: {
              type: 'object',
              additionalProperties: true,
            },
          },
        },
        allOf: [
          {
            if: {
              properties: { operation: { const: 'generation' } },
              required: ['operation'],
            },
            then: {
              properties: { input: { not: { required: ['video'] } } },
            },
            else: {
              properties: {
                input: {
                  required: ['video'],
                },
              },
            },
          },
        ],
      },
      VideoTaskResultVideo: {
        type: 'object',
        additionalProperties: false,
        required: ['asset_id', 'index', 'url', 'temporary', 'url_auth'],
        properties: {
          asset_id: { type: 'string', pattern: '^asset_' },
          index: { type: 'integer', minimum: 0 },
          url: { type: 'string', format: 'uri-reference' },
          mime_type: { type: 'string' },
          filename: { type: 'string' },
          width: { type: 'integer', minimum: 0 },
          height: { type: 'integer', minimum: 0 },
          duration_ms: { type: 'integer', format: 'int64', minimum: 0 },
          temporary: { const: true },
          url_auth: {
            type: 'string',
            enum: ['none', 'resource_api_key'],
          },
        },
      },
      VideoTaskResult: {
        type: 'object',
        additionalProperties: false,
        required: ['videos'],
        properties: {
          videos: { type: 'array', items: ref('VideoTaskResultVideo') },
        },
      },
      VideoTaskError: {
        type: 'object',
        additionalProperties: false,
        required: ['code', 'message', 'retryable'],
        properties: {
          code: { type: 'string' },
          message: { type: 'string' },
          retryable: { type: 'boolean' },
        },
      },
      VideoTask: {
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
          'error',
          'created_at',
          'started_at',
          'completed_at',
          'updated_at',
        ],
        properties: {
          id: { type: 'string', pattern: '^task_' },
          object: { const: 'video.task' },
          model: { type: 'string' },
          operation: ref('VideoTaskOperation'),
          status: ref('VideoTaskStatus'),
          progress: { type: 'integer', minimum: 0, maximum: 100 },
          result: { oneOf: [ref('VideoTaskResult'), { type: 'null' }] },
          error: { oneOf: [ref('VideoTaskError'), { type: 'null' }] },
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
      VideoTaskListResponse: {
        type: 'object',
        additionalProperties: false,
        required: ['object', 'data', 'has_more'],
        properties: {
          object: { const: 'list' },
          data: { type: 'array', items: ref('VideoTask') },
          first_id: { type: 'string' },
          last_id: { type: 'string' },
          has_more: { type: 'boolean' },
        },
      },
      VideoTaskBatchQueryRequest: {
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
      VideoTaskBatchResponse: {
        type: 'object',
        additionalProperties: false,
        required: ['object', 'data'],
        properties: {
          object: { const: 'list' },
          data: { type: 'array', items: ref('VideoTask') },
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
        enum: [
          'image.task.succeeded',
          'image.task.failed',
          'video.task.succeeded',
          'video.task.failed',
        ],
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
            properties: {
              object: { oneOf: [ref('ImageTask'), ref('VideoTask')] },
            },
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
      ResourceExpired: {
        description: 'The temporary upstream video resource has expired.',
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
