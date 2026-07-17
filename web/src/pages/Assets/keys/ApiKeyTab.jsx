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
  Pagination,
  Space,
  Spin,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Copy,
  ExternalLink,
  Eye,
  EyeOff,
  KeyRound,
  Plus,
  RefreshCcw,
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
const PAGE_SIZE = 10;

function fullAPIKey(value) {
  if (!value) return '';
  return value.startsWith('sk-') ? value : `sk-${value}`;
}

function tokenStatus(status, t) {
  switch (status) {
    case 1:
      return <Tag color='green'>{t('启用')}</Tag>;
    case 2:
      return <Tag color='grey'>{t('禁用')}</Tag>;
    case 3:
      return <Tag color='orange'>{t('已过期')}</Tag>;
    case 4:
      return <Tag color='red'>{t('额度已用尽')}</Tag>;
    default:
      return <Tag>{t('未知')}</Tag>;
  }
}

function defaultTokenName() {
  const now = new Date();
  const date = [
    now.getFullYear(),
    String(now.getMonth() + 1).padStart(2, '0'),
    String(now.getDate()).padStart(2, '0'),
  ].join('');
  const time = [
    String(now.getHours()).padStart(2, '0'),
    String(now.getMinutes()).padStart(2, '0'),
    String(now.getSeconds()).padStart(2, '0'),
  ].join('');
  return `resource-center-${date}-${time}`;
}

