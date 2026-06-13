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
  CalendarDays,
  CreditCard,
  Layers,
  RefreshCw,
  Search,
  TrendingUp,
  Users,
  WalletCards,
} from 'lucide-react';
import dayjs from 'dayjs';
import { DATE_RANGE_PRESETS } from '../../constants/console.constants';
import {
  API,
  renderNumber,
  renderPaymentAmount,
  renderQuota,
  renderQuotaWithAmount,
  showError,
} from '../../helpers';

const { Text, Title } = Typography;

const CHART_CONFIG = { mode: 'desktop-browser' };

const defaultDateRange = () => [
  dayjs().subtract(29, 'day').startOf('day').toDate(),
  dayjs().endOf('day').toDate(),
];

const formatPercent = (value) => `${Number(value || 0).toFixed(1)}%`;
const formatMoney = (value) => renderPaymentAmount(value || 0);

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

const InviteStats = () => {
  const { t } = useTranslation();
  const [username, setUsername] = useState('');
  const [dateRange, setDateRange] = useState(defaultDateRange());
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(false);
  const [rankPage, setRankPage] = useState(1);
  const [rankPageSize, setRankPageSize] = useState(20);
  const [selectedInviteUser, setSelectedInviteUser] = useState(null);
  const [selectedInviteUserDetail, setSelectedInviteUserDetail] =
    useState(null);
  const [selectedInviteUserLoading, setSelectedInviteUserLoading] =
    useState(false);

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

  const loadStats = async (page = 1, pageSize = rankPageSize) => {
    const normalizedUsername = username.trim();
    if (!normalizedUsername) {
      showError(t('请输入邀请人用户名'));
      return;
    }
    if (!normalizedRange) {
      showError(t('请选择时间范围'));
      return;
    }
    setLoading(true);
    try {
      const res = await API.get('/api/invite_code/consumption', {
        params: {
          username: normalizedUsername,
          start_time: normalizedRange.startTime,
          end_time: normalizedRange.endTime,
          p: page,
          page_size: pageSize,
        },
      });
      const { success, message, data } = res.data;
      if (success) {
        setStats(data);
        setRankPage(data?.user_rank?.page || page);
        setRankPageSize(data?.user_rank?.page_size || pageSize);
        setSelectedInviteUser(null);
        setSelectedInviteUserDetail(null);
      } else {
        showError(message || t('加载失败'));
      }
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  const summary = stats?.summary || {};
  const models = stats?.models || [];
  const trend = stats?.trend || [];
  const subscriptionUsage = stats?.subscription_usage || {};
  const subscriptionUsageSummary = subscriptionUsage.summary || {};
  const subscriptionUsageModels = subscriptionUsage.models || [];
  const subscriptionUsageTrend = subscriptionUsage.trend || [];
  const subscriptionPurchase = stats?.subscription_purchase || {};
  const subscriptionPurchaseSummary = subscriptionPurchase.summary || {};
  const subscriptionPurchasePlans = subscriptionPurchase.plans || [];
  const subscriptionPurchaseTrend = subscriptionPurchase.trend || [];
  const userRank = stats?.user_rank || {};
  const userRankItems = userRank.items || [];
  const groupStats = stats?.group_stats || [];
  const inviteUserDetailSummary = selectedInviteUserDetail?.summary || {};
  const inviteUserDetailModels = selectedInviteUserDetail?.models || [];
  const inviteUserDetailTrend = selectedInviteUserDetail?.trend || [];
  const hasWalletData =
    models.length > 0 || trend.some((item) => item.quota > 0);
  const hasSubscriptionUsageData =
    subscriptionUsageModels.length > 0 ||
    subscriptionUsageTrend.some((item) => item.quota > 0);
  const hasSubscriptionPurchaseData =
    subscriptionPurchasePlans.length > 0 ||
    subscriptionPurchaseTrend.some((item) => item.amount > 0);

  const handleSearch = () => {
    setRankPage(1);
    loadStats(1, rankPageSize);
  };

  const handleRankPageChange = (page) => {
    loadStats(page, rankPageSize);
  };

  const handleRankPageSizeChange = (pageSize) => {
    setRankPageSize(pageSize);
    setRankPage(1);
    loadStats(1, pageSize);
  };

  const loadInviteUserDetail = async (record) => {
    const normalizedUsername = username.trim();
    if (!record?.user_id || !normalizedUsername || !normalizedRange) {
      return;
    }
    setSelectedInviteUser(record);
    setSelectedInviteUserDetail(null);
    setSelectedInviteUserLoading(true);
    try {
      const res = await API.get('/api/invite_code/consumption/user', {
        params: {
          username: normalizedUsername,
          user_id: record.user_id,
          start_time: normalizedRange.startTime,
          end_time: normalizedRange.endTime,
        },
      });
      const { success, message, data } = res.data;
      if (success) {
        setSelectedInviteUserDetail(data);
      } else {
        showError(message || t('加载失败'));
      }
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setSelectedInviteUserLoading(false);
    }
  };

  const modelRankSpec = useMemo(
    () => ({
      type: 'bar',
      data: [
        {
          id: 'invite-model-rank',
          values: models.slice(0, 20).map((item) => ({
            model_name: item.model_name,
            amount: item.amount || 0,
            quota: item.quota || 0,
            request_count: item.request_count || 0,
          })),
        },
      ],
      direction: 'horizontal',
      xField: 'amount',
      yField: 'model_name',
      title: {
        visible: true,
        text: t('模型消费排行'),
        subtext: t('仅统计余额消费，不含订阅包额度'),
      },
      axes: [
        {
          orient: 'bottom',
          type: 'linear',
          title: { visible: true, text: t('金额') },
        },
        { orient: 'left', type: 'band', label: { visible: true } },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: t('消费金额'),
              value: (datum) => renderQuotaWithAmount(datum.amount || 0),
            },
            {
              key: t('消费额度'),
              value: (datum) => renderQuota(datum.quota || 0),
            },
            {
              key: t('请求数'),
              value: (datum) => renderNumber(datum.request_count || 0),
            },
          ],
        },
      },
    }),
    [models, t],
  );

  const trendSpec = useMemo(
    () => ({
      type: 'line',
      data: [
        {
          id: 'invite-consumption-trend',
          values: trend.map((item) => ({
            label: item.label,
            amount: item.amount || 0,
            quota: item.quota || 0,
            request_count: item.request_count || 0,
          })),
        },
      ],
      xField: 'label',
      yField: 'amount',
      point: { visible: true },
      line: { style: { lineWidth: 2 } },
      title: {
        visible: true,
        text: t('余额消费趋势'),
        subtext: t('按天汇总邀请用户余额消费金额'),
      },
      axes: [
        { orient: 'bottom', type: 'band' },
        {
          orient: 'left',
          type: 'linear',
          title: { visible: true, text: t('金额') },
        },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: t('消费金额'),
              value: (datum) => renderQuotaWithAmount(datum.amount || 0),
            },
            {
              key: t('消费额度'),
              value: (datum) => renderQuota(datum.quota || 0),
            },
            {
              key: t('请求数'),
              value: (datum) => renderNumber(datum.request_count || 0),
            },
          ],
        },
      },
    }),
    [trend, t],
  );

  const subscriptionUsageRankSpec = useMemo(
    () => ({
      type: 'bar',
      data: [
        {
          id: 'invite-subscription-usage-rank',
          values: subscriptionUsageModels.slice(0, 20).map((item) => ({
            model_name: item.model_name,
            amount: item.amount || 0,
            quota: item.quota || 0,
            request_count: item.request_count || 0,
          })),
        },
      ],
      direction: 'horizontal',
      xField: 'amount',
      yField: 'model_name',
      title: {
        visible: true,
        text: t('订阅额度使用排行'),
        subtext: t('仅表示订阅包额度消耗，不等同于收入'),
      },
      axes: [
        {
          orient: 'bottom',
          type: 'linear',
          title: { visible: true, text: t('额度金额') },
        },
        { orient: 'left', type: 'band', label: { visible: true } },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: t('折算额度'),
              value: (datum) => renderQuotaWithAmount(datum.amount || 0),
            },
            {
              key: t('消费额度'),
              value: (datum) => renderQuota(datum.quota || 0),
            },
            {
              key: t('请求数'),
              value: (datum) => renderNumber(datum.request_count || 0),
            },
          ],
        },
      },
    }),
    [subscriptionUsageModels, t],
  );

  const subscriptionPurchaseTrendSpec = useMemo(
    () => ({
      type: 'line',
      data: [
        {
          id: 'invite-subscription-purchase-trend',
          values: subscriptionPurchaseTrend.map((item) => ({
            label: item.label,
            amount: item.amount || 0,
            order_count: item.order_count || 0,
            buyer_count: item.buyer_count || 0,
          })),
        },
      ],
      xField: 'label',
      yField: 'amount',
      point: { visible: true },
      line: { style: { lineWidth: 2 } },
      title: {
        visible: true,
        text: t('订阅包购买趋势'),
        subtext: t('按天汇总成功支付的订阅订单金额'),
      },
      axes: [
        { orient: 'bottom', type: 'band' },
        {
          orient: 'left',
          type: 'linear',
          title: { visible: true, text: t('实付金额') },
        },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: t('实付金额'),
              value: (datum) => formatMoney(datum.amount || 0),
            },
            {
              key: t('订单数'),
              value: (datum) => renderNumber(datum.order_count || 0),
            },
            {
              key: t('购买人数'),
              value: (datum) => renderNumber(datum.buyer_count || 0),
            },
          ],
        },
      },
    }),
    [subscriptionPurchaseTrend, t],
  );

  const inviteUserTrendSpec = useMemo(
    () => ({
      type: 'line',
      data: [
        {
          id: 'invite-selected-user-trend',
          values: inviteUserDetailTrend.map((item) => ({
            label: item.label,
            amount: item.amount || 0,
            quota: item.quota || 0,
            request_count: item.request_count || 0,
          })),
        },
      ],
      xField: 'label',
      yField: 'amount',
      point: { visible: true },
      line: { style: { lineWidth: 2 } },
      title: {
        visible: true,
        text: t('用户总消耗趋势'),
        subtext: t('余额和订阅额度按天合计'),
      },
      axes: [
        { orient: 'bottom', type: 'band' },
        {
          orient: 'left',
          type: 'linear',
          title: { visible: true, text: t('金额') },
        },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: t('消费金额'),
              value: (datum) => renderQuotaWithAmount(datum.amount || 0),
            },
            {
              key: t('消费额度'),
              value: (datum) => renderQuota(datum.quota || 0),
            },
            {
              key: t('请求数'),
              value: (datum) => renderNumber(datum.request_count || 0),
            },
          ],
        },
      },
    }),
    [inviteUserDetailTrend, t],
  );

  const inviteUserModelRankSpec = useMemo(
    () => ({
      type: 'bar',
      data: [
        {
          id: 'invite-selected-user-model-rank',
          values: inviteUserDetailModels
            .slice()
            .sort((a, b) => (b.amount || 0) - (a.amount || 0))
            .slice(0, 15)
            .map((item) => ({
              model_name: item.model_name,
              amount: item.amount || 0,
              quota: item.quota || 0,
              request_count: item.request_count || 0,
            })),
        },
      ],
      direction: 'horizontal',
      xField: 'amount',
      yField: 'model_name',
      title: {
        visible: true,
        text: t('用户模型消耗排行'),
        subtext: t('该用户当前时间范围内各模型消耗'),
      },
      axes: [
        {
          orient: 'bottom',
          type: 'linear',
          title: { visible: true, text: t('金额') },
        },
        { orient: 'left', type: 'band', label: { visible: true } },
      ],
      tooltip: {
        mark: {
          content: [
            {
              key: t('消费金额'),
              value: (datum) => renderQuotaWithAmount(datum.amount || 0),
            },
            {
              key: t('消费额度'),
              value: (datum) => renderQuota(datum.quota || 0),
            },
            {
              key: t('请求数'),
              value: (datum) => renderNumber(datum.request_count || 0),
            },
          ],
        },
      },
    }),
    [inviteUserDetailModels, t],
  );

  const walletColumns = useMemo(
    () => [
      {
        title: t('模型'),
        dataIndex: 'model_name',
        render: (value) => <Tag shape='circle'>{value || '-'}</Tag>,
      },
      {
        title: t('消费金额'),
        dataIndex: 'amount',
        render: (value) => renderQuotaWithAmount(value || 0),
        sorter: (a, b) => (a.amount || 0) - (b.amount || 0),
      },
      {
        title: t('消费额度'),
        dataIndex: 'quota',
        render: (value) => renderQuota(value || 0),
        sorter: (a, b) => (a.quota || 0) - (b.quota || 0),
      },
      {
        title: t('请求数'),
        dataIndex: 'request_count',
        render: (value) => renderNumber(value || 0),
        sorter: (a, b) => (a.request_count || 0) - (b.request_count || 0),
      },
      {
        title: t('占比'),
        dataIndex: 'percent',
        render: (value) => formatPercent(value),
        sorter: (a, b) => (a.percent || 0) - (b.percent || 0),
      },
    ],
    [t],
  );

  const subscriptionUsageColumns = useMemo(
    () => [
      {
        title: t('模型'),
        dataIndex: 'model_name',
        render: (value) => <Tag shape='circle'>{value || '-'}</Tag>,
      },
      {
        title: t('折算额度'),
        dataIndex: 'amount',
        render: (value) => renderQuotaWithAmount(value || 0),
        sorter: (a, b) => (a.amount || 0) - (b.amount || 0),
      },
      {
        title: t('消费额度'),
        dataIndex: 'quota',
        render: (value) => renderQuota(value || 0),
        sorter: (a, b) => (a.quota || 0) - (b.quota || 0),
      },
      {
        title: t('请求数'),
        dataIndex: 'request_count',
        render: (value) => renderNumber(value || 0),
        sorter: (a, b) => (a.request_count || 0) - (b.request_count || 0),
      },
      {
        title: t('占比'),
        dataIndex: 'percent',
        render: (value) => formatPercent(value),
        sorter: (a, b) => (a.percent || 0) - (b.percent || 0),
      },
    ],
    [t],
  );

  const subscriptionPurchaseColumns = useMemo(
    () => [
      {
        title: t('订阅包'),
        dataIndex: 'plan_title',
        render: (value, record) => (
          <Tag shape='circle'>{value || `#${record.plan_id || '-'}`}</Tag>
        ),
      },
      {
        title: t('实付金额'),
        dataIndex: 'amount',
        render: (value) => formatMoney(value || 0),
        sorter: (a, b) => (a.amount || 0) - (b.amount || 0),
      },
      {
        title: t('订单数'),
        dataIndex: 'order_count',
        render: (value) => renderNumber(value || 0),
        sorter: (a, b) => (a.order_count || 0) - (b.order_count || 0),
      },
      {
        title: t('购买人数'),
        dataIndex: 'buyer_count',
        render: (value) => renderNumber(value || 0),
        sorter: (a, b) => (a.buyer_count || 0) - (b.buyer_count || 0),
      },
      {
        title: t('占比'),
        dataIndex: 'percent',
        render: (value) => formatPercent(value),
        sorter: (a, b) => (a.percent || 0) - (b.percent || 0),
      },
    ],
    [t],
  );

  const inviteUserRankColumns = useMemo(
    () => [
      {
        title: t('用户'),
        dataIndex: 'username',
        render: (value, record) => (
          <div className='flex flex-col gap-1'>
            <Text strong>{value || '-'}</Text>
            <Text type='tertiary' size='small'>
              #{record.user_id || '-'}
            </Text>
          </div>
        ),
      },
      {
        title: t('分组'),
        dataIndex: 'group',
        render: (value) => <Tag shape='circle'>{value || '-'}</Tag>,
      },
      {
        title: t('总消费金额'),
        dataIndex: 'total_amount',
        render: (value) => renderQuotaWithAmount(value || 0),
        sorter: (a, b) => (a.total_amount || 0) - (b.total_amount || 0),
      },
      {
        title: t('余额消费金额'),
        dataIndex: 'wallet_amount',
        render: (value, record) => (
          <div className='flex flex-col gap-1'>
            <Text>{renderQuotaWithAmount(value || 0)}</Text>
            <Text type='tertiary' size='small'>
              {renderQuota(record.wallet_quota || 0)}
            </Text>
          </div>
        ),
        sorter: (a, b) => (a.wallet_amount || 0) - (b.wallet_amount || 0),
      },
      {
        title: t('订阅额度使用'),
        dataIndex: 'subscription_amount',
        render: (value, record) => (
          <div className='flex flex-col gap-1'>
            <Text>{renderQuotaWithAmount(value || 0)}</Text>
            <Text type='tertiary' size='small'>
              {renderQuota(record.subscription_quota || 0)}
            </Text>
          </div>
        ),
        sorter: (a, b) =>
          (a.subscription_amount || 0) - (b.subscription_amount || 0),
      },
      {
        title: t('请求数'),
        dataIndex: 'total_request_count',
        render: (value) => renderNumber(value || 0),
        sorter: (a, b) =>
          (a.total_request_count || 0) - (b.total_request_count || 0),
      },
    ],
    [t],
  );

  const inviteUserDetailColumns = useMemo(
    () => [
      {
        title: t('模型'),
        dataIndex: 'model_name',
        render: (value) => <Tag shape='circle'>{value || '-'}</Tag>,
      },
      {
        title: t('总消费金额'),
        dataIndex: 'amount',
        render: (value) => renderQuotaWithAmount(value || 0),
        sorter: (a, b) => (a.amount || 0) - (b.amount || 0),
      },
      {
        title: t('消费额度'),
        dataIndex: 'quota',
        render: (value) => renderQuota(value || 0),
        sorter: (a, b) => (a.quota || 0) - (b.quota || 0),
      },
      {
        title: t('请求数'),
        dataIndex: 'request_count',
        render: (value) => renderNumber(value || 0),
        sorter: (a, b) => (a.request_count || 0) - (b.request_count || 0),
      },
      {
        title: t('占比'),
        dataIndex: 'percent',
        render: (value) => formatPercent(value),
        sorter: (a, b) => (a.percent || 0) - (b.percent || 0),
      },
    ],
    [t],
  );

  const groupStatsColumns = useMemo(
    () => [
      {
        title: t('分组'),
        dataIndex: 'group',
        render: (value) => <Tag shape='circle'>{value || '-'}</Tag>,
      },
      {
        title: t('总消费金额'),
        dataIndex: 'total_amount',
        render: (value, record) => (
          <div className='flex flex-col gap-1'>
            <Text>{renderQuotaWithAmount(value || 0)}</Text>
            <Text type='tertiary' size='small'>
              {renderQuota(record.total_quota || 0)}
            </Text>
          </div>
        ),
        sorter: (a, b) => (a.total_amount || 0) - (b.total_amount || 0),
      },
      {
        title: t('余额消费金额'),
        dataIndex: 'wallet_amount',
        render: (value) => renderQuotaWithAmount(value || 0),
        sorter: (a, b) => (a.wallet_amount || 0) - (b.wallet_amount || 0),
      },
      {
        title: t('订阅额度使用'),
        dataIndex: 'subscription_amount',
        render: (value) => renderQuotaWithAmount(value || 0),
        sorter: (a, b) =>
          (a.subscription_amount || 0) - (b.subscription_amount || 0),
      },
      {
        title: t('用户数'),
        dataIndex: 'user_count',
        render: (value) => renderNumber(value || 0),
        sorter: (a, b) => (a.user_count || 0) - (b.user_count || 0),
      },
      {
        title: t('请求数'),
        dataIndex: 'total_request_count',
        render: (value) => renderNumber(value || 0),
        sorter: (a, b) =>
          (a.total_request_count || 0) - (b.total_request_count || 0),
      },
    ],
    [t],
  );

  return (
    <div className='mt-[60px] px-2'>
      <div className='flex flex-col gap-4'>
        <Card className='!rounded-lg'>
          <div className='flex flex-col gap-4'>
            <div className='flex flex-col lg:flex-row lg:items-center lg:justify-between gap-3'>
              <div>
                <Title heading={5} className='!mb-1'>
                  {t('邀请统计')}
                </Title>
                <Text type='tertiary'>
                  {t('按邀请人查询下级用户的余额消费、订阅使用和订阅购买')}
                </Text>
              </div>
              <Tag color='blue' shape='circle'>
                {t('余额消费和订阅口径分开展示')}
              </Tag>
            </div>
            <div className='grid grid-cols-1 lg:grid-cols-[minmax(180px,280px)_minmax(260px,1fr)_auto] gap-3'>
              <Input
                prefix={<Search size={16} />}
                value={username}
                onChange={(value) => setUsername(value)}
                placeholder={t('邀请人用户名')}
                showClear
                onEnterPress={handleSearch}
              />
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
              <Space>
                <Button
                  type='primary'
                  icon={<Search size={16} />}
                  loading={loading}
                  onClick={handleSearch}
                >
                  {t('查询')}
                </Button>
                <Button
                  icon={<RefreshCw size={16} />}
                  disabled={!stats}
                  loading={loading}
                  onClick={() => loadStats(rankPage, rankPageSize)}
                >
                  {t('刷新')}
                </Button>
              </Space>
            </div>
          </div>
        </Card>

        <div className='grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4'>
          <SummaryCard
            title={t('余额消费金额')}
            value={renderQuotaWithAmount(summary.wallet_amount || 0)}
            hint={renderQuota(summary.wallet_quota || 0)}
            icon={<WalletCards size={20} />}
          />
          <SummaryCard
            title={t('邀请用户数')}
            value={renderNumber(summary.invite_user_count || 0)}
            hint={
              stats?.inviter?.username
                ? `@${stats.inviter.username}`
                : t('等待查询')
            }
            icon={<Users size={20} />}
          />
          <SummaryCard
            title={t('余额请求数')}
            value={renderNumber(summary.request_count || 0)}
            hint={`${t('模型数')} ${renderNumber(summary.model_count || 0)}`}
            icon={<TrendingUp size={20} />}
          />
          <SummaryCard
            title={t('订阅额度使用')}
            value={renderQuota(subscriptionUsageSummary.quota || 0)}
            hint={`${t('请求数')} ${renderNumber(subscriptionUsageSummary.request_count || 0)}`}
            icon={<CalendarDays size={20} />}
          />
        </div>
        <div className='grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4'>
          <SummaryCard
            title={t('订阅包购买金额')}
            value={formatMoney(subscriptionPurchaseSummary.amount || 0)}
            hint={`${t('订单数')} ${renderNumber(subscriptionPurchaseSummary.order_count || 0)}`}
            icon={<CreditCard size={20} />}
          />
          <SummaryCard
            title={t('订阅购买人数')}
            value={renderNumber(subscriptionPurchaseSummary.buyer_count || 0)}
            hint={`${t('订阅包数')} ${renderNumber(subscriptionPurchaseSummary.plan_count || 0)}`}
            icon={<Users size={20} />}
          />
          <SummaryCard
            title={t('订阅使用模型数')}
            value={renderNumber(subscriptionUsageSummary.model_count || 0)}
            hint={t('不等同于订阅收入')}
            icon={<TrendingUp size={20} />}
          />
          <SummaryCard
            title={t('订阅消费拆分提示')}
            value={renderQuota(summary.excluded_subscription_quota || 0)}
            hint={`${t('已从余额消费中排除')} ${renderNumber(summary.excluded_subscription_request_count || 0)} ${t('个请求')}`}
            icon={<CalendarDays size={20} />}
          />
        </div>
        {(subscriptionPurchaseSummary.invalidated_order_count || 0) > 0 && (
          <div className='grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4'>
            <SummaryCard
              title={t('管理员作废订阅金额')}
              value={formatMoney(
                subscriptionPurchaseSummary.invalidated_amount || 0,
              )}
              hint={`${t('已从订阅包购买金额中排除')} ${renderNumber(subscriptionPurchaseSummary.invalidated_order_count || 0)} ${t('个订单')}`}
              icon={<CreditCard size={20} />}
            />
          </div>
        )}

        <div className='grid grid-cols-1 xl:grid-cols-2 gap-4'>
          <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
            <div className='h-96'>
              {hasWalletData ? (
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
                  title={t('暂无模型消费数据')}
                  description={t('请选择邀请人和时间范围后查询')}
                />
              )}
            </div>
          </Card>
          <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
            <div className='h-96'>
              {hasWalletData ? (
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
                  description={t('余额消费趋势将在查询后展示')}
                />
              )}
            </div>
          </Card>
        </div>
        <div className='grid grid-cols-1 xl:grid-cols-2 gap-4'>
          <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
            <div className='h-96'>
              {hasSubscriptionUsageData ? (
                <VChart
                  spec={subscriptionUsageRankSpec}
                  option={CHART_CONFIG}
                />
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
                  title={t('暂无订阅额度使用数据')}
                  description={t('订阅包额度使用将在查询后展示')}
                />
              )}
            </div>
          </Card>
          <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
            <div className='h-96'>
              {hasSubscriptionPurchaseData ? (
                <VChart
                  spec={subscriptionPurchaseTrendSpec}
                  option={CHART_CONFIG}
                />
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
                  title={t('暂无订阅包购买数据')}
                  description={t('成功支付的订阅订单将在查询后展示')}
                />
              )}
            </div>
          </Card>
        </div>

        <div className='grid grid-cols-1 xl:grid-cols-2 gap-4'>
          <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
            <div className='mb-3 flex flex-col gap-2 md:flex-row md:items-center md:justify-between'>
              <div>
                <Title heading={6} className='!mb-1'>
                  <span className='inline-flex items-center gap-2'>
                    <Users size={16} />
                    {t('邀请人员消费排行')}
                  </span>
                </Title>
                <Text type='tertiary'>
                  {t('点击邀请用户行查看该用户消费明细')}
                </Text>
              </div>
              <Tag color='green' shape='circle'>
                {t('默认前20，可分页查看更多')}
              </Tag>
            </div>
            <Table
              rowKey='user_id'
              columns={inviteUserRankColumns}
              dataSource={userRankItems}
              loading={loading}
              onRow={(record) => ({
                onClick: () => loadInviteUserDetail(record),
                className: 'cursor-pointer',
              })}
              pagination={{
                currentPage: userRank.page || rankPage,
                pageSize: userRank.page_size || rankPageSize,
                total: userRank.total || 0,
                showSizeChanger: true,
                pageSizeOpts: [20, 50, 100],
                onPageChange: handleRankPageChange,
                onPageSizeChange: handleRankPageSizeChange,
              }}
              scroll={{ x: 'max-content' }}
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
                  title={t('暂无邀请人员消费数据')}
                  description={t('当前筛选范围内没有消费日志')}
                />
              }
            />
          </Card>
          <Card
            className='!rounded-lg'
            title={
              <div className='flex items-center gap-2'>
                <Layers size={16} />
                {t('分组消耗统计')}
              </div>
            }
          >
            <Table
              rowKey='group'
              columns={groupStatsColumns}
              dataSource={groupStats}
              loading={loading}
              pagination={false}
              scroll={{ x: 'max-content' }}
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
                  title={t('暂无分组消耗数据')}
                  description={t('当前筛选范围内没有消费日志')}
                />
              }
            />
          </Card>
        </div>

        <Card
          className='!rounded-lg'
          title={
            <div className='flex items-center gap-2'>
              <BarChart3 size={16} />
              {t('模型消费明细')}
            </div>
          }
        >
          <Table
            rowKey='model_name'
            columns={walletColumns}
            dataSource={models}
            loading={loading}
            pagination={false}
            scroll={{ x: 'max-content' }}
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
                title={t('暂无数据')}
              />
            }
          />
        </Card>
        <div className='grid grid-cols-1 xl:grid-cols-2 gap-4'>
          <Card
            className='!rounded-lg'
            title={
              <div className='flex items-center gap-2'>
                <BarChart3 size={16} />
                {t('订阅额度使用明细')}
              </div>
            }
          >
            <Table
              rowKey='model_name'
              columns={subscriptionUsageColumns}
              dataSource={subscriptionUsageModels}
              loading={loading}
              pagination={false}
              scroll={{ x: 'max-content' }}
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
                  title={t('暂无数据')}
                />
              }
            />
          </Card>
          <Card
            className='!rounded-lg'
            title={
              <div className='flex items-center gap-2'>
                <CreditCard size={16} />
                {t('订阅包购买明细')}
              </div>
            }
          >
            <Table
              rowKey='plan_id'
              columns={subscriptionPurchaseColumns}
              dataSource={subscriptionPurchasePlans}
              loading={loading}
              pagination={false}
              scroll={{ x: 'max-content' }}
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
                  title={t('暂无数据')}
                />
              }
            />
          </Card>
        </div>
      </div>
      <SideSheet
        title={
          selectedInviteUser
            ? `${t('邀请人员消费明细')} · ${selectedInviteUser.username || selectedInviteUser.user_id}`
            : t('邀请人员消费明细')
        }
        visible={!!selectedInviteUser}
        onCancel={() => {
          setSelectedInviteUser(null);
          setSelectedInviteUserDetail(null);
        }}
        width='min(980px, 100vw)'
        placement='right'
      >
        {selectedInviteUser && (
          <div className='flex flex-col gap-4'>
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4'>
              <SummaryCard
                title={t('总消费金额')}
                value={renderQuotaWithAmount(
                  inviteUserDetailSummary.total_amount ??
                    selectedInviteUser.total_amount ??
                    0,
                )}
                hint={`${t('用户')} #${selectedInviteUser.user_id || '-'}`}
                icon={<WalletCards size={18} />}
              />
              <SummaryCard
                title={t('余额消费金额')}
                value={renderQuotaWithAmount(
                  inviteUserDetailSummary.wallet_amount ??
                    selectedInviteUser.wallet_amount ??
                    0,
                )}
                hint={renderQuota(
                  inviteUserDetailSummary.wallet_quota ??
                    selectedInviteUser.wallet_quota ??
                    0,
                )}
                icon={<WalletCards size={18} />}
              />
              <SummaryCard
                title={t('订阅额度使用')}
                value={renderQuotaWithAmount(
                  inviteUserDetailSummary.subscription_amount ??
                    selectedInviteUser.subscription_amount ??
                    0,
                )}
                hint={renderQuota(
                  inviteUserDetailSummary.subscription_quota ??
                    selectedInviteUser.subscription_quota ??
                    0,
                )}
                icon={<CalendarDays size={18} />}
              />
              <SummaryCard
                title={t('总请求数')}
                value={renderNumber(
                  inviteUserDetailSummary.total_request_count ??
                    selectedInviteUser.total_request_count ??
                    0,
                )}
                hint={`${t('模型数')} ${renderNumber(inviteUserDetailSummary.model_count || 0)}`}
                icon={<TrendingUp size={18} />}
              />
            </div>
            <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
              <div className='h-80'>
                {selectedInviteUserLoading ? (
                  <div className='flex h-full items-center justify-center'>
                    <Text type='tertiary'>{t('加载中')}</Text>
                  </div>
                ) : inviteUserDetailTrend.some((item) => item.quota > 0) ? (
                  <VChart spec={inviteUserTrendSpec} option={CHART_CONFIG} />
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
            {inviteUserDetailModels.length > 0 ? (
              <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
                <div className='h-80'>
                  <VChart
                    spec={inviteUserModelRankSpec}
                    option={CHART_CONFIG}
                  />
                </div>
              </Card>
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
                  title={t('暂无用户模型消费数据')}
                  description={t('该用户在当前筛选范围内没有模型消费记录')}
                />
              </Card>
            )}
            <Table
              rowKey='model_name'
              columns={inviteUserDetailColumns}
              dataSource={inviteUserDetailModels}
              loading={selectedInviteUserLoading}
              pagination={false}
              scroll={{ x: 'max-content' }}
            />
          </div>
        )}
      </SideSheet>
    </div>
  );
};

export default InviteStats;
