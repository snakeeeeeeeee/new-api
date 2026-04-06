import React, { useState } from 'react';
import { createPortal } from 'react-dom';
import { getHealthHistoryColor } from './healthUtils';

const EMPTY_SLOT_COLOR = 'rgba(148, 163, 184, 0.18)';

const STATUS_TONE = {
  healthy: {
    dot: '#10b981',
    text: '#10b981',
    background: 'rgba(16, 185, 129, 0.12)',
    border: 'rgba(16, 185, 129, 0.22)',
  },
  degraded: {
    dot: '#f59e0b',
    text: '#f59e0b',
    background: 'rgba(245, 158, 11, 0.12)',
    border: 'rgba(245, 158, 11, 0.22)',
  },
  failed: {
    dot: '#f43f5e',
    text: '#f43f5e',
    background: 'rgba(244, 63, 94, 0.12)',
    border: 'rgba(244, 63, 94, 0.22)',
  },
};

const formatLatency = (value) => (typeof value === 'number' ? `${value} ms` : '--');

const formatClockTime = (value) => {
  if (!value) return '--';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleTimeString('zh-CN', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
};

const getTooltipTheme = () => ({
  background: 'var(--semi-color-bg-overlay)',
  border: 'var(--semi-color-border)',
  text: 'var(--semi-color-text-0)',
  mutedText: 'var(--semi-color-text-2)',
  divider: 'var(--semi-color-border)',
  subtleFill: 'var(--semi-color-fill-0)',
});

const HealthTimeline = ({
  historySlots,
  slotHeight,
  gap,
  t,
}) => {
  const [tooltip, setTooltip] = useState(null);
  const theme = getTooltipTheme();

  return (
    <>
      <div className='flex items-center' style={{ gap }}>
        {historySlots.map((item, index) => (
          <div
            key={`${item?.checkedAt || 'empty'}-${index}`}
            className='flex-1 rounded-sm'
            style={{
              height: slotHeight,
              minWidth: 0,
              backgroundColor: item ? getHealthHistoryColor(item.status) : EMPTY_SLOT_COLOR,
              cursor: item ? 'default' : 'auto',
            }}
            onMouseEnter={item ? (event) => setTooltip({ point: item, x: event.clientX, y: event.clientY }) : undefined}
            onMouseMove={item ? (event) => setTooltip((current) => (
              current ? { ...current, x: event.clientX, y: event.clientY } : null
            )) : undefined}
            onMouseLeave={item ? () => setTooltip(null) : undefined}
          />
        ))}
      </div>

      {tooltip && typeof document !== 'undefined' && createPortal(
        <div
          className='pointer-events-none fixed z-[9999] w-52 rounded-xl border p-3 shadow-2xl'
          style={{
            left: Math.min(tooltip.x + 12, window.innerWidth - 228),
            top: Math.max(16, tooltip.y - 132),
            background: theme.background,
            borderColor: theme.border,
            color: theme.text,
            backdropFilter: 'blur(16px)',
            WebkitBackdropFilter: 'blur(16px)',
          }}
        >
          <div
            className='mb-3 flex items-center justify-between border-b pb-2'
            style={{ borderColor: theme.divider }}
          >
            <span
              className='inline-flex items-center gap-1 rounded-md px-2 py-1 text-[11px] font-semibold uppercase'
              style={{
                color: (STATUS_TONE[tooltip.point.status] || STATUS_TONE.failed).text,
                backgroundColor: (STATUS_TONE[tooltip.point.status] || STATUS_TONE.failed).background,
                boxShadow: `inset 0 0 0 1px ${(STATUS_TONE[tooltip.point.status] || STATUS_TONE.failed).border}`,
              }}
            >
              <span
                className='h-2 w-2 rounded-full'
                style={{ backgroundColor: (STATUS_TONE[tooltip.point.status] || STATUS_TONE.failed).dot }}
              />
              {tooltip.point.statusLabel || tooltip.point.status}
            </span>
            <span
              className='font-mono text-[11px]'
              style={{ color: theme.mutedText }}
            >
              {formatClockTime(tooltip.point.checkedAt)}
            </span>
          </div>

          <div className='space-y-1.5 text-sm'>
            <div className='flex items-center justify-between gap-4'>
              <span style={{ color: theme.mutedText }}>{t('延迟')}</span>
              <span
                className='font-mono font-semibold'
                style={{ color: theme.text }}
              >
                {formatLatency(tooltip.point.latencyMs)}
              </span>
            </div>
            <div className='flex items-center justify-between gap-4'>
              <span style={{ color: theme.mutedText }}>Ping</span>
              <span
                className='font-mono font-semibold'
                style={{ color: theme.text }}
              >
                {formatLatency(tooltip.point.pingLatencyMs)}
              </span>
            </div>
          </div>

          {tooltip.point.message ? (
            <div
              className='mt-3 rounded-lg px-2.5 py-2 text-[11px]'
              style={{
                background: theme.subtleFill,
                color: theme.mutedText,
              }}
            >
              {tooltip.point.message}
            </div>
          ) : null}
        </div>,
        document.body,
      )}
    </>
  );
};

export default HealthTimeline;
