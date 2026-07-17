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
  Switch,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { ArrowRight, Copy, KeyRound, Save, Send } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { copy, showError, showSuccess } from '../../../helpers';
import { useWebhooks, webhookErrorMessage } from './useWebhooks';

const { Text, Title } = Typography;

export default function WebhookTab({ onOpenApiKeys }) {
  const { t } = useTranslation();
  const api = useWebhooks();
  const [url, setUrl] = useState('');
  const [enabledDraft, setEnabledDraft] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);

  const load = async () => {
    try {
      const config = await api.loadConfig();
      setUrl(config.url || '');
      setEnabledDraft(config.status === 'enabled');
    } catch (error) {
      showError(webhookErrorMessage(error, t('加载 Webhook 配置失败')));
    }
  };

  useEffect(() => {
    load();
  }, []);

  const save = async () => {
    const normalizedURL = url.trim();
    if (enabledDraft && !normalizedURL) {
      showError(t('填写回调 URL 后才能启用 Webhook'));
      return;
    }
    if (enabledDraft && !api.config.resource_key_configured) {
      showError(t('请先生成并启用资源 API Key'));
      return;
    }
    setSaving(true);
    try {
      const response = await api.saveConfig({
        url: normalizedURL,
        enabled: enabledDraft,
      });
      setUrl(response.url || '');
      setEnabledDraft(response.status === 'enabled');
      showSuccess(t('Webhook 配置已保存'));
    } catch (error) {
      showError(webhookErrorMessage(error, t('保存 Webhook 配置失败')));
    } finally {
      setSaving(false);
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

  const enabled = api.config.configured && api.config.status === 'enabled';
  const dirty =
    url.trim() !== (api.config.url || '') || enabledDraft !== enabled;

  const changeEnabled = (nextEnabled) => {
    if (nextEnabled && !url.trim()) {
      showError(t('填写回调 URL 后才能启用 Webhook'));
      return;
    }
    if (nextEnabled && !api.config.resource_key_configured) {
      showError(t('请先生成并启用资源 API Key'));
      return;
    }
    setEnabledDraft(nextEnabled);
  };

  const copyURL = async () => {
    if (await copy(api.config.url)) showSuccess(t('Webhook 地址已复制'));
  };

  return (
    <section className='flex max-w-3xl flex-col gap-5'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between'>
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
        <div className='flex min-h-11 shrink-0 items-center gap-3'>
          <Tag color={enabledDraft ? 'green' : 'grey'}>
            {enabledDraft ? t('启用') : t('停用')}
          </Tag>
          <Text strong>{t('启用 Webhook')}</Text>
          <Switch
            checked={enabledDraft}
            disabled={api.loading || saving}
            aria-label={t('启用 Webhook')}
            onChange={changeEnabled}
          />
        </div>
      </div>

      <label className='flex flex-col gap-2'>
        <Text strong>{t('Webhook地址')}</Text>
        <div className='flex min-w-0 gap-2'>
          <Input
            value={url}
            onChange={setUrl}
            placeholder='https://example.com/webhook'
            disabled={api.loading || saving}
            showClear
          />
          {api.config.url && !dirty && (
            <Tooltip content={t('复制 Webhook 地址')}>
              <Button
                icon={<Copy size={17} />}
                aria-label={t('复制 Webhook 地址')}
                onClick={copyURL}
              />
            </Tooltip>
          )}
        </div>
        <Text type='tertiary' size='small'>
          {t('填写回调 URL，打开启用开关并保存即可。')}
        </Text>
      </label>

      <div className='border-y border-semi-color-border py-4'>
        <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <div className='flex min-w-0 items-start gap-3'>
            <KeyRound
              size={20}
              className='mt-0.5 shrink-0 text-semi-color-primary'
            />
            <div className='min-w-0'>
              <Text strong>{t('回调验证使用资源 API Key')}</Text>
              <Text type='tertiary' size='small' className='mt-1 block'>
                {api.config.resource_key_configured
                  ? t('当前使用资源 API Key（ak_...）')
                  : t('尚未配置启用中的资源 API Key')}
              </Text>
            </div>
          </div>
          <Button icon={<ArrowRight size={16} />} onClick={onOpenApiKeys}>
            {api.config.resource_key_configured
              ? t('查看资源 API Key')
              : t('生成资源 API Key')}
          </Button>
        </div>
      </div>

      <div className='flex flex-wrap items-center gap-2'>
        <Button
          type='primary'
          icon={<Save size={16} />}
          loading={saving}
          onClick={save}
        >
          {t('保存配置')}
        </Button>
        <Tooltip content={dirty ? t('请先保存当前配置') : undefined}>
          <span>
            <Button
              icon={<Send size={16} />}
              loading={testing}
              disabled={!enabled || dirty}
              onClick={sendTest}
            >
              {t('发送测试')}
            </Button>
          </span>
        </Tooltip>
        {dirty && (
          <Text type='warning' size='small'>
            {t('有未保存的更改')}
          </Text>
        )}
      </div>
    </section>
  );
}
