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
import { useTranslation } from 'react-i18next';
import {
  Button,
  Input,
  Popconfirm,
  SideSheet,
  Space,
  Spin,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconArrowDown,
  IconArrowUp,
  IconClose,
  IconDelete,
  IconEdit,
  IconPlus,
  IconSave,
} from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../../helpers';
import { useIsMobile } from '../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const AggregateGroupCategorySideSheet = ({
  visible,
  categories,
  otherCount,
  onClose,
  onChanged,
}) => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [newName, setNewName] = useState('');
  const [editingId, setEditingId] = useState(null);
  const [editingName, setEditingName] = useState('');
  const [actionKey, setActionKey] = useState('');

  const sortedCategories = useMemo(
    () =>
      [...(categories || [])].sort(
        (left, right) =>
          left.order_index - right.order_index || left.id - right.id,
      ),
    [categories],
  );
  const busy = Boolean(actionKey);
  const iconButtonStyle = isMobile ? { width: 44, height: 44 } : undefined;

  useEffect(() => {
    if (!visible) {
      setNewName('');
      setEditingId(null);
      setEditingName('');
      setActionKey('');
    }
  }, [visible]);

  const validateName = (value, currentId = null) => {
    const name = value.trim();
    if (!name) {
      showError(t('业务分类名称不能为空'));
      return null;
    }
    if (Array.from(name).length > 32) {
      showError(t('业务分类名称不能超过 32 个字符'));
      return null;
    }
    const normalizedName = name.toLowerCase();
    if (
      sortedCategories.some(
        (category) =>
          category.id !== currentId &&
          category.name.trim().toLowerCase() === normalizedName,
      )
    ) {
      showError(t('业务分类名称已存在'));
      return null;
    }
    return name;
  };

  const runAction = async (key, request, successMessage) => {
    setActionKey(key);
    try {
      const res = await request();
      const { success, message } = res.data || {};
      if (!success) {
        showError(t(message));
        return false;
      }
      showSuccess(successMessage);
      await onChanged?.();
      return true;
    } catch (error) {
      showError(error?.message || t('操作失败'));
      return false;
    } finally {
      setActionKey('');
    }
  };

  const handleCreate = async () => {
    const name = validateName(newName);
    if (!name) return;
    const success = await runAction(
      'create',
      () => API.post('/api/aggregate_group/categories', { name }),
      t('业务分类新增成功'),
    );
    if (success) setNewName('');
  };

  const handleRename = async (category) => {
    const name = validateName(editingName, category.id);
    if (!name) return;
    const success = await runAction(
      `rename-${category.id}`,
      () => API.put(`/api/aggregate_group/categories/${category.id}`, { name }),
      t('业务分类更新成功'),
    );
    if (success) {
      setEditingId(null);
      setEditingName('');
    }
  };

  const handleMove = async (index, offset) => {
    const targetIndex = index + offset;
    if (targetIndex < 0 || targetIndex >= sortedCategories.length) return;
    const categoryIds = sortedCategories.map((category) => category.id);
    [categoryIds[index], categoryIds[targetIndex]] = [
      categoryIds[targetIndex],
      categoryIds[index],
    ];
    await runAction(
      `move-${sortedCategories[index].id}`,
      () =>
        API.put('/api/aggregate_group/categories/order', {
          category_ids: categoryIds,
        }),
      t('业务分类排序已更新'),
    );
  };

  const handleDelete = async (category) => {
    await runAction(
      `delete-${category.id}`,
      () => API.delete(`/api/aggregate_group/categories/${category.id}`),
      t('业务分类删除成功'),
    );
  };

  const renderIconButton = ({
    label,
    icon,
    onClick,
    disabled,
    loading,
    type = 'tertiary',
    tooltip = true,
  }) => {
    const button = (
      <Button
        aria-label={label}
        title={tooltip ? undefined : label}
        icon={icon}
        theme='borderless'
        type={type}
        style={iconButtonStyle}
        disabled={busy || disabled}
        loading={loading}
        onClick={onClick}
      />
    );

    return tooltip ? <Tooltip content={label}>{button}</Tooltip> : button;
  };

  return (
    <SideSheet
      visible={visible}
      onCancel={onClose}
      placement='right'
      width={isMobile ? '100%' : 480}
      title={<Title heading={4}>{t('业务分类管理')}</Title>}
      bodyStyle={{ padding: isMobile ? 16 : 24 }}
    >
      <Spin spinning={busy}>
        <div className='flex flex-col gap-2 sm:flex-row'>
          <Input
            value={newName}
            maxLength={32}
            showClear
            disabled={busy}
            placeholder={t('输入分类名称')}
            onChange={setNewName}
            onEnterPress={handleCreate}
          />
          <Button
            type='primary'
            icon={<IconPlus />}
            loading={actionKey === 'create'}
            disabled={busy || !newName.trim()}
            onClick={handleCreate}
          >
            {t('新增')}
          </Button>
        </div>

        <div
          className='mt-4 border-t'
          style={{ borderColor: 'var(--semi-color-border)' }}
        >
          {sortedCategories.map((category, index) => {
            const isEditing = editingId === category.id;
            return (
              <div
                key={category.id}
                className='flex flex-col gap-2 py-3 border-b sm:flex-row sm:items-center'
                style={{ borderColor: 'var(--semi-color-border)' }}
              >
                <div className='min-w-0 flex-1'>
                  {isEditing ? (
                    <Input
                      autoFocus
                      value={editingName}
                      maxLength={32}
                      disabled={busy}
                      onChange={setEditingName}
                      onEnterPress={() => handleRename(category)}
                    />
                  ) : (
                    <Text strong ellipsis={{ showTooltip: true }}>
                      {category.name}
                    </Text>
                  )}
                  <div className='mt-1'>
                    <Text type='tertiary' size='small'>
                      {t('{{total}} 个聚合分组', {
                        total: category.aggregate_group_count || 0,
                      })}
                    </Text>
                  </div>
                </div>

                <Space spacing={4} wrap>
                  {isEditing ? (
                    <>
                      {renderIconButton({
                        label: t('保存分类名称'),
                        icon: <IconSave />,
                        loading: actionKey === `rename-${category.id}`,
                        disabled: !editingName.trim(),
                        onClick: () => handleRename(category),
                      })}
                      {renderIconButton({
                        label: t('取消改名'),
                        icon: <IconClose />,
                        onClick: () => {
                          setEditingId(null);
                          setEditingName('');
                        },
                      })}
                    </>
                  ) : (
                    <>
                      {renderIconButton({
                        label: t('上移 {{name}}', { name: category.name }),
                        icon: <IconArrowUp />,
                        disabled: index === 0,
                        loading: actionKey === `move-${category.id}`,
                        onClick: () => handleMove(index, -1),
                      })}
                      {renderIconButton({
                        label: t('下移 {{name}}', { name: category.name }),
                        icon: <IconArrowDown />,
                        disabled: index === sortedCategories.length - 1,
                        loading: actionKey === `move-${category.id}`,
                        onClick: () => handleMove(index, 1),
                      })}
                      {renderIconButton({
                        label: t('修改 {{name}} 的名称', {
                          name: category.name,
                        }),
                        icon: <IconEdit />,
                        onClick: () => {
                          setEditingId(category.id);
                          setEditingName(category.name);
                        },
                      })}
                      <Popconfirm
                        disabled={busy}
                        title={t(
                          '删除分类“{{name}}”后，关联的 {{total}} 个聚合分组将移入“其他”，确认删除？',
                          {
                            name: category.name,
                            total: category.aggregate_group_count || 0,
                          },
                        )}
                        onConfirm={() => handleDelete(category)}
                      >
                        {renderIconButton({
                          label: t('删除 {{name}}', { name: category.name }),
                          icon: <IconDelete />,
                          type: 'danger',
                          loading: actionKey === `delete-${category.id}`,
                          tooltip: false,
                        })}
                      </Popconfirm>
                    </>
                  )}
                </Space>
              </div>
            );
          })}

          <div className='flex items-center gap-3 py-3'>
            <div className='min-w-0 flex-1'>
              <Text strong>{t('其他')}</Text>
              <div className='mt-1'>
                <Text type='tertiary' size='small'>
                  {t('{{total}} 个聚合分组', { total: otherCount || 0 })}
                </Text>
              </div>
            </div>
            <Tag size='small'>{t('系统分类')}</Tag>
          </div>
        </div>
      </Spin>
    </SideSheet>
  );
};

export default AggregateGroupCategorySideSheet;
