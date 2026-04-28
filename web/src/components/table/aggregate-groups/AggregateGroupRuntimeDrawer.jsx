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
import { API, showError, timestamp2string } from '../../../helpers';
import {
  Banner,
  Button,
  Card,
  Descriptions,
  Empty,
  Modal,
  SideSheet,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconClose, IconMaximize, IconRefresh } from '@douyinfe/semi-icons';
import { useIsMobile } from '../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const formatTime = (timestamp) => {
  if (!timestamp) {
    return '-';
  }
  return timestamp2string(timestamp);
};

const getTriggerReasonLabel = (reason, t) => {
  switch (reason) {
    case 'consecutive_failures':
      return t('连续失败触发');
    case 'consecutive_slows':
      return t('连续慢请求触发');
    default:
      return '-';
  }
};

const getCurrentTriggerReasonLabel = (route, t) => {
  if (!route?.is_degraded) {
    return '-';
  }
  return getTriggerReasonLabel(route?.last_trigger_reason, t);
};

const isRouteRecentlyUsed = (route) => {
  return (route?.rpm ?? 0) > 0;
};

const getRouteStatusConfig = (route, t, routingMode = 'failover') => {
  if ((route?.priority_count ?? 0) <= 0) {
    return { color: 'grey', text: 'Unavailable' };
  }
  if (routingMode === 'cluster') {
    if (route?.is_soft_fallback) {
      return { color: 'orange', text: 'Fallback' };
    }
    if (route?.is_degraded) {
      return { color: 'yellow', text: 'Reduced' };
    }
    if (isRouteRecentlyUsed(route)) {
      return { color: 'green', text: 'In Use' };
    }
    return { color: 'blue', text: 'Available' };
  }
  if (route?.is_degraded) {
    return { color: 'red', text: 'Skipped' };
  }
  if (route?.is_active) {
    return { color: 'green', text: 'In Use' };
  }
  return { color: 'blue', text: 'Standby' };
};

const renderSwitchTag = (enabled, t) => {
  return enabled ? (
    <Tag color='green'>{t('开启')}</Tag>
  ) : (
    <Tag color='grey'>{t('关闭')}</Tag>
  );
};

const getRouteVisualStyle = (route, t, routingMode = 'failover') => {
  const statusConfig = getRouteStatusConfig(route, t, routingMode);
  switch (statusConfig.text) {
    case 'In Use':
      return {
        ...statusConfig,
        fillStart: '#effdf4',
        fillEnd: '#f7fff9',
        border: 'rgba(34, 197, 94, 0.35)',
        accent: '#16a34a',
        badgeFill: '#dcfce7',
        badgeText: '#166534',
        badgeStroke: 'rgba(34, 197, 94, 0.18)',
      };
    case 'Skipped':
      return {
        ...statusConfig,
        fillStart: '#fff1f2',
        fillEnd: '#fff7f7',
        border: 'rgba(239, 68, 68, 0.35)',
        accent: '#dc2626',
        badgeFill: '#ffe4e6',
        badgeText: '#9f1239',
        badgeStroke: 'rgba(244, 63, 94, 0.18)',
      };
    case 'Fallback':
      return {
        ...statusConfig,
        fillStart: '#fff7ed',
        fillEnd: '#fffaf5',
        border: 'rgba(251, 146, 60, 0.38)',
        accent: '#ea580c',
        badgeFill: '#ffedd5',
        badgeText: '#9a3412',
        badgeStroke: 'rgba(251, 146, 60, 0.22)',
      };
    case 'Reduced':
      return {
        ...statusConfig,
        fillStart: '#fefce8',
        fillEnd: '#fffdf3',
        border: 'rgba(234, 179, 8, 0.38)',
        accent: '#ca8a04',
        badgeFill: '#fef9c3',
        badgeText: '#854d0e',
        badgeStroke: 'rgba(234, 179, 8, 0.22)',
      };
    case 'Unavailable':
      return {
        ...statusConfig,
        fillStart: '#f8fafc',
        fillEnd: '#f1f5f9',
        border: 'rgba(148, 163, 184, 0.35)',
        accent: '#64748b',
        badgeFill: '#e2e8f0',
        badgeText: '#475569',
        badgeStroke: 'rgba(100, 116, 139, 0.16)',
      };
    default:
      return {
        ...statusConfig,
        fillStart: '#eff6ff',
        fillEnd: '#f8fbff',
        border: 'rgba(96, 165, 250, 0.28)',
        accent: '#2563eb',
        badgeFill: '#dbeafe',
        badgeText: '#1d4ed8',
        badgeStroke: 'rgba(59, 130, 246, 0.16)',
      };
  }
};

const hasRecentTrigger = (route) => {
  return (route?.last_trigger_at ?? 0) > 0;
};

const truncateLabel = (text, maxLength = 26) => {
  if (!text) {
    return '-';
  }
  return text.length > maxLength ? `${text.slice(0, maxLength - 1)}…` : text;
};

const getPillWidth = (
  text,
  minWidth = 64,
  charWidth = 8.4,
  horizontalPadding = 24,
) => {
  if (!text) {
    return minWidth;
  }
  return Math.max(minWidth, text.length * charWidth + horizontalPadding);
};

const isClusterRouteFlowing = (route) => {
  return (
    (isRouteRecentlyUsed(route) || route?.is_soft_fallback) &&
    (route?.priority_count ?? 0) > 0
  );
};

