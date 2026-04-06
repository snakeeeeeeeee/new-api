import React from 'react';
import { Card, Tag, Typography } from '@douyinfe/semi-ui';
import {
  formatCheckedAtLabel,
  getHealthCheckForModel,
  HEALTH_STATUS_COLORS,
} from './healthUtils';
import HealthTimeline from './HealthTimeline';

const { Text } = Typography;
const PANEL_HISTORY_SLOTS = 60;

const ModelHealthPanel = ({ modelData, selectedGroup, healthChecksMap, t }) => {
  const health = getHealthCheckForModel(
    healthChecksMap,
    modelData?.model_name,
    selectedGroup,
  );

  if (!health) {
    return null;
  }

  const statusColor = HEALTH_STATUS_COLORS[health.status] || 'grey';
  const checkedAt = formatCheckedAtLabel(health.checkedAt);
  const historyPoints = Array.isArray(health.history) ? health.history.slice(-PANEL_HISTORY_SLOTS) : [];
  const historySlots = Array.from(
    { length: PANEL_HISTORY_SLOTS },
    (_, index) => historyPoints[index] || null,
  );

  return (
    <Card className='!rounded-2xl shadow-sm border-0 mt-3'>
      <div className='mb-3 flex items-center justify-between gap-3'>
        <div>
          <div className='text-sm font-semibold text-gray-900'>{t('健康监测')}</div>
          <div className='text-xs text-gray-500'>
            {selectedGroup} / {modelData?.model_name}
          </div>
        </div>
        <Tag color={statusColor} shape='circle'>
          {health.statusLabel || health.status}
        </Tag>
      </div>

      <div className='grid grid-cols-3 gap-3 mb-4'>
        <div className='rounded-2xl bg-black/[0.03] px-3 py-3'>
          <Text type='tertiary'>{t('健康度')}</Text>
          <div className='text-2xl font-semibold text-gray-900'>
            {typeof health.healthScore === 'number'
              ? `${health.healthScore.toFixed(1)}%`
              : '--'}
          </div>
        </div>
        <div className='rounded-2xl bg-black/[0.03] px-3 py-3'>
          <Text type='tertiary'>{t('对话延迟')}</Text>
          <div className='text-2xl font-semibold text-gray-900'>
            {health.latencyMs || '--'} ms
          </div>
        </div>
        <div className='rounded-2xl bg-black/[0.03] px-3 py-3'>
          <Text type='tertiary'>{t('平均延迟')}</Text>
          <div className='text-2xl font-semibold text-gray-900'>
            {health.averageLatencyMs || '--'} ms
          </div>
        </div>
      </div>

      <div className='mt-1'>
        <div className='mb-3 flex items-center justify-end text-sm text-gray-600'>
          <span>
            {t('上次')}: <span className='font-semibold text-gray-900'>{checkedAt || '--'}</span>
          </span>
        </div>
        <HealthTimeline historySlots={historySlots} slotHeight={42} gap={3} t={t} />
        <div className='mt-2 flex items-center justify-between text-xs font-semibold uppercase tracking-[0.08em] text-gray-400'>
          <span>Past</span>
          <span>Now</span>
        </div>
      </div>
    </Card>
  );
};

export default ModelHealthPanel;
