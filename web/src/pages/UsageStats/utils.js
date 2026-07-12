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

import dayjs from 'dayjs';

export const CHART_CONFIG = { mode: 'desktop-browser' };
export const DEFAULT_PAGE_SIZE = 20;
const DEFAULT_QUOTA_PER_UNIT = 500000;

export const createDefaultFilters = () => ({
  dateRange: [dayjs().startOf('day').toDate(), dayjs().endOf('day').toDate()],
  modelName: '',
  username: '',
  group: '',
  channel: '',
  trendGranularity: 'auto',
});

export const buildUsageStatsParams = (filters) => {
  const range = filters?.dateRange;
  if (!Array.isArray(range) || range.length !== 2) {
    return null;
  }
  const start = range[0] ? dayjs(range[0]).startOf('day') : null;
  const end = range[1] ? dayjs(range[1]).endOf('day') : null;
  if (!start?.isValid() || !end?.isValid()) {
    return null;
  }
  const params = {
    start_timestamp: start.unix(),
    end_timestamp: end.unix(),
    limit: 50,
  };
  const optional = [
    ['model_name', filters.modelName],
    ['username', filters.username],
    ['group', filters.group],
    ['channel', filters.channel],
  ];
  optional.forEach(([key, value]) => {
    const normalized = String(value || '').trim();
    if (normalized) params[key] = normalized;
  });
  if (filters.trendGranularity !== 'auto') {
    params.trend_granularity = filters.trendGranularity;
  }
  return params;
};

export const filtersCacheKey = (filters) => {
  const params = buildUsageStatsParams(filters);
  return params ? JSON.stringify(params) : '';
};

const getQuotaPerUnitValue = () => {
  const value = Number(localStorage.getItem('quota_per_unit'));
  return Number.isFinite(value) && value > 0 ? value : DEFAULT_QUOTA_PER_UNIT;
};

export const quotaToUSD = (quota) =>
  Number(quota || 0) / getQuotaPerUnitValue();

export const formatUSDValue = (value) => {
  const amount = Number(value || 0);
  if (amount > 0 && amount < 0.01) return `$${amount.toFixed(4)}`;
  return `$${amount.toFixed(2)}`;
};

export const formatQuotaUSD = (quota) => formatUSDValue(quotaToUSD(quota));

export const formatTokensMillion = (tokens) => {
  const millions = Number(tokens || 0) / 1000000;
  if (millions > 0 && millions < 0.0001) return '<0.0001M';
  return `${millions.toFixed(millions > 0 && millions < 0.01 ? 4 : 2)}M`;
};

export const formatDateTime = (timestamp) => {
  if (!timestamp) return '-';
  return new Date(timestamp * 1000).toLocaleString();
};

export const formatUseTime = (value) => `${Number(value || 0).toFixed(1)}s`;

export const normalizeDetailsByUser = (details = []) =>
  details.reduce((result, item) => {
    if (!result[item.user_id]) result[item.user_id] = [];
    result[item.user_id].push({
      ...item,
      key: `${item.user_id}-${item.model_name}`,
    });
    return result;
  }, {});