const AggregateClusterTopologyCanvas = ({
  routes = [],
  selectedRouteKey,
  onSelectRoute,
  isMobile,
  reducedMotion,
  expanded = false,
  idPrefix = 'aggregate-cluster',
  t,
}) => {
  const nodeWidth = expanded ? (isMobile ? 284 : 392) : isMobile ? 270 : 360;
  const nodeHeight = expanded ? 152 : 140;
  const nodeGap = expanded ? 24 : 18;
  const sourceWidth = isMobile ? 64 : 84;
  const sourceHeight = isMobile ? 78 : 94;
  const paddingX = isMobile ? 10 : 20;
  const paddingY = expanded ? 24 : 18;
  const lineGap = isMobile ? 30 : 64;
  const nodeX = paddingX + sourceWidth + lineGap;
  const viewWidth = nodeX + nodeWidth + paddingX;
  const listHeight =
    routes.length * nodeHeight + Math.max(routes.length - 1, 0) * nodeGap;
  const viewHeight = Math.max(listHeight, sourceHeight) + paddingY * 2;
  const sourceX = paddingX;
  const sourceY = viewHeight / 2 - sourceHeight / 2;
  const sourceCenterX = sourceX + sourceWidth / 2;
  const sourceCenterY = sourceY + sourceHeight / 2;
  const sourceRightX = sourceX + sourceWidth;
  const totalRPM = routes.reduce((sum, route) => sum + (route?.rpm || 0), 0);

  const nodes = routes.map((route, index) => {
    const y = paddingY + index * (nodeHeight + nodeGap);
    return {
      route,
      x: nodeX,
      y,
      width: nodeWidth,
      height: nodeHeight,
      centerY: y + nodeHeight / 2,
      isSelected: selectedRouteKey === route.route_group,
      visualStyle: getRouteVisualStyle(route, t, 'cluster'),
      isFlowing: isClusterRouteFlowing(route),
    };
  });

  const paths = nodes.map((node, index) => {
    const startX = sourceRightX;
    const startY = sourceCenterY;
    const endX = node.x;
    const endY = node.centerY;
    const c1X = startX + lineGap * 0.42;
    const c2X = endX - lineGap * 0.42;
    return {
      id: `${idPrefix}-flow-${node.route.route_index}-${index}`,
      path: `M ${startX} ${startY} C ${c1X} ${startY}, ${c2X} ${endY}, ${endX} ${endY}`,
      isFlowing: node.isFlowing,
    };
  });

  return (
    <div className='overflow-x-auto pb-1'>
      <svg
        width='100%'
        height={viewHeight}
        viewBox={`0 0 ${viewWidth} ${viewHeight}`}
        fill='none'
        aria-hidden='true'
        style={{ minWidth: isMobile ? undefined : viewWidth }}
      >
        <defs>
          <linearGradient
            id={`${idPrefix}-request-gradient`}
            x1='0'
            y1='0'
            x2='1'
            y2='1'
          >
            <stop offset='0%' stopColor='#eef6ff' />
            <stop offset='100%' stopColor='#f8fbff' />
          </linearGradient>
          <filter
            id={`${idPrefix}-flow-glow`}
            x='-20%'
            y='-20%'
            width='140%'
            height='140%'
          >
            <feGaussianBlur stdDeviation='1.6' result='coloredBlur' />
            <feMerge>
              <feMergeNode in='coloredBlur' />
              <feMergeNode in='SourceGraphic' />
            </feMerge>
          </filter>
          <marker
            id={`${idPrefix}-active-arrow`}
            markerWidth='8'
            markerHeight='8'
            refX='6'
            refY='4'
            orient='auto'
            markerUnits='strokeWidth'
          >
            <path d='M 0 0 L 8 4 L 0 8 z' fill='#22c55e' />
          </marker>
          {nodes.map((node) => {
            const gradientId = `${idPrefix}-node-gradient-${node.route.route_index}`;
            const glowId = `${idPrefix}-node-glow-${node.route.route_index}`;
            return (
              <React.Fragment key={gradientId}>
                <linearGradient id={gradientId} x1='0' y1='0' x2='1' y2='1'>
                  <stop offset='0%' stopColor={node.visualStyle.fillStart} />
                  <stop offset='100%' stopColor={node.visualStyle.fillEnd} />
                </linearGradient>
                <filter
                  id={glowId}
                  x='-20%'
                  y='-20%'
                  width='160%'
                  height='160%'
                >
                  <feGaussianBlur stdDeviation='8' result='blur' />
                  <feMerge>
                    <feMergeNode in='blur' />
                    <feMergeNode in='SourceGraphic' />
                  </feMerge>
                </filter>
              </React.Fragment>
            );
          })}
        </defs>

        {paths.map((pathItem) => (
          <g key={pathItem.id}>
            <path
              d={pathItem.path}
              stroke={pathItem.isFlowing ? '#bbf7d0' : '#dbe3ef'}
              strokeOpacity={pathItem.isFlowing ? 0.98 : 0.78}
              strokeWidth={pathItem.isFlowing ? 3.2 : 1.6}
              strokeLinecap='round'
              fill='none'
            />
            {pathItem.isFlowing ? (
              <>
                <path
                  d={pathItem.path}
                  stroke='#22c55e'
                  strokeOpacity='0.96'
                  strokeWidth='2.4'
                  strokeLinecap='round'
                  strokeDasharray='10 12'
                  markerEnd={`url(#${idPrefix}-active-arrow)`}
                  filter={`url(#${idPrefix}-flow-glow)`}
                  fill='none'
                >
                  {!reducedMotion ? (
                    <animate
                      attributeName='stroke-dashoffset'
                      from='44'
                      to='0'
                      dur='1.15s'
                      repeatCount='indefinite'
                    />
                  ) : null}
                </path>
                {!reducedMotion ? (
                  <circle
                    r='3.4'
                    fill='#16a34a'
                    opacity='0.95'
                    filter={`url(#${idPrefix}-flow-glow)`}
                  >
                    <animateMotion
                      dur='1.7s'
                      repeatCount='indefinite'
                      path={pathItem.path}
                    />
                  </circle>
                ) : null}
              </>
            ) : null}
          </g>
        ))}

        <g>
          <rect
            x={sourceX}
            y={sourceY}
            width={sourceWidth}
            height={sourceHeight}
            rx='22'
            fill={`url(#${idPrefix}-request-gradient)`}
            stroke='rgba(59, 130, 246, 0.28)'
            strokeWidth='1.4'
          />
          <circle
            cx={sourceCenterX}
            cy={sourceY + 28}
            r={isMobile ? 9 : 10}
            fill='#2563eb'
            opacity='0.88'
          />
          <path
            d={`M ${sourceCenterX - (isMobile ? 15 : 18)} ${sourceY + 57} C ${
              sourceCenterX - (isMobile ? 12 : 15)
            } ${sourceY + 43}, ${sourceCenterX + (isMobile ? 12 : 15)} ${
              sourceY + 43
            }, ${sourceCenterX + (isMobile ? 15 : 18)} ${sourceY + 57}`}
            stroke='#2563eb'
            strokeWidth={isMobile ? 7 : 8}
            strokeLinecap='round'
            opacity='0.8'
          />
          <text
            x={sourceCenterX}
            y={sourceY + sourceHeight - 12}
            textAnchor='middle'
            fill='#1e3a8a'
            fontSize='11'
            fontWeight='700'
          >
            {t('请求')}
          </text>
          <text
            x={sourceCenterX}
            y={sourceY + sourceHeight + 16}
            textAnchor='middle'
            fill='#64748b'
            fontSize='10.5'
            fontWeight='600'
          >
            RPM {totalRPM}
          </text>
        </g>

        {nodes.map((node) => {
          const gradientId = `${idPrefix}-node-gradient-${node.route.route_index}`;
          const glowId = `${idPrefix}-node-glow-${node.route.route_index}`;
          const statusText = node.visualStyle.text;
          const badgeWidth = getPillWidth(statusText, 72, 8.1, 24);
          const badgeX = node.x + node.width - badgeWidth - 16;
          const label = truncateLabel(
            node.route.route_group,
            isMobile ? 22 : expanded ? 36 : 30,
          );
          const metricGap = isMobile ? 6 : 8;
          const metricInset = 16;
          const metricWidth =
            (node.width - metricInset * 2 - metricGap * 2) / 3;
          const metricY = node.y + node.height - 50;
          const metricHeight = 34;
          const metrics = [
            { label: 'RPM', value: node.route.rpm ?? 0 },
            { label: t('成功'), value: node.route.success_rpm ?? 0 },
            {
              label: t('权重'),
              value: `${node.route.weight ?? 0}/${node.route.effective_weight ?? 0}`,
            },
          ];

          return (
            <g
              key={`${node.route.route_group}-${node.route.route_index}`}
              role='button'
              tabIndex={0}
              aria-label={`${node.route.route_group} ${statusText}`}
              style={{ cursor: 'pointer' }}
              onClick={() => onSelectRoute(node.route.route_group)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  onSelectRoute(node.route.route_group);
                }
              }}
            >
              <title>{node.route.route_group}</title>
              {node.isSelected ? (
                <rect
                  x={node.x - 3}
                  y={node.y - 3}
                  width={node.width + 6}
                  height={node.height + 6}
                  rx='26'
                  fill='none'
                  stroke={node.visualStyle.accent}
                  strokeOpacity='0.12'
                  strokeWidth='6'
                  filter={`url(#${glowId})`}
                />
              ) : null}
              <rect
                x={node.x}
                y={node.y}
                width={node.width}
                height={node.height}
                rx='22'
                fill={`url(#${gradientId})`}
                stroke={
                  node.isSelected
                    ? node.visualStyle.accent
                    : node.visualStyle.border
                }
                strokeOpacity={node.isSelected ? 0.95 : 1}
                strokeWidth={node.isSelected ? 2.2 : 1.25}
              />
              <circle
                cx={node.x}
                cy={node.centerY}
                r='4.2'
                fill={node.isFlowing ? '#16a34a' : '#94a3b8'}
              />
              <text
                x={node.x + 18}
                y={node.y + 27}
                fill='#64748b'
                fontSize='11.5'
                fontWeight='600'
              >
                {t('节点')} {node.route.route_index + 1}
              </text>
              <rect
                x={badgeX}
                y={node.y + 12}
                width={badgeWidth}
                height='24'
                rx='12'
                fill={node.visualStyle.badgeFill}
                stroke={node.visualStyle.badgeStroke}
                strokeWidth='1'
              />
              <text
                x={badgeX + badgeWidth / 2}
                y={node.y + 24.5}
                textAnchor='middle'
                fill={node.visualStyle.badgeText}
                dominantBaseline='middle'
                fontSize='10.5'
                fontWeight='700'
              >
                {statusText}
              </text>
              <text
                x={node.x + 18}
                y={node.y + 62}
                fill='#0f172a'
                fontSize={isMobile ? '14.5' : '16'}
                fontWeight='700'
              >
                {label}
              </text>

              {metrics.map((metric, index) => {
                const metricX =
                  node.x + metricInset + index * (metricWidth + metricGap);
                return (
                  <g key={`${node.route.route_group}-${metric.label}`}>
                    <rect
                      x={metricX}
                      y={metricY}
                      width={metricWidth}
                      height={metricHeight}
                      rx='12'
                      fill='rgba(255,255,255,0.74)'
                      stroke='rgba(148, 163, 184, 0.18)'
                      strokeWidth='1'
                    />
                    <text
                      x={metricX + 10}
                      y={metricY + 13}
                      fill='#64748b'
                      fontSize='10'
                      fontWeight='600'
                    >
                      {metric.label}
                    </text>
                    <text
                      x={metricX + 10}
                      y={metricY + 29}
                      fill='#0f172a'
                      fontSize='14'
                      fontWeight='700'
                    >
                      {metric.value}
                    </text>
                  </g>
                );
              })}
            </g>
          );
        })}
      </svg>
    </div>
  );
};

