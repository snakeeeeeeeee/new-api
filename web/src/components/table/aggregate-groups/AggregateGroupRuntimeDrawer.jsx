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
  API,
  showError,
  timestamp2string,
} from '../../../helpers';
import {
  Banner,
  Button,
  Card,
  Descriptions,
  Empty,
  Select,
  SideSheet,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconClose, IconRefresh } from '@douyinfe/semi-icons';
import { useIsMobile } from '../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const formatTime = (timestamp) => {
  if (!timestamp) {
    return '-';
  }
  return timestamp2string(timestamp);
};

const getTriggerReasonLabel = (reason, t) => {
  switch (reason) {
    case 'consecutive_failures':
      return t('连续失败触发');
    case 'consecutive_slows':
      return t('连续慢请求触发');
    default:
      return '-';
  }
};

const renderStatusTag = (active, degraded, t) => {
  if (active) {
    return <Tag color='green'>{t('当前活跃')}</Tag>;
  }
  if (degraded) {
    return <Tag color='red'>{t('已降级')}</Tag>;
  }
  return <Tag color='blue'>{t('正常')}</Tag>;
};

const renderSwitchTag = (enabled, t) => {
  return enabled ? (
    <Tag color='green'>{t('开启')}</Tag>
  ) : (
    <Tag color='grey'>{t('关闭')}</Tag>
  );
};

