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
import {
  Banner,
  Button,
  Card,
  DatePicker,
  Empty,
  Input,
  InputNumber,
  Popconfirm,
  Select,
  SideSheet,
  Space,
  Spin,
  Switch,
  TabPane,
  Tabs,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Copy,
  Download,
  Eye,
  RefreshCw,
  Save,
  Search,
  Trash2,
} from 'lucide-react';
import CardPro from '../../components/common/ui/CardPro';
import CardTable from '../../components/common/ui/CardTable';
import { DATE_RANGE_PRESETS } from '../../constants/console.constants';
import {
  API,
  copy,
  createCardProPagination,
  showError,
  showSuccess,
} from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const defaultSettings = {
  enabled: false,
  ttl_minutes: 60,
  max_storage_mib: 256,
  max_files: 1000,
  priority_user_ids: [],
  priority_channel_ids: [],
};

const defaultFilters = {
  dateRange: [],
  request_id: '',
  user_id: undefined,
  channel_id: undefined,
  error_keyword: '',
};

const formatTime = (timestamp) => {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
};

const formatBytes = (bytes) => {
  const value = Number(bytes) || 0;
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`;
  if (value < 1024 * 1024 * 1024) {
    return `${(value / 1024 / 1024).toFixed(1)} MiB`;
  }
  return `${(value / 1024 / 1024 / 1024).toFixed(1)} GiB`;
};

const renderJSON = (value) => JSON.stringify(value ?? {}, null, 2);

const formatBody = (body) => {
  if (!body) return '';
  try {
    return JSON.stringify(JSON.parse(body), null, 2);
  } catch {
    return body;
  }
};

const optionForID = (id, prefix) => ({
  value: Number(id),
  label: `${prefix} #${id}`,
});

const mergeOptions = (current, additions) => {
  const merged = new Map(
    current.map((option) => [Number(option.value), option]),
  );
  additions.forEach((option) => merged.set(Number(option.value), option));
  return [...merged.values()];
};

const outcomeTag = (outcome, t) => {
  const values = {
    pending: { color: 'grey', label: t('处理中') },
    fallback_succeeded: { color: 'green', label: t('Fallback 成功') },
    final_failure: { color: 'red', label: t('最终失败') },
    stream_incomplete: { color: 'orange', label: t('流不完整') },
    client_disconnected: { color: 'grey', label: t('客户端已断开') },
  };
  const value = values[outcome] || { color: 'grey', label: outcome || '-' };
  return <Tag color={value.color}>{value.label}</Tag>;
};

const DetailField = ({ label, value, mono = false }) => (
  <div className='min-w-0 border-0 border-b border-solid border-semi-color-border pb-2'>
    <div className='mb-1 text-xs text-semi-color-text-2'>{label}</div>
    <div
      className={`break-words text-sm text-semi-color-text-0 ${mono ? 'break-all font-mono text-[13px]' : ''}`}
    >
      {value === undefined || value === null || value === '' ? '-' : value}
    </div>
  </div>
);

const JSONSection = ({ title, value, emptyText }) => (
  <section className='min-w-0'>
    <Text strong>{title}</Text>
    {value && Object.keys(value).length > 0 ? (
      <pre className='mt-2 max-h-[360px] overflow-auto whitespace-pre-wrap break-all rounded-md bg-semi-color-fill-0 p-3 font-mono text-xs leading-5'>
        {renderJSON(value)}
      </pre>
    ) : (
      <div className='mt-2 rounded-md bg-semi-color-fill-0 p-3'>
        <Text type='tertiary'>{emptyText}</Text>
      </div>
    )}
  </section>
);

