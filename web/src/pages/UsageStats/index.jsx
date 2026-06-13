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
import { useTranslation } from 'react-i18next';
import { VChart } from '@visactor/react-vchart';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import {
  Button,
  Card,
  DatePicker,
  Empty,
  Input,
  Select,
  SideSheet,
  Space,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  BarChart3,
  Clock3,
  RefreshCw,
  Search,
  Sparkles,
  TrendingUp,
  Users,
  WalletCards,
} from 'lucide-react';
import dayjs from 'dayjs';
import { DATE_RANGE_PRESETS } from '../../constants/console.constants';
import { API, renderNumber, showError } from '../../helpers';

const { Text, Title } = Typography;

const CHART_CONFIG = { mode: 'desktop-browser' };

const defaultDateRange = () => [
  dayjs().subtract(6, 'day').startOf('day').toDate(),
  dayjs().endOf('day').toDate(),
];

const DEFAULT_QUOTA_PER_UNIT = 500000;

const formatDateTime = (timestamp) => {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
};

const formatUseTime = (value) => `${Number(value || 0).toFixed(1)}s`;

const getQuotaPerUnitValue = () => {
  const value = Number(localStorage.getItem('quota_per_unit'));
  return Number.isFinite(value) && value > 0 ? value : DEFAULT_QUOTA_PER_UNIT;
};

const quotaToUSD = (quota) => Number(quota || 0) / getQuotaPerUnitValue();

const formatUSDValue = (value) => {
  const amount = Number(value || 0);
  if (amount > 0 && amount < 0.01) {
    return `$${amount.toFixed(4)}`;
  }
  return `$${amount.toFixed(2)}`;
};

const formatQuotaUSD = (quota) => formatUSDValue(quotaToUSD(quota));

const tokensToMillion = (tokens) => Number(tokens || 0) / 1000000;

const formatTokenMillionsValue = (value) => {
  const millions = Number(value || 0);
  if (millions > 0 && millions < 0.0001) {
    return '<0.0001M';
  }
  const digits = millions > 0 && millions < 0.01 ? 4 : 2;
  return `${millions.toFixed(digits)}M`;
};

const formatTokensMillion = (tokens) =>
  formatTokenMillionsValue(tokensToMillion(tokens));

const normalizeDetailsByUser = (details = []) =>
  details.reduce((acc, item) => {
    const userId = item.user_id;
    if (!acc[userId]) {
      acc[userId] = [];
    }
    acc[userId].push({
      ...item,
      key: `${item.user_id}-${item.model_name}`,
    });
    return acc;
  }, {});

const SummaryCard = ({ title, value, hint, icon }) => (
  <Card className='!rounded-lg shadow-sm border-0'>
    <div className='flex items-start justify-between gap-3'>
      <div className='min-w-0'>
        <Text type='tertiary' className='text-sm'>
          {title}
        </Text>
        <div className='mt-2 break-words text-2xl font-semibold leading-8'>
          {value}
        </div>
        <Text type='tertiary' className='text-xs'>
          {hint}
        </Text>
      </div>
      <div className='rounded-lg bg-[var(--semi-color-fill-0)] p-3 text-[var(--semi-color-primary)]'>
        {icon}
      </div>
    </div>
  </Card>
);

