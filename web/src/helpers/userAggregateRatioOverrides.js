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

export const aggregateRouteModelRuleKey = (rule = {}) =>
  JSON.stringify([
    String(rule.aggregate_group || '').trim(),
    String(rule.real_group || '').trim(),
    String(rule.model_name || '').trim(),
  ]);

export const getAggregateRouteGroups = (group = {}) => {
  const groups = [];
  const seen = new Set();
  const add = (value) => {
    const realGroup = String(value || '').trim();
    if (!realGroup || seen.has(realGroup)) return;
    seen.add(realGroup);
    groups.push(realGroup);
  };

  (group.targets || []).forEach((target) => add(target?.real_group));
  const pools = group.client_route_pools;
  const cliPool = pools?.claude_code_cli;
  if (pools?.enabled && cliPool?.enabled) {
    (cliPool.targets || []).forEach((target) => add(target?.real_group));
  }
  return groups;
};

export const normalizeAggregateRatioOverrides = (overrides = {}) =>
  Object.entries(overrides || {})
    .map(([group, ratio]) => ({
      key: String(group || '').trim(),
      group: String(group || '').trim(),
      ratio: Number(ratio),
    }))
    .filter((row) => row.group && Number.isFinite(row.ratio) && row.ratio >= 0);

export const normalizeUserRouteModelRatioOverrides = (
  rules = [],
  aggregateGroups = [],
) => {
  const groupMap = new Map(
    (aggregateGroups || [])
      .filter((group) => group?.name && Number(group.status) === 1)
      .map((group) => [group.name, group]),
  );
  const normalized = [];
  const seen = new Set();
  (rules || []).forEach((rule) => {
    const aggregateGroup = String(rule?.aggregate_group || '').trim();
    const realGroup = String(rule?.real_group || '').trim();
    const modelName = String(rule?.model_name || '').trim();
    const groupRatio = Number(rule?.group_ratio);
    const group = groupMap.get(aggregateGroup);
    if (
      !group ||
      !getAggregateRouteGroups(group).includes(realGroup) ||
      !modelName ||
      !Number.isFinite(groupRatio) ||
      groupRatio < 0
    ) {
      return;
    }
    const item = {
      aggregate_group: aggregateGroup,
      real_group: realGroup,
      model_name: modelName,
      group_ratio: groupRatio,
      enabled: rule?.enabled !== false,
    };
    const key = aggregateRouteModelRuleKey(item);
    if (seen.has(key)) return;
    seen.add(key);
    normalized.push({ ...item, key });
  });
  return normalized;
};

export const resolveUserRouteModelRatio = ({
  rule,
  aggregateGroup,
  aggregateOverrides = {},
}) => {
  const hasUserAggregateRatio = Object.prototype.hasOwnProperty.call(
    aggregateOverrides || {},
    rule?.aggregate_group,
  );
  const aggregateRatio = hasUserAggregateRatio
    ? Number(aggregateOverrides[rule.aggregate_group])
    : Number(aggregateGroup?.group_ratio ?? 1);
  const globalRule = (
    aggregateGroup?.route_model_group_ratio_overrides || []
  ).find(
    (item) =>
      item?.enabled !== false &&
      item?.real_group === rule?.real_group &&
      item?.model_name === rule?.model_name &&
      Number.isFinite(Number(item?.group_ratio)) &&
      Number(item.group_ratio) >= 0,
  );
  const inheritedRatio = globalRule
    ? Number(globalRule.group_ratio)
    : aggregateRatio;
  const userRatio = Number(rule?.group_ratio);
  return {
    inheritedRatio,
    effectiveRatio:
      rule?.enabled !== false && Number.isFinite(userRatio) && userRatio >= 0
        ? userRatio
        : inheritedRatio,
    inheritedSource: globalRule
      ? 'global_route_model'
      : hasUserAggregateRatio
        ? 'user_aggregate'
        : 'aggregate_default',
  };
};
