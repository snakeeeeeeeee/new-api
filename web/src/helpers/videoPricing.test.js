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
  bindVideoPricingModels,
  bindVideoPricingPolicyModels,
  calculateVideoPricingPreview,
  copyVideoPricingProfile,
  deleteVideoPricingProfile,
  getVideoPricingLogSummary,
  getVideoPricingProfileModels,
  normalizeVideoPricing,
  removeVideoPricingBinding,
  updateVideoPricingBinding,
  validateVideoPricing,
} from './videoPricing';

const config = {
  version: 1,
  profiles: {
    'seedance-720p': {
      name: 'Seedance 720p',
      billing_mode: 'per_second',
      unit_price: 0.03,
    },
  },
  model_bindings: {
    'seedance-1.5-pro-720p': {
      profile: 'seedance-720p',
      subscription_enabled: false,
    },
  },
};

describe('video pricing helpers', () => {
  test('normalizes and calculates unit price times seconds times group ratio', () => {
    const normalized = normalizeVideoPricing(config);
    expect(validateVideoPricing(normalized)).toEqual([]);
    expect(
      calculateVideoPricingPreview({
        profile: normalized.profiles['seedance-720p'],
        seconds: 6,
        groupRatio: 1.5,
      }),
    ).toEqual({
      seconds: 6,
      unit_price: 0.03,
      subtotal: 0.18,
      group_ratio: 1.5,
      total: 0.27,
    });
  });

  test('keeps exact model names and defaults subscription eligibility to false', () => {
    const bound = bindVideoPricingModels(
      config,
      [' Video-720p ', 'video-720p'],
      'seedance-720p',
    );
    expect(bound.model_bindings['Video-720p'].subscription_enabled).toBe(false);
    expect(bound.model_bindings['video-720p']).toBeDefined();
  });

  test('supports policy-only bindings and validates invalid policy bindings', () => {
    const policy = bindVideoPricingPolicyModels(config, ['legacy-video-model']);
    expect(policy.model_bindings['legacy-video-model']).toEqual({
      subscription_enabled: true,
    });
    policy.model_bindings.invalid = { subscription_enabled: false };
    expect(
      validateVideoPricing(policy).some((error) => error.includes('invalid')),
    ).toBe(true);
  });

  test('copies profiles and only deletes profiles without model bindings', () => {
    const copied = copyVideoPricingProfile(
      config,
      'seedance-720p',
      'seedance-copy',
      'Seedance copy',
    );
    expect(copied.profiles['seedance-copy'].name).toBe('Seedance copy');
    copied.model_bindings.eligible = {
      profile: 'seedance-copy',
      subscription_enabled: true,
    };
    copied.model_bindings.wallet = {
      profile: 'seedance-copy',
      subscription_enabled: false,
    };
    expect(getVideoPricingProfileModels(copied, 'seedance-copy')).toEqual([
      'eligible',
      'wallet',
    ]);
    const protectedConfig = deleteVideoPricingProfile(copied, 'seedance-copy');
    expect(protectedConfig.profiles['seedance-copy']).toBeDefined();
    const unbound = removeVideoPricingBinding(
      removeVideoPricingBinding(copied, 'eligible'),
      'wallet',
    );
    const removed = deleteVideoPricingProfile(unbound, 'seedance-copy');
    expect(removed.profiles['seedance-copy']).toBeUndefined();
  });

  test('updates priced and policy-only bindings and supports unbinding', () => {
    const withCopy = copyVideoPricingProfile(
      config,
      'seedance-720p',
      'seedance-copy',
      'Seedance copy',
    );
    const repriced = updateVideoPricingBinding(
      withCopy,
      'seedance-1.5-pro-720p',
      { profile: 'seedance-copy', subscription_enabled: true },
    );
    expect(repriced.model_bindings['seedance-1.5-pro-720p']).toEqual({
      profile: 'seedance-copy',
      subscription_enabled: true,
    });
    const policyOnly = updateVideoPricingBinding(
      repriced,
      'seedance-1.5-pro-720p',
      { profile: '' },
    );
    expect(policyOnly.model_bindings['seedance-1.5-pro-720p']).toEqual({
      subscription_enabled: true,
    });
    const removed = removeVideoPricingBinding(
      policyOnly,
      'seedance-1.5-pro-720p',
    );
    expect(removed.model_bindings['seedance-1.5-pro-720p']).toBeUndefined();
  });

  test('rejects missing prices, unsupported modes, and missing profiles', () => {
    const invalid = structuredClone(config);
    delete invalid.profiles['seedance-720p'].unit_price;
    invalid.profiles.bad = {
      name: 'Bad',
      billing_mode: 'per_request',
      unit_price: 1,
    };
    invalid.model_bindings.orphan = {
      profile: 'missing',
      subscription_enabled: false,
    };
    const errors = validateVideoPricing(invalid);
    expect(errors.some((error) => error.includes('每秒单价'))).toBe(true);
    expect(errors.some((error) => error.includes('per_second'))).toBe(true);
    expect(errors.some((error) => error.includes('orphan'))).toBe(true);
  });

  test('reads immutable pricing and execution audit fields', () => {
    expect(
      getVideoPricingLogSummary({
        video_pricing_snapshot: {
          public_model: 'seedance-1.5-pro-720p',
          profile_id: 'seedance-720p',
          billing_mode: 'video_per_second',
          unit_price: 0.03,
          seconds: 6,
          basis: 'generation_output',
          subtotal: 0.18,
          group_ratio: 1.5,
          final_quota: 270000,
          subscription_enabled: false,
        },
        video_execution_audit: {
          reported_duration_ms: 5800,
          matches_request: false,
        },
      }),
    ).toEqual(
      expect.objectContaining({
        seconds: 6,
        unit_price: 0.03,
        total: 0.27,
        reported_duration_ms: 5800,
        matches_request: false,
      }),
    );
  });
});
