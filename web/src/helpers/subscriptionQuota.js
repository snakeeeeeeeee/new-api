export const normalizeSubscriptionQuotaSummary = (summary = {}) => {
  const activeCount = Number(summary?.active_count || 0);
  const unlimitedCount = Number(summary?.unlimited_count || 0);
  const amountTotal = Number(summary?.amount_total || 0);
  const amountUsed = Number(summary?.amount_used || 0);
  const amountRemain = Number(summary?.amount_remain || 0);
  const nextResetTime = Number(summary?.next_reset_time || 0);
  const earliestEndTime = Number(summary?.earliest_end_time || 0);

  return {
    activeCount,
    unlimitedCount,
    amountTotal,
    amountUsed,
    amountRemain,
    nextResetTime,
    earliestEndTime,
    hasActive: activeCount > 0,
    hasUnlimited: unlimitedCount > 0,
    hasLimited: amountTotal > 0,
    limitedCount: Math.max(0, activeCount - unlimitedCount),
    usagePercent:
      amountTotal > 0
        ? Math.min(100, Math.max(0, (amountUsed / amountTotal) * 100))
        : 0,
    remainPercent:
      amountTotal > 0
        ? Math.min(100, Math.max(0, (amountRemain / amountTotal) * 100))
        : 0,
  };
};
