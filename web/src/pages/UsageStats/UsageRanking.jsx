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
import { renderNumber } from '../../helpers';
import { formatDateTime, formatQuotaUSD, formatTokensMillion } from './utils';

const { Text, Title } = Typography;

const UsageRanking = ({
  data,
  loading,
  mode,
  onModeChange,
  onUserSelect,
  isMobile,
}) => {
  const { t } = useTranslation();
  const totalItems = data?.ranking || [];
  const subscriptionItems = data?.subscription_ranking || [];
  const items = mode === 'subscription' ? subscriptionItems : totalItems;

  const columns = useMemo(() => {
    const userColumn = {
      title: t('用户'),
      dataIndex: 'username',
      render: (_, record, index) => (
        <div className='min-w-0'>
          <div className='truncate font-medium'>
            <span className='mr-2 tabular-nums text-[var(--semi-color-text-2)]'>
              #{index + 1}
            </span>
            {record.username || '-'}
          </div>
          <Text type='tertiary' size='small'>
            {t('ID')} {record.user_id}
          </Text>
        </div>
      ),
    };
    const quotaColumn = {
      title: mode === 'subscription' ? t('订阅包使用额度') : t('总消耗额度'),
      dataIndex: 'quota',
      sorter: (a, b) => (a.quota || 0) - (b.quota || 0),
      render: (value) => (
        <span className='font-medium tabular-nums'>
          {formatQuotaUSD(value)}
        </span>
      ),
    };
    const requestColumn = {
      title: t('请求数'),
      dataIndex: 'request_count',
      sorter: (a, b) => (a.request_count || 0) - (b.request_count || 0),
      render: (value) => renderNumber(value || 0),
    };
    const lastColumn = {
      title: t('最后请求时间'),
      dataIndex: 'last_request_at',
      render: (value) => formatDateTime(value),
    };
    if (isMobile) return [userColumn, quotaColumn, requestColumn, lastColumn];
    const desktopColumns = [userColumn, quotaColumn];
    if (mode === 'total') {
      desktopColumns.push(
        {
          title: t('订阅包'),
          dataIndex: 'subscription_quota',
          render: (value) => formatQuotaUSD(value || 0),
        },
        {
          title: t('按量计费'),
          dataIndex: 'wallet_quota',
          render: (value) => formatQuotaUSD(value || 0),
        },
        {
          title: t('来源未知'),
          dataIndex: 'unknown_quota',
          render: (value) => formatQuotaUSD(value || 0),
        },
      );
    }
    desktopColumns.push(
      requestColumn,
      {
        title: t('Token 消耗 (M)'),
        dataIndex: 'total_tokens',
        render: (value) => formatTokensMillion(value || 0),
      },
      lastColumn,
    );
    return desktopColumns;
  }, [isMobile, mode, t]);

  return (
    <Card className='!rounded-lg' bodyStyle={{ padding: isMobile ? 12 : 16 }}>
      <div className='mb-3 flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
        <div>
          <Title heading={6} className='!mb-1'>
            {mode === 'subscription' ? t('订阅包消耗排行') : t('用户消耗排行')}
          </Title>
          <Text type='tertiary'>{t('点击用户行查看该用户的模型明细')}</Text>
        </div>
        <Tag color={mode === 'subscription' ? 'violet' : 'blue'} shape='circle'>
          {t('按消耗额度倒序')}
        </Tag>
      </div>
      <Tabs
        activeKey={mode}
        onChange={onModeChange}
        type='button'
        className='mb-3'
      >
        <TabPane itemKey='total' tab={t('总消耗')} />
        <TabPane itemKey='subscription' tab={t('订阅包消耗')} />
      </Tabs>
      <Table
        rowKey='user_id'
        columns={columns}
        dataSource={items}
        loading={loading}
        pagination={{
          pageSize: 20,
          showSizeChanger: !isMobile,
          pageSizeOpts: [10, 20, 50],
        }}
        onRow={(record) => ({
          onClick: () =>
            onUserSelect(
              record,
              mode === 'subscription' ? 'subscription' : 'all',
            ),
          className: 'cursor-pointer',
        })}
        scroll={isMobile ? undefined : { x: 'max-content' }}
        empty={
          <Empty
            title={
              mode === 'subscription'
                ? t('暂无订阅包消耗数据')
                : t('暂无用户消耗数据')
            }
            description={t('当前筛选范围内没有消费日志')}
          />
        }
      />
    </Card>
  );
};

export default UsageRanking;
