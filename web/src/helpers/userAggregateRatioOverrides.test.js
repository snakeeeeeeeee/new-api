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
import {
  aggregateRouteModelRuleKey,
  getAggregateRouteGroups,
  normalizeUserRouteModelRatioOverrides,
  resolveUserRouteModelRatio,
} from './userAggregateRatioOverrides';

const group = {
  name: 'aggregate-a',
  status: 1,
  group_ratio: 1.5,
  targets: [{ real_group: 'default' }],
  client_route_pools: {
    enabled: true,
    claude_code_cli: {
      enabled: true,
      targets: [{ real_group: 'cli' }],
    },
  },
  route_model_group_ratio_overrides: [
    {
      real_group: 'default',
      model_name: 'Exact-Model',
      group_ratio: 2,
      enabled: true,
    },
  ],
};

describe('user aggregate route model ratio helpers', () => {
  test('normalizes valid rules, preserves zero and treats model case exactly', () => {
    const rules = normalizeUserRouteModelRatioOverrides(
      [
        {
          aggregate_group: ' aggregate-a ',
          real_group: 'default',
          model_name: ' Exact-Model ',
          group_ratio: 0,
        },
        {
          aggregate_group: 'aggregate-a',
          real_group: 'default',
          model_name: 'exact-model',
          group_ratio: 3,
          enabled: false,
        },
        {
          aggregate_group: 'aggregate-a',
          real_group: 'missing',
          model_name: 'ignored',
          group_ratio: 1,
        },
      ],
      [group],
    );

    expect(rules).toHaveLength(2);
    expect(rules[0]).toEqual(
      expect.objectContaining({ group_ratio: 0, enabled: true }),
    );
    expect(rules[1]).toEqual(
      expect.objectContaining({ model_name: 'exact-model', enabled: false }),
    );
    expect(aggregateRouteModelRuleKey(rules[0])).not.toBe(
      aggregateRouteModelRuleKey(rules[1]),
    );
  });

  test('uses only enabled client route pools', () => {
    expect(getAggregateRouteGroups(group)).toEqual(['default', 'cli']);
    expect(
      getAggregateRouteGroups({
        ...group,
        client_route_pools: {
          ...group.client_route_pools,
          enabled: false,
        },
      }),
    ).toEqual(['default']);
  });

  test('calculates inherited and final values in billing precedence order', () => {
    const enabled = resolveUserRouteModelRatio({
      rule: {
        aggregate_group: group.name,
        real_group: 'default',
        model_name: 'Exact-Model',
        group_ratio: 0,
        enabled: true,
      },
      aggregateGroup: group,
      aggregateOverrides: { [group.name]: 0.5 },
    });
    expect(enabled).toEqual({
      inheritedRatio: 2,
      effectiveRatio: 0,
      inheritedSource: 'global_route_model',
    });

    const disabled = resolveUserRouteModelRatio({
      rule: {
        aggregate_group: group.name,
        real_group: 'default',
        model_name: 'other-model',
        group_ratio: 9,
        enabled: false,
      },
      aggregateGroup: group,
      aggregateOverrides: { [group.name]: 0 },
    });
    expect(disabled).toEqual({
      inheritedRatio: 0,
      effectiveRatio: 0,
      inheritedSource: 'user_aggregate',
    });
  });
});
