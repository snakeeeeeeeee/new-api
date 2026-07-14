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
  bindImagePricingModels,
  calculateImagePricingPreview,
  copyImagePricingProfile,
  deleteImagePricingProfile,
  getImageExecutionAuditSummary,
  getImagePricingLogSummary,
  matchesPricingBillingType,
  normalizeImagePricing,
  resolveImagePricingTier,
  resolvePricingBillingType,
  validateImagePricing,
} from './imagePricing';

const config = {
  version: 1,
  profiles: {
    quality: {
      name: 'ADOBE quality',
      parameter: 'quality',
      default_tier: 'low',
      tiers: [
        {
          key: 'low',
          upstream_value: 'low',
          aliases: ['auto'],
          unit_price: 0.04,
        },
        {
          key: 'high',
          upstream_value: 'high',
          aliases: [],
          unit_price: 0.15,
        },
      ],
    },
  },
  model_bindings: {
    'adobe-gpt-image-2-count': { profile: 'quality', max_n: 10 },
  },
};

describe('image pricing helpers', () => {
  test('uses the configured default tier for an omitted parameter', () => {
    const profile = normalizeImagePricing(config).profiles.quality;
    expect(resolveImagePricingTier(profile, '')).toEqual(
      expect.objectContaining({
        effective_value: 'low',
        upstream_value: 'low',
        source: 'default',
      }),
    );
  });

  test('normalizes aliases and calculates unit times n times group ratio', () => {
    const profile = normalizeImagePricing(config).profiles.quality;
    expect(
      calculateImagePricingPreview({
        profile,
        rawValue: 'AUTO',
        n: 3,
        groupRatio: 1.5,
      }),
    ).toEqual(
      expect.objectContaining({
        effective_value: 'low',
        source: 'alias',
        n: 3,
        subtotal: 0.12,
        total: 0.18,
      }),
    );
  });

  test('validates aliases, missing profiles, and max_n bounds', () => {
    const invalid = normalizeImagePricing(config);
    invalid.profiles.quality.tiers[1].aliases = ['auto'];
    invalid.model_bindings['adobe-gpt-image-2-count'].max_n = 129;
    invalid.model_bindings.orphan = { profile: 'missing', max_n: 1 };
    const errors = validateImagePricing(invalid);
    expect(errors.some((error) => error.includes('auto'))).toBe(true);
    expect(errors.some((error) => error.includes('128'))).toBe(true);
    expect(errors.some((error) => error.includes('orphan'))).toBe(true);
  });

  test('preserves a zero unit price', () => {
    const normalized = normalizeImagePricing(config);
    normalized.profiles.quality.tiers[0].unit_price = 0;
    expect(validateImagePricing(normalized)).toEqual([]);
  });

  test('uses the system max when max_n is omitted', () => {
    const withoutMax = structuredClone(config);
    delete withoutMax.model_bindings['adobe-gpt-image-2-count'].max_n;
    expect(
      normalizeImagePricing(withoutMax).model_bindings[
        'adobe-gpt-image-2-count'
      ].max_n,
    ).toBe(128);
  });

  test('rejects an explicit null max_n instead of treating it as omitted', () => {
    const withNullMax = structuredClone(config);
    withNullMax.model_bindings['adobe-gpt-image-2-count'].max_n = null;
    expect(validateImagePricing(withNullMax).length).toBeGreaterThan(0);
  });

  test('does not silently turn a missing unit price into a free tier', () => {
    const withoutPrice = structuredClone(config);
    delete withoutPrice.profiles.quality.tiers[0].unit_price;
    expect(validateImagePricing(withoutPrice).length).toBeGreaterThan(0);
  });

  test('copies and deletes profiles without leaving orphaned bindings', () => {
    const copied = copyImagePricingProfile(
      config,
      'quality',
      'quality-copy',
      'Quality copy',
    );
    expect(copied.profiles['quality-copy'].name).toBe('Quality copy');
    copied.profiles['quality-copy'].tiers[0].aliases.push('draft');
    expect(config.profiles.quality.tiers[0].aliases).toEqual(['auto']);

    const bound = bindImagePricingModels(
      copied,
      ['custom-a', ' custom-b ', 'custom-a'],
      'quality-copy',
      7,
    );
    expect(bound.model_bindings['custom-a']).toEqual({
      profile: 'quality-copy',
      max_n: 7,
    });
    expect(bound.model_bindings['custom-b']).toEqual({
      profile: 'quality-copy',
      max_n: 7,
    });

    const removed = deleteImagePricingProfile(bound, 'quality-copy');
    expect(removed.profiles['quality-copy']).toBeUndefined();
    expect(removed.model_bindings['custom-a']).toBeUndefined();
    expect(removed.model_bindings['adobe-gpt-image-2-count']).toBeDefined();
  });

  test('derives new and legacy public billing types', () => {
    expect(
      resolvePricingBillingType({
        quota_type: 1,
        billing_type: 'per_image_parameter',
      }),
    ).toBe('per_image_parameter');
    expect(resolvePricingBillingType({ quota_type: 0 })).toBe('per_token');
    expect(resolvePricingBillingType({ quota_type: 1 })).toBe('per_request');
    expect(
      matchesPricingBillingType(
        { quota_type: 1, image_pricing: { tiers: [] } },
        'per_image_parameter',
      ),
    ).toBe(true);
  });

  test('reads compatible image pricing log snapshot fields', () => {
    expect(
      getImagePricingLogSummary({
        group_ratio: 1.2,
        image_pricing_snapshot: {
          parameter: 'quality',
          effective_value: 'high',
          unit_price_usd: 0.15,
          image_count: 2,
        },
      }),
    ).toEqual(
      expect.objectContaining({
        parameter: 'quality',
        effective_value: 'high',
        n: 2,
        unit_price: 0.15,
        group_ratio: 1.2,
        total: 0.36,
      }),
    );
  });

  test('reads returned image execution audit separately from pricing', () => {
    expect(
      getImageExecutionAuditSummary({
        image_execution_audit: {
          quality: 'high',
          size: '2048x2048',
          resolution: '2k',
          image_count: 2,
          total_tokens: 345,
          input_tokens: 120,
          output_tokens: 225,
          actual_quota: 999,
        },
      }),
    ).toEqual({
      quality: 'high',
      size: '2048x2048',
      resolution: '2k',
      image_count: 2,
      total_tokens: 345,
      input_tokens: 120,
      output_tokens: 225,
      actual_quota: 999,
    });
  });
});
