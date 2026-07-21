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

import React from 'react';
import {
  Card,
  Col,
  Row,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { formatAction, formatAge, formatPlatform } from './utils';

const { Text, Title } = Typography;

const MetricStrip = ({ items }) => (
  <div className='grid grid-cols-2 overflow-hidden rounded-md border border-solid border-semi-color-border bg-semi-color-bg-2 sm:grid-cols-3 xl:grid-cols-6'>
    {items.map((item) => (
      <div
        key={item.label}
        className='min-h-[76px] border-0 border-b border-r border-solid border-semi-color-border p-3 last:border-r-0 sm:border-b-0'
      >
        <Text type='tertiary' size='small'>
          {item.label}
        </Text>
        <div className='mt-1 text-xl font-semibold tabular-nums'>
          {item.value}
        </div>
      </div>
    ))}
  </div>
);

const WorkerPanel = ({ title, section, webhook, t }) => {
  const { queue, worker } = section;
  const fields = [
    [t('等待中'), queue.pending],
    [t('当前可执行'), queue.due],
    [t('处理中'), queue.processing],
    [t('过期租约'), queue.stale],
    [t('失败'), queue.failed],
    [t('最近一小时完成'), queue.completed_recent],
    [t('最老等待'), formatAge(queue.oldest_due_age_seconds, t)],
    [
      t('平均耗时'),
      t('{{duration}} ms', { duration: worker.average_duration_ms || 0 }),
    ],
  ];
  if (webhook) fields.splice(5, 0, [t('已丢弃'), queue.discarded]);
  return (
    <Card bodyStyle={{ padding: 16 }}>
      <div className='mb-3 flex flex-wrap items-center justify-between gap-2'>
        <Title heading={5} style={{ margin: 0 }}>
          {title}
        </Title>
        <div className='flex items-center gap-2'>
          <Tag color={worker.running ? 'green' : 'grey'}>
            {worker.running ? t('运行中') : t('未运行')}
          </Tag>
          <Tag color={worker.saturated ? 'orange' : 'blue'}>
            {worker.in_flight}/{worker.concurrency}
          </Tag>
        </div>
      </div>
      <div className='grid grid-cols-2 gap-x-4 gap-y-3 md:grid-cols-3 xl:grid-cols-4'>
        {fields.map(([label, value]) => (
          <div key={label} className='min-w-0'>
            <Text type='tertiary' size='small'>
              {label}
            </Text>
            <div className='mt-0.5 truncate font-medium tabular-nums'>
              {value}
            </div>
          </div>
        ))}
      </div>
      <div className='mt-4 flex flex-wrap gap-2 border-0 border-t border-solid border-semi-color-border pt-3'>
        <Tag>
          {t('可用槽位')} {worker.available}
        </Tag>
        {webhook ? (
          <Tag>
            {t('单端点')} {worker.endpoint_concurrency}
          </Tag>
        ) : null}
        <Tag>
          {t('请求超时')} {worker.request_timeout_seconds}s
        </Tag>
        <Tag color={worker.timed_out_since_start > 0 ? 'orange' : 'grey'}>
          {t('启动后超时')} {worker.timed_out_since_start}
        </Tag>
      </div>
    </Card>
  );
};

const OverviewTab = ({ stats, loading }) => {
  const { t } = useTranslation();
  const aggregateColumns = [
    {
      title: t('维度'),
      dataIndex: 'name',
      render: (value) => <Text strong>{value || '-'}</Text>,
    },
    {
      title: t('数量'),
      dataIndex: 'count',
      width: 100,
      render: (value) => <span className='tabular-nums'>{value}</span>,
    },
  ];
  const rows = [
    {
      title: t('按平台'),
      data: stats.by_platform.map((item) => ({
        name: formatPlatform(item.platform),
        count: item.count,
      })),
    },
    {
      title: t('按动作'),
      data: stats.by_action.map((item) => ({
        name: formatAction(item.action, t),
        count: item.count,
      })),
    },
    {
      title: t('按渠道'),
      data: stats.by_channel.map((item) => ({
        name: `#${item.channel_id}`,
        count: item.count,
      })),
    },
  ];
  const metrics = [
    [t('未完成任务'), stats.total_unfinished],
    [t('待超时处理'), stats.timeout_pending],
    [t('生图当前可执行'), stats.image_dispatch.queue.due],
    [t('Webhook 当前可执行'), stats.webhook_delivery.queue.due],
    [
      t('过期租约'),
      stats.image_dispatch.queue.stale + stats.webhook_delivery.queue.stale,
    ],
    [
      t('最近一小时完成'),
      stats.image_dispatch.queue.completed_recent +
        stats.webhook_delivery.queue.completed_recent,
    ],
  ].map(([label, value]) => ({ label, value }));

  return (
    <Spin spinning={loading}>
      <div className='flex min-h-[360px] flex-col gap-3'>
        <MetricStrip items={metrics} />
        <Row gutter={[12, 12]}>
          <Col xs={24} xl={12}>
            <WorkerPanel
              title={t('异步生图分发')}
              section={stats.image_dispatch}
              t={t}
            />
          </Col>
          <Col xs={24} xl={12}>
            <WorkerPanel
              title={t('Webhook 投递')}
              section={stats.webhook_delivery}
              webhook
              t={t}
            />
          </Col>
        </Row>
        <Row gutter={[12, 12]}>
          {rows.map((item) => (
            <Col xs={24} lg={8} key={item.title}>
              <section className='rounded-md border border-solid border-semi-color-border bg-semi-color-bg-2 p-3'>
                <Title heading={6} style={{ margin: '0 0 8px' }}>
                  {item.title}
                </Title>
                <Table
                  columns={aggregateColumns}
                  dataSource={item.data}
                  pagination={false}
                  size='small'
                  rowKey='name'
                />
              </section>
            </Col>
          ))}
        </Row>
      </div>
    </Spin>
  );
};

export default OverviewTab;
