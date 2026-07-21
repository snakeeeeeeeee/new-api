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

export const EMPTY_QUEUE = {
  pending: 0,
  due: 0,
  processing: 0,
  stale: 0,
  failed: 0,
  discarded: 0,
  completed_recent: 0,
  oldest_due_age_seconds: 0,
};

export const EMPTY_WORKER = {
  running: false,
  concurrency: 0,
  endpoint_concurrency: 0,
  in_flight: 0,
  available: 0,
  saturated: false,
  started_at: 0,
  attempted_since_start: 0,
  succeeded_since_start: 0,
  failed_since_start: 0,
  timed_out_since_start: 0,
  average_duration_ms: 0,
  request_timeout_seconds: 0,
};

const normalizeSection = (value = {}) => ({
  queue: { ...EMPTY_QUEUE, ...(value.queue || {}) },
  worker: { ...EMPTY_WORKER, ...(value.worker || {}) },
});

export const normalizeStats = (value = {}) => ({
  total_unfinished: value.total_unfinished || 0,
  timeout_pending: value.timeout_pending || 0,
  over_10_minutes: value.over_10_minutes || 0,
  over_30_minutes: value.over_30_minutes || 0,
  over_60_minutes: value.over_60_minutes || 0,
  by_status: value.by_status || [],
  by_platform: value.by_platform || [],
  by_action: value.by_action || [],
  by_channel: value.by_channel || [],
  recent_window_seconds: value.recent_window_seconds || 3600,
  image_dispatch: normalizeSection(value.image_dispatch),
  webhook_delivery: normalizeSection(value.webhook_delivery),
});

export const PLATFORM_LABELS = {
  24: 'Gemini',
  41: 'VertexAI',
  48: 'xAI',
  50: 'Kling',
  51: '即梦',
  52: 'Vidu',
  54: '豆包视频 / Seedance',
  55: 'Sora',
  mj: 'Midjourney',
  suno: 'Suno',
};

export const ACTION_LABELS = {
  generate: '生成',
  textGenerate: '文生任务',
  firstTailGenerate: '首尾帧',
  referenceGenerate: '参考生成',
  remixGenerate: 'Remix',
  videoGeneration: '视频生成',
  videoEdit: '视频编辑',
  videoExtension: '视频扩展',
};

export const formatPlatform = (value) => {
  const normalized = String(value || '').trim();
  return PLATFORM_LABELS[normalized] || normalized || '-';
};

export const formatAction = (value, t) => {
  const normalized = String(value || '').trim();
  return normalized ? t(ACTION_LABELS[normalized] || normalized) : '-';
};

export const formatAge = (seconds, t) => {
  const value = Number(seconds || 0);
  if (value < 60) return t('{{count}} 秒', { count: value });
  if (value < 3600)
    return t('{{count}} 分钟', { count: Math.floor(value / 60) });
  return t('{{count}} 小时', { count: Math.floor(value / 3600) });
};

export const statusColor = (status) => {
  switch (status) {
    case 'succeeded':
    case 'success':
    case 'delivered':
      return 'green';
    case 'failed':
    case 'failure':
    case 'discarded':
      return 'red';
    case 'processing':
    case 'running':
      return 'blue';
    case 'pending':
    case 'queued':
    case 'submitted':
      return 'orange';
    default:
      return 'grey';
  }
};

export const statusLabel = (status, t) => {
  const labels = {
    pending: '等待中',
    processing: '处理中',
    delivered: '已送达',
    failed: '失败',
    discarded: '已丢弃',
    queued: '排队中',
    submitted: '已提交',
    succeeded: '已成功',
    success: '已成功',
    failure: '失败',
  };
  return t(labels[status] || status || '未知');
};
