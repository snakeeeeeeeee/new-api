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
  InputNumber,
  Popconfirm,
  Row,
  Space,
  Spin,
  Switch,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import EditAggregateGroupModal from './EditAggregateGroupModal';
import AggregateGroupRuntimeDrawer from './AggregateGroupRuntimeDrawer';

const { Text } = Typography;

const defaultStrategyInputs = {
  'aggregate_group.smart_strategy_enabled': false,
  'aggregate_group.consecutive_failure_threshold': 2,
  'aggregate_group.degrade_duration_seconds': 600,
  'aggregate_group.slow_request_threshold_seconds': 30,
  'aggregate_group.consecutive_slow_threshold': 3,
};

const AggregateGroupsPage = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [optionsLoading, setOptionsLoading] = useState(false);
  const [groups, setGroups] = useState([]);
  const [realGroupOptions, setRealGroupOptions] = useState([]);
  const [userGroupOptions, setUserGroupOptions] = useState([]);
  const [editingGroup, setEditingGroup] = useState({ id: undefined });
  const [showEdit, setShowEdit] = useState(false);
  const [runtimeGroup, setRuntimeGroup] = useState(null);
  const [showRuntime, setShowRuntime] = useState(false);
  const [strategyInputs, setStrategyInputs] = useState(defaultStrategyInputs);
  const [strategyInputsRow, setStrategyInputsRow] =
    useState(defaultStrategyInputs);

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
      setRealGroupOptions(options);
      setUserGroupOptions(options);
    } catch (error) {
      showError(error?.message || t('获取分组选项失败'));
    }
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
        if (!Number.isNaN(parsedValue) && parsedValue > 0) {
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
      loadGroups();
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

  const columns = useMemo(
    () => [
      {
        title: t('聚合分组'),
        dataIndex: 'name',
        key: 'name',
        render: (_, record) => (
          <Space>
            <Text strong>{record.name}</Text>
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
        title: t('倍率'),
        dataIndex: 'group_ratio',
        key: 'group_ratio',
        render: (value) => `${value}x`,
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
          loadGroups();
        }}
        realGroupOptions={realGroupOptions}
        userGroupOptions={userGroupOptions}
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
          <div className='flex items-center justify-between gap-2 w-full'>
            <div>
              <Text strong>{t('聚合分组管理')}</Text>
              <div className='text-xs text-gray-600 mt-1'>
                {t('为提高可用性, 配置对外可见的逻辑分组与底层真实分组链路')}
              </div>
            </div>
            <Button
              type='primary'
              onClick={() => {
                setEditingGroup({ id: undefined });
                setShowEdit(true);
              }}
            >
              {t('新增聚合分组')}
            </Button>
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
                    {t('为开启智能策略的聚合分组配置连续失败、临时降级和慢请求阈值')}
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
                    <Text strong>{t('连续失败阈值')}</Text>
                  </div>
                  <InputNumber
                    min={1}
                    value={
                      strategyInputs[
                        'aggregate_group.consecutive_failure_threshold'
                      ]
                    }
                    onChange={(value) =>
                      updateStrategyField(
                        'aggregate_group.consecutive_failure_threshold',
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
                    <Text strong>{t('慢请求阈值（秒）')}</Text>
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
                    <Text strong>{t('连续慢请求阈值')}</Text>
                  </div>
                  <InputNumber
                    min={1}
                    value={
                      strategyInputs[
                        'aggregate_group.consecutive_slow_threshold'
                      ]
                    }
                    onChange={(value) =>
                      updateStrategyField(
                        'aggregate_group.consecutive_slow_threshold',
                        Number(value) || 1,
                      )
                    }
                    style={{ width: '100%' }}
                  />
                </Col>
              </Row>
            </Card>
          </Spin>
        ) : null}
        <CardTable
          rowKey='id'
          columns={columns}
          dataSource={groups}
          loading={loading}
          hidePagination
        />
      </CardPro>
    </>
  );
};

export default AggregateGroupsPage;
