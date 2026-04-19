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

import React, { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  API,
  showError,
  showSuccess,
  downloadTextAsFile,
  renderQuota,
  renderQuotaWithAmount,
} from '../../../../helpers';
import {
  displayAmountToQuota,
  getQuotaPerUnit,
  quotaToDisplayAmount,
} from '../../../../helpers/quota';
import { getCurrencyConfig } from '../../../../helpers/render';
import {
  Button,
  Form,
  Modal,
  Select,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';

const EditInviteCodeModal = ({ refresh, editingInviteCode, visible, handleClose }) => {
  const { t } = useTranslation();
  const isEdit = editingInviteCode.id !== undefined;
  const [loading, setLoading] = useState(false);
  const [groups, setGroups] = useState([]);
  const [rewardAmount, setRewardAmount] = useState(0);
  const [rewardQuota, setRewardQuota] = useState(0);
  const [rewardUsedUses, setRewardUsedUses] = useState(0);
  const [rewardRemainingUses, setRewardRemainingUses] = useState(0);
  const [ownerUsername, setOwnerUsername] = useState('');
  const formApiRef = useRef(null);
  const { symbol, type } = useMemo(() => getCurrencyConfig(), []);

  const initValues = useMemo(
    () => ({
      prefix: '',
      count: 1,
      owner_user_id: undefined,
      target_group: '',
      reward_amount_per_use: 0,
      reward_quota_per_use: 0,
      reward_total_uses: 0,
      status: 1,
    }),
    [],
  );

  const loadGroups = async () => {
    const groupsRes = await API.get('/api/group/');
    if (groupsRes.data.success) {
      setGroups(groupsRes.data.data || []);
    }
  };

  const resolveOwnerUsername = async (ownerUserId, { silent = false } = {}) => {
    const normalizedID = parseInt(ownerUserId, 10) || 0;
    if (normalizedID <= 0) {
      setOwnerUsername('');
      return;
    }
    try {
      const res = await API.get(`/api/user/${normalizedID}`);
      const { success, message, data } = res.data;
      if (success) {
        setOwnerUsername(data.username || '');
      } else {
        setOwnerUsername('');
        if (!silent) {
          showError(message);
        }
      }
    } catch (error) {
      setOwnerUsername('');
      if (!silent) {
        showError(error.message);
      }
    }
  };

  const loadInviteCode = async () => {
    if (!isEdit) {
      formApiRef.current?.setValues(initValues);
      setRewardAmount(0);
      setRewardQuota(0);
      setRewardUsedUses(0);
      setRewardRemainingUses(0);
      setOwnerUsername('');
      return;
    }
    setLoading(true);
    try {
      const res = await API.get(`/api/invite_code/${editingInviteCode.id}`);
      const { success, message, data } = res.data;
      if (success) {
        const rewardQuota = parseInt(data.reward_quota_per_use || 0, 10) || 0;
        const usedUses = parseInt(data.reward_used_uses || 0, 10) || 0;
        const remainingUses =
          parseInt(data.remaining_reward_uses || 0, 10) || 0;
        formApiRef.current?.setValues({
          ...initValues,
          ...data,
          reward_amount_per_use: quotaToDisplayAmount(rewardQuota),
          reward_quota_per_use: rewardQuota,
          reward_total_uses: remainingUses,
          status: data.status === 1,
        });
        setRewardAmount(quotaToDisplayAmount(rewardQuota));
        setRewardQuota(rewardQuota);
        setRewardUsedUses(usedUses);
        setRewardRemainingUses(remainingUses);
        setOwnerUsername(data.owner_username || '');
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  useEffect(() => {
    if (visible) {
      loadGroups();
      loadInviteCode();
    }
  }, [visible, editingInviteCode.id]);

  const syncRewardAmountToQuota = (amount) => {
    const quota = displayAmountToQuota(amount);
    setRewardAmount(Number(amount || 0));
    setRewardQuota(quota);
    formApiRef.current?.setValue('reward_amount_per_use', Number(amount || 0));
    formApiRef.current?.setValue('reward_quota_per_use', quota);
  };

  const syncRewardQuotaToAmount = (quota) => {
    const normalizedQuota = Math.max(0, parseInt(quota || 0, 10) || 0);
    const amount = quotaToDisplayAmount(normalizedQuota);
    setRewardQuota(normalizedQuota);
    setRewardAmount(amount);
    formApiRef.current?.setValue('reward_quota_per_use', normalizedQuota);
    formApiRef.current?.setValue('reward_amount_per_use', amount);
  };

  const handleOwnerUserIdChange = (value) => {
    const normalizedID = parseInt(value, 10) || 0;
    formApiRef.current?.setValue('owner_user_id', normalizedID);
    resolveOwnerUsername(normalizedID, { silent: true });
  };

  const handleRewardUsesChange = (value) => {
    const normalizedUses = Math.max(0, parseInt(value || 0, 10) || 0);
    setRewardRemainingUses(normalizedUses);
    formApiRef.current?.setValue('reward_total_uses', normalizedUses);
  };

  const submit = async (values) => {
    setLoading(true);
    try {
      const normalizedRewardUses = parseInt(values.reward_total_uses, 10) || 0;
      const payload = {
        ...values,
        count: parseInt(values.count, 10) || 0,
        owner_user_id: parseInt(values.owner_user_id, 10) || 0,
        reward_quota_per_use: parseInt(values.reward_quota_per_use, 10) || 0,
        reward_total_uses: isEdit
          ? rewardUsedUses + normalizedRewardUses
          : normalizedRewardUses,
        status: values.status ? 1 : 2,
      };

      let res;
      if (isEdit) {
        res = await API.put('/api/invite_code/', {
          ...payload,
          id: editingInviteCode.id,
        });
      } else {
        res = await API.post('/api/invite_code/', payload);
      }

      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }

      showSuccess(isEdit ? t('邀请码更新成功！') : t('邀请码创建成功！'));
      await refresh();

      if (!isEdit && Array.isArray(data) && data.length > 0) {
        Modal.confirm({
          title: t('邀请码创建成功'),
          content: (
            <div>
              <p>{t('邀请码创建成功，是否下载邀请码？')}</p>
              <p>{t('邀请码将以文本文件形式下载。')}</p>
            </div>
          ),
          onOk: () => {
            downloadTextAsFile(data.join('\n'), `${values.prefix || 'invite-code'}.txt`);
          },
        });
      }
      handleClose();
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  return (
    <Modal
      title={
        <Space>
          <Tag color={isEdit ? 'blue' : 'green'} shape='circle'>
            {isEdit ? t('更新') : t('新建')}
          </Tag>
          <Typography.Text strong>
            {isEdit ? t('编辑邀请码') : t('创建邀请码')}
          </Typography.Text>
        </Space>
      }
      visible={visible}
      onCancel={handleClose}
      onOk={() => formApiRef.current?.submitForm()}
      okText={t('提交')}
      cancelText={t('取消')}
      confirmLoading={loading}
      width={640}
    >
      <Form
        initValues={initValues}
        getFormApi={(api) => (formApiRef.current = api)}
        onSubmit={submit}
      >
        <Form.Input
          field='prefix'
          label={t('邀请码前缀')}
          disabled={isEdit}
          placeholder={t('例如：ZY-')}
        />
        {!isEdit && (
          <Form.InputNumber
            field='count'
            label={t('生成数量')}
            min={1}
            max={100}
          />
        )}
        {isEdit && (
          <Form.Input
            field='code'
            label={t('邀请码')}
            disabled
          />
        )}
        <Form.InputNumber
          field='owner_user_id'
          label={t('归属用户 ID')}
          min={1}
          onChange={handleOwnerUserIdChange}
          extraText={
            ownerUsername
              ? `${t('归属用户名称')}：${ownerUsername}`
              : t('输入用户 ID 后自动显示用户名')
          }
        />
        <Form.Select field='target_group' label={t('目标分组')}>
          {groups.map((group) => (
            <Select.Option key={group} value={group}>
              {group}
            </Select.Option>
          ))}
        </Form.Select>
        <Form.InputNumber
          field='reward_amount_per_use'
          label={t('单次赠送金额')}
          min={0}
          step={type === 'TOKENS' ? 1 : 0.01}
          onChange={syncRewardAmountToQuota}
          prefix={type === 'TOKENS' ? undefined : symbol}
          extraText={
            type === 'TOKENS'
              ? t('当前站点按 Tokens 展示，金额与额度数值一致')
              : `${t('输入金额后将自动换算额度')}，1 ${symbol} = ${renderQuota(
                  getQuotaPerUnit(),
                )}`
          }
        />
        <Form.InputNumber
          field='reward_quota_per_use'
          label={t('单次赠送额度')}
          min={0}
          suffix='Token'
          onChange={syncRewardQuotaToAmount}
          extraText={
            type === 'TOKENS'
              ? undefined
              : `${t('等价金额：')}${renderQuotaWithAmount(
                  rewardAmount,
                )} · ${t('当前额度：')}${renderQuota(rewardQuota)}`
          }
        />
        <Form.InputNumber
          field='reward_total_uses'
          label={isEdit ? t('剩余可赠送次数') : t('赠送总次数')}
          min={0}
          onChange={handleRewardUsesChange}
          extraText={
            isEdit
              ? `${t('已使用')} ${rewardUsedUses} ${t('次')}，${t(
                  '此处填写当前还可继续赠送的次数',
                )}`
              : undefined
          }
        />
        <Form.Switch field='status' label={t('启用状态')} initValue={true} />
      </Form>
    </Modal>
  );
};

export default EditInviteCodeModal;
