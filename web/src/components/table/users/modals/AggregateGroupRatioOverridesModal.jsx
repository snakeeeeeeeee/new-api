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
  AutoComplete,
  Button,
  Empty,
  InputNumber,
  Select,
  SideSheet,
  Space,
  Switch,
  TabPane,
  Tabs,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { IconDelete, IconPlusCircle } from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { API, selectFilter, showError, showSuccess } from '../../../../helpers';
import {
  aggregateRouteModelRuleKey,
  getAggregateRouteGroups,
  normalizeAggregateRatioOverrides,
  normalizeUserRouteModelRatioOverrides,
  resolveUserRouteModelRatio,
} from '../../../../helpers/userAggregateRatioOverrides';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import CardTable from '../../../common/ui/CardTable';

const { Text } = Typography;

const formatRatio = (value) => {
  const numberValue = Number(value);
  if (!Number.isFinite(numberValue)) return '-';
  return `${Number(numberValue.toFixed(6))}x`;
};

const formatRatioInput = (value) => {
  if (value === undefined || value === null || value === '') return '';
  const stringValue = String(value);
  if (!/^-?\d+(?:\.\d{6})$/.test(stringValue)) return stringValue;
  return stringValue.replace(/(\.\d*?[1-9])0+$/, '$1').replace(/\.0+$/, '');
};

const emptyState = (description) => (
  <Empty
    image={<IllustrationNoResult style={{ width: 140, height: 140 }} />}
    darkModeImage={
      <IllustrationNoResultDark style={{ width: 140, height: 140 }} />
    }
    description={description}
    style={{ padding: 28 }}
  />
);

