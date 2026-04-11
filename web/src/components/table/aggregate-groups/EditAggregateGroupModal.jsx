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
  Input,
  InputNumber,
  Row,
  Select,
  SideSheet,
  Space,
  Switch,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconArrowDown,
  IconArrowUp,
  IconClose,
  IconSave,
  IconServer,
} from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';
import { useIsMobile } from '../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const defaultInputs = {
  id: undefined,
  name: '',
  display_name: '',
  description: '',
  status: 1,
  group_ratio: 1,
  smart_routing_enabled: false,
  recovery_enabled: true,
  recovery_interval_seconds: 300,
  retry_status_codes: '',
  visible_user_groups: [],
  targets: [],
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

  const isEdit = editingGroup?.id !== undefined;

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
          retry_status_codes: data.retry_status_codes || '',
          visible_user_groups: data.visible_user_groups || [],
          targets: (data.targets || []).map((item) => item.real_group),
        });
      } catch (error) {
        showError(error?.message || t('获取聚合分组详情失败'));
      } finally {
        setLoading(false);
      }
    };

    loadDetail();
  }, [visible, isEdit, editingGroup?.id, t]);

  const availableTargetOptions = useMemo(() => {
    const selected = new Set(inputs.targets);
    return (realGroupOptions || []).map((option) => ({
      ...option,
      disabled: selected.has(option.value) && !inputs.targets.includes(option.value),
    }));
  }, [inputs.targets, realGroupOptions]);

  const updateField = (field, value) => {
    setInputs((prev) => ({
      ...prev,
      [field]: value,
    }));
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
        smart_routing_enabled: inputs.smart_routing_enabled,
        recovery_enabled: inputs.recovery_enabled,
        recovery_interval_seconds: Number(inputs.recovery_interval_seconds),
        retry_status_codes: inputs.retry_status_codes.trim(),
        visible_user_groups: inputs.visible_user_groups,
        targets: inputs.targets.map((real_group) => ({ real_group })),
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
      width={isMobile ? '100%' : 720}
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
        <Card className='!rounded-2xl shadow-sm border-0'>
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
            <Col span={12}>
              <div className='mb-2'>
                <Text strong>{t('分组名称')}</Text>
              </div>
              <Input
                value={inputs.name}
                onChange={(value) => updateField('name', value)}
                placeholder={t('请输入唯一分组名称')}
              />
            </Col>
            <Col span={12}>
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
            <Col span={8}>
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
            <Col span={8}>
              <div className='mb-2'>
                <Text strong>{t('启用状态')}</Text>
              </div>
              <Switch
                checked={inputs.status === 1}
                onChange={(checked) => updateField('status', checked ? 1 : 2)}
              />
            </Col>
            <Col span={8}>
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
            <Col span={8}>
              <div className='mb-2'>
                <Text strong>{t('懒恢复')}</Text>
              </div>
              <Switch
                checked={inputs.recovery_enabled}
                onChange={(checked) => updateField('recovery_enabled', checked)}
              />
            </Col>
            <Col span={12}>
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
            </Col>
            <Col span={12}>
              <div className='mb-2'>
                <Text strong>{t('聚合重试状态码')}</Text>
              </div>
              <Input
                value={inputs.retry_status_codes}
                onChange={(value) => updateField('retry_status_codes', value)}
                placeholder={t('留空沿用系统规则，例如：401,403,429,500-599')}
              />
              <div className='mt-1 text-xs text-gray-500'>
                {t(
                  '仅对当前聚合分组生效；填写后覆盖系统默认重试状态码规则。',
                )}
              </div>
            </Col>
            <Col span={12}>
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
                style={{ width: '100%' }}
              />
            </Col>
          </Row>
        </Card>

        <Card className='!rounded-2xl shadow-sm border-0 mt-3'>
          <div className='flex items-center mb-2'>
            <Avatar size='small' color='green' className='mr-2 shadow-md'>
              <IconServer size={16} />
            </Avatar>
            <div>
              <Text className='text-lg font-medium'>{t('真实分组链')}</Text>
              <div className='text-xs text-gray-600'>
                {t('选择真实分组并按顺序排列，顶部优先级最高')}
              </div>
            </div>
          </div>
          <div className='mb-2'>
            <Text strong>{t('添加真实分组')}</Text>
          </div>
          <Select
            placeholder={t('选择真实分组')}
            value={inputs.targets}
            onChange={(value) => updateField('targets', value || [])}
            optionList={availableTargetOptions}
            multiple
            style={{ width: '100%' }}
          />
          <div className='flex flex-col gap-2 mt-3'>
            {inputs.targets.map((target, index) => (
              <Card key={target} className='!rounded-xl border !shadow-none'>
                <div className='flex items-center justify-between gap-2'>
                  <Space>
                    <Tag color='blue' shape='circle'>
                      {index + 1}
                    </Tag>
                    <Text strong>{target}</Text>
                  </Space>
                  <Space>
                    <Button
                      theme='borderless'
                      icon={<IconArrowUp />}
                      disabled={index === 0}
                      onClick={() => moveTarget(index, -1)}
                    />
                    <Button
                      theme='borderless'
                      icon={<IconArrowDown />}
                      disabled={index === inputs.targets.length - 1}
                      onClick={() => moveTarget(index, 1)}
                    />
                    <Button
                      theme='borderless'
                      type='danger'
                      icon={<IconClose />}
                      onClick={() => removeTarget(index)}
                    />
                  </Space>
                </div>
              </Card>
            ))}
          </div>
        </Card>
      </div>
    </SideSheet>
  );
};

export default EditAggregateGroupModal;
