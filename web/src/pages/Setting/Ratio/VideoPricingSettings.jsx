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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Banner,
  Button,
  Empty,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Select,
  Space,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Copy,
  DollarSign,
  Link2,
  Plus,
  Save,
  Trash2,
  Unlink,
  Video,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  API,
  VIDEO_PRICING_MODE,
  bindVideoPricingModels,
  bindVideoPricingPolicyModels,
  calculateVideoPricingPreview,
  copyVideoPricingProfile,
  deleteVideoPricingProfile,
  getVideoPricingProfileModels,
  normalizeVideoPricing,
  removeVideoPricingBinding,
  selectFilter,
  showError,
  showSuccess,
  showWarning,
  updateVideoPricingBinding,
  validateVideoPricing,
} from '../../../helpers';
import { useIsMobile } from '../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;
const POLICY_ONLY = '__policy_only__';

const createProfileId = (profiles, base = 'video-per-second') => {
  let candidate = base;
  let suffix = 2;
  while (profiles[candidate]) {
    candidate = `${base}-${suffix}`;
    suffix += 1;
  }
  return candidate;
};

const getOptionKeys = (raw) => {
  try {
    const parsed = JSON.parse(raw || '{}');
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? Object.keys(parsed)
      : [];
  } catch (_) {
    return [];
  }
};

const formatUSD = (value) => {
  const number = Number(value);
  if (!Number.isFinite(number)) return '-';
  return `$${number.toFixed(8).replace(/0+$/, '').replace(/\.$/, '')}`;
};

