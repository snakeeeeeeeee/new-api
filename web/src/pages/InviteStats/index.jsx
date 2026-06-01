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
        },
      });
      const { success, message, data } = res.data;
      if (success) {
        setStats(data);
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
  const hasData = models.length > 0 || trend.some((item) => item.quota > 0);

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
        { orient: 'bottom', type: 'linear', title: { visible: true, text: t('金额') } },
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
        { orient: 'left', type: 'linear', title: { visible: true, text: t('金额') } },
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

  const columns = useMemo(
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
                  {t('按邀请人查询下级用户的余额消费贡献')}
                </Text>
              </div>
              <Tag color='blue' shape='circle'>
                {t('不含订阅包额度消费')}
              </Tag>
            </div>
            <div className='grid grid-cols-1 lg:grid-cols-[minmax(180px,280px)_minmax(260px,1fr)_auto] gap-3'>
              <Input
                prefix={<Search size={16} />}
                value={username}
                onChange={(value) => setUsername(value)}
                placeholder={t('邀请人用户名')}
                showClear
                onEnterPress={loadStats}
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
            hint={stats?.inviter?.username ? `@${stats.inviter.username}` : t('等待查询')}
            icon={<Users size={20} />}
          />
          <SummaryCard
            title={t('消费请求数')}
            value={renderNumber(summary.request_count || 0)}
            hint={`${t('模型数')} ${renderNumber(summary.model_count || 0)}`}
            icon={<TrendingUp size={20} />}
          />
          <SummaryCard
            title={t('已排除订阅消费')}
            value={renderQuota(summary.excluded_subscription_quota || 0)}
            hint={`${t('请求数')} ${renderNumber(summary.excluded_subscription_request_count || 0)}`}
            icon={<CalendarDays size={20} />}
          />
        </div>

        <div className='grid grid-cols-1 xl:grid-cols-2 gap-4'>
          <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
            <div className='h-96'>
              {hasData ? (
                <VChart spec={modelRankSpec} option={CHART_CONFIG} />
              ) : (
                <Empty
                  image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
                  darkModeImage={<IllustrationNoResultDark style={{ width: 150, height: 150 }} />}
                  title={t('暂无模型消费数据')}
                  description={t('请选择邀请人和时间范围后查询')}
                />
              )}
            </div>
          </Card>
          <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
            <div className='h-96'>
              {hasData ? (
                <VChart spec={trendSpec} option={CHART_CONFIG} />
              ) : (
                <Empty
                  image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
                  darkModeImage={<IllustrationNoResultDark style={{ width: 150, height: 150 }} />}
                  title={t('暂无趋势数据')}
                  description={t('余额消费趋势将在查询后展示')}
                />
              )}
            </div>
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
            columns={columns}
            dataSource={models}
            loading={loading}
            pagination={false}
            scroll={{ x: 'max-content' }}
            empty={
              <Empty
                image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
                darkModeImage={<IllustrationNoResultDark style={{ width: 150, height: 150 }} />}
                title={t('暂无数据')}
              />
            }
          />
        </Card>
      </div>
    </div>
  );
};

export default InviteStats;
