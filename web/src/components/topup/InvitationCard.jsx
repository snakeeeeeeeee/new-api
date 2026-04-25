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

import React, { useMemo, useState } from 'react';
import {
  Avatar,
  Badge,
  Button,
  Card,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { copy, renderPaymentAmount, showSuccess } from '../../helpers';
import {
  BarChart2,
  CreditCard,
  Gift,
  Ticket,
  TrendingUp,
  Users,
  Zap,
} from 'lucide-react';
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

const SectionHeader = ({ title, count, buttonText, onClick, t }) => (
  <div className='flex items-center justify-between gap-3'>
    <div className='flex flex-col'>
      <Text type='tertiary'>{t(title)}</Text>
      <Text type='quaternary' className='text-xs'>
        {t('共')} {count || 0} {t('条')}
      </Text>
    </div>
    {count > 0 && (
      <Button size='small' theme='borderless' onClick={onClick}>
        {t(buttonText)}
      </Button>
    )}
  </div>
);

const InvitationCard = ({
  t,
  userState,
  renderQuota,
  renderQuotaWithAmount,
  setOpenTransfer,
  inviteRegisterBaseUrl,
}) => {
  const [showInviteCodesModal, setShowInviteCodesModal] = useState(false);
  const [showInviteesModal, setShowInviteesModal] = useState(false);

  const inviteCodesPreview = userState?.user?.invite_codes || [];
  const inviteesPreview = userState?.user?.invitees || [];
  const inviteCodeCount =
    userState?.user?.invite_code_count || inviteCodesPreview.length || 0;
  const inviteeCount =
    userState?.user?.invite_user_count || inviteesPreview.length || 0;
  const boundInviteCode = userState?.user?.bound_invite_code;
  const inviteTotalRechargeAmount =
    userState?.user?.invite_total_recharge_amount || 0;
  const inviteTotalRechargeMoney =
    userState?.user?.invite_total_recharge_money ??
    userState?.user?.invite_total_recharge ??
    0;
  const inviteTotalConsume = userState?.user?.invite_total_consume || 0;

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
        title: t('邀请充值额度'),
        dataIndex: 'invite_total_recharge_amount',
        key: 'invite_total_recharge_amount',
        render: (value) => renderQuotaWithAmount(value || 0),
      },
      {
        title: t('邀请实付金额'),
        dataIndex: 'invite_total_recharge_money',
        key: 'invite_total_recharge_money',
        render: (value, record) =>
          renderPaymentAmount(value ?? record.invite_total_recharge ?? 0),
      },
      {
        title: t('邀请消费额度'),
        dataIndex: 'invite_total_consume',
        key: 'invite_total_consume',
        render: (value) => renderQuota(value || 0),
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
    [inviteRegisterBaseUrl, renderQuota, renderQuotaWithAmount, t],
  );

  const inviteeColumns = useMemo(
    () => [
      {
        title: t('用户名'),
        dataIndex: 'username',
        key: 'username',
      },
      {
        title: t('邀请码'),
        dataIndex: 'invite_code',
        key: 'invite_code',
        render: (value) => value || '-',
      },
      {
        title: t('分组'),
        dataIndex: 'group',
        key: 'group',
      },
      {
        title: t('充值额度'),
        dataIndex: 'invite_total_recharge_amount',
        key: 'invite_total_recharge_amount',
        render: (value) => renderQuotaWithAmount(value || 0),
      },
      {
        title: t('实付金额'),
        dataIndex: 'invite_total_recharge_money',
        key: 'invite_total_recharge_money',
        render: (value, record) =>
          renderPaymentAmount(value ?? record.invite_total_recharge ?? 0),
      },
      {
        title: t('消费额度'),
        dataIndex: 'invite_total_consume',
        key: 'invite_total_consume',
        render: (value) => renderQuota(value || 0),
      },
    ],
    [renderQuota, renderQuotaWithAmount, t],
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

  return (
    <>
      <Card className='!rounded-2xl shadow-sm border-0'>
        <div className='flex items-center mb-4'>
          <Avatar size='small' color='green' className='mr-3 shadow-md'>
            <Gift size={16} />
          </Avatar>
          <div>
            <Typography.Text className='text-lg font-medium'>
              {t('邀请统计')}
            </Typography.Text>
            <div className='text-xs'>
              {t('查看归属到您的邀请码带来的用户数据')}
            </div>
          </div>
        </div>

        <Space vertical style={{ width: '100%' }}>
          <Card
            className='!rounded-xl w-full'
            cover={
              <div
                className='relative min-h-[168px]'
                style={{
                  '--palette-primary-darkerChannel': '0 75 80',
                  backgroundImage: `linear-gradient(0deg, rgba(var(--palette-primary-darkerChannel) / 80%), rgba(var(--palette-primary-darkerChannel) / 80%)), url('/cover-4.webp')`,
                  backgroundSize: 'cover',
                  backgroundPosition: 'center',
                  backgroundRepeat: 'no-repeat',
                }}
              >
                <div className='relative z-10 h-full flex flex-col justify-between p-4'>
                  <div className='flex justify-between items-center'>
                    <Text strong style={{ color: 'white', fontSize: '16px' }}>
                      {t('邀请统计')}
                    </Text>
                  </div>

                  <div className='grid grid-cols-2 md:grid-cols-4 gap-4 mt-4'>
                    {[
                      {
                        label: t('邀请充值额度'),
                        value: renderQuotaWithAmount(inviteTotalRechargeAmount),
                        icon: TrendingUp,
                      },
                      {
                        label: t('邀请实付金额'),
                        value: renderPaymentAmount(inviteTotalRechargeMoney),
                        icon: CreditCard,
                      },
                      {
                        label: t('邀请消费额度'),
                        value: renderQuota(inviteTotalConsume),
                        icon: BarChart2,
                      },
                      {
                        label: t('邀请人数'),
                        value: inviteeCount,
                        icon: Users,
                      },
                    ].map((metric) => {
                      const Icon = metric.icon;
                      return (
                        <div className='text-center min-w-0' key={metric.label}>
                          <div
                            className='text-base sm:text-xl font-bold mb-2 truncate'
                            style={{ color: 'white' }}
                          >
                            {metric.value}
                          </div>
                          <div className='flex items-center justify-center text-sm'>
                            <Icon
                              size={14}
                              className='mr-1 flex-shrink-0'
                              style={{ color: 'rgba(255,255,255,0.8)' }}
                            />
                            <Text
                              ellipsis={{ showTooltip: true }}
                              style={{
                                color: 'rgba(255,255,255,0.8)',
                                fontSize: '12px',
                              }}
                            >
                              {metric.label}
                            </Text>
                          </div>
                        </div>
                      );
                    })}
                  </div>
                </div>
              </div>
            }
          >
            <div className='space-y-2'>
              <Text type='tertiary' className='text-sm'>
                {t('该区域统计的是通过管理员邀请码归属到您名下的新用户数据。')}
              </Text>
            </div>
          </Card>

          {boundInviteCode && (
            <Card
              className='!rounded-xl w-full'
              title={<Text type='tertiary'>{t('当前绑定的邀请码')}</Text>}
            >
              <div className='rounded-lg border border-[var(--semi-color-border)] p-4 space-y-3'>
                <div className='flex flex-wrap items-center gap-2'>
                  <Ticket
                    size={16}
                    className='text-[var(--semi-color-primary)]'
                  />
                  <span className='font-semibold text-base'>
                    {boundInviteCode.code}
                  </span>
                  <InviteCodeStatusTag inviteCode={boundInviteCode} t={t} />
                </div>
                <div className='flex flex-wrap gap-2 text-xs'>
                  <Tag size='small' shape='circle'>
                    {t('归属用户')} {boundInviteCode.owner_username || '-'}
                  </Tag>
                  <Tag size='small' shape='circle'>
                    {t('目标分组')} {boundInviteCode.target_group || '-'}
                  </Tag>
                </div>
                <Text type='tertiary' className='text-xs'>
                  {boundInviteCodeHint}
                </Text>
              </div>
            </Card>
          )}

          <Card
            className='!rounded-xl w-full'
            title={
              <SectionHeader
                title='我名下的邀请码'
                count={inviteCodeCount}
                buttonText='查看全部'
                onClick={() => setShowInviteCodesModal(true)}
                t={t}
              />
            }
          >
            {inviteCodesPreview.length > 0 ? (
              <div className='space-y-3'>
                {inviteCodesPreview.map((inviteCode) => (
                  <div
                    key={inviteCode.id}
                    className='rounded-lg border border-[var(--semi-color-border)] p-3 space-y-3'
                  >
                    <div className='flex flex-wrap items-center gap-2'>
                      <span className='font-semibold'>{inviteCode.code}</span>
                      <InviteCodeStatusTag inviteCode={inviteCode} t={t} />
                      {isManualInviteCode(inviteCode) && (
                        <Tag size='small' shape='circle' color='blue'>
                          {t('手动绑定码')}
                        </Tag>
                      )}
                    </div>
                    <div className='flex flex-wrap gap-2 text-xs'>
                      <Tag size='small' shape='circle'>
                        {t('目标分组')} {inviteCode.target_group}
                      </Tag>
                      <Tag size='small' shape='circle'>
                        {t('剩余次数')} {inviteCode.remaining_reward_uses || 0}
                      </Tag>
                      <Tag size='small' shape='circle'>
                        {t('邀请人数')} {inviteCode.invited_user_count || 0}
                      </Tag>
                      <Tag size='small' shape='circle'>
                        {t('单次赠送')}{' '}
                        {renderQuota(inviteCode.reward_quota_per_use || 0)}
                      </Tag>
                      <Tag size='small' shape='circle'>
                        {t('充值额')}{' '}
                        {renderQuotaWithAmount(
                          inviteCode.invite_total_recharge_amount || 0,
                        )}
                      </Tag>
                      <Tag size='small' shape='circle'>
                        {t('实付')}{' '}
                        {renderPaymentAmount(
                          inviteCode.invite_total_recharge_money ??
                            inviteCode.invite_total_recharge ??
                            0,
                        )}
                      </Tag>
                      <Tag size='small' shape='circle'>
                        {t('消费额')}{' '}
                        {renderQuota(inviteCode.invite_total_consume || 0)}
                      </Tag>
                    </div>
                    {!isManualInviteCode(inviteCode) && (
                      <div className='space-y-2'>
                        <Text type='tertiary' className='text-xs break-all'>
                          {buildInviteRegisterLink(inviteCode.code)}
                        </Text>
                        <Button
                          size='small'
                          theme='borderless'
                          onClick={() => handleCopyInviteLink(inviteCode.code)}
                        >
                          {t('复制邀请链接')}
                        </Button>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <Text type='tertiary'>{t('您当前还没有绑定的邀请码。')}</Text>
            )}
          </Card>

          <Card
            className='!rounded-xl w-full'
            title={
              <SectionHeader
                title='最近被邀请人'
                count={inviteeCount}
                buttonText='查看全部'
                onClick={() => setShowInviteesModal(true)}
                t={t}
              />
            }
          >
            {inviteesPreview.length > 0 ? (
              <div className='space-y-3'>
                {inviteesPreview.map((invitee) => (
                  <div
                    key={invitee.user_id}
                    className='rounded-lg border border-[var(--semi-color-border)] p-3 space-y-3'
                  >
                    <div className='font-semibold'>{invitee.username}</div>
                    <div className='flex flex-wrap gap-2 text-xs'>
                      <Tag size='small' shape='circle'>
                        {t('邀请码')} {invitee.invite_code || '-'}
                      </Tag>
                      <Tag size='small' shape='circle'>
                        {t('分组')} {invitee.group || '-'}
                      </Tag>
                      <Tag size='small' shape='circle'>
                        {t('充值额')}{' '}
                        {renderQuotaWithAmount(
                          invitee.invite_total_recharge_amount || 0,
                        )}
                      </Tag>
                      <Tag size='small' shape='circle'>
                        {t('实付')}{' '}
                        {renderPaymentAmount(
                          invitee.invite_total_recharge_money ??
                            invitee.invite_total_recharge ??
                            0,
                        )}
                      </Tag>
                      <Tag size='small' shape='circle'>
                        {t('消费额')}{' '}
                        {renderQuota(invitee.invite_total_consume || 0)}
                      </Tag>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <Text type='tertiary'>{t('暂无被邀请人记录。')}</Text>
            )}
          </Card>

          {userState?.user?.aff_quota > 0 && (
            <Card
              className='!rounded-xl w-full'
              title={<Text type='tertiary'>{t('历史奖励额度')}</Text>}
            >
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
            </Card>
          )}

          <Card
            className='!rounded-xl w-full'
            title={<Text type='tertiary'>{t('说明')}</Text>}
          >
            <div className='space-y-3'>
              <div className='flex items-start gap-2'>
                <Badge dot type='success' />
                <Text type='tertiary' className='text-sm'>
                  {t(
                    '邀请人数、邀请充值额度、邀请实付金额、邀请消费额度仅统计新邀请码体系。',
                  )}
                </Text>
              </div>

              <div className='flex items-start gap-2'>
                <Badge dot type='success' />
                <Text type='tertiary' className='text-sm'>
                  {t('邀请码已删除或禁用时，历史归属关系和统计仍会保留展示。')}
                </Text>
              </div>

              <div className='flex items-start gap-2'>
                <Badge dot type='success' />
                <Text type='tertiary' className='text-sm'>
                  {t('如需新的归属邀请码，请联系管理员创建并分配。')}
                </Text>
              </div>
            </div>
          </Card>
        </Space>
      </Card>

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
