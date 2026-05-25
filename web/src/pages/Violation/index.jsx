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
  Col,
  DatePicker,
  Empty,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Row,
  Select,
  Space,
  Switch,
  Tag,
  TextArea,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Copy as CopyIcon,
  Eye,
  RefreshCw,
  Save,
  Search,
  ShieldAlert,
  Trash2,
} from 'lucide-react';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import CardTable from '../../components/common/ui/CardTable';
import CardPro from '../../components/common/ui/CardPro';
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

const defaultSetting = {
  enabled: false,
  keywords: '',
  case_sensitive: false,
  action: 'block',
  http_status_code: 403,
  error_code: 'policy_violation',
  error_message: 'Request blocked by policy.',
  max_excerpt_length: 300,
  ban_threshold: 3,
};

const defaultFilters = {
  dateRange: [],
  user_id: '',
  username: '',
  token_id: '',
  token_name: '',
  model_name: '',
  using_group: '',
  aggregate_group: '',
  route_group: '',
  request_id: '',
  matched_word: '',
  action: '',
  banned: '',
};

const actionColorMap = {
  log_only: 'blue',
  block: 'red',
  ban_after_threshold: 'violet',
};

const displayValue = (value) => {
  if (value === undefined || value === null || value === '') return '-';
  return value;
};

const DetailField = ({
  label,
  value,
  children,
  mono = false,
  className = '',
}) => (
  <div
    className={`min-w-0 rounded-md border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] px-3 py-2 ${className}`}
  >
    <div className='mb-1 text-xs font-medium leading-4 text-[var(--semi-color-text-2)]'>
      {label}
    </div>
    <div
      className={`min-h-[20px] break-words text-sm font-medium leading-5 text-[var(--semi-color-text-0)] ${
        mono ? 'break-all font-mono text-[13px] font-normal' : ''
      }`}
    >
      {children ?? displayValue(value)}
    </div>
  </div>
);

const DetailSection = ({ title, children, action }) => (
  <section className='min-w-0'>
    <div className='mb-2 flex min-h-[28px] items-center justify-between gap-3'>
      <Text strong>{title}</Text>
      {action}
    </div>
    {children}
  </section>
);

const PanelSection = ({ title, description, children, className = '' }) => (
  <section
    className={`rounded-md border border-[var(--semi-color-border)] bg-[var(--semi-color-bg-0)] p-3 ${className}`}
  >
    <div className='mb-3'>
      <Text strong>{title}</Text>
      {description ? (
        <div className='mt-1'>
          <Text size='small' type='tertiary'>
            {description}
          </Text>
        </div>
      ) : null}
    </div>
    {children}
  </section>
);

const FieldLabel = ({ children }) => (
  <Text size='small' type='tertiary'>
    {children}
  </Text>
);

const formatTime = (timestamp) => {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
};

const actionLabel = (action, t) => {
  switch (action) {
    case 'log_only':
      return t('只记录');
    case 'ban_after_threshold':
      return t('达到阈值封禁');
    default:
      return t('拦截');
  }
};

const parseMatchedWords = (raw) => {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
};

const buildLogParams = (filters, page, pageSize) => {
  const params = { p: page, page_size: pageSize };
  for (const [key, value] of Object.entries(filters)) {
    if (key === 'dateRange') continue;
    if (value !== '' && value !== null && value !== undefined) {
      params[key] = value;
    }
  }
  if (Array.isArray(filters.dateRange) && filters.dateRange.length === 2) {
    params.start_timestamp = Math.floor(
      Date.parse(filters.dateRange[0]) / 1000,
    );
    params.end_timestamp = Math.floor(Date.parse(filters.dateRange[1]) / 1000);
  }
  return params;
};