export default function VideoPricingSettings({ options, refresh }) {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [config, setConfig] = useState(() =>
    normalizeVideoPricing(options.VideoPricing),
  );
  const [selectedProfileId, setSelectedProfileId] = useState('');
  const [enabledModels, setEnabledModels] = useState([]);
  const [loading, setLoading] = useState(false);
  const [createVisible, setCreateVisible] = useState(false);
  const [createDraft, setCreateDraft] = useState({ id: '', name: '' });
  const [bindingModels, setBindingModels] = useState([]);
  const [bindingProfile, setBindingProfile] = useState(POLICY_ONLY);
  const [bindingSubscriptionEnabled, setBindingSubscriptionEnabled] =
    useState(false);
  const [savingAction, setSavingAction] = useState('');
  const [previewSeconds, setPreviewSeconds] = useState(5);
  const [previewGroupRatio, setPreviewGroupRatio] = useState(1);

  useEffect(() => {
    const nextConfig = normalizeVideoPricing(options.VideoPricing);
    const profileIds = Object.keys(nextConfig.profiles);
    setConfig(nextConfig);
    setSelectedProfileId((current) =>
      current && nextConfig.profiles[current] ? current : profileIds[0] || '',
    );
    setBindingProfile((current) =>
      current === POLICY_ONLY || nextConfig.profiles[current]
        ? current
        : profileIds[0] || POLICY_ONLY,
    );
  }, [options.VideoPricing]);

  useEffect(() => {
    const loadModels = async () => {
      try {
        const response = await API.get('/api/channel/models_enabled');
        if (response.data?.success) {
          setEnabledModels(
            Array.isArray(response.data.data) ? response.data.data : [],
          );
        }
      } catch (error) {
        console.error(t('获取启用模型失败:'), error);
      }
    };
    loadModels();
  }, [t]);

  const profileIds = useMemo(
    () => Object.keys(config.profiles).sort((a, b) => a.localeCompare(b)),
    [config.profiles],
  );
  const selectedProfile = selectedProfileId
    ? config.profiles[selectedProfileId]
    : null;
  const validationErrors = useMemo(
    () => validateVideoPricing(config, t),
    [config, t],
  );
  const modelOptions = useMemo(
    () =>
      Array.from(
        new Set([
          ...enabledModels,
          ...Object.keys(config.model_bindings),
          ...getOptionKeys(options.ModelPrice),
          ...getOptionKeys(options.ModelRatio),
        ]),
      )
        .filter(Boolean)
        .sort((a, b) => a.localeCompare(b))
        .map((model) => ({ label: model, value: model })),
    [
      config.model_bindings,
      enabledModels,
      options.ModelPrice,
      options.ModelRatio,
    ],
  );
  const profileOptions = useMemo(
    () => [
      { value: POLICY_ONLY, label: t('仅订阅策略（保留原价格）') },
      ...profileIds.map((id) => ({
        value: id,
        label: config.profiles[id]?.name || id,
      })),
    ],
    [config.profiles, profileIds, t],
  );
  const preview = useMemo(
    () =>
      selectedProfile
        ? calculateVideoPricingPreview({
            profile: selectedProfile,
            seconds: previewSeconds,
            groupRatio: previewGroupRatio,
          })
        : null,
    [previewGroupRatio, previewSeconds, selectedProfile],
  );

  const persistConfig = async (
    nextConfig,
    successMessage = t('保存成功'),
    action = 'config',
  ) => {
    const normalized = normalizeVideoPricing(nextConfig);
    const errors = validateVideoPricing(normalized, t);
    if (errors.length > 0) {
      showError(errors[0]);
      return false;
    }
    setLoading(true);
    setSavingAction(action);
    try {
      const response = await API.put('/api/option/', {
        key: 'VideoPricing',
        value: JSON.stringify(normalized),
      });
      if (!response.data?.success) {
        showError(response.data?.message || t('保存失败'));
        return false;
      }
      setConfig(normalized);
      showSuccess(successMessage);
      await refresh();
      return true;
    } catch (error) {
      showError(error.message || t('保存失败'));
      return false;
    } finally {
      setSavingAction('');
      setLoading(false);
    }
  };

  const updateProfile = (patch) => {
    if (!selectedProfileId) return;
    setConfig((current) => ({
      ...current,
      profiles: {
        ...current.profiles,
        [selectedProfileId]: {
          ...current.profiles[selectedProfileId],
          ...patch,
        },
      },
    }));
  };

  const openCreateModal = () => {
    setCreateDraft({ id: createProfileId(config.profiles), name: '' });
    setCreateVisible(true);
  };

  const createProfile = async () => {
    const id = createDraft.id.trim();
    const name = createDraft.name.trim();
    if (!id || !name) {
      showWarning(t('请填写模板 ID 和名称'));
      return;
    }
    if (config.profiles[id]) {
      showWarning(t('模板 ID 已存在'));
      return;
    }
    const next = {
      ...config,
      profiles: {
        ...config.profiles,
        [id]: {
          name,
          billing_mode: VIDEO_PRICING_MODE,
          unit_price: 0,
        },
      },
    };
    const saved = await persistConfig(
      next,
      t('价格模板已创建'),
      `profile:create:${id}`,
    );
    if (!saved) return;
    setSelectedProfileId(id);
    setBindingProfile(id);
    setCreateVisible(false);
  };

  const copyProfile = async () => {
    if (!selectedProfile) return;
    const id = createProfileId(config.profiles, `${selectedProfileId}-copy`);
    const next = copyVideoPricingProfile(
      config,
      selectedProfileId,
      id,
      t('{{name}} 副本', { name: selectedProfile.name }),
    );
    const saved = await persistConfig(
      next,
      t('价格模板已复制'),
      `profile:copy:${id}`,
    );
    if (!saved) return;
    setSelectedProfileId(id);
    setBindingProfile(id);
  };

  const deleteProfile = async (profileId) => {
    const targetProfileId = profileId || selectedProfileId;
    if (!targetProfileId) return;
    const boundModels = getVideoPricingProfileModels(config, targetProfileId);
    if (boundModels.length > 0) {
      showWarning(
        t('该模板仍被 {{count}} 个模型使用，请先改绑或解绑', {
          count: boundModels.length,
        }),
      );
      return;
    }
    const next = deleteVideoPricingProfile(config, targetProfileId);
    const saved = await persistConfig(
      next,
      t('价格模板已删除'),
      `profile:delete:${targetProfileId}`,
    );
    if (!saved) return;
    if (targetProfileId === selectedProfileId) {
      const nextId = profileIds.find((id) => id !== targetProfileId) || '';
      setSelectedProfileId(nextId);
      setBindingProfile(nextId || POLICY_ONLY);
    }
  };

  const applyBindings = async () => {
    if (bindingModels.length === 0) {
      showWarning(t('请至少选择或输入一个公开模型名称'));
      return;
    }
    const next =
      bindingProfile === POLICY_ONLY
        ? bindVideoPricingPolicyModels(config, bindingModels)
        : bindVideoPricingModels(
            config,
            bindingModels,
            bindingProfile,
            bindingSubscriptionEnabled,
          );
    const count = bindingModels.length;
    const saved = await persistConfig(
      next,
      t('已配置 {{count}} 个模型', { count }),
      'binding:batch',
    );
    if (!saved) return;
    setBindingModels([]);
  };

  const updateBinding = async (model, patch) => {
    const next = updateVideoPricingBinding(config, model, patch);
    await persistConfig(next, t('模型绑定已更新'), `binding:update:${model}`);
  };

  const updateBindingProfile = (model, profileId) => {
    return updateBinding(
      model,
      profileId === POLICY_ONLY
        ? { profile: '', subscription_enabled: true }
        : { profile: profileId },
    );
  };

  const removeBinding = async (model) => {
    const next = removeVideoPricingBinding(config, model);
    await persistConfig(next, t('模型绑定已解除'), `binding:delete:${model}`);
  };

  const save = () => persistConfig(config);

  const saveProfile = () =>
    persistConfig(
      config,
      t('价格模板修改已保存'),
      `profile:update:${selectedProfileId}`,
    );

  const bindingRows = Object.entries(config.model_bindings)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([model, binding]) => ({ key: model, model, ...binding }));

  const bindingColumns = [
    {
      title: t('公开模型名称'),
      dataIndex: 'model',
      width: 260,
      render: (model) => <Text copyable={{ content: model }}>{model}</Text>,
    },
    {
      title: t('价格模板'),
      dataIndex: 'profile',
      width: 240,
      render: (profileId, record) => (
        <Select
          value={profileId || POLICY_ONLY}
          optionList={profileOptions}
          disabled={loading}
          onChange={(value) => updateBindingProfile(record.model, value)}
          style={{ width: '100%' }}
        />
      ),
    },
    {
      title: t('允许订阅扣费'),
      dataIndex: 'subscription_enabled',
      width: 140,
      render: (enabled, record) => (
        <Switch
          aria-label={t('允许订阅扣费')}
          checked={Boolean(enabled)}
          disabled={loading || !record.profile}
          onChange={(checked) =>
            updateBinding(record.model, { subscription_enabled: checked })
          }
        />
      ),
    },
    {
      title: t('资金策略'),
      dataIndex: 'subscription_enabled',
      width: 180,
      render: (enabled) =>
        enabled ? (
          <Tag color='green'>{t('遵循用户扣费偏好')}</Tag>
        ) : (
          <Tag color='blue'>{t('仅钱包余额')}</Tag>
        ),
    },
    {
      title: t('操作'),
      width: 80,
      render: (_, record) => (
        <Popconfirm
          title={t('确定解除模型 {{model}} 的视频计价绑定吗？', {
            model: record.model,
          })}
          onConfirm={() => removeBinding(record.model)}
        >
          <Button
            aria-label={t('解除绑定')}
            theme='borderless'
            type='danger'
            icon={<Unlink size={15} />}
            loading={savingAction === `binding:delete:${record.model}`}
            disabled={
              loading && savingAction !== `binding:delete:${record.model}`
            }
          />
        </Popconfirm>
      ),
    },
  ];

  const renderMobileBinding = (record) => (
    <div
      key={record.model}
      className='border rounded-md p-3 space-y-3'
      style={{ borderColor: 'var(--semi-color-border)' }}
    >
      <div className='flex items-start justify-between gap-2'>
        <Text copyable={{ content: record.model }}>{record.model}</Text>
        <Popconfirm
          title={t('确定解除模型 {{model}} 的视频计价绑定吗？', {
            model: record.model,
          })}
          onConfirm={() => removeBinding(record.model)}
        >
          <Button
            aria-label={t('解除绑定')}
            theme='borderless'
            type='danger'
            icon={<Unlink size={15} />}
            loading={savingAction === `binding:delete:${record.model}`}
            disabled={
              loading && savingAction !== `binding:delete:${record.model}`
            }
          />
        </Popconfirm>
      </div>
      <div>
        <Text type='tertiary'>{t('价格模板')}</Text>
        <Select
          className='mt-1 w-full'
          value={record.profile || POLICY_ONLY}
          optionList={profileOptions}
          disabled={loading}
          onChange={(value) => updateBindingProfile(record.model, value)}
        />
      </div>
      <div className='flex items-center justify-between gap-3 min-h-11'>
        <div>
          <Text>{t('允许订阅扣费')}</Text>
          <div className='text-xs text-gray-500 mt-1'>
            {record.subscription_enabled
              ? t('遵循用户扣费偏好')
              : t('仅钱包余额')}
          </div>
        </div>
        <Switch
          aria-label={t('允许订阅扣费')}
          checked={Boolean(record.subscription_enabled)}
          disabled={loading || !record.profile}
          onChange={(checked) =>
            updateBinding(record.model, { subscription_enabled: checked })
          }
        />
      </div>
    </div>
  );

  return (
    <div className='space-y-5'>
      <div className='flex flex-col md:flex-row md:items-center justify-between gap-3'>
        <div>
          <Title heading={5}>{t('视频按秒计价')}</Title>
          <Text type='tertiary'>
            {t(
              '按请求的有效视频时长计费；分辨率由公开模型名称区分，不叠加分辨率倍率。',
            )}
          </Text>
        </div>
        <Button
          type='primary'
          icon={<Save size={16} />}
          loading={loading}
          onClick={save}
        >
          {t('保存视频计价配置')}
        </Button>
      </div>

      {validationErrors.length > 0 ? (
        <Banner
          type='warning'
          fullMode={false}
          closeIcon={null}
          title={t('配置尚未完成')}
          description={validationErrors[0]}
        />
      ) : null}

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: isMobile
            ? 'minmax(0, 1fr)'
            : 'minmax(220px, 280px) minmax(0, 1fr)',
          gap: 16,
          alignItems: 'start',
        }}
      >
        <div
          className='border rounded-md p-3'
          style={{ borderColor: 'var(--semi-color-border)' }}
        >
          <div className='flex items-center justify-between mb-3'>
            <Text strong>{t('价格模板')}</Text>
            <Space spacing={4}>
              <Button
                aria-label={t('新增模板')}
                theme='borderless'
                icon={<Plus size={16} />}
                disabled={loading}
                onClick={openCreateModal}
              />
              <Button
                aria-label={t('复制模板')}
                theme='borderless'
                icon={<Copy size={16} />}
                loading={savingAction.startsWith('profile:copy:')}
                disabled={!selectedProfile || loading}
                onClick={copyProfile}
              />
            </Space>
          </div>
          {profileIds.length === 0 ? (
            <Empty
              title={t('暂无视频计价模板')}
              description={t('新增模板后即可配置每秒价格并绑定公开模型。')}
            />
          ) : (
            <div className='space-y-2'>
              {profileIds.map((id) => {
                const profile = config.profiles[id];
                const count = Object.values(config.model_bindings).filter(
                  (binding) => binding.profile === id,
                ).length;
                const selected = id === selectedProfileId;
                const deleteDisabled = count > 0 || loading;
                const deleteButton = (
                  <Button
                    aria-label={t('删除模板')}
                    theme='borderless'
                    type='danger'
                    icon={<Trash2 size={15} />}
                    loading={savingAction === `profile:delete:${id}`}
                    disabled={deleteDisabled}
                  />
                );
                return (
                  <div
                    key={id}
                    className='flex items-center gap-1 border rounded-md pr-1'
                    style={{
                      borderColor: selected
                        ? 'var(--semi-color-primary)'
                        : 'var(--semi-color-border)',
                      background: selected
                        ? 'var(--semi-color-primary-light-default)'
                        : 'transparent',
                    }}
                  >
                    <button
                      type='button'
                      className='min-w-0 flex-1 text-left px-3 py-2 cursor-pointer'
                      onClick={() => {
                        setSelectedProfileId(id);
                        setBindingProfile(id);
                      }}
                    >
                      <div className='font-medium truncate'>{profile.name}</div>
                      <div className='flex flex-wrap items-center gap-2 mt-1 text-xs text-gray-500'>
                        <span>
                          {formatUSD(profile.unit_price)}/{t('秒')}
                        </span>
                        <span>·</span>
                        <span>{t('{{count}} 个模型', { count })}</span>
                      </div>
                    </button>
                    {count > 0 ? (
                      <Tooltip
                        content={t(
                          '该模板仍被 {{count}} 个模型使用，请先改绑或解绑',
                          { count },
                        )}
                      >
                        <span>{deleteButton}</span>
                      </Tooltip>
                    ) : (
                      <Popconfirm
                        title={t('确定删除价格模板 {{name}} 吗？', {
                          name: profile.name || id,
                        })}
                        onConfirm={() => deleteProfile(id)}
                      >
                        {deleteButton}
                      </Popconfirm>
                    )}
                  </div>
                );
              })}
            </div>
          )}
        </div>

        <div className='min-w-0'>
          {!selectedProfile ? (
            <Empty
              title={t('选择或新增价格模板')}
              description={t('每个模板定义一个固定的美元每秒价格。')}
            />
          ) : (
            <div className='space-y-5'>
              <div
                className='border rounded-md p-4'
                style={{ borderColor: 'var(--semi-color-border)' }}
              >
                <div className='flex items-center justify-between gap-3 mb-3'>
                  <Text strong>{t('模板配置')}</Text>
                  <Button
                    type='primary'
                    icon={<Save size={15} />}
                    loading={
                      savingAction === `profile:update:${selectedProfileId}`
                    }
                    disabled={
                      loading &&
                      savingAction !== `profile:update:${selectedProfileId}`
                    }
                    onClick={saveProfile}
                  >
                    {t('保存修改')}
                  </Button>
                </div>
                <div className='grid grid-cols-1 md:grid-cols-3 gap-3'>
                  <div>
                    <Text strong>{t('模板名称')}</Text>
                    <Input
                      className='mt-2'
                      value={selectedProfile.name}
                      onChange={(value) => updateProfile({ name: value })}
                    />
                  </div>
                  <div>
                    <Text strong>{t('计费模式')}</Text>
                    <div className='mt-2 min-h-8 flex items-center'>
                      <Tag color='orange'>{VIDEO_PRICING_MODE}</Tag>
                    </div>
                  </div>
                  <div>
                    <Text strong>{t('每秒美元价')}</Text>
                    <InputNumber
                      className='mt-2 w-full'
                      min={0}
                      precision={8}
                      value={
                        Number.isFinite(selectedProfile.unit_price)
                          ? selectedProfile.unit_price
                          : null
                      }
                      prefix={<DollarSign size={13} />}
                      onChange={(value) => updateProfile({ unit_price: value })}
                    />
                  </div>
                </div>
              </div>

              <div
                className='border rounded-md p-4'
                style={{ borderColor: 'var(--semi-color-border)' }}
              >
                <Text strong>{t('实时费用预览')}</Text>
                <div className='grid grid-cols-1 sm:grid-cols-2 gap-3 mt-3'>
                  <div>
                    <Text type='tertiary'>{t('有效时长（秒）')}</Text>
                    <InputNumber
                      className='mt-1 w-full'
                      min={1}
                      precision={0}
                      value={previewSeconds}
                      onChange={setPreviewSeconds}
                    />
                  </div>
                  <div>
                    <Text type='tertiary'>{t('预览分组倍率')}</Text>
                    <InputNumber
                      className='mt-1 w-full'
                      min={0}
                      precision={4}
                      value={previewGroupRatio}
                      onChange={setPreviewGroupRatio}
                    />
                  </div>
                </div>
                <div
                  className='mt-3 px-3 py-2 rounded-md text-sm'
                  style={{ background: 'var(--semi-color-fill-0)' }}
                >
                  {preview ? (
                    <span>
                      {formatUSD(preview.unit_price)} × {preview.seconds}{' '}
                      {t('秒')} × {preview.group_ratio} ={' '}
                      <strong>{formatUSD(preview.total)}</strong>
                    </span>
                  ) : (
                    <Text type='danger'>
                      {t('请输入有效的整数秒数和分组倍率')}
                    </Text>
                  )}
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      <div
        className='border rounded-md p-4'
        style={{ borderColor: 'var(--semi-color-border)' }}
      >
        <div className='flex items-center gap-2 mb-1'>
          <Video size={16} />
          <Text strong>{t('模型绑定与资金策略')}</Text>
        </div>
        <Text type='tertiary'>
          {t(
            '模型名称精确匹配且区分大小写。绑定价格模板后接管原固定价格；未允许订阅时始终只扣钱包余额。',
          )}
        </Text>
        <div className='grid grid-cols-1 lg:grid-cols-[minmax(0,1fr)_260px_180px_auto] gap-3 mt-3'>
          <Select
            multiple
            allowCreate
            filter={selectFilter}
            autoClearSearchValue={false}
            searchPosition='dropdown'
            value={bindingModels}
            optionList={modelOptions}
            placeholder={t('选择或输入多个公开模型名称')}
            disabled={loading}
            onChange={setBindingModels}
          />
          <Select
            value={bindingProfile}
            optionList={profileOptions}
            disabled={loading}
            onChange={(value) => {
              setBindingProfile(value);
              if (value === POLICY_ONLY) setBindingSubscriptionEnabled(true);
            }}
          />
          <div
            className='flex items-center justify-between gap-3 px-3 rounded-md min-h-8'
            style={{ background: 'var(--semi-color-fill-0)' }}
          >
            <Text>{t('允许订阅')}</Text>
            <Switch
              aria-label={t('允许订阅扣费')}
              checked={
                bindingProfile === POLICY_ONLY || bindingSubscriptionEnabled
              }
              disabled={loading || bindingProfile === POLICY_ONLY}
              onChange={setBindingSubscriptionEnabled}
            />
          </div>
          <Button
            type='primary'
            icon={<Link2 size={15} />}
            loading={savingAction === 'binding:batch'}
            disabled={loading && savingAction !== 'binding:batch'}
            onClick={applyBindings}
          >
            {t('批量配置')}
          </Button>
        </div>

        {bindingRows.length === 0 ? (
          <Empty className='mt-4' description={t('暂无视频模型绑定')} />
        ) : isMobile ? (
          <div className='space-y-3 mt-4'>
            {bindingRows.map(renderMobileBinding)}
          </div>
        ) : (
          <Table
            className='mt-3'
            columns={bindingColumns}
            dataSource={bindingRows}
            pagination={bindingRows.length > 10 ? { pageSize: 10 } : false}
            size='small'
            scroll={{ x: 900 }}
          />
        )}
      </div>

      <Modal
        title={t('新增视频计价模板')}
        visible={createVisible}
        onCancel={() => setCreateVisible(false)}
        onOk={createProfile}
      >
        <div className='space-y-4'>
          <div>
            <Text strong>{t('模板 ID')}</Text>
            <Input
              className='mt-2'
              value={createDraft.id}
              placeholder={t('例如 seedance-720p')}
              onChange={(value) =>
                setCreateDraft((current) => ({ ...current, id: value }))
              }
            />
          </div>
          <div>
            <Text strong>{t('模板名称')}</Text>
            <Input
              className='mt-2'
              value={createDraft.name}
              placeholder={t('例如 Seedance 720p')}
              onChange={(value) =>
                setCreateDraft((current) => ({ ...current, name: value }))
              }
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}
