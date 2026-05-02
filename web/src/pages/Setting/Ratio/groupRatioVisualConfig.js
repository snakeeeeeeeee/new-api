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

export const GROUP_RATIO_INPUT_KEYS = [
  'GroupRatio',
  'UserUsableGroups',
  'GroupGroupRatio',
  'group_ratio_setting.group_special_usable_group',
  'AutoGroups',
  'DefaultUseAutoGroup',
];

export const DEFAULT_GROUP_RATIO_INPUTS = {
  GroupRatio: '{}',
  UserUsableGroups: '{}',
  GroupGroupRatio: '{}',
  'group_ratio_setting.group_special_usable_group': '{}',
  AutoGroups: '[]',
  DefaultUseAutoGroup: false,
};

const makeRowId = (prefix, index) => `${prefix}-${index}-${Date.now()}`;

const parseJsonValue = (rawValue, fallback, fieldName) => {
  if (rawValue === undefined || rawValue === null || rawValue === '') {
    return fallback;
  }

  try {
    return JSON.parse(rawValue);
  } catch (error) {
    throw new Error(`${fieldName} 不是合法的 JSON`);
  }
};

const ensurePlainObject = (value, fieldName) => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) {
    throw new Error(`${fieldName} 必须是 JSON 对象`);
  }
  return value;
};

const normalizeText = (value) => String(value ?? '').trim();

const normalizeRatio = (value, fallback = 1) => {
  const numberValue = Number(value);
  return Number.isFinite(numberValue) ? numberValue : fallback;
};

const formatJSON = (value) => JSON.stringify(value, null, 2);

export const isUserGroupName = (groupName) => {
  const name = normalizeText(groupName);
  return name === 'default' || name.startsWith('UserGroup-');
};

export const isRouteGroupName = (groupName) => {
  const name = normalizeText(groupName);
  return Boolean(name) && !isUserGroupName(name);
};

export const parseGroupRatioVisualConfig = (inputs) => {
  const groupRatio = ensurePlainObject(
    parseJsonValue(inputs.GroupRatio, {}, '分组倍率'),
    '分组倍率',
  );
  const userUsableGroups = ensurePlainObject(
    parseJsonValue(inputs.UserUsableGroups, {}, '用户可选分组'),
    '用户可选分组',
  );
  const groupGroupRatio = ensurePlainObject(
    parseJsonValue(inputs.GroupGroupRatio, {}, '分组特殊倍率'),
    '分组特殊倍率',
  );
  const specialUsableGroup = ensurePlainObject(
    parseJsonValue(
      inputs['group_ratio_setting.group_special_usable_group'],
      {},
      '分组特殊可用分组',
    ),
    '分组特殊可用分组',
  );
  const autoGroups = parseJsonValue(inputs.AutoGroups, [], '自动分组auto');

  if (!Array.isArray(autoGroups)) {
    throw new Error('自动分组auto 必须是 JSON 字符串数组');
  }

  const groupNames = Array.from(
    new Set([...Object.keys(groupRatio), ...Object.keys(userUsableGroups)]),
  );

  const baseGroups = groupNames.map((groupName, index) => ({
    id: makeRowId('base', index),
    groupName,
    ratio: normalizeRatio(groupRatio[groupName]),
    userSelectable: Object.prototype.hasOwnProperty.call(
      userUsableGroups,
      groupName,
    ),
    description:
      userUsableGroups[groupName] === undefined
        ? ''
        : String(userUsableGroups[groupName]),
  }));

  const specialRatios = [];
  Object.entries(groupGroupRatio).forEach(([userGroup, ratioMap]) => {
    if (!ratioMap || typeof ratioMap !== 'object' || Array.isArray(ratioMap)) {
      return;
    }
    Object.entries(ratioMap).forEach(([usingGroup, ratio]) => {
      specialRatios.push({
        id: makeRowId('special-ratio', specialRatios.length),
        userGroup,
        usingGroup,
        ratio: normalizeRatio(ratio),
      });
    });
  });

  const specialUsableGroups = [];
  Object.entries(specialUsableGroup).forEach(([userGroup, operations]) => {
    if (
      !operations ||
      typeof operations !== 'object' ||
      Array.isArray(operations)
    ) {
      return;
    }
    Object.entries(operations).forEach(([operationKey, description]) => {
      let action = 'add';
      let targetGroup = operationKey;
      if (operationKey.startsWith('+:')) {
        targetGroup = operationKey.slice(2);
      } else if (operationKey.startsWith('-:')) {
        action = 'remove';
        targetGroup = operationKey.slice(2);
      }
      specialUsableGroups.push({
        id: makeRowId('special-usable', specialUsableGroups.length),
        userGroup,
        action,
        targetGroup,
        description: String(description ?? ''),
      });
    });
  });

  return {
    baseGroups,
    autoGroups: autoGroups.map((group) => String(group)),
    defaultUseAutoGroup: Boolean(inputs.DefaultUseAutoGroup),
    specialRatios,
    specialUsableGroups,
  };
};