const BodySection = ({
  title,
  fragment,
  missingText,
  truncatedText,
  hashLabel,
}) => {
  if (!fragment) {
    return (
      <section>
        <Text strong>{title}</Text>
        <div className='mt-2 rounded-md bg-semi-color-fill-0 p-3'>
          <Text type='tertiary'>{missingText}</Text>
        </div>
      </section>
    );
  }
  return (
    <section className='min-w-0'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <Text strong>{title}</Text>
        <Space wrap>
          {fragment.content_type ? <Tag>{fragment.content_type}</Tag> : null}
          <Tag>{formatBytes(fragment.original_size)}</Tag>
          {fragment.truncated ? (
            <Tag color='orange'>{truncatedText}</Tag>
          ) : null}
        </Space>
      </div>
      {fragment.skip_reason ? (
        <Banner
          className='mt-2'
          type='warning'
          closeIcon={null}
          description={fragment.skip_reason}
        />
      ) : null}
      {fragment.body ? (
        <pre className='mt-2 max-h-[420px] overflow-auto whitespace-pre-wrap break-all rounded-md bg-semi-color-fill-0 p-3 font-mono text-xs leading-5'>
          {formatBody(fragment.body)}
        </pre>
      ) : null}
      {fragment.sha256 ? (
        <div className='mt-2 break-all font-mono text-xs text-semi-color-text-2'>
          {hashLabel}: {fragment.sha256}
        </div>
      ) : null}
    </section>
  );
};

