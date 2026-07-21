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
import { Button, Select, Tabs, Typography } from '@douyinfe/semi-ui';
import { IconRefresh } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../../helpers';
import OverviewTab from './OverviewTab';
import AsyncTasksTab from './AsyncTasksTab';
import WebhookDeliveriesTab from './WebhookDeliveriesTab';
import SettingsTab from './SettingsTab';
import { normalizeStats } from './utils';

const { Text, Title } = Typography;

const REFRESH_OPTIONS = [
  { value: 0, label: '关闭' },
  { value: 5, label: '5 秒' },
  { value: 15, label: '15 秒' },
  { value: 30, label: '30 秒' },
];

const AsyncTask = () => {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState('overview');
  const [refreshInterval, setRefreshInterval] = useState(5);
  const [refreshToken, setRefreshToken] = useState(0);
  const [stats, setStats] = useState(normalizeStats());
  const [statsLoading, setStatsLoading] = useState(false);

  useEffect(() => {
    if (activeTab !== 'overview') return;
    let cancelled = false;
    const loadStats = async () => {
      setStatsLoading(true);
      try {
        const response = await API.get('/api/task/async/stats');
        if (!cancelled && response.data.success) {
          setStats(normalizeStats(response.data.data));
        } else if (!cancelled) {
          showError(response.data.message || t('加载失败'));
        }
      } catch {
        if (!cancelled) showError(t('加载失败'));
      } finally {
        if (!cancelled) setStatsLoading(false);
      }
    };
    loadStats();
    return () => {
      cancelled = true;
    };
  }, [activeTab, refreshToken, t]);

  useEffect(() => {
    if (!refreshInterval || activeTab === 'settings') return undefined;
    const timer = window.setInterval(() => {
      if (document.visibilityState === 'visible') {
        setRefreshToken((value) => value + 1);
      }
    }, refreshInterval * 1000);
    return () => window.clearInterval(timer);
  }, [activeTab, refreshInterval]);

  return (
    <div className='mt-[60px] px-2 pb-6'>
      <div className='flex flex-col gap-3'>
        <div className='flex flex-wrap items-center justify-between gap-3'>
          <div>
            <Title heading={4} style={{ margin: 0 }}>
              {t('异步任务管理')}
            </Title>
            <Text type='tertiary'>{t('队列与投递运行状态')}</Text>
          </div>
          <div className='flex items-center gap-2'>
            <Select
              prefix={t('自动刷新')}
              value={refreshInterval}
              style={{ width: 150 }}
              optionList={REFRESH_OPTIONS.map((item) => ({
                ...item,
                label: t(item.label),
              }))}
              onChange={setRefreshInterval}
              disabled={activeTab === 'settings'}
            />
            <Button
              icon={<IconRefresh />}
              aria-label={t('刷新')}
              onClick={() => setRefreshToken((value) => value + 1)}
            >
              {t('刷新')}
            </Button>
          </div>
        </div>

        <Tabs
          type='line'
          activeKey={activeTab}
          onChange={setActiveTab}
          tabList={[
            { tab: t('概览'), itemKey: 'overview' },
            { tab: t('异步任务'), itemKey: 'tasks' },
            { tab: t('Webhook 投递'), itemKey: 'webhooks' },
            { tab: t('设置'), itemKey: 'settings' },
          ]}
        />

        {activeTab === 'overview' ? (
          <OverviewTab stats={stats} loading={statsLoading} />
        ) : null}
        {activeTab === 'tasks' ? (
          <AsyncTasksTab refreshToken={refreshToken} />
        ) : null}
        {activeTab === 'webhooks' ? (
          <WebhookDeliveriesTab refreshToken={refreshToken} />
        ) : null}
        {activeTab === 'settings' ? <SettingsTab /> : null}
      </div>
    </div>
  );
};

export default AsyncTask;
