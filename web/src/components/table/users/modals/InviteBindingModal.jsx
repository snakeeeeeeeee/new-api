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
  Button,
  Divider,
  Input,
  Modal,
  Select,
  Space,
  Spin,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconLink, IconSearch, IconDelete } from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../../../helpers';

const { Text, Title } = Typography;

const InviteBindingModal = ({ visible, onCancel, user, refresh, t }) => {
  const [loading, setLoading] = useState(false);
  const [searching, setSearching] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [unbinding, setUnbinding] = useState(false);
  const [currentUser, setCurrentUser] = useState(null);
  const [currentOwner, setCurrentOwner] = useState(null);
  const [currentInviteCode, setCurrentInviteCode] = useState(null);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [ownerOptions, setOwnerOptions] = useState([]);
  const [selectedOwnerId, setSelectedOwnerId] = useState(undefined);
  const [inviteCodes, setInviteCodes] = useState([]);
  const [selectedInviteCodeId, setSelectedInviteCodeId] = useState(0);

  const targetUserId = user?.id;

  const hasCurrentBinding =
    Boolean(currentUser?.inviter_id) ||
    Boolean(currentUser?.invite_code_owner_id) ||
    Boolean(currentUser?.invite_code_id);

  const addOwnerOption = useCallback((owner) => {
    if (!owner?.id) {
      return;
    }
    setOwnerOptions((prev) => {
      if (prev.some((item) => Number(item.value) === Number(owner.id))) {
        return prev;
      }
      return [
        ...prev,
        {
          label: `${owner.username || `#${owner.id}`} (#${owner.id})`,
          value: owner.id,
        },
      ];
    });
  }, []);

  const loadOwnerInviteCodes = useCallback(
    async (ownerUserId) => {
      const normalizedOwnerId = Number(ownerUserId) || 0;
      if (normalizedOwnerId <= 0) {
        setInviteCodes([]);
        setSelectedInviteCodeId(0);
        return;
      }
      try {
        const res = await API.get(
          `/api/user/${normalizedOwnerId}/invite_codes?p=1&page_size=100`,
        );
        const { success, message, data } = res.data;
        if (!success) {
          showError(message);
          setInviteCodes([]);
          return;
        }
        setInviteCodes(data.items || []);
      } catch (error) {
        showError(error.response?.data?.message || error.message);
        setInviteCodes([]);
      }
    },
    [],
  );

  const loadCurrentBinding = useCallback(async () => {
    if (!targetUserId) {
      return;
    }
    setLoading(true);
    setCurrentUser(null);
    setCurrentOwner(null);
    setCurrentInviteCode(null);
    setInviteCodes([]);
    setSelectedInviteCodeId(0);

    try {
      const res = await API.get(`/api/user/${targetUserId}`);
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }

      setCurrentUser(data);
      const ownerId = data.invite_code_owner_id || data.inviter_id || 0;
      setSelectedOwnerId(ownerId || undefined);

      if (ownerId > 0) {
        try {
          const ownerRes = await API.get(`/api/user/${ownerId}`);
          if (ownerRes.data?.success) {
            setCurrentOwner(ownerRes.data.data);
            addOwnerOption(ownerRes.data.data);
          }
        } catch (error) {
          setCurrentOwner(null);
        }
        await loadOwnerInviteCodes(ownerId);
      }

      if (data.invite_code_id > 0) {
        setSelectedInviteCodeId(data.invite_code_id);
        try {
          const codeRes = await API.get(`/api/invite_code/${data.invite_code_id}`);
          if (codeRes.data?.success) {
            setCurrentInviteCode(codeRes.data.data);
          }
        } catch (error) {
          setCurrentInviteCode(null);
        }
      }
    } catch (error) {
      showError(error.response?.data?.message || error.message);
    } finally {
      setLoading(false);
    }
  }, [addOwnerOption, loadOwnerInviteCodes, targetUserId]);

  useEffect(() => {
    if (!visible) {
      return;
    }
    setSearchKeyword('');
    setOwnerOptions([]);
    loadCurrentBinding();
  }, [loadCurrentBinding, visible]);

  const searchOwners = async () => {
    const keyword = searchKeyword.trim();
    if (!keyword) {
      showError(t('请输入用户 ID 或用户名'));
      return;
    }
    setSearching(true);
    try {
      const res = await API.get(
        `/api/user/search?keyword=${encodeURIComponent(keyword)}&p=1&page_size=20`,
      );
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const options = (data.items || [])
        .filter((item) => Number(item.id) !== Number(targetUserId))
        .map((item) => ({
          label: `${item.username || `#${item.id}`} (#${item.id})`,
          value: item.id,
        }));
      setOwnerOptions(options);
      if (options.length === 0) {
        showError(t('未找到可绑定的目标邀请人'));
      }
    } catch (error) {
      showError(error.response?.data?.message || error.message);
    } finally {
      setSearching(false);
    }
  };

  const handleOwnerChange = async (value) => {
    const ownerId = Number(value) || 0;
    setSelectedOwnerId(ownerId || undefined);
    setSelectedInviteCodeId(0);
    await loadOwnerInviteCodes(ownerId);
  };

  const inviteCodeOptions = useMemo(() => {
    const ownerId = Number(selectedOwnerId) || 0;
    const options = [
      {
        label:
          ownerId > 0
            ? `${t('自动手动绑定码')} MANUAL-${ownerId}`
            : t('自动手动绑定码'),
        value: 0,
      },
    ];
    inviteCodes.forEach((code) => {
      options.push({
        label: `${code.code} (#${code.id})${
          code.status === 2 ? ` · ${t('已禁用')}` : ''
        }`,
        value: code.id,
      });
    });
    return options;
  }, [inviteCodes, selectedOwnerId, t]);

  const submit = async () => {
    const ownerId = Number(selectedOwnerId) || 0;
    if (ownerId <= 0) {
      showError(t('请选择目标邀请人'));
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.put(`/api/user/${targetUserId}/invite_binding`, {
        owner_user_id: ownerId,
        invite_code_id: Number(selectedInviteCodeId) || 0,
      });
      const { success, message } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      showSuccess(t('邀请归属绑定成功'));
      await refresh?.();
      onCancel();
    } catch (error) {
      showError(error.response?.data?.message || error.message);
    } finally {
      setSubmitting(false);
    }
  };

  const unbind = () => {
    Modal.confirm({
      title: t('确认解绑邀请归属'),
      content: t('解绑后该用户将不再计入任何邀请人的新邀请统计。'),
      okText: t('解绑'),
      cancelText: t('取消'),
      onOk: async () => {
        setUnbinding(true);
        try {
          const res = await API.delete(`/api/user/${targetUserId}/invite_binding`);
          const { success, message } = res.data;
          if (!success) {
            showError(message);
            return;
          }
          showSuccess(t('邀请归属已解绑'));
          await refresh?.();
          onCancel();
        } catch (error) {
          showError(error.response?.data?.message || error.message);
        } finally {
          setUnbinding(false);
        }
      },
    });
  };

  const currentOwnerText = currentOwner
    ? `${currentOwner.username} (#${currentOwner.id})`
    : currentUser?.invite_code_owner_id || currentUser?.inviter_id
      ? `#${currentUser.invite_code_owner_id || currentUser.inviter_id}`
      : t('无邀请人');

  const currentCodeText = currentInviteCode
    ? `${currentInviteCode.code} (#${currentInviteCode.id})`
    : currentUser?.invite_code_id
      ? `#${currentUser.invite_code_id}`
      : t('无邀请码');

  return (
    <Modal
      centered
      visible={visible}
      onCancel={onCancel}
      title={
        <Space>
          <IconLink />
          <span>{t('绑定邀请人')}</span>
        </Space>
      }
      footer={
        <Space>
          <Button onClick={onCancel}>{t('取消')}</Button>
          <Button
            type='danger'
            theme='borderless'
            icon={<IconDelete />}
            disabled={!hasCurrentBinding}
            loading={unbinding}
            onClick={unbind}
          >
            {t('解绑')}
          </Button>
          <Button type='primary' loading={submitting} onClick={submit}>
            {t('提交')}
          </Button>
        </Space>
      }
      width={640}
    >
      <Spin spinning={loading}>
        <div className='flex flex-col gap-4'>
          <div className='rounded-md border border-solid border-[var(--semi-color-border)] px-4 py-3'>
            <div className='flex flex-col gap-2'>
              <Title heading={6} className='!m-0'>
                {user?.username || currentUser?.username || '-'} (ID:{' '}
                {targetUserId || '-'})
              </Title>
              <div className='flex flex-wrap gap-2'>
                <Tag color={hasCurrentBinding ? 'green' : 'grey'} shape='circle'>
                  {hasCurrentBinding ? t('已绑定') : t('未绑定')}
                </Tag>
                <Tag color='white' shape='circle'>
                  {t('当前邀请人')}：{currentOwnerText}
                </Tag>
                <Tag color='white' shape='circle'>
                  {t('当前邀请码')}：{currentCodeText}
                </Tag>
              </div>
            </div>
          </div>

          <div>
            <Text strong>{t('目标邀请人')}</Text>
            <div className='flex gap-2 mt-2'>
              <Input
                value={searchKeyword}
                placeholder={t('输入用户 ID / 用户名搜索')}
                onChange={setSearchKeyword}
                onEnterPress={searchOwners}
              />
              <Button
                type='primary'
                icon={<IconSearch />}
                loading={searching}
                onClick={searchOwners}
              >
                {t('搜索')}
              </Button>
            </div>
            <Select
              className='mt-2 w-full'
              placeholder={t('请选择目标邀请人')}
              value={selectedOwnerId}
              optionList={ownerOptions}
              onChange={handleOwnerChange}
            />
          </div>

          <Divider margin='8px' />

          <div>
            <Text strong>{t('目标邀请码')}</Text>
            <Select
              className='mt-2 w-full'
              value={selectedInviteCodeId}
              optionList={inviteCodeOptions}
              onChange={(value) => setSelectedInviteCodeId(Number(value) || 0)}
              disabled={!selectedOwnerId}
            />
            <Text type='tertiary' size='small' className='block mt-2'>
              {t('不选择具体邀请码时，会自动创建或复用该邀请人的 MANUAL 手动绑定码。')}
            </Text>
          </div>
        </div>
      </Spin>
    </Modal>
  );
};

export default InviteBindingModal;
