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

const toNonNegativeNumberOrNull = (value) => {
  if (
    typeof value === 'boolean' ||
    value === '' ||
    value === null ||
    value === undefined
  ) {
    return null;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null;
};

const toPositiveNumber = (value) => {
  const parsed = toNonNegativeNumberOrNull(value);
  return parsed !== null && parsed > 0 ? parsed : 0;
};

const toOptionalBoolean = (value) => {
  if (typeof value === 'boolean') return value;
  if (value === 1 || value === '1' || value === 'true') return true;
  if (value === 0 || value === '0' || value === 'false') return false;
  return null;
};

export function normalizePromptCacheUsage(record = {}, other = {}) {
  record = record && typeof record === 'object' ? record : {};
  other = other && typeof other === 'object' ? other : {};

  const cacheReadTokens = toPositiveNumber(other.cache_tokens);
  const cacheWriteTokens5m = toPositiveNumber(other.cache_creation_tokens_5m);
  const cacheWriteTokens1h = toPositiveNumber(other.cache_creation_tokens_1h);
  const splitCacheWriteTokens = cacheWriteTokens5m + cacheWriteTokens1h;
  const legacyCacheWriteTokens = Math.max(
    toPositiveNumber(other.cache_creation_tokens),
    splitCacheWriteTokens,
  );
  const billedCacheWriteTokensFromLog = Math.max(
    toPositiveNumber(other.cache_write_tokens),
    legacyCacheWriteTokens,
  );

  const explicitBillingEnabled = toOptionalBoolean(
    other.cache_write_billing_enabled,
  );
  const cacheWriteBillingEnabled =
    explicitBillingEnabled ?? billedCacheWriteTokensFromLog > 0;
  const hasReportedCacheWriteTokens = Object.prototype.hasOwnProperty.call(
    other,
    'cache_write_tokens_reported',
  );
  const reportedCacheWriteTokensValue = hasReportedCacheWriteTokens
    ? toNonNegativeNumberOrNull(other.cache_write_tokens_reported)
    : null;
  const cacheWriteTokensReportValid =
    hasReportedCacheWriteTokens && reportedCacheWriteTokensValue !== null;
  const cacheWriteTokensReported = hasReportedCacheWriteTokens
    ? (reportedCacheWriteTokensValue ?? 0)
    : billedCacheWriteTokensFromLog;
  const cacheWriteTokensBilled = cacheWriteBillingEnabled
    ? hasReportedCacheWriteTokens
      ? cacheWriteTokensReported
      : billedCacheWriteTokensFromLog
    : 0;

  const isClaude =
    toOptionalBoolean(other.claude) === true ||
    other.usage_semantic === 'anthropic';
  const rowPromptTokens = toPositiveNumber(record.prompt_tokens);
  const explicitInputTokensTotal = toNonNegativeNumberOrNull(
    other.input_tokens_total,
  );
  const inputTokensTotal =
    explicitInputTokensTotal ??
    (isClaude
      ? rowPromptTokens + cacheReadTokens + cacheWriteTokensReported
      : rowPromptTokens);

  const ordinaryInputTokens =
    explicitInputTokensTotal !== null || !isClaude
      ? Math.max(inputTokensTotal - cacheReadTokens - cacheWriteTokensBilled, 0)
      : rowPromptTokens;

  const cacheWriteRatio =
    toNonNegativeNumberOrNull(other.cache_creation_ratio) ?? 1;
  const cacheWriteRatio5m =
    toNonNegativeNumberOrNull(other.cache_creation_ratio_5m) ?? cacheWriteRatio;
  const cacheWriteRatio1h =
    toNonNegativeNumberOrNull(other.cache_creation_ratio_1h) ?? cacheWriteRatio;

  return {
    isClaude,
    inputTokensTotal,
    ordinaryInputTokens,
    cacheReadTokens,
    cacheWriteTokensWasReported: hasReportedCacheWriteTokens,
    cacheWriteTokensReportValid,
    cacheWriteTokensReported,
    cacheWriteTokensBilled,
    cacheWriteBillingEnabled,
    cacheWriteRatio,
    cacheWriteTokens5m: cacheWriteBillingEnabled ? cacheWriteTokens5m : 0,
    cacheWriteRatio5m,
    cacheWriteTokens1h: cacheWriteBillingEnabled ? cacheWriteTokens1h : 0,
    cacheWriteRatio1h,
    hasSplitCacheWrite: cacheWriteBillingEnabled && splitCacheWriteTokens > 0,
  };
}
