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

export const VIDEO_PRICING_BILLING_TYPE = 'per_video_second';
export const VIDEO_PRICING_MODE = 'per_second';

const asObject = (value) =>
  value && typeof value === 'object' && !Array.isArray(value) ? value : {};

const asTrimmedString = (value) =>
  value === undefined || value === null ? '' : String(value).trim();

const firstFiniteNumber = (...values) => {
  for (const value of values) {
    if (value === undefined || value === null || value === '') continue;
    const parsed = Number(value);
    if (Number.isFinite(parsed)) return parsed;
  }
  return null;
};

const fallbackTranslate = (key, values = {}) =>
  String(key).replace(/\{\{(\w+)\}\}/g, (_, name) => values[name] ?? '');

export const createEmptyVideoPricing = () => ({
  version: 1,
  profiles: {},
  model_bindings: {},
});

export const cloneVideoPricing = (raw) => {
  const source = asObject(raw);
  return {
    version: source.version ?? 1,
    profiles: Object.fromEntries(
      Object.entries(asObject(source.profiles)).map(([id, profile]) => [
        id,
        { ...asObject(profile) },
      ]),
    ),
    model_bindings: Object.fromEntries(
      Object.entries(asObject(source.model_bindings)).map(
        ([model, binding]) => [model, { ...asObject(binding) }],
      ),
    ),
  };
};

export const normalizeVideoPricing = (raw) => {
  let parsed = raw;
  if (typeof raw === 'string') {
    try {
      parsed = JSON.parse(raw);
    } catch (_) {
      return createEmptyVideoPricing();
    }
  }

  const source = asObject(parsed);
  const profiles = Object.fromEntries(
    Object.entries(asObject(source.profiles))
      .map(([id, profile]) => {
        const normalized = asObject(profile);
        const rawPrice = normalized.unit_price;
        const unitPrice =
          rawPrice === undefined || rawPrice === null || rawPrice === ''
            ? Number.NaN
            : Number(rawPrice);
        return [
          asTrimmedString(id),
          {
            name: asTrimmedString(normalized.name),
            billing_mode: asTrimmedString(
              normalized.billing_mode,
            ).toLowerCase(),
            unit_price: Number.isFinite(unitPrice) ? unitPrice : Number.NaN,
          },
        ];
      })
      .filter(([id]) => Boolean(id)),
  );

  const modelBindings = Object.fromEntries(
    Object.entries(asObject(source.model_bindings))
      .map(([model, binding]) => {
        const normalized = asObject(binding);
        const profile = asTrimmedString(normalized.profile);
        return [
          asTrimmedString(model),
          {
            ...(profile ? { profile } : {}),
            subscription_enabled: normalized.subscription_enabled === true,
          },
        ];
      })
      .filter(([model]) => Boolean(model)),
  );

  return {
    version: source.version ?? 1,
    profiles,
    model_bindings: modelBindings,
  };
};

export const copyVideoPricingProfile = (
  raw,
  sourceProfileId,
  targetProfileId,
  targetName,
) => {
  const config = cloneVideoPricing(raw);
  const source = config.profiles[sourceProfileId];
  const targetId = asTrimmedString(targetProfileId);
  if (!source || !targetId || config.profiles[targetId]) return config;
  config.profiles[targetId] = {
    ...source,
    name: asTrimmedString(targetName) || source.name,
  };
  return config;
};

export const deleteVideoPricingProfile = (raw, profileId) => {
  const config = cloneVideoPricing(raw);
  delete config.profiles[profileId];
  Object.entries(config.model_bindings).forEach(([model, binding]) => {
    if (binding.profile !== profileId) return;
    if (binding.subscription_enabled === true) {
      config.model_bindings[model] = { subscription_enabled: true };
    } else {
      delete config.model_bindings[model];
    }
  });
  return config;
};

export const bindVideoPricingModels = (
  raw,
  models,
  profileId,
  subscriptionEnabled = false,
) => {
  const config = cloneVideoPricing(raw);
  if (!config.profiles[profileId]) return config;
  [...new Set(Array.isArray(models) ? models : [])].forEach((model) => {
    const name = asTrimmedString(model);
    if (name) {
      config.model_bindings[name] = {
        profile: profileId,
        subscription_enabled: subscriptionEnabled === true,
      };
    }
  });
  return config;
};

