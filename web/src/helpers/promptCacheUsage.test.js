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
import { normalizePromptCacheUsage } from './promptCacheUsage';

describe('normalizePromptCacheUsage', () => {
  test('separates configured GPT cache writes from total input', () => {
    const usage = normalizePromptCacheUsage(
      { prompt_tokens: 999999 },
      {
        input_tokens_total: 208343,
        cache_tokens: 206592,
        cache_write_tokens_reported: 729,
        cache_write_tokens: 729,
        cache_creation_ratio: 1.25,
        cache_write_billing_enabled: true,
      },
    );

    expect(usage.inputTokensTotal).toBe(208343);
    expect(usage.ordinaryInputTokens).toBe(1022);
    expect(usage.cacheWriteTokensReported).toBe(729);
    expect(usage.hasVisibleCacheWrite).toBe(true);
    expect(usage.cacheWriteTokensWasReported).toBe(true);
    expect(usage.cacheWriteTokensReportValid).toBe(true);
    expect(usage.cacheWriteTokensBilled).toBe(729);
    expect(usage.cacheWriteRatio).toBe(1.25);
  });

  test('keeps unconfigured GPT cache writes in ordinary input', () => {
    const usage = normalizePromptCacheUsage(
      { prompt_tokens: 208343 },
      {
        cache_tokens: 206592,
        cache_write_tokens_reported: 729,
        cache_write_billing_enabled: false,
      },
    );

    expect(usage.ordinaryInputTokens).toBe(1751);
    expect(usage.cacheWriteTokensReported).toBe(729);
    expect(usage.cacheWriteTokensBilled).toBe(0);
    expect(usage.cacheWriteBillingEnabled).toBe(false);
  });

  test('prefers normalized total input over the legacy prompt column', () => {
    const usage = normalizePromptCacheUsage(
      { prompt_tokens: 5000 },
      { input_tokens_total: 1000, cache_tokens: 100 },
    );

    expect(usage.inputTokensTotal).toBe(1000);
    expect(usage.ordinaryInputTokens).toBe(900);
  });

  test('infers separately billed writes from an old OpenAI log', () => {
    const usage = normalizePromptCacheUsage(
      { prompt_tokens: 1000 },
      {
        cache_tokens: 100,
        cache_creation_tokens: 100,
        cache_creation_ratio: 1.25,
      },
    );

    expect(usage.cacheWriteBillingEnabled).toBe(true);
    expect(usage.cacheWriteTokensReported).toBe(100);
    expect(usage.cacheWriteTokensWasReported).toBe(false);
    expect(usage.cacheWriteTokensReportValid).toBe(false);
    expect(usage.cacheWriteTokensBilled).toBe(100);
    expect(usage.ordinaryInputTokens).toBe(800);
  });

  test('preserves Claude split 5m and 1h cache writes', () => {
    const usage = normalizePromptCacheUsage(
      { prompt_tokens: 65 },
      {
        claude: true,
        cache_tokens: 200533,
        cache_creation_tokens: 729,
        cache_creation_tokens_5m: 700,
        cache_creation_tokens_1h: 29,
        cache_creation_ratio: 1.25,
        cache_creation_ratio_1h: 2,
      },
    );

    expect(usage.isClaude).toBe(true);
    expect(usage.inputTokensTotal).toBe(201327);
    expect(usage.ordinaryInputTokens).toBe(65);
    expect(usage.cacheWriteTokensBilled).toBe(729);
    expect(usage.cacheWriteTokens5m).toBe(700);
    expect(usage.cacheWriteTokens1h).toBe(29);
    expect(usage.cacheWriteRatio1h).toBe(2);
    expect(usage.hasSplitCacheWrite).toBe(true);
  });

  test('honors an explicit disabled flag over legacy-looking fields', () => {
    const usage = normalizePromptCacheUsage(
      { prompt_tokens: 1000 },
      {
        cache_creation_tokens: 100,
        cache_creation_tokens_5m: 60,
        cache_creation_tokens_1h: 40,
        cache_write_tokens_reported: 100,
        cache_write_billing_enabled: false,
      },
    );

    expect(usage.cacheWriteBillingEnabled).toBe(false);
    expect(usage.cacheWriteTokensBilled).toBe(0);
    expect(usage.cacheWriteTokens5m).toBe(0);
    expect(usage.cacheWriteTokens1h).toBe(0);
    expect(usage.hasSplitCacheWrite).toBe(false);
    expect(usage.ordinaryInputTokens).toBe(1000);
  });

  test('preserves a configured zero cache write ratio', () => {
    const usage = normalizePromptCacheUsage(
      { prompt_tokens: 1000 },
      {
        cache_write_tokens_reported: 100,
        cache_write_tokens: 100,
        cache_creation_ratio: 0,
        cache_write_billing_enabled: true,
      },
    );

    expect(usage.cacheWriteBillingEnabled).toBe(true);
    expect(usage.cacheWriteTokensBilled).toBe(100);
    expect(usage.cacheWriteRatio).toBe(0);
    expect(usage.ordinaryInputTokens).toBe(900);
  });

  test('preserves an explicitly reported zero over legacy fields', () => {
    const usage = normalizePromptCacheUsage(
      { prompt_tokens: 1000 },
      {
        cache_write_tokens_reported: 0,
        cache_creation_tokens: 100,
        cache_write_billing_enabled: true,
      },
    );

    expect(usage.cacheWriteTokensReported).toBe(0);
    expect(usage.cacheWriteTokensWasReported).toBe(true);
    expect(usage.cacheWriteTokensReportValid).toBe(true);
    expect(usage.cacheWriteTokensBilled).toBe(0);
    expect(usage.cacheWriteBillingEnabled).toBe(true);
    expect(usage.hasVisibleCacheWrite).toBe(false);
    expect(usage.ordinaryInputTokens).toBe(1000);
  });

  test('preserves an unconfigured explicitly reported zero', () => {
    const usage = normalizePromptCacheUsage(
      { prompt_tokens: 1000 },
      {
        cache_write_tokens_reported: 0,
        cache_write_billing_enabled: false,
      },
    );

    expect(usage.cacheWriteTokensWasReported).toBe(true);
    expect(usage.cacheWriteTokensReportValid).toBe(true);
    expect(usage.cacheWriteTokensReported).toBe(0);
    expect(usage.hasVisibleCacheWrite).toBe(false);
    expect(usage.cacheWriteTokensBilled).toBe(0);
    expect(usage.cacheWriteBillingEnabled).toBe(false);
    expect(usage.ordinaryInputTokens).toBe(1000);
  });

  test('marks a negative reported value invalid', () => {
    const usage = normalizePromptCacheUsage(
      { prompt_tokens: 1000 },
      {
        cache_write_tokens_reported: -5,
        cache_write_billing_enabled: false,
      },
    );

    expect(usage.cacheWriteTokensWasReported).toBe(true);
    expect(usage.cacheWriteTokensReportValid).toBe(false);
    expect(usage.cacheWriteTokensReported).toBe(0);
    expect(usage.cacheWriteTokensBilled).toBe(0);
    expect(usage.ordinaryInputTokens).toBe(1000);
  });

  test('accepts logs without an other payload', () => {
    expect(normalizePromptCacheUsage({ prompt_tokens: 12 }, null)).toEqual(
      expect.objectContaining({
        inputTokensTotal: 12,
        ordinaryInputTokens: 12,
        cacheReadTokens: 0,
        cacheWriteTokensReported: 0,
        hasVisibleCacheWrite: false,
        cacheWriteTokensWasReported: false,
        cacheWriteTokensReportValid: false,
      }),
    );
  });
});
