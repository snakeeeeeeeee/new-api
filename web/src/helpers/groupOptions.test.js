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

import { describe, expect, test } from 'bun:test';
import { buildGroupOption, groupTokenOptionsByCategory } from './groupOptions';

const rawGroups = {
  auto: {
    desc: 'Auto',
    ratio: 1,
    type: 'auto',
  },
  vip: {
    desc: 'VIP',
    ratio: 1.2,
    type: 'real',
  },
  'legacy-image': {
    desc: 'Legacy Image',
    ratio: 1,
    type: 'aggregate',
    category_id: 0,
  },
  'gpt-text': {
    desc: 'Zulu Text',
    ratio: 1.2,
    type: 'aggregate',
    category_id: 4,
    category_name: '文本',
    category_order: 0,
  },
  'claude-ha': {
    desc: 'Alpha Text',
    ratio: 1,
    type: 'aggregate',
    category_id: 4,
    category_name: '文本',
    category_order: 0,
  },
  'flux-image': {
    desc: 'Flux Image',
    ratio: 1,
    type: 'aggregate',
    category_id: 7,
    category_name: '生图',
    category_order: 1,
  },
};

describe('token group category options', () => {
  const options = Object.entries(rawGroups).map(([group, info]) =>
    buildGroupOption(group, info),
  );

  test('keeps aggregate category metadata from the user groups response', () => {
    expect(options.find((option) => option.value === 'gpt-text')).toEqual(
      expect.objectContaining({
        categoryId: 4,
        categoryName: '文本',
        categoryOrder: 0,
        groupType: 'aggregate',
      }),
    );
  });

  test('orders custom sections and puts real and uncategorized groups last', () => {
    const sections = groupTokenOptionsByCategory(options, '其他');

    expect(sections.map((section) => section.label)).toEqual([
      '文本',
      '生图',
      '其他',
    ]);
    expect(sections[0].options.map((option) => option.value)).toEqual([
      'claude-ha',
      'gpt-text',
    ]);
    expect(sections[2].options.map((option) => option.value)).toEqual([
      'legacy-image',
      'vip',
    ]);
    expect(
      sections
        .flatMap((section) => section.options)
        .some((option) => option.value === 'auto'),
    ).toBe(false);
  });
});
