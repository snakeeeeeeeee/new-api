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
  Avatar,
  Button,
  Card,
  Col,
  Collapse,
  Input,
  InputNumber,
  Modal,
  Row,
  Select,
  SideSheet,
  Space,
  Switch,
  Tag,
  Tabs,
  TabPane,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconArrowDown,
  IconArrowUp,
  IconClose,
  IconDelete,
  IconPlus,
  IconSave,
  IconServer,
} from '@douyinfe/semi-icons';
import { API, selectFilter, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';
import { useIsMobile } from '../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const routingModeLabels = {
  failover: 'Failover 故障转移',
  cluster: 'Cluster 集群分发',
};

const routeAffinityStrategyOptions = [
  {
    label: '平台账号（兼容旧逻辑）',
    value: 'platform_user',
  },
  {
    label: '智能请求标识粘性（多人共用推荐）',
    value: 'request_first',
  },
  {
    label: '仅请求标识粘性（无标识则按权重）',
    value: 'request_only',
  },
  {
    label: '关闭亲和',
    value: 'off',
  },
];

const routeAffinitySourceTypes = [
  { label: 'Header', value: 'header' },
  { label: 'Query', value: 'query' },
  { label: 'JSON Body', value: 'gjson' },
  { label: 'Context Int', value: 'context_int' },
  { label: 'Context String', value: 'context_string' },
];

const defaultRouteAffinityKeySources = () => [
  { type: 'header', key: 'X-Aggregate-Affinity-Key' },
  { type: 'query', key: 'aggregate_route_affinity_key' },
  { type: 'gjson', path: 'metadata.aggregate_route_affinity_key' },
  { type: 'gjson', path: 'metadata.user_id' },
  { type: 'gjson', path: 'prompt_cache_key' },
  { type: 'gjson', path: 'user' },
  { type: 'gjson', path: 'cachedContent' },
];

const normalizeRouteAffinityKeySources = (value) => {
  const sources = Array.isArray(value) && value.length > 0
    ? value
    : defaultRouteAffinityKeySources();
  return sources
    .map((source) => ({
      type: source?.type || 'gjson',
      key: source?.key || '',
      path: source?.path || '',
    }))
    .filter((source) => source.type || source.key || source.path);
};

const defaultClientRoutePools = () => ({
  enabled: false,
  claude_code_cli: {
    enabled: false,
    fallback_to_default: true,
    targets: [],
  },
});

const normalizeClientRoutePools = (value) => {
  const defaults = defaultClientRoutePools();
  const claudeCodeCLI = value?.claude_code_cli || {};
  return {
    enabled: !!value?.enabled,
    claude_code_cli: {
      enabled: !!claudeCodeCLI.enabled,
      fallback_to_default:
        claudeCodeCLI.fallback_to_default === undefined ||
        claudeCodeCLI.fallback_to_default === null
          ? true
          : !!claudeCodeCLI.fallback_to_default,
      targets: (claudeCodeCLI.targets || []).map((target) => ({
        real_group: target.real_group,
        weight:
          target.weight === undefined || target.weight === null
            ? 100
            : target.weight,
      })),
    },
  };
};

const defaultInputs = {
  id: undefined,
  name: '',
  display_name: '',
  description: '',
  status: 1,
  group_ratio: 1,
  routing_mode: 'failover',
  smart_routing_enabled: false,
  recovery_enabled: true,
  recovery_interval_seconds: 300,
  cluster_affinity_ttl_seconds: 300,
  route_affinity_strategy: 'platform_user',
  route_affinity_key_sources: defaultRouteAffinityKeySources(),
  retry_status_codes: '',
  visible_user_groups: [],
  targets: [],
  client_route_pools: defaultClientRoutePools(),
};

