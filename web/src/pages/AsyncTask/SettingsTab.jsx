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
import { IconDelete, IconPlus } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';

const { Text, Title } = Typography;

const DEFAULT_OPTIONS = {
  'async_task_setting.default_timeout_minutes': 30,
  'async_task_setting.query_limit': 1000,
  'async_task_setting.webhook_max_attempts': 3,
  'async_task_setting.webhook_retry_interval_seconds': 30,
  'async_task_setting.image_dispatch_concurrency': 20,
  'async_task_setting.webhook_delivery_concurrency': 20,
  'async_task_setting.webhook_endpoint_concurrency': 2,
  'async_task_setting.image_dispatch_request_timeout_seconds': 30,
  'async_task_setting.webhook_delivery_request_timeout_seconds': 10,
  'async_task_setting.timeout_overrides': '[]',
};

const DEFAULT_IMAGE_HANDLE_CONFIG = {
  base_url: '',
  api_key: '',
  internal_base_url: '',
  internal_secret_id: 'image_handle_1',
  internal_secret: '',
  callback_secret: '',
  debug_upstream: false,
  sync_image_enabled: false,
  sync_image_result_policy: 'follow_request',
  sync_image_default_format: 'url',
  usage_precharge_enabled: true,
  precharge_amount_per_image: 0,
  configured: false,
};

const PLATFORM_OPTIONS = [
  ['48', 'xAI'],
  ['54', '豆包视频 / Seedance'],
  ['55', 'Sora'],
  ['50', 'Kling'],
  ['51', '即梦'],
  ['52', 'Vidu'],
  ['41', 'VertexAI'],
  ['24', 'Gemini'],
  ['suno', 'Suno'],
  ['mj', 'Midjourney'],
].map(([value, label]) => ({ value, label }));

const ACTION_OPTIONS = [
  ['', '全部动作'],
  ['videoGeneration', '视频生成'],
  ['videoEdit', '视频编辑'],
  ['videoExtension', '视频扩展'],
  ['generate', '生成'],
  ['textGenerate', '文生任务'],
  ['firstTailGenerate', '首尾帧'],
  ['referenceGenerate', '参考生成'],
  ['remixGenerate', 'Remix'],
].map(([value, label]) => ({ value, label }));

