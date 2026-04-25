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

import React, { useMemo } from 'react';
import {
  Button,
  Card,
  Empty,
  Form,
  Input,
  Popconfirm,
  Space,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';
import { useInviteCodesData } from '../../../hooks/invite-codes/useInviteCodesData';
import {
  copy,
  renderNumber,
  renderPaymentAmount,
  renderQuota,
  renderQuotaWithAmount,
  showSuccess,
} from '../../../helpers';
import EditInviteCodeModal from './modals/EditInviteCodeModal';

const InviteCodesPage = () => {
  const { t } = useTranslation();
  const inviteCodesData = useInviteCodesData();
  const {
    inviteCodes,
    loading,
    activePage,
    pageSize,
    inviteCodeCount,
    formInitValues,
    setFormApi,
    searchInviteCodes,
    handlePageChange,
    handlePageSizeChange,
    showEdit,
    editingInviteCode,
    setEditingInviteCode,
    setShowEdit,
    refresh,
    updateInviteCodeStatus,
    deleteInviteCode,
  } = inviteCodesData;

  const columns = useMemo(
    () => [
      {
        title: t('邀请码'),
        dataIndex: 'code',
        render: (text) => <Tag shape='circle'>{text}</Tag>,
      },
      {
        title: t('前缀'),
        dataIndex: 'prefix',
      },
      {
        title: t('归属用户'),
        dataIndex: 'owner_username',
        render: (text, record) => text || `#${record.owner_user_id}`,
      },
      {
        title: t('目标分组'),
        dataIndex: 'target_group',
      },
      {
        title: t('单次赠送额度'),
        dataIndex: 'reward_quota_per_use',
        render: (value) => renderQuota(value || 0),
      },
      {
        title: t('赠送次数'),
        dataIndex: 'reward_total_uses',
        render: (_, record) =>
          `${record.reward_used_uses || 0} / ${record.reward_total_uses || 0} (${t('剩余')} ${record.remaining_reward_uses || 0})`,
      },
      {
        title: t('邀请人数'),
        dataIndex: 'invited_user_count',
        render: (value) => renderNumber(value || 0),
      },
      {
        title: t('邀请充值额度'),
        dataIndex: 'invite_total_recharge_amount',
        render: (value) => renderQuotaWithAmount(value || 0),
      },
      {
        title: t('邀请实付金额'),
        dataIndex: 'invite_total_recharge_money',
        render: (value, record) =>
          renderPaymentAmount(value ?? record.invite_total_recharge ?? 0),
      },
      {
        title: t('邀请消费额度'),
        dataIndex: 'invite_total_consume',
        render: (value) => renderQuota(value || 0),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (value) =>
          value === 1 ? (
            <Tag color='green' shape='circle'>
              {t('已启用')}
            </Tag>
          ) : (
            <Tag color='red' shape='circle'>
              {t('已禁用')}
            </Tag>
          ),
      },
      {
        title: t('操作'),
        dataIndex: 'operate',
        fixed: 'right',
        render: (_, record) => (
          <Space>
            <Button
              size='small'
              onClick={async () => {
                if (await copy(record.code)) {
                  showSuccess(t('邀请码已复制到剪贴板'));
                }
              }}
            >
              {t('复制邀请码')}
            </Button>
            <Button
              size='small'
              onClick={() => {
                setEditingInviteCode(record);
                setShowEdit(true);
              }}
            >
              {t('编辑')}
            </Button>
            <Button
              size='small'
              type={record.status === 1 ? 'danger' : 'primary'}
              onClick={() =>
                updateInviteCodeStatus(record, record.status === 1 ? 2 : 1)
              }
            >
              {record.status === 1 ? t('禁用') : t('启用')}
            </Button>
            <Popconfirm
              title={t('确定是否要删除此邀请码？')}
              onConfirm={() => deleteInviteCode(record)}
            >
              <Button size='small' type='danger' theme='borderless'>
                {t('删除')}
              </Button>
            </Popconfirm>
          </Space>
        ),
      },
    ],
    [
      t,
      setEditingInviteCode,
      setShowEdit,
      updateInviteCodeStatus,
      deleteInviteCode,
    ],
  );

  return (
    <>
      <EditInviteCodeModal
        refresh={refresh}
        editingInviteCode={editingInviteCode}
        visible={showEdit}
        handleClose={() => setShowEdit(false)}
      />
      <Card>
        <div className='flex flex-col gap-4'>
          <div className='flex flex-col md:flex-row md:items-center md:justify-between gap-3'>
            <div>
              <Typography.Title heading={5} className='!mb-1'>
                {t('邀请码管理')}
              </Typography.Title>
              <Typography.Text type='tertiary'>
                {t('批量生成并管理归属用户邀请码')}
              </Typography.Text>
            </div>
            <Button
              theme='solid'
              onClick={() => {
                setEditingInviteCode({ id: undefined });
                setShowEdit(true);
              }}
            >
              {t('新增邀请码')}
            </Button>
          </div>

          <Form
            initValues={formInitValues}
            getFormApi={setFormApi}
            layout='horizontal'
            onSubmit={searchInviteCodes}
          >
            <div className='flex flex-col md:flex-row gap-3'>
              <Form.Input
                field='searchKeyword'
                noLabel
                placeholder={t('搜索邀请码 / 前缀 / 归属用户')}
                style={{ flex: 1 }}
              />
              <Button htmlType='submit' type='primary'>
                {t('搜索')}
              </Button>
            </div>
          </Form>

          <Table
            rowKey='id'
            columns={columns}
            dataSource={inviteCodes}
            loading={loading}
            pagination={{
              currentPage: activePage,
              pageSize: pageSize,
              total: inviteCodeCount,
              pageSizeOpts: [10, 20, 50, 100],
              showSizeChanger: true,
              onPageChange: handlePageChange,
              onPageSizeChange: handlePageSizeChange,
            }}
            scroll={{ x: 'max-content' }}
            empty={
              <Empty
                image={
                  <IllustrationNoResult style={{ width: 150, height: 150 }} />
                }
                darkModeImage={
                  <IllustrationNoResultDark
                    style={{ width: 150, height: 150 }}
                  />
                }
                description={t('搜索无结果')}
                style={{ padding: 30 }}
              />
            }
          />
        </div>
      </Card>
    </>
  );
};

export default InviteCodesPage;
