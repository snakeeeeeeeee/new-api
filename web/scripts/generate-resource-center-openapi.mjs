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

const adobeVideoModelCapabilities = {
  'adobe-kling-3.0-{720p|1080p}': {
    prompt_max_characters: 1200,
    durations: {
      minimum: 3,
      maximum: 15,
      integer_seconds: true,
    },
    aspect_ratios: ['16:9', '9:16'],
    reference_modes: {
      frame: {
        minimum_images: 1,
        maximum_images: 2,
      },
    },
    generate_audio_option: 'output.generate_audio',
  },
  'adobe-kling-3.0-omni-{720p|1080p}': {
    prompt_max_characters: 1200,
    durations: {
      minimum: 3,
      maximum: 15,
      integer_seconds: true,
    },
    aspect_ratios: ['16:9', '9:16'],
    reference_modes: {
      frame: {
        maximum_images: 2,
      },
      images: {
        maximum_images: 3,
      },
    },
    generate_audio_option: 'output.generate_audio',
  },
  'adobe-veo-3.1-standard-{720p|1080p}': {
    prompt_max_characters: 1200,
    durations: [4, 6, 8],
    aspect_ratios: ['16:9', '9:16'],
    reference_modes: {
      frame: {
        maximum_images: 2,
      },
      images: {
        maximum_images: 3,
        duration: 8,
        aspect_ratio: '16:9',
      },
    },
    generate_audio_option: 'output.generate_audio',
  },
  'adobe-veo-3.1-fast-{720p|1080p}': {
    prompt_max_characters: 1200,
    durations: [4, 6, 8],
    aspect_ratios: ['16:9', '9:16'],
    reference_modes: {
      frame: {
        maximum_images: 2,
      },
    },
    generate_audio_option: 'output.generate_audio',
  },
};

const leonardoVideoModelCapabilities = Object.fromEntries(
  [
    'leonardo-seedance-2.0-fast-480p',
    'leonardo-seedance-2.0-fast-720p',
    'leonardo-seedance-2.0-480p',
    'leonardo-seedance-2.0-720p',
    'leonardo-seedance-2.0-1080p',
  ].map((model) => [
    model,
    {
      channel: 'Self-Leonardo',
      upstream_model: model.replace(/^leonardo-/, ''),
      operations: ['generation'],
      prompt_max_characters: 1200,
      durations: {
        minimum: 4,
        maximum: 15,
        integer_seconds: true,
      },
      aspect_ratios: ['21:9', '16:9', '4:3', '1:1', '3:4', '9:16'],
      reference_modes: {
        media: {
          maximum_images: 4,
          maximum_videos: 3,
          maximum_total: 7,
        },
      },
      audio_reference: false,
      generate_audio_option: 'output.generate_audio',
    },
  ]),
);