const EditAggregateGroupModal = ({
  visible,
  editingGroup,
  onClose,
  onSuccess,
  realGroupOptions,
  userGroupOptions,
}) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState(defaultInputs);
  const [affinityAdvancedActiveKey, setAffinityAdvancedActiveKey] = useState([]);

  const isEdit = editingGroup?.id !== undefined;
  const isClusterMode = inputs.routing_mode === 'cluster';

  useEffect(() => {
    if (!visible) {
      setInputs(defaultInputs);
      return;
    }
    if (!isEdit) {
      setInputs(defaultInputs);
      return;
    }

    const loadDetail = async () => {
      setLoading(true);
      try {
        const res = await API.get(`/api/aggregate_group/${editingGroup.id}`);
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(t(message || '获取聚合分组详情失败'));
          return;
        }
        setInputs({
          id: data.id,
          name: data.name || '',
          display_name: data.display_name || '',
          description: data.description || '',
          status: data.status || 1,
          group_ratio: data.group_ratio === undefined ? 1 : data.group_ratio,
          routing_mode: data.routing_mode || 'failover',
          smart_routing_enabled:
            data.smart_routing_enabled === undefined
              ? false
              : data.smart_routing_enabled,
          recovery_enabled:
            data.recovery_enabled === undefined ? true : data.recovery_enabled,
          recovery_interval_seconds:
            data.recovery_interval_seconds === undefined
              ? 300
              : data.recovery_interval_seconds,
          cluster_affinity_ttl_seconds:
            data.cluster_affinity_ttl_seconds === undefined
              ? 300
              : data.cluster_affinity_ttl_seconds,
          route_affinity_strategy:
            data.route_affinity_strategy || 'platform_user',
          route_affinity_key_sources: normalizeRouteAffinityKeySources(
            data.route_affinity_key_sources,
          ),
          retry_status_codes: data.retry_status_codes || '',
          visible_user_groups: data.visible_user_groups || [],
          targets: (data.targets || []).map((item) => ({
            real_group: item.real_group,
            weight:
              item.weight === undefined || item.weight === null
                ? 100
                : item.weight,
          })),
          client_route_pools: normalizeClientRoutePools(
            data.client_route_pools,
          ),
        });
      } catch (error) {
        showError(error?.message || t('获取聚合分组详情失败'));
      } finally {
        setLoading(false);
      }
    };

    loadDetail();
  }, [visible, isEdit, editingGroup?.id, t]);

  const selectedTargetValues = useMemo(
    () => inputs.targets.map((target) => target.real_group),
    [inputs.targets],
  );

  const clientRoutePools = useMemo(
    () => normalizeClientRoutePools(inputs.client_route_pools),
    [inputs.client_route_pools],
  );

  const claudeCliPool = clientRoutePools.claude_code_cli;

  const selectedClaudeCliTargetValues = useMemo(
    () => claudeCliPool.targets.map((target) => target.real_group),
    [claudeCliPool.targets],
  );

  const availableTargetOptions = useMemo(() => {
    const selected = new Set(selectedTargetValues);
    return (realGroupOptions || []).map((option) => ({
      ...option,
      disabled:
        selected.has(option.value) &&
        !selectedTargetValues.includes(option.value),
    }));
  }, [realGroupOptions, selectedTargetValues]);

  const updateField = (field, value) => {
    setInputs((prev) => ({
      ...prev,
      [field]: value,
    }));
  };

  const updateRouteAffinitySource = (index, patch) => {
    setInputs((prev) => ({
      ...prev,
      route_affinity_key_sources: prev.route_affinity_key_sources.map(
        (source, currentIndex) => {
          if (currentIndex !== index) {
            return source;
          }
          const next = {
            ...source,
            ...patch,
          };
          if (patch.type && patch.type !== source.type) {
            if (patch.type === 'gjson') {
              next.path = source.path || source.key || '';
              next.key = '';
            } else {
              next.key = source.key || source.path || '';
              next.path = '';
            }
          }
          return next;
        },
      ),
    }));
  };

  const addRouteAffinitySource = () => {
    setInputs((prev) => ({
      ...prev,
      route_affinity_key_sources: [
        ...prev.route_affinity_key_sources,
        { type: 'gjson', path: '' },
      ],
    }));
  };

  const removeRouteAffinitySource = (index) => {
    setInputs((prev) => ({
      ...prev,
      route_affinity_key_sources: prev.route_affinity_key_sources.filter(
        (_, currentIndex) => currentIndex !== index,
      ),
    }));
  };

  const handleRoutingModeChange = (key) => {
    const nextMode = key || 'failover';
    const currentMode = inputs.routing_mode || 'failover';
    if (nextMode === currentMode) {
      return;
    }
    Modal.confirm({
      title: t('确认切换路由模式？'),
      content: t(
        '确认后表单将切换为 {{mode}} 配置；提交保存后，聚合分组将以 {{mode}} 模式运行。',
        {
          mode: t(routingModeLabels[nextMode] || nextMode),
        },
      ),
      okText: t('确认切换'),
      cancelText: t('取消'),
      onOk: () => updateField('routing_mode', nextMode),
    });
  };

  const moveTarget = (index, direction) => {
    setInputs((prev) => {
      const nextTargets = [...prev.targets];
      const targetIndex = index + direction;
      if (targetIndex < 0 || targetIndex >= nextTargets.length) {
        return prev;
      }
      const temp = nextTargets[index];
      nextTargets[index] = nextTargets[targetIndex];
      nextTargets[targetIndex] = temp;
      return {
        ...prev,
        targets: nextTargets,
      };
    });
  };

  const removeTarget = (index) => {
    setInputs((prev) => ({
      ...prev,
      targets: prev.targets.filter((_, currentIndex) => currentIndex !== index),
    }));
  };

  const updateTargetSelection = (values) => {
    setInputs((prev) => {
      const targetMap = new Map(
        prev.targets.map((target) => [target.real_group, target]),
      );
      return {
        ...prev,
        targets: (values || []).map((realGroup) => {
          const previous = targetMap.get(realGroup);
          if (previous) {
            return previous;
          }
          return {
            real_group: realGroup,
            weight: 100,
          };
        }),
      };
    });
  };

  const updateTargetWeight = (index, value) => {
    setInputs((prev) => {
      const nextTargets = prev.targets.map((target, currentIndex) => {
        if (currentIndex !== index) {
          return target;
        }
        return {
          ...target,
          weight: Math.max(0, Number(value || 0)),
        };
      });
      return {
        ...prev,
        targets: nextTargets,
      };
    });
  };

  const updateClientRoutePools = (updater) => {
    setInputs((prev) => {
      const current = normalizeClientRoutePools(prev.client_route_pools);
      const next =
        typeof updater === 'function' ? updater(current) : updater || current;
      return {
        ...prev,
        client_route_pools: normalizeClientRoutePools(next),
      };
    });
  };

  const updateClaudeCliPoolSelection = (values) => {
    updateClientRoutePools((current) => {
      const targetMap = new Map(
        current.claude_code_cli.targets.map((target) => [
          target.real_group,
          target,
        ]),
      );
      return {
        ...current,
        claude_code_cli: {
          ...current.claude_code_cli,
          targets: (values || []).map((realGroup) => {
            const previous = targetMap.get(realGroup);
            if (previous) {
              return previous;
            }
            return {
              real_group: realGroup,
              weight: 100,
            };
          }),
        },
      };
    });
  };

  const updateClaudeCliPoolWeight = (index, value) => {
    updateClientRoutePools((current) => ({
      ...current,
      claude_code_cli: {
        ...current.claude_code_cli,
        targets: current.claude_code_cli.targets.map((target, targetIndex) => {
          if (targetIndex !== index) {
            return target;
          }
          return {
            ...target,
            weight: Math.max(0, Number(value || 0)),
          };
        }),
      },
    }));
  };

  const moveClaudeCliPoolTarget = (index, direction) => {
    updateClientRoutePools((current) => {
      const targets = [...current.claude_code_cli.targets];
      const nextIndex = index + direction;
      if (nextIndex < 0 || nextIndex >= targets.length) {
        return current;
      }
      const [item] = targets.splice(index, 1);
      targets.splice(nextIndex, 0, item);
      return {
        ...current,
        claude_code_cli: {
          ...current.claude_code_cli,
          targets,
        },
      };
    });
  };

  const renderRetryStatusCodes = () => (
    <Col xs={24} sm={12}>
      <div className='mb-2'>
        <Text strong>{t('聚合重试状态码')}</Text>
      </div>
      <Input
        value={inputs.retry_status_codes}
        onChange={(value) => updateField('retry_status_codes', value)}
        placeholder={t('留空沿用系统规则，例如：401,403,429,500-599')}
      />
      <div className='mt-1 text-xs text-gray-500'>
        {t('仅对当前聚合分组生效；填写后覆盖系统默认重试状态码规则。')}
      </div>
    </Col>
  );

  const renderRouteAffinityConfig = () => {
    const strategy = inputs.route_affinity_strategy || 'platform_user';
    const requestMode =
      strategy === 'request_first' || strategy === 'request_only';
    return (
      <div className='mt-4 rounded-lg border border-gray-100 bg-white px-3 py-3'>
        <div className='flex items-start justify-between gap-3 flex-wrap'>
          <div>
            <Text strong>{t('子分组亲和策略')}</Text>
            <div className='mt-1 text-xs text-gray-500'>
              {t(
                '用于决定同一用户或同一请求标识在亲和保持时间内尽量回到同一子分组。',
              )}
            </div>
          </div>
          <Select
            value={strategy}
            optionList={routeAffinityStrategyOptions.map((option) => ({
              ...option,
              label: t(option.label),
            }))}
            onChange={(value) =>
              updateField('route_affinity_strategy', value || 'platform_user')
            }
            style={{ width: isMobile ? '100%' : 260 }}
          />
        </div>
        <div className='mt-3 rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 text-xs text-gray-600'>
          {strategy === 'platform_user' &&
            t('按平台账号 ID 亲和，完全兼容旧逻辑；一个 token 被多人共用时会被视为同一个账号。')}
          {strategy === 'request_first' &&
            t('自动识别请求里的用户/会话标识，识别不到再按平台账号；适合多人共用同一个 Key 的场景。')}
          {strategy === 'request_only' &&
            t('只使用请求里的用户/会话标识；识别不到时不会强行粘住，继续按权重选择。')}
          {strategy === 'off' &&
            t('关闭子分组亲和，每次都按当前候选和权重选择。')}
        </div>

        {requestMode && (
          <div className='mt-3'>
            <div className='rounded-lg border border-green-100 bg-green-50 px-3 py-2 text-xs text-green-700'>
              {t(
                '系统会自动识别 Claude / OpenAI / Codex / Gemini 常见用户或会话标识；一般无需修改高级来源。',
              )}
            </div>
            <Collapse
              keepDOM
              className='mt-3'
              activeKey={affinityAdvancedActiveKey}
              onChange={(activeKey) => {
                const keys = Array.isArray(activeKey) ? activeKey : [activeKey];
                setAffinityAdvancedActiveKey(keys.filter(Boolean));
              }}
            >
              <Collapse.Panel
                header={t('高级：自定义请求标识来源')}
                itemKey='sources'
              >
                <div className='flex items-center justify-between gap-3 flex-wrap'>
                  <div>
                    <Text strong>{t('请求标识来源')}</Text>
                    <div className='mt-1 text-xs text-gray-500'>
                      {t(
                        '按顺序提取，命中第一个非空值；以后新增平台通常只需要补一个 Header、Query 或 JSON path。',
                      )}
                    </div>
                  </div>
                  <Button
                    icon={<IconPlus />}
                    theme='light'
                    onClick={addRouteAffinitySource}
                  >
                    {t('添加来源')}
                  </Button>
                </div>
                <div className='flex flex-col gap-2 mt-3'>
                  {inputs.route_affinity_key_sources.length === 0 ? (
                    <div className='rounded-lg border border-dashed border-gray-200 bg-gray-50 px-3 py-4 text-center text-sm text-gray-500'>
                      {t('未配置请求标识来源，将使用系统默认来源')}
                    </div>
                  ) : (
                    inputs.route_affinity_key_sources.map((source, index) => {
                      const isGjson = source.type === 'gjson';
                      return (
                        <div
                          key={`${source.type}-${source.key}-${source.path}-${index}`}
                          className='rounded-lg border border-gray-200 bg-gray-50 px-3 py-3'
                        >
                          <div className='flex items-center gap-2 flex-wrap'>
                            <Tag color='green' shape='circle'>
                              {index + 1}
                            </Tag>
                            <Select
                              value={source.type || 'gjson'}
                              optionList={routeAffinitySourceTypes.map((option) => ({
                                ...option,
                                label: t(option.label),
                              }))}
                              onChange={(value) =>
                                updateRouteAffinitySource(index, {
                                  type: value || 'gjson',
                                })
                              }
                              style={{ width: 150 }}
                            />
                            <Input
                              value={isGjson ? source.path : source.key}
                              onChange={(value) =>
                                updateRouteAffinitySource(
                                  index,
                                  isGjson ? { path: value } : { key: value },
                                )
                              }
                              placeholder={
                                isGjson
                                  ? 'metadata.user_id'
                                  : 'X-Aggregate-Affinity-Key'
                              }
                              style={{ flex: 1, minWidth: 220 }}
                            />
                            <Button
                              theme='borderless'
                              type='danger'
                              icon={<IconDelete />}
                              onClick={() => removeRouteAffinitySource(index)}
                              aria-label={t('删除请求标识来源')}
                            />
                          </div>
                        </div>
                      );
                    })
                  )}
                </div>
              </Collapse.Panel>
            </Collapse>
          </div>
        )}
      </div>
    );
  };

  const renderTargetList = () => (
    <div className='mt-4'>
      <div className='mb-2'>
        <Text strong>{t('添加真实分组')}</Text>
      </div>
      <Select
        placeholder={t('选择真实分组')}
        value={selectedTargetValues}
        onChange={updateTargetSelection}
        optionList={availableTargetOptions}
        multiple
        filter={selectFilter}
        searchPosition='dropdown'
        autoClearSearchValue={false}
        style={{ width: '100%' }}
      />

      <div className='mt-3 rounded-lg border border-gray-100 bg-gray-50 px-3 py-2 text-xs text-gray-600'>
        {isClusterMode
          ? t(
              '权重表示相对流量比例，例如 100/200 约等于 1:2；0 表示不参与普通加权随机。顺序会保留，用于切回 Failover 后的链路顺序。',
            )
          : t(
              'Failover 按列表顺序形成 A -> B -> C 链路；权重在该模式不参与路由，因此不会显示权重编辑。',
            )}
      </div>

      <div className='flex flex-col gap-2 mt-3'>
        {inputs.targets.length === 0 ? (
          <div className='rounded-lg border border-dashed border-gray-200 bg-white px-3 py-4 text-center text-sm text-gray-500'>
            {t('请选择至少一个真实分组')}
          </div>
        ) : (
          inputs.targets.map((target, index) => (
            <div
              key={target.real_group}
              className='rounded-lg border border-gray-200 bg-white px-3 py-3'
            >
              <div className='flex items-center justify-between gap-3 flex-wrap'>
                <Space>
                  <Tag color={isClusterMode ? 'green' : 'blue'} shape='circle'>
                    {index + 1}
                  </Tag>
                  <div>
                    <Text strong>{target.real_group}</Text>
                    <div className='text-xs text-gray-500'>
                      {isClusterMode ? t('子分组节点') : t('链路节点')}
                    </div>
                  </div>
                </Space>
                <Space align='center'>
                  {isClusterMode && (
                    <div className='flex items-center gap-2'>
                      <div className='text-right leading-tight'>
                        <Text type='secondary' className='text-xs'>
                          {t('权重')}
                        </Text>
                        <div className='text-[11px] text-gray-500'>
                          {t('越大流量越多')}
                        </div>
                      </div>
                      <InputNumber
                        min={0}
                        value={target.weight}
                        onChange={(value) => updateTargetWeight(index, value)}
                        aria-label={t('Cluster 权重')}
                        style={{ width: 104 }}
                      />
                    </div>
                  )}
                  <Button
                    theme='borderless'
                    icon={<IconArrowUp />}
                    disabled={index === 0}
                    onClick={() => moveTarget(index, -1)}
                    aria-label={t('上移真实分组')}
                  />
                  <Button
                    theme='borderless'
                    icon={<IconArrowDown />}
                    disabled={index === inputs.targets.length - 1}
                    onClick={() => moveTarget(index, 1)}
                    aria-label={t('下移真实分组')}
                  />
                  <Button
                    theme='borderless'
                    type='danger'
                    icon={<IconClose />}
                    onClick={() => removeTarget(index)}
                    aria-label={t('移除真实分组')}
                  />
                </Space>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );

  const renderClientRoutePools = () => (
    <div className='mt-5 border-t border-gray-100 pt-4'>
      <div className='flex items-start justify-between gap-3 flex-wrap'>
        <div>
          <Text strong>{t('客户端专用流量池')}</Text>
          <div className='mt-1 text-xs text-gray-500'>
            {t(
              '识别到指定客户端后优先进入专用池；Failover 下按专用池顺序故障转移，Cluster 下按专用权重分发。',
            )}
          </div>
        </div>
        <Switch
          checked={clientRoutePools.enabled}
          onChange={(checked) =>
            updateClientRoutePools((current) => ({
              ...current,
              enabled: checked,
            }))
          }
        />
      </div>

      <div className='mt-3 rounded-lg border border-gray-200 bg-white px-3 py-3'>
        <div className='flex items-start justify-between gap-3 flex-wrap'>
          <div>
            <Text strong>{t('Claude Code CLI 定向池')}</Text>
            <div className='mt-1 text-xs text-gray-500'>
              {t(
                '命中条件为 /v1/messages、Claude 模型、User-Agent 包含 claude-cli/；当前模式下跟随上方 Failover 或 Cluster 选路。',
              )}
            </div>
          </div>
          <Switch
            checked={claudeCliPool.enabled}
            disabled={!clientRoutePools.enabled}
            onChange={(checked) =>
              updateClientRoutePools((current) => ({
                ...current,
                claude_code_cli: {
                  ...current.claude_code_cli,
                  enabled: checked,
                },
              }))
            }
          />
        </div>

        <Row gutter={12} className='mt-3'>
          <Col xs={24} sm={12}>
            <div className='mb-2'>
              <Text strong>{t('专用池目标子分组')}</Text>
            </div>
            <Select
              placeholder={t('选择 Claude CLI 专用真实分组')}
              value={selectedClaudeCliTargetValues}
              onChange={updateClaudeCliPoolSelection}
              optionList={realGroupOptions || []}
              multiple
              filter={selectFilter}
              searchPosition='dropdown'
              autoClearSearchValue={false}
              disabled={!clientRoutePools.enabled || !claudeCliPool.enabled}
              style={{ width: '100%' }}
            />
            <div className='mt-1 text-xs text-gray-500'>
              {t('专用池与默认流量池互相独立；Failover 使用列表顺序，Cluster 使用专用权重。')}
            </div>
          </Col>
          <Col xs={24} sm={12}>
            <div className='mb-2'>
              <Text strong>{t('专用池不可用时回退默认池')}</Text>
            </div>
            <Switch
              checked={claudeCliPool.fallback_to_default}
              disabled={!clientRoutePools.enabled || !claudeCliPool.enabled}
              onChange={(checked) =>
                updateClientRoutePools((current) => ({
                  ...current,
                  claude_code_cli: {
                    ...current.claude_code_cli,
                    fallback_to_default: checked,
                  },
                }))
              }
            />
            <div className='mt-1 text-xs text-gray-500'>
              {t('开启后专用池无可用节点或耗尽时继续走默认流量池；关闭则直接返回无可用路由。')}
            </div>
          </Col>
        </Row>

        <div className='flex flex-col gap-2 mt-3'>
          {claudeCliPool.targets.length === 0 ? (
            <div className='rounded-lg border border-dashed border-gray-200 bg-gray-50 px-3 py-4 text-center text-sm text-gray-500'>
              {t('未选择 Claude CLI 专用子分组')}
            </div>
          ) : (
            claudeCliPool.targets.map((target, index) => (
              <div
                key={target.real_group}
                className='rounded-lg border border-gray-200 bg-gray-50 px-3 py-3'
              >
                <div className='flex items-center justify-between gap-3 flex-wrap'>
                  <Space>
                    <Tag color='blue' shape='circle'>
                      CLI
                    </Tag>
                    <div>
                      <Text strong>{target.real_group}</Text>
                      <div className='text-xs text-gray-500'>
                        {t('Claude CLI 专用池节点')}
                      </div>
                    </div>
                  </Space>
                  <div className='flex items-center gap-2'>
                    {isClusterMode ? (
                      <>
                        <div className='text-right leading-tight'>
                          <Text type='secondary' className='text-xs'>
                            {t('专用权重')}
                          </Text>
                          <div className='text-[11px] text-gray-500'>
                            {t('仅 Cluster 专用池内生效')}
                          </div>
                        </div>
                        <InputNumber
                          min={0}
                          value={target.weight}
                          onChange={(value) =>
                            updateClaudeCliPoolWeight(index, value)
                          }
                          disabled={!clientRoutePools.enabled || !claudeCliPool.enabled}
                          aria-label={t('Claude CLI 专用池权重')}
                          style={{ width: 112 }}
                        />
                      </>
                    ) : (
                      <div className='text-right leading-tight'>
                        <Text type='secondary' className='text-xs'>
                          {t('链路顺序')}
                        </Text>
                        <div className='text-[11px] text-gray-500'>
                          {index + 1}
                        </div>
                      </div>
                    )}
                    <Button
                      theme='borderless'
                      icon={<IconArrowUp />}
                      disabled={
                        !clientRoutePools.enabled ||
                        !claudeCliPool.enabled ||
                        index === 0
                      }
                      onClick={() => moveClaudeCliPoolTarget(index, -1)}
                      aria-label={t('上移 Claude CLI 专用子分组')}
                    />
                    <Button
                      theme='borderless'
                      icon={<IconArrowDown />}
                      disabled={
                        !clientRoutePools.enabled ||
                        !claudeCliPool.enabled ||
                        index === claudeCliPool.targets.length - 1
                      }
                      onClick={() => moveClaudeCliPoolTarget(index, 1)}
                      aria-label={t('下移 Claude CLI 专用子分组')}
                    />
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  );

  const handleSubmit = async () => {
    setLoading(true);
    try {
      const payload = {
        id: inputs.id,
        name: inputs.name.trim(),
        display_name: inputs.display_name.trim(),
        description: inputs.description.trim(),
        status: inputs.status,
        group_ratio: Number(inputs.group_ratio),
        routing_mode: inputs.routing_mode || 'failover',
        smart_routing_enabled: inputs.smart_routing_enabled,
        recovery_enabled: inputs.recovery_enabled,
        recovery_interval_seconds: Number(inputs.recovery_interval_seconds),
        cluster_affinity_ttl_seconds: Number(
          inputs.cluster_affinity_ttl_seconds || 300,
        ),
        route_affinity_strategy:
          inputs.route_affinity_strategy || 'platform_user',
        route_affinity_key_sources: normalizeRouteAffinityKeySources(
          inputs.route_affinity_key_sources,
        ),
        retry_status_codes: inputs.retry_status_codes.trim(),
        visible_user_groups: inputs.visible_user_groups,
        targets: inputs.targets.map((target) => ({
          real_group: target.real_group,
          weight: Number(target.weight || 0),
        })),
        client_route_pools: {
          enabled: clientRoutePools.enabled,
          claude_code_cli: {
            enabled: claudeCliPool.enabled,
            fallback_to_default: claudeCliPool.fallback_to_default,
            targets: claudeCliPool.targets.map((target) => ({
              real_group: target.real_group,
              weight: Number(target.weight || 0),
            })),
          },
        },
      };
      const res = isEdit
        ? await API.put('/api/aggregate_group', payload)
        : await API.post('/api/aggregate_group', payload);
      const { success, message } = res.data;
      if (!success) {
        showError(t(message));
        return;
      }
      showSuccess(isEdit ? t('聚合分组更新成功') : t('聚合分组创建成功'));
      onSuccess?.();
    } catch (error) {
      showError(error?.message || t('保存失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <SideSheet
      visible={visible}
      onCancel={onClose}
      placement='right'
      width={isMobile ? '100%' : 760}
      bodyStyle={{ padding: '0' }}
      closeIcon={null}
      title={
        <Space>
          <Tag color={isEdit ? 'blue' : 'green'} shape='circle'>
            {isEdit ? t('更新') : t('新建')}
          </Tag>
          <Title heading={4} className='m-0'>
            {isEdit ? t('编辑聚合分组') : t('创建聚合分组')}
          </Title>
        </Space>
      }
      footer={
        <div className='flex justify-end bg-white'>
          <Space>
            <Button
              theme='solid'
              className='!rounded-lg'
              icon={<IconSave />}
              loading={loading}
              onClick={handleSubmit}
            >
              {t('提交')}
            </Button>
            <Button
              theme='light'
              type='primary'
              className='!rounded-lg'
              icon={<IconClose />}
              onClick={onClose}
            >
              {t('取消')}
            </Button>
          </Space>
        </div>
      }
    >
      <div className='p-2'>
        <Card className='!rounded-lg shadow-sm border-0'>
          <div className='flex items-center mb-2'>
            <Avatar size='small' color='blue' className='mr-2 shadow-md'>
              <IconServer size={16} />
            </Avatar>
            <div>
              <Text className='text-lg font-medium'>{t('基本信息')}</Text>
              <div className='text-xs text-gray-600'>
                {t('配置聚合分组名称、倍率和可见范围')}
              </div>
            </div>
          </div>
          <Row gutter={12}>
            <Col xs={24} sm={12}>
              <div className='mb-2'>
                <Text strong>{t('分组名称')}</Text>
              </div>
              <Input
                value={inputs.name}
                onChange={(value) => updateField('name', value)}
                placeholder={t('请输入唯一分组名称')}
              />
            </Col>
            <Col xs={24} sm={12}>
              <div className='mb-2'>
                <Text strong>{t('显示名称')}</Text>
              </div>
              <Input
                value={inputs.display_name}
                onChange={(value) => updateField('display_name', value)}
                placeholder={t('请输入显示名称')}
              />
            </Col>
            <Col span={24}>
              <div className='mb-2'>
                <Text strong>{t('描述')}</Text>
              </div>
              <TextArea
                value={inputs.description}
                onChange={(value) => updateField('description', value)}
                placeholder={t('可选，填写面向管理员的描述')}
                autosize
              />
            </Col>
            <Col xs={24} sm={8}>
              <div className='mb-2'>
                <Text strong>{t('聚合倍率')}</Text>
              </div>
              <InputNumber
                min={0}
                step={0.1}
                value={inputs.group_ratio}
                onChange={(value) => updateField('group_ratio', value)}
                style={{ width: '100%' }}
              />
            </Col>
            <Col xs={12} sm={8}>
              <div className='mb-2'>
                <Text strong>{t('启用状态')}</Text>
              </div>
              <Switch
                checked={inputs.status === 1}
                onChange={(checked) => updateField('status', checked ? 1 : 2)}
              />
            </Col>
            <Col xs={12} sm={8}>
              <div className='mb-2'>
                <Text strong>{t('当前分组启用智能策略')}</Text>
              </div>
              <Switch
                checked={inputs.smart_routing_enabled}
                onChange={(checked) =>
                  updateField('smart_routing_enabled', checked)
                }
              />
            </Col>
            <Col xs={24} sm={12}>
              <div className='mb-2'>
                <Text strong>{t('可见用户组')}</Text>
              </div>
              <Select
                placeholder={t('请选择用户身份组')}
                value={inputs.visible_user_groups}
                onChange={(value) =>
                  updateField('visible_user_groups', value || [])
                }
                optionList={userGroupOptions}
                multiple
                filter={selectFilter}
                searchPosition='dropdown'
                autoClearSearchValue={false}
                style={{ width: '100%' }}
              />
            </Col>
          </Row>
        </Card>

        <Card className='!rounded-lg shadow-sm border-0 mt-3'>
          <div className='flex items-center mb-2'>
            <Avatar size='small' color='green' className='mr-2 shadow-md'>
              <IconServer size={16} />
            </Avatar>
            <div>
              <Text className='text-lg font-medium'>{t('路由模式配置')}</Text>
              <div className='text-xs text-gray-600'>
                {t('选择当前聚合分组的子分组路由方式，并配置对应参数')}
              </div>
            </div>
          </div>
          <Tabs
            activeKey={inputs.routing_mode || 'failover'}
            onChange={handleRoutingModeChange}
            type='button'
          >
            <TabPane itemKey='failover' tab={t('Failover 故障转移')}>
              <div className='mt-3'>
                <div className='rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-xs text-blue-700'>
                  {t(
                    'Failover 按真实分组顺序失败切换，适合主备高可用；旧聚合分组默认保持该模式。',
                  )}
                </div>
                <Row gutter={12} className='mt-3'>
                  <Col xs={12} sm={8}>
                    <div className='mb-2'>
                      <Text strong>{t('懒恢复')}</Text>
                    </div>
                    <Switch
                      checked={inputs.recovery_enabled}
                      onChange={(checked) =>
                        updateField('recovery_enabled', checked)
                      }
                    />
                    <div className='mt-1 text-xs text-gray-500'>
                      {t('开启后按恢复间隔回到链路头部重试')}
                    </div>
                  </Col>
                  <Col xs={24} sm={8}>
                    <div className='mb-2'>
                      <Text strong>{t('恢复间隔（秒）')}</Text>
                    </div>
                    <InputNumber
                      min={1}
                      value={inputs.recovery_interval_seconds}
                      onChange={(value) =>
                        updateField('recovery_interval_seconds', value)
                      }
                      disabled={!inputs.recovery_enabled}
                      style={{ width: '100%' }}
                    />
                    <div className='mt-1 text-xs text-gray-500'>
                      {t('仅 Failover 懒恢复使用')}
                    </div>
	                  </Col>
	                  {renderRetryStatusCodes()}
	                </Row>
	                {renderTargetList()}
	              </div>
	            </TabPane>
            <TabPane itemKey='cluster' tab={t('Cluster 集群分发')}>
              <div className='mt-3'>
                <div className='rounded-lg border border-green-100 bg-green-50 px-3 py-2 text-xs text-green-700'>
                  {t(
                    'Cluster 让多个子分组同时承接流量；同一用户在亲和保持时间内尽量固定到同一可用子分组。',
                  )}
                </div>
                <Row gutter={12} className='mt-3'>
                  <Col xs={24} sm={12}>
                    <div className='mb-2'>
                      <Text strong>{t('Cluster 亲和保持时间（秒）')}</Text>
                    </div>
                    <InputNumber
                      min={1}
                      value={inputs.cluster_affinity_ttl_seconds}
                      onChange={(value) =>
                        updateField('cluster_affinity_ttl_seconds', value || 300)
                      }
                      style={{ width: '100%' }}
                    />
                    <div className='mt-1 text-xs text-gray-500'>
                      {t('同一用户尽量固定到同一子分组的时间，到期后重新按权重选择。')}
                    </div>
                  </Col>
	                  {renderRetryStatusCodes()}
	                </Row>
	                {renderRouteAffinityConfig()}
	                {renderTargetList()}
	              </div>
	            </TabPane>
          </Tabs>
          {renderClientRoutePools()}
        </Card>
      </div>
    </SideSheet>
  );
};

export default EditAggregateGroupModal;
