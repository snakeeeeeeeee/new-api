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
  Select,
  SideSheet,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  API,
  renderGroupOption,
  selectFilter,
  showError,
  showSuccess,
} from '../../../../helpers';
import { useIsMobile } from '../../../../hooks/common/useIsMobile';

const { Text } = Typography;

const normalizeBusinessGroupOptions = (options = []) =>
  (options || []).map((group) => ({
    label: group.label || group.value,
    value: group.value,
    groupType: group.group_type || group.groupType || 'real',
  }));

const ExtraUsableGroupsModal = ({ visible, onCancel, user, t, onSuccess }) => {
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [businessGroupOptions, setBusinessGroupOptions] = useState([]);
  const [extraUsableGroups, setExtraUsableGroups] = useState([]);

  const loadGroups = async () => {
    if (!user?.id) {
      setBusinessGroupOptions([]);
      setExtraUsableGroups([]);
      return;
    }
    setLoading(true);
    try {
      const res = await API.get(`/api/user/${user.id}/extra_usable_groups`);
      if (res.data?.success) {
        setBusinessGroupOptions(
          normalizeBusinessGroupOptions(
            res.data.data?.business_group_options || [],
          ),
        );
        setExtraUsableGroups(
          Array.isArray(res.data.data?.extra_usable_groups)
            ? res.data.data.extra_usable_groups
            : [],
        );
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
    loadGroups();
  }, [visible, user?.id]);

  const saveGroups = async () => {
    if (!user?.id) {
      showError(t('用户信息缺失'));
      return;
    }
    setSaving(true);
    try {
      const res = await API.put(`/api/user/${user.id}/extra_usable_groups`, {
        extra_usable_groups: extraUsableGroups,
      });
      if (res.data?.success) {
        setBusinessGroupOptions(
          normalizeBusinessGroupOptions(
            res.data.data?.business_group_options || [],
          ),
        );
        setExtraUsableGroups(
          Array.isArray(res.data.data?.extra_usable_groups)
            ? res.data.data.extra_usable_groups
            : [],
        );
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

  return (
    <SideSheet
      visible={visible}
      placement='right'
      width={isMobile ? '100%' : 640}
      bodyStyle={{ padding: 0 }}
      onCancel={onCancel}
      title={
        <Space>
          <Tag color='blue' shape='circle'>
            {t('授权')}
          </Tag>
          <Typography.Title heading={4} className='m-0'>
            {t('业务分组额外授权')}
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
            onClick={saveGroups}
          >
            {t('保存')}
          </Button>
        </Space>
      }
    >
      <Spin spinning={loading}>
        <div className='p-4'>
          <Text size='small' type='secondary'>
            {t('额外可用业务分组')}
          </Text>
          <Select
            className='mt-1'
            placeholder={t('请选择额外可用业务分组')}
            value={extraUsableGroups}
            onChange={(value) =>
              setExtraUsableGroups(Array.isArray(value) ? value : [])
            }
            optionList={businessGroupOptions}
            multiple
            filter={selectFilter}
            renderOptionItem={renderGroupOption}
            searchPosition='dropdown'
            style={{ width: '100%' }}
            showClear
            autoClearSearchValue={false}
            emptyContent={t('暂无可用业务分组')}
          />
          <Text type='tertiary' size='small' className='block mt-1'>
            {t('仅授权该用户额外可见和使用业务分组，不改变用户主分组')}
          </Text>
        </div>
      </Spin>
    </SideSheet>
  );
};

export default ExtraUsableGroupsModal;
