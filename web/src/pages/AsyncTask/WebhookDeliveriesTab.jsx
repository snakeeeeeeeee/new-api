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

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import {
  Button,
  DatePicker,
  Empty,
  Input,
  Pagination,
  Popconfirm,
  Select,
  SideSheet,
  Spin,
  Table,
  Tabs,
  TabPane,
  Tag,
  Timeline,
  Typography,
} from '@douyinfe/semi-ui';
import { IconEyeOpened, IconRefresh } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import { statusColor, statusLabel } from './utils';

const { Text, Title } = Typography;
const DEFAULT_FILTERS = {
  delivery_id: '',
  user_id: '',
  status: '',
  event_type: '',
  http_status: '',
  date_range: [],
};

const DetailField = ({ label, value, copyable }) => (
  <div className='grid grid-cols-[112px_minmax(0,1fr)] gap-3 border-0 border-b border-solid border-semi-color-border pb-2'>
    <Text type='tertiary'>{label}</Text>
    <Text copyable={copyable} ellipsis={{ showTooltip: true }}>
      {value || '-'}
    </Text>
  </div>
);

const WebhookDeliveriesTab = ({ refreshToken }) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [draftFilters, setDraftFilters] = useState(DEFAULT_FILTERS);
  const [filters, setFilters] = useState(DEFAULT_FILTERS);
  const [items, setItems] = useState([]);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [detailVisible, setDetailVisible] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState(null);
  const [detailID, setDetailID] = useState('');
  const [retryingID, setRetryingID] = useState('');
  const [listVersion, setListVersion] = useState(0);
  const [filtersExpanded, setFiltersExpanded] = useState(false);
  const detailRefreshReadyRef = useRef(false);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      try {
        const params = { p: page, page_size: pageSize, ...filters };
        delete params.date_range;
        if (filters.date_range?.length === 2) {
          const startTimestamp = Math.floor(
            new Date(filters.date_range[0]).getTime() / 1000,
          );
          const endTimestamp = Math.floor(
            new Date(filters.date_range[1]).getTime() / 1000,
          );
          if (Number.isFinite(startTimestamp))
            params.start_timestamp = startTimestamp;
          if (Number.isFinite(endTimestamp))
            params.end_timestamp = endTimestamp;
        }
        const response = await API.get('/api/task/async/webhook-deliveries', {
          params,
        });
        if (!cancelled && response.data.success) {
          setItems(response.data.data?.items || []);
          setTotal(response.data.data?.total || 0);
        } else if (!cancelled) {
          showError(response.data.message || t('加载失败'));
        }
      } catch {
        if (!cancelled) showError(t('加载失败'));
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    load();
    return () => {
      cancelled = true;
    };
  }, [filters, listVersion, page, pageSize, refreshToken, t]);

  const loadDetail = useCallback(
    async (deliveryID, { showLoading = true, quiet = false } = {}) => {
      if (showLoading) setDetailLoading(true);
      try {
        const response = await API.get(
          `/api/task/async/webhook-deliveries/${deliveryID}`,
        );
        if (response.data.success) setDetail(response.data.data);
        else if (!quiet) showError(response.data.message || t('加载失败'));
      } catch {
        if (!quiet) showError(t('加载失败'));
      } finally {
        if (showLoading) setDetailLoading(false);
      }
    },
    [t],
  );

  const openDetail = async (deliveryID) => {
    detailRefreshReadyRef.current = false;
    setDetailID(deliveryID);
    setDetailVisible(true);
    await loadDetail(deliveryID);
  };

  useEffect(() => {
    if (!detailVisible || !detailID) return;
    if (!detailRefreshReadyRef.current) {
      detailRefreshReadyRef.current = true;
      return;
    }
    loadDetail(detailID, { showLoading: false, quiet: true });
  }, [detailID, detailVisible, loadDetail, refreshToken]);

  const closeDetail = () => {
    detailRefreshReadyRef.current = false;
    setDetailVisible(false);
    setDetailID('');
    setDetail(null);
  };

  const retryDelivery = async (deliveryID) => {
    setRetryingID(deliveryID);
    try {
      const response = await API.post(
        `/api/task/async/webhook-deliveries/${deliveryID}/retry`,
      );
      if (!response.data.success) {
        showError(response.data.message || t('重试失败'));
        return;
      }
      showSuccess(t('已重新加入投递队列'));
      setListVersion((value) => value + 1);
      if (detailVisible) await loadDetail(deliveryID);
    } catch (error) {
      showError(error.response?.data?.message || t('重试失败'));
    } finally {
      setRetryingID('');
    }
  };

  const retryButton = (record, compact = false) => {
    if (!['failed', 'discarded'].includes(record?.status)) return null;
    return (
      <Popconfirm
        title={t('确认重新投递？')}
        content={t('该投递会重置尝试次数并立即重新进入队列。')}
        okText={t('确认')}
        cancelText={t('取消')}
        onConfirm={() => retryDelivery(record.delivery_id)}
      >
        <Button
          icon={<IconRefresh />}
          loading={retryingID === record.delivery_id}
          aria-label={t('重新投递')}
          theme={compact ? 'borderless' : 'light'}
        >
          {compact ? null : t('重新投递')}
        </Button>
      </Popconfirm>
    );
  };

  const columns = useMemo(
    () => [
      {
        title: t('投递 ID'),
        dataIndex: 'delivery_id',
        width: 220,
        render: (value) => (
          <Text copyable ellipsis={{ showTooltip: true }}>
            {value}
          </Text>
        ),
      },
      {
        title: t('事件'),
        width: 210,
        render: (_, record) => (
          <div>
            <div>{record.event_type}</div>
            <Text type='tertiary' ellipsis={{ showTooltip: true }}>
              {record.event_id}
            </Text>
          </div>
        ),
      },
      {
        title: t('用户'),
        width: 140,
        render: (_, record) => (
          <div>
            <div>{record.username || '-'}</div>
            <Text type='tertiary'>#{record.user_id}</Text>
          </div>
        ),
      },
      {
        title: t('端点'),
        dataIndex: 'endpoint_url',
        width: 240,
        render: (value) => (
          <Text ellipsis={{ showTooltip: true }}>{value}</Text>
        ),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        width: 100,
        render: (value) => (
          <Tag color={statusColor(value)}>{statusLabel(value, t)}</Tag>
        ),
      },
      { title: t('尝试次数'), dataIndex: 'attempts', width: 90 },
      {
        title: t('HTTP'),
        dataIndex: 'last_http_status',
        width: 80,
        render: (value) => value || '-',
      },
      {
        title: t('创建时间'),
        dataIndex: 'created_at',
        width: 170,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('最近错误'),
        dataIndex: 'last_error',
        width: 240,
        render: (value) => (
          <Text
            type={value ? 'danger' : 'tertiary'}
            ellipsis={{ showTooltip: true }}
          >
            {value || '-'}
          </Text>
        ),
      },
      {
        title: t('操作'),
        fixed: 'right',
        width: 96,
        render: (_, record) => (
          <div className='flex items-center'>
            <Button
              theme='borderless'
              icon={<IconEyeOpened />}
              aria-label={t('查看详情')}
              onClick={() => openDetail(record.delivery_id)}
            />
            {retryButton(record, true)}
          </div>
        ),
      },
    ],
    [retryingID, t],
  );

  const applyFilters = () => {
    setPage(1);
    setFilters({ ...draftFilters });
  };
  const resetFilters = () => {
    setDraftFilters(DEFAULT_FILTERS);
    setFilters(DEFAULT_FILTERS);
    setPage(1);
  };
  const selected = detail?.delivery;

  return (
    <section className='flex min-h-[420px] flex-col gap-3'>
      <div className='grid grid-cols-1 gap-2 rounded-md border border-solid border-semi-color-border bg-semi-color-bg-2 p-3 sm:grid-cols-2 lg:grid-cols-5 xl:grid-cols-7'>
        <Input
          prefix={t('投递 ID')}
          value={draftFilters.delivery_id}
          onChange={(value) =>
            setDraftFilters((current) => ({ ...current, delivery_id: value }))
          }
          showClear
        />
        <Select
          prefix={t('状态')}
          value={draftFilters.status}
          onChange={(value) =>
            setDraftFilters((current) => ({ ...current, status: value || '' }))
          }
          showClear
          optionList={[
            'pending',
            'processing',
            'delivered',
            'failed',
            'discarded',
          ].map((value) => ({ value, label: statusLabel(value, t) }))}
        />
        {!isMobile || filtersExpanded ? (
          <>
            <Input
              prefix={t('用户 ID')}
              value={draftFilters.user_id}
              onChange={(value) =>
                setDraftFilters((current) => ({ ...current, user_id: value }))
              }
              showClear
            />
            <Select
              prefix={t('事件类型')}
              value={draftFilters.event_type}
              onChange={(value) =>
                setDraftFilters((current) => ({
                  ...current,
                  event_type: value || '',
                }))
              }
              showClear
              optionList={[
                'image.task.succeeded',
                'image.task.failed',
                'webhook.test',
              ].map((value) => ({ value, label: value }))}
            />
            <Input
              prefix={t('HTTP')}
              value={draftFilters.http_status}
              onChange={(value) =>
                setDraftFilters((current) => ({
                  ...current,
                  http_status: value,
                }))
              }
              showClear
            />
            <DatePicker
              className='w-full lg:col-span-2'
              type='dateTimeRange'
              value={draftFilters.date_range}
              onChange={(value) =>
                setDraftFilters((current) => ({
                  ...current,
                  date_range: value || [],
                }))
              }
              placeholder={[t('开始时间'), t('结束时间')]}
              showClear
            />
          </>
        ) : null}
        <div className='flex gap-2 sm:col-span-2 lg:col-span-5 xl:col-span-7'>
          <Button type='primary' onClick={applyFilters}>
            {t('查询')}
          </Button>
          <Button onClick={resetFilters}>{t('重置')}</Button>
          {isMobile ? (
            <Button onClick={() => setFiltersExpanded((value) => !value)}>
              {t('更多筛选')}
            </Button>
          ) : null}
        </div>
      </div>

      <div className='max-w-full overflow-x-auto rounded-md border border-solid border-semi-color-border'>
        <Table
          columns={columns}
          dataSource={items}
          rowKey='delivery_id'
          loading={loading}
          pagination={false}
          size='small'
          scroll={{ x: 'max-content' }}
          empty={<Empty description={t('暂无 Webhook 投递')} />}
        />
      </div>
      <div className='flex justify-end'>
        <Pagination
          currentPage={page}
          pageSize={pageSize}
          total={total}
          showSizeChanger={!isMobile}
          pageSizeOptions={[10, 20, 50, 100]}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPage(1);
            setPageSize(size);
          }}
        />
      </div>

      <SideSheet
        placement='right'
        title={t('Webhook 投递详情')}
        visible={detailVisible}
        onCancel={closeDetail}
        width='min(720px, 100vw)'
        footer={null}
      >
        <Spin spinning={detailLoading}>
          {selected ? (
            <div className='flex min-h-[300px] flex-col gap-4'>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <div className='flex items-center gap-2'>
                  <Tag color={statusColor(selected.status)}>
                    {statusLabel(selected.status, t)}
                  </Tag>
                  <Tag>
                    {selected.attempts} {t('次尝试')}
                  </Tag>
                </div>
                {retryButton(selected)}
              </div>
              <div className='flex flex-col gap-2'>
                <DetailField
                  label={t('投递 ID')}
                  value={selected.delivery_id}
                  copyable
                />
                <DetailField
                  label={t('事件 ID')}
                  value={selected.event_id}
                  copyable
                />
                <DetailField
                  label={t('事件类型')}
                  value={selected.event_type}
                />
                <DetailField
                  label={t('用户')}
                  value={`${selected.username || '-'} (#${selected.user_id})`}
                />
                <DetailField
                  label={t('端点')}
                  value={selected.endpoint_url}
                  copyable
                />
                <DetailField
                  label={t('HTTP')}
                  value={selected.last_http_status || '-'}
                />
                <DetailField
                  label={t('最近错误')}
                  value={selected.last_error}
                />
              </div>

              <Tabs type='line'>
                <TabPane tab={t('请求 Payload')} itemKey='payload'>
                  <pre className='mt-3 max-h-[420px] overflow-auto rounded-md bg-semi-color-fill-0 p-3 text-xs leading-5'>
                    {JSON.stringify(detail.payload, null, 2)}
                  </pre>
                </TabPane>
                <TabPane tab={t('尝试时间线')} itemKey='attempts'>
                  <div className='pt-4'>
                    {detail.attempts?.length ? (
                      <Timeline mode='left'>
                        {detail.attempts.map((attempt) => (
                          <Timeline.Item
                            key={attempt.attempt_id}
                            time={timestamp2string(attempt.created_at)}
                            type={
                              attempt.http_status >= 200 &&
                              attempt.http_status < 300
                                ? 'success'
                                : 'error'
                            }
                          >
                            <div className='flex flex-col gap-1'>
                              <Title heading={6} style={{ margin: 0 }}>
                                {t('第 {{count}} 次尝试', {
                                  count: attempt.attempt_number,
                                })}
                              </Title>
                              <Text>
                                {t('HTTP')} {attempt.http_status || '-'} ·{' '}
                                {t('{{duration}} ms', {
                                  duration: attempt.duration_ms,
                                })}
                              </Text>
                              {attempt.error ? (
                                <Text type='danger'>{attempt.error}</Text>
                              ) : null}
                              {attempt.response_body ? (
                                <pre className='overflow-auto rounded-md bg-semi-color-fill-0 p-2 text-xs'>
                                  {attempt.response_body}
                                </pre>
                              ) : null}
                            </div>
                          </Timeline.Item>
                        ))}
                      </Timeline>
                    ) : (
                      <Empty description={t('暂无尝试记录')} />
                    )}
                  </div>
                </TabPane>
              </Tabs>
            </div>
          ) : (
            <Empty description={t('暂无投递详情')} />
          )}
        </Spin>
      </SideSheet>
    </section>
  );
};

export default WebhookDeliveriesTab;
