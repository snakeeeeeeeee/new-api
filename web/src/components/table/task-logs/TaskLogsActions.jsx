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
import { Select, Typography } from '@douyinfe/semi-ui';
import { IconEyeOpened } from '@douyinfe/semi-icons';
import CompactModeToggle from '../../common/ui/CompactModeToggle';

const { Text } = Typography;

const REFRESH_OPTIONS = [
  { value: 0, label: '关闭' },
  { value: 5, label: '5 秒' },
  { value: 15, label: '15 秒' },
  { value: 30, label: '30 秒' },
];

const TaskLogsActions = ({
  compactMode,
  setCompactMode,
  refreshInterval,
  handleRefreshIntervalChange,
  t,
}) => {
  return (
    <div className='flex flex-col md:flex-row justify-between items-start md:items-center gap-2 w-full'>
      <div className='flex items-center text-orange-500 mb-2 md:mb-0'>
        <IconEyeOpened className='mr-2' />
        <Text>{t('任务记录')}</Text>
      </div>
      <div className='flex flex-wrap items-center gap-2'>
        <Select
          prefix={t('自动刷新')}
          value={refreshInterval}
          style={{ width: 150 }}
          optionList={REFRESH_OPTIONS.map((item) => ({
            ...item,
            label: t(item.label),
          }))}
          onChange={handleRefreshIntervalChange}
        />
        <CompactModeToggle
          compactMode={compactMode}
          setCompactMode={setCompactMode}
          t={t}
        />
      </div>
    </div>
  );
};

export default TaskLogsActions;