const AggregateTopologyCanvas = ({
  routes = [],
  selectedRouteKey,
  onSelectRoute,
  isMobile,
  reducedMotion,
  routingMode = 'failover',
  expanded = false,
  idPrefix = 'aggregate-runtime',
  t,
}) => {
  const isClusterMode = routingMode === 'cluster';
  if (isClusterMode) {
    return (
      <AggregateClusterTopologyCanvas
        routes={routes}
        selectedRouteKey={selectedRouteKey}
        onSelectRoute={onSelectRoute}
        isMobile={isMobile}
        reducedMotion={reducedMotion}
        expanded={expanded}
        idPrefix={idPrefix}
        t={t}
      />
    );
  }

  const nodeWidth = isMobile ? 304 : 276;
  const nodeHeight = 172;
  const gap = isMobile ? 72 : 104;
  const clusterGapX = isMobile ? 24 : 28;
  const clusterGapY = isMobile ? 24 : 28;
  const clusterColumns =
    isClusterMode && !isMobile ? Math.min(3, Math.max(routes.length, 1)) : 1;
  const clusterRows = isClusterMode
    ? Math.ceil(Math.max(routes.length, 1) / clusterColumns)
    : 1;
  const paddingX = isMobile ? 16 : 20;
  const paddingY = 18;
  const viewWidth = isClusterMode
    ? clusterColumns * nodeWidth +
      Math.max(clusterColumns - 1, 0) * clusterGapX +
      paddingX * 2
    : isMobile
      ? nodeWidth + paddingX * 2
      : routes.length * nodeWidth +
        Math.max(routes.length - 1, 0) * gap +
        paddingX * 2;
  const viewHeight = isClusterMode
    ? clusterRows * nodeHeight +
      Math.max(clusterRows - 1, 0) * clusterGapY +
      paddingY * 2
    : isMobile
      ? routes.length * nodeHeight +
        Math.max(routes.length - 1, 0) * gap +
        paddingY * 2
      : nodeHeight + paddingY * 2;

  const nodes = routes.map((route, index) => {
    const clusterCol = isClusterMode ? index % clusterColumns : 0;
    const clusterRow = isClusterMode ? Math.floor(index / clusterColumns) : 0;
    const x = isClusterMode
      ? paddingX + clusterCol * (nodeWidth + clusterGapX)
      : isMobile
        ? paddingX
        : paddingX + index * (nodeWidth + gap);
    const y = isClusterMode
      ? paddingY + clusterRow * (nodeHeight + clusterGapY)
      : isMobile
        ? paddingY + index * (nodeHeight + gap)
        : paddingY;
    return {
      route,
      x,
      y,
      width: nodeWidth,
      height: nodeHeight,
      centerX: x + nodeWidth / 2,
      centerY: y + nodeHeight / 2,
      isSelected: selectedRouteKey === route.route_group,
      visualStyle: getRouteVisualStyle(route, t, routingMode),
    };
  });

  const paths = isClusterMode
    ? []
    : nodes.slice(0, -1).map((node, index) => {
        const nextNode = nodes[index + 1];
        const path = isMobile
          ? `M ${node.centerX} ${node.y + node.height} C ${node.centerX} ${node.y + node.height + gap * 0.28}, ${nextNode.centerX} ${nextNode.y - gap * 0.28}, ${nextNode.centerX} ${nextNode.y}`
          : `M ${node.x + node.width} ${node.centerY} C ${node.x + node.width + gap * 0.28} ${node.centerY}, ${nextNode.x - gap * 0.28} ${nextNode.centerY}, ${nextNode.x} ${nextNode.centerY}`;
        return {
          id: `${node.route.route_group}-${nextNode.route.route_group}-${index}`,
          path,
        };
      });

  return (
    <div className={isMobile ? 'overflow-y-auto' : 'overflow-x-auto pb-1'}>
      <svg
        width={isMobile ? '100%' : viewWidth}
        height={viewHeight}
        viewBox={`0 0 ${viewWidth} ${viewHeight}`}
        fill='none'
        aria-hidden='true'
      >
        <defs>
          {nodes.map((node) => {
            const gradientId = `${idPrefix}-node-gradient-${node.route.route_index}`;
            const glowId = `${idPrefix}-node-glow-${node.route.route_index}`;
            return (
              <React.Fragment key={gradientId}>
                <linearGradient id={gradientId} x1='0' y1='0' x2='1' y2='1'>
                  <stop offset='0%' stopColor={node.visualStyle.fillStart} />
                  <stop offset='100%' stopColor={node.visualStyle.fillEnd} />
                </linearGradient>
                <filter
                  id={glowId}
                  x='-20%'
                  y='-20%'
                  width='160%'
                  height='160%'
                >
                  <feGaussianBlur stdDeviation='8' result='blur' />
                  <feMerge>
                    <feMergeNode in='blur' />
                    <feMergeNode in='SourceGraphic' />
                  </feMerge>
                </filter>
              </React.Fragment>
            );
          })}

          {paths.map((pathItem) => {
            const markerId = `${idPrefix}-arrow-${pathItem.id}`;
            const glowId = `${idPrefix}-flow-glow-${pathItem.id}`;
            return (
              <React.Fragment key={markerId}>
                <marker
                  id={markerId}
                  markerWidth='8'
                  markerHeight='8'
                  refX='6'
                  refY='4'
                  orient='auto'
                  markerUnits='strokeWidth'
                >
                  <path
                    d='M 0 0 L 8 4 L 0 8 z'
                    fill='#3b82f6'
                    fillOpacity='0.85'
                  />
                </marker>
                <filter
                  id={glowId}
                  x='-20%'
                  y='-20%'
                  width='140%'
                  height='140%'
                >
                  <feGaussianBlur stdDeviation='1.4' result='coloredBlur' />
                  <feMerge>
                    <feMergeNode in='coloredBlur' />
                    <feMergeNode in='SourceGraphic' />
                  </feMerge>
                </filter>
              </React.Fragment>
            );
          })}
        </defs>

        {paths.map((pathItem) => {
          const markerId = `${idPrefix}-arrow-${pathItem.id}`;
          const glowId = `${idPrefix}-flow-glow-${pathItem.id}`;
          return (
            <g key={pathItem.id}>
              <path
                d={pathItem.path}
                stroke='#cbd5e1'
                strokeOpacity='0.75'
                strokeWidth='1.4'
                strokeLinecap='round'
                fill='none'
              />
              <path
                d={pathItem.path}
                stroke='#60a5fa'
                strokeOpacity='0.95'
                strokeWidth='2.2'
                strokeLinecap='round'
                strokeDasharray='10 12'
                markerEnd={`url(#${markerId})`}
                filter={`url(#${glowId})`}
                fill='none'
              >
                {!reducedMotion ? (
                  <animate
                    attributeName='stroke-dashoffset'
                    from='44'
                    to='0'
                    dur='1.2s'
                    repeatCount='indefinite'
                  />
                ) : null}
              </path>
              {!reducedMotion ? (
                <circle
                  r='3.4'
                  fill='#3b82f6'
                  opacity='0.96'
                  filter={`url(#${glowId})`}
                >
                  <animateMotion
                    dur='1.9s'
                    repeatCount='indefinite'
                    path={pathItem.path}
                  />
                </circle>
              ) : null}
            </g>
          );
        })}

        {nodes.map((node) => {
          const gradientId = `${idPrefix}-node-gradient-${node.route.route_index}`;
          const glowId = `${idPrefix}-node-glow-${node.route.route_index}`;
          const statusText = node.visualStyle.text;
          const badgeHeight = 24;
          const badgeRadius = 12;
          const headerInset = 16;
          const headerY = node.y + 14;
          const badgeWidth = getPillWidth(statusText, 74, 8.3, 26);
          const label = truncateLabel(
            node.route.route_group,
            isMobile ? 30 : 28,
          );
          const secondMetricLabel = isClusterMode ? t('权重') : t('可选层级');
          const secondMetricValue = isClusterMode
            ? (node.route.weight ?? 0)
            : (node.route.priority_count ?? 0);
          const recentTrigger = hasRecentTrigger(node.route);
          const recentTriggerText = t('最近触发');
          const recentTriggerWidth = getPillWidth(
            recentTriggerText,
            74,
            8.2,
            22,
          );
          const recentTriggerX =
            node.x + node.width - headerInset - recentTriggerWidth;
          const centeredBadgeX = node.centerX - badgeWidth / 2;
          const mainBadgeX = recentTrigger
            ? Math.max(
                node.x + 60,
                Math.min(centeredBadgeX, recentTriggerX - badgeWidth - 10),
              )
            : centeredBadgeX;
          const metricGap = 12;
          const metricInset = 16;
          const metricWidth = (node.width - metricInset * 2 - metricGap) / 2;
          const metricY = node.y + 112;
          const metricHeight = 44;

          return (
            <g
              key={`${node.route.route_group}-${node.route.route_index}`}
              role='button'
              tabIndex={0}
              aria-label={`${node.route.route_group} ${statusText}`}
              style={{ cursor: 'pointer' }}
              onClick={() => onSelectRoute(node.route.route_group)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  onSelectRoute(node.route.route_group);
                }
              }}
            >
              <title>{node.route.route_group}</title>
              {node.isSelected ? (
                <rect
                  x={node.x - 3}
                  y={node.y - 3}
                  width={node.width + 6}
                  height={node.height + 6}
                  rx='28'
                  fill='none'
                  stroke={node.visualStyle.accent}
                  strokeOpacity='0.12'
                  strokeWidth='6'
                  filter={`url(#${glowId})`}
                />
              ) : null}
              <rect
                x={node.x}
                y={node.y}
                width={node.width}
                height={node.height}
                rx='26'
                fill={`url(#${gradientId})`}
                stroke={
                  node.isSelected
                    ? node.visualStyle.accent
                    : node.visualStyle.border
                }
                strokeOpacity={node.isSelected ? 0.95 : 1}
                strokeWidth={node.isSelected ? 2.2 : 1.25}
              />
              <text
                x={node.x + 18}
                y={node.y + 28}
                fill='#64748b'
                fontSize='11.5'
                fontWeight='600'
              >
                {t('节点')} {node.route.route_index + 1}
              </text>

              <rect
                x={mainBadgeX}
                y={headerY}
                width={badgeWidth}
                height={badgeHeight}
                rx={badgeRadius}
                fill={node.visualStyle.badgeFill}
                stroke={node.visualStyle.badgeStroke}
                strokeWidth='1'
              />
              <text
                x={mainBadgeX + badgeWidth / 2}
                y={headerY + badgeHeight / 2 + 0.5}
                textAnchor='middle'
                fill={node.visualStyle.badgeText}
                dominantBaseline='middle'
                fontSize='10.5'
                fontWeight='700'
              >
                {statusText}
              </text>

              {recentTrigger ? (
                <>
                  <rect
                    x={recentTriggerX}
                    y={headerY}
                    width={recentTriggerWidth}
                    height={badgeHeight}
                    rx={badgeRadius}
                    fill='#fff7ed'
                    stroke='rgba(251, 146, 60, 0.24)'
                    strokeWidth='1'
                  />
                  <text
                    x={recentTriggerX + recentTriggerWidth / 2}
                    y={headerY + badgeHeight / 2 + 0.5}
                    textAnchor='middle'
                    fill='#9a3412'
                    dominantBaseline='middle'
                    fontSize='10.5'
                    fontWeight='700'
                  >
                    {recentTriggerText}
                  </text>
                </>
              ) : null}

              <text
                x={node.x + 18}
                y={node.y + 80}
                fill='#0f172a'
                fontSize='16'
                fontWeight='700'
              >
                {label}
              </text>

              <rect
                x={node.x + metricInset}
                y={metricY}
                width={metricWidth}
                height={metricHeight}
                rx='16'
                fill='rgba(255,255,255,0.72)'
                stroke='rgba(148, 163, 184, 0.18)'
                strokeWidth='1'
              />
              <text
                x={node.x + metricInset + 14}
                y={metricY + 15}
                fill='#64748b'
                fontSize='10.5'
                fontWeight='600'
              >
                RPM
              </text>
              <text
                x={node.x + metricInset + 14}
                y={metricY + 33}
                fill='#0f172a'
                fontSize='16'
                fontWeight='700'
              >
                {node.route.rpm ?? 0}
              </text>

              <rect
                x={node.x + metricInset + metricWidth + metricGap}
                y={metricY}
                width={metricWidth}
                height={metricHeight}
                rx='16'
                fill='rgba(255,255,255,0.72)'
                stroke='rgba(148, 163, 184, 0.18)'
                strokeWidth='1'
              />
              <text
                x={node.x + metricInset + metricWidth + metricGap + 14}
                y={metricY + 15}
                fill='#64748b'
                fontSize='10.5'
                fontWeight='600'
              >
                {secondMetricLabel}
              </text>
              <text
                x={node.x + metricInset + metricWidth + metricGap + 14}
                y={metricY + 33}
                fill='#0f172a'
                fontSize='16'
                fontWeight='700'
              >
                {secondMetricValue}
              </text>
            </g>
          );
        })}
      </svg>
    </div>
  );
};

