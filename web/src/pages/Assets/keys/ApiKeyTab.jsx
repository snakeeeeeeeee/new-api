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
  Popconfirm,
  Space,
  Spin,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Copy,
  Eye,
  EyeOff,
  KeyRound,
  Power,
  PowerOff,
  RefreshCcw,
  Trash2,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  API,
  copy,
  showError,
  showSuccess,
  timestamp2string,
} from '../../../helpers';

const { Text, Title } = Typography;
const KEY_STATUS_ENABLED = 1;
const KEY_STATUS_DISABLED = 2;

function maskResourceKey(value) {
  if (!value) return '';
  if (value.length <= 12) return `${value.slice(0, 4)}••••${value.slice(-2)}`;
  return `${value.slice(0, 7)}••••••••••${value.slice(-4)}`;
}

export default function ApiKeyTab() {
  const { t } = useTranslation();
  const [resourceKey, setResourceKey] = useState(null);
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [visible, setVisible] = useState(false);
  const [actionKeyID, setActionKeyID] = useState(0);

  const loadKey = async () => {
    setLoading(true);
    try {
      const response = await API.get('/api/assets/keys?p=1&page_size=1');
      const { success, data, message } = response.data || {};
      if (!success) {
        showError(message || t('加载资源 API Key 失败'));
        return;
      }
      setResourceKey(data?.items?.[0] || null);
      setVisible(false);
    } catch (error) {
      showError(error?.message || t('加载资源 API Key 失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadKey();
  }, []);

  const generateKey = async () => {
    setCreating(true);
    try {
      const response = await API.post('/api/assets/keys', {
        name: 'resource-center',
        expired_at: -1,
        allow_ips: '',
      });
      const { success, data, message } = response.data || {};
      if (!success || !data?.key) {
        showError(message || t('生成资源 API Key 失败'));
        return;
      }
      setResourceKey(data);
      setVisible(true);
      showSuccess(t('资源 API Key 已生成'));
    } catch (error) {
      showError(error?.message || t('生成资源 API Key 失败'));
    } finally {
      setCreating(false);
    }
  };

  const changeStatus = async () => {
    if (!resourceKey) return;
    const nextStatus =
      resourceKey.status === KEY_STATUS_ENABLED
        ? KEY_STATUS_DISABLED
        : KEY_STATUS_ENABLED;
    setActionKeyID(resourceKey.id);
    try {
      const response = await API.put(
        `/api/assets/keys/${resourceKey.id}/status`,
        { status: nextStatus },
      );
      const { success, message } = response.data || {};
      if (!success) {
        showError(message || t('更新资源 API Key 状态失败'));
        return;
      }
      setResourceKey((current) => ({ ...current, status: nextStatus }));
      showSuccess(
        nextStatus === KEY_STATUS_ENABLED
          ? t('资源 API Key 已启用')
          : t('资源 API Key 已停用'),
      );
    } catch (error) {
      showError(error?.message || t('更新资源 API Key 状态失败'));
    } finally {
      setActionKeyID(0);
    }
  };

  const deleteKey = async () => {
    if (!resourceKey) return;
    setActionKeyID(resourceKey.id);
    try {
      const response = await API.delete(`/api/assets/keys/${resourceKey.id}`);
      const { success, message } = response.data || {};
      if (!success) {
        showError(message || t('删除资源 API Key 失败'));
        return;
      }
      setResourceKey(null);
      setVisible(false);
      showSuccess(t('资源 API Key 已删除'));
    } catch (error) {
      showError(error?.message || t('删除资源 API Key 失败'));
    } finally {
      setActionKeyID(0);
    }
  };

  const copyKey = async () => {
    if (resourceKey && (await copy(resourceKey.key))) {
      showSuccess(t('资源 API Key 已复制'));
    }
  };

  const enabled = resourceKey?.status === KEY_STATUS_ENABLED;

  return (
    <section className='flex max-w-3xl flex-col gap-5'>
      <div className='min-w-0'>
        <Title heading={5} className='!mb-1'>
          {t('资源 API Key')}
        </Title>
        <Text type='tertiary'>
          {t(
            '资源 API Key 用于任务查询、预上传和资源访问；异步生图提交使用普通 API Token。',
          )}
        </Text>
      </div>

      <Text type='tertiary' size='small'>
        {t(
          '资源 API Key 以 ak_ 开头，与令牌管理中的普通 sk- Token 相互独立。Webhook 使用配置页单独生成的 wk- Key。',
        )}
      </Text>

      <div className='border-y border-semi-color-border py-5'>
        {loading ? (
          <div className='flex min-h-36 items-center justify-center'>
            <Spin />
          </div>
        ) : !resourceKey ? (
          <div className='flex min-h-36 flex-col items-center justify-center gap-3 text-center'>
            <KeyRound size={26} className='text-semi-color-tertiary' />
            <Text type='tertiary'>{t('尚未生成资源 API Key')}</Text>
            <Button
              type='primary'
              icon={<KeyRound size={16} />}
              loading={creating}
              onClick={generateKey}
            >
              {t('生成资源 API Key')}
            </Button>
          </div>
        ) : (
          <div className='flex flex-col gap-4'>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <div className='flex flex-wrap items-center gap-2'>
                <Text strong>{t('资源 API Key')}</Text>
                <Tag color={enabled ? 'green' : 'grey'}>
                  {enabled ? t('启用') : t('停用')}
                </Tag>
              </div>
              <Text type='tertiary' size='small'>
                {resourceKey.last_used_at > 0
                  ? `${t('最近使用')} ${timestamp2string(resourceKey.last_used_at)}`
                  : `${t('创建于')} ${timestamp2string(resourceKey.created_at)}`}
              </Text>
            </div>

            <div className='flex min-w-0 items-start gap-2'>
              <Text
                code
                className='min-w-0 flex-1 break-all rounded-sm bg-semi-color-fill-0 px-3 py-2'
              >
                {visible ? resourceKey.key : maskResourceKey(resourceKey.key)}
              </Text>
              <Tooltip
                content={
                  visible ? t('隐藏资源 API Key') : t('显示资源 API Key')
                }
              >
                <Button
                  icon={visible ? <EyeOff size={17} /> : <Eye size={17} />}
                  aria-label={
                    visible ? t('隐藏资源 API Key') : t('显示资源 API Key')
                  }
                  onClick={() => setVisible((current) => !current)}
                />
              </Tooltip>
              <Tooltip content={t('复制资源 API Key')}>
                <Button
                  icon={<Copy size={17} />}
                  aria-label={t('复制资源 API Key')}
                  onClick={copyKey}
                />
              </Tooltip>
            </div>

            <Space spacing={8} wrap>
              <Tooltip content={enabled ? t('停用') : t('启用')}>
                <Button
                  icon={enabled ? <PowerOff size={16} /> : <Power size={16} />}
                  loading={actionKeyID === resourceKey.id}
                  onClick={changeStatus}
                >
                  {enabled ? t('停用') : t('启用')}
                </Button>
              </Tooltip>
              <Popconfirm
                title={t('确定重新生成资源 API Key？')}
                content={t('重新生成后，旧 Key 会立即失效。')}
                onConfirm={generateKey}
              >
                <Button icon={<RefreshCcw size={16} />} loading={creating}>
                  {t('重新生成')}
                </Button>
              </Popconfirm>
              <Popconfirm
                title={t('确定删除资源 API Key？')}
                content={t(
                  '删除后使用该 Key 的 API 调用和 Webhook 验证会立即失效。',
                )}
                onConfirm={deleteKey}
              >
                <Button
                  type='danger'
                  icon={<Trash2 size={16} />}
                  loading={actionKeyID === resourceKey.id}
                >
                  {t('删除')}
                </Button>
              </Popconfirm>
            </Space>
          </div>
        )}
      </div>
    </section>
  );
}
