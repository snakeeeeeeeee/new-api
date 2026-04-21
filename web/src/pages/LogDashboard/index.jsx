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
import { API, renderNumber, showError } from '../../helpers';
import { useTranslation } from 'react-i18next';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import { VChart } from '@visactor/react-vchart';
import CardTable from '../../components/common/ui/CardTable';
import {
  Activity,
  BadgeAlert,
  CheckCircle2,
  Clock3,
  RefreshCw,
} from 'lucide-react';
import {
  Button,
  Card,
  Empty,
  Space,
  TabPane,
  Tabs,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';

const { Text, Title } = Typography;

const WINDOW_OPTIONS = [
  { key: '1h', label: '最近 1 小时' },
  { key: '6h', label: '最近 6 小时' },
  { key: '24h', label: '最近 24 小时' },
];

const TREND_MODE_OPTIONS = [
  { key: 'overall', label: '整体' },
  { key: 'group', label: '分组' },
  { key: 'channel', label: '渠道' },
];

const AUTO_REFRESH_MS = 60 * 1000;

const formatPercent = (value) => `${Number(value || 0).toFixed(1)}%`;
const formatUseTime = (value) => `${Number(value || 0).toFixed(1)}s`;
const formatDateTime = (timestamp) => {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
};

const SummaryCard = ({ title, value, hint, icon }) => (
  <Card className='!rounded-2xl shadow-sm border-0'>
    <div className='flex items-start justify-between gap-3'>
      <div className='min-w-0'>
        <Text type='tertiary' className='text-sm'>
          {title}
        </Text>
        <div className='mt-2 text-2xl font-semibold'>{value}</div>
        <Text type='tertiary' className='text-xs'>
          {hint}
        </Text>
      </div>
      <div className='rounded-xl bg-[var(--semi-color-fill-0)] p-3 text-[var(--semi-color-primary)]'>
        {icon}
      </div>
    </div>
  </Card>
);

const LogDashboardPage = () => {
  const { t } = useTranslation();
  const [activeWindow, setActiveWindow] = useState('1h');
  const [activeTrendMode, setActiveTrendMode] = useState('overall');
  const [dashboard, setDashboard] = useState(null);
  const [loading, setLoading] = useState(false);

  const loadDashboard = async (windowKey = activeWindow, silent = false) => {
    if (!silent) {
      setLoading(true);
    }
    try {
      const res = await API.get('/api/log/dashboard', {
        params: { window: windowKey },
      });
      const { success, message, data } = res.data;
      if (success) {
        setDashboard(data);
      } else {
        showError(message || t('加载失败'));
      }
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      if (!silent) {
        setLoading(false);
      }
    }
  };

  useEffect(() => {
    initVChartSemiTheme({
      isWatchingThemeSwitch: true,
    });
  }, []);

  useEffect(() => {
    loadDashboard(activeWindow);
  }, [activeWindow]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      loadDashboard(activeWindow, true);
    }, AUTO_REFRESH_MS);
    return () => window.clearInterval(timer);
  }, [activeWindow]);

  const trendSpec = useMemo(() => {
    const trendValues = (dashboard?.trend || []).flatMap((point) => [
      {
        label: point.label,
        type: t('成功'),
        count: point.successful_requests || 0,
      },
      {
        label: point.label,
        type: t('失败'),
        count: point.failed_requests || 0,
      },
    ]);

    return {
      type: 'bar',
      data: [
        {
          id: 'log-dashboard-trend',
          values: trendValues,
        },
      ],
      xField: 'label',
      yField: 'count',
      seriesField: 'type',
      stack: true,
      title: {
        visible: true,
        text: t('请求趋势'),
        subtext: t('成功/失败按时间窗口分桶统计'),
      },
      legends: {
        visible: true,
        orient: 'top',
      },
      axes: [
        { orient: 'bottom', type: 'band', label: { visible: true } },
        { orient: 'left', type: 'linear', title: { visible: true, text: t('请求数') } },
      ],
      color: {
        specified: {
          [t('成功')]: '#22c55e',
          [t('失败')]: '#ef4444',
        },
      },
      tooltip: {
        mark: {
          content: [
            {
              key: (datum) => datum.type,
              value: (datum) => renderNumber(datum.count || 0),
            },
          ],
        },
      },
    };
  }, [dashboard?.trend, t]);

  const buildDimensionTrendSpec = (values, chartTitle, chartSubtext) => ({
    type: 'bar',
    data: [
      {
        id: `${activeTrendMode}-trend`,
        values,
      },
    ],
    xField: 'label',
    yField: 'count',
    seriesField: 'series',
    stack: true,
    title: {
      visible: true,
      text: chartTitle,
      subtext: chartSubtext,
    },
    legends: {
      visible: true,
      orient: 'top',
    },
    axes: [
      { orient: 'bottom', type: 'band', label: { visible: true } },
      { orient: 'left', type: 'linear', title: { visible: true, text: t('请求数') } },
    ],
    tooltip: {
      mark: {
        content: [
          {
            key: (datum) => datum.series,
            value: (datum) =>
              `${t('请求')} ${renderNumber(datum.count || 0)} · ${t('成功')} ${renderNumber(datum.success_count || 0)} · ${t('失败')} ${renderNumber(datum.failure_count || 0)}`,
          },
        ],
      },
    },
  });

  const groupTrendSpec = useMemo(() => {
    const values = (dashboard?.group_trend || []).map((point) => ({
      ...point,
      series: point.is_aggregate_group
        ? `${point.series} · ${t('聚合')}`
        : point.series,
    }));
    return buildDimensionTrendSpec(
      values,
      t('分组趋势'),
      t('按最终请求结果统计各分组随时间变化'),
    );
  }, [activeTrendMode, dashboard?.group_trend, t]);

  const channelTrendSpec = useMemo(() => {
    return buildDimensionTrendSpec(
      dashboard?.channel_trend || [],
      t('渠道趋势'),
      t('按渠道尝试统计各渠道随时间变化'),
    );
  }, [activeTrendMode, dashboard?.channel_trend, t]);

  const channelColumns = useMemo(
    () => [
      {
        title: t('渠道'),
        dataIndex: 'channel_name',
        key: 'channel_name',
        render: (_, record) => (
          <div className='flex flex-col'>
            <span className='font-medium'>{record.channel_name}</span>
            <Text type='tertiary' className='text-xs'>
              channel#{record.channel_id}
            </Text>
          </div>
        ),
      },
      {
        title: t('尝试数'),
        dataIndex: 'attempt_count',
        key: 'attempt_count',
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('成功数'),
        dataIndex: 'success_count',
        key: 'success_count',
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('失败数'),
        dataIndex: 'failure_count',
        key: 'failure_count',
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('成功率'),
        dataIndex: 'success_rate',
        key: 'success_rate',
        render: (value) => formatPercent(value),
      },
      {
        title: t('失败率'),
        dataIndex: 'error_rate',
        key: 'error_rate',
        render: (value) => formatPercent(value),
      },
      {
        title: t('平均耗时'),
        dataIndex: 'average_use_time_seconds',
        key: 'average_use_time_seconds',
        render: (value) => formatUseTime(value),
      },
      {
        title: t('Top 状态码'),
        key: 'top_status_code',
        render: (_, record) =>
          record.top_status_code ? (
            <Tag shape='circle' color='red'>
              {record.top_status_code} · {renderNumber(record.top_status_code_count || 0)}
            </Tag>
          ) : (
            '-'
          ),
      },
      {
        title: t('Top 错误信息'),
        key: 'top_error_message',
        render: (_, record) =>
          record.top_error_message ? (
            <div className='max-w-[320px]'>
              <div className='truncate'>{record.top_error_message}</div>
              <Text type='tertiary' className='text-xs'>
                {t('次数')} {renderNumber(record.top_error_message_count || 0)}
              </Text>
            </div>
          ) : (
            '-'
          ),
      },
    ],
    [t],
  );

  const groupColumns = useMemo(
    () => [
      {
        title: t('分组'),
        dataIndex: 'group_name',
        key: 'group_name',
        render: (value, record) => (
          <div className='flex flex-wrap items-center gap-2'>
            <span className='font-medium'>{value || '-'}</span>
            {record?.is_aggregate_group ? (
              <Tag shape='circle' color='blue'>
                {t('聚合')}
              </Tag>
            ) : null}
          </div>
        ),
      },
      {
        title: t('总请求数'),
        dataIndex: 'total_requests',
        key: 'total_requests',
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('成功数'),
        dataIndex: 'success_count',
        key: 'success_count',
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('失败数'),
        dataIndex: 'failure_count',
        key: 'failure_count',
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('成功率'),
        dataIndex: 'success_rate',
        key: 'success_rate',
        render: (value) => formatPercent(value),
      },
      {
        title: t('失败率'),
        dataIndex: 'error_rate',
        key: 'error_rate',
        render: (value) => formatPercent(value),
      },
      {
        title: t('平均成功耗时'),
        dataIndex: 'average_success_use_time_seconds',
        key: 'average_success_use_time_seconds',
        render: (value) => formatUseTime(value),
      },
      {
        title: t('Top 状态码'),
        key: 'top_status_code',
        render: (_, record) =>
          record.top_status_code ? (
            <Tag shape='circle' color='red'>
              {record.top_status_code} · {renderNumber(record.top_status_code_count || 0)}
            </Tag>
          ) : (
            '-'
          ),
      },
      {
        title: t('Top 错误信息'),
        key: 'top_error_message',
        render: (_, record) =>
          record.top_error_message ? (
            <div className='max-w-[320px]'>
              <div className='truncate'>{record.top_error_message}</div>
              <Text type='tertiary' className='text-xs'>
                {t('次数')} {renderNumber(record.top_error_message_count || 0)}
              </Text>
            </div>
          ) : (
            '-'
          ),
      },
    ],
    [t],
  );

  const errorMessageColumns = useMemo(
    () => [
      {
        title: t('错误信息'),
        dataIndex: 'message',
        key: 'message',
        render: (value) => <div className='max-w-[480px] break-words'>{value}</div>,
      },
      {
        title: t('次数'),
        dataIndex: 'count',
        key: 'count',
        render: (value) => renderNumber(value || 0),
      },
    ],
    [t],
  );

  const statusCodeColumns = useMemo(
    () => [
      {
        title: t('状态码'),
        dataIndex: 'status_code',
        key: 'status_code',
        render: (value) => <Tag shape='circle'>{value}</Tag>,
      },
      {
        title: t('次数'),
        dataIndex: 'count',
        key: 'count',
        render: (value) => renderNumber(value || 0),
      },
    ],
    [t],
  );

  const summary = dashboard?.summary || {
    total_requests: 0,
    successful_requests: 0,
    failed_requests: 0,
    success_rate: 0,
    error_rate: 0,
    average_success_use_time_seconds: 0,
  };

  return (
    <div className='mt-[60px] px-2 pb-4 space-y-4'>
      <Card className='!rounded-2xl shadow-sm border-0'>
        <div className='flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between'>
          <div>
            <Title heading={4} className='!mb-1'>
              {t('日志看板')}
            </Title>
            <Text type='tertiary'>
              {t('基于近窗口日志统计平台最终成功率与各渠道尝试健康度')}
            </Text>
          </div>
          <div className='flex flex-col gap-3 lg:items-end'>
            <Space wrap>
              {WINDOW_OPTIONS.map((option) => (
                <Button
                  key={option.key}
                  theme={activeWindow === option.key ? 'solid' : 'outline'}
                  type={activeWindow === option.key ? 'primary' : 'tertiary'}
                  onClick={() => setActiveWindow(option.key)}
                >
                  {t(option.label)}
                </Button>
              ))}
              <Button
                theme='outline'
                type='tertiary'
                icon={<RefreshCw size={14} />}
                onClick={() => loadDashboard(activeWindow)}
                loading={loading}
              >
                {t('刷新')}
              </Button>
            </Space>
            <Space wrap>
              <Tag shape='circle' color='blue'>
                {t('自动刷新 60 秒')}
              </Tag>
              <Text type='tertiary' className='text-xs'>
                {t('最近更新时间')}：{formatDateTime(dashboard?.generated_at)}
              </Text>
            </Space>
          </div>
        </div>
      </Card>

      <div className='grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4'>
        <SummaryCard
          title={t('总请求数')}
          value={renderNumber(summary.total_requests)}
          hint={t('按 request_id 去重后的最终请求数')}
          icon={<Activity size={18} />}
        />
        <SummaryCard
          title={t('最终成功率')}
          value={formatPercent(summary.success_rate)}
          hint={`${t('成功')} ${renderNumber(summary.successful_requests)}`}
          icon={<CheckCircle2 size={18} />}
        />
        <SummaryCard
          title={t('最终失败率')}
          value={formatPercent(summary.error_rate)}
          hint={`${t('失败')} ${renderNumber(summary.failed_requests)}`}
          icon={<BadgeAlert size={18} />}
        />
        <SummaryCard
          title={t('平均成功耗时')}
          value={formatUseTime(summary.average_success_use_time_seconds)}
          hint={t('仅统计最终成功请求')}
          icon={<Clock3 size={18} />}
        />
      </div>

      <Card
        className='!rounded-2xl shadow-sm border-0'
        bodyStyle={{ padding: 8 }}
        loading={loading}
        title={
          <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
            <div>
              <Title heading={5} className='!mb-1'>
                {t('请求趋势')}
              </Title>
              <Text type='tertiary'>
                {activeTrendMode === 'overall'
                  ? t('成功/失败按时间窗口分桶统计')
                  : activeTrendMode === 'group'
                    ? t('按分组统计，包含聚合分组')
                    : t('按渠道尝试统计')}
              </Text>
            </div>
            <Tabs
              type='slash'
              activeKey={activeTrendMode}
              onChange={setActiveTrendMode}
            >
              {TREND_MODE_OPTIONS.map((option) => (
                <TabPane key={option.key} itemKey={option.key} tab={t(option.label)} />
              ))}
            </Tabs>
          </div>
        }
      >
        {(activeTrendMode === 'overall' && dashboard?.trend?.length > 0) ||
        (activeTrendMode === 'group' && (dashboard?.group_trend || []).length > 0) ||
        (activeTrendMode === 'channel' && (dashboard?.channel_trend || []).length > 0) ? (
          <div className='h-[360px]'>
            <VChart
              spec={
                activeTrendMode === 'overall'
                  ? trendSpec
                  : activeTrendMode === 'group'
                    ? groupTrendSpec
                    : channelTrendSpec
              }
            />
          </div>
        ) : (
          <Empty description={t('当前窗口暂无趋势数据')} style={{ padding: 48 }} />
        )}
      </Card>

      <Card className='!rounded-2xl shadow-sm border-0' loading={loading}>
        <div className='mb-4'>
          <Title heading={5} className='!mb-1'>
            {t('渠道统计')}
          </Title>
          <Text type='tertiary'>
            {t('按渠道尝试统计成功率、失败率与最近错误分布')}
          </Text>
        </div>
        <CardTable
          rowKey='channel_id'
          columns={channelColumns}
          dataSource={dashboard?.channels || []}
          loading={loading}
          hidePagination
          scroll={{ x: 'max-content' }}
          empty={<Empty description={t('当前窗口暂无渠道尝试数据')} style={{ padding: 24 }} />}
        />
      </Card>

      <Card className='!rounded-2xl shadow-sm border-0' loading={loading}>
        <div className='mb-4'>
          <Title heading={5} className='!mb-1'>
            {t('分组统计')}
          </Title>
          <Text type='tertiary'>
            {t('按最终请求结果统计各分组成功率、失败率与最近错误分布')}
          </Text>
        </div>
        <CardTable
          rowKey='group_name'
          columns={groupColumns}
          dataSource={dashboard?.groups || []}
          loading={loading}
          hidePagination
          scroll={{ x: 'max-content' }}
          empty={<Empty description={t('当前窗口暂无分组统计数据')} style={{ padding: 24 }} />}
        />
      </Card>

      <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
        <Card className='!rounded-2xl shadow-sm border-0' loading={loading}>
          <div className='mb-4'>
            <Title heading={5} className='!mb-1'>
              {t('Top 错误信息')}
            </Title>
            <Text type='tertiary'>
              {t('按最终失败请求聚合，已剔除 request id 等动态片段')}
            </Text>
          </div>
          <CardTable
            rowKey={(record, index) => `${record.message}-${index}`}
            columns={errorMessageColumns}
            dataSource={dashboard?.top_error_messages || []}
            loading={loading}
            hidePagination
            empty={<Empty description={t('当前窗口暂无错误信息')} style={{ padding: 24 }} />}
          />
        </Card>

        <Card className='!rounded-2xl shadow-sm border-0' loading={loading}>
          <div className='mb-4'>
            <Title heading={5} className='!mb-1'>
              {t('Top 状态码')}
            </Title>
            <Text type='tertiary'>
              {t('按最终失败请求的最后一条错误状态码聚合')}
            </Text>
          </div>
          <CardTable
            rowKey={(record, index) => `${record.status_code}-${index}`}
            columns={statusCodeColumns}
            dataSource={dashboard?.top_status_codes || []}
            loading={loading}
            hidePagination
            empty={<Empty description={t('当前窗口暂无状态码统计')} style={{ padding: 24 }} />}
          />
        </Card>
      </div>
    </div>
  );
};

export default LogDashboardPage;
