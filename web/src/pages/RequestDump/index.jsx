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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Banner,
  Button,
  Card,
  Checkbox,
  Col,
  Form,
  Input,
  InputNumber,
  Radio,
  Row,
  Select,
  Space,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Bug,
  Clipboard,
  Pause,
  Play,
  RefreshCw,
  Search,
  Trash2,
} from 'lucide-react';
import { API, copy, showError, showSuccess } from '../../helpers';

const { Text, Title } = Typography;

const POLL_INTERVAL_MS = 3000;
const MAX_LOCAL_EVENTS = 500;

const defaultForm = {
  user_ids: '',
  token_ids: '',
  token_names: '',
  models: '',
  paths: '/v1/chat/completions',
  aggregate_groups: '',
  keywords: '',
  case_sensitive: false,
  duration_seconds: 300,
  max_count: 20,
  print_on: 'all',
  log_level: 'info',
  print_url: true,
  print_headers: true,
  print_body: true,
  print_upstream_body: false,
  max_body_kb: 256,
  trace_responses_stream: false,
  trace_responses_stream_key_events_only: false,
  max_stream_events_per_request: 200,
};

const splitList = (value) =>
  String(value || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);

const parseIdList = (value) =>
  splitList(value)
    .map((item) => Number(item))
    .filter((item) => Number.isInteger(item) && item > 0);

const formatTime = (timestamp) => {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
};

const eventToText = (event) => JSON.stringify(event, null, 2);