const ViolationPage = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [setting, setSetting] = useState(defaultSetting);
  const [keywordCount, setKeywordCount] = useState(0);
  const [statusLoading, setStatusLoading] = useState(false);
  const [saveLoading, setSaveLoading] = useState(false);
  const [logs, setLogs] = useState([]);
  const [logCount, setLogCount] = useState(0);
  const [logsLoading, setLogsLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [filters, setFilters] = useState(defaultFilters);
  const [detail, setDetail] = useState(null);
  const [cleanupDate, setCleanupDate] = useState(null);

  const loadStatus = async () => {
    setStatusLoading(true);
    try {
      const res = await API.get('/api/violation/status');
      if (res.data.success) {
        const data = res.data.data || {};
        setSetting({ ...defaultSetting, ...(data.setting || {}) });
        setKeywordCount(data.keyword_count || 0);
      } else {
        showError(res.data.message || t('加载失败'));
      }
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setStatusLoading(false);
    }
  };

  const loadLogs = async (
    page = activePage,
    size = pageSize,
    nextFilters = filters,
  ) => {
    setLogsLoading(true);
    try {
      const res = await API.get('/api/violation/logs', {
        params: buildLogParams(nextFilters, page, size),
      });
      if (res.data.success) {
        const data = res.data.data || {};
        setLogs((data.items || []).map((item) => ({ ...item, key: item.id })));
        setLogCount(data.total || 0);
        setActivePage(data.page || page);
        setPageSize(data.page_size || size);
      } else {
        showError(res.data.message || t('加载失败'));
      }
    } catch (error) {
      showError(error.message || t('加载失败'));
    } finally {
      setLogsLoading(false);
    }
  };

  useEffect(() => {
    loadStatus();
    loadLogs(1, pageSize, filters);
  }, []);

  const updateSettingField = (field, value) => {
    setSetting((current) => ({ ...current, [field]: value }));
  };

  const saveSetting = async () => {
    setSaveLoading(true);
    try {
      const payload = {
        ...setting,
        http_status_code: Number(setting.http_status_code) || 403,
        max_excerpt_length: Number(setting.max_excerpt_length) || 300,
        ban_threshold: Number(setting.ban_threshold) || 3,
      };
      const res = await API.put('/api/violation/setting', payload);
      if (res.data.success) {
        setSetting({ ...defaultSetting, ...(res.data.data || {}) });
        showSuccess(t('保存成功'));
        loadStatus();
      } else {
        showError(res.data.message || t('保存失败'));
      }
    } catch (error) {
      showError(error.message || t('保存失败'));
    } finally {
      setSaveLoading(false);
    }
  };

  const submitFilters = (values) => {
    const nextFilters = { ...defaultFilters, ...values };
    setFilters(nextFilters);
    loadLogs(1, pageSize, nextFilters);
  };

  const resetFilters = () => {
    setFilters(defaultFilters);
    loadLogs(1, pageSize, defaultFilters);
  };

  const handlePageChange = (page) => {
    loadLogs(page, pageSize, filters);
  };

  const handlePageSizeChange = (size) => {
    loadLogs(1, size, filters);
  };

  const cleanupLogs = async () => {
    if (!cleanupDate) {
      showError(t('请选择清理截止时间'));
      return;
    }
    try {
      const targetTimestamp = Math.floor(Date.parse(cleanupDate) / 1000);
      const res = await API.delete('/api/violation/logs', {
        params: { target_timestamp: targetTimestamp },
      });
      if (res.data.success) {
        showSuccess(
          t('已清理 {{count}} 条记录', { count: res.data.data || 0 }),
        );
        loadLogs(1, pageSize, filters);
      } else {
        showError(res.data.message || t('清理失败'));
      }
    } catch (error) {
      showError(error.message || t('清理失败'));
    }
  };

  const columns = useMemo(
    () => [
      {
        title: t('时间'),
        dataIndex: 'created_at',
        key: 'created_at',
        width: 170,
        render: (value) => formatTime(value),
      },
      {
        title: t('用户'),
        dataIndex: 'username',
        key: 'username',
        width: 170,
        render: (_, record) => (
          <div>
            <div>{record.username || '-'}</div>
            <Text type='tertiary' size='small'>
              ID {record.user_id || '-'}
            </Text>
          </div>
        ),
      },
      {
        title: t('令牌'),
        dataIndex: 'token_name',
        key: 'token_name',
        width: 160,
        render: (_, record) => (
          <div>
            <div>{record.token_name || '-'}</div>
            <Text type='tertiary' size='small'>
              ID {record.token_id || '-'}
            </Text>
          </div>
        ),
      },
      {
        title: t('模型'),
        dataIndex: 'model_name',
        key: 'model_name',
        width: 160,
      },
      {
        title: t('分组'),
        dataIndex: 'using_group',
        key: 'using_group',
        width: 210,
        render: (_, record) => (
          <Space spacing={4} wrap>
            <Tag size='small'>{record.using_group || '-'}</Tag>
            {record.aggregate_group ? (
              <Tag size='small' color='purple'>
                {record.aggregate_group}
              </Tag>
            ) : null}
            {record.route_group ? (
              <Tag size='small' color='cyan'>
                {record.route_group}
              </Tag>
            ) : null}
          </Space>
        ),
      },
      {
        title: t('命中词'),
        dataIndex: 'matched_words',
        key: 'matched_words',
        width: 220,
        render: (value) => (
          <Space spacing={4} wrap>
            {parseMatchedWords(value).map((word) => (
              <Tag key={word} color='red' size='small'>
                {word}
              </Tag>
            ))}
          </Space>
        ),
      },
      {
        title: t('策略'),
        dataIndex: 'action',
        key: 'action',
        width: 140,
        render: (value, record) => (
          <Space spacing={4}>
            <Tag color={actionColorMap[value] || 'grey'}>
              {actionLabel(value, t)}
            </Tag>
            {record.banned ? <Tag color='red'>{t('已封禁')}</Tag> : null}
          </Space>
        ),
      },
      {
        title: t('Request ID'),
        dataIndex: 'request_id',
        key: 'request_id',
        width: 180,
        render: (value) => (
          <Text
            link
            ellipsis={{ showTooltip: true }}
            onClick={() => value && copy(value)}
          >
            {value || '-'}
          </Text>
        ),
      },
      {
        title: '',
        key: 'action_col',
        width: 80,
        fixed: 'right',
        render: (_, record) => (
          <Tooltip content={t('查看详情')}>
            <Button
              size='small'
              theme='borderless'
              type='tertiary'
              icon={<Eye size={14} />}
              aria-label={t('查看详情')}
              onClick={() => setDetail(record)}
            />
          </Tooltip>
        ),
      },
    ],
    [t],
  );

  const matchedWordsForDetail = detail
    ? parseMatchedWords(detail.matched_words)
    : [];

  const copyDetail = () => {
    if (!detail) return;
    copy(JSON.stringify(detail, null, 2)).then((ok) => {
      if (ok) showSuccess(t('已复制'));
    });
  };

  const copyDetailValue = (value) => {
    if (!value) return;
    copy(String(value)).then((ok) => {
      if (ok) showSuccess(t('已复制'));
    });
  };

  return (
    <div className='px-2 py-4 md:px-6'>
      <div className='mb-4 flex flex-col gap-2 md:flex-row md:items-center md:justify-between'>
        <div>
          <Title heading={3} className='!mb-1'>
            {t('风险检测')}
          </Title>
          <Text type='tertiary'>
            {t('配置安全风控词，查看命中记录和当前处置结果')}
          </Text>
        </div>
        <Space>
          <Tag color={setting.enabled ? 'green' : 'grey'} size='large'>
            {setting.enabled ? t('已启用') : t('已关闭')}
          </Tag>
          <Tag size='large'>
            {keywordCount} {t('个关键词')}
          </Tag>
          <Button
            icon={<RefreshCw size={16} />}
            onClick={loadStatus}
            loading={statusLoading}
          >
            {t('刷新')}
          </Button>
        </Space>
      </div>

      <Banner
        type='warning'
        closeIcon={null}
        style={{ marginBottom: 16 }}
        description={t(
          '命中记录只保存上下文片段，不保存完整 prompt。清理历史记录会降低累计命中次数。',
        )}
      />

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={8}>
          <Card
            className='!rounded-lg'
            bodyStyle={{ padding: isMobile ? 14 : 16 }}
          >
            <div className='mb-4 flex items-start justify-between gap-3'>
              <div>
                <Text strong>{t('检测策略')}</Text>
                <div className='mt-1'>
                  <Text size='small' type='tertiary'>
                    {t('命中后按策略记录、拦截或禁用账号')}
                  </Text>
                </div>
              </div>
              <ShieldAlert
                size={18}
                className='shrink-0 text-[var(--semi-color-warning)]'
              />
            </div>
            <Form layout='vertical'>
              <div className='mb-3 flex items-center justify-between gap-3 rounded-md border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] p-3'>
                <div>
                  <Text strong>{t('启用风险检测')}</Text>
                  <div>
                    <Text size='small' type='tertiary'>
                      {t('关闭时 relay 只做开关判断')}
                    </Text>
                  </div>
                </div>
                <Switch
                  checked={setting.enabled}
                  onChange={(value) => updateSettingField('enabled', value)}
                />
              </div>

              <div className='space-y-3'>
                <PanelSection
                  title={t('匹配规则')}
                  description={t('一行一个关键词，未命中时不访问数据库')}
                >
                  <div className='mb-3'>
                    <FieldLabel>{t('安全风控词')}</FieldLabel>
                  </div>
                  <TextArea
                    placeholder={t('一行一个关键词')}
                    value={setting.keywords}
                    autosize={{ minRows: 8, maxRows: 14 }}
                    onChange={(value) => updateSettingField('keywords', value)}
                    showClear
                  />
                  <div className='mb-3 mt-1'>
                    <Text size='small' type='tertiary'>
                      {t('开启后未命中只做内存匹配，不访问数据库')}
                    </Text>
                  </div>

                  <div className='flex items-center justify-between gap-3'>
                    <Text>{t('关键词区分大小写')}</Text>
                    <Switch
                      checked={setting.case_sensitive}
                      onChange={(value) =>
                        updateSettingField('case_sensitive', value)
                      }
                    />
                  </div>
                </PanelSection>

                <PanelSection
                  title={t('处置策略')}
                  description={t('第一版本不扣费，不禁用令牌或渠道')}
                >
                  <div className='mb-1'>
                    <FieldLabel>{t('命中策略')}</FieldLabel>
                  </div>
                  <Select
                    className='mb-3 w-full'
                    value={setting.action}
                    onChange={(value) => updateSettingField('action', value)}
                  >
                    <Select.Option value='log_only'>
                      {t('只记录，继续请求')}
                    </Select.Option>
                    <Select.Option value='block'>
                      {t('记录并拦截本次请求')}
                    </Select.Option>
                    <Select.Option value='ban_after_threshold'>
                      {t('记录、拦截，达到阈值封禁账号')}
                    </Select.Option>
                  </Select>

                  <div>
                    <FieldLabel>{t('封禁阈值')}</FieldLabel>
                    <InputNumber
                      className='mt-1 w-full'
                      min={1}
                      max={1000000}
                      value={setting.ban_threshold}
                      onChange={(value) =>
                        updateSettingField('ban_threshold', value)
                      }
                    />
                  </div>
                </PanelSection>

                <PanelSection
                  title={t('响应配置')}
                  description={t('拦截时返回给调用方的状态码和错误信息')}
                >
                  <div className='grid grid-cols-1 gap-3 md:grid-cols-2'>
                    <div>
                      <FieldLabel>{t('HTTP 状态码')}</FieldLabel>
                      <InputNumber
                        className='mt-1 w-full'
                        min={400}
                        max={599}
                        value={setting.http_status_code}
                        onChange={(value) =>
                          updateSettingField('http_status_code', value)
                        }
                      />
                    </div>
                    <div>
                      <FieldLabel>{t('片段长度')}</FieldLabel>
                      <InputNumber
                        className='mt-1 w-full'
                        min={1}
                        max={2000}
                        value={setting.max_excerpt_length}
                        onChange={(value) =>
                          updateSettingField('max_excerpt_length', value)
                        }
                      />
                    </div>
                    <div className='md:col-span-2'>
                      <FieldLabel>{t('业务错误码')}</FieldLabel>
                      <Input
                        className='mt-1'
                        value={setting.error_code}
                        onChange={(value) =>
                          updateSettingField('error_code', value)
                        }
                      />
                    </div>
                  </div>

                  <div className='mt-3'>
                    <FieldLabel>{t('错误信息')}</FieldLabel>
                    <Input
                      className='mt-1'
                      value={setting.error_message}
                      onChange={(value) =>
                        updateSettingField('error_message', value)
                      }
                    />
                  </div>
                </PanelSection>

                <Button
                  className='w-full'
                  type='primary'
                  icon={<Save size={16} />}
                  loading={saveLoading}
                  onClick={saveSetting}
                >
                  {t('保存配置')}
                </Button>
              </div>
            </Form>
          </Card>

          <Card className='mt-4 !rounded-lg'>
            <Text strong>{t('清理命中记录')}</Text>
            <div className='mt-3 flex flex-col gap-3'>
              <DatePicker
                type='dateTime'
                placeholder={t('清理此时间之前的记录')}
                value={cleanupDate}
                onChange={setCleanupDate}
                showClear
              />
              <Popconfirm
                title={t('确认清理历史命中记录？')}
                content={t('清理后会降低累计命中次数。')}
                okText={t('确认')}
                cancelText={t('取消')}
                onConfirm={cleanupLogs}
              >
                <Button
                  type='danger'
                  theme='outline'
                  icon={<Trash2 size={16} />}
                >
                  {t('清理历史记录')}
                </Button>
              </Popconfirm>
            </div>
          </Card>
        </Col>

        <Col xs={24} xl={16}>
          <CardPro
            type='type2'
            statsArea={
              <div className='mb-1 flex flex-col gap-2 md:flex-row md:items-center md:justify-between'>
                <div>
                  <Text strong>{t('命中记录')}</Text>
                  <div className='mt-1'>
                    <Text size='small' type='tertiary'>
                      {t('查看触发用户、模型、分组和命中片段')}
                    </Text>
                  </div>
                </div>
                <Space>
                  <Tag size='large'>
                    {t('共 {{count}} 条', { count: logCount })}
                  </Tag>
                  <Button
                    icon={<RefreshCw size={14} />}
                    onClick={() => loadLogs(activePage, pageSize, filters)}
                    loading={logsLoading}
                  >
                    {t('刷新')}
                  </Button>
                </Space>
              </div>
            }
            searchArea={
              <Form
                initValues={filters}
                onSubmit={submitFilters}
                allowEmpty
                layout='vertical'
                trigger='change'
              >
                <div className='grid grid-cols-1 gap-2 md:grid-cols-2 lg:grid-cols-4'>
                  <div className='lg:col-span-2'>
                    <Form.DatePicker
                      field='dateRange'
                      className='w-full'
                      type='dateTimeRange'
                      placeholder={[t('开始时间'), t('结束时间')]}
                      showClear
                      pure
                      size='small'
                      presets={DATE_RANGE_PRESETS.map((preset) => ({
                        text: t(preset.text),
                        start: preset.start(),
                        end: preset.end(),
                      }))}
                    />
                  </div>
                  <Form.Input
                    field='username'
                    prefix={<Search size={14} />}
                    placeholder={t('用户名')}
                    pure
                    size='small'
                    showClear
                  />
                  <Form.Input
                    field='user_id'
                    prefix={<Search size={14} />}
                    placeholder={t('用户 ID')}
                    pure
                    size='small'
                    showClear
                  />
                  <Form.Input
                    field='token_name'
                    prefix={<Search size={14} />}
                    placeholder={t('令牌名称')}
                    pure
                    size='small'
                    showClear
                  />
                  <Form.Input
                    field='token_id'
                    prefix={<Search size={14} />}
                    placeholder={t('Token ID')}
                    pure
                    size='small'
                    showClear
                  />
                  <Form.Input
                    field='model_name'
                    prefix={<Search size={14} />}
                    placeholder={t('模型')}
                    pure
                    size='small'
                    showClear
                  />
                  <Form.Input
                    field='matched_word'
                    prefix={<Search size={14} />}
                    placeholder={t('命中词')}
                    pure
                    size='small'
                    showClear
                  />
                  <Form.Input
                    field='using_group'
                    prefix={<Search size={14} />}
                    placeholder={t('实际分组')}
                    pure
                    size='small'
                    showClear
                  />
                  <Form.Input
                    field='aggregate_group'
                    prefix={<Search size={14} />}
                    placeholder={t('聚合分组')}
                    pure
                    size='small'
                    showClear
                  />
                  <Form.Input
                    field='route_group'
                    prefix={<Search size={14} />}
                    placeholder={t('路由分组')}
                    pure
                    size='small'
                    showClear
                  />
                  <Form.Input
                    field='request_id'
                    prefix={<Search size={14} />}
                    placeholder='Request ID'
                    pure
                    size='small'
                    showClear
                  />
                  <Form.Select
                    field='action'
                    placeholder={t('策略')}
                    pure
                    size='small'
                    showClear
                  >
                    <Select.Option value='log_only'>
                      {t('只记录')}
                    </Select.Option>
                    <Select.Option value='block'>{t('拦截')}</Select.Option>
                    <Select.Option value='ban_after_threshold'>
                      {t('阈值封禁')}
                    </Select.Option>
                  </Form.Select>
                  <Form.Select
                    field='banned'
                    placeholder={t('是否封禁')}
                    pure
                    size='small'
                    showClear
                  >
                    <Select.Option value='true'>{t('是')}</Select.Option>
                    <Select.Option value='false'>{t('否')}</Select.Option>
                  </Form.Select>
                </div>
                <Space className='mt-3'>
                  <Button
                    htmlType='submit'
                    type='primary'
                    icon={<Search size={14} />}
                    loading={logsLoading}
                  >
                    {t('查询')}
                  </Button>
                  <Button onClick={resetFilters}>{t('重置')}</Button>
                </Space>
              </Form>
            }
            paginationArea={createCardProPagination({
              currentPage: activePage,
              pageSize,
              total: logCount,
              onPageChange: handlePageChange,
              onPageSizeChange: handlePageSizeChange,
              isMobile,
              t,
            })}
            t={t}
          >
            <CardTable
              columns={columns}
              dataSource={logs}
              rowKey='id'
              loading={logsLoading}
              scroll={isMobile ? undefined : { x: 'max-content' }}
              className='rounded-xl overflow-hidden'
              size='small'
              hidePagination
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
                  description={t('搜索无结果')}
                  style={{ padding: 30 }}
                />
              }
            />
          </CardPro>
        </Col>
      </Row>

      <Modal
        title={t('命中详情')}
        visible={!!detail}
        onCancel={() => setDetail(null)}
        footer={
          <div className='flex justify-end gap-2'>
            <Button onClick={() => setDetail(null)}>{t('关闭')}</Button>
            <Button
              type='primary'
              theme='solid'
              icon={<CopyIcon size={14} />}
              onClick={copyDetail}
            >
              {t('复制详情')}
            </Button>
          </div>
        }
        width={isMobile ? 'calc(100vw - 24px)' : 920}
        bodyStyle={{ paddingTop: 12 }}
      >
        {detail ? (
          <div className='space-y-5'>
            <div className='flex flex-col gap-3 rounded-md border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] p-3 md:flex-row md:items-center md:justify-between'>
              <div className='min-w-0'>
                <div className='mb-1 flex flex-wrap items-center gap-2'>
                  <Tag
                    color={actionColorMap[detail.action] || 'grey'}
                    size='large'
                  >
                    {actionLabel(detail.action, t)}
                  </Tag>
                  {detail.banned ? (
                    <Tag color='red' size='large'>
                      {t('已封禁')}
                    </Tag>
                  ) : (
                    <Tag color='green' size='large'>
                      {t('未封禁')}
                    </Tag>
                  )}
                  <Tag size='large'>
                    {detail.is_stream ? t('流式') : t('非流式')}
                  </Tag>
                </div>
                <Text type='tertiary' size='small'>
                  {formatTime(detail.created_at)}
                </Text>
              </div>
              <div className='flex flex-wrap gap-2 md:justify-end'>
                <Tag color='orange' size='large'>
                  HTTP {displayValue(detail.http_status_code)}
                </Tag>
                <Tag color='red' size='large'>
                  {displayValue(detail.error_code)}
                </Tag>
              </div>
            </div>

            <DetailSection title={t('命中信息')}>
              <div className='grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3'>
                <DetailField
                  label={t('用户')}
                  value={`${displayValue(detail.username)} (${displayValue(detail.user_id)})`}
                />
                <DetailField
                  label={t('令牌')}
                  value={`${displayValue(detail.token_name)} (${displayValue(detail.token_id)})`}
                />
                <DetailField label={t('模型')} value={detail.model_name} mono />
                <DetailField label={t('用户分组')} value={detail.user_group} />
                <DetailField label={t('实际分组')} value={detail.using_group} />
                <DetailField
                  label={t('聚合分组')}
                  value={detail.aggregate_group}
                />
                <DetailField label={t('路由分组')} value={detail.route_group} />
                <DetailField label={t('命中词')} className='md:col-span-2'>
                  <div className='flex flex-wrap gap-1'>
                    {matchedWordsForDetail.length > 0
                      ? matchedWordsForDetail.map((word) => (
                          <Tag key={word} color='red' size='small'>
                            {word}
                          </Tag>
                        ))
                      : '-'}
                  </div>
                </DetailField>
              </div>
            </DetailSection>

            <DetailSection title={t('请求信息')}>
              <div className='grid grid-cols-1 gap-3 md:grid-cols-2'>
                <DetailField label='Request ID' value={detail.request_id} mono>
                  <div className='flex min-w-0 items-start justify-between gap-2'>
                    <span className='min-w-0 break-all'>
                      {displayValue(detail.request_id)}
                    </span>
                    {detail.request_id ? (
                      <Button
                        size='small'
                        theme='borderless'
                        type='tertiary'
                        icon={<CopyIcon size={13} />}
                        aria-label={t('复制 Request ID')}
                        onClick={() => copyDetailValue(detail.request_id)}
                      />
                    ) : null}
                  </div>
                </DetailField>
                <DetailField
                  label={t('请求路径')}
                  value={detail.request_path}
                  mono
                >
                  <div className='flex min-w-0 items-start justify-between gap-2'>
                    <span className='min-w-0 break-all'>
                      {displayValue(detail.request_path)}
                    </span>
                    {detail.request_path ? (
                      <Button
                        size='small'
                        theme='borderless'
                        type='tertiary'
                        icon={<CopyIcon size={13} />}
                        aria-label={t('复制请求路径')}
                        onClick={() => copyDetailValue(detail.request_path)}
                      />
                    ) : null}
                  </div>
                </DetailField>
              </div>
            </DetailSection>

            <DetailSection title={t('上下文片段')}>
              <pre className='max-h-[280px] overflow-auto whitespace-pre-wrap break-words rounded-md border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] p-3 font-mono text-[13px] leading-5 text-[var(--semi-color-text-0)]'>
                {detail.text_excerpt || '-'}
              </pre>
            </DetailSection>
          </div>
        ) : null}
      </Modal>
    </div>
  );
};

export default ViolationPage;