const UsageStatsPage = () => {
  const { t } = useTranslation();
  const [dateRange, setDateRange] = useState(defaultDateRange());
  const [modelName, setModelName] = useState('');
  const [username, setUsername] = useState('');
  const [group, setGroup] = useState('');
  const [channel, setChannel] = useState('');
  const [trendGranularity, setTrendGranularity] = useState('auto');
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(false);
  const [selectedUser, setSelectedUser] = useState(null);
  const [selectedUserStats, setSelectedUserStats] = useState(null);
  const [selectedUserLoading, setSelectedUserLoading] = useState(false);

  useEffect(() => {
    initVChartSemiTheme({
      isWatchingThemeSwitch: true,
    });
  }, []);

  const normalizedRange = useMemo(() => {
    if (!Array.isArray(dateRange) || dateRange.length !== 2) {
      return null;
    }
    const start = dateRange[0] ? dayjs(dateRange[0]).startOf('day') : null;
    const end = dateRange[1] ? dayjs(dateRange[1]).endOf('day') : null;
    if (!start?.isValid() || !end?.isValid()) {
      return null;
    }
    return {
      startTime: start.unix(),
      endTime: end.unix(),
    };
  }, [dateRange]);

  const loadStats = async () => {
    if (!normalizedRange) {
      showError(t('请选择时间范围'));
      return;
    }

    setLoading(true);
    try {
      const params = {
        start_timestamp: normalizedRange.startTime,
        end_timestamp: normalizedRange.endTime,
        limit: 50,
      };
      if (modelName.trim()) {
        params.model_name = modelName.trim();
      }
      if (username.trim()) {
        params.username = username.trim();
      }
      if (group.trim()) {
        params.group = group.trim();
      }
      if (channel.trim()) {
        params.channel = channel.trim();
      }
      if (trendGranularity !== 'auto') {
        params.trend_granularity = trendGranularity;
      }

      const res = await API.get('/api/log/usage_stats', { params });
      const { success, message, data } = res.data;
      if (success) {
        setStats(data);
        setSelectedUser(null);
        setSelectedUserStats(null);
      } else {
        showError(message || t('加载失败'));
      }
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadStats();
  }, []);

  const summary = stats?.summary || {};
  const ranking = stats?.ranking || [];
  const trend = stats?.trend || [];
  const models = stats?.models || [];
  const detailsByUser = useMemo(
    () => normalizeDetailsByUser(stats?.user_model_details || []),
    [stats?.user_model_details],
  );
  const selectedUserDetails = useMemo(
    () =>
      selectedUserStats?.user_model_details?.length > 0
        ? normalizeDetailsByUser(selectedUserStats.user_model_details)[
            selectedUser?.user_id
          ] || []
        : selectedUser
          ? detailsByUser[selectedUser.user_id] || []
          : [],
    [detailsByUser, selectedUser, selectedUserStats?.user_model_details],
  );
  const selectedUserSummary = selectedUserStats?.summary || {};
  const selectedUserRank =
    selectedUserStats?.ranking?.[0] || selectedUser || {};
  const selectedUserTrend = selectedUserStats?.trend || [];
  const hasUsageData =
    ranking.length > 0 ||
    models.length > 0 ||
    trend.some((item) => item.request_count > 0);

  const loadSelectedUserStats = async (record) => {
    if (!record || !normalizedRange) {
      return;
    }
    setSelectedUser(record);
    setSelectedUserStats(null);
    setSelectedUserLoading(true);
    try {
      const params = {
        start_timestamp: normalizedRange.startTime,
        end_timestamp: normalizedRange.endTime,
        user_id: record.user_id,
        limit: 50,
      };
      if (modelName.trim()) {
        params.model_name = modelName.trim();
      }
      if (group.trim()) {
        params.group = group.trim();
      }
      if (channel.trim()) {
        params.channel = channel.trim();
      }
      if (trendGranularity !== 'auto') {
        params.trend_granularity = trendGranularity;
      }
      const res = await API.get('/api/log/usage_stats', { params });
      const { success, message, data } = res.data;
      if (success) {
        setSelectedUserStats(data);
      } else {
        showError(message || t('加载失败'));
      }
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setSelectedUserLoading(false);
    }
  };

  const trendSpec = useMemo(
    () => ({
      type: 'line',
      data: [
        {
          id: 'usage-stats-trend',
          values: trend.map((item) => ({
            label: item.label,
            amount_usd: quotaToUSD(item.quota),
            quota: item.quota || 0,
            request_count: item.request_count || 0,
            input_tokens: item.input_tokens ?? item.prompt_tokens ?? 0,
            cache_tokens: item.cache_tokens || 0,
            completion_tokens: item.completion_tokens || 0,
            total_tokens: item.total_tokens || 0,
          })),
        },
      ],
      xField: 'label',
      yField: 'amount_usd',
      point: { visible: true },
      title: {
        visible: true,
        text: t('额度趋势'),
        subtext: t('按筛选条件统计消费额度、请求数和 Tokens'),
      },
      axes: [
        { orient: 'bottom', type: 'band', label: { visible: true } },
        {
          orient: 'left',
          type: 'linear',
          title: { visible: true, text: '$' },
          label: {
            formatMethod: (value) => formatUSDValue(value),
          },
        },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: t('消耗额度 ($)'),
              value: (datum) => formatUSDValue(datum.amount_usd || 0),
            },
            {
              key: t('请求数'),
              value: (datum) => renderNumber(datum.request_count || 0),
            },
            {
              key: t('Token 消耗 (M)'),
              value: (datum) => formatTokensMillion(datum.total_tokens || 0),
            },
            {
              key: t('输入(含缓存写) (M)'),
              value: (datum) => formatTokensMillion(datum.input_tokens || 0),
            },
            {
              key: t('缓存读取 (M)'),
              value: (datum) => formatTokensMillion(datum.cache_tokens || 0),
            },
            {
              key: t('输出 (M)'),
              value: (datum) =>
                formatTokensMillion(datum.completion_tokens || 0),
            },
          ],
        },
      },
    }),
    [trend, t],
  );

  const modelRankSpec = useMemo(
    () => ({
      type: 'bar',
      data: [
        {
          id: 'usage-stats-model-rank',
          values: models.slice(0, 20).map((item) => ({
            model_name: item.model_name || t('未知模型'),
            amount_usd: quotaToUSD(item.quota),
            quota: item.quota || 0,
            request_count: item.request_count || 0,
            input_tokens: item.input_tokens ?? item.prompt_tokens ?? 0,
            cache_tokens: item.cache_tokens || 0,
            completion_tokens: item.completion_tokens || 0,
            total_tokens: item.total_tokens || 0,
          })),
        },
      ],
      direction: 'horizontal',
      xField: 'amount_usd',
      yField: 'model_name',
      title: {
        visible: true,
        text: t('模型消耗排行'),
        subtext: t('当前筛选范围内消耗最高的模型'),
      },
      axes: [
        {
          orient: 'bottom',
          type: 'linear',
          title: { visible: true, text: '$' },
          label: {
            formatMethod: (value) => formatUSDValue(value),
          },
        },
        { orient: 'left', type: 'band', label: { visible: true } },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: t('消耗额度 ($)'),
              value: (datum) => formatUSDValue(datum.amount_usd || 0),
            },
            {
              key: t('请求数'),
              value: (datum) => renderNumber(datum.request_count || 0),
            },
            {
              key: t('Token 消耗 (M)'),
              value: (datum) => formatTokensMillion(datum.total_tokens || 0),
            },
            {
              key: t('输入(含缓存写) (M)'),
              value: (datum) => formatTokensMillion(datum.input_tokens || 0),
            },
            {
              key: t('缓存读取 (M)'),
              value: (datum) => formatTokensMillion(datum.cache_tokens || 0),
            },
            {
              key: t('输出 (M)'),
              value: (datum) =>
                formatTokensMillion(datum.completion_tokens || 0),
            },
          ],
        },
      },
    }),
    [models, t],
  );

  const selectedUserQuotaSpec = useMemo(
    () => ({
      type: 'bar',
      data: [
        {
          id: 'usage-stats-user-model-quota',
          values: selectedUserDetails
            .slice()
            .sort((a, b) => (b.quota || 0) - (a.quota || 0))
            .slice(0, 15)
            .map((item) => ({
              model_name: item.model_name || t('未知模型'),
              amount_usd: quotaToUSD(item.quota),
              quota: item.quota || 0,
              request_count: item.request_count || 0,
              input_tokens: item.input_tokens ?? item.prompt_tokens ?? 0,
              cache_tokens: item.cache_tokens || 0,
              completion_tokens: item.completion_tokens || 0,
              total_tokens: item.total_tokens || 0,
            })),
        },
      ],
      direction: 'horizontal',
      xField: 'amount_usd',
      yField: 'model_name',
      title: {
        visible: true,
        text: t('消耗额度 ($)'),
        subtext: t('该用户各模型的美元消耗'),
      },
      axes: [
        {
          orient: 'bottom',
          type: 'linear',
          title: { visible: true, text: '$' },
          label: {
            formatMethod: (value) => formatUSDValue(value),
          },
        },
        { orient: 'left', type: 'band', label: { visible: true } },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: t('消耗额度 ($)'),
              value: (datum) => formatUSDValue(datum.amount_usd || 0),
            },
            {
              key: t('Token 消耗 (M)'),
              value: (datum) => formatTokensMillion(datum.total_tokens || 0),
            },
            {
              key: t('输入(含缓存写) (M)'),
              value: (datum) => formatTokensMillion(datum.input_tokens || 0),
            },
            {
              key: t('缓存读取 (M)'),
              value: (datum) => formatTokensMillion(datum.cache_tokens || 0),
            },
            {
              key: t('输出 (M)'),
              value: (datum) =>
                formatTokensMillion(datum.completion_tokens || 0),
            },
            {
              key: t('请求数'),
              value: (datum) => renderNumber(datum.request_count || 0),
            },
          ],
        },
      },
    }),
    [selectedUserDetails, t],
  );

  const selectedUserTrendSpec = useMemo(
    () => ({
      type: 'line',
      data: [
        {
          id: 'usage-stats-selected-user-trend',
          values: selectedUserTrend.map((item) => ({
            label: item.label,
            amount_usd: quotaToUSD(item.quota),
            quota: item.quota || 0,
            request_count: item.request_count || 0,
          })),
        },
      ],
      xField: 'label',
      yField: 'amount_usd',
      point: { visible: true },
      line: { style: { lineWidth: 2 } },
      title: {
        visible: true,
        text: t('用户总消耗趋势'),
        subtext: t('不区分模型，仅按时间汇总消耗额度'),
      },
      axes: [
        { orient: 'bottom', type: 'band' },
        {
          orient: 'left',
          type: 'linear',
          title: { visible: true, text: '$' },
          label: {
            formatMethod: (value) => formatUSDValue(value),
          },
        },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: t('消耗额度 ($)'),
              value: (datum) => formatUSDValue(datum.amount_usd || 0),
            },
            {
              key: t('请求数'),
              value: (datum) => renderNumber(datum.request_count || 0),
            },
          ],
        },
      },
    }),
    [selectedUserTrend, t],
  );

  const selectedUserTokensSpec = useMemo(
    () => ({
      type: 'bar',
      data: [
        {
          id: 'usage-stats-user-model-tokens',
          values: selectedUserDetails
            .slice()
            .sort((a, b) => (b.total_tokens || 0) - (a.total_tokens || 0))
            .slice(0, 15)
            .map((item) => ({
              model_name: item.model_name || t('未知模型'),
              token_millions: tokensToMillion(item.total_tokens),
              quota: item.quota || 0,
              request_count: item.request_count || 0,
              input_tokens: item.input_tokens ?? item.prompt_tokens ?? 0,
              cache_tokens: item.cache_tokens || 0,
              completion_tokens: item.completion_tokens || 0,
              total_tokens: item.total_tokens || 0,
            })),
        },
      ],
      direction: 'horizontal',
      xField: 'token_millions',
      yField: 'model_name',
      title: {
        visible: true,
        text: t('Token 消耗 (M)'),
        subtext: t('该用户各模型的百万 Token 消耗'),
      },
      axes: [
        {
          orient: 'bottom',
          type: 'linear',
          title: { visible: true, text: 'M Tokens' },
          label: {
            formatMethod: (value) => formatTokenMillionsValue(value),
          },
        },
        { orient: 'left', type: 'band', label: { visible: true } },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: t('Token 消耗 (M)'),
              value: (datum) =>
                formatTokenMillionsValue(datum.token_millions || 0),
            },
            {
              key: t('输入(含缓存写) (M)'),
              value: (datum) => formatTokensMillion(datum.input_tokens || 0),
            },
            {
              key: t('缓存读取 (M)'),
              value: (datum) => formatTokensMillion(datum.cache_tokens || 0),
            },
            {
              key: t('输出 (M)'),
              value: (datum) =>
                formatTokensMillion(datum.completion_tokens || 0),
            },
            {
              key: t('消耗额度 ($)'),
              value: (datum) => formatQuotaUSD(datum.quota || 0),
            },
            {
              key: t('请求数'),
              value: (datum) => renderNumber(datum.request_count || 0),
            },
          ],
        },
      },
    }),
    [selectedUserDetails, t],
  );

  const rankingColumns = useMemo(
    () => [
      {
        title: t('排名'),
        dataIndex: 'rank',
        width: 76,
        render: (_, __, index) => index + 1,
      },
      {
        title: t('用户'),
        dataIndex: 'username',
        render: (_, record) => (
          <div className='min-w-0'>
            <div className='truncate font-medium'>{record.username || '-'}</div>
            <Text type='tertiary' size='small'>
              {t('ID')} {record.user_id}
            </Text>
          </div>
        ),
      },
      {
        title: t('消耗额度 ($)'),
        dataIndex: 'quota',
        sorter: (a, b) => (a.quota || 0) - (b.quota || 0),
        render: (value) => formatQuotaUSD(value || 0),
      },
      {
        title: t('请求数'),
        dataIndex: 'request_count',
        sorter: (a, b) => (a.request_count || 0) - (b.request_count || 0),
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('Token 消耗 (M)'),
        dataIndex: 'total_tokens',
        sorter: (a, b) => (a.total_tokens || 0) - (b.total_tokens || 0),
        render: (value) => formatTokensMillion(value || 0),
      },
      {
        title: t('平均耗时'),
        dataIndex: 'average_use_time',
        render: (value) => formatUseTime(value),
      },
      {
        title: t('最后请求时间'),
        dataIndex: 'last_request_at',
        render: (value) => formatDateTime(value),
      },
    ],
    [t],
  );

  const detailColumns = useMemo(
    () => [
      {
        title: t('模型'),
        dataIndex: 'model_name',
        render: (value) => <Tag shape='circle'>{value || t('未知模型')}</Tag>,
      },
      {
        title: t('消耗额度 ($)'),
        dataIndex: 'quota',
        sorter: (a, b) => (a.quota || 0) - (b.quota || 0),
        render: (value) => formatQuotaUSD(value || 0),
      },
      {
        title: t('请求数'),
        dataIndex: 'request_count',
        sorter: (a, b) => (a.request_count || 0) - (b.request_count || 0),
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('Token 消耗 (M)'),
        dataIndex: 'total_tokens',
        sorter: (a, b) => (a.total_tokens || 0) - (b.total_tokens || 0),
        render: (value) => formatTokensMillion(value || 0),
      },
      {
        title: t('输入(含缓存写) (M)'),
        dataIndex: 'input_tokens',
        sorter: (a, b) =>
          (a.input_tokens ?? a.prompt_tokens ?? 0) -
          (b.input_tokens ?? b.prompt_tokens ?? 0),
        render: (value, record) =>
          formatTokensMillion(value ?? record.prompt_tokens ?? 0),
      },
      {
        title: t('缓存读取 (M)'),
        dataIndex: 'cache_tokens',
        sorter: (a, b) => (a.cache_tokens || 0) - (b.cache_tokens || 0),
        render: (value) => formatTokensMillion(value || 0),
      },
      {
        title: t('输出 (M)'),
        dataIndex: 'completion_tokens',
        sorter: (a, b) =>
          (a.completion_tokens || 0) - (b.completion_tokens || 0),
        render: (value) => formatTokensMillion(value || 0),
      },
      {
        title: t('平均耗时'),
        dataIndex: 'average_use_time',
        render: (value) => formatUseTime(value),
      },
    ],
    [t],
  );

  return (
    <div className='mt-[60px] px-2'>
      <div className='flex flex-col gap-4'>
        <Card className='!rounded-lg'>
          <div className='flex flex-col gap-4'>
            <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
              <div>
                <Title heading={5} className='!mb-1'>
                  {t('用量统计')}
                </Title>
                <Text type='tertiary'>
                  {t('按消费日志统计用户、模型和时间维度的额度消耗')}
                </Text>
              </div>
              <Tag color='blue' shape='circle'>
                {stats?.trend_granularity === 'day'
                  ? t('按天统计')
                  : t('按小时统计')}
              </Tag>
            </div>
            <div className='grid grid-cols-1 gap-3 xl:grid-cols-[minmax(260px,1.1fr)_minmax(150px,0.7fr)_minmax(150px,0.7fr)_minmax(120px,0.5fr)_minmax(120px,0.5fr)_minmax(120px,0.5fr)_auto]'>
              <DatePicker
                type='dateRange'
                value={dateRange}
                onChange={setDateRange}
                placeholder={[t('开始日期'), t('结束日期')]}
                showClear
                className='w-full'
                presets={DATE_RANGE_PRESETS.map((preset) => ({
                  text: t(preset.text),
                  start: preset.start(),
                  end: preset.end(),
                }))}
              />
              <Input
                prefix={<Search size={16} />}
                value={modelName}
                onChange={(value) => setModelName(value)}
                placeholder={t('模型，支持 % 通配')}
                showClear
                onEnterPress={loadStats}
              />
              <Input
                prefix={<Users size={16} />}
                value={username}
                onChange={(value) => setUsername(value)}
                placeholder={t('用户名')}
                showClear
                onEnterPress={loadStats}
              />
              <Input
                value={group}
                onChange={(value) => setGroup(value)}
                placeholder={t('分组')}
                showClear
                onEnterPress={loadStats}
              />
              <Input
                value={channel}
                onChange={(value) => setChannel(value)}
                placeholder={t('渠道 ID')}
                showClear
                onEnterPress={loadStats}
              />
              <Select
                value={trendGranularity}
                onChange={setTrendGranularity}
                optionList={[
                  { label: t('自动粒度'), value: 'auto' },
                  { label: t('小时'), value: 'hour' },
                  { label: t('天'), value: 'day' },
                ]}
              />
              <Space>
                <Button
                  type='primary'
                  icon={<Search size={16} />}
                  loading={loading}
                  onClick={loadStats}
                >
                  {t('查询')}
                </Button>
                <Button
                  icon={<RefreshCw size={16} />}
                  disabled={!stats}
                  loading={loading}
                  onClick={loadStats}
                >
                  {t('刷新')}
                </Button>
              </Space>
            </div>
          </div>
        </Card>

        <div className='grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4'>
          <SummaryCard
            title={t('总消耗额度')}
            value={formatQuotaUSD(summary.quota || 0)}
            hint={`${t('请求数')} ${renderNumber(summary.request_count || 0)}`}
            icon={<WalletCards size={20} />}
          />
          <SummaryCard
            title={t('活跃用户')}
            value={renderNumber(summary.active_user_count || 0)}
            hint={`${t('排行用户')} ${renderNumber(ranking.length || 0)}`}
            icon={<Users size={20} />}
          />
          <SummaryCard
            title={t('总 Tokens')}
            value={formatTokensMillion(summary.total_tokens || 0)}
            hint={`${t('输入(含缓存写)')} ${formatTokensMillion(summary.input_tokens ?? summary.prompt_tokens ?? 0)} / ${t('缓存读取')} ${formatTokensMillion(summary.cache_tokens || 0)} / ${t('输出')} ${formatTokensMillion(summary.completion_tokens || 0)}`}
            icon={<Sparkles size={20} />}
          />
          <SummaryCard
            title={t('模型数量')}
            value={renderNumber(models.length || 0)}
            hint={
              stats?.generated_at
                ? `${t('生成时间')} ${formatDateTime(stats.generated_at)}`
                : t('等待查询')
            }
            icon={<BarChart3 size={20} />}
          />
        </div>

        <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
          <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
            <div className='h-96'>
              {hasUsageData ? (
                <VChart spec={trendSpec} option={CHART_CONFIG} />
              ) : (
                <Empty
                  image={
                    <IllustrationNoResult style={{ width: 150, height: 150 }} />
                  }
                  darkModeImage={
                    <IllustrationNoResultDark
                      style={{ width: 150, height: 150 }}
                    />
                  }
                  title={t('暂无趋势数据')}
                  description={t('调整筛选条件后重新查询')}
                />
              )}
            </div>
          </Card>
          <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
            <div className='h-96'>
              {models.length > 0 ? (
                <VChart spec={modelRankSpec} option={CHART_CONFIG} />
              ) : (
                <Empty
                  image={
                    <IllustrationNoResult style={{ width: 150, height: 150 }} />
                  }
                  darkModeImage={
                    <IllustrationNoResultDark
                      style={{ width: 150, height: 150 }}
                    />
                  }
                  title={t('暂无模型消耗数据')}
                  description={t('当前筛选范围内没有消费日志')}
                />
              )}
            </div>
          </Card>
        </div>

        <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
          <div className='mb-3 flex flex-col gap-2 md:flex-row md:items-center md:justify-between'>
            <div>
              <Title heading={6} className='!mb-1'>
                {t('用户消耗排行')}
              </Title>
              <Text type='tertiary'>{t('点击用户行查看该用户的模型明细')}</Text>
            </div>
            <Tag color='green' shape='circle'>
              {t('按消耗额度倒序')}
            </Tag>
          </div>
          <Table
            rowKey='user_id'
            columns={rankingColumns}
            dataSource={ranking}
            loading={loading}
            pagination={{
              pageSize: 20,
              showSizeChanger: true,
              pageSizeOpts: [10, 20, 50],
            }}
            onRow={(record) => ({
              onClick: () => loadSelectedUserStats(record),
              className: 'cursor-pointer',
            })}
            empty={
              <Empty
                image={
                  <IllustrationNoResult style={{ width: 150, height: 150 }} />
                }
                darkModeImage={
                  <IllustrationNoResultDark
                    style={{ width: 150, height: 150 }}
                  />
                }
                title={t('暂无用户消耗数据')}
                description={t('当前筛选范围内没有消费日志')}
              />
            }
          />
        </Card>
      </div>

      <SideSheet
        title={
          selectedUser
            ? `${t('用户模型明细')} · ${selectedUser.username || selectedUser.user_id}`
            : t('用户模型明细')
        }
        visible={!!selectedUser}
        onCancel={() => setSelectedUser(null)}
        width='min(980px, 100vw)'
        placement='right'
      >
        {selectedUser && (
          <div className='flex flex-col gap-4'>
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
              <SummaryCard
                title={t('消耗额度 ($)')}
                value={formatQuotaUSD(
                  selectedUserSummary.quota ?? selectedUser.quota ?? 0,
                )}
                hint={`${t('ID')} ${selectedUser.user_id}`}
                icon={<WalletCards size={18} />}
              />
              <SummaryCard
                title={t('请求数')}
                value={renderNumber(
                  selectedUserSummary.request_count ??
                    selectedUser.request_count ??
                    0,
                )}
                hint={formatDateTime(selectedUserRank.last_request_at)}
                icon={<TrendingUp size={18} />}
              />
              <SummaryCard
                title={t('Token 消耗 (M)')}
                value={formatTokensMillion(
                  selectedUserSummary.total_tokens ??
                    selectedUser.total_tokens ??
                    0,
                )}
                hint={`${t('平均耗时')} ${formatUseTime(selectedUserRank.average_use_time)}`}
                icon={<Clock3 size={18} />}
              />
            </div>
            <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
              <div className='h-80'>
                {selectedUserLoading ? (
                  <div className='flex h-full items-center justify-center'>
                    <Text type='tertiary'>{t('加载中')}</Text>
                  </div>
                ) : selectedUserTrend.some((item) => item.quota > 0) ? (
                  <VChart spec={selectedUserTrendSpec} option={CHART_CONFIG} />
                ) : (
                  <Empty
                    image={
                      <IllustrationNoResult
                        style={{ width: 150, height: 150 }}
                      />
                    }
                    darkModeImage={
                      <IllustrationNoResultDark
                        style={{ width: 150, height: 150 }}
                      />
                    }
                    title={t('暂无用户趋势数据')}
                    description={t('该用户在当前筛选范围内没有消费趋势')}
                  />
                )}
              </div>
            </Card>
            {selectedUserDetails.length > 0 ? (
              <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
                <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
                  <div className='h-80'>
                    <VChart
                      spec={selectedUserQuotaSpec}
                      option={CHART_CONFIG}
                    />
                  </div>
                </Card>
                <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
                  <div className='h-80'>
                    <VChart
                      spec={selectedUserTokensSpec}
                      option={CHART_CONFIG}
                    />
                  </div>
                </Card>
              </div>
            ) : (
              <Card className='!rounded-lg'>
                <Empty
                  image={
                    <IllustrationNoResult style={{ width: 150, height: 150 }} />
                  }
                  darkModeImage={
                    <IllustrationNoResultDark
                      style={{ width: 150, height: 150 }}
                    />
                  }
                  title={t('暂无模型明细')}
                  description={t('该用户在当前筛选范围内没有模型消费记录')}
                />
              </Card>
            )}
            {selectedUserDetails.length > 0 && (
              <Table
                rowKey='key'
                columns={detailColumns}
                dataSource={selectedUserDetails}
                loading={selectedUserLoading}
                pagination={false}
                scroll={{ x: true }}
              />
            )}
          </div>
        )}
      </SideSheet>
    </div>
  );
};

export default UsageStatsPage;