export const bindVideoPricingPolicyModels = (raw, models) => {
  const config = cloneVideoPricing(raw);
  [...new Set(Array.isArray(models) ? models : [])].forEach((model) => {
    const name = asTrimmedString(model);
    if (name) {
      config.model_bindings[name] = { subscription_enabled: true };
    }
  });
  return config;
};

export const calculateVideoPricingPreview = ({
  profile,
  seconds,
  groupRatio = 1,
}) => {
  const duration = Number(seconds);
  const ratio = Number(groupRatio);
  const unitPrice = Number(profile?.unit_price);
  if (
    !Number.isInteger(duration) ||
    duration <= 0 ||
    !Number.isFinite(ratio) ||
    ratio < 0 ||
    !Number.isFinite(unitPrice) ||
    unitPrice < 0
  ) {
    return null;
  }
  return {
    seconds: duration,
    unit_price: unitPrice,
    subtotal: unitPrice * duration,
    group_ratio: ratio,
    total: unitPrice * duration * ratio,
  };
};

export const validateVideoPricing = (raw, t = fallbackTranslate) => {
  const source = typeof raw === 'string' ? normalizeVideoPricing(raw) : raw;
  const config = normalizeVideoPricing(source);
  const errors = [];

  if (Number(config.version) !== 1) {
    errors.push(t('视频按秒计价配置版本必须为 1'));
  }

  Object.entries(config.profiles).forEach(([profileId, profile]) => {
    const prefix = t('模板 {{name}}', { name: profile.name || profileId });
    if (!profile.name) errors.push(t('模板名称不能为空'));
    if (profile.billing_mode !== VIDEO_PRICING_MODE) {
      errors.push(t('{{prefix}} 的计费模式必须为 per_second', { prefix }));
    }
    if (!Number.isFinite(profile.unit_price) || profile.unit_price < 0) {
      errors.push(t('{{prefix}} 的每秒单价必须是有限的非负数', { prefix }));
    }
  });

  Object.entries(config.model_bindings).forEach(([model, binding]) => {
    if (binding.profile && !config.profiles[binding.profile]) {
      errors.push(t('模型 {{model}} 绑定的视频计价模板不存在', { model }));
    }
    if (!binding.profile && binding.subscription_enabled !== true) {
      errors.push(t('模型 {{model}} 的策略绑定必须允许订阅扣费', { model }));
    }
  });

  return errors;
};

export const getVideoPricingLogSummary = (other) => {
  if (!other || typeof other !== 'object') return null;
  const snapshot = other.video_pricing_snapshot || other.video_pricing;
  if (!snapshot || typeof snapshot !== 'object' || Array.isArray(snapshot)) {
    return null;
  }
  const seconds = firstFiniteNumber(
    snapshot.seconds,
    snapshot.effective_seconds,
    snapshot.requested_seconds,
  );
  const unitPrice = firstFiniteNumber(
    snapshot.unit_price,
    snapshot.unit_price_usd,
  );
  const groupRatio = firstFiniteNumber(
    snapshot.group_ratio,
    other.user_group_ratio !== -1 ? other.user_group_ratio : null,
    other.group_ratio,
    1,
  );
  const subtotal = firstFiniteNumber(
    snapshot.subtotal,
    unitPrice !== null && seconds !== null ? unitPrice * seconds : null,
  );
  const total = firstFiniteNumber(
    snapshot.total,
    snapshot.total_usd,
    subtotal !== null && groupRatio !== null ? subtotal * groupRatio : null,
  );
  const audit = asObject(other.video_execution_audit);
  const reportedDurationMs = firstFiniteNumber(audit.reported_duration_ms);

  return {
    public_model: asTrimmedString(snapshot.public_model),
    profile_id: asTrimmedString(snapshot.profile_id),
    billing_mode: asTrimmedString(snapshot.billing_mode),
    basis: asTrimmedString(snapshot.basis),
    seconds,
    unit_price: unitPrice,
    subtotal,
    group_ratio: groupRatio ?? 1,
    total,
    final_quota: firstFiniteNumber(snapshot.final_quota),
    subscription_enabled: snapshot.subscription_enabled === true,
    reported_duration_ms: reportedDurationMs,
    matches_request:
      typeof audit.matches_request === 'boolean'
        ? audit.matches_request
        : reportedDurationMs !== null && seconds !== null
          ? reportedDurationMs === seconds * 1000
          : null,
  };
};