const AggregateGroupRuntimeDrawer = ({
  visible,
  aggregateGroup,
  onClose,
  t,
}) => {
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [runtimeData, setRuntimeData] = useState(null);
  const [selectedModel, setSelectedModel] = useState('');
  const [selectedRouteKey, setSelectedRouteKey] = useState('');
  const [reducedMotion, setReducedMotion] = useState(false);
  const [topologyModalVisible, setTopologyModalVisible] = useState(false);

  const fetchRuntime = async (modelName = '') => {
    if (!aggregateGroup?.id) {
      return;
    }
    setLoading(true);
    try {
      const params = modelName ? { model: modelName } : undefined;
      const res = await API.get(
        `/api/aggregate_group/${aggregateGroup.id}/runtime`,
        {
          params,
        },
      );
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(t(message || '获取聚合分组运行态失败'));
        return;
      }
      setRuntimeData(data);
      setSelectedModel(data?.selected_model || '');
      const nextSelectedRoute =
        data?.runtime?.routes?.find((route) => route?.is_active)?.route_group ||
        data?.runtime?.routes?.[0]?.route_group ||
        '';
      setSelectedRouteKey(nextSelectedRoute);
    } catch (error) {
      showError(error?.message || t('获取聚合分组运行态失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (visible && aggregateGroup?.id) {
      fetchRuntime();
      return;
    }
    setRuntimeData(null);
    setSelectedModel('');
    setTopologyModalVisible(false);
  }, [visible, aggregateGroup?.id]);

  useEffect(() => {
    if (
      typeof window === 'undefined' ||
      typeof window.matchMedia !== 'function'
    ) {
      return undefined;
    }
    const mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)');
    const update = () => setReducedMotion(mediaQuery.matches);
    update();
    if (typeof mediaQuery.addEventListener === 'function') {
      mediaQuery.addEventListener('change', update);
      return () => mediaQuery.removeEventListener('change', update);
    }
    mediaQuery.addListener(update);
    return () => mediaQuery.removeListener(update);
  }, []);

  const handleRefresh = () => {
    fetchRuntime(selectedModel);
  };

  const handleModelChange = (value) => {
    fetchRuntime(value);
  };

  const runtimeGroup = runtimeData?.aggregate_group || aggregateGroup;
  const routingMode = runtimeGroup?.routing_mode || 'failover';
  const isClusterMode = routingMode === 'cluster';
  const smartStrategy = runtimeData?.smart_strategy;
  const runtime = runtimeData?.runtime;
  const activeRoute = runtime?.active_route;
  const models = useMemo(
    () => runtimeData?.models || [],
    [runtimeData?.models],
  );
  const selectedRoute = useMemo(() => {
    if (!runtime?.routes?.length) {
      return null;
    }
    return (
      runtime.routes.find((route) => route.route_group === selectedRouteKey) ||
      runtime.routes[0]
    );
  }, [runtime?.routes, selectedRouteKey]);
  const degradedRouteCount = useMemo(
    () => runtime?.routes?.filter((route) => route.is_degraded).length || 0,
    [runtime?.routes],
  );
  const routeTotalRPM = useMemo(
    () =>
      runtime?.routes?.reduce((sum, route) => sum + (route?.rpm || 0), 0) || 0,
    [runtime?.routes],
  );

  return (
    <>
      <SideSheet
        placement='right'
        visible={visible}
        onCancel={onClose}
        width={isMobile ? '100%' : 760}
        closeIcon={
          <Button
            className='semi-button-tertiary semi-button-size-small semi-button-borderless'
            type='button'
            icon={<IconClose />}
            onClick={onClose}
          />
        }
        title={
          <Space>
            <Tag color='blue' shape='circle'>
              {t('运行态')}
            </Tag>
            <Title heading={4} className='m-0'>
              {runtimeGroup?.display_name ||
                runtimeGroup?.name ||
                t('聚合分组')}
            </Title>
          </Space>
        }
        bodyStyle={{
          padding: '0',
          display: 'flex',
          flexDirection: 'column',
          borderBottom: '1px solid var(--semi-color-border)',
        }}
      >
        <div className='p-2'>
          {loading && !runtimeData ? (
            <div className='flex items-center justify-center py-12'>
              <Spin size='large' tip={t('加载中...')} />
            </div>
          ) : (
            <div className='space-y-3'>
              {!smartStrategy?.effective_enabled ? (
                <Banner
                  type='warning'
                  bordered
                  title={t('智能策略当前未生效')}
                  description={t(
                    '下方状态仅作为保留运行态展示，不参与当前路由跳过决策',
                  )}
                />
              ) : null}

              <Card className='border-0 shadow-sm'>
                <div className='flex items-center justify-between gap-2 mb-3'>
                  <div>
                    <Text strong>{t('运行摘要')}</Text>
                    <div className='text-xs text-gray-600 mt-1'>
                      {t('查看当前模型下的活跃真实分组与智能策略状态')}
                    </div>
                  </div>
                  <Button
                    icon={<IconRefresh />}
                    theme='outline'
                    onClick={handleRefresh}
                    loading={loading}
                  >
                    {t('刷新')}
                  </Button>
                </div>
                <Descriptions
                  data={[
                    {
                      key: t('聚合分组'),
                      value: (
                        <Space>
                          <Text strong>{runtimeGroup?.name || '-'}</Text>
                          {runtimeGroup?.display_name ? (
                            <Tag color='blue'>{runtimeGroup.display_name}</Tag>
                          ) : null}
                        </Space>
                      ),
                    },
                    {
                      key: t('全局智能策略'),
                      value: renderSwitchTag(smartStrategy?.global_enabled, t),
                    },
                    {
                      key: t('当前分组智能策略'),
                      value: renderSwitchTag(smartStrategy?.group_enabled, t),
                    },
                    {
                      key: t('是否生效'),
                      value: renderSwitchTag(
                        smartStrategy?.effective_enabled,
                        t,
                      ),
                    },
                    {
                      key: t('路由模式'),
                      value: (
                        <Tag color={isClusterMode ? 'blue' : 'grey'}>
                          {isClusterMode ? 'Cluster' : 'Failover'}
                        </Tag>
                      ),
                    },
                    {
                      key: isClusterMode
                        ? t('最近命中真实分组')
                        : t('当前活跃真实分组'),
                      value: activeRoute?.active_group ? (
                        <Tag color='green'>{activeRoute.active_group}</Tag>
                      ) : (
                        <Text type='secondary'>{t('暂无运行态')}</Text>
                      ),
                    },
                    {
                      key: isClusterMode
                        ? t('最近命中开始时间')
                        : t('当前活跃开始时间'),
                      value: formatTime(activeRoute?.active_since_at),
                    },
                    {
                      key: t('懒恢复策略'),
                      value: runtimeGroup?.recovery_enabled
                        ? t('{{seconds}} 秒后懒恢复', {
                            seconds:
                              runtimeGroup?.recovery_interval_seconds || 0,
                          })
                        : t('关闭'),
                    },
                    {
                      key: t('当前模型'),
                      value: selectedModel || '-',
                    },
                    {
                      key: t('降级节点数'),
                      value: degradedRouteCount,
                    },
                    {
                      key: 'RPM',
                      value: routeTotalRPM,
                    },
                  ]}
                />
              </Card>

              <Card className='border-0 shadow-sm'>
                <div className='flex items-center justify-between gap-2 mb-3'>
                  <div>
                    <Text strong>{t('模型选择')}</Text>
                    <div className='text-xs text-gray-600 mt-1'>
                      {t('点击模型名称切换当前链路视图')}
                    </div>
                  </div>
                </div>
                {!models.length ? (
                  <Text type='secondary'>{t('当前聚合分组下暂无模型')}</Text>
                ) : (
                  <Space wrap>
                    {models.map((modelName) => (
                      <Tag
                        key={modelName}
                        color={selectedModel === modelName ? 'blue' : 'grey'}
                        shape='circle'
                        className='cursor-pointer transition-all duration-200'
                        style={{
                          paddingInline: 10,
                          paddingBlock: 4,
                          borderWidth: selectedModel === modelName ? 2 : 1,
                        }}
                        onClick={() => {
                          if (modelName !== selectedModel) {
                            handleModelChange(modelName);
                          }
                        }}
                      >
                        {modelName}
                      </Tag>
                    ))}
                  </Space>
                )}
              </Card>

              {!models.length ? (
                <Card className='border-0 shadow-sm'>
                  <Empty description={t('当前聚合分组下暂无模型')} />
                </Card>
              ) : !runtime ? (
                <Card className='border-0 shadow-sm'>
                  <Empty description={t('当前模型暂无运行态')} />
                </Card>
              ) : (
                <>
                  <Card className='border-0 shadow-sm'>
                    <div className='flex items-center justify-between gap-2 mb-3'>
                      <div>
                        <Text strong>{t('链路拓扑')}</Text>
                        <div className='text-xs text-gray-600 mt-1'>
                          {isClusterMode
                            ? t(
                                '左侧请求入口连接到纵向子分组；Reduced 表示节点降级但仍按有效权重参与分发，Fallback 表示无健康候选时兜底使用',
                              )
                            : t(
                                '按照真实分组顺序展示当前模型的聚合链路，点击节点查看详情',
                              )}
                        </div>
                      </div>
                      <Space>
                        {!isClusterMode && activeRoute?.active_group ? (
                          <Tag color='green'>{activeRoute.active_group}</Tag>
                        ) : null}
                        <Button
                          theme='outline'
                          icon={<IconMaximize />}
                          onClick={() => setTopologyModalVisible(true)}
                        >
                          {t('放大查看')}
                        </Button>
                      </Space>
                    </div>
                    <div className={isMobile ? '' : ''}>
                      <AggregateTopologyCanvas
                        routes={runtime.routes || []}
                        selectedRouteKey={selectedRoute?.route_group}
                        onSelectRoute={setSelectedRouteKey}
                        isMobile={isMobile}
                        reducedMotion={reducedMotion}
                        routingMode={routingMode}
                        idPrefix='aggregate-runtime-drawer'
                        t={t}
                      />
                    </div>
                  </Card>

                  <Card className='border-0 shadow-sm'>
                    <div className='flex items-center justify-between gap-2 mb-3'>
                      <div>
                        <Text strong>{t('节点详情')}</Text>
                        <div className='text-xs text-gray-600 mt-1'>
                          {selectedRoute?.route_group || t('未选择节点')}
                        </div>
                      </div>
                      {selectedRoute ? (
                        <Tag
                          color={
                            getRouteVisualStyle(selectedRoute, t, routingMode)
                              .color
                          }
                        >
                          {
                            getRouteVisualStyle(selectedRoute, t, routingMode)
                              .text
                          }
                        </Tag>
                      ) : null}
                    </div>
                    {selectedRoute ? (
                      <Descriptions
                        data={[
                          {
                            key: 'RPM',
                            value: selectedRoute.rpm ?? 0,
                          },
                          {
                            key: t('成功 RPM'),
                            value: selectedRoute.success_rpm ?? 0,
                          },
                          {
                            key: t('失败 RPM'),
                            value: selectedRoute.failure_rpm ?? 0,
                          },
                          {
                            key: t('权重'),
                            value: selectedRoute.weight ?? 0,
                          },
                          ...(isClusterMode
                            ? [
                                {
                                  key: t('有效权重'),
                                  value: selectedRoute.effective_weight ?? 0,
                                },
                              ]
                            : []),
                          {
                            key: isClusterMode
                              ? t('最近 60 秒使用')
                              : t('当前是否使用'),
                            value: isClusterMode
                              ? isRouteRecentlyUsed(selectedRoute)
                                ? t('有流量')
                                : t('无流量')
                              : selectedRoute?.is_active
                                ? t('是')
                                : t('否'),
                          },
                          ...(isClusterMode
                            ? [
                                {
                                  key: t('软回退使用'),
                                  value: selectedRoute?.is_soft_fallback
                                    ? t('是')
                                    : t('否'),
                                },
                              ]
                            : []),
                          {
                            key: t('当前索引'),
                            value: selectedRoute.route_index + 1,
                          },
                          {
                            key: t('可选层级'),
                            value: selectedRoute.priority_count ?? 0,
                          },
                          {
                            key: t('最近触发'),
                            value: hasRecentTrigger(selectedRoute)
                              ? t('有')
                              : t('无'),
                          },
                          {
                            key: t('当前降级到期时间'),
                            value: formatTime(selectedRoute.degraded_until),
                          },
                          {
                            key: t('当前连续失败计数'),
                            value: selectedRoute.consecutive_failures ?? 0,
                          },
                          {
                            key: t('当前连续慢请求计数'),
                            value: selectedRoute.consecutive_slows ?? 0,
                          },
                          {
                            key: t('最近成功时间'),
                            value: formatTime(selectedRoute.last_success_at),
                          },
                          {
                            key: t('最近失败时间'),
                            value: formatTime(selectedRoute.last_failure_at),
                          },
                          {
                            key: t('最近慢请求时间'),
                            value: formatTime(selectedRoute.last_slow_at),
                          },
                          {
                            key: t('当前降级原因'),
                            value: getCurrentTriggerReasonLabel(
                              selectedRoute,
                              t,
                            ),
                          },
                          {
                            key: t('当前降级开始时间'),
                            value: selectedRoute?.is_degraded
                              ? formatTime(selectedRoute.last_trigger_at)
                              : '-',
                          },
                          {
                            key: t('最近触发原因'),
                            value: getTriggerReasonLabel(
                              selectedRoute.last_trigger_reason,
                              t,
                            ),
                          },
                          {
                            key: t('最近触发时间'),
                            value: formatTime(selectedRoute.last_trigger_at),
                          },
                          {
                            key: isClusterMode
                              ? t('最近命中开始时间')
                              : t('当前活跃开始时间'),
                            value: selectedRoute?.is_active
                              ? formatTime(activeRoute?.active_since_at)
                              : '-',
                          },
                          {
                            key: t('最近链路切换时间'),
                            value: formatTime(activeRoute?.last_switch_at),
                          },
                        ]}
                      />
                    ) : (
                      <Empty description={t('请选择链路节点查看详情')} />
                    )}
                  </Card>
                </>
              )}
            </div>
          )}
        </div>
      </SideSheet>
      <Modal
        visible={topologyModalVisible}
        onCancel={() => setTopologyModalVisible(false)}
        footer={null}
        size={isMobile ? 'full-width' : 'large'}
        title={
          <Space>
            <Tag color={isClusterMode ? 'blue' : 'grey'} shape='circle'>
              {isClusterMode ? 'Cluster' : 'Failover'}
            </Tag>
            <Title heading={5} className='m-0'>
              {t('链路拓扑')}
            </Title>
          </Space>
        }
        bodyStyle={{ padding: isMobile ? 12 : 20 }}
      >
        <div className='mb-3 text-xs text-gray-600'>
          {isClusterMode
            ? t(
                '左侧请求节点表示当前模型请求入口；绿色流动线连接最近 60 秒有流量的 In Use、Reduced 或 Fallback 子分组。',
              )
            : t('按照真实分组顺序展示当前模型的聚合链路，点击节点查看详情')}
        </div>
        <AggregateTopologyCanvas
          routes={runtime?.routes || []}
          selectedRouteKey={selectedRoute?.route_group}
          onSelectRoute={setSelectedRouteKey}
          isMobile={isMobile}
          reducedMotion={reducedMotion}
          routingMode={routingMode}
          expanded
          idPrefix='aggregate-runtime-modal'
          t={t}
        />
      </Modal>
    </>
  );
};

export default AggregateGroupRuntimeDrawer;
