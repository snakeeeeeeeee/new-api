import React from 'react';
import { Tag } from '@douyinfe/semi-ui';
import {
  getHealthCheckForModel,
  HEALTH_STATUS_COLORS,
  formatCheckedAtLabel,
} from './healthUtils';
import HealthTimeline from './HealthTimeline';

const MINI_HISTORY_SLOTS = 60;

const HealthMiniSection = ({ modelName, selectedGroup, healthChecksMap, t }) => {
  const health = getHealthCheckForModel(healthChecksMap, modelName, selectedGroup);
  if (!health) {
    return null;
  }

  const score = typeof health.healthScore === 'number' ? health.healthScore.toFixed(1) : '--';
  const checkedAt = formatCheckedAtLabel(health.checkedAt);
  const statusColor = HEALTH_STATUS_COLORS[health.status] || 'grey';
  const historyPoints = Array.isArray(health.history) ? health.history.slice(-MINI_HISTORY_SLOTS) : [];
  const historySlots = Array.from(
    { length: MINI_HISTORY_SLOTS },
    (_, index) => historyPoints[index] || null,
  );

  return (
    <div className='mt-3'>
      <div className='mb-2 flex items-center justify-between gap-3'>
        <Tag color={statusColor} shape='circle' size='small'>
          {health.statusLabel || health.status}
        </Tag>
        <div className='text-[11px] text-gray-600'>
          {t('上次')}: <span className='font-semibold text-gray-900'>{checkedAt || '--'}</span>
        </div>
      </div>

      <div className='rounded-xl bg-black/[0.03] px-2.5 py-2.5'>
        <div className='grid grid-cols-3 gap-2 text-xs'>
          <div className='rounded-lg bg-white/70 px-2.5 py-2'>
            <div className='mb-1 text-[10px] text-gray-500'>{t('健康度')}</div>
            <div className='text-sm font-semibold text-gray-900'>{score}%</div>
          </div>
          <div className='rounded-lg bg-white/70 px-2.5 py-2'>
            <div className='mb-1 text-[10px] text-gray-500'>{t('对话延迟')}</div>
            <div className='text-sm font-semibold text-gray-900'>
              {health.latencyMs || '--'} ms
            </div>
          </div>
          <div className='rounded-lg bg-white/70 px-2.5 py-2'>
            <div className='mb-1 text-[10px] text-gray-500'>{t('平均延迟')}</div>
            <div className='text-sm font-semibold text-gray-900'>
              {health.averageLatencyMs || '--'} ms
            </div>
          </div>
        </div>

        <div className='mt-3'>
          <HealthTimeline historySlots={historySlots} slotHeight={14} gap={2} t={t} />
        </div>
      </div>
    </div>
  );
};

export default HealthMiniSection;