const ErrorSnapshots = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [status, setStatus] = useState(null);
  const [settings, setSettings] = useState(defaultSettings);
  const [statusLoading, setStatusLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [snapshots, setSnapshots] = useState([]);
  const [total, setTotal] = useState(0);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [listLoading, setListLoading] = useState(false);
  const [filters, setFilters] = useState(defaultFilters);
  const [appliedFilters, setAppliedFilters] = useState(defaultFilters);
  const [userOptions, setUserOptions] = useState([]);
  const [channelOptions, setChannelOptions] = useState([]);
  const [detailVisible, setDetailVisible] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detail, setDetail] = useState(null);
  const [cleanupLoading, setCleanupLoading] = useState(false);
  const [clearLoading, setClearLoading] = useState(false);

  const loadStatus = async () => {
    setStatusLoading(true);
    try {
      const res = await API.get('/api/request_dump/error_snapshots/status');
      if (!res.data.success) {
        showError(res.data.message || t('加载失败'));
        return;
      }
      const next = res.data.data || {};
      const nextSettings = { ...defaultSettings, ...(next.settings || {}) };
      setStatus(next);
      setSettings(nextSettings);
      setUserOptions((current) =>
        mergeOptions(
          current,
          nextSettings.priority_user_ids.map((id) =>
            optionForID(id, t('用户')),
          ),
        ),
      );
      setChannelOptions((current) =>
        mergeOptions(
          current,
          nextSettings.priority_channel_ids.map((id) =>
            optionForID(id, t('渠道')),
          ),
        ),
      );
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setStatusLoading(false);
    }
  };

  const buildListParams = (nextFilters, page, size) => {
    const params = { p: page, page_size: size };
    ['request_id', 'user_id', 'channel_id', 'error_keyword'].forEach((key) => {
      const value = nextFilters[key];
      if (value !== '' && value !== undefined && value !== null) {
        params[key] = value;
      }
    });
    if (
      Array.isArray(nextFilters.dateRange) &&
      nextFilters.dateRange.length === 2
    ) {
      params.start_timestamp = Math.floor(
        new Date(nextFilters.dateRange[0]).getTime() / 1000,
      );
      params.end_timestamp = Math.floor(
        new Date(nextFilters.dateRange[1]).getTime() / 1000,
      );
    }
    return params;
  };

  const loadSnapshots = async (
    page = activePage,
    size = pageSize,
    nextFilters = appliedFilters,
  ) => {
    setListLoading(true);
    try {
      const res = await API.get('/api/request_dump/error_snapshots', {
        params: buildListParams(nextFilters, page, size),
      });
      if (!res.data.success) {
        showError(res.data.message || t('加载失败'));
        return;
      }
      const data = res.data.data || {};
      setSnapshots(data.items || []);
      setTotal(data.total || 0);
      setActivePage(data.page || page);
      setPageSize(data.page_size || size);
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setListLoading(false);
    }
  };

  const searchUsers = async (keyword = '') => {
    try {
      const res = await API.get(
        '/api/request_dump/error_snapshots/select_options',
        {
          params: { type: 'user', keyword },
        },
      );
      if (!res.data.success) return;
      const options = (res.data.data || []).map((user) => ({
        value: user.id,
        label: `${user.username}${
          user.display_name && user.display_name !== user.username
            ? ` / ${user.display_name}`
            : ''
        } (#${user.id})`,
      }));
      setUserOptions((current) => mergeOptions(current, options));
    } catch {
      // The selector is optional; saved IDs remain visible and removable.
    }
  };

  const searchChannels = async (keyword = '') => {
    try {
      const res = await API.get(
        '/api/request_dump/error_snapshots/select_options',
        {
          params: { type: 'channel', keyword },
        },
      );
      if (!res.data.success) return;
      const options = (res.data.data || []).map((channel) => ({
        value: channel.id,
        label: `${channel.name} (#${channel.id})`,
      }));
      setChannelOptions((current) => mergeOptions(current, options));
    } catch {
      // The selector is optional; saved IDs remain visible and removable.
    }
  };

  useEffect(() => {
    loadStatus();
    loadSnapshots(1, 20, defaultFilters);
    searchUsers();
    searchChannels();
  }, []);

  const saveSettings = async () => {
    setSaving(true);
    try {
      const res = await API.put(
        '/api/request_dump/error_snapshots/settings',
        settings,
      );
      if (!res.data.success) {
        showError(res.data.message || t('保存失败'));
        return;
      }
      const next = res.data.data || {};
      setStatus(next);
      setSettings({ ...defaultSettings, ...(next.settings || {}) });
      showSuccess(t('错误快照配置已保存'));
      loadSnapshots(1, pageSize, appliedFilters);
    } catch (error) {
      showError(error.message || t('保存失败'));
    } finally {
      setSaving(false);
    }
  };

  const submitFilters = () => {
    const next = { ...filters };
    setAppliedFilters(next);
    loadSnapshots(1, pageSize, next);
  };

  const resetFilters = () => {
    setFilters(defaultFilters);
    setAppliedFilters(defaultFilters);
    loadSnapshots(1, pageSize, defaultFilters);
  };

  const openDetail = async (snapshot) => {
    setDetailVisible(true);
    setDetailLoading(true);
    setDetail({ snapshot, payload: null });
    try {
      const res = await API.get(
        `/api/request_dump/error_snapshots/${snapshot.id}`,
      );
      if (!res.data.success) {
        showError(res.data.message || t('加载失败'));
        return;
      }
      setDetail(res.data.data || null);
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setDetailLoading(false);
    }
  };

  const downloadSnapshot = async (snapshot) => {
    try {
      const res = await API.get(
        `/api/request_dump/error_snapshots/${snapshot.id}/download`,
        { responseType: 'blob' },
      );
      const url = URL.createObjectURL(res.data);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = `${snapshot.id}.json.gz`;
      document.body.appendChild(anchor);
      anchor.click();
      document.body.removeChild(anchor);
      URL.revokeObjectURL(url);
      showSuccess(t('快照已下载'));
    } catch (error) {
      showError(error.message || t('下载失败'));
    }
  };

  const deleteSnapshot = async (snapshot) => {
    try {
      const res = await API.delete(
        `/api/request_dump/error_snapshots/${snapshot.id}`,
      );
      if (!res.data.success) {
        showError(res.data.message || t('删除失败'));
        return;
      }
      if (detail?.snapshot?.id === snapshot.id) {
        setDetailVisible(false);
        setDetail(null);
      }
      showSuccess(t('快照已删除'));
      await Promise.all([
        loadStatus(),
        loadSnapshots(activePage, pageSize, appliedFilters),
      ]);
    } catch (error) {
      showError(error.message || t('删除失败'));
    }
  };

  const cleanupSnapshots = async () => {
    setCleanupLoading(true);
    try {
      const res = await API.post('/api/request_dump/error_snapshots/cleanup');
      if (!res.data.success) {
        showError(res.data.message || t('清理失败'));
        return;
      }
      setStatus(res.data.data || null);
      showSuccess(t('错误快照清理完成'));
      loadSnapshots(1, pageSize, appliedFilters);
    } catch (error) {
      showError(error.message || t('清理失败'));
    } finally {
      setCleanupLoading(false);
    }
  };

  const clearSnapshots = async () => {
    setClearLoading(true);
    try {
      const res = await API.delete('/api/request_dump/error_snapshots');
      if (!res.data.success) {
        showError(res.data.message || t('清空失败'));
        return;
      }
      showSuccess(t('错误快照已清空'));
      await Promise.all([
        loadStatus(),
        loadSnapshots(1, pageSize, appliedFilters),
      ]);
    } catch (error) {
      showError(error.message || t('清空失败'));
    } finally {
      setClearLoading(false);
    }
  };

  const columns = useMemo(
    () => [
      {
        title: t('时间'),
        dataIndex: 'created_at',
        width: 170,
        render: (value) => formatTime(value),
      },
      {
        title: t('Request ID'),
        dataIndex: 'request_id',
        width: 210,
        render: (value) => (
          <div className='flex min-w-0 items-center gap-1'>
            <Tooltip content={value || '-'}>
              <Text
                ellipsis={{ showTooltip: false }}
                className='max-w-[160px] font-mono text-xs'
              >
                {value || '-'}
              </Text>
            </Tooltip>
            {value ? (
              <Tooltip content={t('复制 Request ID')}>
                <Button
                  size='small'
                  theme='borderless'
                  icon={<Copy size={13} />}
                  aria-label={t('复制 Request ID')}
                  onClick={(event) => {
                    event.stopPropagation();
                    copy(value).then((ok) => ok && showSuccess(t('已复制')));
                  }}
                />
              </Tooltip>
            ) : null}
          </div>
        ),
      },
      {
        title: t('用户'),
        dataIndex: 'username',
        width: 150,
        render: (value, record) => `${value || '-'} (#${record.user_id || 0})`,
      },
      {
        title: t('渠道'),
        dataIndex: 'channel_name',
        width: 170,
        render: (value, record) =>
          record.channel_id ? `${value || '-'} (#${record.channel_id})` : '-',
      },
      {
        title: t('模型'),
        dataIndex: 'model_name',
        width: 170,
        render: (value) => value || '-',
      },
      {
        title: t('错误'),
        dataIndex: 'error_message',
        width: 300,
        render: (value, record) => (
          <div className='min-w-0'>
            <div className='mb-1 flex flex-wrap gap-1'>
              <Tag color='red'>{record.status_code || 0}</Tag>
              {record.error_code ? <Tag>{record.error_code}</Tag> : null}
            </div>
            <Text ellipsis={{ showTooltip: true, rows: 2 }}>
              {value || '-'}
            </Text>
          </div>
        ),
      },
      {
        title: t('尝试'),
        dataIndex: 'retry_index',
        width: 90,
        render: (value, record) => (
          <Space vertical align='start' spacing={2}>
            <Text>#{Number(value || 0) + 1}</Text>
            {record.internal_retry ? (
              <Tag size='small'>{t('内部重试')}</Tag>
            ) : null}
          </Space>
        ),
      },
      {
        title: t('采集级别'),
        dataIndex: 'capture_level',
        width: 105,
        render: (value) => (
          <Tag color={value === 'priority' ? 'orange' : 'blue'}>
            {value === 'priority' ? t('重点') : t('摘要')}
          </Tag>
        ),
      },
      {
        title: t('最终结果'),
        dataIndex: 'final_outcome',
        width: 120,
        render: (value) => outcomeTag(value, t),
      },
      {
        title: t('操作'),
        key: 'actions',
        fixed: 'right',
        width: 142,
        render: (_, record) => (
          <Space spacing={2}>
            <Tooltip content={t('查看详情')}>
              <Button
                size='small'
                theme='borderless'
                icon={<Eye size={15} />}
                aria-label={t('查看详情')}
                onClick={(event) => {
                  event.stopPropagation();
                  openDetail(record);
                }}
              />
            </Tooltip>
            <Tooltip content={t('下载')}>
              <Button
                size='small'
                theme='borderless'
                icon={<Download size={15} />}
                aria-label={t('下载')}
                onClick={(event) => {
                  event.stopPropagation();
                  downloadSnapshot(record);
                }}
              />
            </Tooltip>
            <Popconfirm
              title={t('确认删除此错误快照？')}
              content={t('索引和压缩文件将同时删除，且无法恢复。')}
              okText={t('确认')}
              cancelText={t('取消')}
              onConfirm={() => deleteSnapshot(record)}
            >
              <Tooltip content={t('删除')}>
                <Button
                  size='small'
                  type='danger'
                  theme='borderless'
                  icon={<Trash2 size={15} />}
                  aria-label={t('删除')}
                  onClick={(event) => event.stopPropagation()}
                />
              </Tooltip>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [detail?.snapshot?.id, activePage, pageSize, appliedFilters],
  );

  const storage = status?.storage || {};
  const payload = detail?.payload || {};
  const selectedSnapshot = detail?.snapshot || null;
  const metricItems = [
    [t('启用状态'), settings.enabled ? t('已启用') : t('已关闭')],
    [
      t('磁盘使用量'),
      `${formatBytes(storage.total_bytes)} / ${settings.max_storage_mib} MiB`,
    ],
    [t('文件数'), `${storage.file_count || 0} / ${settings.max_files}`],
    [t('最旧快照'), formatTime(storage.oldest_at)],
    [t('丢弃数'), status?.dropped_count || 0],
    [t('最近清理'), formatTime(status?.last_cleanup_at)],
  ];

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-col gap-2 md:flex-row md:items-center md:justify-between'>
        <div>
          <Title heading={4} className='!mb-1'>
            {t('自动错误快照')}
          </Title>
          <Text type='tertiary'>
            {t('自动记录每次失败 attempt，包括最终被 fallback 掩盖的错误')}
          </Text>
        </div>
        <Space wrap>
          <Tag color={settings.enabled ? 'green' : 'grey'} size='large'>
            {settings.enabled ? t('已启用') : t('已关闭')}
          </Tag>
          <Button
            icon={<RefreshCw size={15} />}
            loading={statusLoading || listLoading}
            onClick={() => {
              loadStatus();
              loadSnapshots(activePage, pageSize, appliedFilters);
            }}
          >
            {t('刷新')}
          </Button>
        </Space>
      </div>

      <Banner
        type='info'
        closeIcon={null}
        description={t(
          '普通错误只采集诊断摘要，不保存 Prompt；重点用户或渠道会额外保存脱敏后的客户端和上游请求。所有快照都受 TTL、容量、文件数和 128 KiB 单文件上限约束。',
        )}
      />

      {status?.last_error ? (
        <Banner
          type='danger'
          closeIcon={null}
          description={`${t('最近存储错误')}: ${status.last_error}`}
        />
      ) : null}

      <Card className='!rounded-lg'>
        <Spin spinning={statusLoading}>
          <div className='grid grid-cols-2 gap-x-4 gap-y-3 lg:grid-cols-3 xl:grid-cols-6'>
            {metricItems.map(([label, value]) => (
              <div key={label} className='min-w-0'>
                <div className='text-xs text-semi-color-text-2'>{label}</div>
                <div className='mt-1 break-words text-sm font-semibold text-semi-color-text-0'>
                  {value}
                </div>
              </div>
            ))}
          </div>
        </Spin>
      </Card>

      <Card className='!rounded-lg'>
        <div className='mb-4 flex flex-col gap-2 md:flex-row md:items-start md:justify-between'>
          <div>
            <Text strong>{t('采集与保留配置')}</Text>
            <div className='mt-1'>
              <Text size='small' type='tertiary'>
                {t(
                  '保存后立即影响新请求；关闭后停止新增，已有文件仍按 TTL 清理。',
                )}
              </Text>
            </div>
          </div>
          <Switch
            checked={settings.enabled}
            onChange={(enabled) =>
              setSettings((current) => ({ ...current, enabled }))
            }
          />
        </div>

        <div className='grid grid-cols-1 gap-3 md:grid-cols-3'>
          <label>
            <Text size='small' type='tertiary'>
              {t('TTL（分钟）')}
            </Text>
            <InputNumber
              className='mt-1 w-full'
              min={5}
              max={10080}
              value={settings.ttl_minutes}
              onChange={(ttl_minutes) =>
                setSettings((current) => ({ ...current, ttl_minutes }))
              }
            />
          </label>
          <label>
            <Text size='small' type='tertiary'>
              {t('容量上限（MiB）')}
            </Text>
            <InputNumber
              className='mt-1 w-full'
              min={16}
              max={10240}
              value={settings.max_storage_mib}
              onChange={(max_storage_mib) =>
                setSettings((current) => ({ ...current, max_storage_mib }))
              }
            />
          </label>
          <label>
            <Text size='small' type='tertiary'>
              {t('文件数上限')}
            </Text>
            <InputNumber
              className='mt-1 w-full'
              min={10}
              max={100000}
              value={settings.max_files}
              onChange={(max_files) =>
                setSettings((current) => ({ ...current, max_files }))
              }
            />
          </label>
          <label className='md:col-span-2'>
            <Text size='small' type='tertiary'>
              {t('重点用户')}
            </Text>
            <Select
              className='mt-1 w-full'
              multiple
              showClear
              filter
              optionList={userOptions}
              value={settings.priority_user_ids}
              placeholder={t('按用户名、显示名称或 ID 搜索')}
              onSearch={searchUsers}
              onChange={(priority_user_ids) =>
                setSettings((current) => ({ ...current, priority_user_ids }))
              }
            />
          </label>
          <label>
            <Text size='small' type='tertiary'>
              {t('重点渠道')}
            </Text>
            <Select
              className='mt-1 w-full'
              multiple
              showClear
              filter
              optionList={channelOptions}
              value={settings.priority_channel_ids}
              placeholder={t('按渠道名称或 ID 搜索')}
              onSearch={searchChannels}
              onChange={(priority_channel_ids) =>
                setSettings((current) => ({
                  ...current,
                  priority_channel_ids,
                }))
              }
            />
          </label>
          <div className='md:col-span-3'>
            <Text size='small' type='tertiary'>
              {t('存储路径')}
            </Text>
            <Input
              className='mt-1 font-mono'
              value={status?.storage_path || '-'}
              readOnly
            />
          </div>
        </div>

        <div className='mt-4 flex justify-end'>
          <Button
            type='primary'
            icon={<Save size={15} />}
            loading={saving}
            onClick={saveSettings}
          >
            {t('保存配置')}
          </Button>
        </div>
      </Card>

      <CardPro
        type='type2'
        statsArea={
          <div className='flex flex-col gap-2 md:flex-row md:items-center md:justify-between'>
            <div>
              <Text strong>{t('失败 Attempt')}</Text>
              <div className='mt-1'>
                <Text size='small' type='tertiary'>
                  {t('同一 Request ID 可能对应多次内部重试或跨渠道 fallback')}
                </Text>
              </div>
            </div>
            <Space wrap>
              <Tag size='large'>{t('共 {{count}} 条', { count: total })}</Tag>
              <Popconfirm
                title={t('确认立即清理错误快照？')}
                content={t('将删除已过期或超过当前容量、文件数限制的快照。')}
                okText={t('确认')}
                cancelText={t('取消')}
                onConfirm={cleanupSnapshots}
              >
                <Button icon={<RefreshCw size={14} />} loading={cleanupLoading}>
                  {t('立即清理')}
                </Button>
              </Popconfirm>
              <Popconfirm
                title={t('确认清空全部错误快照？')}
                content={t('全部索引和压缩文件都会被删除，且无法恢复。')}
                okText={t('确认')}
                cancelText={t('取消')}
                onConfirm={clearSnapshots}
              >
                <Button
                  type='danger'
                  theme='outline'
                  icon={<Trash2 size={14} />}
                  loading={clearLoading}
                >
                  {t('清空全部')}
                </Button>
              </Popconfirm>
            </Space>
          </div>
        }
        searchArea={
          <div>
            <div className='grid grid-cols-1 gap-2 md:grid-cols-2 lg:grid-cols-5'>
              <DatePicker
                className='w-full lg:col-span-2'
                type='dateTimeRange'
                placeholder={[t('开始时间'), t('结束时间')]}
                showClear
                pure
                size='small'
                value={filters.dateRange}
                presets={DATE_RANGE_PRESETS.map((preset) => ({
                  text: t(preset.text),
                  start: preset.start(),
                  end: preset.end(),
                }))}
                onChange={(dateRange) =>
                  setFilters((current) => ({
                    ...current,
                    dateRange: dateRange || [],
                  }))
                }
              />
              <Input
                prefix={<Search size={14} />}
                placeholder={t('Request ID')}
                showClear
                size='small'
                value={filters.request_id}
                onChange={(request_id) =>
                  setFilters((current) => ({ ...current, request_id }))
                }
              />
              <Select
                className='w-full'
                filter
                showClear
                size='small'
                optionList={userOptions}
                value={filters.user_id}
                placeholder={t('用户')}
                onSearch={searchUsers}
                onChange={(user_id) =>
                  setFilters((current) => ({ ...current, user_id }))
                }
              />
              <Select
                className='w-full'
                filter
                showClear
                size='small'
                optionList={channelOptions}
                value={filters.channel_id}
                placeholder={t('渠道')}
                onSearch={searchChannels}
                onChange={(channel_id) =>
                  setFilters((current) => ({ ...current, channel_id }))
                }
              />
              <Input
                className='lg:col-span-2'
                prefix={<Search size={14} />}
                placeholder={t('错误关键词')}
                showClear
                size='small'
                value={filters.error_keyword}
                onChange={(error_keyword) =>
                  setFilters((current) => ({ ...current, error_keyword }))
                }
                onEnterPress={submitFilters}
              />
            </div>
            <Space className='mt-3'>
              <Button
                type='primary'
                icon={<Search size={14} />}
                loading={listLoading}
                onClick={submitFilters}
              >
                {t('查询')}
              </Button>
              <Button onClick={resetFilters}>{t('重置')}</Button>
            </Space>
          </div>
        }
        paginationArea={createCardProPagination({
          currentPage: activePage,
          pageSize,
          total,
          onPageChange: (page) => loadSnapshots(page, pageSize, appliedFilters),
          onPageSizeChange: (size) => loadSnapshots(1, size, appliedFilters),
          isMobile,
          t,
        })}
        t={t}
      >
        <CardTable
          columns={columns}
          dataSource={snapshots}
          rowKey='id'
          loading={listLoading}
          hidePagination
          size='small'
          scroll={isMobile ? undefined : { x: 'max-content' }}
          onRow={(record) => ({
            onClick: () => openDetail(record),
            style: { cursor: 'pointer' },
          })}
          empty={<Empty description={t('暂无错误快照')} />}
        />
      </CardPro>

      <SideSheet
        placement='right'
        title={t('错误快照详情')}
        visible={detailVisible}
        onCancel={() => setDetailVisible(false)}
        width='min(760px, 100vw)'
        footer={null}
      >
        <Spin spinning={detailLoading}>
          {selectedSnapshot ? (
            <div className='flex min-h-[240px] flex-col gap-4'>
              <div className='flex flex-wrap items-center justify-between gap-2'>
                <Space wrap>
                  {outcomeTag(selectedSnapshot.final_outcome, t)}
                  <Tag
                    color={
                      selectedSnapshot.capture_level === 'priority'
                        ? 'orange'
                        : 'blue'
                    }
                  >
                    {selectedSnapshot.capture_level === 'priority'
                      ? t('重点')
                      : t('摘要')}
                  </Tag>
                  {selectedSnapshot.payload_truncated ? (
                    <Tag color='orange'>{t('快照已截断')}</Tag>
                  ) : null}
                </Space>
                <Space wrap>
                  <Button
                    icon={<Copy size={14} />}
                    onClick={() =>
                      copy(selectedSnapshot.request_id).then(
                        (ok) => ok && showSuccess(t('已复制')),
                      )
                    }
                  >
                    {t('复制 Request ID')}
                  </Button>
                  <Button
                    icon={<Download size={14} />}
                    onClick={() => downloadSnapshot(selectedSnapshot)}
                  >
                    {t('下载')}
                  </Button>
                  <Popconfirm
                    title={t('确认删除此错误快照？')}
                    content={t('索引和压缩文件将同时删除，且无法恢复。')}
                    okText={t('确认')}
                    cancelText={t('取消')}
                    onConfirm={() => deleteSnapshot(selectedSnapshot)}
                  >
                    <Button
                      type='danger'
                      theme='outline'
                      icon={<Trash2 size={14} />}
                    >
                      {t('删除')}
                    </Button>
                  </Popconfirm>
                </Space>
              </div>

              <Tabs type='line'>
                <TabPane tab={t('概览')} itemKey='overview'>
                  <div className='grid grid-cols-1 gap-3 pt-3 sm:grid-cols-2'>
                    <DetailField
                      label={t('Request ID')}
                      value={selectedSnapshot.request_id}
                      mono
                    />
                    <DetailField
                      label={t('时间')}
                      value={formatTime(selectedSnapshot.created_at)}
                    />
                    <DetailField
                      label={t('用户')}
                      value={`${selectedSnapshot.username || '-'} (#${selectedSnapshot.user_id || 0})`}
                    />
                    <DetailField
                      label={t('渠道')}
                      value={
                        selectedSnapshot.channel_id
                          ? `${selectedSnapshot.channel_name || '-'} (#${selectedSnapshot.channel_id})`
                          : '-'
                      }
                    />
                    <DetailField
                      label={t('模型')}
                      value={selectedSnapshot.model_name}
                    />
                    <DetailField
                      label={t('请求路径')}
                      value={selectedSnapshot.request_path}
                      mono
                    />
                    <DetailField
                      label={t('聚合分组')}
                      value={selectedSnapshot.aggregate_group}
                    />
                    <DetailField
                      label={t('子分组')}
                      value={selectedSnapshot.route_group}
                    />
                    <DetailField
                      label={t('错误码')}
                      value={`${selectedSnapshot.status_code || 0} / ${selectedSnapshot.error_code || '-'}`}
                      mono
                    />
                    <DetailField
                      label={t('快照大小')}
                      value={`${formatBytes(selectedSnapshot.compressed_size)} / ${formatBytes(selectedSnapshot.original_size)}`}
                    />
                    <div className='sm:col-span-2'>
                      <DetailField
                        label={t('错误信息')}
                        value={selectedSnapshot.error_message}
                      />
                    </div>
                  </div>
                  <div className='mt-4 flex flex-col gap-4'>
                    <JSONSection
                      title={t('路由信息')}
                      value={payload.route}
                      emptyText={t('没有路由诊断信息')}
                    />
                    <JSONSection
                      title={t('计时信息')}
                      value={payload.timing}
                      emptyText={t('没有计时诊断信息')}
                    />
                  </div>
                </TabPane>
                <TabPane tab={t('请求')} itemKey='request'>
                  <div className='flex flex-col gap-5 pt-3'>
                    {selectedSnapshot.capture_level !== 'priority' ? (
                      <Banner
                        type='info'
                        closeIcon={null}
                        description={t(
                          '该快照未命中重点观察规则，只采集诊断摘要，客户端和上游请求正文未保存。',
                        )}
                      />
                    ) : null}
                    <BodySection
                      title={t('客户端请求')}
                      fragment={payload.client_request}
                      missingText={t('客户端请求正文未采集')}
                      truncatedText={t('快照已截断')}
                      hashLabel={t('SHA-256')}
                    />
                    <BodySection
                      title={t('上游请求')}
                      fragment={payload.upstream_request}
                      missingText={t('上游请求正文未采集')}
                      truncatedText={t('快照已截断')}
                      hashLabel={t('SHA-256')}
                    />
                    <JSONSection
                      title={t('请求元数据')}
                      value={payload.request}
                      emptyText={t('没有请求元数据')}
                    />
                  </div>
                </TabPane>
                <TabPane tab={t('上游响应')} itemKey='response'>
                  <div className='pt-3'>
                    <BodySection
                      title={t('异常上游响应')}
                      fragment={payload.upstream_response}
                      missingText={t('没有可用的上游响应片段')}
                      truncatedText={t('快照已截断')}
                      hashLabel={t('SHA-256')}
                    />
                  </div>
                </TabPane>
                <TabPane tab={t('流状态')} itemKey='stream'>
                  <div className='pt-3'>
                    <JSONSection
                      title={t('流事件摘要')}
                      value={payload.stream}
                      emptyText={t('该错误没有流状态摘要')}
                    />
                  </div>
                </TabPane>
              </Tabs>
            </div>
          ) : (
            <Empty description={t('暂无详情')} />
          )}
        </Spin>
      </SideSheet>
    </div>
  );
};

export default ErrorSnapshots;
