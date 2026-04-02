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
  showSuccess,
  stringToColor,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';
import CardPro from '../../common/ui/CardPro';
import CardTable from '../../common/ui/CardTable';
import { Button, Popconfirm, Space, Tag, Typography } from '@douyinfe/semi-ui';
import EditAggregateGroupModal from './EditAggregateGroupModal';

const { Text } = Typography;

const AggregateGroupsPage = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [groups, setGroups] = useState([]);
  const [realGroupOptions, setRealGroupOptions] = useState([]);
  const [userGroupOptions, setUserGroupOptions] = useState([]);
  const [editingGroup, setEditingGroup] = useState({ id: undefined });
  const [showEdit, setShowEdit] = useState(false);

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

  useEffect(() => {
    loadGroups();
    loadGroupOptions();
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
