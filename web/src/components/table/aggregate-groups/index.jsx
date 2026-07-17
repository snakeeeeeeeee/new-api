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
  compareObjects,
  isRoot,
  showError,
  showSuccess,
  showWarning,
  stringToColor,
  toBoolean,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';
import CardPro from '../../common/ui/CardPro';
import CardTable from '../../common/ui/CardTable';
import {
  Button,
  Card,
  Col,
  Input,
  InputNumber,
  Popconfirm,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import EditAggregateGroupModal from './EditAggregateGroupModal';
import AggregateGroupRuntimeDrawer from './AggregateGroupRuntimeDrawer';
import AggregateGroupCategorySideSheet from './AggregateGroupCategorySideSheet';
import { Search, Tags } from 'lucide-react';

const { Text } = Typography;

const defaultStrategyInputs = {
  'aggregate_group.smart_strategy_enabled': false,
  'aggregate_group.failure_rate_window_seconds': 60,
  'aggregate_group.failure_rate_min_requests': 100,
  'aggregate_group.failure_rate_threshold_percent': 5,
  'aggregate_group.slow_rate_window_seconds': 60,
  'aggregate_group.slow_rate_min_requests': 100,
  'aggregate_group.slow_rate_threshold_percent': 30,
  'aggregate_group.degrade_duration_seconds': 600,
  'aggregate_group.cluster_degraded_weight_percent': 50,
  'aggregate_group.slow_request_threshold_seconds': 30,
  'aggregate_group.slow_first_response_threshold_seconds': 0,
};

const isUserVisibleGroup = (group) =>
  group === 'default' || String(group || '').startsWith('UserGroup-');

const isRealRouteGroup = (group) => !isUserVisibleGroup(group);

const normalizeSearchValue = (value) => String(value || '').toLowerCase();
const defaultSearchFilters = {
  aggregate: '',
  target: '',
  visibleGroup: '',
  status: 'all',
  category: 'all',
};

const AggregateGroupsPage = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [optionsLoading, setOptionsLoading] = useState(false);
  const [groups, setGroups] = useState([]);
  const [categories, setCategories] = useState([]);
  const [categoriesLoading, setCategoriesLoading] = useState(false);
  const [realGroupOptions, setRealGroupOptions] = useState([]);
  const [userGroupOptions, setUserGroupOptions] = useState([]);
  const [editingGroup, setEditingGroup] = useState({ id: undefined });
  const [showEdit, setShowEdit] = useState(false);
  const [runtimeGroup, setRuntimeGroup] = useState(null);
  const [showRuntime, setShowRuntime] = useState(false);
  const [showCategories, setShowCategories] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState([]);
  const [batchCategoryId, setBatchCategoryId] = useState(undefined);
  const [batchAssigning, setBatchAssigning] = useState(false);
  const [strategyInputs, setStrategyInputs] = useState(defaultStrategyInputs);
  const [strategyInputsRow, setStrategyInputsRow] = useState(
    defaultStrategyInputs,
  );
  const [searchFilters, setSearchFilters] = useState(defaultSearchFilters);

  const loadGroups = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/aggregate_group/');
      const { success, message, data } = res.data;
      if (!success) {
        showError(t(message));
        return;
      }
      setGroups(
        (data || []).map((item) => ({
          ...item,
          key: item.id,
        })),
      );
    } catch (error) {
      showError(error?.message || t('获取聚合分组失败'));
    } finally {
      setLoading(false);
    }
  };

  const loadGroupOptions = async () => {
    try {
      const res = await API.get('/api/group/');
      const { success, data, message } = res.data;
      if (!success) {
        showError(t(message));
        return;
      }
      const options = (data || []).map((group) => ({
        label: group,
        value: group,
      }));
      setRealGroupOptions(
        options.filter((option) => isRealRouteGroup(option.value)),
      );
      setUserGroupOptions(
        options.filter((option) => isUserVisibleGroup(option.value)),
      );
    } catch (error) {
      showError(error?.message || t('获取分组选项失败'));
    }
  };

  const loadCategories = async () => {
    setCategoriesLoading(true);
    try {
      const res = await API.get('/api/aggregate_group/categories');
      const { success, data, message } = res.data;
      if (!success) {
        showError(t(message));
        return [];
      }
      const nextCategories = data || [];
      setCategories(nextCategories);
      return nextCategories;
    } catch (error) {
      showError(error?.message || t('获取业务分类失败'));
      return [];
    } finally {
      setCategoriesLoading(false);
    }
  };

  const refreshGroupsAndCategories = async () => {
    await Promise.all([loadGroups(), loadCategories()]);
  };

  const loadStrategyOptions = async () => {
    setOptionsLoading(true);
    try {
      const res = await API.get('/api/option/');
      const { success, data, message } = res.data;
      if (!success) {
        showError(t(message));
        return;
      }
      const nextInputs = { ...defaultStrategyInputs };
      (data || []).forEach((item) => {
        if (!(item.key in nextInputs)) {
          return;
        }
        if (item.key === 'aggregate_group.smart_strategy_enabled') {
          nextInputs[item.key] = toBoolean(item.value);
          return;
        }
        const parsedValue = Number(item.value);
        if (!Number.isNaN(parsedValue) && parsedValue >= 0) {
          nextInputs[item.key] = parsedValue;
        }
      });
      setStrategyInputs(nextInputs);
      setStrategyInputsRow(structuredClone(nextInputs));
    } catch (error) {
      showError(error?.message || t('获取聚合分组全局策略失败'));
    } finally {
      setOptionsLoading(false);
    }
  };

  useEffect(() => {
    loadGroups();
    loadCategories();
    loadGroupOptions();
    if (isRoot()) {
      loadStrategyOptions();
    }
  }, []);

  const handleDelete = async (id) => {
    try {
      const res = await API.delete(`/api/aggregate_group/${id}`);
      const { success, message } = res.data;
      if (!success) {
        showError(t(message));
        return;
      }
      showSuccess(t('删除成功'));
      await refreshGroupsAndCategories();
    } catch (error) {
      showError(error?.message || t('删除失败'));
    }
  };

  const updateStrategyField = (key, value) => {
    setStrategyInputs((prev) => ({
      ...prev,
      [key]: value,
    }));
  };

  const handleSaveStrategy = async () => {
    const updateArray = compareObjects(strategyInputs, strategyInputsRow);
    if (!updateArray.length) {
      showWarning(t('你似乎并没有修改什么'));
      return;
    }
    setOptionsLoading(true);
    try {
      await Promise.all(
        updateArray.map((item) =>
          API.put('/api/option/', {
            key: item.key,
            value: strategyInputs[item.key],
          }),
        ),
      );
      showSuccess(t('聚合分组全局策略保存成功'));
      setStrategyInputsRow(structuredClone(strategyInputs));
    } catch (error) {
      showError(error?.message || t('聚合分组全局策略保存失败'));
    } finally {
      setOptionsLoading(false);
    }
  };

  const updateSearchFilter = (key, value) => {
    setSelectedRowKeys([]);
    setBatchCategoryId(undefined);
    setSearchFilters((prev) => ({
      ...prev,
      [key]: value ?? '',
    }));
  };

  const filteredGroups = useMemo(() => {
    const aggregateKeyword = normalizeSearchValue(
      searchFilters.aggregate.trim(),
    );
    const targetKeyword = normalizeSearchValue(searchFilters.target.trim());
    const visibleGroupKeyword = normalizeSearchValue(
      searchFilters.visibleGroup.trim(),
    );
    const statusFilter = searchFilters.status || 'all';
    const categoryFilter = searchFilters.category;

    if (
      !aggregateKeyword &&
      !targetKeyword &&
      !visibleGroupKeyword &&
      statusFilter === 'all' &&
      categoryFilter === 'all'
    ) {
      return groups;
    }

    return groups.filter((group) => {
      if (statusFilter === 'enabled' && group.status !== 1) {
        return false;
      }
      if (statusFilter === 'disabled' && group.status === 1) {
        return false;
      }
      if (
        categoryFilter !== 'all' &&
        Number(group.category_id || 0) !== Number(categoryFilter)
      ) {
        return false;
      }

      const aggregateText = [group.name, group.display_name]
        .map(normalizeSearchValue)
        .join(' ');
      const targetText = (group.targets || [])
        .map((target) => normalizeSearchValue(target.real_group))
        .join(' ');
      const visibleGroupText = (group.visible_user_groups || [])
        .map(normalizeSearchValue)
        .join(' ');

      return (
        (!aggregateKeyword || aggregateText.includes(aggregateKeyword)) &&
        (!targetKeyword || targetText.includes(targetKeyword)) &&
        (!visibleGroupKeyword || visibleGroupText.includes(visibleGroupKeyword))
      );
    });
  }, [groups, searchFilters]);

  const hasActiveSearchFilters = useMemo(
    () =>
      Boolean(
        searchFilters.aggregate ||
          searchFilters.target ||
          searchFilters.visibleGroup ||
          searchFilters.status !== 'all' ||
          searchFilters.category !== 'all',
      ),
    [searchFilters],
  );

  const categoryOptions = useMemo(
    () => [
      ...categories.map((category) => ({
        label: category.name,
        value: category.id,
      })),
      { label: t('其他'), value: 0 },
    ],
    [categories, t],
  );

  const otherCategoryCount = useMemo(
    () => groups.filter((group) => Number(group.category_id || 0) === 0).length,
    [groups],
  );

  const batchCategoryName = useMemo(
    () =>
      categoryOptions.find((option) => option.value === batchCategoryId)
        ?.label || '',
    [batchCategoryId, categoryOptions],
  );

  const clearSelection = () => {
    setSelectedRowKeys([]);
    setBatchCategoryId(undefined);
  };

  const handleBatchAssign = async () => {
    if (selectedRowKeys.length === 0 || batchCategoryId === undefined) return;
    setBatchAssigning(true);
    try {
      const res = await API.put('/api/aggregate_group/categories/assign', {
        aggregate_group_ids: selectedRowKeys,
        category_id: batchCategoryId,
      });
      const { success, message } = res.data || {};
      if (!success) {
        showError(t(message));
        return;
      }
      showSuccess(t('批量修改业务分类成功'));
      clearSelection();
      await refreshGroupsAndCategories();
    } catch (error) {
      showError(error?.message || t('批量修改业务分类失败'));
    } finally {
      setBatchAssigning(false);
    }
  };

  const columns = useMemo(
    () => [
      {
        title: t('聚合分组'),
        dataIndex: 'name',
        key: 'name',
        render: (_, record) => (
          <Space>
            <Text strong>{record.name}</Text>
            <Tag
              color={
                (record.routing_mode || 'failover') === 'cluster'
                  ? 'green'
                  : 'blue'
              }
              size='small'
            >
              {(record.routing_mode || 'failover') === 'cluster'
                ? t('Cluster 集群')
                : t('Failover 故障转移')}
            </Tag>
            <Tag color='blue' shape='circle'>
              {record.display_name}
            </Tag>
            {record.smart_routing_enabled ? (
              <Tag color='orange' size='small'>
                {t('已启用智能策略')}
              </Tag>
            ) : null}
            <Tag color={record.status === 1 ? 'green' : 'grey'} size='small'>
              {record.status === 1 ? t('启用') : t('禁用')}
            </Tag>
          </Space>
        ),
      },
      {
        title: t('分类'),
        dataIndex: 'category_name',
        key: 'category_name',
        render: (value) => <Tag size='small'>{value || t('其他')}</Tag>,
      },
      {
        title: t('倍率'),
        dataIndex: 'group_ratio',
        key: 'group_ratio',
        render: (value) => `${value}x`,
      },
      {
        title: t('模型倍率规则'),
        dataIndex: 'enabled_route_model_group_ratio_override_count',
        key: 'enabled_route_model_group_ratio_override_count',
        render: (value = 0) => (
          <Tag color={value > 0 ? 'orange' : 'grey'} size='small'>
            {t('{{count}} 条启用', { count: value })}
          </Tag>
        ),
      },
      {
        title: t('真实分组链'),
        dataIndex: 'targets',
        key: 'targets',
        render: (targets = []) => (
          <Space wrap>
            {targets.map((target, index) => (
              <Tag
                key={`${target.real_group}-${index}`}
                shape='circle'
                color={stringToColor(target.real_group)}
              >
                {index + 1}. {target.real_group}
              </Tag>
            ))}
          </Space>
        ),
      },
      {
        title: t('可见用户组'),
        dataIndex: 'visible_user_groups',
        key: 'visible_user_groups',
        render: (groups = []) => (
          <Space wrap>
            {groups.map((group) => (
              <Tag key={group} shape='circle'>
                {group}
              </Tag>
            ))}
          </Space>
        ),
      },
      {
        title: t('恢复策略'),
        dataIndex: 'recovery_enabled',
        key: 'recovery_enabled',
        render: (_, record) =>
          record.recovery_enabled
            ? t('{{seconds}} 秒后懒恢复', {
                seconds: record.recovery_interval_seconds,
              })
            : t('关闭'),
      },
      {
        title: '',
        key: 'action',
        render: (_, record) => (
          <Space>
            <Button
              size='small'
              onClick={() => {
                setEditingGroup(record);
                setShowEdit(true);
              }}
            >
              {t('编辑')}
            </Button>
            <Button
              size='small'
              theme='outline'
              onClick={() => {
                setRuntimeGroup(record);
                setShowRuntime(true);
              }}
            >
              {t('运行态')}
            </Button>
            <Popconfirm
              title={t('确认删除该聚合分组？')}
              onConfirm={() => handleDelete(record.id)}
            >
              <Button size='small' type='danger'>
                {t('删除')}
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [t],
  );

  return (
    <>
      <EditAggregateGroupModal
        visible={showEdit}
        editingGroup={editingGroup}
        onClose={() => {
          setShowEdit(false);
          setEditingGroup({ id: undefined });
        }}
        onSuccess={() => {
          setShowEdit(false);
          setEditingGroup({ id: undefined });
          refreshGroupsAndCategories();
        }}
        realGroupOptions={realGroupOptions}
        userGroupOptions={userGroupOptions}
        categoryOptions={categoryOptions}
      />

      <AggregateGroupCategorySideSheet
        visible={showCategories}
        categories={categories}
        otherCount={otherCategoryCount}
        onClose={() => setShowCategories(false)}
        onChanged={async () => {
          clearSelection();
          await refreshGroupsAndCategories();
        }}
      />

      <AggregateGroupRuntimeDrawer
        visible={showRuntime}
        aggregateGroup={runtimeGroup}
        onClose={() => {
          setShowRuntime(false);
          setRuntimeGroup(null);
        }}
        t={t}
      />

      <CardPro
        type='type1'
        descriptionArea={
          <div className='flex flex-col gap-3 w-full'>
            <div className='flex flex-col gap-3 md:flex-row md:items-center md:justify-between'>
              <div className='min-w-0'>
                <Text strong>{t('聚合分组管理')}</Text>
                <div className='text-xs text-gray-600 mt-1'>
                  {t('为提高可用性, 配置对外可见的逻辑分组与底层真实分组链路')}
                </div>
              </div>
              <Space wrap>
                <Button
                  theme='outline'
                  icon={<Tags size={16} />}
                  onClick={() => setShowCategories(true)}
                >
                  {t('管理分类')}
                </Button>
                <Button
                  type='primary'
                  onClick={() => {
                    setEditingGroup({ id: undefined });
                    setShowEdit(true);
                  }}
                >
                  {t('新增聚合分组')}
                </Button>
              </Space>
            </div>
            <div className='grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-5'>
              <Input
                prefix={<Search size={16} />}
                showClear
                value={searchFilters.aggregate}
                onChange={(value) => updateSearchFilter('aggregate', value)}
                placeholder={t('搜索聚合分组')}
              />
              <Input
                prefix={<Search size={16} />}
                showClear
                value={searchFilters.target}
                onChange={(value) => updateSearchFilter('target', value)}
                placeholder={t('搜索真实分组链')}
              />
              <Input
                prefix={<Search size={16} />}
                showClear
                value={searchFilters.visibleGroup}
                onChange={(value) => updateSearchFilter('visibleGroup', value)}
                placeholder={t('搜索可见用户组')}
              />
              <Select
                value={searchFilters.status}
                onChange={(value) =>
                  updateSearchFilter('status', value || 'all')
                }
                style={{ width: '100%' }}
              >
                <Select.Option value='all'>{t('全部状态')}</Select.Option>
                <Select.Option value='enabled'>{t('启用')}</Select.Option>
                <Select.Option value='disabled'>{t('禁用')}</Select.Option>
              </Select>
              <Select
                value={searchFilters.category}
                loading={categoriesLoading}
                onChange={(value) => updateSearchFilter('category', value)}
                style={{ width: '100%' }}
              >
                <Select.Option value='all'>{t('全部分类')}</Select.Option>
                {categoryOptions.map((option) => (
                  <Select.Option key={option.value} value={option.value}>
                    {option.label}
                  </Select.Option>
                ))}
              </Select>
            </div>
            {hasActiveSearchFilters ? (
              <Text type='tertiary' className='text-xs'>
                {t('匹配 {{count}} / {{total}} 个', {
                  count: filteredGroups.length,
                  total: groups.length,
                })}
              </Text>
            ) : null}
          </div>
        }
        t={t}
      >
        {isRoot() ? (
          <Spin spinning={optionsLoading}>
            <Card className='!rounded-2xl shadow-sm border-0 mb-3'>
              <div className='flex items-center justify-between gap-2 mb-3'>
                <div>
                  <Text strong>{t('聚合分组全局策略')}</Text>
                  <div className='text-xs text-gray-600 mt-1'>
                    {t(
                      '为开启智能策略的聚合分组配置滑动窗口错误率、慢率、临时降级和 Cluster 有效权重',
                    )}
                  </div>
                </div>
                <Button type='primary' onClick={handleSaveStrategy}>
                  {t('保存聚合分组策略')}
                </Button>
              </div>
              <Row gutter={16}>
                <Col xs={24} sm={12} md={8}>
                  <div className='mb-2'>
                    <Text strong>{t('启用智能策略')}</Text>
                  </div>
                  <Switch
                    checked={
                      strategyInputs['aggregate_group.smart_strategy_enabled']
                    }
                    onChange={(checked) =>
                      updateStrategyField(
                        'aggregate_group.smart_strategy_enabled',
                        checked,
                      )
                    }
                  />
                </Col>
                <Col xs={24} sm={12} md={8}>
                  <div className='mb-2'>
                    <Text strong>{t('错误率窗口（秒）')}</Text>
                    <div className='text-xs text-gray-500 mt-1'>
                      {t('统计最近窗口内的可重试失败占尝试请求比例')}
                    </div>
                  </div>
                  <InputNumber
                    min={1}
                    max={3600}
                    value={
                      strategyInputs[
                        'aggregate_group.failure_rate_window_seconds'
                      ]
                    }
                    onChange={(value) =>
                      updateStrategyField(
                        'aggregate_group.failure_rate_window_seconds',
                        Number(value) || 1,
                      )
                    }
                    style={{ width: '100%' }}
                  />
                </Col>
                <Col xs={24} sm={12} md={8}>
                  <div className='mb-2'>
                    <Text strong>{t('错误率最小样本数')}</Text>
                  </div>
                  <InputNumber
                    min={1}
                    value={
                      strategyInputs[
                        'aggregate_group.failure_rate_min_requests'
                      ]
                    }
                    onChange={(value) =>
                      updateStrategyField(
                        'aggregate_group.failure_rate_min_requests',
                        Number(value) || 1,
                      )
                    }
                    style={{ width: '100%' }}
                  />
                </Col>
                <Col xs={24} sm={12} md={8}>
                  <div className='mb-2'>
                    <Text strong>{t('错误率阈值（%）')}</Text>
                  </div>
                  <InputNumber
                    min={1}
                    max={100}
                    value={
                      strategyInputs[
                        'aggregate_group.failure_rate_threshold_percent'
                      ]
                    }
                    onChange={(value) =>
                      updateStrategyField(
                        'aggregate_group.failure_rate_threshold_percent',
                        Number(value) || 1,
                      )
                    }
                    style={{ width: '100%' }}
                  />
                </Col>
                <Col xs={24} sm={12} md={8}>
                  <div className='mb-2'>
                    <Text strong>{t('慢率窗口（秒）')}</Text>
                    <div className='text-xs text-gray-500 mt-1'>
                      {t('统计最近窗口内慢成功请求占成功请求比例')}
                    </div>
                  </div>
                  <InputNumber
                    min={1}
                    max={3600}
                    value={
                      strategyInputs['aggregate_group.slow_rate_window_seconds']
                    }
                    onChange={(value) =>
                      updateStrategyField(
                        'aggregate_group.slow_rate_window_seconds',
                        Number(value) || 1,
                      )
                    }
                    style={{ width: '100%' }}
                  />
                </Col>
                <Col xs={24} sm={12} md={8}>
                  <div className='mb-2'>
                    <Text strong>{t('慢率最小样本数')}</Text>
                  </div>
                  <InputNumber
                    min={1}
                    value={
                      strategyInputs['aggregate_group.slow_rate_min_requests']
                    }
                    onChange={(value) =>
                      updateStrategyField(
                        'aggregate_group.slow_rate_min_requests',
                        Number(value) || 1,
                      )
                    }
                    style={{ width: '100%' }}
                  />
                </Col>
                <Col xs={24} sm={12} md={8}>
                  <div className='mb-2'>
                    <Text strong>{t('慢率阈值（%）')}</Text>
                  </div>
                  <InputNumber
                    min={1}
                    max={100}
                    value={
                      strategyInputs[
                        'aggregate_group.slow_rate_threshold_percent'
                      ]
                    }
                    onChange={(value) =>
                      updateStrategyField(
                        'aggregate_group.slow_rate_threshold_percent',
                        Number(value) || 1,
                      )
                    }
                    style={{ width: '100%' }}
                  />
                </Col>
                <Col xs={24} sm={12} md={8}>
                  <div className='mb-2'>
                    <Text strong>{t('临时降级时长（秒）')}</Text>
                  </div>
                  <InputNumber
                    min={1}
                    value={
                      strategyInputs['aggregate_group.degrade_duration_seconds']
                    }
                    onChange={(value) =>
                      updateStrategyField(
                        'aggregate_group.degrade_duration_seconds',
                        Number(value) || 1,
                      )
                    }
                    style={{ width: '100%' }}
                  />
                </Col>
                <Col xs={24} sm={12} md={8}>
                  <div className='mb-2'>
                    <Text strong>{t('Cluster 降级有效权重比例（%）')}</Text>
                    <div className='text-xs text-gray-500 mt-1'>
                      {t('仅 Cluster 模式生效；正数权重降级后最低保留 1')}
                    </div>
                  </div>
                  <InputNumber
                    min={1}
                    max={100}
                    value={
                      strategyInputs[
                        'aggregate_group.cluster_degraded_weight_percent'
                      ]
                    }
                    onChange={(value) =>
                      updateStrategyField(
                        'aggregate_group.cluster_degraded_weight_percent',
                        Number(value) || 50,
                      )
                    }
                    style={{ width: '100%' }}
                  />
                </Col>
                <Col xs={24} sm={12} md={8}>
                  <div className='mb-2'>
                    <Text strong>{t('慢请求阈值（秒）')}</Text>
                    <div className='text-xs text-gray-500 mt-1'>
                      {t('按请求总耗时统计慢成功请求')}
                    </div>
                  </div>
                  <InputNumber
                    min={1}
                    value={
                      strategyInputs[
                        'aggregate_group.slow_request_threshold_seconds'
                      ]
                    }
                    onChange={(value) =>
                      updateStrategyField(
                        'aggregate_group.slow_request_threshold_seconds',
                        Number(value) || 1,
                      )
                    }
                    style={{ width: '100%' }}
                  />
                </Col>
                <Col xs={24} sm={12} md={8}>
                  <div className='mb-2'>
                    <Text strong>{t('首字慢阈值（秒）')}</Text>
                    <div className='text-xs text-gray-500 mt-1'>
                      {t('仅流式请求生效；0 表示关闭首字慢统计')}
                    </div>
                  </div>
                  <InputNumber
                    min={0}
                    value={
                      strategyInputs[
                        'aggregate_group.slow_first_response_threshold_seconds'
                      ]
                    }
                    onChange={(value) =>
                      updateStrategyField(
                        'aggregate_group.slow_first_response_threshold_seconds',
                        Math.max(0, Number(value) || 0),
                      )
                    }
                    style={{ width: '100%' }}
                  />
                </Col>
              </Row>
            </Card>
          </Spin>
        ) : null}
        {selectedRowKeys.length > 0 ? (
          <div
            className='mb-3 flex flex-col gap-3 border-y py-3 sm:flex-row sm:items-center'
            style={{ borderColor: 'var(--semi-color-border)' }}
          >
            <Text strong>
              {t('已选择 {{total}} 项', { total: selectedRowKeys.length })}
            </Text>
            <Select
              value={batchCategoryId}
              placeholder={t('修改为')}
              disabled={batchAssigning}
              onChange={setBatchCategoryId}
              style={{ width: '100%', maxWidth: 240 }}
              optionList={categoryOptions}
            />
            <Space wrap>
              <Popconfirm
                disabled={batchCategoryId === undefined || batchAssigning}
                title={t('将 {{total}} 个聚合分组修改为“{{category}}”分类？', {
                  total: selectedRowKeys.length,
                  category: batchCategoryName,
                })}
                onConfirm={handleBatchAssign}
              >
                <Button
                  type='primary'
                  loading={batchAssigning}
                  disabled={batchCategoryId === undefined || batchAssigning}
                >
                  {t('应用')}
                </Button>
              </Popconfirm>
              <Button
                theme='borderless'
                disabled={batchAssigning}
                onClick={clearSelection}
              >
                {t('清除选择')}
              </Button>
            </Space>
          </div>
        ) : null}
        <CardTable
          rowKey='id'
          columns={columns}
          dataSource={filteredGroups}
          loading={loading}
          hidePagination
          rowSelection={{
            selectedRowKeys,
            onChange: (keys) => setSelectedRowKeys(keys),
          }}
        />
      </CardPro>
    </>
  );
};

export default AggregateGroupsPage;
