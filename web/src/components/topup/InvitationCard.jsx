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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Avatar,
  Button,
  Card,
  Input,
  Popconfirm,
  Select,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { API, copy, showError, showSuccess } from '../../helpers';
import { Gift, LineChart, Search, Ticket, Zap } from 'lucide-react';
import InviteDetailsModal from './modals/InviteDetailsModal';

const { Text } = Typography;

const isManualInviteCode = (inviteCode) => {
  const prefix = String(inviteCode?.prefix || '').toUpperCase();
  const code = String(inviteCode?.code || '').toUpperCase();
  return (
    Boolean(inviteCode?.is_manual) ||
    prefix === 'MANUAL' ||
    code.startsWith('MANUAL-')
  );
};

const buildInviteCodeState = (inviteCode) => {
  if (!inviteCode) {
    return { label: '未知状态', color: 'grey' };
  }
  if (inviteCode.is_deleted || inviteCode.invite_code_deleted) {
    return { label: '已删除', color: 'grey' };
  }
  if (inviteCode.status === 1 || inviteCode.invite_code_status === 1) {
    return { label: '已启用', color: 'green' };
  }
  return { label: '已禁用', color: 'orange' };
};

const InviteCodeStatusTag = ({ inviteCode, t }) => {
  const state = buildInviteCodeState(inviteCode);
  return (
    <Tag size='small' shape='circle' color={state.color}>
      {t(state.label)}
    </Tag>
  );
};

const renderUSDStatsAmount = (value) => {
  const amount = Number(value) || 0;
  return `$${amount.toFixed(2)}`;
};

const quotaToUSD = (quota) => {
  const quotaPerUnit = Number(localStorage.getItem('quota_per_unit') || 1);
  if (!quotaPerUnit || quotaPerUnit <= 0) {
    return 0;
  }
  return (Number(quota) || 0) / quotaPerUnit;
};

const renderQuotaAsUSD = (quota) => renderUSDStatsAmount(quotaToUSD(quota));

const getRechargeUSD = (stats) =>
  stats?.recharge_usd ?? stats?.recharge_money ?? 0;

const getConsumeUSD = (stats) =>
  stats?.consume_usd ?? quotaToUSD(stats?.consume_quota);

const StatBlock = ({ label, value }) => (
  <div className='rounded-lg border border-[var(--semi-color-border)] bg-[var(--semi-color-bg-1)] px-4 py-3 min-w-0'>
    <div className='text-lg font-semibold truncate'>{value}</div>
    <Text type='tertiary' className='text-xs'>
      {label}
    </Text>
  </div>
);

const EmptyLine = ({ children }) => (
  <div className='rounded-lg border border-[var(--semi-color-border)] px-4 py-3'>
    <Text type='tertiary'>{children}</Text>
  </div>
);

const SectionHeader = ({ title, count, t }) => (
  <div className='flex flex-col'>
    <Text strong>{title}</Text>
    <Text type='quaternary' className='text-xs'>
      {t('共')} {count || 0} {t('条')}
    </Text>
  </div>
);

const CardActionButton = ({ children, onClick, loading }) => (
  <Button
    size='small'
    theme='borderless'
    loading={loading}
    onClick={onClick}
    className='shrink-0'
  >
    {children}
  </Button>
);

const ListRow = ({ children }) => (
  <div className='rounded-lg bg-[var(--semi-color-fill-0)] px-3 py-2 transition-colors hover:bg-[var(--semi-color-fill-1)]'>
    {children}
  </div>
);

const InlineMeta = ({ children }) => (
  <Text type='tertiary' className='text-xs leading-5'>
    {children}
  </Text>
);

const TableWrap = ({ children }) => (
  <div className='overflow-hidden rounded-lg border border-[var(--semi-color-border)]'>
    {children}
  </div>
);

const CardTitleBar = ({ title, extra }) => (
  <div className='flex items-start justify-between gap-4 w-full'>
    <div className='min-w-0'>{title}</div>
    {extra && (
      <div className='flex shrink-0 items-center justify-end gap-2'>
        {extra}
      </div>
    )}
  </div>
);