const AggregateGroupRatioOverridesModal = ({
  visible,
  onCancel,
  user,
  t,
  onSuccess,
}) => {
  const isMobile = useIsMobile();
  const [activeTab, setActiveTab] = useState('aggregate');
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [aggregateGroups, setAggregateGroups] = useState([]);
  const [defaultRows, setDefaultRows] = useState([]);
  const [routeRows, setRouteRows] = useState([]);
  const [selectedGroup, setSelectedGroup] = useState(undefined);
  const [newRatio, setNewRatio] = useState(1);
  const [draftRule, setDraftRule] = useState({
    aggregate_group: '',
    real_group: '',
    model_name: '',
    group_ratio: 1,
    enabled: true,
  });
  const [modelOptions, setModelOptions] = useState({});
  const [modelLoading, setModelLoading] = useState({});

  const groupMap = useMemo(
    () =>
      new Map(
        (aggregateGroups || [])
          .filter((group) => group?.name)
          .map((group) => [group.name, group]),
      ),
    [aggregateGroups],
  );

  const aggregateOverrides = useMemo(
    () =>
      Object.fromEntries(
        defaultRows.map((row) => [row.group, Number(row.ratio)]),
      ),
    [defaultRows],
  );

  const selectedDefaultGroups = useMemo(
    () => new Set(defaultRows.map((row) => row.group)),
    [defaultRows],
  );

  const aggregateGroupOptions = useMemo(
    () =>
      (aggregateGroups || [])
        .filter((group) => Number(group?.status) === 1)
        .map((group) => ({
          label:
            group.display_name && group.display_name !== group.name
              ? `${group.display_name} (${group.name})`
              : group.name,
          value: group.name,
        })),
    [aggregateGroups],
  );

  const defaultGroupOptions = useMemo(
    () =>
      aggregateGroupOptions.map((option) => ({
        ...option,
        disabled: selectedDefaultGroups.has(option.value),
      })),
    [aggregateGroupOptions, selectedDefaultGroups],
  );

  const routeGroupOptions = (aggregateGroupName) =>
    getAggregateRouteGroups(groupMap.get(aggregateGroupName)).map((group) => ({
      label: group,
      value: group,
    }));

  const modelCacheKey = (aggregateGroup, realGroup) =>
    JSON.stringify([aggregateGroup || '', realGroup || '']);

  const loadModels = async (aggregateGroup, realGroup) => {
    if (!user?.id || !aggregateGroup || !realGroup) return;
    const key = modelCacheKey(aggregateGroup, realGroup);
    if (modelOptions[key] || modelLoading[key]) return;
    setModelLoading((prev) => ({ ...prev, [key]: true }));
    try {
      const res = await API.get(
        `/api/user/${user.id}/aggregate_group_ratio_overrides/models`,
        { params: { aggregate_group: aggregateGroup, real_group: realGroup } },
      );
      if (!res.data?.success) {
        showError(res.data?.message || t('获取子分组模型失败'));
        return;
      }
      setModelOptions((prev) => ({
        ...prev,
        [key]: (res.data.data || []).map((modelName) => ({
          label: modelName,
          value: modelName,
        })),
      }));
    } catch (error) {
      showError(error.response?.data?.message || error.message);
    } finally {
      setModelLoading((prev) => ({ ...prev, [key]: false }));
    }
  };

  const modelsForRule = (rule) => {
    const key = modelCacheKey(rule.aggregate_group, rule.real_group);
    const options = [...(modelOptions[key] || [])];
    if (
      rule.model_name &&
      !options.some((option) => option.value === rule.model_name)
    ) {
      options.unshift({ label: rule.model_name, value: rule.model_name });
    }
    return options;
  };

  const resetDraftRule = (groups = aggregateGroups) => {
    const firstGroup = (groups || []).find(
      (group) => Number(group?.status) === 1,
    );
    const firstRealGroup = getAggregateRouteGroups(firstGroup)[0] || '';
    setDraftRule({
      aggregate_group: firstGroup?.name || '',
      real_group: firstRealGroup,
      model_name: '',
      group_ratio: Number(firstGroup?.group_ratio ?? 1),
      enabled: true,
    });
  };

  const loadOverrides = async () => {
    if (!user?.id) return;
    setLoading(true);
    try {
      const res = await API.get(
        `/api/user/${user.id}/aggregate_group_ratio_overrides`,
      );
      if (!res.data?.success) {
        showError(res.data?.message || t('加载失败'));
        return;
      }
      const data = res.data.data || {};
      const groups = data.aggregate_groups || [];
      setAggregateGroups(groups);
      setDefaultRows(normalizeAggregateRatioOverrides(data.overrides));
      setRouteRows(
        normalizeUserRouteModelRatioOverrides(
          data.route_model_group_ratio_overrides,
          groups,
        ).map((rule, index) => ({ ...rule, _row_id: `stored-${index}` })),
      );
      resetDraftRule(groups);
    } catch (error) {
      showError(error.response?.data?.message || error.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible) return;
    setActiveTab('aggregate');
    setSelectedGroup(undefined);
    setNewRatio(1);
    setModelOptions({});
    setModelLoading({});
    loadOverrides();
  }, [visible, user?.id]);

  const addDefaultOverride = () => {
    const ratio = Number(newRatio);
    if (!selectedGroup) {
      showError(t('请选择聚合分组'));
      return;
    }
    if (!Number.isFinite(ratio) || ratio < 0) {
      showError(t('特殊倍率不能小于 0'));
      return;
    }
    if (selectedDefaultGroups.has(selectedGroup)) {
      showError(t('该聚合分组已配置'));
      return;
    }
    setDefaultRows((prev) => [
      ...prev,
      { key: selectedGroup, group: selectedGroup, ratio },
    ]);
    setSelectedGroup(undefined);
    setNewRatio(1);
  };

  const updateRouteRow = (rowID, patch) => {
    setRouteRows((prev) =>
      prev.map((row) => (row._row_id === rowID ? { ...row, ...patch } : row)),
    );
  };

  const addRouteRule = () => {
    const aggregateGroup = String(draftRule.aggregate_group || '').trim();
    const realGroup = String(draftRule.real_group || '').trim();
    const modelName = String(draftRule.model_name || '').trim();
    const groupRatio = Number(draftRule.group_ratio);
    if (!groupMap.has(aggregateGroup)) {
      showError(t('请选择聚合分组'));
      return;
    }
    if (!routeGroupOptions(aggregateGroup).some((o) => o.value === realGroup)) {
      showError(t('倍率规则引用了未配置的子分组'));
      return;
    }
    if (!modelName) {
      showError(t('倍率规则的模型名称不能为空'));
      return;
    }
    if (!Number.isFinite(groupRatio) || groupRatio < 0) {
      showError(t('倍率规则必须使用大于等于 0 的有限数值'));
      return;
    }
    const rule = {
      aggregate_group: aggregateGroup,
      real_group: realGroup,
      model_name: modelName,
      group_ratio: groupRatio,
      enabled: draftRule.enabled !== false,
    };
    const key = aggregateRouteModelRuleKey(rule);
    if (routeRows.some((row) => aggregateRouteModelRuleKey(row) === key)) {
      showError(
        t('子分组 {{group}} 的模型 {{model}} 重复配置', {
          group: realGroup,
          model: modelName,
        }),
      );
      return;
    }
    setRouteRows((prev) => [
      ...prev,
      { ...rule, _row_id: `new-${Date.now()}-${prev.length}` },
    ]);
    resetDraftRule();
  };

  const validateRouteRows = () => {
    const seen = new Set();
    for (const row of routeRows) {
      const aggregateGroup = String(row.aggregate_group || '').trim();
      const realGroup = String(row.real_group || '').trim();
      const modelName = String(row.model_name || '').trim();
      const groupRatio = Number(row.group_ratio);
      if (!groupMap.has(aggregateGroup)) return t('请选择聚合分组');
      if (
        !routeGroupOptions(aggregateGroup).some((o) => o.value === realGroup)
      ) {
        return t('倍率规则引用了未配置的子分组');
      }
      if (!modelName) return t('倍率规则的模型名称不能为空');
      if (!Number.isFinite(groupRatio) || groupRatio < 0) {
        return t('倍率规则必须使用大于等于 0 的有限数值');
      }
      const key = aggregateRouteModelRuleKey({
        aggregate_group: aggregateGroup,
        real_group: realGroup,
        model_name: modelName,
      });
      if (seen.has(key)) {
        return t('子分组 {{group}} 的模型 {{model}} 重复配置', {
          group: realGroup,
          model: modelName,
        });
      }
      seen.add(key);
    }
    return '';
  };

  const saveOverrides = async () => {
    if (!user?.id) {
      showError(t('用户信息缺失'));
      return;
    }
    const overrides = {};
    for (const row of defaultRows) {
      const ratio = Number(row.ratio);
      if (!row.group || !Number.isFinite(ratio) || ratio < 0) {
        showError(t('特殊倍率不能小于 0'));
        return;
      }
      overrides[row.group] = ratio;
    }
    const routeError = validateRouteRows();
    if (routeError) {
      showError(routeError);
      return;
    }
    const routeRules = routeRows.map((row) => ({
      aggregate_group: String(row.aggregate_group).trim(),
      real_group: String(row.real_group).trim(),
      model_name: String(row.model_name).trim(),
      group_ratio: Number(row.group_ratio),
      enabled: row.enabled !== false,
    }));
    setSaving(true);
    try {
      const res = await API.put(
        `/api/user/${user.id}/aggregate_group_ratio_overrides`,
        {
          overrides,
          route_model_group_ratio_overrides: routeRules,
        },
      );
      if (!res.data?.success) {
        showError(res.data?.message || t('保存失败'));
        return;
      }
      const data = res.data.data || {};
      const groups = data.aggregate_groups || aggregateGroups;
      setAggregateGroups(groups);
      setDefaultRows(normalizeAggregateRatioOverrides(data.overrides));
      setRouteRows(
        normalizeUserRouteModelRatioOverrides(
          data.route_model_group_ratio_overrides,
          groups,
        ).map((rule, index) => ({ ...rule, _row_id: `saved-${index}` })),
      );
      showSuccess(t('保存成功'));
      onSuccess?.();
    } catch (error) {
      showError(error.response?.data?.message || error.message);
    } finally {
      setSaving(false);
    }
  };

  const inheritedLabel = (rule) => {
    const group = groupMap.get(rule.aggregate_group);
    const view = resolveUserRouteModelRatio({
      rule,
      aggregateGroup: group,
      aggregateOverrides,
    });
    const source =
      view.inheritedSource === 'global_route_model'
        ? t('配置的子分组模型倍率')
        : view.inheritedSource === 'user_aggregate'
          ? t('特殊倍率')
          : t('默认聚合倍率');
    return { ...view, source };
  };

  const defaultColumns = [
    {
      title: t('聚合分组'),
      dataIndex: 'group',
      render: (groupName) => (
        <div className='min-w-0'>
          <Text strong>
            {groupMap.get(groupName)?.display_name || groupName}
          </Text>
          <div className='text-xs text-gray-500 truncate'>{groupName}</div>
        </div>
      ),
    },
    {
      title: t('默认聚合倍率'),
      width: 130,
      render: (_, row) => formatRatio(groupMap.get(row.group)?.group_ratio),
    },
    {
      title: t('特殊倍率'),
      width: 180,
      render: (_, row) => (
        <InputNumber
          min={0}
          precision={6}
          formatter={formatRatioInput}
          step={0.1}
          value={row.ratio}
          onChange={(value) =>
            setDefaultRows((prev) =>
              prev.map((item) =>
                item.group === row.group ? { ...item, ratio: value } : item,
              ),
            )
          }
          style={{ width: '100%' }}
        />
      ),
    },
    {
      title: t('操作'),
      width: 70,
      render: (_, row) => (
        <Tooltip content={t('删除')}>
          <Button
            icon={<IconDelete />}
            type='danger'
            theme='borderless'
            aria-label={t('删除')}
            onClick={() =>
              setDefaultRows((prev) =>
                prev.filter((item) => item.group !== row.group),
              )
            }
          />
        </Tooltip>
      ),
    },
  ];

  const routeColumns = [
    {
      title: t('聚合分组'),
      width: 190,
      render: (_, row) => (
        <Select
          value={row.aggregate_group}
          optionList={aggregateGroupOptions}
          filter={selectFilter}
          onChange={(value) => {
            const realGroup = routeGroupOptions(value)[0]?.value || '';
            updateRouteRow(row._row_id, {
              aggregate_group: value,
              real_group: realGroup,
              model_name: '',
            });
          }}
          style={{ width: '100%' }}
        />
      ),
    },
    {
      title: t('子分组'),
      width: 140,
      render: (_, row) => (
        <Select
          value={row.real_group}
          optionList={routeGroupOptions(row.aggregate_group)}
          onChange={(value) =>
            updateRouteRow(row._row_id, {
              real_group: value,
              model_name: '',
            })
          }
          style={{ width: '100%' }}
        />
      ),
    },
    {
      title: t('精确模型'),
      width: 210,
      render: (_, row) => {
        const key = modelCacheKey(row.aggregate_group, row.real_group);
        return (
          <AutoComplete
            value={row.model_name}
            data={modelsForRule(row)}
            onChange={(value) =>
              updateRouteRow(row._row_id, { model_name: value || '' })
            }
            onDropdownVisibleChange={(open) =>
              open && loadModels(row.aggregate_group, row.real_group)
            }
            loading={!!modelLoading[key]}
            showClear
            style={{ width: '100%' }}
          />
        );
      },
    },
    {
      title: t('用户级倍率'),
      width: 125,
      render: (_, row) => (
        <InputNumber
          min={0}
          precision={6}
          formatter={formatRatioInput}
          step={0.1}
          value={row.group_ratio}
          onChange={(value) =>
            updateRouteRow(row._row_id, { group_ratio: value })
          }
          style={{ width: '100%' }}
        />
      ),
    },
    {
      title: t('启用'),
      width: 70,
      render: (_, row) => (
        <Switch
          checked={row.enabled !== false}
          onChange={(checked) =>
            updateRouteRow(row._row_id, { enabled: checked })
          }
        />
      ),
    },
    {
      title: t('继承倍率'),
      width: 115,
      render: (_, row) => {
        const view = inheritedLabel(row);
        return (
          <Tooltip content={view.source}>
            <Tag color='white'>{formatRatio(view.inheritedRatio)}</Tag>
          </Tooltip>
        );
      },
    },
    {
      title: t('最终倍率'),
      width: 105,
      render: (_, row) => (
        <Tag color='green'>
          {formatRatio(inheritedLabel(row).effectiveRatio)}
        </Tag>
      ),
    },
    {
      title: t('操作'),
      width: 65,
      render: (_, row) => (
        <Tooltip content={t('删除倍率规则')}>
          <Button
            icon={<IconDelete />}
            type='danger'
            theme='borderless'
            aria-label={t('删除倍率规则')}
            onClick={() =>
              setRouteRows((prev) =>
                prev.filter((item) => item._row_id !== row._row_id),
              )
            }
          />
        </Tooltip>
      ),
    },
  ];

  const renderRouteFields = (rule, onChange, isDraft = false) => {
    const key = modelCacheKey(rule.aggregate_group, rule.real_group);
    return (
      <>
        <div className='min-w-0'>
          <Text size='small' type='secondary'>
            {t('聚合分组')}
          </Text>
          <Select
            className='mt-1'
            value={rule.aggregate_group}
            optionList={aggregateGroupOptions}
            filter={selectFilter}
            onChange={(value) => {
              const realGroup = routeGroupOptions(value)[0]?.value || '';
              onChange({
                aggregate_group: value,
                real_group: realGroup,
                model_name: '',
              });
            }}
            style={{ width: '100%' }}
          />
        </div>
        <div className='min-w-0'>
          <Text size='small' type='secondary'>
            {t('子分组')}
          </Text>
          <Select
            className='mt-1'
            value={rule.real_group}
            optionList={routeGroupOptions(rule.aggregate_group)}
            onChange={(value) =>
              onChange({ real_group: value, model_name: '' })
            }
            style={{ width: '100%' }}
          />
        </div>
        <div className='min-w-0'>
          <Text size='small' type='secondary'>
            {t('精确模型')}
          </Text>
          <AutoComplete
            className='mt-1'
            value={rule.model_name}
            data={modelsForRule(rule)}
            onChange={(value) => onChange({ model_name: value || '' })}
            onDropdownVisibleChange={(open) =>
              open && loadModels(rule.aggregate_group, rule.real_group)
            }
            loading={!!modelLoading[key]}
            placeholder={t('选择或输入精确模型名')}
            showClear
            style={{ width: '100%' }}
          />
        </div>
        <div>
          <Text size='small' type='secondary'>
            {t('用户级倍率')}
          </Text>
          <InputNumber
            className='mt-1'
            min={0}
            precision={6}
            formatter={formatRatioInput}
            step={0.1}
            value={rule.group_ratio}
            onChange={(value) => onChange({ group_ratio: value })}
            style={{ width: '100%' }}
          />
        </div>
        <div>
          <Text size='small' type='secondary'>
            {t('启用')}
          </Text>
          <div className='mt-2'>
            <Switch
              checked={rule.enabled !== false}
              onChange={(checked) => onChange({ enabled: checked })}
            />
          </div>
        </div>
        {!isDraft ? (
          <div className='flex items-end justify-between gap-3'>
            <div>
              <Text size='small' type='secondary'>
                {t('最终倍率')}
              </Text>
              <div className='mt-1'>
                <Tag color='green'>
                  {formatRatio(inheritedLabel(rule).effectiveRatio)}
                </Tag>
              </div>
            </div>
            <Tooltip content={t('删除倍率规则')}>
              <Button
                icon={<IconDelete />}
                type='danger'
                theme='borderless'
                aria-label={t('删除倍率规则')}
                onClick={() =>
                  setRouteRows((prev) =>
                    prev.filter((item) => item._row_id !== rule._row_id),
                  )
                }
              />
            </Tooltip>
          </div>
        ) : null}
      </>
    );
  };

  return (
    <SideSheet
      visible={visible}
      placement='right'
      width={isMobile ? '100%' : 1080}
      bodyStyle={{ padding: 0 }}
      onCancel={onCancel}
      title={
        <div className='flex items-center gap-2 min-w-0 flex-wrap'>
          <Tag color='blue' shape='circle'>
            {t('倍率')}
          </Tag>
          <Typography.Title heading={4} className='m-0'>
            {t('分组特殊倍率')}
          </Typography.Title>
          <Text type='tertiary'>
            {user?.username || '-'} ({t('ID')}: {user?.id || '-'})
          </Text>
        </div>
      }
      footer={
        <Space className='w-full justify-end'>
          <Button onClick={onCancel}>{t('取消')}</Button>
          <Button
            type='primary'
            theme='solid'
            loading={saving}
            onClick={saveOverrides}
          >
            {t('保存')}
          </Button>
        </Space>
      }
    >
      <Tabs
        type='button'
        activeKey={activeTab}
        onChange={setActiveTab}
        className='px-4 pt-3'
      >
        <TabPane tab={t('聚合分组倍率')} itemKey='aggregate'>
          <div className='py-4'>
            <div className='grid grid-cols-1 gap-3 md:grid-cols-[minmax(260px,1fr)_160px_auto] md:items-end mb-4'>
              <div className='min-w-0'>
                <Text size='small' type='secondary'>
                  {t('聚合分组')}
                </Text>
                <Select
                  className='mt-1'
                  placeholder={t('搜索聚合分组')}
                  value={selectedGroup}
                  onChange={setSelectedGroup}
                  optionList={defaultGroupOptions}
                  loading={loading}
                  filter={selectFilter}
                  style={{ width: '100%' }}
                  showClear
                />
              </div>
              <div>
                <Text size='small' type='secondary'>
                  {t('特殊倍率')}
                </Text>
                <InputNumber
                  className='mt-1'
                  min={0}
                  precision={6}
                  formatter={formatRatioInput}
                  step={0.1}
                  value={newRatio}
                  onChange={setNewRatio}
                  style={{ width: '100%' }}
                />
              </div>
              <Button
                type='primary'
                theme='light'
                icon={<IconPlusCircle />}
                onClick={addDefaultOverride}
              >
                {t('添加')}
              </Button>
            </div>
            <CardTable
              columns={defaultColumns}
              dataSource={defaultRows}
              rowKey='group'
              loading={loading}
              scroll={{ x: 640 }}
              hidePagination
              empty={emptyState(t('暂无分组特殊倍率'))}
              size='middle'
            />
          </div>
        </TabPane>
        <TabPane tab={t('子分组模型倍率')} itemKey='route-model'>
          <div className='py-4'>
            <div className='border border-solid border-gray-200 rounded-lg p-3 mb-4'>
              <div className='grid grid-cols-1 gap-3 md:grid-cols-[1fr_0.8fr_1.2fr_130px_70px_auto] md:items-end'>
                {renderRouteFields(
                  draftRule,
                  (patch) => setDraftRule((prev) => ({ ...prev, ...patch })),
                  true,
                )}
                <Button
                  type='primary'
                  theme='light'
                  icon={<IconPlusCircle />}
                  onClick={addRouteRule}
                >
                  {t('添加')}
                </Button>
              </div>
            </div>
            {isMobile ? (
              routeRows.length === 0 ? (
                emptyState(t('暂无子分组模型倍率规则'))
              ) : (
                <div className='flex flex-col gap-3'>
                  {routeRows.map((rule) => (
                    <div
                      key={rule._row_id}
                      className='border border-solid border-gray-200 rounded-lg p-3 grid grid-cols-1 gap-3'
                    >
                      {renderRouteFields(rule, (patch) =>
                        updateRouteRow(rule._row_id, patch),
                      )}
                    </div>
                  ))}
                </div>
              )
            ) : (
              <CardTable
                columns={routeColumns}
                dataSource={routeRows}
                rowKey='_row_id'
                loading={loading}
                scroll={{ x: 1120 }}
                hidePagination
                empty={emptyState(t('暂无子分组模型倍率规则'))}
                size='middle'
              />
            )}
          </div>
        </TabPane>
      </Tabs>
    </SideSheet>
  );
};

export default AggregateGroupRatioOverridesModal;
