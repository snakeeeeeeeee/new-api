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

import React from 'react';
import { Modal, Typography } from '@douyinfe/semi-ui';

const { Text } = Typography;

const PromoteRootUserModal = ({ visible, onCancel, onConfirm, user, t }) => {
  return (
    <Modal
      title={t('确定要提升为超级管理员吗？')}
      visible={visible}
      onCancel={onCancel}
      onOk={onConfirm}
      type='warning'
    >
      <div className='space-y-2'>
        <Text>
          {t('此操作会让该管理员拥有全部菜单和系统设置权限。')}
        </Text>
        <Text type='secondary'>
          {user?.username ? `${t('目标用户')}: ${user.username}` : ''}
        </Text>
      </div>
    </Modal>
  );
};

export default PromoteRootUserModal;
