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

export const IMAGE_PRICING_BILLING_TYPE = 'per_image_parameter';
export const IMAGE_PRICING_PARAMETERS = ['quality', 'size', 'resolution'];
export const MAX_IMAGE_N = 128;

export const createEmptyImagePricing = () => ({
  version: 1,
  profiles: {},
  model_bindings: {},
});

export const cloneImagePricing = (raw) => {
  const source =
    raw && typeof raw === 'object' ? raw : createEmptyImagePricing();
  return {
    version: source.version ?? 1,
    profiles: Object.fromEntries(
      Object.entries(source.profiles || {}).map(([id, profile]) => [
        id,
        {
          ...profile,
          tiers: (profile.tiers || []).map((tier) => ({
            ...tier,
            aliases: [...(tier.aliases || [])],
          })),
        },
      ]),
    ),
    model_bindings: Object.fromEntries(
      Object.entries(source.model_bindings || {}).map(([model, binding]) => [
        model,
        { ...binding },
      ]),
    ),
  };
};

export const copyImagePricingProfile = (
  raw,
  sourceProfileId,
  targetProfileId,
  targetName,
) => {
  const config = cloneImagePricing(raw);
  const source = config.profiles[sourceProfileId];
  if (!source || !targetProfileId || config.profiles[targetProfileId]) {
    return config;
  }
  config.profiles[targetProfileId] = {
    ...source,
    name: targetName || source.name,
    tiers: source.tiers.map((tier) => ({
      ...tier,
      aliases: [...(tier.aliases || [])],
    })),
  };
  return config;
};

export const deleteImagePricingProfile = (raw, profileId) => {
  const config = cloneImagePricing(raw);
  delete config.profiles[profileId];
  Object.entries(config.model_bindings).forEach(([model, binding]) => {
    if (binding.profile === profileId) {
      delete config.model_bindings[model];
    }
  });
  return config;
};

export const bindImagePricingModels = (
  raw,
  models,
  profileId,
  maxN = MAX_IMAGE_N,
) => {
  const config = cloneImagePricing(raw);
  if (!config.profiles[profileId]) return config;
  [...new Set(Array.isArray(models) ? models : [])].forEach((model) => {
    const name = asTrimmedString(model);
    if (name) {
      config.model_bindings[name] = { profile: profileId, max_n: maxN };
    }
  });
  return config;
};

const asObject = (value) =>
  value && typeof value === 'object' && !Array.isArray(value) ? value : {};

const asTrimmedString = (value) =>
  value === undefined || value === null ? '' : String(value).trim();

const normalizeAliases = (aliases) =>
  Array.isArray(aliases)
    ? [...new Set(aliases.map(asTrimmedString).filter(Boolean))]
    : [];

export const normalizeImagePricing = (raw) => {
  let parsed = raw;
  if (typeof raw === 'string') {
    try {
      parsed = JSON.parse(raw);
    } catch (_) {
      return createEmptyImagePricing();
    }
  }

  const source = asObject(parsed);
  const profiles = Object.fromEntries(
    Object.entries(asObject(source.profiles)).map(([id, profile]) => {
      const normalizedProfile = asObject(profile);
      return [
        id,
        {
          name: asTrimmedString(normalizedProfile.name),
          parameter: IMAGE_PRICING_PARAMETERS.includes(
            normalizedProfile.parameter,
          )
            ? normalizedProfile.parameter
            : 'quality',
          default_tier: asTrimmedString(normalizedProfile.default_tier),
          tiers: Array.isArray(normalizedProfile.tiers)
            ? normalizedProfile.tiers.map((tier) => {
                const normalizedTier = asObject(tier);
                const rawPrice =
                  normalizedTier.unit_price ?? normalizedTier.price;
                const price =
                  rawPrice === undefined || rawPrice === null || rawPrice === ''
                    ? Number.NaN
                    : Number(rawPrice);
                return {
                  key: asTrimmedString(normalizedTier.key),
                  upstream_value: asTrimmedString(
                    normalizedTier.upstream_value,
                  ),
                  aliases: normalizeAliases(normalizedTier.aliases),
                  unit_price: Number.isFinite(price) ? price : Number.NaN,
                };
              })
            : [],
        },
      ];
    }),
  );

  const modelBindings = Object.fromEntries(
    Object.entries(asObject(source.model_bindings)).map(([model, binding]) => {
      const normalizedBinding = asObject(binding);
      const rawMaxN = Object.prototype.hasOwnProperty.call(
        normalizedBinding,
        'max_n',
      )
        ? normalizedBinding.max_n
        : MAX_IMAGE_N;
      const maxN =
        rawMaxN === null || rawMaxN === '' ? Number.NaN : Number(rawMaxN);
      return [
        asTrimmedString(model),
        {
          profile: asTrimmedString(normalizedBinding.profile),
          max_n: Number.isInteger(maxN) ? maxN : Number.NaN,
        },
      ];
    }),
  );

  return {
    version: 1,
    profiles,
    model_bindings: Object.fromEntries(
      Object.entries(modelBindings).filter(([model]) => Boolean(model)),
    ),
  };
};