const SectionShell = ({ title, description, action, children }) => (
  <section className='space-y-3'>
    <div className='flex flex-col sm:flex-row sm:items-end sm:justify-between gap-2'>
      <div>
        <Typography.Title heading={5} className='!mb-1'>
          {title}
        </Typography.Title>
        {description && (
          <Text type='tertiary' className='text-sm'>
            {description}
          </Text>
        )}
      </div>
      {action}
    </div>
    {children}
  </section>
);

const SurfaceCard = ({ title, extra, children, className = '' }) => (
  <Card
    className={`!rounded-xl w-full shadow-sm ${className}`}
    title={<CardTitleBar title={title} extra={extra} />}
  >
    {children}
  </Card>
);

const matchesInviteeKeyword = (invitee, keyword) => {
  const text = [
    invitee?.username,
    invitee?.invite_code,
    invitee?.group,
    invitee?.user_id,
  ]
    .filter((item) => item !== undefined && item !== null)
    .join(' ')
    .toLowerCase();
  return text.includes(keyword);
};

const InvitationCard = ({
  t,
  userState,
  renderQuota,
  setOpenTransfer,
  inviteRegisterBaseUrl,
  refreshUser,
  pageMode = false,
}) => {
  const [showInviteCodesModal, setShowInviteCodesModal] = useState(false);
  const [showInviteesModal, setShowInviteesModal] = useState(false);
  const [agentStats, setAgentStats] = useState(null);
  const [agentStatsLoading, setAgentStatsLoading] = useState(false);
  const [agentStatsPeriod, setAgentStatsPeriod] = useState('day');
  const [enablingInviteeId, setEnablingInviteeId] = useState(0);
  const [inviteeSearchKeyword, setInviteeSearchKeyword] = useState('');

  const inviteCodesPreview = userState?.user?.invite_codes || [];
  const inviteesPreview = userState?.user?.invitees || [];
  const inviteCodeCount =
    userState?.user?.invite_code_count || inviteCodesPreview.length || 0;
  const inviteeCount =
    userState?.user?.invite_user_count || inviteesPreview.length || 0;
  const boundInviteCode = userState?.user?.bound_invite_code;
  const inviteTotalRechargeMoney =
    userState?.user?.invite_total_recharge_money ??
    userState?.user?.invite_total_recharge ??
    0;
  const inviteTotalConsume = userState?.user?.invite_total_consume || 0;
  const inviteAgentLevel = userState?.user?.invite_agent_level || 0;
  const canGrantInvitation = Boolean(userState?.user?.can_grant_invitation);
  const hasAgentStatsAccess = inviteAgentLevel > 0;
  const agentTrend = agentStats?.direct_trend || [];
  const hasAgentTrendData = agentTrend.some(
    (point) =>
      (point?.recharge_usd || point?.recharge_money || 0) > 0 ||
      (point?.consume_usd || point?.consume_quota || 0) > 0,
  );
  const secondLevelStats = agentStats?.second_level_stats || [];
  const normalizedInviteeSearchKeyword = inviteeSearchKeyword
    .trim()
    .toLowerCase();
  const recentInviteesForGrant = canGrantInvitation
    ? inviteesPreview
        .filter(
          (invitee) =>
            !normalizedInviteeSearchKeyword ||
            matchesInviteeKeyword(invitee, normalizedInviteeSearchKeyword),
        )
        .slice(0, 6)
    : [];
  const activeInviteeCount = inviteesPreview.filter(
    (invitee) => invitee.invitation_enabled,
  ).length;
  const inviteOverviewMetrics = [
    {
      label: t('邀请人数'),
      value: inviteeCount,
    },
    {
      label: t('实付金额'),
      value: renderUSDStatsAmount(inviteTotalRechargeMoney),
    },
    {
      label: t('消费等额'),
      value: renderQuotaAsUSD(inviteTotalConsume),
    },
    {
      label: t('已开启邀请功能'),
      value: activeInviteeCount,
    },
  ];

  const buildInviteRegisterLink = (code) => {
    const baseUrl = (
      inviteRegisterBaseUrl ||
      window.location.origin ||
      ''
    ).replace(/\/$/, '');
    return `${baseUrl}/register?invite_code=${encodeURIComponent(code || '')}`;
  };

  const handleCopyInviteLink = async (code) => {
    const link = buildInviteRegisterLink(code);
    if (await copy(link)) {
      showSuccess(t('邀请链接已复制到剪贴板'));
    }
  };

  const loadAgentStats = useCallback(async () => {
    if (!hasAgentStatsAccess) {
      setAgentStats(null);
      return;
    }
    setAgentStatsLoading(true);
    try {
      const res = await API.get(
        `/api/user/self/invite_agent_stats?period=${agentStatsPeriod}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        setAgentStats(data || null);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setAgentStatsLoading(false);
    }
  }, [agentStatsPeriod, hasAgentStatsAccess]);

  useEffect(() => {
    loadAgentStats();
  }, [loadAgentStats]);

  const enableInviteeInvitation = async (invitee) => {
    const inviteeId = invitee?.user_id || 0;
    if (!inviteeId) return;
    setEnablingInviteeId(inviteeId);
    try {
      const res = await API.post(
        `/api/user/self/invitees/${inviteeId}/enable_invitation`,
      );
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('邀请功能已开启'));
        await Promise.all([loadAgentStats(), refreshUser?.()]);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setEnablingInviteeId(0);
    }
  };

  const buildTrendSpec = useCallback(
    (trend = []) => ({
      type: 'common',
      data: [
        {
          id: 'trend',
          values: trend.flatMap((point) => [
            {
              label: point.label,
              value: getRechargeUSD(point),
              type: t('实付金额'),
            },
            {
              label: point.label,
              value: getConsumeUSD(point),
              type: t('消费等额'),
            },
          ]),
        },
      ],
      series: [
        {
          type: 'line',
          xField: 'label',
          yField: 'value',
          seriesField: 'type',
          point: { visible: false },
          line: { style: { lineWidth: 2 } },
        },
      ],
      axes: [
        { orient: 'bottom', type: 'band' },
        { orient: 'left', type: 'linear' },
      ],
      legends: { visible: true, orient: 'bottom' },
      tooltip: { visible: true },
    }),
    [t],
  );

  const inviteCodeColumns = useMemo(
    () => [
      {
        title: t('邀请码'),
        dataIndex: 'code',
        key: 'code',
        render: (_, record) => (
          <div className='flex flex-wrap items-center gap-2'>
            <span className='font-semibold'>{record.code}</span>
            <InviteCodeStatusTag inviteCode={record} t={t} />
            {isManualInviteCode(record) && (
              <Tag size='small' shape='circle' color='blue'>
                {t('手动绑定码')}
              </Tag>
            )}
          </div>
        ),
      },
      {
        title: t('目标分组'),
        dataIndex: 'target_group',
        key: 'target_group',
      },
      {
        title: t('单次赠送'),
        dataIndex: 'reward_quota_per_use',
        key: 'reward_quota_per_use',
        render: (value) => renderQuota(value || 0),
      },
      {
        title: t('赠送次数'),
        key: 'reward_uses',
        render: (_, record) =>
          `${record.reward_used_uses || 0}/${record.reward_total_uses || 0}`,
      },
      {
        title: t('剩余可赠送'),
        dataIndex: 'remaining_reward_uses',
        key: 'remaining_reward_uses',
      },
      {
        title: t('邀请人数'),
        dataIndex: 'invited_user_count',
        key: 'invited_user_count',
      },
      {
        title: t('实付金额'),
        dataIndex: 'invite_total_recharge_money',
        key: 'invite_total_recharge_money',
        render: (value, record) =>
          renderUSDStatsAmount(value ?? record.invite_total_recharge ?? 0),
      },
      {
        title: t('消费等额'),
        dataIndex: 'invite_total_consume',
        key: 'invite_total_consume',
        render: (value) => renderQuotaAsUSD(value || 0),
      },
      {
        title: t('邀请链接'),
        key: 'invite_link',
        render: (_, record) =>
          isManualInviteCode(record) ? (
            <Text type='tertiary'>{t('仅用于后台归属统计')}</Text>
          ) : (
            <Button
              size='small'
              theme='borderless'
              onClick={() => handleCopyInviteLink(record.code)}
            >
              {t('复制邀请链接')}
            </Button>
          ),
      },
    ],
    [inviteRegisterBaseUrl, renderQuota, t],
  );

  const inviteeColumns = useMemo(
    () => [
      {
        title: t('用户名'),
        dataIndex: 'username',
        key: 'username',
        width: 140,
      },
      {
        title: t('邀请码'),
        dataIndex: 'invite_code',
        key: 'invite_code',
        render: (value) => value || '-',
        width: 150,
      },
      {
        title: t('分组'),
        dataIndex: 'group',
        key: 'group',
        width: 100,
      },
      {
        title: t('实付金额'),
        dataIndex: 'invite_total_recharge_money',
        key: 'invite_total_recharge_money',
        render: (value, record) =>
          renderUSDStatsAmount(value ?? record.invite_total_recharge ?? 0),
        width: 100,
      },
      {
        title: t('消费等额'),
        dataIndex: 'invite_total_consume',
        key: 'invite_total_consume',
        render: (value) => renderQuotaAsUSD(value || 0),
        width: 100,
      },
      {
        title: t('邀请功能'),
        key: 'invitation_enabled',
        width: 140,
        render: (_, record) => {
          if (record.invitation_enabled) {
            return (
              <Tag size='small' shape='circle' color='green'>
                {t('已开启')}
              </Tag>
            );
          }
          if (!canGrantInvitation) {
            return (
              <Tag size='small' shape='circle'>
                {t('未开启')}
              </Tag>
            );
          }
          return (
            <Popconfirm
              title={t('确定给该用户开启邀请功能？')}
              content={t('系统会自动生成一个零奖励的邀请码。')}
              onConfirm={() => enableInviteeInvitation(record)}
            >
              <Button
                size='small'
                theme='borderless'
                loading={enablingInviteeId === record.user_id}
              >
                {t('开启')}
              </Button>
            </Popconfirm>
          );
        },
      },
    ],
    [
      canGrantInvitation,
      enablingInviteeId,
      t,
    ],
  );

  const boundInviteCodeHint = (() => {
    if (!boundInviteCode) {
      return '';
    }
    if (boundInviteCode.is_deleted) {
      return t('该邀请码已删除，但您的历史归属关系仍会保留。');
    }
    if (boundInviteCode.status !== 1) {
      return t('该邀请码已禁用，但您的历史归属关系仍会保留。');
    }
    return t('该邀请码当前正常，用于识别您的归属来源。');
  })();

  const pageHeader = pageMode ? (
    <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3'>
      <div>
        <Typography.Title heading={3} className='!mb-1'>
          {t('邀请管理')}
        </Typography.Title>
        <Text type='tertiary'>
          {t('管理邀请码、被邀请人和邀请数据')}
        </Text>
      </div>
      <Button
        size='small'
        theme='borderless'
        loading={agentStatsLoading}
        onClick={() => {
          loadAgentStats();
          refreshUser?.();
        }}
      >
        {t('刷新')}
      </Button>
    </div>
  ) : (
    <div className='flex items-center'>
      <Avatar size='small' color='green' className='mr-3 shadow-md'>
        <Gift size={16} />
      </Avatar>
      <div>
        <Typography.Text className='text-lg font-medium'>
          {t('邀请统计')}
        </Typography.Text>
        <div className='text-xs text-[var(--semi-color-text-2)]'>
          {t('查看归属到您的邀请码带来的用户数据')}
        </div>
      </div>
    </div>
  );

  const overviewAndTrend = (
    <SurfaceCard
      title={<Text strong>{t('概览与趋势')}</Text>}
      extra={
        hasAgentStatsAccess ? (
          <Select
            size='small'
            value={agentStatsPeriod}
            onChange={setAgentStatsPeriod}
            style={{ width: 104 }}
          >
            <Select.Option value='day'>{t('按天')}</Select.Option>
            <Select.Option value='month'>{t('按月')}</Select.Option>
          </Select>
        ) : null
      }
    >
      <div className='grid grid-cols-2 lg:grid-cols-4 gap-3'>
        {inviteOverviewMetrics.map((metric) => (
          <StatBlock
            key={metric.label}
            label={metric.label}
            value={metric.value}
          />
        ))}
      </div>
      <div className='mt-3 flex flex-wrap gap-2'>
        <Tag
          size='small'
          shape='circle'
          color={inviteAgentLevel ? 'green' : 'grey'}
        >
          {inviteAgentLevel ? t('邀请功能已开启') : t('邀请功能未开启')}
        </Tag>
        {canGrantInvitation && (
          <Tag size='small' shape='circle' color='blue'>
            {t('可给被邀请人开启邀请码')}
          </Tag>
        )}
      </div>

      {hasAgentStatsAccess && (
        <div className='mt-5 border-t border-[var(--semi-color-border)] pt-4'>
          <div className='mb-3 flex items-center gap-2'>
            <LineChart size={16} />
            <Text strong>{t('邀请趋势')}</Text>
          </div>
          {agentStatsLoading && !agentStats ? (
            <div className='h-72 flex items-center justify-center'>
              <Text type='tertiary'>{t('统计加载中')}</Text>
            </div>
          ) : hasAgentTrendData ? (
            <div className='h-72'>
              <VChart
                spec={buildTrendSpec(agentTrend)}
                option={{ mode: 'desktop-browser' }}
              />
            </div>
          ) : (
            <div className='h-40 flex items-center justify-center rounded-lg border border-[var(--semi-color-border)]'>
              <Text type='tertiary'>{t('暂无邀请统计数据')}</Text>
            </div>
          )}
        </div>
      )}
    </SurfaceCard>
  );

  const renderInviteeInvitationAction = (invitee, buttonTheme = 'borderless') => {
    if (invitee.invitation_enabled) {
      return (
        <Tag size='small' shape='circle' color='green'>
          {t('已开启')}
        </Tag>
      );
    }
    if (!canGrantInvitation) {
      return (
        <Tag size='small' shape='circle'>
          {t('未开启')}
        </Tag>
      );
    }
    return (
      <Popconfirm
        title={t('确定给该用户开启邀请功能？')}
        content={t('系统会自动生成一个零奖励的邀请码。')}
        onConfirm={() => enableInviteeInvitation(invitee)}
      >
        <Button
          size='small'
          type={buttonTheme === 'solid' ? 'primary' : 'tertiary'}
          theme={buttonTheme}
          loading={enablingInviteeId === invitee.user_id}
          className='shrink-0'
        >
          {t('开启邀请功能')}
        </Button>
      </Popconfirm>
    );
  };

  const grantInvitationSection = canGrantInvitation && (
    <SectionShell
      title={t('开通邀请功能')}
      description={t('从被邀请人中选择需要继续邀请用户的人，可搜索后开启。')}
    >
      <SurfaceCard
        title={
          <SectionHeader title={t('最近被邀请人')} count={inviteeCount} t={t} />
        }
        extra={
          inviteeCount > 0 ? (
            <CardActionButton onClick={() => setShowInviteesModal(true)}>
              {t('查看全部')}
            </CardActionButton>
          ) : null
        }
      >
        <div className='space-y-3'>
          <Input
            prefix={<Search size={14} />}
            value={inviteeSearchKeyword}
            onChange={setInviteeSearchKeyword}
            placeholder={t('搜索用户名、邀请码或分组')}
            showClear
          />

          {recentInviteesForGrant.length > 0 ? (
            <div className='space-y-2'>
              {recentInviteesForGrant.map((invitee) => (
                <ListRow key={invitee.user_id}>
                  <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3'>
                    <div className='min-w-0'>
                      <div className='font-semibold truncate'>
                        {invitee.username}
                      </div>
                      <InlineMeta>
                        {t('邀请码')} {invitee.invite_code || '-'} ·{' '}
                        {invitee.group || '-'}
                      </InlineMeta>
                    </div>
                    <div className='flex flex-wrap items-center gap-2 text-xs'>
                      <Tag size='small' shape='circle'>
                        {t('实付金额')}{' '}
                        {renderUSDStatsAmount(
                          invitee.invite_total_recharge_money ??
                            invitee.invite_total_recharge ??
                            0,
                        )}
                      </Tag>
                      <Tag size='small' shape='circle'>
                        {t('消费等额')}{' '}
                        {renderQuotaAsUSD(invitee.invite_total_consume || 0)}
                      </Tag>
                      {renderInviteeInvitationAction(invitee, 'solid')}
                    </div>
                  </div>
                </ListRow>
              ))}
            </div>
          ) : (
            <EmptyLine>
              {normalizedInviteeSearchKeyword
                ? t('没有匹配的被邀请人。')
                : t('暂无被邀请人记录。')}
            </EmptyLine>
          )}
        </div>
      </SurfaceCard>
    </SectionShell>
  );

  const inviteeSummaryCard = (
    <SurfaceCard
      title={<SectionHeader title={t('被邀请人')} count={inviteeCount} t={t} />}
      extra={
        inviteeCount > 0 ? (
          <CardActionButton onClick={() => setShowInviteesModal(true)}>
            {t('查看全部')}
          </CardActionButton>
        ) : null
      }
    >
      {inviteesPreview.length > 0 ? (
        <div className='grid grid-cols-1 md:grid-cols-3 gap-3'>
          <div className='rounded-lg bg-[var(--semi-color-fill-0)] px-4 py-3'>
            <div className='text-lg font-semibold'>{inviteeCount}</div>
            <Text type='tertiary' className='text-xs'>
              {t('累计被邀请人')}
            </Text>
          </div>
          <div className='rounded-lg bg-[var(--semi-color-fill-0)] px-4 py-3'>
            <div className='text-lg font-semibold'>{activeInviteeCount}</div>
            <Text type='tertiary' className='text-xs'>
              {t('已开启邀请功能')}
            </Text>
          </div>
          <div className='rounded-lg bg-[var(--semi-color-fill-0)] px-4 py-3'>
            <div className='text-lg font-semibold'>
              {Math.max(inviteeCount - activeInviteeCount, 0)}
            </div>
            <Text type='tertiary' className='text-xs'>
              {t('未开启邀请功能')}
            </Text>
          </div>
        </div>
      ) : (
        <EmptyLine>{t('暂无被邀请人记录。')}</EmptyLine>
      )}
    </SurfaceCard>
  );

  const inviteeListCard = canGrantInvitation ? inviteeSummaryCard : (
    <SurfaceCard
      title={
        <SectionHeader
          title={t('被邀请人')}
          count={inviteeCount}
          t={t}
        />
      }
      extra={
        inviteeCount > 0 ? (
          <CardActionButton onClick={() => setShowInviteesModal(true)}>
            {t('查看全部')}
          </CardActionButton>
        ) : null
      }
    >
      {inviteesPreview.length > 0 ? (
        <div className='space-y-2'>
          {inviteesPreview.slice(0, 5).map((invitee) => (
            <ListRow key={invitee.user_id}>
              <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3'>
                <div className='min-w-0'>
                  <div className='font-semibold truncate'>{invitee.username}</div>
                  <InlineMeta>
                    {t('邀请码')} {invitee.invite_code || '-'} ·{' '}
                    {invitee.group || '-'}
                  </InlineMeta>
                </div>
                <div className='flex flex-wrap items-center gap-2 text-xs'>
                  <Tag size='small' shape='circle'>
                    {t('实付金额')}{' '}
                    {renderUSDStatsAmount(
                      invitee.invite_total_recharge_money ??
                        invitee.invite_total_recharge ??
                        0,
                    )}
                  </Tag>
                  <Tag size='small' shape='circle'>
                    {t('消费等额')}{' '}
                    {renderQuotaAsUSD(invitee.invite_total_consume || 0)}
                  </Tag>
                  {renderInviteeInvitationAction(invitee)}
                </div>
              </div>
            </ListRow>
          ))}
        </div>
      ) : (
        <EmptyLine>{t('暂无被邀请人记录。')}</EmptyLine>
      )}
    </SurfaceCard>
  );

  const secondLevelStatsSection = canGrantInvitation && (
    <SurfaceCard
      title={<Text strong>{t('被邀请人邀请统计')}</Text>}
      extra={
        <CardActionButton
          loading={agentStatsLoading}
          onClick={loadAgentStats}
        >
          {t('刷新统计')}
        </CardActionButton>
      }
    >
      {secondLevelStats.length > 0 || agentStatsLoading ? (
        <TableWrap>
          <Table
            size='small'
            rowKey='user_id'
            loading={agentStatsLoading}
            dataSource={secondLevelStats}
            pagination={false}
            scroll={{ x: 'max-content' }}
            columns={[
              {
                title: t('用户'),
                key: 'user',
                dataIndex: 'username',
                render: (value, record) => `#${record.user_id} ${value}`,
              },
              {
                title: t('邀请码'),
                key: 'invite_code',
                dataIndex: 'invite_code',
              },
              {
                title: t('被邀请人充值'),
                key: 'self_recharge_amount',
                dataIndex: 'self_stats',
                render: (value) => renderUSDStatsAmount(getRechargeUSD(value)),
              },
              {
                title: t('被邀请人消费'),
                key: 'self_consume_quota',
                dataIndex: 'self_stats',
                render: (value) => renderUSDStatsAmount(getConsumeUSD(value)),
              },
              {
                title: t('其邀请用户充值'),
                key: 'invitee_recharge_amount',
                dataIndex: 'invitee_stats',
                render: (value) => renderUSDStatsAmount(getRechargeUSD(value)),
              },
              {
                title: t('其邀请用户消费'),
                key: 'invitee_consume_quota',
                dataIndex: 'invitee_stats',
                render: (value) => renderUSDStatsAmount(getConsumeUSD(value)),
              },
            ]}
          />
        </TableWrap>
      ) : (
        <EmptyLine>{t('暂无被邀请人邀请数据')}</EmptyLine>
      )}
    </SurfaceCard>
  );

  const inviteCodeCard = (
    <SurfaceCard
      title={
        <SectionHeader
          title={t('我名下的邀请码')}
          count={inviteCodeCount}
          t={t}
        />
      }
      extra={
        inviteCodeCount > 0 ? (
          <CardActionButton onClick={() => setShowInviteCodesModal(true)}>
            {t('查看全部')}
          </CardActionButton>
        ) : null
      }
    >
      {inviteCodesPreview.length > 0 ? (
        <TableWrap>
          <Table
            size='small'
            rowKey={(record) => record.id || record.code}
            dataSource={inviteCodesPreview}
            pagination={false}
            scroll={{ x: 'max-content' }}
            columns={inviteCodeColumns}
          />
        </TableWrap>
      ) : (
        <EmptyLine>{t('您当前还没有绑定的邀请码。')}</EmptyLine>
      )}
    </SurfaceCard>
  );

  const boundInviteCodeCard = boundInviteCode && (
    <SurfaceCard title={<Text strong>{t('当前绑定的邀请码')}</Text>}>
      <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3'>
        <div className='min-w-0'>
          <div className='flex flex-wrap items-center gap-2'>
            <Ticket size={16} className='text-[var(--semi-color-primary)]' />
            <span className='font-semibold'>{boundInviteCode.code}</span>
            <InviteCodeStatusTag inviteCode={boundInviteCode} t={t} />
          </div>
          <Text type='tertiary' className='text-xs'>
            {boundInviteCodeHint}
          </Text>
        </div>
        <div className='flex flex-wrap gap-2 text-xs'>
          <Tag size='small' shape='circle'>
            {t('归属用户')} {boundInviteCode.owner_username || '-'}
          </Tag>
          <Tag size='small' shape='circle'>
            {t('目标分组')} {boundInviteCode.target_group || '-'}
          </Tag>
        </div>
      </div>
    </SurfaceCard>
  );

  const historicalRewardCard = userState?.user?.aff_quota > 0 && (
    <SurfaceCard title={<Text strong>{t('历史奖励额度')}</Text>}>
      <div className='flex items-center justify-between gap-4'>
        <div>
          <div className='text-lg font-semibold'>
            {renderQuota(userState?.user?.aff_quota || 0)}
          </div>
          <Text type='tertiary' className='text-sm'>
            {t('可划转到余额的旧邀请奖励')}
          </Text>
        </div>
        <Button
          type='primary'
          theme='solid'
          onClick={() => setOpenTransfer(true)}
          className='!rounded-lg'
        >
          <Zap size={12} className='mr-1' />
          {t('划转到余额')}
        </Button>
      </div>
    </SurfaceCard>
  );

  const invitationContent = (
    <div className='space-y-7'>
      <SectionShell
        title={t('数据表现')}
        description={t('先看整体数据，再看被邀请人带来的延伸数据。')}
      >
        <div className='space-y-4'>
          {overviewAndTrend}
          {secondLevelStatsSection}
        </div>
      </SectionShell>

      {grantInvitationSection}

      <SectionShell
        title={t('邀请资产')}
        description={t('查看被邀请人概况、邀请码和当前绑定关系。')}
      >
        <div className='space-y-4'>
          {inviteeListCard}
          {inviteCodeCard}
          {(boundInviteCodeCard || historicalRewardCard) && (
            <div className='grid grid-cols-1 xl:grid-cols-2 gap-4'>
              {boundInviteCodeCard}
              {historicalRewardCard}
            </div>
          )}
        </div>
      </SectionShell>

      <SectionShell title={t('说明')}>
        <div className='rounded-xl border border-[var(--semi-color-border)] bg-[var(--semi-color-bg-1)] px-4 py-3 text-sm text-[var(--semi-color-text-2)] space-y-2'>
          <div>{t('邀请统计仅包含新邀请码体系的数据。')}</div>
          <div>
            {t('实付金额不包含赠送额度。消费等额按当前额度汇率换算，不区分额度来源。')}
          </div>
        </div>
      </SectionShell>
    </div>
  );

  return (
    <>
      {pageMode ? (
        <div className='space-y-6'>
          {pageHeader}
          {invitationContent}
        </div>
      ) : (
        <Card className='!rounded-2xl shadow-sm border-0'>
          <div className='space-y-5'>
            {pageHeader}
            {invitationContent}
          </div>
        </Card>
      )}
      <InviteDetailsModal
        visible={showInviteCodesModal}
        onCancel={() => setShowInviteCodesModal(false)}
        t={t}
        title={t('我名下的邀请码')}
        endpoint='/api/user/self/invite_codes'
        columns={inviteCodeColumns}
        emptyText={t('暂无邀请码记录')}
      />
      <InviteDetailsModal
        visible={showInviteesModal}
        onCancel={() => setShowInviteesModal(false)}
        t={t}
        title={t('全部被邀请人')}
        endpoint='/api/user/self/invitees'
        columns={inviteeColumns}
        emptyText={t('暂无被邀请人记录')}
      />
    </>
  );
};

export default InvitationCard;