export const serializeGroupRatioVisualConfig = (config) => {
  const groupRatio = {};
  const userUsableGroups = {};
  const groupGroupRatio = {};
  const specialUsableGroup = {};

  config.baseGroups.forEach((group) => {
    const groupName = normalizeText(group.groupName);
    if (!groupName) return;
    groupRatio[groupName] = normalizeRatio(group.ratio);
    if (group.userSelectable) {
      userUsableGroups[groupName] =
        normalizeText(group.description) || groupName;
    }
  });

  config.specialRatios.forEach((row) => {
    const userGroup = normalizeText(row.userGroup);
    const usingGroup = normalizeText(row.usingGroup);
    if (!userGroup || !usingGroup) return;
    if (!groupGroupRatio[userGroup]) {
      groupGroupRatio[userGroup] = {};
    }
    groupGroupRatio[userGroup][usingGroup] = normalizeRatio(row.ratio);
  });

  config.specialUsableGroups.forEach((row) => {
    const userGroup = normalizeText(row.userGroup);
    const targetGroup = normalizeText(row.targetGroup);
    if (!userGroup || !targetGroup) return;
    if (!specialUsableGroup[userGroup]) {
      specialUsableGroup[userGroup] = {};
    }
    const operationKey =
      row.action === 'remove' ? `-:${targetGroup}` : `+:${targetGroup}`;
    specialUsableGroup[userGroup][operationKey] =
      normalizeText(row.description) || targetGroup;
  });

  return {
    GroupRatio: formatJSON(groupRatio),
    UserUsableGroups: formatJSON(userUsableGroups),
    GroupGroupRatio: formatJSON(groupGroupRatio),
    'group_ratio_setting.group_special_usable_group':
      formatJSON(specialUsableGroup),
    AutoGroups: formatJSON(
      config.autoGroups.map((group) => normalizeText(group)).filter(Boolean),
    ),
    DefaultUseAutoGroup: Boolean(config.defaultUseAutoGroup),
  };
};

export const buildGroupOptions = (config) => {
  const names = new Set();
  config.baseGroups.forEach((group) => {
    const groupName = normalizeText(group.groupName);
    if (groupName) names.add(groupName);
  });
  config.autoGroups.forEach((group) => {
    const groupName = normalizeText(group);
    if (groupName) names.add(groupName);
  });
  config.specialRatios.forEach((row) => {
    const userGroup = normalizeText(row.userGroup);
    const usingGroup = normalizeText(row.usingGroup);
    if (userGroup) names.add(userGroup);
    if (usingGroup) names.add(usingGroup);
  });
  config.specialUsableGroups.forEach((row) => {
    const userGroup = normalizeText(row.userGroup);
    const targetGroup = normalizeText(row.targetGroup);
    if (userGroup) names.add(userGroup);
    if (targetGroup) names.add(targetGroup);
  });

  return Array.from(names)
    .sort((a, b) => a.localeCompare(b))
    .map((groupName) => ({ label: groupName, value: groupName }));
};

export const buildUserGroupOptions = (config) =>
  buildGroupOptions(config).filter((option) => isUserGroupName(option.value));

export const buildRouteGroupOptions = (config) =>
  buildGroupOptions(config).filter((option) => isRouteGroupName(option.value));

export const getGroupReferences = (config, groupName) => {
  const name = normalizeText(groupName);
  if (!name) return [];

  const references = [];
  if (config.autoGroups.includes(name)) {
    references.push('auto 分组顺序');
  }
  if (
    config.specialRatios.some(
      (row) =>
        normalizeText(row.userGroup) === name ||
        normalizeText(row.usingGroup) === name,
    )
  ) {
    references.push('分组特殊倍率');
  }
  if (
    config.specialUsableGroups.some(
      (row) =>
        normalizeText(row.userGroup) === name ||
        normalizeText(row.targetGroup) === name,
    )
  ) {
    references.push('分组特殊可用分组');
  }

  return references;
};

