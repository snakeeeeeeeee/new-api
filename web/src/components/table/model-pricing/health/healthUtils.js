export const HEALTH_STATUS_COLORS = {
  healthy: 'green',
  degraded: 'orange',
  failed: 'red',
};

export const HEALTH_HISTORY_COLORS = {
  healthy: '#10b981',
  degraded: '#f59e0b',
  failed: '#f43f5e',
};

export const getHealthHistoryColor = (status) =>
  HEALTH_HISTORY_COLORS[status] || '#cbd5e1';

const normalizeKey = (groupName, modelName) =>
  `${String(groupName || '').trim().toLowerCase()}::${String(modelName || '').trim().toLowerCase()}`;

export const buildHealthChecksMap = (dashboard) => {
  const checks = dashboard?.checks || dashboard?.dashboard?.checks || [];
  const map = {};
  checks.forEach((check) => {
    const key = normalizeKey(check.groupName, check.model || check.name);
    if (!key || key === '::') return;
    map[key] = check;
  });
  return map;
};

export const getHealthCheckForModel = (
  healthChecksMap,
  modelName,
  selectedGroup,
) => {
  if (!healthChecksMap || !modelName || !selectedGroup || selectedGroup === 'all') {
    return null;
  }
  return healthChecksMap[normalizeKey(selectedGroup, modelName)] || null;
};

export const formatCheckedAtLabel = (checkedAt) => {
  if (!checkedAt) return '';
  const ts = new Date(checkedAt).getTime();
  if (Number.isNaN(ts)) return checkedAt;
  const diff = Math.max(0, Math.floor((Date.now() - ts) / 1000));
  if (diff < 60) return `${diff}s`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
  return `${Math.floor(diff / 86400)}d`;
};
