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
  Modal,
  Spin,
  Switch,
  Typography,
} from '@douyinfe/semi-ui';
import { ShieldCheck } from 'lucide-react';
import { API, showError, showSuccess } from '../../../../helpers';

const { Text } = Typography;

const MENU_SECTIONS = [
  {
    title: '核心配置',
    items: [
      ['channel', '渠道管理', 'API 渠道配置与检测'],
      ['aggregate_group', '聚合分组', '聚合分组与真实分组链路'],
      ['models', '模型管理', '模型元数据与供应商配置'],
      ['deployment', '模型部署', '模型部署和实例管理'],
    ],
  },
  {
    title: '用户与权益',
    items: [
      ['user', '用户管理', '用户账户、额度和绑定管理'],
      ['subscription', '订阅管理', '套餐和用户订阅管理'],
      ['redemption', '兑换码管理', '兑换码生成与维护'],
      ['invite_code', '邀请码管理', '邀请码配置'],
      ['invite_stats', '邀请统计', '邀请消费与收益统计'],
    ],
  },
  {
    title: '审计与诊断',
    items: [
      ['log_dashboard', '日志看板', '日志查询和运行态统计'],
      ['usage_stats', '用量统计', '用户与模型消耗排行'],
      ['async_task', '异步任务管理', '异步任务调度与超时监控'],
      ['assets', '资源管理中心', '生成图片和视频资源管理'],
      ['request_dump', 'Dump 分析', '临时请求诊断控制台'],
      ['violation', '风险检测', '安全风控词命中审计'],
      ['compatibility', '兼容管理', '协议与渠道兼容策略'],
    ],
  },
];

const AdminMenuPermissionsModal = ({ visible, onCancel, user, t }) => {
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [selectedKeys, setSelectedKeys] = useState([]);

  const selectedSet = useMemo(() => new Set(selectedKeys), [selectedKeys]);

  const loadPermissions = async () => {
    if (!visible || !user?.id) return;
    setLoading(true);
    try {
      const res = await API.get(`/api/user/${user.id}/admin_menu_permissions`);
      const { success, data, message } = res.data;
      if (success) {
        setSelectedKeys(data?.menu_keys || []);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadPermissions();
  }, [visible, user?.id]);

  const toggleKey = (key, checked) => {
    setSelectedKeys((current) => {
      const next = new Set(current);
      if (checked) {
        next.add(key);
      } else {
        next.delete(key);
      }
      return Array.from(next);
    });
  };

  const selectAll = () => {
    setSelectedKeys(MENU_SECTIONS.flatMap((section) => section.items.map(([key]) => key)));
  };

  const clearAll = () => {
    setSelectedKeys([]);
  };

  const save = async () => {
    if (!user?.id) return;
    setSaving(true);
    try {
      const res = await API.put(`/api/user/${user.id}/admin_menu_permissions`, {
        menu_keys: selectedKeys,
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('保存成功'));
        onCancel?.();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(t('保存失败，请重试'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      title={
        <div className='flex items-center gap-2'>
          <ShieldCheck size={18} />
          <span>{t('菜单权限')}</span>
        </div>
      }
      visible={visible}
      onCancel={onCancel}
      onOk={save}
      confirmLoading={saving}
      width={760}
      okText={t('保存设置')}
      cancelText={t('取消')}
      bodyStyle={{ padding: '20px 24px' }}
    >
      <Spin spinning={loading}>
        <div className='mb-4 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
          <div>
            <Text strong>{user?.username || '-'}</Text>
            <Text type='secondary' className='ml-2'>
              {t('选择该管理员可以看到和直接访问的菜单')}
            </Text>
          </div>
          <div className='flex gap-2'>
            <Button size='small' type='tertiary' onClick={selectAll}>
              {t('全选')}
            </Button>
            <Button size='small' type='tertiary' onClick={clearAll}>
              {t('清空')}
            </Button>
          </div>
        </div>

        <div className='grid grid-cols-1 gap-3 md:grid-cols-3'>
          {MENU_SECTIONS.map((section) => (
            <div
              key={section.title}
              className='rounded-lg p-3'
              style={{
                border: '1px solid var(--semi-color-border)',
                background: 'var(--semi-color-bg-0)',
              }}
            >
              <div
                className='mb-3 text-sm font-semibold'
                style={{ color: 'var(--semi-color-text-0)' }}
              >
                {t(section.title)}
              </div>
              <div className='space-y-3'>
                {section.items.map(([key, title, description]) => (
                  <div
                    key={key}
                    className='flex min-h-[48px] items-center justify-between gap-3 rounded-md px-3 py-2'
                    style={{ background: 'var(--semi-color-fill-0)' }}
                  >
                    <div className='min-w-0'>
                      <div
                        className='text-sm font-medium'
                        style={{ color: 'var(--semi-color-text-0)' }}
                      >
                        {t(title)}
                      </div>
                      <div
                        className='text-xs leading-5'
                        style={{ color: 'var(--semi-color-text-2)' }}
                      >
                        {t(description)}
                      </div>
                    </div>
                    <Switch
                      checked={selectedSet.has(key)}
                      onChange={(checked) => toggleKey(key, checked)}
                    />
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </Spin>
    </Modal>
  );
};

export default AdminMenuPermissionsModal;