export const removeGroupAndReferences = (config, groupName) => {
  const name = normalizeText(groupName);
  return {
    ...config,
    baseGroups: config.baseGroups.filter(
      (group) => normalizeText(group.groupName) !== name,
    ),
    autoGroups: config.autoGroups.filter(
      (group) => normalizeText(group) !== name,
    ),
    specialRatios: config.specialRatios.filter(
      (row) =>
        normalizeText(row.userGroup) !== name &&
        normalizeText(row.usingGroup) !== name,
    ),
    specialUsableGroups: config.specialUsableGroups.filter(
      (row) =>
        normalizeText(row.userGroup) !== name &&
        normalizeText(row.targetGroup) !== name,
    ),
  };
};

export const validateGroupRatioVisualConfig = (config) => {
  const groupNames = new Set();
  for (const group of config.baseGroups) {
    const groupName = normalizeText(group.groupName);
    if (!groupName) {
      return { valid: false, message: '基础分组中存在空分组名' };
    }
    if (groupNames.has(groupName)) {
      return { valid: false, message: `基础分组重复: ${groupName}` };
    }
    groupNames.add(groupName);
    if (!Number.isFinite(Number(group.ratio)) || Number(group.ratio) < 0) {
      return { valid: false, message: `分组倍率不能小于 0: ${groupName}` };
    }
  }

  const autoSeen = new Set();
  for (const group of config.autoGroups) {
    const groupName = normalizeText(group);
    if (!groupName) {
      return { valid: false, message: 'auto 分组中存在空分组名' };
    }
    if (!groupNames.has(groupName)) {
      return {
        valid: false,
        message: `auto 分组引用了不存在的基础分组: ${groupName}`,
      };
    }
    if (!isRouteGroupName(groupName)) {
      return {
        valid: false,
        message: `auto 分组只能选择真实路由分组，不能是 default 或 UserGroup-*：${groupName}`,
      };
    }
    if (autoSeen.has(groupName)) {
      return { valid: false, message: `auto 分组重复: ${groupName}` };
    }
    autoSeen.add(groupName);
  }

  const specialRatioSeen = new Set();
  for (const row of config.specialRatios) {
    const userGroup = normalizeText(row.userGroup);
    const usingGroup = normalizeText(row.usingGroup);
    if (!userGroup || !usingGroup) {
      return { valid: false, message: '分组特殊倍率中存在空分组' };
    }
    if (!isUserGroupName(userGroup)) {
      return {
        valid: false,
        message: `分组特殊倍率的用户分组只能是 default 或 UserGroup-*：${userGroup}`,
      };
    }
    if (!isRouteGroupName(usingGroup)) {
      return {
        valid: false,
        message: `分组特殊倍率的使用分组不能是 default 或 UserGroup-*：${usingGroup}`,
      };
    }
    if (!Number.isFinite(Number(row.ratio)) || Number(row.ratio) < 0) {
      return {
        valid: false,
        message: `特殊倍率不能小于 0: ${userGroup} -> ${usingGroup}`,
      };
    }
    const key = `${userGroup}::${usingGroup}`;
    if (specialRatioSeen.has(key)) {
      return {
        valid: false,
        message: `分组特殊倍率重复: ${userGroup} -> ${usingGroup}`,
      };
    }
    specialRatioSeen.add(key);
  }

  const specialUsableSeen = new Set();
  for (const row of config.specialUsableGroups) {
    const userGroup = normalizeText(row.userGroup);
    const targetGroup = normalizeText(row.targetGroup);
    if (!userGroup || !targetGroup) {
      return { valid: false, message: '分组特殊可用分组中存在空分组' };
    }
    if (!isUserGroupName(userGroup)) {
      return {
        valid: false,
        message: `分组特殊可用分组的用户分组只能是 default 或 UserGroup-*：${userGroup}`,
      };
    }
    if (!isRouteGroupName(targetGroup)) {
      return {
        valid: false,
        message: `分组特殊可用分组的目标分组不能是 default 或 UserGroup-*：${targetGroup}`,
      };
    }
    const action = row.action === 'remove' ? 'remove' : 'add';
    const key = `${userGroup}::${action}::${targetGroup}`;
    if (specialUsableSeen.has(key)) {
      return {
        valid: false,
        message: `分组特殊可用分组重复: ${userGroup} -> ${targetGroup}`,
      };
    }
    specialUsableSeen.add(key);
  }

  return { valid: true, message: '' };
};

export const validateGroupRatioInputs = (inputs) => {
  const config = parseGroupRatioVisualConfig(inputs);
  return validateGroupRatioVisualConfig(config);
};
