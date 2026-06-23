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
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  Row,
  Select,
  Spin,
  Switch,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconDelete, IconPlus, IconRefresh } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';

const { Text, Title } = Typography;

const DEFAULT_OPTIONS = {
  'async_task_setting.default_timeout_minutes': 30,
  'async_task_setting.query_limit': 1000,
  'async_task_setting.timeout_overrides': '[]',
};

const DEFAULT_IMAGE_HANDLE_CONFIG = {
  base_url: '',
  api_key: '',
  internal_base_url: '',
  internal_secret_id: 'image_handle_1',
  internal_secret: '',
  callback_secret: '',
  configured: false,
};

const PLATFORM_LABELS = {
  24: 'Gemini',
  41: 'VertexAI',
  50: 'Kling',
  51: '即梦',
  52: 'Vidu',
  48: 'xAI',
  54: '豆包视频 / Seedance',
  55: 'Sora',
  mj: 'Midjourney',
  suno: 'Suno',
};

const ACTION_LABELS = {
  generate: '生成',
  textGenerate: '文生任务',
  firstTailGenerate: '首尾帧',
  referenceGenerate: '参考生成',
  remixGenerate: 'Remix',
  videoGeneration: '视频生成',
  videoEdit: '视频编辑',
  videoExtension: '视频扩展',
};

const PLATFORM_OPTIONS = [
  { value: '48', label: 'xAI' },
  { value: '54', label: '豆包视频 / Seedance' },
  { value: '55', label: 'Sora' },
  { value: '50', label: 'Kling' },
  { value: '51', label: '即梦' },
  { value: '52', label: 'Vidu' },
  { value: '41', label: 'VertexAI' },
  { value: '24', label: 'Gemini' },
  { value: 'suno', label: 'Suno' },
  { value: 'mj', label: 'Midjourney' },
];

const ACTION_OPTIONS = [
  { value: '', label: '全部动作' },
  { value: 'videoGeneration', label: '视频生成' },
  { value: 'videoEdit', label: '视频编辑' },
  { value: 'videoExtension', label: '视频扩展' },
  { value: 'generate', label: '生成' },
  { value: 'textGenerate', label: '文生任务' },
  { value: 'firstTailGenerate', label: '首尾帧' },
  { value: 'referenceGenerate', label: '参考生成' },
  { value: 'remixGenerate', label: 'Remix' },
];

const formatPlatform = (value) => {
  const normalized = String(value || '').trim();
  if (!normalized) {
    return '';
  }
  const label = PLATFORM_LABELS[normalized];
  return label || normalized;
};

const formatAction = (value, translate = (label) => label) => {
  const normalized = String(value || '').trim();
  if (!normalized) {
    return translate('全部动作');
  }
  const label = ACTION_LABELS[normalized];
  return label ? translate(label) : normalized;
};