const normalizedMatchValue = (value) => asTrimmedString(value).toLowerCase();

const fallbackTranslate = (key, values = {}) =>
  String(key).replace(/\{\{(\w+)\}\}/g, (_, name) => values[name] ?? '');

export const resolveImagePricingTier = (profile, rawValue) => {
  if (!profile || !Array.isArray(profile.tiers)) return null;
  const raw = asTrimmedString(rawValue);
  const usedDefault = raw === '';
  const target = normalizedMatchValue(usedDefault ? profile.default_tier : raw);

  const tier = profile.tiers.find((candidate) => {
    const acceptedValues = [candidate.key, ...(candidate.aliases || [])];
    return acceptedValues.some(
      (value) => normalizedMatchValue(value) === target,
    );
  });
  if (!tier) return null;

  const matchedAlias = !usedDefault
    ? (tier.aliases || []).find(
        (alias) => normalizedMatchValue(alias) === target,
      ) || ''
    : '';

  return {
    tier,
    raw_value: raw,
    effective_value: tier.key,
    upstream_value: tier.upstream_value || tier.key,
    source: usedDefault ? 'default' : matchedAlias ? 'alias' : 'explicit',
    matched_alias: matchedAlias,
  };
};

export const getImagePricingProfileForModel = (config, modelName) => {
  const normalized = normalizeImagePricing(config);
  const binding = normalized.model_bindings[modelName];
  if (!binding) return null;
  const profile = normalized.profiles[binding.profile];
  if (!profile) return null;
  return { binding, profile, profile_id: binding.profile };
};

export const calculateImagePricingPreview = ({
  profile,
  rawValue = '',
  n = 1,
  groupRatio = 1,
}) => {
  const resolved = resolveImagePricingTier(profile, rawValue);
  if (!resolved) return null;
  const count = Number(n);
  const ratio = Number(groupRatio);
  if (!Number.isInteger(count) || count < 1 || !Number.isFinite(ratio)) {
    return null;
  }
  const unitPrice = Number(resolved.tier.unit_price);
  if (!Number.isFinite(unitPrice) || unitPrice < 0) return null;

  return {
    ...resolved,
    n: count,
    group_ratio: ratio,
    unit_price: unitPrice,
    subtotal: unitPrice * count,
    total: unitPrice * count * ratio,
  };
};

export const validateImagePricing = (raw, t = fallbackTranslate) => {
  const config = normalizeImagePricing(raw);
  const errors = [];
  Object.entries(config.profiles).forEach(([profileId, profile]) => {
    const prefix = t('模板 {{name}}', { name: profile.name || profileId });
    if (!profile.name) {
      errors.push(t('模板名称不能为空'));
    }

    if (!IMAGE_PRICING_PARAMETERS.includes(profile.parameter)) {
      errors.push(t('{{prefix}} 的计价参数无效', { prefix }));
    }
    if (profile.tiers.length === 0) {
      errors.push(t('{{prefix}} 至少需要一个价格档位', { prefix }));
      return;
    }

    const acceptedValues = new Map();
    profile.tiers.forEach((tier, index) => {
      if (!tier.key) {
        errors.push(
          t('{{prefix}} 第 {{index}} 档的客户端值不能为空', {
            prefix,
            index: index + 1,
          }),
        );
      }
      if (!tier.upstream_value) {
        errors.push(
          t('{{prefix}} 第 {{index}} 档的上游值不能为空', {
            prefix,
            index: index + 1,
          }),
        );
      }
      if (!Number.isFinite(tier.unit_price) || tier.unit_price < 0) {
        errors.push(
          t('{{prefix}} 第 {{index}} 档的单价必须是有限的非负数', {
            prefix,
            index: index + 1,
          }),
        );
      }

      [tier.key, ...(tier.aliases || [])].filter(Boolean).forEach((value) => {
        const normalized = normalizedMatchValue(value);
        if (acceptedValues.has(normalized)) {
          errors.push(
            t('{{prefix}} 的客户端值或别名重复：{{value}}', {
              prefix,
              value,
            }),
          );
        } else {
          acceptedValues.set(normalized, tier.key);
        }
      });
    });

    if (
      !profile.tiers.some(
        (tier) =>
          normalizedMatchValue(tier.key) ===
          normalizedMatchValue(profile.default_tier),
      )
    ) {
      errors.push(t('{{prefix}} 的默认档位不存在', { prefix }));
    }
  });

  Object.entries(config.model_bindings).forEach(([modelName, binding]) => {
    if (!config.profiles[binding.profile]) {
      errors.push(t('模型 {{model}} 绑定的模板不存在', { model: modelName }));
    }
    if (
      !Number.isInteger(binding.max_n) ||
      binding.max_n < 1 ||
      binding.max_n > MAX_IMAGE_N
    ) {
      errors.push(
        t('模型 {{model}} 的最大张数必须在 1 到 {{max}} 之间', {
          model: modelName,
          max: MAX_IMAGE_N,
        }),
      );
    }
  });

  return errors;
};

