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
  Button,
  Empty,
  InputNumber,
  Select,
  SideSheet,
  Space,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import { IconDelete, IconPlusCircle } from '@douyinfe/semi-icons';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  API,
  selectFilter,
  showError,
  showSuccess,
} from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';
import CardTable from '../../../common/ui/CardTable';

const { Text } = Typography;

const formatRatio = (value) => {
  if (value === undefined || value === null || value === '') {
    return '-';
  }
  const numberValue = Number(value);
  if (!Number.isFinite(numberValue)) {
    return '-';
  }
  return `${Number(numberValue.toFixed(6))}x`;
};

const formatRatioInput = (value) => {
  if (value === undefined || value === null || value === '') {
    return '';
  }
  const stringValue = String(value);
  if (!/^-?\d+(?:\.\d{6})$/.test(stringValue)) {
    return stringValue;
  }
  return stringValue.replace(/(\.\d*?[1-9])0+$/, '$1').replace(/\.0+$/, '');
};

const normalizeOverrides = (overrides = {}) =>
  Object.entries(overrides || {})
    .filter(([group, ratio]) => group && Number(ratio) >= 0)
    .map(([group, ratio]) => ({
      key: group,
      group,
      ratio: Number(ratio),
    }));

const AggregateGroupRatioOverridesModal = ({
  visible,
  onCancel,
  user,
  t,
  onSuccess,
}) => {
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [groupsLoading, setGroupsLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [aggregateGroups, setAggregateGroups] = useState([]);
  const [rows, setRows] = useState([]);
  const [selectedGroup, setSelectedGroup] = useState(undefined);
  const [newRatio, setNewRatio] = useState(1);

  const groupMap = useMemo(() => {
    const map = new Map();
    (aggregateGroups || []).forEach((group) => {
      if (group?.name) {
        map.set(group.name, group);
      }
    });
    return map;
  }, [aggregateGroups]);

  const selectedGroups = useMemo(
    () => new Set(rows.map((row) => row.group)),
    [rows],
  );

  const groupOptions = useMemo(
    () =>
      (aggregateGroups || [])
        .filter((group) => Number(group?.status) === 1)
        .map((group) => {
          const displayName = group.display_name || group.name;
          const desc = group.description ? ` · ${group.description}` : '';
          return {
            label: `${displayName} (${group.name}) ${formatRatio(group.group_ratio)}${desc}`,
            value: group.name,
            disabled: selectedGroups.has(group.name),
            group,
          };
        }),
    [aggregateGroups, selectedGroups],
  );

  const loadAggregateGroups = async () => {
    setGroupsLoading(true);
    try {
      const res = await API.get('/api/aggregate_group/');
      if (res.data?.success) {
        setAggregateGroups(res.data.data || []);
      } else {
        showError(res.data?.message || t('加载失败'));
      }
    } catch (error) {
      showError(error.response?.data?.message || error.message);
    } finally {
      setGroupsLoading(false);
    }
  };

  const loadOverrides = async () => {
    if (!user?.id) {
      setRows([]);
      return;
    }
    setLoading(true);
    try {
      const res = await API.get(
        `/api/user/${user.id}/aggregate_group_ratio_overrides`,
      );
      if (res.data?.success) {
        setRows(normalizeOverrides(res.data.data?.overrides));
      } else {
        showError(res.data?.message || t('加载失败'));
      }
    } catch (error) {
      showError(error.response?.data?.message || error.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible) {
      return;
    }
    setSelectedGroup(undefined);
    setNewRatio(1);
    loadAggregateGroups();
    loadOverrides();
  }, [visible, user?.id]);

  const addOverride = () => {
    if (!selectedGroup) {
      showError(t('请选择聚合分组'));
      return;
    }
    const ratio = Number(newRatio);
    if (!Number.isFinite(ratio) || ratio < 0) {
      showError(t('特殊倍率不能小于 0'));
      return;
    }
    if (selectedGroups.has(selectedGroup)) {
      showError(t('该聚合分组已配置'));
      return;
    }
    setRows((prev) => [
      ...prev,
      {
        key: selectedGroup,
        group: selectedGroup,
        ratio,
      },
    ]);
    setSelectedGroup(undefined);
    setNewRatio(1);
  };

  const updateRowRatio = (group, value) => {
    const ratio = Number(value);
    setRows((prev) =>
      prev.map((row) =>
        row.group === group
          ? {
              ...row,
              ratio: Number.isFinite(ratio) && ratio >= 0 ? ratio : 0,
            }
          : row,
      ),
    );
  };

  const removeRow = (group) => {
    setRows((prev) => prev.filter((row) => row.group !== group));
  };

  const saveOverrides = async () => {
    if (!user?.id) {
      showError(t('用户信息缺失'));
      return;
    }
    const overrides = {};
    for (const row of rows) {
      const ratio = Number(row.ratio);
      if (!row.group || !Number.isFinite(ratio) || ratio < 0) {
        showError(t('特殊倍率不能小于 0'));
        return;
      }
      overrides[row.group] = ratio;
    }
    setSaving(true);
    try {
      const res = await API.put(
        `/api/user/${user.id}/aggregate_group_ratio_overrides`,
        { overrides },
      );
      if (res.data?.success) {
        setRows(normalizeOverrides(res.data.data?.overrides));
        showSuccess(t('保存成功'));
        onSuccess?.();
      } else {
        showError(res.data?.message || t('保存失败'));
      }
    } catch (error) {
      showError(error.response?.data?.message || error.message);
    } finally {
      setSaving(false);
    }
  };

  const columns = useMemo(
    () => [
      {
        title: t('聚合分组'),
        dataIndex: 'group',
        key: 'group',
        render: (groupName) => {
          const group = groupMap.get(groupName);
          return (
            <div className='min-w-0'>
              <Space spacing={4} wrap>
                <Text strong>{group?.display_name || groupName}</Text>
                <Tag color='blue' size='small' shape='circle'>
                  {groupName}
                </Tag>
              </Space>
              {group?.description ? (
                <div className='text-xs text-gray-500 mt-1 truncate'>
                  {group.description}
                </div>
              ) : null}
            </div>
          );
        },
      },
      {
        title: t('原倍率'),
        key: 'original_ratio',
        width: 110,
        render: (_, record) => (
          <Tag color='white' shape='circle'>
            {formatRatio(groupMap.get(record.group)?.group_ratio)}
          </Tag>
        ),
      },
      {
        title: t('特殊倍率'),
        dataIndex: 'ratio',
        key: 'ratio',
        width: 180,
        render: (ratio, record) => (
          <InputNumber
            min={0}
            precision={6}
            formatter={formatRatioInput}
            step={0.1}
            value={ratio}
            onChange={(value) => updateRowRatio(record.group, value)}
            style={{ width: '100%' }}
          />
        ),
      },
      {
        title: t('操作'),
        key: 'operate',
        width: 80,
        render: (_, record) => (
          <Tooltip content={t('删除')}>
            <Button
              icon={<IconDelete />}
              type='danger'
              theme='borderless'
              size='small'
              aria-label={t('删除')}
              onClick={() => removeRow(record.group)}
            />
          </Tooltip>
        ),
      },
    ],
    [groupMap, t],
  );

  return (
    <SideSheet
      visible={visible}
      placement='right'
      width={isMobile ? '100%' : 840}
      bodyStyle={{ padding: 0 }}
      onCancel={onCancel}
      title={
        <Space>
          <Tag color='blue' shape='circle'>
            {t('倍率')}
          </Tag>
          <Typography.Title heading={4} className='m-0'>
            {t('分组特殊倍率')}
          </Typography.Title>
          <Text type='tertiary' className='ml-2'>
            {user?.username || '-'} (ID: {user?.id || '-'})
          </Text>
        </Space>
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
      <div className='p-4'>
        <div className='flex flex-col gap-2 md:flex-row md:items-end mb-4'>
          <div className='flex-1 min-w-0'>
            <Text size='small' type='secondary'>
              {t('聚合分组')}
            </Text>
            <Select
              className='mt-1'
              placeholder={t('搜索聚合分组')}
              value={selectedGroup}
              onChange={setSelectedGroup}
              optionList={groupOptions}
              loading={groupsLoading}
              filter={selectFilter}
              searchPosition='dropdown'
              style={{ width: '100%' }}
              showClear
              emptyContent={t('暂无可添加聚合分组')}
            />
          </div>
          <div className='md:w-40'>
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
              onChange={(value) => setNewRatio(value)}
              style={{ width: '100%' }}
            />
          </div>
          <Button
            type='primary'
            theme='light'
            icon={<IconPlusCircle />}
            onClick={addOverride}
          >
            {t('添加')}
          </Button>
        </div>

        <div style={{ maxHeight: isMobile ? 'calc(100vh - 260px)' : 560, overflowY: 'auto' }}>
          <CardTable
            columns={columns}
            dataSource={rows}
            rowKey='group'
            loading={loading}
            scroll={{ x: 'max-content' }}
            hidePagination
            empty={
              <Empty
                image={
                  <IllustrationNoResult style={{ width: 150, height: 150 }} />
                }
                darkModeImage={
                  <IllustrationNoResultDark
                    style={{ width: 150, height: 150 }}
                  />
                }
                description={t('暂无分组特殊倍率')}
                style={{ padding: 30 }}
              />
            }
            size='middle'
          />
        </div>
      </div>
    </SideSheet>
  );
};

export default AggregateGroupRatioOverridesModal;
