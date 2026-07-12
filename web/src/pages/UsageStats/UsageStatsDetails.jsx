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
import {
  Card,
  Empty,
  SideSheet,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  renderNumber,
  renderPaymentAmount,
  renderQuotaWithAmount,
} from '../../helpers';
import {
  CHART_CONFIG,
  formatDateTime,
  formatQuotaUSD,
  formatTokensMillion,
  formatUSDValue,
  normalizeDetailsByUser,
  quotaToUSD,
} from './utils';

const { Text } = Typography;

const InlineMetric = ({ label, value, hint }) => (
  <div className='min-w-0 border-l-2 border-[var(--semi-color-primary)] pl-3'>
    <Text type='tertiary' size='small'>
      {label}
    </Text>
    <div className='mt-1 truncate text-lg font-semibold tabular-nums'>
      {value}
    </div>
    {hint && (
      <Text type='tertiary' size='small'>
        {hint}
      </Text>
    )}
  </div>
);

const UsageStatsDetails = ({
  userState,
  onCloseUser,
  fundingState,
  onCloseFunding,
  onFundingPageChange,
  onFundingPageSizeChange,
  isMobile,
}) => {
  const { t } = useTranslation();
  const userData = userState?.data;
  const userSummary = userData?.summary || {};
  const userRecord = userState?.record;
  const userRank = userData?.ranking?.[0] || userRecord || {};
  const userDetails = userRecord
    ? normalizeDetailsByUser(userData?.user_model_details || [])[
        userRecord.user_id
      ] || []
    : [];

  const userTrendSpec = useMemo(
    () => ({
      type: 'line',
      data: [
        {
          id: 'selected-usage-user-trend',
          values: (userData?.trend || []).map((item) => ({
            label: item.label,
            amount: quotaToUSD(item.quota),
            request_count: item.request_count || 0,
          })),
        },
      ],
      xField: 'label',
      yField: 'amount',
      point: { visible: true },
      title: {
        visible: true,
        text: t('用户总消耗趋势'),
        subtext:
          userState?.source === 'subscription'
            ? t('仅统计订阅包额度')
            : t('按当前筛选条件统计'),
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
    }),
    [t, userData?.trend, userState?.source],
  );

  const userColumns = useMemo(() => {
    const columns = [
      {
        title: t('模型'),
        dataIndex: 'model_name',
        render: (value) => <Tag shape='circle'>{value || '-'}</Tag>,
      },
      {
        title: t('消耗额度 ($)'),
        dataIndex: 'quota',
        sorter: (a, b) => (a.quota || 0) - (b.quota || 0),
        render: formatQuotaUSD,
      },
      {
        title: t('请求数'),
        dataIndex: 'request_count',
        render: (value) => renderNumber(value || 0),
      },
    ];
    if (!isMobile) {
      columns.push(
        {
          title: t('订阅包'),
          dataIndex: 'subscription_quota',
          render: formatQuotaUSD,
        },
        {
          title: t('按量计费'),
          dataIndex: 'wallet_quota',
          render: formatQuotaUSD,
        },
        {
          title: t('Token 消耗 (M)'),
          dataIndex: 'total_tokens',
          render: formatTokensMillion,
        },
      );
    }
    return columns;
  }, [isMobile, t]);

  const isRecharge = fundingState?.mode === 'recharge';
  const fundingRecord = fundingState?.record;
  const fundingPage = isRecharge
    ? fundingState?.data?.recharge_details || {}
    : fundingState?.data?.subscription_purchase_details || {};
  const fundingItems = fundingPage.items || [];
  const fundingColumns = useMemo(() => {
    if (isRecharge) {
      const columns = [
        {
          title: t('订单号'),
          dataIndex: 'trade_no',
          render: (value) => <Text copyable>{value || '-'}</Text>,
        },
        {
          title: t('余额充值实付'),
          dataIndex: 'money',
          render: renderPaymentAmount,
        },
        {
          title: t('完成时间'),
          dataIndex: 'complete_time',
          render: formatDateTime,
        },
      ];
      if (!isMobile) {
        columns.splice(
          1,
          0,
          {
            title: t('支付方式'),
            dataIndex: 'payment_method',
            render: (value) => <Tag shape='circle'>{value || '-'}</Tag>,
          },
          {
            title: t('余额充值额度'),
            dataIndex: 'amount',
            render: renderQuotaWithAmount,
          },
        );
      }
      return columns;
    }
    const columns = [
      {
        title: t('订单号'),
        dataIndex: 'trade_no',
        render: (value) => <Text copyable>{value || '-'}</Text>,
      },
      {
        title: t('订阅套餐'),
        dataIndex: 'plan_title',
        render: (value, record) =>
          value || `${t('套餐')} #${record.plan_id || '-'}`,
      },
      {
        title: t('订阅包购买金额'),
        dataIndex: 'money',
        render: renderPaymentAmount,
      },
      {
        title: t('完成时间'),
        dataIndex: 'complete_time',
        render: formatDateTime,
      },
    ];
    if (!isMobile) {
      columns.splice(2, 0, {
        title: t('支付方式'),
        dataIndex: 'payment_method',
        render: (value) => <Tag shape='circle'>{value || '-'}</Tag>,
      });
    }
    return columns;
  }, [isMobile, isRecharge, t]);

  return (
    <>
      <SideSheet
        title={
          userRecord
            ? `${userState?.source === 'subscription' ? t('订阅包消耗明细') : t('用户模型明细')} · ${userRecord.username || userRecord.user_id}`
            : t('用户模型明细')
        }
        visible={!!userRecord}
        onCancel={onCloseUser}
        width={isMobile ? '100%' : 920}
        placement='right'
      >
        {userRecord && (
          <div className='flex flex-col gap-4'>
            <div className='grid grid-cols-2 gap-4 xl:grid-cols-4'>
              <InlineMetric
                label={t('总消耗额度')}
                value={formatQuotaUSD(
                  userSummary.quota ?? userRecord.quota ?? 0,
                )}
                hint={`${t('请求数')} ${renderNumber(userSummary.request_count ?? userRecord.request_count ?? 0)}`}
              />
              <InlineMetric
                label={t('订阅包使用额度')}
                value={formatQuotaUSD(
                  userSummary.subscription_quota ??
                    userRecord.subscription_quota ??
                    0,
                )}
              />
              <InlineMetric
                label={t('按量使用额度')}
                value={formatQuotaUSD(
                  userSummary.wallet_quota ?? userRecord.wallet_quota ?? 0,
                )}
              />
              <InlineMetric
                label={t('Token 消耗 (M)')}
                value={formatTokensMillion(
                  userSummary.total_tokens ?? userRecord.total_tokens ?? 0,
                )}
                hint={formatDateTime(userRank.last_request_at)}
              />
            </div>
            <Card className='!rounded-lg' bodyStyle={{ padding: 8 }}>
              <div className='h-72 md:h-80'>
                {userState.loading ? (
                  <div className='flex h-full items-center justify-center'>
                    <Text type='tertiary'>{t('加载中')}</Text>
                  </div>
                ) : (userData?.trend || []).some(
                    (item) => item.request_count > 0,
                  ) ? (
                  <VChart spec={userTrendSpec} option={CHART_CONFIG} />
                ) : (
                  <Empty title={t('暂无用户趋势数据')} />
                )}
              </div>
            </Card>
            <Table
              rowKey='key'
              columns={userColumns}
              dataSource={userDetails}
              loading={userState.loading}
              pagination={false}
              scroll={isMobile ? undefined : { x: 'max-content' }}
              empty={<Empty title={t('暂无模型明细')} />}
            />
          </div>
        )}
      </SideSheet>

      <SideSheet
        title={
          fundingRecord
            ? `${isRecharge ? t('用户余额充值详情') : t('用户订阅包购买详情')} · ${fundingRecord.username || fundingRecord.user_id}`
            : isRecharge
              ? t('用户余额充值详情')
              : t('用户订阅包购买详情')
        }
        visible={!!fundingRecord}
        onCancel={onCloseFunding}
        width={isMobile ? '100%' : 980}
        placement='right'
      >
        {fundingRecord && (
          <Table
            rowKey='id'
            columns={fundingColumns}
            dataSource={fundingItems}
            loading={fundingState.loading}
            pagination={{
              currentPage: fundingPage.page || 1,
              pageSize: fundingPage.page_size || 20,
              total: fundingPage.total || 0,
              showSizeChanger: !isMobile,
              pageSizeOpts: [10, 20, 50, 100],
              onPageChange: onFundingPageChange,
              onPageSizeChange: onFundingPageSizeChange,
            }}
            scroll={isMobile ? undefined : { x: 'max-content' }}
            empty={
              <Empty
                title={
                  isRecharge ? t('暂无余额充值订单') : t('暂无订阅包购买订单')
                }
              />
            }
          />
        )}
      </SideSheet>
    </>
  );
};

export default UsageStatsDetails;