const AggregateGroupRuntimeDrawer = ({
  visible,
  aggregateGroup,
  onClose,
  t,
}) => {
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [runtimeData, setRuntimeData] = useState(null);
  const [selectedModel, setSelectedModel] = useState('');

  const fetchRuntime = async (modelName = '') => {
    if (!aggregateGroup?.id) {
      return;
    }
    setLoading(true);
    try {
      const params = modelName ? { model: modelName } : undefined;
      const res = await API.get(`/api/aggregate_group/${aggregateGroup.id}/runtime`, {
        params,
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(t(message || '获取聚合分组运行态失败'));
        return;
      }
      setRuntimeData(data);
      setSelectedModel(data?.selected_model || '');
    } catch (error) {
      showError(error?.message || t('获取聚合分组运行态失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (visible && aggregateGroup?.id) {
      fetchRuntime();
      return;
    }
    setRuntimeData(null);
    setSelectedModel('');
  }, [visible, aggregateGroup?.id]);

  const handleRefresh = () => {
    fetchRuntime(selectedModel);
  };

  const handleModelChange = (value) => {
    fetchRuntime(value);
  };

  const runtimeGroup = runtimeData?.aggregate_group || aggregateGroup;
  const smartStrategy = runtimeData?.smart_strategy;
  const runtime = runtimeData?.runtime;
  const activeRoute = runtime?.active_route;
  const modelOptions = useMemo(
    () =>
      (runtimeData?.models || []).map((modelName) => ({
        label: modelName,
        value: modelName,
      })),
    [runtimeData?.models],
  );

  return (
    <SideSheet
      placement='right'
      visible={visible}
      onCancel={onClose}
      width={isMobile ? '100%' : 760}
      closeIcon={
        <Button
          className='semi-button-tertiary semi-button-size-small semi-button-borderless'
          type='button'
          icon={<IconClose />}
          onClick={onClose}
        />
      }
      title={
        <Space>
          <Tag color='blue' shape='circle'>
            {t('运行态')}
          </Tag>
          <Title heading={4} className='m-0'>
            {runtimeGroup?.display_name || runtimeGroup?.name || t('聚合分组')}
          </Title>
        </Space>
      }
      bodyStyle={{
        padding: '0',
        display: 'flex',
        flexDirection: 'column',
        borderBottom: '1px solid var(--semi-color-border)',
      }}
    >
      <div className='p-2'>
        {loading && !runtimeData ? (
          <div className='flex items-center justify-center py-12'>
            <Spin size='large' tip={t('加载中...')} />
          </div>
        ) : (
          <div className='space-y-3'>
            {!smartStrategy?.effective_enabled ? (
              <Banner
                type='warning'
                bordered
                title={t('智能策略当前未生效')}
                description={t('下方状态仅作为保留运行态展示，不参与当前路由跳过决策')}
              />
            ) : null}

            <Card className='border-0 shadow-sm'>
              <div className='flex items-center justify-between gap-2 mb-3'>
                <div>
                  <Text strong>{t('运行摘要')}</Text>
                  <div className='text-xs text-gray-600 mt-1'>
                    {t('查看当前模型下的活跃真实分组与智能策略状态')}
                  </div>
                </div>
                <Button
                  icon={<IconRefresh />}
                  theme='outline'
                  onClick={handleRefresh}
                  loading={loading}
                >
                  {t('刷新')}
                </Button>
              </div>
              <Descriptions
                data={[
                  {
                    key: t('聚合分组'),
                    value: (
                      <Space>
                        <Text strong>{runtimeGroup?.name || '-'}</Text>
                        {runtimeGroup?.display_name ? (
                          <Tag color='blue'>{runtimeGroup.display_name}</Tag>
                        ) : null}
                      </Space>
                    ),
                  },
                  {
                    key: t('全局智能策略'),
                    value: renderSwitchTag(smartStrategy?.global_enabled, t),
                  },
                  {
                    key: t('当前分组智能策略'),
                    value: renderSwitchTag(smartStrategy?.group_enabled, t),
                  },
                  {
                    key: t('是否生效'),
                    value: renderSwitchTag(smartStrategy?.effective_enabled, t),
                  },
                  {
                    key: t('当前活跃真实分组'),
                    value: activeRoute?.active_group ? (
                      <Tag color='green'>{activeRoute.active_group}</Tag>
                    ) : (
                      <Text type='secondary'>{t('暂无运行态')}</Text>
                    ),
                  },
                  {
                    key: t('懒恢复策略'),
                    value:
                      runtimeGroup?.recovery_enabled
                        ? t('{{seconds}} 秒后懒恢复', {
                            seconds: runtimeGroup?.recovery_interval_seconds || 0,
                          })
                        : t('关闭'),
                  },
                ]}
              />
            </Card>

            <Card className='border-0 shadow-sm'>
              <div className='flex items-center justify-between gap-2 mb-3'>
                <div>
                  <Text strong>{t('模型选择')}</Text>
                  <div className='text-xs text-gray-600 mt-1'>
                    {t('切换模型查看当前聚合分组下的运行态')}
                  </div>
                </div>
              </div>
              <Select
                placeholder={t('请选择模型')}
                value={selectedModel || undefined}
                optionList={modelOptions}
                onChange={handleModelChange}
                loading={loading}
                disabled={!modelOptions.length}
                filter
              />
            </Card>

            {!runtimeData?.models?.length ? (
              <Card className='border-0 shadow-sm'>
                <Empty description={t('当前聚合分组下暂无模型')} />
              </Card>
            ) : !runtime ? (
              <Card className='border-0 shadow-sm'>
                <Empty description={t('当前模型暂无运行态')} />
              </Card>
            ) : (
              <>
                <Card className='border-0 shadow-sm'>
                  <div className='flex items-center justify-between gap-2 mb-3'>
                    <div>
                      <Text strong>{t('活跃链路')}</Text>
                      <div className='text-xs text-gray-600 mt-1'>
                        {selectedModel || t('未选择模型')}
                      </div>
                    </div>
                    {activeRoute?.active_group ? (
                      <Tag color='green'>{activeRoute.active_group}</Tag>
                    ) : null}
                  </div>
                  <Descriptions
                    data={[
                      {
                        key: t('当前索引'),
                        value:
                          activeRoute && activeRoute.active_index >= 0
                            ? activeRoute.active_index + 1
                            : '-',
                      },
                      {
                        key: t('最近切换时间'),
                        value: formatTime(activeRoute?.last_switch_at),
                      },
                      {
                        key: t('最近成功时间'),
                        value: formatTime(activeRoute?.last_success_at),
                      },
                      {
                        key: t('最近失败时间'),
                        value: formatTime(activeRoute?.last_fail_at),
                      },
                    ]}
                  />
                </Card>

                <div className='space-y-3'>
                  {runtime.routes?.map((route) => (
                    <Card
                      key={`${route.route_group}-${route.route_index}`}
                      className='border-0 shadow-sm'
                      title={
                        <div className='flex items-center justify-between gap-2'>
                          <Space>
                            <Text strong>
                              {route.route_index + 1}. {route.route_group}
                            </Text>
                            {renderStatusTag(
                              route.is_active,
                              route.is_degraded,
                              t,
                            )}
                          </Space>
                          {route.is_degraded ? (
                            <Tag color='red'>{t('临时降级中')}</Tag>
                          ) : null}
                        </div>
                      }
                    >
                      <Descriptions
                        data={[
                          {
                            key: t('降级到期时间'),
                            value: formatTime(route.degraded_until),
                          },
                          {
                            key: t('连续失败数'),
                            value: route.consecutive_failures ?? 0,
                          },
                          {
                            key: t('连续慢请求数'),
                            value: route.consecutive_slows ?? 0,
                          },
                          {
                            key: t('最近成功时间'),
                            value: formatTime(route.last_success_at),
                          },
                          {
                            key: t('最近失败时间'),
                            value: formatTime(route.last_failure_at),
                          },
                          {
                            key: t('最近慢请求时间'),
                            value: formatTime(route.last_slow_at),
                          },
                          {
                            key: t('当前/最近触发原因'),
                            value: getTriggerReasonLabel(
                              route.last_trigger_reason,
                              t,
                            ),
                          },
                          {
                            key: t('最近触发时间'),
                            value: formatTime(route.last_trigger_at),
                          },
                        ]}
                      />
                    </Card>
                  ))}
                </div>
              </>
            )}
          </div>
        )}
      </div>
    </SideSheet>
  );
};

export default AggregateGroupRuntimeDrawer;