export default function ApiKeyTab() {
  const { t } = useTranslation();
  const [tokens, setTokens] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [name, setName] = useState('');
  const [loading, setLoading] = useState(false);
  const [creating, setCreating] = useState(false);
  const [resolvedKeys, setResolvedKeys] = useState({});
  const [visibleKeys, setVisibleKeys] = useState({});
  const [keyLoading, setKeyLoading] = useState({});

  const loadTokens = async (targetPage = page) => {
    setLoading(true);
    try {
      const response = await API.get(
        `/api/token/?p=${targetPage}&size=${PAGE_SIZE}`,
      );
      const { success, data, message } = response.data || {};
      if (!success) {
        showError(message || t('加载 API Key 失败'));
        return;
      }
      setTokens(data?.items || []);
      setTotal(data?.total || 0);
      setPage(data?.page || targetPage);
    } catch (error) {
      showError(error?.message || t('加载 API Key 失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadTokens(1);
  }, []);

  const createAPIKey = async () => {
    setCreating(true);
    try {
      const response = await API.post('/api/token/', {
        name: name.trim() || defaultTokenName(),
        expired_time: -1,
        remain_quota: 0,
        unlimited_quota: true,
        model_limits_enabled: false,
        model_limits: '',
        allow_ips: '',
        group: '',
        cross_group_retry: false,
      });
      const { success, data, message } = response.data || {};
      if (!success) {
        showError(message || t('生成 API Key 失败'));
        return;
      }
      await loadTokens(1);
      if (data?.id && data?.key) {
        setResolvedKeys((current) => ({
          ...current,
          [data.id]: fullAPIKey(data.key),
        }));
        setVisibleKeys((current) => ({ ...current, [data.id]: true }));
      }
      setName('');
      showSuccess(t('API Key 已生成'));
    } catch (error) {
      showError(error?.message || t('生成 API Key 失败'));
    } finally {
      setCreating(false);
    }
  };

  const resolveKey = async (token) => {
    if (resolvedKeys[token.id]) return resolvedKeys[token.id];
    setKeyLoading((current) => ({ ...current, [token.id]: true }));
    try {
      const response = await API.post(`/api/token/${token.id}/key`);
      const { success, data, message } = response.data || {};
      if (!success || !data?.key) {
        throw new Error(message || t('获取 API Key 失败'));
      }
      const key = fullAPIKey(data.key);
      setResolvedKeys((current) => ({ ...current, [token.id]: key }));
      return key;
    } catch (error) {
      showError(error?.message || t('获取 API Key 失败'));
      return '';
    } finally {
      setKeyLoading((current) => ({ ...current, [token.id]: false }));
    }
  };

  const toggleKey = async (token) => {
    if (visibleKeys[token.id]) {
      setVisibleKeys((current) => ({ ...current, [token.id]: false }));
      return;
    }
    const key = await resolveKey(token);
    if (key) {
      setVisibleKeys((current) => ({ ...current, [token.id]: true }));
    }
  };

  const copyKey = async (token) => {
    const key = await resolveKey(token);
    if (key && (await copy(key))) showSuccess(t('API Key 已复制'));
  };

  return (
    <section className='flex max-w-4xl flex-col gap-5'>
      <div className='flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between'>
        <div className='min-w-0'>
          <Title heading={5} className='!mb-1'>
            {t('API Key')}
          </Title>
          <Text type='tertiary'>
            {t('用于创建和查询异步任务、预上传文件以及访问生成资源')}
          </Text>
        </div>
        <Button
          icon={<ExternalLink size={16} />}
          onClick={() => window.open('/console/token', '_self')}
        >
          {t('管理全部 API Key')}
        </Button>
      </div>

      <div className='flex flex-col gap-2 sm:flex-row sm:items-end'>
        <label className='flex min-w-0 flex-1 flex-col gap-2'>
          <Text strong>{t('名称（可选）')}</Text>
          <Input
            value={name}
            onChange={setName}
            prefix={<KeyRound size={16} />}
            placeholder={t('留空将自动生成名称')}
            maxLength={50}
            showClear
          />
        </label>
        <Button
          type='primary'
          icon={<Plus size={16} />}
          loading={creating}
          onClick={createAPIKey}
        >
          {t('生成 API Key')}
        </Button>
      </div>

      <Text type='tertiary' size='small'>
        {t(
          '生成后可在此随时查看和复制；权限、额度和有效期可在令牌管理中修改。',
        )}
      </Text>

      <div className='border-y border-semi-color-border'>
        {loading ? (
          <div className='flex min-h-32 items-center justify-center'>
            <Spin />
          </div>
        ) : tokens.length === 0 ? (
          <div className='flex min-h-32 items-center justify-center'>
            <Text type='tertiary'>{t('暂无 API Key')}</Text>
          </div>
        ) : (
          <div className='divide-y divide-semi-color-border'>
            {tokens.map((token) => {
              const visible = Boolean(visibleKeys[token.id]);
              const displayKey = visible
                ? resolvedKeys[token.id]
                : fullAPIKey(token.key);
              return (
                <div
                  key={token.id}
                  className='grid grid-cols-1 gap-3 py-4 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)_auto] sm:items-center sm:gap-4'
                >
                  <div className='min-w-0'>
                    <div className='flex flex-wrap items-center gap-2'>
                      <Text strong ellipsis={{ showTooltip: true }}>
                        {token.name}
                      </Text>
                      {tokenStatus(token.status, t)}
                    </div>
                    <Text type='tertiary' size='small'>
                      {timestamp2string(token.created_time)}
                    </Text>
                  </div>
                  <Text code className='min-w-0 break-all'>
                    {displayKey}
                  </Text>
                  <Space spacing={4}>
                    <Tooltip
                      content={visible ? t('隐藏 API Key') : t('显示 API Key')}
                    >
                      <Button
                        theme='borderless'
                        type='tertiary'
                        icon={
                          visible ? <EyeOff size={17} /> : <Eye size={17} />
                        }
                        loading={Boolean(keyLoading[token.id])}
                        aria-label={
                          visible ? t('隐藏 API Key') : t('显示 API Key')
                        }
                        onClick={() => toggleKey(token)}
                      />
                    </Tooltip>
                    <Tooltip content={t('复制 API Key')}>
                      <Button
                        theme='borderless'
                        type='tertiary'
                        icon={<Copy size={17} />}
                        loading={Boolean(keyLoading[token.id])}
                        aria-label={t('复制 API Key')}
                        onClick={() => copyKey(token)}
                      />
                    </Tooltip>
                  </Space>
                </div>
              );
            })}
          </div>
        )}
      </div>

      <div className='flex flex-wrap items-center justify-between gap-2'>
        <Button
          icon={<RefreshCcw size={16} />}
          loading={loading}
          onClick={() => loadTokens(page)}
        >
          {t('刷新')}
        </Button>
        {total > PAGE_SIZE && (
          <Pagination
            currentPage={page}
            pageSize={PAGE_SIZE}
            total={total}
            onPageChange={loadTokens}
          />
        )}
      </div>
    </section>
  );
}