const normalizeOverrides = (value) => {
  if (Array.isArray(value)) {
    return value;
  }
  if (!value) {
    return [];
  }
  try {
    const parsed = JSON.parse(value);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
};

const normalizeStats = (value = {}) => ({
  total_unfinished: value.total_unfinished || 0,
  timeout_pending: value.timeout_pending || 0,
  over_10_minutes: value.over_10_minutes || 0,
  over_30_minutes: value.over_30_minutes || 0,
  over_60_minutes: value.over_60_minutes || 0,
  by_status: value.by_status || [],
  by_platform: value.by_platform || [],
  by_action: value.by_action || [],
  by_channel: value.by_channel || [],
});

const AsyncTask = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [savingImageHandle, setSavingImageHandle] = useState(false);
  const [options, setOptions] = useState(DEFAULT_OPTIONS);
  const [imageHandleConfig, setImageHandleConfig] = useState(
    DEFAULT_IMAGE_HANDLE_CONFIG,
  );
  const [stats, setStats] = useState(normalizeStats());

  const overrides = useMemo(
    () => normalizeOverrides(options['async_task_setting.timeout_overrides']),
    [options],
  );

  const loadData = async () => {
    setLoading(true);
    try {
      const [optionRes, statsRes, imageHandleRes] = await Promise.all([
        API.get('/api/option/'),
        API.get('/api/task/async/stats'),
        API.get('/api/task/async/image-handle/config'),
      ]);
      if (optionRes.data.success) {
        const next = { ...DEFAULT_OPTIONS };
        optionRes.data.data.forEach((item) => {
          if (Object.prototype.hasOwnProperty.call(next, item.key)) {
            next[item.key] = item.value;
          }
        });
        setOptions(next);
      } else {
        showError(optionRes.data.message);
      }
      if (statsRes.data.success) {
        setStats(normalizeStats(statsRes.data.data));
      } else {
        showError(statsRes.data.message);
      }
      if (imageHandleRes.data.success) {
        setImageHandleConfig({
          ...DEFAULT_IMAGE_HANDLE_CONFIG,
          ...(imageHandleRes.data.data || {}),
        });
      } else {
        showError(imageHandleRes.data.message);
      }
    } catch {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const updateOption = (key, value) => {
    setOptions((prev) => ({ ...prev, [key]: value }));
  };

  const updateOverrides = (value) => {
    updateOption('async_task_setting.timeout_overrides', JSON.stringify(value));
  };

  const updateOverride = (index, patch) => {
    updateOverrides(
      overrides.map((item, itemIndex) =>
        itemIndex === index ? { ...item, ...patch } : item,
      ),
    );
  };

  const addOverride = () => {
    updateOverrides([
      ...overrides,
      {
        platform: '48',
        action: '',
        timeout_minutes: Number(
          options['async_task_setting.default_timeout_minutes'] || 30,
        ),
        enabled: true,
      },
    ]);
  };

  const removeOverride = (index) => {
    updateOverrides(overrides.filter((_, itemIndex) => itemIndex !== index));
  };

  const saveOptions = async () => {
    setSaving(true);
    try {
      const payloads = [
        {
          key: 'async_task_setting.default_timeout_minutes',
          value: String(options['async_task_setting.default_timeout_minutes']),
        },
        {
          key: 'async_task_setting.query_limit',
          value: String(options['async_task_setting.query_limit']),
        },
        {
          key: 'async_task_setting.timeout_overrides',
          value: JSON.stringify(overrides),
        },
      ];
      const results = await Promise.all(
        payloads.map((payload) => API.put('/api/option/', payload)),
      );
      const failed = results.find((res) => !res.data.success);
      if (failed) {
        showError(failed.data.message || t('保存失败，请重试'));
        return;
      }
      showSuccess(t('保存成功'));
      await loadData();
    } catch {
      showError(t('保存失败，请重试'));
    } finally {
      setSaving(false);
    }
  };

  const updateImageHandleConfig = (key, value) => {
    setImageHandleConfig((prev) => ({ ...prev, [key]: value }));
  };

  const renderImageHandleInput = (key, label, placeholder, extraText) => (
    <Form.Slot label={label} extraText={extraText}>
      <Input
        placeholder={placeholder}
        value={imageHandleConfig[key] || ''}
        onChange={(value) => updateImageHandleConfig(key, value)}
        showClear
        style={{ width: '100%' }}
      />
    </Form.Slot>
  );

  const saveImageHandleConfig = async () => {
    setSavingImageHandle(true);
    try {
      const res = await API.put('/api/task/async/image-handle/config', {
        base_url: imageHandleConfig.base_url || '',
        api_key: imageHandleConfig.api_key || '',
        internal_base_url: imageHandleConfig.internal_base_url || '',
        internal_secret_id:
          imageHandleConfig.internal_secret_id || 'image_handle_1',
        internal_secret: imageHandleConfig.internal_secret || '',
        callback_secret: imageHandleConfig.callback_secret || '',
      });
      if (!res.data.success) {
        showError(res.data.message || t('保存失败，请重试'));
        return;
      }
      setImageHandleConfig({
        ...DEFAULT_IMAGE_HANDLE_CONFIG,
        ...(res.data.data || {}),
      });
      showSuccess(t('保存成功'));
    } catch {
      showError(t('保存失败，请重试'));
    } finally {
      setSavingImageHandle(false);
    }
  };

  const statItems = [
    { label: t('未完成任务'), value: stats.total_unfinished, color: 'blue' },
    { label: t('待超时处理'), value: stats.timeout_pending, color: 'red' },
    { label: t('超过10分钟'), value: stats.over_10_minutes, color: 'orange' },
    { label: t('超过30分钟'), value: stats.over_30_minutes, color: 'orange' },
    { label: t('超过60分钟'), value: stats.over_60_minutes, color: 'red' },
  ];

  const overrideColumns = [
    {
      title: t('平台'),
      dataIndex: 'platform',
      render: (_, record, index) => (
        <Select
          value={String(record.platform || '')}
          style={{ width: '100%' }}
          optionList={PLATFORM_OPTIONS}
          onChange={(value) => updateOverride(index, { platform: value })}
        />
      ),
    },
    {
      title: t('动作'),
      dataIndex: 'action',
      render: (_, record, index) => (
        <Select
          value={record.action || ''}
          style={{ width: '100%' }}
          optionList={ACTION_OPTIONS.map((item) => ({
            ...item,
            label: t(item.label),
          }))}
          onChange={(value) => updateOverride(index, { action: value })}
        />
      ),
    },
    {
      title: t('超时时间'),
      dataIndex: 'timeout_minutes',
      render: (_, record, index) => (
        <InputNumber
          min={1}
          step={1}
          suffix={t('分钟')}
          value={record.timeout_minutes}
          onChange={(value) =>
            updateOverride(index, { timeout_minutes: parseInt(value || 1) })
          }
        />
      ),
    },
    {
      title: t('启用'),
      dataIndex: 'enabled',
      width: 100,
      render: (_, record, index) => (
        <Switch
          checked={record.enabled !== false}
          onChange={(value) => updateOverride(index, { enabled: value })}
        />
      ),
    },
    {
      title: t('操作'),
      width: 88,
      render: (_, __, index) => (
        <Button
          icon={<IconDelete />}
          theme='borderless'
          type='danger'
          onClick={() => removeOverride(index)}
          aria-label={t('删除')}
        />
      ),
    },
  ];

  const aggregateColumns = [
    {
      title: t('维度'),
      dataIndex: 'name',
      render: (value) => <Text strong>{value || t('未设置')}</Text>,
    },
    {
      title: t('数量'),
      dataIndex: 'count',
      width: 120,
      render: (value) => <Tag color='blue'>{value}</Tag>,
    },
  ];

  const platformRows = stats.by_platform.map((item) => ({
    name: formatPlatform(item.platform),
    count: item.count,
  }));
  const actionRows = stats.by_action.map((item) => ({
    name: formatAction(item.action, t),
    count: item.count,
  }));
  const channelRows = stats.by_channel.map((item) => ({
    name: `#${item.channel_id}`,
    count: item.count,
  }));

  return (
    <div className='mt-[60px] px-2'>
      <Spin spinning={loading}>
        <div className='flex items-center justify-between mb-3'>
          <Title heading={4} style={{ margin: 0 }}>
            {t('异步任务管理')}
          </Title>
          <Button icon={<IconRefresh />} onClick={loadData}>
            {t('刷新')}
          </Button>
        </div>

        <Row gutter={[12, 12]}>
          {statItems.map((item) => (
            <Col xs={12} sm={8} md={5} lg={5} xl={5} key={item.label}>
              <Card bodyStyle={{ padding: 14 }}>
                <Text type='tertiary'>{item.label}</Text>
                <div className='mt-2'>
                  <Tag color={item.color} size='large'>
                    {item.value}
                  </Tag>
                </div>
              </Card>
            </Col>
          ))}
        </Row>

        <Card style={{ marginTop: 12 }}>
          <Title heading={5}>{t('调度设置')}</Title>
          <Row gutter={16}>
            <Col xs={24} md={8}>
              <Text>{t('默认超时时间')}</Text>
              <InputNumber
                min={1}
                step={1}
                suffix={t('分钟')}
                style={{ width: '100%', marginTop: 8 }}
                value={Number(
                  options['async_task_setting.default_timeout_minutes'],
                )}
                onChange={(value) =>
                  updateOption(
                    'async_task_setting.default_timeout_minutes',
                    parseInt(value || 30),
                  )
                }
              />
            </Col>
            <Col xs={24} md={8}>
              <Text>{t('每轮查询任务数')}</Text>
              <InputNumber
                min={1}
                step={10}
                style={{ width: '100%', marginTop: 8 }}
                value={Number(options['async_task_setting.query_limit'])}
                onChange={(value) =>
                  updateOption(
                    'async_task_setting.query_limit',
                    parseInt(value || 1000),
                  )
                }
              />
            </Col>
          </Row>
        </Card>

        <Card style={{ marginTop: 12 }}>
          <div className='flex items-center justify-between mb-3'>
            <div>
              <Title heading={5} style={{ margin: 0 }}>
                {t('异步图片执行器')}
              </Title>
              <Text type='tertiary'>
                {t(
                  '配置 new-api 提交到 image-handle 的地址，以及 image-handle resolve lease 的签名密钥。',
                )}
              </Text>
            </div>
            <Tag color={imageHandleConfig.configured ? 'green' : 'red'}>
              {imageHandleConfig.configured ? t('已配置') : t('未配置')}
            </Tag>
          </div>
          <Form layout='vertical'>
            <Row gutter={16}>
              <Col xs={24} md={12}>
                {renderImageHandleInput(
                  'base_url',
                  t('image-handle 服务地址'),
                  'http://image-handle:8787',
                  t('new-api 用它提交和轮询 image-handle 任务'),
                )}
              </Col>
              <Col xs={24} md={12}>
                {renderImageHandleInput(
                  'api_key',
                  t('image-handle API Key'),
                  t('和 image-handle 的 PROVIDER_API_KEYS 对齐'),
                )}
              </Col>
              <Col xs={24} md={12}>
                {renderImageHandleInput(
                  'internal_base_url',
                  t('internal resolve 访问地址'),
                  'http://new-api:3000',
                  t('必须是 image-handle 容器或 worker 能访问的 new-api 地址'),
                )}
              </Col>
              <Col xs={24} md={12}>
                {renderImageHandleInput(
                  'internal_secret_id',
                  t('internal resolve Secret ID'),
                  'image_handle_1',
                )}
              </Col>
              <Col xs={24} md={12}>
                {renderImageHandleInput(
                  'internal_secret',
                  t('internal resolve Secret'),
                  '',
                  t('和 image-handle 的 CREDENTIAL_LEASE_SECRETS_JSON 对齐'),
                )}
              </Col>
              <Col xs={24} md={12}>
                {renderImageHandleInput(
                  'callback_secret',
                  t('Callback 兜底 Secret'),
                  '',
                  t('建议填写。作为 image-handle callback 默认验签密钥；单个图片渠道可用 settings.callback_secret 覆盖'),
                )}
              </Col>
            </Row>
            <Button
              type='primary'
              loading={savingImageHandle}
              onClick={saveImageHandleConfig}
            >
              {t('保存执行器配置')}
            </Button>
          </Form>
        </Card>

        <Card style={{ marginTop: 12 }}>
          <div className='flex items-center justify-between mb-3'>
            <Title heading={5} style={{ margin: 0 }}>
              {t('超时覆盖')}
            </Title>
            <Button icon={<IconPlus />} onClick={addOverride}>
              {t('新增覆盖')}
            </Button>
          </div>
          <Table
            columns={overrideColumns}
            dataSource={overrides.map((item, index) => ({ ...item, key: index }))}
            pagination={false}
            size='small'
          />
          <div className='mt-4'>
            <Button type='primary' loading={saving} onClick={saveOptions}>
              {t('保存设置')}
            </Button>
          </div>
        </Card>

        <Row gutter={[12, 12]} style={{ marginTop: 12 }}>
          <Col xs={24} lg={8}>
            <Card title={t('按平台')}>
              <Table
                columns={aggregateColumns}
                dataSource={platformRows}
                pagination={false}
                size='small'
              />
            </Card>
          </Col>
          <Col xs={24} lg={8}>
            <Card title={t('按动作')}>
              <Table
                columns={aggregateColumns}
                dataSource={actionRows}
                pagination={false}
                size='small'
              />
            </Card>
          </Col>
          <Col xs={24} lg={8}>
            <Card title={t('按渠道')}>
              <Table
                columns={aggregateColumns}
                dataSource={channelRows}
                pagination={false}
                size='small'
              />
            </Card>
          </Col>
        </Row>
      </Spin>
    </div>
  );
};

export default AsyncTask;
