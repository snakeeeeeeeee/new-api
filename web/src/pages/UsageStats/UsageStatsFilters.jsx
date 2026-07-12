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

import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, DatePicker, Input, Select, Tooltip } from '@douyinfe/semi-ui';
import {
  ChevronDown,
  ChevronUp,
  RefreshCw,
  RotateCcw,
  Search,
  SlidersHorizontal,
  Users,
} from 'lucide-react';
import { DATE_RANGE_PRESETS } from '../../constants/console.constants';

const UsageStatsFilters = ({
  value,
  onChange,
  onApply,
  onRefresh,
  onReset,
  loading,
  canRefresh,
}) => {
  const { t } = useTranslation();
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const updateField = (field, nextValue) =>
    onChange({ ...value, [field]: nextValue });

  return (
    <section className='sticky top-16 z-30 border-b border-[var(--semi-color-border)] bg-[var(--semi-color-bg-0)]/95 py-3 backdrop-blur-sm'>
      <div className='flex flex-col gap-3'>
        <div className='grid grid-cols-1 gap-2 md:grid-cols-2 xl:grid-cols-[minmax(260px,1.25fr)_minmax(160px,0.75fr)_minmax(180px,0.85fr)_auto_auto]'>
          <DatePicker
            type='dateRange'
            value={value.dateRange}
            onChange={(nextValue) => updateField('dateRange', nextValue)}
            placeholder={[t('开始日期'), t('结束日期')]}
            showClear
            className='w-full'
            presets={DATE_RANGE_PRESETS.map((preset) => ({
              text: t(preset.text),
              start: preset.start(),
              end: preset.end(),
            }))}
          />
          <Input
            prefix={<Users size={16} />}
            value={value.username}
            onChange={(nextValue) => updateField('username', nextValue)}
            placeholder={t('用户名')}
            showClear
            onEnterPress={onApply}
          />
          <Input
            prefix={<Search size={16} />}
            value={value.modelName}
            onChange={(nextValue) => updateField('modelName', nextValue)}
            placeholder={t('模型，支持 % 通配')}
            showClear
            onEnterPress={onApply}
          />
          <Button
            type='primary'
            icon={<Search size={16} />}
            loading={loading}
            onClick={onApply}
            className='min-h-10'
          >
            {t('查询')}
          </Button>
          <div className='flex gap-2'>
            <Button
              icon={<SlidersHorizontal size={16} />}
              onClick={() => setAdvancedOpen((open) => !open)}
              className='min-h-10 flex-1'
            >
              {t('更多筛选')}
              {advancedOpen ? (
                <ChevronUp size={14} />
              ) : (
                <ChevronDown size={14} />
              )}
            </Button>
            <Tooltip content={t('刷新')}>
              <Button
                aria-label={t('刷新')}
                icon={<RefreshCw size={16} />}
                disabled={!canRefresh}
                loading={loading}
                onClick={onRefresh}
                className='min-h-10 min-w-10'
              />
            </Tooltip>
            <Tooltip content={t('重置')}>
              <Button
                aria-label={t('重置')}
                icon={<RotateCcw size={16} />}
                disabled={loading}
                onClick={onReset}
                className='min-h-10 min-w-10'
              />
            </Tooltip>
          </div>
        </div>

        {advancedOpen && (
          <div className='grid grid-cols-1 gap-2 border-t border-[var(--semi-color-border)] pt-3 sm:grid-cols-3'>
            <Input
              value={value.group}
              onChange={(nextValue) => updateField('group', nextValue)}
              placeholder={t('分组')}
              showClear
              onEnterPress={onApply}
            />
            <Input
              value={value.channel}
              onChange={(nextValue) => updateField('channel', nextValue)}
              placeholder={t('渠道 ID')}
              showClear
              onEnterPress={onApply}
            />
            <Select
              value={value.trendGranularity}
              onChange={(nextValue) =>
                updateField('trendGranularity', nextValue)
              }
              optionList={[
                { label: t('自动粒度'), value: 'auto' },
                { label: t('小时'), value: 'hour' },
                { label: t('天'), value: 'day' },
              ]}
              className='w-full'
            />
          </div>
        )}
      </div>
    </section>
  );
};

export default UsageStatsFilters;