const normalizeOverrides = (value) => {
  if (Array.isArray(value)) return value;
  try {
    const parsed = JSON.parse(value || '[]');
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
};

const normalizeMoneyDisplay = (value) => {
  const numeric = Number(value);
  return Number.isFinite(numeric) ? String(Number(numeric.toFixed(6))) : '';
};

const normalizeMoneyInput = (value) => {
  const normalized = String(value).replace(/[^\d.]/g, '');
  const parts = normalized.split('.');
  return parts.length > 1 ? `${parts[0]}.${parts.slice(1).join('')}` : parts[0];
};

const SettingsTab = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [savingImageHandle, setSavingImageHandle] = useState(false);
  const [options, setOptions] = useState(DEFAULT_OPTIONS);
  const [imageHandleConfig, setImageHandleConfig] = useState(
    DEFAULT_IMAGE_HANDLE_CONFIG,
  );

  const overrides = useMemo(
    () => normalizeOverrides(options['async_task_setting.timeout_overrides']),
    [options],
  );

  const loadSettings = async () => {
    setLoading(true);
    try {
      const [optionResponse, imageHandleResponse] = await Promise.all([
        API.get('/api/option/'),
        API.get('/api/task/async/image-handle/config'),
      ]);
      if (optionResponse.data.success) {
        const next = { ...DEFAULT_OPTIONS };
        optionResponse.data.data.forEach((item) => {
          if (Object.prototype.hasOwnProperty.call(next, item.key))
            next[item.key] = item.value;
        });
        setOptions(next);
      } else showError(optionResponse.data.message || t('加载失败'));
      if (imageHandleResponse.data.success) {
        setImageHandleConfig({
          ...DEFAULT_IMAGE_HANDLE_CONFIG,
          ...(imageHandleResponse.data.data || {}),
          precharge_amount_per_image: normalizeMoneyDisplay(
            imageHandleResponse.data.data?.precharge_amount_per_image || 0,
          ),
        });
      } else showError(imageHandleResponse.data.message || t('加载失败'));
    } catch {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadSettings();
  }, []);

  const updateOption = (key, value) =>
    setOptions((current) => ({ ...current, [key]: value }));
  const updateImageHandle = (key, value) =>
    setImageHandleConfig((current) => ({ ...current, [key]: value }));
  const updateOverrides = (value) =>
    updateOption('async_task_setting.timeout_overrides', JSON.stringify(value));

  const saveOptions = async () => {
    setSaving(true);
    try {
      const keys = Object.keys(DEFAULT_OPTIONS);
      const responses = await Promise.all(
        keys.map((key) =>
          API.put('/api/option/', {
            key,
            value:
              key === 'async_task_setting.timeout_overrides'
                ? JSON.stringify(overrides)
                : String(options[key]),
          }),
        ),
      );
      const failed = responses.find((response) => !response.data.success);
      if (failed) {
        showError(failed.data.message || t('保存失败，请重试'));
        return;
      }
      showSuccess(t('保存成功'));
      await loadSettings();
    } catch {
      showError(t('保存失败，请重试'));
    } finally {
      setSaving(false);
    }
  };

  const saveImageHandleConfig = async () => {
    const prechargeAmount = Number(
      imageHandleConfig.precharge_amount_per_image,
    );
    if (
      imageHandleConfig.usage_precharge_enabled &&
      (!Number.isFinite(prechargeAmount) || prechargeAmount <= 0)
    ) {
      showError(t('开启异步图片预扣估算时，请填写大于 0 的每张图预扣费用'));
      return;
    }
    setSavingImageHandle(true);
    try {
      const response = await API.put('/api/task/async/image-handle/config', {
        base_url: imageHandleConfig.base_url || '',
        api_key: imageHandleConfig.api_key || '',
        internal_base_url: imageHandleConfig.internal_base_url || '',
        internal_secret_id:
          imageHandleConfig.internal_secret_id || 'image_handle_1',
        internal_secret: imageHandleConfig.internal_secret || '',
        callback_secret: imageHandleConfig.callback_secret || '',
        debug_upstream: Boolean(imageHandleConfig.debug_upstream),
        sync_image_enabled: Boolean(imageHandleConfig.sync_image_enabled),
        sync_image_result_policy:
          imageHandleConfig.sync_image_result_policy || 'follow_request',
        sync_image_default_format:
          imageHandleConfig.sync_image_default_format || 'url',
        usage_precharge_enabled: Boolean(
          imageHandleConfig.usage_precharge_enabled,
        ),
        precharge_amount_per_image: Number.isFinite(prechargeAmount)
          ? prechargeAmount
          : 0,
      });
      if (!response.data.success) {
        showError(response.data.message || t('保存失败，请重试'));
        return;
      }
      setImageHandleConfig({
        ...DEFAULT_IMAGE_HANDLE_CONFIG,
        ...(response.data.data || {}),
        precharge_amount_per_image: normalizeMoneyDisplay(
          response.data.data?.precharge_amount_per_image || 0,
        ),
      });
      showSuccess(t('保存成功'));
    } catch {
      showError(t('保存失败，请重试'));
    } finally {
      setSavingImageHandle(false);
    }
  };

  const numericSetting = (key, label, min, max, suffix) => (
    <Form.Slot label={label}>
      <InputNumber
        min={min}
        max={max}
        suffix={suffix}
        style={{ width: '100%' }}
        value={Number(options[key])}
        onChange={(value) => updateOption(key, parseInt(value || min))}
      />
    </Form.Slot>
  );

  const secretInput = (key, label, password = false) => (
    <Form.Slot label={label}>
      <Input
        mode={password ? 'password' : 'text'}
        value={imageHandleConfig[key] || ''}
        onChange={(value) => updateImageHandle(key, value)}
        showClear
      />
    </Form.Slot>
  );

  const overrideColumns = [
    {
      title: t('平台'),
      dataIndex: 'platform',
      render: (_, record, index) => (
        <Select
          value={record.platform || ''}
          style={{ width: '100%' }}
          optionList={PLATFORM_OPTIONS}
          onChange={(value) =>
            updateOverrides(
              overrides.map((item, itemIndex) =>
                itemIndex === index ? { ...item, platform: value } : item,
              ),
            )
          }
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
          onChange={(value) =>
            updateOverrides(
              overrides.map((item, itemIndex) =>
                itemIndex === index ? { ...item, action: value } : item,
              ),
            )
          }
        />
      ),
    },
    {
      title: t('超时时间'),
      dataIndex: 'timeout_minutes',
      width: 170,
      render: (_, record, index) => (
        <InputNumber
          min={1}
          suffix={t('分钟')}
          value={record.timeout_minutes}
          onChange={(value) =>
            updateOverrides(
              overrides.map((item, itemIndex) =>
                itemIndex === index
                  ? { ...item, timeout_minutes: parseInt(value || 1) }
                  : item,
              ),
            )
          }
        />
      ),
    },
    {
      title: t('启用'),
      dataIndex: 'enabled',
      width: 90,
      render: (_, record, index) => (
        <Switch
          checked={record.enabled !== false}
          onChange={(value) =>
            updateOverrides(
              overrides.map((item, itemIndex) =>
                itemIndex === index ? { ...item, enabled: value } : item,
              ),
            )
          }
        />
      ),
    },
    {
      title: t('操作'),
      width: 72,
      render: (_, __, index) => (
        <Button
          theme='borderless'
          type='danger'
          icon={<IconDelete />}
          aria-label={t('删除')}
          onClick={() =>
            updateOverrides(
              overrides.filter((_, itemIndex) => itemIndex !== index),
            )
          }
        />
      ),
    },
  ];

  return (
    <Spin spinning={loading}>
      <div className='flex min-h-[420px] flex-col gap-3'>
        <Card bodyStyle={{ padding: 16 }}>
          <Title heading={5} style={{ margin: '0 0 16px' }}>
            {t('Worker 调度')}
          </Title>
          <Form layout='vertical'>
            <Row gutter={[16, 8]}>
              <Col xs={24} md={12} xl={6}>
                {numericSetting(
                  'async_task_setting.image_dispatch_concurrency',
                  t('生图分发并发'),
                  1,
                  100,
                )}
              </Col>
              <Col xs={24} md={12} xl={6}>
                {numericSetting(
                  'async_task_setting.image_dispatch_request_timeout_seconds',
                  t('生图请求超时'),
                  1,
                  300,
                  t('秒'),
                )}
              </Col>
              <Col xs={24} md={12} xl={6}>
                {numericSetting(
                  'async_task_setting.webhook_delivery_concurrency',
                  t('Webhook 总并发'),
                  1,
                  100,
                )}
              </Col>
              <Col xs={24} md={12} xl={6}>
                {numericSetting(
                  'async_task_setting.webhook_endpoint_concurrency',
                  t('Webhook 单端点并发'),
                  1,
                  Math.min(
                    10,
                    Number(
                      options[
                        'async_task_setting.webhook_delivery_concurrency'
                      ],
                    ) || 1,
                  ),
                )}
              </Col>
              <Col xs={24} md={12} xl={6}>
                {numericSetting(
                  'async_task_setting.webhook_delivery_request_timeout_seconds',
                  t('Webhook 请求超时'),
                  1,
                  300,
                  t('秒'),
                )}
              </Col>
              <Col xs={24} md={12} xl={6}>
                {numericSetting(
                  'async_task_setting.webhook_max_attempts',
                  t('Webhook 最大尝试次数（含首次）'),
                  1,
                  10,
                )}
              </Col>
              <Col xs={24} md={12} xl={6}>
                {numericSetting(
                  'async_task_setting.webhook_retry_interval_seconds',
                  t('Webhook 重试间隔'),
                  1,
                  3600,
                  t('秒'),
                )}
              </Col>
              <Col xs={24} md={12} xl={6}>
                {numericSetting(
                  'async_task_setting.default_timeout_minutes',
                  t('默认任务超时'),
                  1,
                  undefined,
                  t('分钟'),
                )}
              </Col>
              <Col xs={24} md={12} xl={6}>
                {numericSetting(
                  'async_task_setting.query_limit',
                  t('任务轮询查询上限'),
                  1,
                  undefined,
                )}
              </Col>
            </Row>
          </Form>
        </Card>

        <Card bodyStyle={{ padding: 16 }}>
          <div className='mb-4 flex flex-wrap items-center justify-between gap-2'>
            <Title heading={5} style={{ margin: 0 }}>
              {t('异步图片执行器')}
            </Title>
            <Tag color={imageHandleConfig.configured ? 'green' : 'red'}>
              {imageHandleConfig.configured ? t('已配置') : t('未配置')}
            </Tag>
          </div>
          <Form layout='vertical'>
            <Row gutter={[16, 8]}>
              <Col xs={24} md={12}>
                {secretInput('base_url', t('image-handle 服务地址'))}
              </Col>
              <Col xs={24} md={12}>
                {secretInput('api_key', t('image-handle API Key'), true)}
              </Col>
              <Col xs={24} md={12}>
                {secretInput(
                  'internal_base_url',
                  t('internal resolve 访问地址'),
                )}
              </Col>
              <Col xs={24} md={12}>
                {secretInput(
                  'internal_secret_id',
                  t('internal resolve Secret ID'),
                )}
              </Col>
              <Col xs={24} md={12}>
                {secretInput(
                  'internal_secret',
                  t('internal resolve Secret'),
                  true,
                )}
              </Col>
              <Col xs={24} md={12}>
                {secretInput(
                  'callback_secret',
                  t('Callback 兜底 Secret'),
                  true,
                )}
              </Col>
              <Col xs={24} md={8}>
                <Form.Slot label={t('上游调试日志')}>
                  <Switch
                    checked={Boolean(imageHandleConfig.debug_upstream)}
                    onChange={(value) =>
                      updateImageHandle('debug_upstream', value)
                    }
                  />
                </Form.Slot>
              </Col>
              <Col xs={24} md={8}>
                <Form.Slot label={t('经 image-handle 同步执行')}>
                  <Switch
                    checked={Boolean(imageHandleConfig.sync_image_enabled)}
                    onChange={(value) =>
                      updateImageHandle('sync_image_enabled', value)
                    }
                  />
                </Form.Slot>
              </Col>
              <Col xs={24} md={8}>
                <Form.Slot label={t('启用预扣估算')}>
                  <Switch
                    checked={Boolean(imageHandleConfig.usage_precharge_enabled)}
                    onChange={(value) =>
                      updateImageHandle('usage_precharge_enabled', value)
                    }
                  />
                </Form.Slot>
              </Col>
              <Col xs={24} md={8}>
                <Form.Slot label={t('返回格式策略')}>
                  <Select
                    style={{ width: '100%' }}
                    value={imageHandleConfig.sync_image_result_policy}
                    optionList={[
                      ['follow_request', '跟随请求参数'],
                      ['force_url', '强制 URL'],
                      ['force_base64', '强制 Base64'],
                    ].map(([value, label]) => ({ value, label: t(label) }))}
                    onChange={(value) =>
                      updateImageHandle('sync_image_result_policy', value)
                    }
                  />
                </Form.Slot>
              </Col>
              <Col xs={24} md={8}>
                <Form.Slot label={t('未指定时默认格式')}>
                  <Select
                    style={{ width: '100%' }}
                    disabled={
                      imageHandleConfig.sync_image_result_policy !==
                      'follow_request'
                    }
                    value={imageHandleConfig.sync_image_default_format}
                    optionList={[
                      { value: 'url', label: 'URL' },
                      { value: 'base64', label: 'Base64' },
                    ]}
                    onChange={(value) =>
                      updateImageHandle('sync_image_default_format', value)
                    }
                  />
                </Form.Slot>
              </Col>
              <Col xs={24} md={8}>
                <Form.Slot label={t('每张图预扣费用（$）')}>
                  <Input
                    disabled={!imageHandleConfig.usage_precharge_enabled}
                    value={imageHandleConfig.precharge_amount_per_image ?? '0'}
                    onChange={(value) =>
                      updateImageHandle(
                        'precharge_amount_per_image',
                        normalizeMoneyInput(value),
                      )
                    }
                  />
                </Form.Slot>
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

        <Card bodyStyle={{ padding: 16 }}>
          <div className='mb-3 flex flex-wrap items-center justify-between gap-2'>
            <Title heading={5} style={{ margin: 0 }}>
              {t('超时覆盖')}
            </Title>
            <Button
              icon={<IconPlus />}
              onClick={() =>
                updateOverrides([
                  ...overrides,
                  {
                    platform: '48',
                    action: '',
                    timeout_minutes: Number(
                      options['async_task_setting.default_timeout_minutes'] ||
                        30,
                    ),
                    enabled: true,
                  },
                ])
              }
            >
              {t('新增覆盖')}
            </Button>
          </div>
          <div className='max-w-full overflow-x-auto'>
            <Table
              columns={overrideColumns}
              dataSource={overrides.map((item, index) => ({
                ...item,
                key: index,
              }))}
              pagination={false}
              size='small'
              scroll={{ x: 'max-content' }}
            />
          </div>
          <div className='mt-4'>
            <Button type='primary' loading={saving} onClick={saveOptions}>
              {t('保存 Worker 与超时设置')}
            </Button>
          </div>
        </Card>
      </div>
    </Spin>
  );
};

export default SettingsTab;