export const resolvePricingBillingType = (record) => {
  if (
    record?.billing_type === IMAGE_PRICING_BILLING_TYPE ||
    record?.image_pricing
  ) {
    return IMAGE_PRICING_BILLING_TYPE;
  }
  return Number(record?.quota_type) === 0 ? 'per_token' : 'per_request';
};

export const matchesPricingBillingType = (record, filter) => {
  if (filter === 'all') return true;
  const billingType = resolvePricingBillingType(record);
  if (filter === 0 || filter === 'per_token') {
    return billingType === 'per_token';
  }
  if (filter === 1 || filter === 'per_request') {
    return billingType === 'per_request';
  }
  return billingType === filter;
};

const firstFiniteNumber = (...values) => {
  for (const value of values) {
    if (value === undefined || value === null || value === '') continue;
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return null;
};

export const getImagePricingLogSummary = (other) => {
  if (!other || typeof other !== 'object') return null;
  const snapshot = other.image_pricing_snapshot || other.image_pricing;
  if (!snapshot || typeof snapshot !== 'object') return null;

  const parameter = asTrimmedString(
    snapshot.parameter || snapshot.parameter_name,
  );
  const effectiveValue = asTrimmedString(
    snapshot.effective_value ||
      snapshot.tier_key ||
      snapshot.effective_tier ||
      snapshot.upstream_value,
  );
  const count = firstFiniteNumber(
    snapshot.n,
    snapshot.image_count,
    other.image_count,
  );
  const unitPrice = firstFiniteNumber(
    snapshot.unit_price,
    snapshot.unit_price_usd,
    snapshot.unit_usd,
  );
  const groupRatio = firstFiniteNumber(
    snapshot.group_ratio,
    other.user_group_ratio !== -1 ? other.user_group_ratio : null,
    other.group_ratio,
    1,
  );
  const total = firstFiniteNumber(
    snapshot.total,
    snapshot.total_usd,
    snapshot.final_usd,
    unitPrice !== null && count !== null && groupRatio !== null
      ? unitPrice * count * groupRatio
      : null,
  );

  return {
    parameter,
    effective_value: effectiveValue,
    upstream_value: asTrimmedString(snapshot.upstream_value),
    source: asTrimmedString(snapshot.source || snapshot.value_source),
    n: count ?? 1,
    unit_price: unitPrice,
    group_ratio: groupRatio ?? 1,
    total,
  };
};

export const getImageExecutionAuditSummary = (other) => {
  const audit = other?.image_execution_audit;
  if (!audit || typeof audit !== 'object' || Array.isArray(audit)) return null;
  const summary = {
    quality: asTrimmedString(audit.quality),
    size: asTrimmedString(audit.size),
    resolution: asTrimmedString(audit.resolution),
    image_count: firstFiniteNumber(audit.image_count),
    total_tokens: firstFiniteNumber(audit.total_tokens),
    input_tokens: firstFiniteNumber(audit.input_tokens),
    output_tokens: firstFiniteNumber(audit.output_tokens),
    actual_quota: firstFiniteNumber(audit.actual_quota),
  };
  return Object.values(summary).some((value) => value !== '' && value !== null)
    ? summary
    : null;
};
