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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import {
  Button,
  Col,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Select,
  Space,
  Spin,
  Switch,
  Table,
  Tabs,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconArrowDown,
  IconArrowUp,
  IconDelete,
  IconPlus,
  IconSave,
} from '@douyinfe/semi-icons';
import {
  compareObjects,
  API,
  selectFilter,
  showError,
  showSuccess,
  showWarning,
  verifyJSON,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';
import {
  buildGroupOptions,
  buildRouteGroupOptions,
  buildUserGroupOptions,
  DEFAULT_GROUP_RATIO_INPUTS,
  GROUP_RATIO_INPUT_KEYS,
  getGroupReferences,
  parseGroupRatioVisualConfig,
  removeGroupAndReferences,
  serializeGroupRatioVisualConfig,
  validateGroupRatioVisualConfig,
} from './groupRatioVisualConfig';

const { Text } = Typography;
const { TabPane } = Tabs;

const createRowId = (prefix) =>
  `${prefix}-${Date.now()}-${Math.random().toString(16).slice(2)}`;

const sectionStyle = {
  border: '1px solid var(--semi-color-border)',
  borderRadius: 8,
  padding: 18,
  background: 'var(--semi-color-fill-0)',
};

const groupedRulesGridStyle = {
  display: 'grid',
  gridTemplateColumns: 'repeat(3, minmax(0, 1fr))',
  gridAutoRows: 620,
  gap: 16,
  width: '100%',
  maxHeight: 1892,
  overflowY: 'auto',
  paddingRight: 4,
  alignItems: 'start',
};

const groupedRuleCardStyle = {
  minWidth: 0,
  border: '1px solid var(--semi-color-border)',
  borderRadius: 8,
  padding: 14,
  background: 'var(--semi-color-bg-0)',
  boxShadow: '0 1px 3px rgba(15, 23, 42, 0.08)',
  height: '100%',
  display: 'flex',
  flexDirection: 'column',
};

const groupedRuleTableViewportStyle = {
  flex: '1 1 auto',
  minHeight: 0,
  overflowY: 'auto',
  overflowX: 'auto',
  border: '1px solid var(--semi-color-border)',
  borderRadius: 6,
};

const normalizeInputs = (options) => {
  const currentInputs = { ...DEFAULT_GROUP_RATIO_INPUTS };
  GROUP_RATIO_INPUT_KEYS.forEach((key) => {
    if (Object.prototype.hasOwnProperty.call(options || {}, key)) {
      currentInputs[key] = options[key];
    }
  });
  return currentInputs;
};

const validateAutoGroupsJSON = (value) => {
  if (!value || value.trim() === '') return true;
  try {
    const parsed = JSON.parse(value);
    return (
      Array.isArray(parsed) && parsed.every((item) => typeof item === 'string')
    );
  } catch (error) {
    return false;
  }
};

const groupRowsByUserGroup = (rows, emptyLabel) => {
  const groupMap = new Map();
  rows.forEach((row) => {
    const key = row.userGroup || emptyLabel;
    if (!groupMap.has(key)) {
      groupMap.set(key, []);
    }
    groupMap.get(key).push(row);
  });
  return Array.from(groupMap.entries()).map(([userGroup, children]) => ({
    userGroup: userGroup === emptyLabel ? '' : userGroup,
    children,
  }));
};

export default function GroupRatioSettings(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('visual');
  const [inputs, setInputs] = useState(DEFAULT_GROUP_RATIO_INPUTS);
  const [inputsRow, setInputsRow] = useState(DEFAULT_GROUP_RATIO_INPUTS);
  const [visualConfig, setVisualConfig] = useState(() =>
    parseGroupRatioVisualConfig(DEFAULT_GROUP_RATIO_INPUTS),
  );
  const refForm = useRef();

  const groupOptions = useMemo(
    () => buildGroupOptions(visualConfig),
    [visualConfig],
  );
  const userGroupOptions = useMemo(
    () => buildUserGroupOptions(visualConfig),
    [visualConfig],
  );
  const routeGroupOptions = useMemo(
    () => buildRouteGroupOptions(visualConfig),
    [visualConfig],
  );
  const specialRatioGroups = useMemo(
    () => groupRowsByUserGroup(visualConfig.specialRatios, '__empty__'),
    [visualConfig.specialRatios],
  );
  const specialUsableGroups = useMemo(
    () => groupRowsByUserGroup(visualConfig.specialUsableGroups, '__empty__'),
    [visualConfig.specialUsableGroups],
  );

  const setInputsAndForm = (nextInputs) => {
    setInputs(nextInputs);
    refForm.current?.setValues?.(nextInputs);
  };

  const updateVisualConfig = (nextConfig) => {
    setVisualConfig(nextConfig);
    setInputsAndForm({
      ...inputs,
      ...serializeGroupRatioVisualConfig(nextConfig),
    });
  };

  const updateVisualWith = (updater) => {
    updateVisualConfig(
      typeof updater === 'function' ? updater(visualConfig) : updater,
    );
  };

  const updateInput = (key, value) => {
    setInputsAndForm({ ...inputs, [key]: value });
  };

  const validateBeforeSave = () => {
    try {
      if (activeTab === 'visual') {
        const visualValidation = validateGroupRatioVisualConfig(visualConfig);
        if (!visualValidation.valid) return visualValidation;
      }
      return { valid: true, message: '' };
    } catch (error) {
      return { valid: false, message: error.message || t('请检查输入') };
    }
  };

  async function onSubmit() {
    try {
      const validation = validateBeforeSave();
      if (!validation.valid) {
        showError(t(validation.message));
        return;
      }

      if (refForm.current?.validate) {
        await refForm.current.validate();
      }

      const updateArray = compareObjects(inputs, inputsRow);
      if (!updateArray.length) {
        showWarning(t('你似乎并没有修改什么'));
        return;
      }

      const requestQueue = updateArray.map((item) => {
        const value =
          typeof inputs[item.key] === 'boolean'
            ? String(inputs[item.key])
            : inputs[item.key];
        return API.put('/api/option/', { key: item.key, value });
      });

      setLoading(true);
      Promise.all(requestQueue)
        .then((res) => {
          if (res.includes(undefined)) {
            return showError(
              requestQueue.length > 1
                ? t('部分保存失败，请重试')
                : t('保存失败'),
            );
          }

          for (let i = 0; i < res.length; i++) {
            if (!res[i].data.success) {
              return showError(res[i].data.message);
            }
          }

          showSuccess(t('保存成功'));
          props.refresh();
        })
        .catch((error) => {
          console.error('Unexpected error:', error);
          showError(t('保存失败，请重试'));
        })
        .finally(() => {
          setLoading(false);
        });
    } catch (error) {
      showError(t('请检查输入'));
      console.error(error);
    }
  }

  useEffect(() => {
    const currentInputs = normalizeInputs(props.options);
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    refForm.current?.setValues?.(currentInputs);
    try {
      setVisualConfig(parseGroupRatioVisualConfig(currentInputs));
    } catch (error) {
      console.error(error);
    }
  }, [props.options]);

  const handleTabChange = (key) => {
    if (key === 'visual') {
      try {
        setVisualConfig(parseGroupRatioVisualConfig(inputs));
      } catch (error) {
        showError(t(error.message || '不是合法的 JSON 字符串'));
        return;
      }
    }
    setActiveTab(key);
  };

  const addBaseGroup = () => {
    updateVisualWith((current) => ({
      ...current,
      baseGroups: [
        ...current.baseGroups,
        {
          id: createRowId('base'),
          groupName: '',
          ratio: 1,
          userSelectable: true,
          description: '',
        },
      ],
    }));
  };

  const updateBaseGroup = (id, patch) => {
    updateVisualWith((current) => ({
      ...current,
      baseGroups: current.baseGroups.map((group) =>
        group.id === id ? { ...group, ...patch } : group,
      ),
    }));
  };

  const deleteBaseGroup = (record) => {
    const references = getGroupReferences(visualConfig, record.groupName);
    const applyDelete = () =>
      updateVisualConfig(
        removeGroupAndReferences(visualConfig, record.groupName),
      );

    if (references.length === 0) {
      applyDelete();
      return;
    }

    Modal.confirm({
      title: t('确认删除该分组？'),
      content: (
        <div>
          <div>{t('该分组正在被以下配置引用，删除时会同步移除这些引用：')}</div>
          <div style={{ marginTop: 8 }}>
            {references.map((reference) => (
              <Tag key={reference} color='orange' style={{ marginBottom: 4 }}>
                {t(reference)}
              </Tag>
            ))}
          </div>
        </div>
      ),
      okText: t('确认删除'),
      cancelText: t('取消'),
      onOk: applyDelete,
    });
  };

  const addSpecialRatioGroup = () => {
    updateVisualWith((current) => ({
      ...current,
      specialRatios: [
        ...current.specialRatios,
        {
          id: createRowId('special-ratio'),
          userGroup: '',
          usingGroup: '',
          ratio: 1,
        },
      ],
    }));
  };

  const addSpecialRatioValue = (userGroup) => {
    updateVisualWith((current) => ({
      ...current,
      specialRatios: [
        ...current.specialRatios,
        {
          id: createRowId('special-ratio'),
          userGroup,
          usingGroup: '',
          ratio: 1,
        },
      ],
    }));
  };

  const updateSpecialRatioGroup = (oldUserGroup, nextUserGroup) => {
    updateVisualWith((current) => ({
      ...current,
      specialRatios: current.specialRatios.map((row) =>
        (row.userGroup || '') === (oldUserGroup || '')
          ? { ...row, userGroup: nextUserGroup || '' }
          : row,
      ),
    }));
  };

  const deleteSpecialRatioGroup = (userGroup) => {
    updateVisualWith((current) => ({
      ...current,
      specialRatios: current.specialRatios.filter(
        (row) => (row.userGroup || '') !== (userGroup || ''),
      ),
    }));
  };

  const updateSpecialRatio = (id, patch) => {
    updateVisualWith((current) => ({
      ...current,
      specialRatios: current.specialRatios.map((row) =>
        row.id === id ? { ...row, ...patch } : row,
      ),
    }));
  };

  const deleteSpecialRatio = (id) => {
    updateVisualWith((current) => ({
      ...current,
      specialRatios: current.specialRatios.filter((row) => row.id !== id),
    }));
  };

  const addSpecialUsableGroup = () => {
    updateVisualWith((current) => ({
      ...current,
      specialUsableGroups: [
        ...current.specialUsableGroups,
        {
          id: createRowId('special-usable'),
          userGroup: '',
          action: 'add',
          targetGroup: '',
          description: '',
        },
      ],
    }));
  };

  const addSpecialUsableValue = (userGroup) => {
    updateVisualWith((current) => ({
      ...current,
      specialUsableGroups: [
        ...current.specialUsableGroups,
        {
          id: createRowId('special-usable'),
          userGroup,
          action: 'add',
          targetGroup: '',
          description: '',
        },
      ],
    }));
  };

  const updateSpecialUsableGroupKey = (oldUserGroup, nextUserGroup) => {
    updateVisualWith((current) => ({
      ...current,
      specialUsableGroups: current.specialUsableGroups.map((row) =>
        (row.userGroup || '') === (oldUserGroup || '')
          ? { ...row, userGroup: nextUserGroup || '' }
          : row,
      ),
    }));
  };

  const deleteSpecialUsableGroupKey = (userGroup) => {
    updateVisualWith((current) => ({
      ...current,
      specialUsableGroups: current.specialUsableGroups.filter(
        (row) => (row.userGroup || '') !== (userGroup || ''),
      ),
    }));
  };

  const updateSpecialUsableGroup = (id, patch) => {
    updateVisualWith((current) => ({
      ...current,
      specialUsableGroups: current.specialUsableGroups.map((row) =>
        row.id === id ? { ...row, ...patch } : row,
      ),
    }));
  };

  const deleteSpecialUsableGroup = (id) => {
    updateVisualWith((current) => ({
      ...current,
      specialUsableGroups: current.specialUsableGroups.filter(
        (row) => row.id !== id,
      ),
    }));
  };

  const moveAutoGroup = (index, direction) => {
    updateVisualWith((current) => {
      const nextIndex = index + direction;
      if (nextIndex < 0 || nextIndex >= current.autoGroups.length) {
        return current;
      }
      const nextAutoGroups = [...current.autoGroups];
      const [item] = nextAutoGroups.splice(index, 1);
      nextAutoGroups.splice(nextIndex, 0, item);
      return { ...current, autoGroups: nextAutoGroups };
    });
  };

  const removeAutoGroup = (groupName) => {
    updateVisualWith((current) => ({
      ...current,
      autoGroups: current.autoGroups.filter((group) => group !== groupName),
    }));
  };

  const renderSection = (title, description, action, children) => (
    <div style={sectionStyle}>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          gap: 12,
          alignItems: 'flex-start',
          marginBottom: 12,
          flexWrap: 'wrap',
        }}
      >
        <div>
          <Text strong>{t(title)}</Text>
          {description ? (
            <div style={{ color: 'var(--semi-color-text-2)', marginTop: 4 }}>
              {t(description)}
            </div>
          ) : null}
        </div>
        {action}
      </div>
      {children}
    </div>
  );

  const groupSelect = (value, onChange, placeholder, optionList) => (
    <Select
      size='small'
      value={value || undefined}
      placeholder={t(placeholder)}
      optionList={optionList}
      filter={selectFilter}
      searchPosition='dropdown'
      showClear
      style={{ width: '100%' }}
      onChange={(nextValue) => onChange(nextValue || '')}
    />
  );

  const baseGroupColumns = [
    {
      title: t('分组名称'),
      dataIndex: 'groupName',
      width: 180,
      render: (_, record) => (
        <Input
          value={record.groupName}
          placeholder={t('例如 vip')}
          onChange={(value) => updateBaseGroup(record.id, { groupName: value })}
        />
      ),
    },
    {
      title: t('倍率'),
      dataIndex: 'ratio',
      width: 130,
      render: (_, record) => (
        <InputNumber
          min={0}
          value={record.ratio}
          onChange={(value) => updateBaseGroup(record.id, { ratio: value })}
          style={{ width: '100%' }}
        />
      ),
    },
    {
      title: t('用户可选'),
      dataIndex: 'userSelectable',
      width: 120,
      render: (_, record) => (
        <Switch
          checked={record.userSelectable}
          onChange={(value) =>
            updateBaseGroup(record.id, { userSelectable: value })
          }
        />
      ),
    },
    {
      title: t('分组描述'),
      dataIndex: 'description',
      render: (_, record) => (
        <Input
          value={record.description}
          placeholder={t('展示给用户的分组描述')}
          disabled={!record.userSelectable}
          onChange={(value) =>
            updateBaseGroup(record.id, { description: value })
          }
        />
      ),
    },
    {
      title: t('操作'),
      width: 96,
      render: (_, record) => (
        <Button
          type='danger'
          theme='borderless'
          icon={<IconDelete />}
          onClick={() => deleteBaseGroup(record)}
          aria-label={t('删除分组')}
        />
      ),
    },
  ];

  const autoGroupColumns = [
    {
      title: t('顺序'),
      width: 90,
      render: (_, record, index) => <Tag color='blue'>{index + 1}</Tag>,
    },
    {
      title: t('分组'),
      dataIndex: 'group',
      render: (group) => <Text strong>{group}</Text>,
    },
    {
      title: t('操作'),
      width: 180,
      render: (_, record, index) => (
        <Space>
          <Button
            theme='borderless'
            icon={<IconArrowUp />}
            disabled={index === 0}
            onClick={() => moveAutoGroup(index, -1)}
            aria-label={t('上移分组')}
          />
          <Button
            theme='borderless'
            icon={<IconArrowDown />}
            disabled={index === visualConfig.autoGroups.length - 1}
            onClick={() => moveAutoGroup(index, 1)}
            aria-label={t('下移分组')}
          />
          <Button
            type='danger'
            theme='borderless'
            icon={<IconDelete />}
            onClick={() => removeAutoGroup(record.group)}
            aria-label={t('移除分组')}
          />
        </Space>
      ),
    },
  ];

  const specialRatioColumns = [
    {
      title: t('使用分组'),
      dataIndex: 'usingGroup',
      render: (_, record) =>
        groupSelect(
          record.usingGroup,
          (value) => updateSpecialRatio(record.id, { usingGroup: value }),
          '选择使用分组',
          routeGroupOptions,
        ),
    },
    {
      title: t('特殊倍率'),
      dataIndex: 'ratio',
      width: 140,
      render: (_, record) => (
        <InputNumber
          size='small'
          min={0}
          value={record.ratio}
          onChange={(value) => updateSpecialRatio(record.id, { ratio: value })}
          style={{ width: '100%' }}
        />
      ),
    },
    {
      title: t('操作'),
      width: 96,
      render: (_, record) => (
        <Button
          size='small'
          type='danger'
          theme='borderless'
          icon={<IconDelete />}
          onClick={() => deleteSpecialRatio(record.id)}
          aria-label={t('删除特殊倍率')}
        />
      ),
    },
  ];

  const specialUsableColumns = [
    {
      title: t('操作类型'),
      dataIndex: 'action',
      width: 140,
      render: (_, record) => (
        <Select
          size='small'
          value={record.action}
          optionList={[
            { label: t('添加'), value: 'add' },
            { label: t('移除'), value: 'remove' },
          ]}
          style={{ width: '100%' }}
          onChange={(value) =>
            updateSpecialUsableGroup(record.id, { action: value || 'add' })
          }
        />
      ),
    },
    {
      title: t('目标分组'),
      dataIndex: 'targetGroup',
      render: (_, record) =>
        groupSelect(
          record.targetGroup,
          (value) =>
            updateSpecialUsableGroup(record.id, { targetGroup: value }),
          '选择目标分组',
          routeGroupOptions,
        ),
    },
    {
      title: t('描述'),
      dataIndex: 'description',
      render: (_, record) => (
        <Input
          size='small'
          value={record.description}
          placeholder={
            record.action === 'remove'
              ? t('可选，便于识别移除原因')
              : t('添加给用户的分组描述')
          }
          onChange={(value) =>
            updateSpecialUsableGroup(record.id, { description: value })
          }
        />
      ),
    },
    {
      title: t('操作'),
      width: 96,
      render: (_, record) => (
        <Button
          size='small'
          type='danger'
          theme='borderless'
          icon={<IconDelete />}
          onClick={() => deleteSpecialUsableGroup(record.id)}
          aria-label={t('删除特殊可用分组规则')}
        />
      ),
    },
  ];

  const autoGroupTableData = visualConfig.autoGroups.map((group) => ({
    group,
  }));

  const renderGroupedRules = ({
    groups,
    emptyText,
    addGroupText,
    onAddGroup,
    onUpdateGroup,
    onDeleteGroup,
    onAddValue,
    tableColumns,
    tableEmptyText,
    addValueText,
  }) => (
    <Space vertical align='start' style={{ width: '100%', gap: 12 }}>
      <Button icon={<IconPlus />} onClick={onAddGroup}>
        {t(addGroupText)}
      </Button>
      {groups.length === 0 ? (
        <div style={{ color: 'var(--semi-color-text-2)' }}>{t(emptyText)}</div>
      ) : (
        <div style={groupedRulesGridStyle}>
          {groups.map((group, index) => (
            <div
              key={`${group.userGroup || 'empty'}-${index}`}
              style={groupedRuleCardStyle}
            >
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  gap: 10,
                  alignItems: 'flex-start',
                  marginBottom: 12,
                }}
              >
                <div style={{ minWidth: 0, flex: '1 1 auto' }}>
                  <Text strong>{t('用户分组 key')}</Text>
                  <Select
                    size='small'
                    value={group.userGroup || undefined}
                    placeholder={t('选择用户分组 key')}
                    optionList={userGroupOptions}
                    filter={selectFilter}
                    searchPosition='dropdown'
                    showClear
                    style={{ width: '100%', marginTop: 8 }}
                    onChange={(value) =>
                      onUpdateGroup(group.userGroup, value || '')
                    }
                  />
                  <div
                    style={{
                      color: 'var(--semi-color-text-2)',
                      fontSize: 12,
                      marginTop: 6,
                      lineHeight: 1.5,
                    }}
                  >
                    {t('这里只能选择 default 或 UserGroup-* 用户分组。')}
                  </div>
                </div>
                <Space spacing={4} style={{ flex: '0 0 auto', paddingTop: 20 }}>
                  <Button
                    size='small'
                    icon={<IconPlus />}
                    onClick={() => onAddValue(group.userGroup)}
                    aria-label={t(addValueText)}
                  />
                  <Button
                    size='small'
                    type='danger'
                    theme='borderless'
                    icon={<IconDelete />}
                    onClick={() => onDeleteGroup(group.userGroup)}
                    aria-label={t('删除用户分组规则')}
                  />
                </Space>
              </div>
              <div
                style={{
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  marginBottom: 8,
                }}
              >
                <Text type='secondary' size='small'>
                  {t('规则明细')} ({group.children.length})
                </Text>
                <Button
                  size='small'
                  theme='borderless'
                  icon={<IconPlus />}
                  onClick={() => onAddValue(group.userGroup)}
                >
                  {t(addValueText)}
                </Button>
              </div>
              <div style={groupedRuleTableViewportStyle}>
                <Table
                  columns={tableColumns}
                  dataSource={group.children}
                  rowKey='id'
                  pagination={false}
                  empty={t(tableEmptyText)}
                  scroll={{ x: 'max-content' }}
                />
              </div>
            </div>
          ))}
        </div>
      )}
    </Space>
  );

  return (
    <Spin spinning={loading}>
      <Tabs activeKey={activeTab} onChange={handleTabChange} type='button'>
        <TabPane tab={t('可视化配置')} itemKey='visual'>
          <Space vertical align='start' style={{ width: '100%', gap: 16 }}>
            {renderSection(
              '基础分组',
              '维护分组倍率和用户新建令牌时可选的分组描述。',
              <Button icon={<IconPlus />} onClick={addBaseGroup}>
                {t('新增分组')}
              </Button>,
              <div style={{ width: '100%', overflowX: 'auto' }}>
                <Table
                  columns={baseGroupColumns}
                  dataSource={visualConfig.baseGroups}
                  rowKey='id'
                  pagination={false}
                  empty={t('暂无分组')}
                />
              </div>,
            )}

            {renderSection(
              '分组特殊倍率',
              '先选择 default 或 UserGroup-* 用户分组 key，再为它添加多个真实路由分组倍率。',
              null,
              renderGroupedRules({
                groups: specialRatioGroups,
                emptyText: '暂无特殊倍率',
                addGroupText: '新增用户分组 key',
                onAddGroup: addSpecialRatioGroup,
                onUpdateGroup: updateSpecialRatioGroup,
                onDeleteGroup: deleteSpecialRatioGroup,
                onAddValue: addSpecialRatioValue,
                tableColumns: specialRatioColumns,
                tableEmptyText: '暂无特殊倍率',
                addValueText: '新增使用分组',
              }),
            )}

            {renderSection(
              '分组特殊可用分组',
              '先选择 default 或 UserGroup-* 用户分组 key，再为它添加或移除多个真实路由分组。',
              null,
              renderGroupedRules({
                groups: specialUsableGroups,
                emptyText: '暂无特殊可用分组规则',
                addGroupText: '新增用户分组 key',
                onAddGroup: addSpecialUsableGroup,
                onUpdateGroup: updateSpecialUsableGroupKey,
                onDeleteGroup: deleteSpecialUsableGroupKey,
                onAddValue: addSpecialUsableValue,
                tableColumns: specialUsableColumns,
                tableEmptyText: '暂无特殊可用分组规则',
                addValueText: '新增目标分组',
              }),
            )}

            {renderSection(
              '自动分组auto',
              'auto 会按列表顺序尝试分组，可用上移和下移调整优先级。',
              null,
              <>
                <Row gutter={16}>
                  <Col xs={24} sm={16}>
                    <Text strong>{t('auto 分组列表')}</Text>
                    <Select
                      value={visualConfig.autoGroups}
                      optionList={routeGroupOptions}
                      multiple
                      filter={selectFilter}
                      searchPosition='dropdown'
                      autoClearSearchValue={false}
                      placeholder={t('选择加入 auto 的真实路由分组')}
                      style={{ width: '100%', marginTop: 8 }}
                      onChange={(value) =>
                        updateVisualWith((current) => ({
                          ...current,
                          autoGroups: value || [],
                        }))
                      }
                    />
                  </Col>
                  <Col xs={24} sm={8}>
                    <Text strong>{t('默认使用 auto')}</Text>
                    <div style={{ marginTop: 12 }}>
                      <Switch
                        checked={visualConfig.defaultUseAutoGroup}
                        onChange={(value) =>
                          updateVisualWith((current) => ({
                            ...current,
                            defaultUseAutoGroup: value,
                          }))
                        }
                      />
                    </div>
                    <div
                      style={{
                        color: 'var(--semi-color-text-2)',
                        marginTop: 8,
                      }}
                    >
                      {t(
                        '创建令牌默认选择auto分组，初始令牌也将设为auto（否则留空，为用户默认分组）',
                      )}
                    </div>
                  </Col>
                </Row>
                <div style={{ marginTop: 12, overflowX: 'auto' }}>
                  <Table
                    columns={autoGroupColumns}
                    dataSource={autoGroupTableData}
                    rowKey='group'
                    pagination={false}
                    empty={t('暂无 auto 分组')}
                  />
                </div>
              </>,
            )}
          </Space>
        </TabPane>

        <TabPane tab={t('JSON 原始配置')} itemKey='json'>
          <Form
            values={inputs}
            getFormApi={(formAPI) => (refForm.current = formAPI)}
            style={{ marginBottom: 15 }}
          >
            <Row gutter={16}>
              <Col xs={24} sm={16}>
                <Form.TextArea
                  label={t('分组倍率')}
                  placeholder={t('为一个 JSON 文本，键为分组名称，值为倍率')}
                  extraText={t(
                    '分组倍率设置，可以在此处新增分组或修改现有分组的倍率，格式为 JSON 字符串，例如：{"vip": 0.5, "test": 1}，表示 vip 分组的倍率为 0.5，test 分组的倍率为 1',
                  )}
                  field={'GroupRatio'}
                  autosize={{ minRows: 6, maxRows: 12 }}
                  trigger='blur'
                  stopValidateWithError
                  rules={[
                    {
                      validator: (rule, value) => verifyJSON(value),
                      message: t('不是合法的 JSON 字符串'),
                    },
                  ]}
                  onChange={(value) => updateInput('GroupRatio', value)}
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={16}>
                <Form.TextArea
                  label={t('用户可选分组')}
                  placeholder={t(
                    '为一个 JSON 文本，键为分组名称，值为分组描述',
                  )}
                  extraText={t(
                    '用户新建令牌时可选的分组，格式为 JSON 字符串，例如：{"vip": "VIP 用户", "test": "测试"}，表示用户可以选择 vip 分组和 test 分组',
                  )}
                  field={'UserUsableGroups'}
                  autosize={{ minRows: 6, maxRows: 12 }}
                  trigger='blur'
                  stopValidateWithError
                  rules={[
                    {
                      validator: (rule, value) => verifyJSON(value),
                      message: t('不是合法的 JSON 字符串'),
                    },
                  ]}
                  onChange={(value) => updateInput('UserUsableGroups', value)}
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={16}>
                <Form.TextArea
                  label={t('分组特殊倍率')}
                  placeholder={t('为一个 JSON 文本')}
                  extraText={t(
                    '键为分组名称，值为另一个 JSON 对象，键为分组名称，值为该分组的用户的特殊分组倍率，例如：{"vip": {"default": 0.5, "test": 1}}，表示 vip 分组的用户在使用default分组的令牌时倍率为0.5，使用test分组时倍率为1',
                  )}
                  field={'GroupGroupRatio'}
                  autosize={{ minRows: 6, maxRows: 12 }}
                  trigger='blur'
                  stopValidateWithError
                  rules={[
                    {
                      validator: (rule, value) => verifyJSON(value),
                      message: t('不是合法的 JSON 字符串'),
                    },
                  ]}
                  onChange={(value) => updateInput('GroupGroupRatio', value)}
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={16}>
                <Form.TextArea
                  label={t('分组特殊可用分组')}
                  placeholder={t('为一个 JSON 文本')}
                  extraText={t(
                    '键为用户分组名称，值为操作映射对象。内层键以"+:"开头表示添加指定分组（键值为分组名称，值为描述），以"-:"开头表示移除指定分组（键值为分组名称），不带前缀的键直接添加该分组。例如：{"vip": {"+:premium": "高级分组", "special": "特殊分组", "-:default": "默认分组"}}，表示 vip 分组的用户可以使用 premium 和 special 分组，同时移除 default 分组的访问权限',
                  )}
                  field={'group_ratio_setting.group_special_usable_group'}
                  autosize={{ minRows: 6, maxRows: 12 }}
                  trigger='blur'
                  stopValidateWithError
                  rules={[
                    {
                      validator: (rule, value) => verifyJSON(value),
                      message: t('不是合法的 JSON 字符串'),
                    },
                  ]}
                  onChange={(value) =>
                    updateInput(
                      'group_ratio_setting.group_special_usable_group',
                      value,
                    )
                  }
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col xs={24} sm={16}>
                <Form.TextArea
                  label={t('自动分组auto，从第一个开始选择')}
                  placeholder={t('为一个 JSON 文本')}
                  field={'AutoGroups'}
                  autosize={{ minRows: 6, maxRows: 12 }}
                  trigger='blur'
                  stopValidateWithError
                  rules={[
                    {
                      validator: (rule, value) => validateAutoGroupsJSON(value),
                      message: t(
                        '必须是有效的 JSON 字符串数组，例如：["g1","g2"]',
                      ),
                    },
                  ]}
                  onChange={(value) => updateInput('AutoGroups', value)}
                />
              </Col>
            </Row>
            <Row gutter={16}>
              <Col span={16}>
                <Form.Switch
                  label={t(
                    '创建令牌默认选择auto分组，初始令牌也将设为auto（否则留空，为用户默认分组）',
                  )}
                  field={'DefaultUseAutoGroup'}
                  onChange={(value) =>
                    updateInput('DefaultUseAutoGroup', value)
                  }
                />
              </Col>
            </Row>
          </Form>
        </TabPane>
      </Tabs>

      <Button type='primary' icon={<IconSave />} onClick={onSubmit}>
        {t('保存分组相关设置')}
      </Button>
    </Spin>
  );
}
