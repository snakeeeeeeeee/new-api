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

import React, { useEffect, useState } from 'react';
import {
  Button,
  Input,
  Popconfirm,
  Space,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Copy,
  Eye,
  EyeOff,
  Pencil,
  Power,
  PowerOff,
  RefreshCcw,
  Save,
  Send,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { copy, showError, showSuccess } from '../../../helpers';
import { useWebhooks, webhookErrorMessage } from './useWebhooks';

const { Text, Title } = Typography;

export default function WebhookTab() {
  const { t } = useTranslation();
  const api = useWebhooks();
  const [url, setUrl] = useState('');
  const [editing, setEditing] = useState(false);
  const [keyVisible, setKeyVisible] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [disabling, setDisabling] = useState(false);
  const [enabling, setEnabling] = useState(false);
  const [regenerating, setRegenerating] = useState(false);

  const load = async () => {
    try {
      const config = await api.loadConfig();
      setUrl(config.url || '');
      setEditing(false);
      setKeyVisible(false);
    } catch (error) {
      showError(webhookErrorMessage(error, t('加载 Webhook 配置失败')));
    }
  };

  useEffect(() => {
    load();
  }, []);

  const save = async () => {
    const normalizedURL = url.trim();
    if (!normalizedURL) {
      showError(t('请输入 Webhook URL'));
      return;
    }
    setSaving(true);
    try {
      const response = await api.saveConfig({ url: normalizedURL });
      setKeyVisible(!api.config.configured && Boolean(response.key));
      setEditing(false);
      showSuccess(t('Webhook 配置已保存'));
    } catch (error) {
      showError(webhookErrorMessage(error, t('保存 Webhook 配置失败')));
    } finally {
      setSaving(false);
    }
  };

  const enable = async () => {
    setEnabling(true);
    try {
      await api.saveConfig({ url: api.config.url });
      showSuccess(t('Webhook 已启用'));
    } catch (error) {
      showError(webhookErrorMessage(error, t('启用 Webhook 失败')));
    } finally {
      setEnabling(false);
    }
  };

  const regenerateKey = async () => {
    setRegenerating(true);
    try {
      const response = await api.saveConfig({
        url: api.config.url,
        regenerate_key: true,
      });
      setKeyVisible(Boolean(response.key));
      showSuccess(t('Webhook 密钥已重新生成'));
    } catch (error) {
      showError(webhookErrorMessage(error, t('重新生成 Webhook 密钥失败')));
    } finally {
      setRegenerating(false);
    }
  };

  const sendTest = async () => {
    setTesting(true);
    try {
      await api.testConfig();
      showSuccess(t('测试事件已进入投递队列'));
    } catch (error) {
      showError(webhookErrorMessage(error, t('测试 Webhook 失败')));
    } finally {
      setTesting(false);
    }
  };

  const disable = async () => {
    setDisabling(true);
    try {
      await api.disableConfig();
      showSuccess(t('Webhook 已停用'));
    } catch (error) {
      showError(webhookErrorMessage(error, t('停用 Webhook 失败')));
    } finally {
      setDisabling(false);
    }
  };

  const enabled = api.config.configured && api.config.status === 'enabled';
  const configured = api.config.configured;
  const showEditor = !configured || editing;

  const beginEdit = () => {
    setUrl(api.config.url || '');
    setEditing(true);
  };

  const cancelEdit = () => {
    setUrl(api.config.url || '');
    setEditing(false);
  };

  const copyURL = async () => {
    if (await copy(api.config.url)) showSuccess(t('Webhook 地址已复制'));
  };

  const copyKey = async () => {
    if (await copy(api.config.key)) showSuccess(t('Webhook 密钥已复制'));
  };

  return (
    <section className='max-w-2xl flex flex-col gap-5'>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <Title heading={5} className='!mb-1'>
            {t('任务 Webhook')}
          </Title>
          <Text type='tertiary'>
            {t('图片任务完成后发送成功或失败事件；后续任务类型共用此配置')}
          </Text>
          <Text type='tertiary' size='small' className='mt-1 block'>
            {t('每个事件只发送一次，不检查响应，也不会重试')}
          </Text>
        </div>
        <Tag color={enabled ? 'green' : 'grey'} style={{ flexShrink: 0 }}>
          {enabled ? t('启用') : t('停用')}
        </Tag>
      </div>

      {showEditor ? (
        <div className='flex flex-col gap-4'>
          <label className='flex flex-col gap-2'>
            <Text strong>{t('Webhook地址')}</Text>
            <Input
              value={url}
              onChange={setUrl}
              placeholder='https://example.com/webhook'
              disabled={api.loading}
              showClear
            />
          </label>
          <Text type='tertiary'>
            {configured
              ? t('地址修改不会更改现有密钥')
              : t('保存后将生成一个 wk- 开头的密钥')}
          </Text>
          <Space wrap>
            <Button
              type='primary'
              icon={<Save size={16} />}
              loading={saving}
              onClick={save}
            >
              {configured ? t('保存修改') : t('创建并生成密钥')}
            </Button>
            {configured && <Button onClick={cancelEdit}>{t('取消')}</Button>}
          </Space>
        </div>
      ) : (
        <>
          <div className='border-y border-semi-color-border divide-y divide-semi-color-border'>
            <div className='grid grid-cols-1 sm:grid-cols-[140px_minmax(0,1fr)_44px] gap-2 sm:gap-4 sm:items-center py-4'>
              <Text type='tertiary'>{t('Webhook地址')}</Text>
              <Text className='break-all'>{api.config.url}</Text>
              <Tooltip content={t('复制 Webhook 地址')}>
                <Button
                  theme='borderless'
                  type='tertiary'
                  icon={<Copy size={17} />}
                  aria-label={t('复制 Webhook 地址')}
                  onClick={copyURL}
                />
              </Tooltip>
            </div>
            <div className='grid grid-cols-1 sm:grid-cols-[140px_minmax(0,1fr)_auto] gap-2 sm:gap-4 sm:items-center py-4'>
              <Text type='tertiary'>{t('Webhook 验证 Key')}</Text>
              <div className='min-w-0'>
                <Text code className='break-all'>
                  {api.config.key_configured
                    ? keyVisible
                      ? api.config.key
                      : '••••••••••••••••'
                    : '—'}
                </Text>
                {!api.config.key_configured && (
                  <Text type='danger' size='small' className='ml-2'>
                    {t('需要重新生成')}
                  </Text>
                )}
              </div>
              <Space>
                {api.config.key_configured && (
                  <>
                    <Tooltip
                      content={keyVisible ? t('隐藏密钥') : t('显示密钥')}
                    >
                      <Button
                        theme='borderless'
                        type='tertiary'
                        icon={
                          keyVisible ? <EyeOff size={17} /> : <Eye size={17} />
                        }
                        aria-label={keyVisible ? t('隐藏密钥') : t('显示密钥')}
                        onClick={() => setKeyVisible((current) => !current)}
                      />
                    </Tooltip>
                    <Tooltip content={t('复制密钥')}>
                      <Button
                        theme='borderless'
                        type='tertiary'
                        icon={<Copy size={17} />}
                        aria-label={t('复制密钥')}
                        onClick={copyKey}
                      />
                    </Tooltip>
                  </>
                )}
                <Popconfirm
                  title={t('确定重新生成 Webhook 密钥？')}
                  content={t('旧密钥将立即失效，请先准备好更新接收端配置。')}
                  onConfirm={regenerateKey}
                >
                  <Button
                    theme='borderless'
                    type='tertiary'
                    icon={<RefreshCcw size={16} />}
                    loading={regenerating}
                    aria-label={t('重新生成')}
                    title={t('重新生成')}
                  />
                </Popconfirm>
              </Space>
            </div>
          </div>

          <Space wrap>
            <Button icon={<Pencil size={16} />} onClick={beginEdit}>
              {t('编辑')}
            </Button>
            {enabled ? (
              <Button
                icon={<Send size={16} />}
                loading={testing}
                onClick={sendTest}
              >
                {t('发送测试')}
              </Button>
            ) : (
              <Button
                type='primary'
                icon={<Power size={16} />}
                loading={enabling}
                onClick={enable}
              >
                {t('启用')}
              </Button>
            )}
            {enabled && (
              <Popconfirm
                title={t('确定停用任务 Webhook？')}
                content={t('再次保存配置即可重新启用')}
                onConfirm={disable}
              >
                <Button
                  type='danger'
                  icon={<PowerOff size={16} />}
                  loading={disabling}
                >
                  {t('停用')}
                </Button>
              </Popconfirm>
            )}
          </Space>
        </>
      )}
    </section>
  );
}
