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
import {
  Card,
  Empty,
  Table,
  TabPane,
  Tabs,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  renderNumber,
  renderPaymentAmount,
  renderQuotaWithAmount,
} from '../../helpers';
import { formatDateTime } from './utils';

const { Text, Title } = Typography;

const FundingRecords = ({
  mode,
  onModeChange,
  data,
  loading,
  isMobile,
  onPageChange,
  onPageSizeChange,
  onUserSelect,
}) => {
  const { t } = useTranslation();
  const isRecharge = mode === 'recharge';
  const summary = isRecharge
    ? data?.recharge_summary || {}
    : data?.subscription_purchase_summary || {};
  const pageData = isRecharge
    ? data?.recharge_ranking || {}
    : data?.subscription_purchase_ranking || {};
  const items = pageData.items || [];

  const metrics = isRecharge
    ? [
        [t('余额充值额度'), renderQuotaWithAmount(summary.amount || 0)],
        [t('余额充值实付'), renderPaymentAmount(summary.money || 0)],
        [t('充值用户'), renderNumber(summary.user_count || 0)],
        [t('充值笔数'), renderNumber(summary.order_count || 0)],
        [t('最后充值时间'), formatDateTime(summary.last_topup_at)],
      ]
    : [
        [t('订阅包购买金额'), renderPaymentAmount(summary.money || 0)],
        [t('订阅购买人数'), renderNumber(summary.user_count || 0)],
        [t('订阅购买笔数'), renderNumber(summary.order_count || 0)],
        [t('订阅包数'), renderNumber(summary.plan_count || 0)],
        [t('最后购买时间'), formatDateTime(summary.last_purchase_at)],
      ];

  const columns = useMemo(() => {
    const userColumn = {
      title: t('用户'),
      dataIndex: 'username',
      width: isMobile ? 200 : 180,
      ellipsis: true,
      render: (_, record, index) => (
        <div className='min-w-0'>
          <div className='truncate font-medium'>
            <span className='mr-2 tabular-nums text-[var(--semi-color-text-2)]'>
              #
              {(Number(pageData.page || 1) - 1) *
                Number(pageData.page_size || 20) +
                index +
                1}
            </span>
            {record.username || '-'}
          </div>
          <Text type='tertiary' size='small'>
            {t('ID')} {record.user_id}
          </Text>
        </div>
      ),
    };
    if (isRecharge) {
      const result = [
        userColumn,
        {
          title: t('余额充值实付'),
          dataIndex: 'money',
          width: isMobile ? 130 : 120,
          sorter: (a, b) => (a.money || 0) - (b.money || 0),
          render: (value) => renderPaymentAmount(value || 0),
        },
        {
          title: t('充值笔数'),
          dataIndex: 'order_count',
          width: isMobile ? 90 : 80,
          render: (value) => renderNumber(value || 0),
        },
        {
          title: t('最后充值时间'),
          dataIndex: 'last_topup_at',
          width: 160,
          render: formatDateTime,
        },
      ];
      if (!isMobile) {
        result.splice(2, 0, {
          title: t('余额充值额度'),
          dataIndex: 'amount',
          width: 140,
          render: (value) => renderQuotaWithAmount(value || 0),
        });
      }
      return result;
    }
    const result = [
      userColumn,
      {
        title: t('订阅包购买金额'),
        dataIndex: 'money',
        width: isMobile ? 130 : 120,
        sorter: (a, b) => (a.money || 0) - (b.money || 0),
        render: (value) => renderPaymentAmount(value || 0),
      },
      {
        title: t('订阅购买笔数'),
        dataIndex: 'order_count',
        width: isMobile ? 90 : 80,
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('最后购买时间'),
        dataIndex: 'last_purchase_at',
        width: 160,
        render: formatDateTime,
      },
    ];
    if (!isMobile) {
      result.splice(3, 0, {
        title: t('订阅包数'),
        dataIndex: 'plan_count',
        width: 100,
        render: (value) => renderNumber(value || 0),
      });
    }
    return result;
  }, [isMobile, isRecharge, pageData.page, pageData.page_size, t]);

  return (
    <Card className='!rounded-lg' bodyStyle={{ padding: isMobile ? 12 : 16 }}>
      <div className='mb-3 flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
        <div>
          <Title heading={6} className='!mb-1'>
            {isRecharge ? t('用户余额充值排行') : t('用户订阅包购买排行')}
          </Title>
          <Text type='tertiary'>
            {isRecharge
              ? t('点击用户行查看该用户的充值订单详情')
              : t('点击用户行查看该用户的订阅包购买详情')}
          </Text>
        </div>
        <Tag color={isRecharge ? 'orange' : 'violet'} shape='circle'>
          {isRecharge ? t('按实付金额倒序') : t('已排除管理员作废订单')}
        </Tag>
      </div>
      <Tabs
        activeKey={mode}
        onChange={onModeChange}
        type='button'
        className='mb-3'
      >
        <TabPane itemKey='recharge' tab={t('余额充值')} />
        <TabPane itemKey='subscription_purchase' tab={t('订阅购买')} />
      </Tabs>
      <div className='mb-4 grid grid-cols-2 gap-3 border-y border-[var(--semi-color-border)] py-3 md:grid-cols-5'>
        {metrics.map(([label, value]) => (
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
      <Table
        rowKey='user_id'
        columns={columns}
        dataSource={items}
        loading={loading}
        pagination={{
          currentPage: pageData.page || 1,
          pageSize: pageData.page_size || 20,
          total: pageData.total || 0,
          showSizeChanger: !isMobile,
          pageSizeOpts: [10, 20, 50, 100],
          onPageChange,
          onPageSizeChange,
        }}
        onRow={(record) => ({
          onClick: () => onUserSelect(record, mode),
          className: 'cursor-pointer',
        })}
        scroll={{ x: isMobile ? 580 : '100%' }}
        empty={
          <Empty
            title={
              isRecharge ? t('暂无用户余额充值数据') : t('暂无订阅包购买数据')
            }
            description={
              isRecharge
                ? t('当前筛选范围内没有成功余额充值订单')
                : t('当前筛选范围内没有有效订阅包购买订单')
            }
          />
        }
      />
    </Card>
  );
};

export default FundingRecords;