const RequestDumpPage = () => {
  const { t } = useTranslation();
  const [formValues, setFormValues] = useState(defaultForm);
  const [status, setStatus] = useState(null);
  const [events, setEvents] = useState([]);
  const [loading, setLoading] = useState(false);
  const [paused, setPaused] = useState(false);
  const [following, setFollowing] = useState(true);
  const [search, setSearch] = useState('');
  const consoleRef = useRef(null);
  const lastEventIdRef = useRef(0);
  const enabledRef = useRef(false);
  const pausedRef = useRef(false);
  const visibleRef = useRef(typeof document === 'undefined' || document.visibilityState === 'visible');
  const enabled = status?.enabled === true;

  const loadStatus = async () => {
    try {
      const res = await API.get('/api/request_dump/status');
      if (res.data.success) {
        const nextStatus = res.data.data;
        setStatus(nextStatus);
        enabledRef.current = nextStatus?.enabled === true;
      }
    } catch (error) {
      showError(error.message || t('加载失败'));
    }
  };

  const fetchEvents = async () => {
    if (pausedRef.current || !enabledRef.current || !visibleRef.current) return;
    try {
      const res = await API.get('/api/request_dump/events', {
        params: { after_id: lastEventIdRef.current, limit: 100 },
      });
      if (!res.data.success) {
        return;
      }
      const payload = res.data.data || {};
      const nextEvents = payload.events || [];
      const nextStatus = payload.status || null;
      setStatus(nextStatus);
      enabledRef.current = nextStatus?.enabled === true;
      if (nextEvents.length === 0) {
        return;
      }
      lastEventIdRef.current = nextEvents[nextEvents.length - 1].id || lastEventIdRef.current;
      setEvents((current) => {
        const merged = [...current, ...nextEvents];
        return merged.slice(Math.max(0, merged.length - MAX_LOCAL_EVENTS));
      });
    } catch (error) {
      showError(error.message || t('加载失败'));
    }
  };

  useEffect(() => {
    loadStatus();
  }, []);

  useEffect(() => {
    pausedRef.current = paused;
  }, [paused]);

  useEffect(() => {
    enabledRef.current = status?.enabled === true;
  }, [status?.enabled]);

  useEffect(() => {
    const handleVisibilityChange = () => {
      visibleRef.current = document.visibilityState === 'visible';
      if (visibleRef.current && enabledRef.current && !pausedRef.current) {
        fetchEvents();
      }
    };
    document.addEventListener('visibilitychange', handleVisibilityChange);
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange);
  }, []);

  useEffect(() => {
    if (!enabled || paused) {
      return undefined;
    }
    fetchEvents();
    const timer = window.setInterval(fetchEvents, POLL_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [enabled, paused]);

  useEffect(() => {
    if (following && consoleRef.current) {
      consoleRef.current.scrollTop = consoleRef.current.scrollHeight;
    }
  }, [events, following]);

  const filteredEvents = useMemo(() => {
    const keyword = search.trim().toLowerCase();
    if (!keyword) return events;
    return events.filter((event) => eventToText(event).toLowerCase().includes(keyword));
  }, [events, search]);

  const startDump = async () => {
    const userIds = parseIdList(formValues.user_ids);
    if (userIds.length === 0) {
      showError(t('必须指定用户 ID'));
      return;
    }

    const payload = {
      user_ids: userIds,
      token_ids: parseIdList(formValues.token_ids),
      token_names: splitList(formValues.token_names),
      models: splitList(formValues.models),
      paths: splitList(formValues.paths),
      aggregate_groups: splitList(formValues.aggregate_groups),
      keywords: splitList(formValues.keywords),
      case_sensitive: formValues.case_sensitive === true,
      duration_seconds: Number(formValues.duration_seconds) || 300,
      max_count: Number(formValues.max_count) || 20,
      print_on: formValues.print_on || 'all',
      log_level: formValues.log_level || 'info',
      print_url: formValues.print_url === true,
      print_headers: formValues.print_headers === true,
      print_body: formValues.print_body === true,
      print_upstream_body: formValues.print_upstream_body === true,
      max_body_bytes: Math.max(1, Number(formValues.max_body_kb) || 256) * 1024,
      trace_responses_stream: formValues.trace_responses_stream === true,
      trace_responses_stream_key_events_only: formValues.trace_responses_stream_key_events_only === true,
      max_stream_events_per_request: Number(formValues.max_stream_events_per_request) || 200,
    };

    setLoading(true);
    try {
      const res = await API.post('/api/request_dump/start', payload);
      if (res.data.success) {
        setStatus(res.data.data);
        enabledRef.current = res.data.data?.enabled === true;
        lastEventIdRef.current = 0;
        setEvents([]);
        showSuccess(t('Dump 已启动'));
      } else {
        showError(res.data.message || t('启动失败'));
      }
    } catch (error) {
      showError(error.message || t('启动失败'));
    } finally {
      setLoading(false);
    }
  };

  const stopDump = async () => {
    setLoading(true);
    try {
      const res = await API.post('/api/request_dump/stop');
      if (res.data.success) {
        setStatus(res.data.data);
        enabledRef.current = res.data.data?.enabled === true;
        showSuccess(t('Dump 已停止'));
      } else {
        showError(res.data.message || t('停止失败'));
      }
    } catch (error) {
      showError(error.message || t('停止失败'));
    } finally {
      setLoading(false);
    }
  };

  const clearConsole = async () => {
    try {
      const res = await API.post('/api/request_dump/clear');
      if (res.data.success) {
        setStatus(res.data.data);
        enabledRef.current = res.data.data?.enabled === true;
        setEvents([]);
        lastEventIdRef.current = 0;
        showSuccess(t('Console 已清空'));
      } else {
        showError(res.data.message || t('清空失败'));
      }
    } catch (error) {
      showError(error.message || t('清空失败'));
    }
  };

  const copyAll = async () => {
    if (filteredEvents.length === 0) {
      showError(t('暂无内容可复制'));
      return;
    }
    const ok = await copy(filteredEvents.map(eventToText).join('\n\n'));
    if (ok) showSuccess(t('已复制'));
  };

  const updateField = (field, value) => {
    setFormValues((current) => ({ ...current, [field]: value }));
  };

  return (
    <div className='px-2 py-4 md:px-6'>
      <div className='mb-4 flex flex-col gap-2 md:flex-row md:items-center md:justify-between'>
        <div>
          <Title heading={3} className='!mb-1'>
            {t('Dump 分析')}
          </Title>
          <Text type='tertiary'>
            {t('临时打印指定用户请求到服务日志和当前页面 Console')}
          </Text>
        </div>
        <Space>
          <Tag color={enabled ? 'green' : 'grey'} size='large'>
            {enabled ? t('运行中') : t('已停止')}
          </Tag>
          <Button icon={<RefreshCw size={16} />} onClick={loadStatus}>
            {t('刷新')}
          </Button>
        </Space>
      </div>

      <Banner
        type='warning'
        closeIcon={null}
        style={{ marginBottom: 16 }}
        description={t(
          '请求体会完整进入服务日志和临时 Console。关闭后不再新增，历史服务日志是否保留取决于部署环境。',
        )}
      />

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={8}>
          <Card className='!rounded-lg'>
            <div className='mb-4 flex items-center justify-between'>
              <Text strong>{t('规则')}</Text>
              <Bug size={18} />
            </div>
            <Form layout='vertical'>
              <Form.Input
                field='user_ids'
                label={t('用户 ID')}
                placeholder='123,456'
                value={formValues.user_ids}
                onChange={(value) => updateField('user_ids', value)}
              />
              <Form.Input
                field='keywords'
                label={t('关键词匹配')}
                placeholder={t('可选，多个用逗号分隔，命中后才打印')}
                value={formValues.keywords}
                onChange={(value) => updateField('keywords', value)}
                extraText={t('类似 grep：匹配 URL、Header、Body、错误信息、模型、聚合分组等已采集字段')}
              />
              <Form.Input
                field='models'
                label={t('模型')}
                placeholder={t('可选，多个用逗号分隔')}
                value={formValues.models}
                onChange={(value) => updateField('models', value)}
              />
              <Form.Input
                field='paths'
                label={t('请求路径')}
                placeholder='/v1/chat/completions,/v1/responses'
                value={formValues.paths}
                onChange={(value) => updateField('paths', value)}
              />
              <Form.Input
                field='aggregate_groups'
                label={t('聚合分组')}
                placeholder={t('可选，多个用逗号分隔')}
                value={formValues.aggregate_groups}
                onChange={(value) => updateField('aggregate_groups', value)}
              />

              <div className='mb-3 rounded-md border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] p-3'>
                <div className='mb-3 flex items-center justify-between'>
                  <Text size='small' strong>{t('高级过滤')}</Text>
                  <Text size='small' type='tertiary'>{t('可选')}</Text>
                </div>
                <Form.Input
                  field='token_ids'
                  label={t('Token ID')}
                  placeholder={t('一般不用填；多个用逗号分隔')}
                  value={formValues.token_ids}
                  onChange={(value) => updateField('token_ids', value)}
                  extraText={t('Token ID 是系统内部 API Token 记录 ID，仅用于精确限定某些令牌')}
                />
                <Form.Input
                  field='token_names'
                  label={t('令牌名称')}
                  placeholder={t('例如 test-codex，多个用逗号分隔')}
                  value={formValues.token_names}
                  onChange={(value) => updateField('token_names', value)}
                  extraText={t('客户创建令牌时填写的名称，不是 API Key，也不是数据库 ID；建议配合用户 ID 使用')}
                />
                <Checkbox checked={formValues.case_sensitive} onChange={(event) => updateField('case_sensitive', event.target.checked)}>
                  {t('关键词区分大小写')}
                </Checkbox>
              </div>

              <div className='grid grid-cols-2 gap-3'>
                <div>
                  <Text size='small'>{t('持续秒数')}</Text>
                  <InputNumber
                    className='mt-1 w-full'
                    min={1}
                    max={1800}
                    value={formValues.duration_seconds}
                    onChange={(value) => updateField('duration_seconds', value)}
                  />
                </div>
                <div>
                  <Text size='small'>{t('最大命中')}</Text>
                  <InputNumber
                    className='mt-1 w-full'
                    min={1}
                    max={100}
                    value={formValues.max_count}
                    onChange={(value) => updateField('max_count', value)}
                  />
                </div>
                <div>
                  <Text size='small'>{t('Body 上限 KB')}</Text>
                  <InputNumber
                    className='mt-1 w-full'
                    min={1}
                    max={1024}
                    value={formValues.max_body_kb}
                    onChange={(value) => updateField('max_body_kb', value)}
                  />
                </div>
                <div>
                  <Text size='small'>{t('打印时机')}</Text>
                  <Radio.Group
                    className='mt-2'
                    type='button'
                    buttonSize='small'
                    value={formValues.print_on}
                    onChange={(event) => updateField('print_on', event.target.value)}
                  >
                    <Radio value='all'>{t('全部')}</Radio>
                    <Radio value='error_only'>{t('仅错误')}</Radio>
                  </Radio.Group>
                </div>
                <div>
                  <Text size='small'>{t('日志级别')}</Text>
                  <Select
                    className='mt-1 w-full'
                    size='small'
                    value={formValues.log_level}
                    onChange={(value) => updateField('log_level', value)}
                  >
                    <Select.Option value='debug'>Debug</Select.Option>
                    <Select.Option value='info'>Info</Select.Option>
                    <Select.Option value='warn'>Warn</Select.Option>
                    <Select.Option value='error'>Error</Select.Option>
                  </Select>
                </div>
              </div>

              <div className='mt-4 grid grid-cols-2 gap-2'>
                <Checkbox checked={formValues.print_url} onChange={(event) => updateField('print_url', event.target.checked)}>
                  {t('URL')}
                </Checkbox>
                <Checkbox checked={formValues.print_headers} onChange={(event) => updateField('print_headers', event.target.checked)}>
                  {t('Header')}
                </Checkbox>
                <Checkbox checked={formValues.print_body} onChange={(event) => updateField('print_body', event.target.checked)}>
                  {t('原始 Body')}
                </Checkbox>
                <Checkbox checked={formValues.print_upstream_body} onChange={(event) => updateField('print_upstream_body', event.target.checked)}>
                  {t('上游 Body')}
                </Checkbox>
              </div>

              <div className='mt-4 rounded-md border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] p-3'>
                <div className='mb-3 flex items-center justify-between'>
                  <Text size='small' strong>{t('流式诊断')}</Text>
                  <Text size='small' type='tertiary'>{t('Responses')}</Text>
                </div>
                <div className='flex flex-col gap-2'>
                  <Checkbox checked={formValues.trace_responses_stream} onChange={(event) => updateField('trace_responses_stream', event.target.checked)}>
                    {t('Responses 流事件追踪')}
                  </Checkbox>
                  <Checkbox checked={formValues.trace_responses_stream_key_events_only} onChange={(event) => updateField('trace_responses_stream_key_events_only', event.target.checked)}>
                    {t('只记录关键事件')}
                  </Checkbox>
                  <div>
                    <Text size='small'>{t('单请求最大流事件')}</Text>
                    <InputNumber
                      className='mt-1 w-full'
                      min={1}
                      max={1000}
                      value={formValues.max_stream_events_per_request}
                      onChange={(value) => updateField('max_stream_events_per_request', value)}
                    />
                  </div>
                  <Text size='small' type='tertiary'>
                    {t('用于排查 Codex / Responses 工具调用卡住。只记录事件类型、工具项类型、错误摘要和结束原因，不记录完整响应正文。建议路径填写 /v1/responses。')}
                  </Text>
                </div>
              </div>

              <Space className='mt-5'>
                <Button type='primary' loading={loading} onClick={startDump}>
                  {t('启动')}
                </Button>
                <Button type='danger' theme='outline' loading={loading} onClick={stopDump}>
                  {t('停止')}
                </Button>
              </Space>
            </Form>
          </Card>

          <Card className='mt-4 !rounded-lg'>
            <div className='grid grid-cols-2 gap-3 text-sm'>
              <div>
                <Text type='tertiary'>{t('已命中')}</Text>
                <div className='mt-1 font-semibold'>{status?.matched_count ?? 0} / {status?.max_count ?? '-'}</div>
              </div>
              <div>
                <Text type='tertiary'>{t('剩余秒数')}</Text>
                <div className='mt-1 font-semibold'>{status?.remaining_seconds ?? 0}</div>
              </div>
              <div>
                <Text type='tertiary'>{t('启动时间')}</Text>
                <div className='mt-1'>{formatTime(status?.started_at)}</div>
              </div>
              <div>
                <Text type='tertiary'>{t('过期时间')}</Text>
                <div className='mt-1'>{formatTime(status?.expires_at)}</div>
              </div>
            </div>
          </Card>
        </Col>

        <Col xs={24} xl={16}>
          <Card className='!rounded-lg'>
            <div className='mb-3 flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
              <Space>
                <Text strong>{t('Console')}</Text>
                <Tag>{filteredEvents.length} / {events.length}</Tag>
                {paused ? <Tag color='amber'>{t('已暂停')}</Tag> : null}
              </Space>
              <Space wrap>
                <Input
                  prefix={<Search size={14} />}
                  placeholder={t('本地搜索 stage / request_id / body')}
                  value={search}
                  onChange={setSearch}
                  style={{ width: 260 }}
                />
                <Tooltip content={paused ? t('继续轮询') : t('暂停轮询')}>
                  <Button
                    icon={paused ? <Play size={16} /> : <Pause size={16} />}
                    onClick={() => setPaused((value) => !value)}
                  />
                </Tooltip>
                <Checkbox checked={following} onChange={(event) => setFollowing(event.target.checked)}>
                  {t('跟随')}
                </Checkbox>
                <Button icon={<Clipboard size={16} />} onClick={copyAll}>
                  {t('复制全部')}
                </Button>
                <Button icon={<Trash2 size={16} />} type='danger' theme='outline' onClick={clearConsole}>
                  {t('清空')}
                </Button>
              </Space>
            </div>

            <div
              ref={consoleRef}
              className='h-[640px] overflow-auto rounded-md bg-[#111827] p-3 font-mono text-xs leading-5 text-[#d1d5db]'
            >
              {filteredEvents.length === 0 ? (
                <div className='flex h-full items-center justify-center text-[#9ca3af]'>
                  {t('暂无 Dump 事件')}
                </div>
              ) : (
                filteredEvents.map((event) => (
                  <div key={event.id} className='mb-3 border-b border-[#374151] pb-3 last:border-b-0'>
                    <div className='mb-1 flex flex-wrap items-center gap-2 text-[#93c5fd]'>
                      <span>#{event.id}</span>
                      <span>{event.stage}</span>
                      <span>{event.request_id || '-'}</span>
                      <span>{event.path || '-'}</span>
                      <Button
                        size='small'
                        theme='borderless'
                        icon={<Clipboard size={13} />}
                        onClick={() => copy(eventToText(event)).then((ok) => ok && showSuccess(t('已复制')))}
                      />
                    </div>
                    <pre className='m-0 whitespace-pre-wrap break-words'>{eventToText(event)}</pre>
                  </div>
                ))
              )}
            </div>
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default RequestDumpPage;
