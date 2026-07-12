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

import React, { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { VChart } from '@visactor/react-vchart';
import { Banner, Card, Empty, Skeleton, Typography } from '@douyinfe/semi-ui';
import {
  Activity,
  CalendarCheck2,
  CircleDollarSign,
  Users,
} from 'lucide-react';
import {
  CHART_CONFIG,
  formatDateTime,
  formatQuotaUSD,
  formatTokensMillion,
  formatUSDValue,
  quotaToUSD,
} from './utils';
import { renderNumber } from '../../helpers';

const { Text, Title } = Typography;

const MetricCard = ({ title, value, hint, icon }) => (
  <Card className='!rounded-lg border border-[var(--semi-color-border)] shadow-none'>
    <div className='flex min-h-24 items-start justify-between gap-3'>
      <div className='min-w-0'>
        <Text type='tertiary' className='text-sm'>
          {title}
        </Text>
        <div className='mt-2 break-words text-2xl font-semibold leading-8 tabular-nums'>
          {value}
        </div>
        <Text type='tertiary' className='text-xs'>
          {hint}
        </Text>
      </div>
      <div className='flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-[var(--semi-color-fill-0)]'>
        {icon}
      </div>
    </div>
  </Card>
);

const UsageOverview = ({ data, loading }) => {
  const { t } = useTranslation();
  const summary = data?.summary || {};
  const trend = data?.trend || [];
  const models = data?.models || [];

  const trendSpec = useMemo(() => {
    const values = trend.flatMap((item) => [
      {
        label: item.label,
        source: t('总消耗'),
        amount: quotaToUSD(item.quota),
        quota: item.quota || 0,
        request_count: item.request_count || 0,
      },
      {
        label: item.label,
        source: t('订阅包'),
        amount: quotaToUSD(item.subscription_quota),
        quota: item.subscription_quota || 0,
        request_count: item.subscription_request_count || 0,
      },
      {
        label: item.label,
        source: t('按量计费'),
        amount: quotaToUSD(item.wallet_quota),
        quota: item.wallet_quota || 0,
        request_count: item.wallet_request_count || 0,
      },
      {
        label: item.label,
        source: t('来源未知'),
        amount: quotaToUSD(item.unknown_quota),
        quota: item.unknown_quota || 0,
        request_count: item.unknown_request_count || 0,
      },
    ]);
    return {
      type: 'line',
      data: [{ id: 'usage-source-trend', values }],
      xField: 'label',
      yField: 'amount',
      seriesField: 'source',
      point: { visible: true },
      legends: { visible: true, orient: 'top' },
      title: {
        visible: true,
        text: t('计费来源趋势'),
        subtext: t('总消耗、订阅包、按量和未知来源'),
      },
      axes: [
        { orient: 'bottom', type: 'band' },
        {
          orient: 'left',
          type: 'linear',
          title: { visible: true, text: '$' },
          label: { formatMethod: formatUSDValue },
        },
      ],
      tooltip: {
        mark: {
          content: [
            { key: t('计费来源'), value: (datum) => datum.source },
            {
              key: t('消耗额度 ($)'),
              value: (datum) => formatUSDValue(datum.amount),
            },
            {
              key: t('请求数'),
              value: (datum) => renderNumber(datum.request_count || 0),
            },
          ],
        },
      },
    };
  }, [trend, t]);

  const modelSpec = useMemo(() => {
    const values = models.slice(0, 15).flatMap((item) => [
      {
        model_name: item.model_name || '-',
        source: t('订阅包'),
        amount: quotaToUSD(item.subscription_quota),
      },
      {
        model_name: item.model_name || '-',
        source: t('按量计费'),
        amount: quotaToUSD(item.wallet_quota),
      },
      {
        model_name: item.model_name || '-',
        source: t('来源未知'),
        amount: quotaToUSD(item.unknown_quota),
      },
    ]);
    return {
      type: 'bar',
      data: [{ id: 'usage-model-composition', values }],
      direction: 'horizontal',
      xField: 'amount',
      yField: 'model_name',
      seriesField: 'source',
      stack: true,
      legends: { visible: true, orient: 'top' },
      title: {
        visible: true,
        text: t('模型消耗构成'),
        subtext: t('按计费来源拆分模型额度'),
      },
      axes: [
        {
          orient: 'bottom',
          type: 'linear',
          title: { visible: true, text: '$' },
          label: { formatMethod: formatUSDValue },
        },
        { orient: 'left', type: 'band' },
      ],
      tooltip: {
        mark: {
          content: [
            { key: t('计费来源'), value: (datum) => datum.source },
            {
              key: t('消耗额度 ($)'),
              value: (datum) => formatUSDValue(datum.amount),
            },
          ],
        },
      },
    };
  }, [models, t]);

  if (loading && !data) {
    return <Skeleton placeholder={<Skeleton.Paragraph rows={8} />} active />;
  }

  const hasUsage = trend.some((item) => item.request_count > 0);
  return (
    <div className='flex flex-col gap-4'>
      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <MetricCard
          title={t('总消耗额度')}
          value={formatQuotaUSD(summary.quota || 0)}
          hint={`${t('请求数')} ${renderNumber(summary.request_count || 0)}`}
          icon={
            <CircleDollarSign size={20} color='var(--semi-color-primary)' />
          }
        />
        <MetricCard
          title={t('订阅包活跃人数')}
          value={renderNumber(summary.subscription_active_user_count || 0)}
          hint={t('当前筛选范围内实际使用')}
          icon={<Users size={20} color='var(--semi-color-indigo-5)' />}
        />
        <MetricCard
          title={t('订阅包使用额度')}
          value={formatQuotaUSD(summary.subscription_quota || 0)}
          hint={`${t('请求数')} ${renderNumber(summary.subscription_request_count || 0)}`}
          icon={<CalendarCheck2 size={20} color='var(--semi-color-purple-5)' />}
        />
        <MetricCard
          title={t('按量使用额度')}
          value={formatQuotaUSD(summary.wallet_quota || 0)}
          hint={`${t('请求数')} ${renderNumber(summary.wallet_request_count || 0)}`}
          icon={<Activity size={20} color='var(--semi-color-green-5)' />}
        />
      </div>

      {Number(summary.unknown_quota || 0) > 0 && (
        <Banner
          type='warning'
          fullMode={false}
          title={t('存在来源未知的消费日志')}
          description={`${formatQuotaUSD(summary.unknown_quota)} · ${t('请求数')} ${renderNumber(summary.unknown_request_count || 0)}`}
        />
      )}

      <div className='grid grid-cols-2 gap-x-4 gap-y-3 border-y border-[var(--semi-color-border)] py-3 sm:grid-cols-3 xl:grid-cols-6'>
        {[
          [t('总活跃用户'), renderNumber(summary.active_user_count || 0)],
          [t('总请求数'), renderNumber(summary.request_count || 0)],
          [t('总 Tokens'), formatTokensMillion(summary.total_tokens || 0)],
          [t('模型数量'), renderNumber(models.length || 0)],
          [
            t('1h缓存补贴'),
            formatQuotaUSD(summary.claude_cache_ttl_subsidy_quota || 0),
          ],
          [t('生成时间'), formatDateTime(data?.generated_at)],
        ].map(([label, value]) => (
          <div key={label} className='min-w-0'>
            <Text type='tertiary' size='small'>
              {label}
            </Text>
            <div className='mt-1 truncate font-medium tabular-nums'>
              {value}
            </div>
          </div>
        ))}
      </div>

      <div className='grid grid-cols-1 gap-4 xl:grid-cols-2'>
        <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
          <div className='h-80 md:h-96' aria-label={t('计费来源趋势')}>
            {hasUsage ? (
              <VChart spec={trendSpec} option={CHART_CONFIG} />
            ) : (
              <Empty
                title={t('暂无趋势数据')}
                description={t('调整筛选条件后重新查询')}
              />
            )}
          </div>
        </Card>
        <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
          <div className='h-80 md:h-96' aria-label={t('模型消耗构成')}>
            {models.length > 0 ? (
              <VChart spec={modelSpec} option={CHART_CONFIG} />
            ) : (
              <Empty
                title={t('暂无模型消耗数据')}
                description={t('当前筛选范围内没有消费日志')}
              />
            )}
          </div>
        </Card>
      </div>
    </div>
  );
};

export default UsageOverview;
