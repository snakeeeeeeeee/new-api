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

export const buildGroupOption = (group, info) => ({
  label: info.desc.length > 20 ? info.desc.substring(0, 20) + '...' : info.desc,
  value: group,
  ratio: info.ratio,
  originalRatio: info.original_ratio,
  ratioOverride: info.ratio_override,
  hasRatioOverride: Boolean(info.has_ratio_override),
  fullLabel: info.desc,
  groupType: info.type || 'real',
  categoryId: Number(info.category_id ?? 0),
  categoryName: info.category_name || '',
  categoryOrder: Number(info.category_order ?? 0),
});

const compareGroupDisplayNames = (left, right) => {
  const leftLabel = String(left.fullLabel || left.label || left.value || '');
  const rightLabel = String(
    right.fullLabel || right.label || right.value || '',
  );
  const labelComparison = leftLabel.localeCompare(rightLabel);
  if (labelComparison !== 0) {
    return labelComparison;
  }
  return String(left.value || '').localeCompare(String(right.value || ''));
};

export const groupTokenOptionsByCategory = (groupOptions, otherLabel) => {
  const customSections = new Map();
  const otherOptions = [];

  (groupOptions || [])
    .filter((option) => option.groupType !== 'auto')
    .forEach((option) => {
      const isCustomCategory =
        option.groupType === 'aggregate' &&
        option.categoryId > 0 &&
        option.categoryName;
      if (!isCustomCategory) {
        otherOptions.push(option);
        return;
      }

      if (!customSections.has(option.categoryId)) {
        customSections.set(option.categoryId, {
          key: `category-${option.categoryId}`,
          label: option.categoryName,
          order: option.categoryOrder,
          options: [],
        });
      }
      customSections.get(option.categoryId).options.push(option);
    });

  const sections = Array.from(customSections.values()).sort(
    (left, right) =>
      left.order - right.order || left.label.localeCompare(right.label),
  );
  sections.forEach((section) => section.options.sort(compareGroupDisplayNames));

  if (otherOptions.length > 0) {
    sections.push({
      key: 'other',
      label: otherLabel,
      order: Number.MAX_SAFE_INTEGER,
      options: otherOptions.sort(compareGroupDisplayNames),
    });
  }

  return sections;
};
