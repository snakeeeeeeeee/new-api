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
import {
  Button,
  Empty,
  Input,
  Pagination,
  Select,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, timestamp2string } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import {
  formatAction,
  formatPlatform,
  statusColor,
  statusLabel,
} from './utils';

const { Text } = Typography;
const DEFAULT_FILTERS = {
  task_id: '',
  user_id: '',
  status: '',
  dispatch_status: '',
  platform: '',
  action: '',
};

const AsyncTasksTab = ({ refreshToken }) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [draftFilters, setDraftFilters] = useState(DEFAULT_FILTERS);
  const [filters, setFilters] = useState(DEFAULT_FILTERS);
  const [items, setItems] = useState([]);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [filtersExpanded, setFiltersExpanded] = useState(false);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      try {
        const response = await API.get('/api/task/async/tasks', {
          params: { p: page, page_size: pageSize, ...filters },
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
  }, [filters, page, pageSize, refreshToken, t]);

  const columns = useMemo(
    () => [
      {
        title: t('任务 ID'),
        dataIndex: 'task',
        width: 220,
        render: (task) => (
          <Text copyable ellipsis={{ showTooltip: true }}>
            {task?.task_id || '-'}
          </Text>
        ),
      },
      {
        title: t('用户'),
        dataIndex: 'task',
        width: 140,
        render: (task) => (
          <div>
            <div>{task?.username || '-'}</div>
            <Text type='tertiary'>#{task?.user_id || 0}</Text>
          </div>
        ),
      },
      {
        title: t('平台 / 动作'),
        dataIndex: 'task',
        width: 170,
        render: (task) => (
          <div>
            <div>
              {formatPlatform(task?.display_platform || task?.platform)}
            </div>
            <Text type='tertiary'>{formatAction(task?.action, t)}</Text>
          </div>
        ),
      },
      {
        title: t('任务状态'),
        dataIndex: 'task',
        width: 110,
        render: (task) => (
          <Tag color={statusColor(task?.status)}>
            {statusLabel(task?.status, t)}
          </Tag>
        ),
      },
      {
        title: t('分发状态'),
        dataIndex: 'dispatch',
        width: 110,
        render: (dispatch) =>
          dispatch ? (
            <Tag color={statusColor(dispatch.status)}>
              {statusLabel(dispatch.status, t)}
            </Tag>
          ) : (
            <Text type='tertiary'>-</Text>
          ),
      },
      {
        title: t('尝试次数'),
        dataIndex: 'dispatch',
        width: 90,
        render: (dispatch) => (
          <span className='tabular-nums'>{dispatch?.attempts || 0}</span>
        ),
      },
      {
        title: t('HTTP'),
        dataIndex: 'dispatch',
        width: 80,
        render: (dispatch) => dispatch?.last_http_status || '-',
      },
      {
        title: t('提交时间'),
        dataIndex: 'task',
        width: 170,
        render: (task) =>
          task?.submit_time ? timestamp2string(task.submit_time) : '-',
      },
      {
        title: t('最近错误'),
        dataIndex: 'dispatch',
        width: 240,
        render: (dispatch) => (
          <Text
            type={dispatch?.last_error ? 'danger' : 'tertiary'}
            ellipsis={{ showTooltip: true }}
          >
            {dispatch?.last_error || '-'}
          </Text>
        ),
      },
    ],
    [t],
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

  return (
    <section className='flex min-h-[420px] flex-col gap-3'>
      <div className='grid grid-cols-1 gap-2 rounded-md border border-solid border-semi-color-border bg-semi-color-bg-2 p-3 sm:grid-cols-2 lg:grid-cols-4 xl:grid-cols-6'>
        <Input
          prefix={t('任务 ID')}
          value={draftFilters.task_id}
          onChange={(value) =>
            setDraftFilters((current) => ({ ...current, task_id: value }))
          }
          showClear
        />
        <Select
          prefix={t('任务状态')}
          value={draftFilters.status}
          onChange={(value) =>
            setDraftFilters((current) => ({ ...current, status: value || '' }))
          }
          showClear
          optionList={[
            'submitted',
            'queued',
            'processing',
            'succeeded',
            'failure',
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
              prefix={t('分发状态')}
              value={draftFilters.dispatch_status}
              onChange={(value) =>
                setDraftFilters((current) => ({
                  ...current,
                  dispatch_status: value || '',
                }))
              }
              showClear
              optionList={[
                'none',
                'pending',
                'processing',
                'delivered',
                'failed',
              ].map((value) => ({
                value,
                label:
                  value === 'none' ? t('无分发记录') : statusLabel(value, t),
              }))}
            />
            <Input
              prefix={t('平台')}
              value={draftFilters.platform}
              onChange={(value) =>
                setDraftFilters((current) => ({ ...current, platform: value }))
              }
              showClear
            />
            <Input
              prefix={t('动作')}
              value={draftFilters.action}
              onChange={(value) =>
                setDraftFilters((current) => ({ ...current, action: value }))
              }
              showClear
            />
          </>
        ) : null}
        <div className='flex gap-2 sm:col-span-2 lg:col-span-4 xl:col-span-6'>
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
          rowKey={(record) => record.task?.task_id}
          loading={loading}
          pagination={false}
          size='small'
          scroll={{ x: 'max-content' }}
          empty={<Empty description={t('暂无异步任务')} />}
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
    </section>
  );
};

export default AsyncTasksTab;