const videoTaskCreateExamples = {
  klingFrame: {
    summary: 'Kling 3.0 required frame image',
    value: {
      model: 'adobe-kling-3.0-720p',
      operation: 'generation',
      input: {
        prompt: 'A red sports car driving along a coastal road at sunrise',
        reference_mode: 'frame',
        image: {
          url: 'https://media.example.com/kling/start.jpg',
          name: 'start',
        },
      },
      output: {
        duration: 3,
        aspect_ratio: '16:9',
        generate_audio: true,
      },
    },
  },
  klingOmniImages: {
    summary: 'Kling 3.0 Omni with three image references',
    value: {
      model: 'adobe-kling-3.0-omni-720p',
      operation: 'generation',
      input: {
        prompt:
          'Use @character, @wardrobe, and @location in one cinematic shot',
        reference_mode: 'images',
        reference_images: [
          {
            url: 'https://media.example.com/kling/character.jpg',
            name: 'character',
          },
          {
            url: 'https://media.example.com/kling/wardrobe.jpg',
            name: 'wardrobe',
          },
          {
            url: 'https://media.example.com/kling/location.jpg',
            name: 'location',
          },
        ],
      },
      output: {
        duration: 5,
        aspect_ratio: '9:16',
        generate_audio: false,
      },
    },
  },
  veoStandardImages: {
    summary: 'Veo 3.1 Standard images mode',
    description:
      'Veo Standard images mode requires exactly the 8-second, 16:9 output combination.',
    value: {
      model: 'adobe-veo-3.1-standard-720p',
      operation: 'generation',
      input: {
        prompt:
          'Create a cinematic product shot using @product, @environment, and @lighting',
        reference_mode: 'images',
        reference_images: [
          {
            url: 'https://media.example.com/veo/product.jpg',
            name: 'product',
          },
          {
            url: 'https://media.example.com/veo/environment.jpg',
            name: 'environment',
          },
          {
            url: 'https://media.example.com/veo/lighting.jpg',
            name: 'lighting',
          },
        ],
      },
      output: {
        duration: 8,
        aspect_ratio: '16:9',
      },
    },
  },
  veoFastFrame: {
    summary: 'Veo 3.1 Fast frame-mode generation',
    value: {
      model: 'adobe-veo-3.1-fast-720p',
      operation: 'generation',
      input: {
        prompt: 'Morning clouds flowing through a mountain valley',
        reference_mode: 'frame',
      },
      output: {
        duration: 4,
        aspect_ratio: '16:9',
        generate_audio: true,
      },
    },
  },
  leonardoText: {
    summary: 'Leonardo Seedance 2.0 Fast text generation',
    description:
      'Leonardo accepts generation only. Pure text may omit input.reference_mode; 4-15 second output uses the model-fixed resolution.',
    value: {
      model: 'leonardo-seedance-2.0-fast-480p',
      operation: 'generation',
      input: {
        prompt: 'A cat walking across a moonlit rooftop',
      },
      output: {
        duration: 4,
        aspect_ratio: '16:9',
        generate_audio: false,
      },
    },
  },
  leonardoMedia: {
    summary: 'Leonardo Seedance with image and video references',
    description:
      'Leonardo requires media when references are present, accepts up to four images and three videos, and does not accept audio references.',
    value: {
      model: 'leonardo-seedance-2.0-720p',
      operation: 'generation',
      input: {
        prompt: 'Keep the subject appearance and follow the reference motion',
        reference_mode: 'media',
        image: {
          url: 'https://media.example.com/leonardo/subject.png',
        },
        reference_images: [
          { url: 'https://media.example.com/leonardo/style.jpg' },
        ],
        reference_videos: [
          { url: 'https://media.example.com/leonardo/motion.mp4' },
        ],
      },
      output: {
        duration: 4,
        aspect_ratio: '16:9',
        generate_audio: true,
      },
    },
  },
};

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
      model: 'gemini-3.1-flash-image',
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
      usage: {
        prompt_tokens: 25,
        completion_tokens: 1680,
        total_tokens: 1705,
        input_tokens: 25,
        output_tokens: 1680,
        completion_tokens_details: {
          reasoning_tokens: 0,
        },
      },
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
        code: '524',
        message:
          'Image generation service is temporarily unavailable. Please try again later.',
        retryable: true,
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
      model: 'adobe-veo-3.1-fast-720p',
      operation: 'generation',
      status: 'succeeded',
      progress: 100,
      result: {
        videos: [
          {
            asset_id: 'asset_video_xxx',
            index: 0,
            url: 'https://adobe-cdn.example.com/video.mp4?X-Amz-Signature=...',
            mime_type: 'video/mp4',
            duration_ms: 4000,
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
      model: 'adobe-kling-3.0-720p',
      operation: 'generation',
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
    version: '2026-07-28',
    summary:
      'Provider-neutral assets, asynchronous image and video tasks, uploads, and Webhooks.',
    description:
      'Public Resource Center API shared by GPT-Image, Gemini Images, and video providers. Creating asynchronous image or video tasks uses a standard API Token; task queries, uploads, assets, and proxied video downloads use a Resource Center API Key; outbound Webhook verification uses a dedicated Webhook Key. Provider differences are expressed only through model capabilities and provider_options.',
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
    {
      name: 'Media Uploads',
      description:
        'Create presigned direct-upload sessions for video reference media.',
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
          'Accepts the normalized JSON contract for generation or edit tasks, or synchronous-style multipart local files for edit tasks. Returns immediately after the task, billing reservation, credential lease, and durable dispatch are stored. Reusing an Idempotency-Key with the same normalized fields and file contents returns the original task. To retry a terminal failed task, submit a new request with a new Idempotency-Key.',
        security: modelTokenSecurity,
        parameters: [
          {
            name: 'Idempotency-Key',
            in: 'header',
            required: false,
            description:
              'Optional caller key, unique per user. Maximum 128 characters. Replaying it returns the original task, including a terminal failed task; use a new key for a new attempt after failure.',
            schema: { type: 'string', maxLength: 128 },
          },
        ],
        requestBody: {
          required: true,
          content: {
            'application/json': {
              schema: ref('ImageTaskCreateRequest'),
              examples: {
                gptImageGeneration: {
                  summary: 'GPT-Image generation',
                  value: {
                    model: 'gpt-image-2',
                    operation: 'generation',
                    input: {
                      prompt: 'A product photo on a white background',
                    },
                    output: {
                      count: 1,
                      size: '1024x1024',
                      quality: 'high',
                      format: 'png',
                    },
                  },
                },
                geminiGeneration: {
                  summary: 'Gemini Images generation',
                  value: {
                    model: 'gemini-3.1-flash-image',
                    operation: 'generation',
                    input: {
                      prompt: 'A product photo on a white background',
                    },
                    output: {
                      count: 1,
                      aspect_ratio: '16:9',
                      resolution: '4K',
                      quality: 'auto',
                      format: 'png',
                    },
                    provider_options: {
                      google: {
                        generationConfig: {
                          temperature: 0.8,
                          thinkingConfig: {
                            thinkingLevel: 'HIGH',
                          },
                        },
                      },
                    },
                  },
                },
                geminiEdit: {
                  summary: 'Gemini Images maskless edit',
                  value: {
                    model: 'gemini-3-pro-image-count',
                    operation: 'edit',
                    input: {
                      prompt: 'Replace the background with a studio wall',
                      images: [
                        {
                          url: 'https://cdn.example.com/input.png',
                        },
                      ],
                    },
                    output: {
                      count: 1,
                      aspect_ratio: '1:1',
                      resolution: '2K',
                      quality: 'auto',
                      format: 'png',
                    },
                  },
                },
              },
            },
            'multipart/form-data': {
              schema: ref('ImageTaskMultipartCreateRequest'),
              encoding: { image: { style: 'form', explode: true } },
              examples: {
                gptImageEdit: {
                  summary: 'GPT-Image local-file edit',
                  value: {
                    model: 'gpt-image-2',
                    prompt: 'Replace the background',
                    n: 1,
                    size: '1024x1024',
                    quality: 'high',
                    output_format: 'png',
                  },
                },
                geminiEdit: {
                  summary: 'Gemini maskless local-file edit',
                  value: {
                    model: 'gemini-3.1-flash-image',
                    prompt: 'Replace the background',
                    n: 1,
                    aspect_ratio: '4:3',
                    resolution: '2K',
                    quality: 'auto',
                    output_format: 'png',
                    provider_options:
                      '{"google":{"generationConfig":{"temperature":0.8}}}',
                  },
                },
              },
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
    '/v1/media/uploads': {
      post: {
        tags: ['Media Uploads'],
        operationId: 'createMediaUploadSessions',
        summary: 'Create reference-media upload sessions',
        description:
          'Returns presigned PUT URLs. Upload bytes directly to object storage using the returned method and headers; new-api proxies metadata only and never receives the file body.',
        security: resourceSecurity,
        requestBody: {
          required: true,
          content: jsonContent(ref('MediaUploadCreateRequest')),
        },
        responses: {
          200: jsonResponse(
            'Presigned upload sessions.',
            ref('MediaUploadSessionListResponse'),
          ),
          413: responseRef('PayloadTooLarge'),
          502: responseRef('BadGateway'),
          503: responseRef('ServiceUnavailable'),
          ...commonErrorResponses,
        },
      },
    },
    '/v1/media/uploads/complete': {
      post: {
        tags: ['Media Uploads'],
        operationId: 'completeMediaUploads',
        summary: 'Confirm reference-media uploads',
        description:
          'Verifies each uploaded object with HEAD, including exact size and declared MIME, then returns temporary HTTP(S) URLs for input reference fields.',
        security: resourceSecurity,
        requestBody: {
          required: true,
          content: jsonContent(ref('MediaUploadCompleteRequest')),
        },
        responses: {
          200: jsonResponse(
            'Confirmed temporary media URLs.',
            ref('MediaUploadListResponse'),
          ),
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
          'Creates a normalized generation, edit, extension, or remix task. Provider capability and parameter limits are validated before precharge and upstream submission. output.generate_audio is the provider-neutral generated-audio control and defaults to true on supported providers; provider_options.*.generate_audio and provider_options.*.reference_mode are rejected. Adobe video prompts accept at most 1200 Unicode characters. Adobe Kling/Veo model SKUs fix the output resolution and reject output.resolution. Kling 3.0 uses 3-15 second frame generation and requires at least one frame image. Kling 3.0 Omni additionally accepts up to three images. Veo 3.1 accepts 4, 6, or 8 seconds; Standard images mode requires 8 seconds and 16:9, while Fast is frame-only. Self-Leonardo exposes five leonardo-seedance-2.0-* models, generation only, 4-15 seconds, six aspect ratios, and HTTP(S) media references limited to four images, three videos, and seven total items. Leonardo does not support frame, images, audio references, data URLs, base64, or provider file references. HTTPS is recommended for reference integrity. Compatibility endpoints under /v1/videos remain unchanged.',
        'x-description-zh-CN':
          '创建规范化的视频生成、编辑、扩展或 Remix 任务，并在预扣费及上游提交前完成模型能力校验。output.generate_audio 是供应商无关的生成音轨开关，支持该能力的渠道省略时默认 true；provider_options.*.generate_audio 和 provider_options.*.reference_mode 会被拒绝。Adobe 视频提示词最多 1200 个 Unicode 字符。Adobe Kling/Veo 模型名固定输出分辨率并拒绝 output.resolution。Kling 3.0 使用 3-15 秒 frame 模式且至少需要一张帧图；Kling 3.0 Omni 额外支持最多三图。Veo 3.1 仅支持 4、6、8 秒；Standard 的 images 模式固定为 8 秒、16:9，Fast 仅支持 frame。Self-Leonardo 公开五个 leonardo-seedance-2.0-* 模型，仅支持 generation、4-15 秒、六种画幅，以及最多 4 图、3 视频、7 项总参考的 HTTP(S) media 模式。Leonardo 不支持 frame、images、音频参考、Data URL、Base64 或 provider file，参考素材建议使用 HTTPS 以保证传输完整性。原有 /v1/videos 兼容接口保持不变。',
        'x-supertoken-video-model-capabilities': {
          ...adobeVideoModelCapabilities,
          ...leonardoVideoModelCapabilities,
        },
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
          content: {
            'application/json': {
              schema: ref('VideoTaskCreateRequest'),
              examples: videoTaskCreateExamples,
            },
          },
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
          'Sent for every normalized video operation with Authorization: Bearer wk-.... data.object is identical to GET /v1/video/tasks/{task_id}. Video URLs are temporary. Request url directly without a SuperToken credential when url_auth is none; use an ak_ Resource Center API Key only when url_auth is resource_api_key.',
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
          size: {
            type: 'string',
            examples: ['1024x1024'],
            description:
              'Output size. For Gemini this is a compatibility mapping to aspect ratio and resolution tier, not an exact pixel guarantee.',
            'x-description-zh-CN':
              '输出尺寸；Gemini 中它是宽高比与分辨率档位的兼容映射，不保证精确像素。',
          },
          aspect_ratio: {
            type: 'string',
            enum: [
              '1:1',
              '2:3',
              '3:2',
              '3:4',
              '4:3',
              '4:5',
              '5:4',
              '9:16',
              '16:9',
              '21:9',
            ],
            description:
              'Requested output aspect ratio. For Gemini, omit size when using this field.',
            'x-description-zh-CN':
              '请求的输出宽高比；Gemini 使用此字段时不要同时提交 size。',
          },
          resolution: {
            type: 'string',
            enum: ['512', '0.5K', '1K', '2K', '4K'],
            description:
              'Requested output resolution tier. Gemini Flash supports 512/0.5K/1K/2K/4K; Gemini Pro supports 1K/2K/4K. 0.5K is normalized to 512.',
            'x-description-zh-CN':
              '请求的输出分辨率档位；Gemini Flash 支持 512/0.5K/1K/2K/4K，Gemini Pro 支持 1K/2K/4K；0.5K 会规范化为 512。',
          },
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
          provider_options: {
            ...ref('ImageProviderOptions'),
            description:
              'Provider-specific options. Gemini uses the google namespace.',
          },
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
          size: {
            type: 'string',
            examples: ['1024x1024'],
            description:
              'Output size. For Gemini this is a compatibility mapping and cannot be combined with aspect_ratio or resolution.',
            'x-description-zh-CN':
              '输出尺寸；Gemini 中它是兼容映射，不能与 aspect_ratio 或 resolution 同时提交。',
          },
          aspect_ratio: {
            type: 'string',
            enum: [
              '1:1',
              '2:3',
              '3:2',
              '3:4',
              '4:3',
              '4:5',
              '5:4',
              '9:16',
              '16:9',
              '21:9',
            ],
          },
          resolution: {
            type: 'string',
            enum: ['512', '0.5K', '1K', '2K', '4K'],
          },
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
          provider_options: {
            type: 'string',
            description: 'A JSON-encoded ImageProviderOptions object.',
            examples: ['{"google":{"generationConfig":{"temperature":0.8}}}'],
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
          code: {
            type: 'string',
            description:
              'Stable business error code. "524" means a retryable internal image-service availability failure; it is not an HTTP status code.',
            'x-description-zh-CN':
              '稳定的业务错误码；“524”表示图片服务内部暂时不可用且可以重试，它不是 HTTP 状态码。',
            examples: ['524'],
          },
          message: {
            type: 'string',
            examples: [
              'Image generation service is temporarily unavailable. Please try again later.',
            ],
          },
          retryable: { type: 'boolean' },
        },
      },
      GeminiImageConfig: {
        type: 'object',
        additionalProperties: false,
        properties: {
          aspectRatio: {
            type: 'string',
            enum: [
              '1:1',
              '2:3',
              '3:2',
              '3:4',
              '4:3',
              '4:5',
              '5:4',
              '9:16',
              '16:9',
              '21:9',
            ],
            description:
              'Gemini output aspect ratio. snake_case alias: aspect_ratio.',
            'x-description-zh-CN':
              'Gemini 输出宽高比；也接受 snake_case 别名 aspect_ratio。',
          },
          imageSize: {
            type: 'string',
            enum: ['512', '0.5K', '1K', '2K', '4K'],
            description:
              'Backward-compatible Gemini output resolution tier. 0.5K normalizes to 512; snake_case alias: image_size. Prefer the public resolution field.',
            'x-description-zh-CN':
              '向后兼容的 Gemini 输出分辨率档位；0.5K 会规范化为 512，也接受 snake_case 别名 image_size。新接入优先使用公开 resolution 字段。',
          },
        },
        description: 'Gemini image output configuration.',
        'x-description-zh-CN': 'Gemini 图片输出配置。',
      },
      GeminiThinkingConfig: {
        type: 'object',
        additionalProperties: false,
        properties: {
          thinkingBudget: {
            type: 'integer',
            minimum: 0,
            description:
              'Thinking token budget. snake_case alias: thinking_budget.',
            'x-description-zh-CN':
              '思考 Token 预算；也接受 snake_case 别名 thinking_budget。',
          },
          thinkingLevel: {
            type: 'string',
            description:
              'Provider-supported thinking level. snake_case alias: thinking_level.',
            'x-description-zh-CN':
              '供应商支持的思考等级；也接受 snake_case 别名 thinking_level。',
          },
          includeThoughts: {
            type: 'boolean',
            description:
              'Whether thought metadata is requested. snake_case alias: include_thoughts.',
            'x-description-zh-CN':
              '是否请求思考元数据；也接受 snake_case 别名 include_thoughts。',
          },
        },
        description: 'Gemini thinking configuration.',
        'x-description-zh-CN': 'Gemini 思考配置。',
      },
      GeminiGenerationConfig: {
        type: 'object',
        additionalProperties: false,
        properties: {
          temperature: {
            type: 'number',
            minimum: 0,
          },
          topP: {
            type: 'number',
            minimum: 0,
            maximum: 1,
            description: 'Nucleus sampling threshold. snake_case alias: top_p.',
            'x-description-zh-CN': '核采样阈值；也接受 snake_case 别名 top_p。',
          },
          topK: {
            type: 'integer',
            minimum: 1,
            description: 'Top-k sampling limit. snake_case alias: top_k.',
            'x-description-zh-CN':
              'Top-k 采样上限；也接受 snake_case 别名 top_k。',
          },
          seed: {
            type: 'integer',
          },
          imageConfig: ref('GeminiImageConfig'),
          thinkingConfig: ref('GeminiThinkingConfig'),
        },
        description:
          'Allowlisted Gemini generationConfig. generation_config and nested snake_case aliases are accepted.',
        'x-description-zh-CN':
          'Gemini generationConfig 白名单；同时接受 generation_config 及嵌套 snake_case 别名。',
      },
      GeminiSafetySetting: {
        type: 'object',
        additionalProperties: false,
        required: ['category', 'threshold'],
        properties: {
          category: {
            type: 'string',
          },
          threshold: {
            type: 'string',
          },
        },
        description: 'One Gemini safety policy entry.',
        'x-description-zh-CN': '一项 Gemini 安全策略。',
      },
      GeminiProviderOptions: {
        type: 'object',
        additionalProperties: false,
        properties: {
          generationConfig: ref('GeminiGenerationConfig'),
          safetySettings: {
            type: 'array',
            items: ref('GeminiSafetySetting'),
          },
        },
        description:
          'Gemini-specific options. generation_config and safety_settings aliases are accepted; aliases cannot be supplied together with camelCase forms.',
        'x-description-zh-CN':
          'Gemini 专属选项；接受 generation_config 与 safety_settings 别名，但同一字段不能同时提交两种命名。',
      },
      ImageProviderOptions: {
        type: 'object',
        additionalProperties: false,
        properties: {
          google: ref('GeminiProviderOptions'),
        },
        description:
          'Provider-specific image options. Gemini requests use provider_options.google.',
        'x-description-zh-CN':
          '按供应商命名空间组织的图片选项；Gemini 使用 provider_options.google。',
      },
      ImageUsageTokenDetails: {
        type: 'object',
        additionalProperties: true,
        properties: {
          cached_tokens: { type: 'integer', minimum: 0 },
          text_tokens: { type: 'integer', minimum: 0 },
          audio_tokens: { type: 'integer', minimum: 0 },
          image_tokens: { type: 'integer', minimum: 0 },
          reasoning_tokens: { type: 'integer', minimum: 0 },
        },
        description: 'Provider-normalized token modality details.',
        'x-description-zh-CN': '归一化后的 Token 模态明细。',
      },
      ImageUsage: {
        type: 'object',
        additionalProperties: true,
        properties: {
          prompt_tokens: { type: 'integer', minimum: 0 },
          completion_tokens: { type: 'integer', minimum: 0 },
          total_tokens: { type: 'integer', minimum: 0 },
          input_tokens: { type: 'integer', minimum: 0 },
          output_tokens: { type: 'integer', minimum: 0 },
          cached_tokens: { type: 'integer', minimum: 0 },
          image_tokens: { type: 'integer', minimum: 0 },
          audio_tokens: { type: 'integer', minimum: 0 },
          prompt_tokens_details: ref('ImageUsageTokenDetails'),
          input_tokens_details: ref('ImageUsageTokenDetails'),
          completion_tokens_details: ref('ImageUsageTokenDetails'),
        },
        description:
          'Normalized usage shared by synchronous responses, task queries, consume logs, and Webhooks.',
        'x-description-zh-CN':
          '同步响应、任务查询、消费日志和 Webhook 共用的归一化用量。',
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
          usage: {
            ...ref('ImageUsage'),
            description: 'Provider-normalized usage data when available.',
          },
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
        type: 'object',
        additionalProperties: false,
        required: ['url'],
        properties: {
          url: {
            type: 'string',
            format: 'uri',
            maxLength: 8192,
            pattern: '^https?://',
            description:
              'An absolute HTTP(S) URL. Data URLs, base64 data, provider file IDs, and multipart uploads are rejected by /v1/video/tasks.',
          },
          name: {
            type: 'string',
            maxLength: 100,
            description:
              'Optional reference name, unique across all image, video, and audio references in the request.',
          },
        },
      },
      VideoTaskInputVideoSource: {
        type: 'object',
        additionalProperties: false,
        properties: {
          url: {
            type: 'string',
            pattern: '^(?:https?://|data:)',
            description:
              'An absolute HTTP(S) URL or data URL containing the source video.',
          },
          provider: {
            type: 'string',
            minLength: 1,
            description:
              'Provider namespace for a provider-managed video file.',
          },
          file_id: {
            type: 'string',
            minLength: 1,
            description: 'Provider-managed source video file identifier.',
          },
          name: {
            type: 'string',
            description:
              'Optional source label. It does not participate in reference-media @name mapping.',
          },
        },
        oneOf: [
          {
            required: ['url'],
            not: {
              anyOf: [{ required: ['provider'] }, { required: ['file_id'] }],
            },
          },
          {
            required: ['provider', 'file_id'],
            not: { required: ['url'] },
          },
        ],
      },
      VideoTaskInput: {
        type: 'object',
        additionalProperties: false,
        required: ['prompt'],
        properties: {
          prompt: { type: 'string', minLength: 1 },
          reference_mode: {
            type: 'string',
            enum: ['frame', 'images', 'media'],
            default: 'frame',
            description:
              'frame accepts image references only and maps at most two images to start/end frames. images accepts up to 3 image references. media accepts up to 9 images, 3 videos, 3 audio files, and 12 references total.',
          },
          image: {
            ...ref('VideoTaskSource'),
            description:
              'One primary image source. For xAI generation this is the starting frame. Support in other operations is provider-specific.',
          },
          reference_images: {
            type: 'array',
            minItems: 1,
            maxItems: 9,
            items: ref('VideoTaskSource'),
            description:
              'Ordered reference image sources. input.image counts toward the image limit.',
          },
          reference_videos: {
            type: 'array',
            minItems: 1,
            maxItems: 3,
            items: ref('VideoTaskSource'),
            description:
              'Reference videos for media mode. Each provider validates actual media duration and rejects values over 15 seconds.',
          },
          reference_audios: {
            type: 'array',
            minItems: 1,
            maxItems: 3,
            items: ref('VideoTaskSource'),
            description:
              'Reference audio files for media mode. Each provider validates actual media duration and rejects values over 15 seconds.',
          },
          video: {
            ...ref('VideoTaskInputVideoSource'),
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
          generate_audio: {
            type: 'boolean',
            description:
              'Whether supported providers should generate an output audio track. Defaults to true when omitted. This does not enable audio-reference inputs.',
          },
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
              'Truly provider-specific advanced options keyed by provider namespace. Generated-audio and reference-mode controls must use output.generate_audio and input.reference_mode.',
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
      MediaUploadFileRequest: {
        type: 'object',
        additionalProperties: false,
        required: ['kind', 'mime_type', 'size_bytes'],
        properties: {
          client_id: { type: 'string', maxLength: 120 },
          kind: { type: 'string', enum: ['image', 'video', 'audio'] },
          filename: { type: 'string', maxLength: 255 },
          mime_type: { type: 'string', minLength: 1, maxLength: 100 },
          size_bytes: { type: 'integer', format: 'int64', minimum: 1 },
        },
      },
      MediaUploadCreateRequest: {
        type: 'object',
        additionalProperties: false,
        required: ['files'],
        properties: {
          files: {
            type: 'array',
            minItems: 1,
            maxItems: 12,
            items: ref('MediaUploadFileRequest'),
          },
        },
      },
      MediaUploadSession: {
        type: 'object',
        additionalProperties: false,
        required: [
          'id',
          'kind',
          'method',
          'upload_url',
          'headers',
          'expires_at',
        ],
        properties: {
          id: { type: 'string' },
          client_id: { type: 'string' },
          kind: { type: 'string', enum: ['image', 'video', 'audio'] },
          method: { const: 'PUT' },
          upload_url: { type: 'string', format: 'uri' },
          headers: {
            type: 'object',
            additionalProperties: { type: 'string' },
          },
          expires_at: { type: 'integer', format: 'int64', minimum: 0 },
        },
      },
      MediaUploadSessionListResponse: {
        type: 'object',
        additionalProperties: false,
        required: ['object', 'data'],
        properties: {
          object: { const: 'media.upload.session.list' },
          data: { type: 'array', items: ref('MediaUploadSession') },
        },
      },
      MediaUploadCompleteRequest: {
        type: 'object',
        additionalProperties: false,
        required: ['upload_ids'],
        properties: {
          upload_ids: {
            type: 'array',
            minItems: 1,
            maxItems: 12,
            uniqueItems: true,
            items: { type: 'string' },
          },
        },
      },
      MediaUpload: {
        type: 'object',
        additionalProperties: false,
        required: [
          'id',
          'kind',
          'url',
          'mime_type',
          'size_bytes',
          'temporary',
          'expires_at',
        ],
        properties: {
          id: { type: 'string' },
          client_id: { type: 'string' },
          kind: { type: 'string', enum: ['image', 'video', 'audio'] },
          url: { type: 'string', format: 'uri', pattern: '^https?://' },
          mime_type: { type: 'string' },
          size_bytes: { type: 'integer', format: 'int64', minimum: 1 },
          temporary: { const: true },
          expires_at: { type: 'integer', format: 'int64', minimum: 0 },
        },
      },
      MediaUploadListResponse: {
        type: 'object',
        additionalProperties: false,
        required: ['object', 'data'],
        properties: {
          object: { const: 'media.upload.list' },
          data: { type: 'array', items: ref('MediaUpload') },
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

const defaultPropertyDescriptions = {
  object: 'Stable object discriminator for client-side dispatch.',
  id: 'Public identifier for this object.',
  task_id: 'Public task identifier.',
  asset_id: 'Public Asset identifier.',
  index: 'Zero-based output position within the task.',
  type: 'Public object, Asset, or event type.',
  url: 'URL used to read this input or output.',
  temporary: 'Whether the URL points to a temporary upstream resource.',
  url_auth: 'Authentication required when requesting url.',
  thumbnail_url: 'Optional public thumbnail URL.',
  mime_type: 'IANA media type when known.',
  filename: 'Suggested filename when known.',
  size_bytes: 'File size in bytes when known.',
  width: 'Pixel width when known.',
  height: 'Pixel height when known.',
  duration_ms: 'Video duration in milliseconds when known.',
  model: 'Model requested for the task or associated with the Asset.',
  status: 'Current public status.',
  metadata: 'Caller metadata returned unchanged with the public object.',
  created_at: 'Creation time as Unix seconds.',
  started_at: 'Execution start time as Unix seconds, or null before start.',
  completed_at:
    'Terminal completion time as Unix seconds, or null before completion.',
  updated_at: 'Last update time as Unix seconds.',
  data: 'Objects returned by this list or envelope.',
  page: 'One-based page number.',
  page_size: 'Maximum number of objects returned per page.',
  total: 'Total number of matching objects.',
  has_more: 'Whether another page is available.',
  asset_ids: 'Public Asset IDs to query.',
  asset_type: 'Asset type filter.',
  start_timestamp: 'Inclusive lower creation-time bound in Unix seconds.',
  end_timestamp: 'Inclusive upper creation-time bound in Unix seconds.',
  operation: 'Requested task operation.',
  input: 'Inputs used to execute the task.',
  output: 'Requested output options or provider output metadata.',
  prompt: 'Natural-language instruction for generation or editing.',
  images: 'Image inputs or image outputs, depending on the containing object.',
  image: 'One image input or a repeated multipart image file field.',
  mask: 'Optional mask input.',
  count: 'Requested number of image outputs.',
  n: 'Requested number of image outputs.',
  size: 'Requested image dimensions.',
  quality: 'Requested provider-supported quality level.',
  format: 'Image format when known.',
  output_format: 'Requested output image format.',
  compression: 'Requested output compression from 0 to 100.',
  output_compression: 'Requested output compression from 0 to 100.',
  background: 'Requested provider-supported background mode.',
  client_reference_id: 'Optional caller business identifier for correlation.',
  progress: 'Best-known task progress percentage from 0 to 100.',
  result: 'Terminal task output, or null before success.',
  usage: 'Provider-reported usage data when available.',
  error: 'Terminal failure details, or null when no failure is present.',
  code: 'Stable machine-readable error code.',
  message: 'Human-readable error message.',
  retryable: 'Whether retrying the operation may succeed.',
  param: 'Request field associated with the error when known.',
  request_id: 'Request identifier for support and diagnostics.',
  first_id: 'First task ID in the current page.',
  last_id: 'Last task ID in the current page; use it as the after cursor.',
  task_ids: 'Public task IDs to query in request order.',
  missing: 'Requested IDs that were not found or do not belong to the user.',
  provider: 'Provider namespace for a provider-managed file reference.',
  file_id: 'Provider-managed file identifier.',
  reference_images: 'Reference image sources used to guide generation.',
  reference_mode: 'Reference mapping mode: frame, images, or media.',
  reference_videos: 'Reference video sources used in media mode.',
  reference_audios: 'Reference audio sources used in media mode.',
  video: 'Source video for edit, extension, or remix.',
  duration: 'Requested output or extension duration in seconds.',
  aspect_ratio: 'Requested provider-supported output aspect ratio.',
  resolution: 'Requested provider-supported output resolution.',
  generate_audio:
    'Whether a supported provider should generate an output audio track.',
  provider_options: 'Provider-specific options grouped by provider namespace.',
  videos: 'Ordered video outputs produced by the task.',
  uploads: 'Base64 uploads with an optional image or mask field designation.',
  field: 'Upload role: image or mask.',
  b64_json: 'Image bytes encoded as base64.',
  base64: 'Image bytes encoded as base64.',
  name: 'Optional caller-defined reference name.',
  kind: 'Reference-media kind.',
  client_id: 'Optional caller identifier used to correlate upload items.',
  method: 'HTTP method required for the direct upload.',
  upload_url: 'Short-lived presigned object-storage upload URL.',
  headers: 'Headers that must be sent with the direct upload.',
  expires_at: 'Expiration time in Unix seconds.',
  files:
    'Reference-media files whose direct upload sessions should be created.',
  upload_ids: 'Upload-session IDs to verify and complete.',
  api_version: 'Version of the outbound Webhook payload contract.',
};

const schemaDescriptions = {
  Error: 'Machine-readable error details returned inside an error envelope.',
  ErrorResponse: 'Shared JSON error envelope.',
  Asset: 'A user-visible generated Asset.',
  AssetListResponse: 'Page-based list of generated Assets.',
  AssetQueryRequest:
    'Asset filters. When asset_ids is present, up to 100 IDs are returned in request order and other filters are ignored.',
  AssetBatchURLRequest: 'Up to 100 Asset IDs whose current URLs are needed.',
  AssetURLItem: 'Current public URL and access requirements for one Asset.',
  AssetURLListResponse: 'Current URLs for requested Assets that were found.',
  ImageTaskSource: 'Public HTTP(S) image input.',
  ImageTaskInput: 'Prompt and optional URL-based image inputs.',
  ImageTaskOutput: 'Optional image output controls.',
  ImageTaskCreateRequest:
    'Normalized JSON image task request. Edit requires at least one input.images item.',
  ImageTaskMultipartCreateRequest:
    'Multipart image edit request using local files. image may be repeated up to 10 times.',
  ImageTaskResultImage: 'One generated image and its Asset metadata.',
  ImageTaskResult: 'Successful image task output.',
  ImageTaskError: 'Terminal image task failure.',
  ImageTask: 'Public image task snapshot.',
  ImageTaskListResponse: 'Cursor-paginated image task list.',
  ImageTaskBatchQueryRequest: 'One to 100 image task IDs.',
  ImageTaskBatchResponse:
    'Image tasks in request order plus IDs that were not visible to the user.',
  VideoTaskSource:
    'One URL-only video reference source with an optional unique name.',
  VideoTaskInputVideoSource:
    'One source video for edit, extension, or remix, represented by an HTTP(S)/data URL or a provider-managed file reference.',
  VideoTaskInput:
    'Prompt plus provider-neutral URL-only reference images, videos, and audios. Non-generation operations require a separate input.video source. Self-Leonardo accepts generation-only media references with at most four images, three videos, and seven total items.',
  VideoTaskOutput: 'Optional provider-neutral video output controls.',
  VideoTaskCreateRequest:
    'Normalized video generation, edit, extension, or remix request. Self-Leonardo model mappings accept generation only.',
  VideoTaskResultVideo: 'One temporary video output and its access metadata.',
  VideoTaskResult: 'Successful video task output.',
  VideoTaskError: 'Terminal video task failure.',
  VideoTask: 'Public video task snapshot.',
  VideoTaskListResponse: 'Cursor-paginated video task list.',
  VideoTaskBatchQueryRequest: 'One to 100 video task IDs.',
  VideoTaskBatchResponse:
    'Video tasks in request order plus IDs that were not visible to the user.',
  ImageMultipartUploadRequest:
    'Temporary multipart upload containing up to 10 image files and at most one mask.',
  ImageBase64UploadItem:
    'Plain base64, a data URL, or an object containing b64_json, base64, or data.',
  ImageBase64UploadRequest:
    'Provide uploads, images, or mask. At most 10 image inputs and one mask are accepted.',
  ImageUpload: 'One temporary uploaded image or mask.',
  ImageUploadListResponse:
    'Uploaded objects plus URL arrays ready for an image edit task.',
  MediaUploadFileRequest:
    'Metadata for one image, video, or audio file uploaded directly to object storage.',
  MediaUploadCreateRequest:
    'One to twelve media files whose presigned PUT sessions should be created.',
  MediaUploadSession: 'One short-lived presigned PUT upload session.',
  MediaUploadSessionListResponse:
    'Presigned upload sessions in the same order as the requested files.',
  MediaUploadCompleteRequest:
    'Upload-session IDs whose object size and MIME should be verified.',
  MediaUpload:
    'One confirmed temporary media URL ready for a video task reference.',
  MediaUploadListResponse:
    'Confirmed temporary media URLs in the requested upload ID order.',
  WebhookEvent:
    'Outbound terminal task event. data.object matches the corresponding task query DTO.',
};

const schemaPropertyOverrides = {
  Error: {
    type: 'Error category for broad client handling.',
  },
  Asset: {
    url: 'Current public Asset URL. Adobe tasks may return the original signed HTTPS CDN URL; preserve its query string and follow url_auth instead of routing it through a local content endpoint.',
    temporary:
      'Whether the URL must be treated as temporary. Adobe signed result URLs are not refreshed or archived after expiry.',
    url_auth:
      'Authentication required when requesting url. none means direct access without a SuperToken credential; resource_api_key requires Bearer ak_....',
    status: 'Public Assets returned by these APIs are available.',
  },
  AssetURLItem: {
    url: 'Current public Asset URL. Preserve signed query parameters and follow url_auth.',
    temporary:
      'Whether the URL must be treated as temporary. Expired Adobe result URLs are not refreshed.',
    url_auth:
      'Authentication required when requesting url. none means direct access without a SuperToken credential; resource_api_key requires Bearer ak_....',
  },
  ImageTaskInput: {
    images:
      'URL-based image inputs. Required with at least one item when operation is edit.',
    mask: 'Optional HTTP(S) mask URL used by supported edit models.',
  },
  ImageTaskCreateRequest: {
    metadata: 'Arbitrary JSON object, stored and returned with the task.',
  },
  ImageTaskMultipartCreateRequest: {
    operation: 'Optional; multipart requests always create edit tasks.',
    image: 'Repeatable local PNG, JPEG, or WebP file; one to 10 files.',
    mask: 'Optional single local PNG, JPEG, or WebP mask file.',
    metadata: 'JSON-encoded object in a multipart text field.',
  },
  ImageUploadListResponse: {
    images: 'Temporary image URLs ready for input.images.',
    mask: 'Temporary mask URL ready for input.mask, or null.',
  },
  VideoTaskInput: {
    image: 'One primary image source, commonly used as a starting frame.',
    reference_images:
      'Ordered image references. input.image counts toward the mode-specific image limit.',
    reference_mode:
      'frame accepts at most two images; images accepts at most three images; media accepts 9 images, 3 videos, 3 audio files, and 12 items total for providers that support them. Self-Leonardo allows media only when references are present and limits it to 4 images, 3 videos, and 7 total items.',
    reference_videos:
      'Up to three URL-only video references in media mode; actual duration must not exceed 15 seconds.',
    reference_audios:
      'Up to three URL-only audio references in media mode for providers that support audio references; Self-Leonardo rejects this field.',
    video: 'Required source video for edit, extension, and remix.',
  },
  VideoTaskOutput: {
    duration:
      'Requested duration in seconds; for extension, this is the new segment duration.',
  },
  VideoTaskResultVideo: {
    url: 'Current video URL. Adobe tasks return the original signed HTTPS CDN URL with its complete query string.',
    duration_ms:
      'Provider result duration metadata in milliseconds. Current Adobe normalized tasks echo the requested output.duration and do not download the media body to probe it.',
    temporary:
      'The URL must be treated as temporary. Adobe signed result URLs are not refreshed or archived after expiry.',
    url_auth:
      'Authentication required when requesting url. none means direct access without a SuperToken credential; resource_api_key requires Bearer ak_....',
  },
  WebhookEvent: {
    id: 'Stable event ID; use it to make receiver processing idempotent.',
    type: 'Terminal image or video task event type.',
    data: 'Event data containing the public task snapshot in data.object.',
  },
};

const defaultPropertyDescriptionsZhCN = {
  object: '用于客户端识别对象类型的稳定标识。',
  id: '当前对象的公开 ID。',
  task_id: '公开任务 ID。',
  asset_id: '公开资源 ID。',
  index: '该输出在任务结果中的位置，从 0 开始。',
  type: '当前对象、资源或事件的公开类型。',
  url: '用于读取该输入或输出的 URL。',
  temporary: '该 URL 是否指向上游临时资源。',
  url_auth: '请求 url 时需要使用的鉴权方式。',
  thumbnail_url: '可选的公开缩略图 URL。',
  mime_type: '已知时返回标准 MIME 类型。',
  filename: '已知时返回建议文件名。',
  size_bytes: '已知时返回文件字节数。',
  width: '已知时返回像素宽度。',
  height: '已知时返回像素高度。',
  duration_ms: '已知时返回视频时长，单位为毫秒。',
  model: '任务请求使用的模型，或生成该资源的模型。',
  status: '当前公开状态。',
  metadata: '调用方元数据，会随公开对象原样返回。',
  created_at: '创建时间，Unix 秒。',
  started_at: '执行开始时间，Unix 秒；尚未开始时为 null。',
  completed_at: '任务终态时间，Unix 秒；尚未完成时为 null。',
  updated_at: '最后更新时间，Unix 秒。',
  data: '当前列表或封装对象中包含的数据。',
  page: '页码，从 1 开始。',
  page_size: '每页最多返回的对象数量。',
  total: '符合条件的对象总数。',
  has_more: '是否还有下一页。',
  asset_ids: '要查询的公开资源 ID 列表。',
  asset_type: '资源类型筛选条件。',
  start_timestamp: '创建时间下限，Unix 秒，包含该时间。',
  end_timestamp: '创建时间上限，Unix 秒，包含该时间。',
  operation: '请求执行的任务操作。',
  input: '执行任务所需的输入。',
  output: '请求的输出选项，或供应商返回的输出元数据。',
  prompt: '用于生成或编辑的自然语言指令。',
  images: '图片输入或图片输出，具体含义取决于所在对象。',
  image: '单张图片输入，或可重复提交的 multipart 图片文件字段。',
  mask: '可选的遮罩图片输入。',
  count: '请求生成的图片数量。',
  n: '请求生成的图片数量。',
  size: '请求的图片尺寸。',
  quality: '供应商支持的图片质量级别。',
  format: '已知时返回图片格式。',
  output_format: '请求的输出图片格式。',
  compression: '请求的输出压缩率，取值 0 到 100。',
  output_compression: '请求的输出压缩率，取值 0 到 100。',
  background: '供应商支持的背景模式。',
  client_reference_id: '可选的调用方业务关联 ID。',
  progress: '当前已知的任务进度百分比，取值 0 到 100。',
  result: '任务成功后的输出；成功前为 null。',
  usage: '供应商返回的用量数据，没有时为空对象。',
  error: '任务失败详情；没有失败时为 null。',
  code: '稳定、可供程序判断的错误码。',
  message: '便于阅读的错误说明。',
  retryable: '重试该操作是否可能成功。',
  param: '已知时返回与错误相关的请求字段。',
  request_id: '用于支持和问题排查的请求 ID。',
  first_id: '当前页第一条任务的 ID。',
  last_id: '当前页最后一条任务的 ID，可作为下一页 after 游标。',
  task_ids: '按请求顺序提交的公开任务 ID 列表。',
  missing: '未找到或不属于当前用户的请求 ID。',
  provider: '供应商托管文件所属的命名空间。',
  file_id: '供应商托管文件的 ID。',
  reference_images: '用于指导生成结果的参考图片来源。',
  reference_mode: '参考素材映射模式：frame、images 或 media。',
  reference_videos: 'media 模式使用的参考视频来源。',
  reference_audios: 'media 模式使用的参考音频来源。',
  video: '编辑、扩展或 Remix 使用的源视频。',
  duration: '请求的输出时长或扩展片段时长，单位为秒。',
  aspect_ratio: '供应商支持的输出宽高比。',
  resolution: '供应商支持的输出分辨率。',
  generate_audio: '支持该能力的供应商是否应生成成片音轨。',
  provider_options: '按供应商命名空间组织的专属参数。',
  videos: '任务生成的有序视频输出列表。',
  uploads: 'Base64 上传列表，可指定每项是 image 还是 mask。',
  field: '上传用途：image 或 mask。',
  b64_json: '使用 Base64 编码的图片字节。',
  base64: '使用 Base64 编码的图片字节。',
  name: '可选的调用方参考素材名称。',
  kind: '参考素材类型。',
  client_id: '用于关联上传项的可选调用方标识。',
  method: '直传对象时必须使用的 HTTP 方法。',
  upload_url: '短期有效的对象存储预签名上传 URL。',
  headers: '直传对象时必须携带的 Header。',
  expires_at: '过期时间，Unix 秒。',
  files: '需要创建直传会话的参考素材文件元数据。',
  upload_ids: '需要校验并完成的上传会话 ID。',
  api_version: '出站 Webhook Payload 的协议版本。',
};

const schemaDescriptionsZhCN = {
  Error: '错误响应中的机器可读错误详情。',
  ErrorResponse: '所有公开接口共用的 JSON 错误封装。',
  Asset: '用户可见的生成资源。',
  AssetListResponse: '使用页码分页的生成资源列表。',
  AssetQueryRequest:
    '资源筛选条件。提供 asset_ids 时，最多按请求顺序返回 100 个 ID，其他筛选条件会被忽略。',
  AssetBatchURLRequest: '需要获取当前 URL 的资源 ID，最多 100 个。',
  AssetURLItem: '单个资源的当前公开 URL 及其访问要求。',
  AssetURLListResponse: '请求的资源中已找到资源的当前 URL。',
  ImageTaskSource: '使用公开 HTTP(S) URL 的图片输入。',
  ImageTaskInput: '提示词和可选的 URL 图片输入。',
  ImageTaskOutput: '可选的图片输出控制参数。',
  ImageTaskCreateRequest:
    '规范化 JSON 图片任务请求。operation 为 edit 时，input.images 至少需要一项。',
  ImageTaskMultipartCreateRequest:
    '使用本地文件的 multipart 图片编辑请求，image 字段最多可重复 10 次。',
  ImageTaskResultImage: '一张生成图片及其资源元数据。',
  ImageTaskResult: '图片任务成功后的输出。',
  ImageTaskError: '图片任务的终态失败信息。',
  ImageTask: '公开图片任务快照。',
  ImageTaskListResponse: '使用游标分页的图片任务列表。',
  ImageTaskBatchQueryRequest: '1 到 100 个图片任务 ID。',
  ImageTaskBatchResponse: '按请求顺序返回图片任务，并列出当前用户不可见的 ID。',
  VideoTaskSource: '一个仅使用 URL 的视频参考素材，可携带唯一名称。',
  VideoTaskInputVideoSource:
    'edit、extension 或 remix 使用的单个源视频，可使用 HTTP(S)/Data URL 或供应商文件引用。',
  VideoTaskInput:
    '提示词及供应商无关、仅使用 URL 的参考图片、视频和音频。非 generation 操作必须另外提供 input.video。',
  VideoTaskOutput: '可选的供应商无关视频输出控制参数。',
  VideoTaskCreateRequest: '规范化的视频生成、编辑、扩展或 Remix 请求。',
  VideoTaskResultVideo: '一个临时视频输出及其访问元数据。',
  VideoTaskResult: '视频任务成功后的输出。',
  VideoTaskError: '视频任务的终态失败信息。',
  VideoTask: '公开视频任务快照。',
  VideoTaskListResponse: '使用游标分页的视频任务列表。',
  VideoTaskBatchQueryRequest: '1 到 100 个视频任务 ID。',
  VideoTaskBatchResponse: '按请求顺序返回视频任务，并列出当前用户不可见的 ID。',
  ImageMultipartUploadRequest:
    '临时 multipart 上传，最多包含 10 张 image 和 1 张 mask。',
  ImageBase64UploadItem:
    '可以是纯 Base64、data URL，或包含 b64_json、base64、data 任一字段的对象。',
  ImageBase64UploadRequest:
    '至少提供 uploads、images 或 mask 之一；最多接受 10 张 image 和 1 张 mask。',
  ImageUpload: '一张临时上传的图片或遮罩。',
  ImageUploadListResponse: '上传对象，以及可直接用于图片编辑任务的 URL。',
  MediaUploadFileRequest:
    '一个直接上传到对象存储的图片、视频或音频文件元数据。',
  MediaUploadCreateRequest: '需要创建预签名 PUT 会话的 1 到 12 个素材文件。',
  MediaUploadSession: '一个短期有效的预签名 PUT 上传会话。',
  MediaUploadSessionListResponse: '与请求文件顺序一致的预签名上传会话。',
  MediaUploadCompleteRequest: '需要校验对象大小和 MIME 的上传会话 ID。',
  MediaUpload: '一个可直接放入视频任务参考字段的临时媒体 URL。',
  MediaUploadListResponse: '按上传 ID 请求顺序返回的临时媒体 URL。',
  WebhookEvent:
    '任务终态出站事件，data.object 与对应任务查询接口返回的 DTO 一致。',
};

const schemaPropertyOverridesZhCN = {
  Error: {
    type: '便于客户端进行大类处理的错误类型。',
  },
  Asset: {
    url: '资源当前公开 URL。Adobe 任务可能直接返回原始 HTTPS CDN 签名地址；必须保留查询参数并根据 url_auth 访问，不要改写为本地内容代理地址。',
    temporary:
      '是否必须把 URL 视为临时地址。Adobe 签名结果地址过期后不会刷新或归档。',
    url_auth:
      '请求 url 所需的鉴权方式。none 表示不附加 SuperToken 密钥直接访问；resource_api_key 表示需要 Bearer ak_...。',
    status: '这些公开接口返回的资源状态固定为 available。',
  },
  AssetURLItem: {
    url: '资源当前公开 URL。必须保留签名查询参数并根据 url_auth 访问。',
    temporary: '是否必须把 URL 视为临时地址。过期的 Adobe 结果地址不会刷新。',
    url_auth:
      '请求 url 所需的鉴权方式。none 表示不附加 SuperToken 密钥直接访问；resource_api_key 表示需要 Bearer ak_...。',
  },
  ImageTaskInput: {
    images: '使用 URL 的图片输入；operation 为 edit 时至少需要一项。',
    mask: '受模型支持时用于编辑任务的可选 HTTP(S) 遮罩 URL。',
  },
  ImageTaskCreateRequest: {
    metadata: '任意 JSON 对象，保存后随任务原样返回。',
  },
  ImageTaskMultipartCreateRequest: {
    operation: '可选；multipart 请求始终创建 edit 任务。',
    image: '可重复提交的本地 PNG、JPEG 或 WebP 文件，数量为 1 到 10。',
    mask: '可选的单个本地 PNG、JPEG 或 WebP 遮罩文件。',
    metadata: 'multipart 文本字段中的 JSON 对象字符串。',
  },
  ImageUploadListResponse: {
    images: '可直接放入 input.images 的临时图片 URL。',
    mask: '可直接放入 input.mask 的临时遮罩 URL，没有时为 null。',
  },
  VideoTaskInput: {
    image: '单个主图片来源，通常用作视频起始帧。',
    reference_images: '有序参考图片；input.image 计入当前模式的图片数量限制。',
    reference_mode:
      'frame 最多接受两张图片；images 最多接受三张图片；media 最多接受 9 图、3 视频、3 音频且总计 12 项。',
    reference_videos:
      'media 模式最多三个仅 URL 的参考视频，实际时长不得超过 15 秒。',
    reference_audios:
      'media 模式最多三个仅 URL 的参考音频，实际时长不得超过 15 秒。',
    video: 'edit、extension 和 remix 操作必填的源视频。',
  },
  VideoTaskOutput: {
    duration: '请求时长，单位为秒；extension 操作中表示新增片段时长。',
  },
  VideoTaskResultVideo: {
    url: '视频当前 URL。Adobe 任务返回原始 HTTPS CDN 签名地址及其完整查询参数。',
    duration_ms:
      '供应商返回的结果时长元数据，单位为毫秒。当前 Adobe 规范化任务回显请求的 output.duration，不下载媒体正文探测时长。',
    temporary:
      '必须把 URL 视为临时地址。Adobe 签名结果地址过期后不会刷新或归档。',
    url_auth:
      '请求 url 所需的鉴权方式。none 表示不附加 SuperToken 密钥直接访问；resource_api_key 表示需要 Bearer ak_...。',
  },
  WebhookEvent: {
    id: '稳定事件 ID，接收端应使用它进行幂等处理。',
    type: '图片或视频任务的终态事件类型。',
    data: '事件数据，公开任务快照位于 data.object。',
  },
};

const extraDescriptionTranslationsZhCN = {
  'Filter by asset type.': '按资源类型筛选。',
  'Public APIs return available assets.':
    '公开接口只返回 available 状态的资源。',
  'Filter by exact task ID.': '按任务 ID 精确筛选。',
  'Filter by exact model.': '按模型名称精确筛选。',
  'Search asset ID, task ID, filename, or URL.':
    '搜索资源 ID、任务 ID、文件名或 URL。',
  'Minimum creation time in Unix seconds.': '创建时间下限，Unix 秒。',
  'Maximum creation time in Unix seconds.': '创建时间上限，Unix 秒。',
  'Page number. Alias: p.': '页码，别名为 p。',
  'Page size. Aliases: ps and size.': '每页数量，别名为 ps 和 size。',
  'Filter by one public task status.': '按一个公开任务状态筛选。',
  'Filter by generation or edit.': '按 generation 或 edit 操作筛选。',
  'Filter by generation, edit, extension, or remix.':
    '按 generation、edit、extension 或 remix 操作筛选。',
  'Filter by the caller business reference.': '按调用方业务关联 ID 筛选。',
  'Exclusive lower creation-time bound in Unix seconds.':
    '创建时间下限，Unix 秒，不包含该时间。',
  'Exclusive upper creation-time bound in Unix seconds.':
    '创建时间上限，Unix 秒，不包含该时间。',
  'Inclusive lower creation-time bound in Unix seconds.':
    '创建时间下限，Unix 秒，包含该时间。',
  'Inclusive upper creation-time bound in Unix seconds.':
    '创建时间上限，Unix 秒，包含该时间。',
  'Task ID cursor returned by the previous page.': '上一页返回的任务 ID 游标。',
  'Page size.': '每页数量。',
  'Optional caller key, unique per user. Maximum 128 characters.':
    '可选的调用方幂等 Key，同一用户内唯一，最长 128 个字符。',
  'Optional caller key, unique per user. Maximum 128 characters. Replaying it returns the original task, including a terminal failed task; use a new key for a new attempt after failure.':
    '可选的调用方幂等 Key，同一用户内唯一，最长 128 个字符。重复使用会返回原任务，包括已经终态失败的任务；失败后要发起新的尝试，请使用新的 Key。',
  'Optional caller key, unique per user. Maximum 128 characters. The same key and request replay the original task; a different request returns 409.':
    '可选的调用方幂等 Key，同一用户内唯一，最长 128 个字符。相同 Key 和请求会返回原任务，不同请求返回 409。',
  'Asset ID such as asset_xxx.': '资源 ID，例如 asset_xxx。',
  'Video Asset ID such as asset_xxx.': '视频资源 ID，例如 asset_xxx。',
  'Optional RFC 9110 byte range.': '可选的 RFC 9110 字节范围。',
  'Public task ID such as task_xxx.': '公开任务 ID，例如 task_xxx。',
  'Canonical task URL.': '该任务的规范查询 URL。',
  'Suggested polling delay in seconds.': '建议的轮询间隔，单位为秒。',
  'true when the original task is replayed.': '返回原幂等任务时为 true。',
  'Asset list.': '资源列表。',
  'Asset URL list.': '资源 URL 列表。',
  'Asset.': '单个资源。',
  'Complete video content.': '完整视频内容。',
  'Partial video content.': '部分视频内容。',
  'Task accepted.': '任务已受理。',
  'Cursor-paginated task list.': '使用游标分页的任务列表。',
  'Task.': '单个任务。',
  'Ordered task list with missing IDs.': '按请求顺序返回任务，并列出缺失 ID。',
  'Temporary uploaded URLs.': '临时上传 URL。',
  'Invalid request.': '请求无效。',
  'Missing or invalid credential.': '鉴权凭据缺失或无效。',
  'API Key, IP, expiry, or user policy denied access.':
    'API Key、IP、有效期或用户策略拒绝访问。',
  'The resource was not found for the authenticated user.':
    '当前鉴权用户下未找到该资源。',
  'The temporary upstream video resource has expired.':
    '上游临时视频资源已过期。',
  'Idempotency conflict or endpoint limit.': '幂等冲突或端点数量超限。',
  'Upload request or file is too large.': '上传请求或文件过大。',
  'The internal upload service failed.': '内部上传服务执行失败。',
  'The internal upload service is not configured.': '内部上传服务尚未配置。',
  'Internal server error.': '服务端内部错误。',
  'Optional. Multipart requests always create edit tasks.':
    '可选；multipart 请求始终创建 edit 任务。',
  'A JSON-encoded object.': '使用 JSON 编码的对象字符串。',
  'Plain base64 or an image data URL.': '纯 Base64 或图片 data URL。',
  'A public HTTP(S) URL or a provider-supported data URL. Asset URLs that require ak_ authentication are not guaranteed to be readable by an upstream provider.':
    '公开 HTTP(S) URL 或供应商支持的 data URL。需要 ak_ 鉴权的资源 URL 不保证上游可读取。',
  'One primary image source. For xAI generation this is the starting frame. Support in other operations is provider-specific.':
    '单个主图片来源；对 xAI generation 来说是起始帧，其他操作是否支持由供应商决定。',
  'Multiple reference image sources. Allowed combinations and provider-specific limits are validated by the selected adaptor.':
    '多个参考图片来源；允许的组合和供应商限制由所选适配器校验。',
  'One source video for edit, extension, or remix operations.':
    'edit、extension 或 remix 操作使用的单个源视频。',
  'Requested output duration. For extension this is the new segment duration. Provider-specific bounds are validated by the selected adaptor.':
    '请求输出时长；extension 中表示新增片段时长，具体范围由所选适配器校验。',
  'Whether supported providers should generate an output audio track. Defaults to true when omitted. This does not enable audio-reference inputs.':
    '支持该能力的供应商是否应生成成片音轨；省略时默认 true。该字段不会启用音频参考输入。',
  'Truly provider-specific advanced options keyed by provider namespace. Generated-audio and reference-mode controls must use output.generate_audio and input.reference_mode.':
    '按供应商命名空间组织的真正专属高级选项；生成音轨和参考模式必须分别使用 output.generate_audio 与 input.reference_mode。',
  'Provider-specific options keyed by provider namespace, for example provider_options.xai.':
    '按供应商命名空间组织的专属选项，例如 provider_options.xai。',
  'Provider-specific options. Gemini uses the google namespace.':
    '供应商专属选项；Gemini 使用 google 命名空间。',
  'A JSON-encoded ImageProviderOptions object.':
    'JSON 字符串编码的 ImageProviderOptions 对象。',
  'Provider-normalized usage data when available.':
    '供应商归一化后的用量数据，没有时为空对象。',
  'CSV with asset_id, task_id, asset_type, url, filename, model, and created_at.':
    'CSV 包含 asset_id、task_id、asset_type、url、filename、model 和 created_at。',
};

const descriptionTranslationsZhCN = new Map();

function collectDescriptionTranslations(source, translated) {
  for (const key of Object.keys(source)) {
    if (source[key] && translated[key]) {
      descriptionTranslationsZhCN.set(source[key], translated[key]);
    }
  }
}

collectDescriptionTranslations(
  defaultPropertyDescriptions,
  defaultPropertyDescriptionsZhCN,
);
collectDescriptionTranslations(schemaDescriptions, schemaDescriptionsZhCN);
for (const schemaName of Object.keys(schemaPropertyOverrides)) {
  collectDescriptionTranslations(
    schemaPropertyOverrides[schemaName],
    schemaPropertyOverridesZhCN[schemaName] || {},
  );
}
for (const [english, chinese] of Object.entries(
  extraDescriptionTranslationsZhCN,
)) {
  descriptionTranslationsZhCN.set(english, chinese);
}

function applySchemaDescriptions(schema, overrides, propertyPath = '') {
  if (!schema || typeof schema !== 'object') return;
  for (const [name, property] of Object.entries(schema.properties || {})) {
    const path = propertyPath ? `${propertyPath}.${name}` : name;
    property.description ||=
      overrides[path] || defaultPropertyDescriptions[name];
    applySchemaDescriptions(property, overrides, path);
  }
  if (schema.items)
    applySchemaDescriptions(schema.items, overrides, propertyPath);
  for (const keyword of ['oneOf', 'anyOf', 'allOf']) {
    for (const candidate of schema[keyword] || []) {
      applySchemaDescriptions(candidate, overrides, propertyPath);
    }
  }
  if (schema.if) applySchemaDescriptions(schema.if, overrides, propertyPath);
  if (schema.then)
    applySchemaDescriptions(schema.then, overrides, propertyPath);
  if (schema.else)
    applySchemaDescriptions(schema.else, overrides, propertyPath);
}

for (const [name, schema] of Object.entries(spec.components.schemas)) {
  schema.description ||= schemaDescriptions[name];
  applySchemaDescriptions(schema, schemaPropertyOverrides[name] || {});
}

function applyLocalizedDescriptions(value) {
  if (!value || typeof value !== 'object') return;
  if (value.description) {
    const translated = descriptionTranslationsZhCN.get(value.description);
    if (translated) value['x-description-zh-CN'] = translated;
  }
  for (const child of Object.values(value)) {
    applyLocalizedDescriptions(child);
  }
}

applyLocalizedDescriptions(spec);

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
