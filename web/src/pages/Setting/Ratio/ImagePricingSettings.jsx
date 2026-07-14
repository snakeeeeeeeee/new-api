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
  Table,
  Tag,
  TagInput,
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
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  API,
  MAX_IMAGE_N,
  bindImagePricingModels,
  calculateImagePricingPreview,
  copyImagePricingProfile,
  deleteImagePricingProfile,
  normalizeImagePricing,
  selectFilter,
  showError,
  showSuccess,
  showWarning,
  validateImagePricing,
} from '../../../helpers';
import { useIsMobile } from '../../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const createProfileId = (profiles, base = 'image-pricing') => {
  let candidate = base;
  let suffix = 2;
  while (profiles[candidate]) {
    candidate = `${base}-${suffix}`;
    suffix += 1;
  }
  return candidate;
};

const createTier = (index) => ({
  key: index === 0 ? 'default' : `tier-${index + 1}`,
  upstream_value: index === 0 ? 'default' : `tier-${index + 1}`,
  aliases: [],
  unit_price: 0,
});

const cloneConfig = (config) => structuredClone(config);

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
  return `$${number.toFixed(6).replace(/0+$/, '').replace(/\.$/, '')}`;
};

export default function ImagePricingSettings({ options, refresh }) {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [config, setConfig] = useState(() =>
    normalizeImagePricing(options.ImagePricing),
  );
  const [selectedProfileId, setSelectedProfileId] = useState('');
  const [enabledModels, setEnabledModels] = useState([]);
  const [loading, setLoading] = useState(false);
  const [createVisible, setCreateVisible] = useState(false);
  const [createDraft, setCreateDraft] = useState({
    id: '',
    name: '',
    parameter: 'quality',
  });
  const [bindingModels, setBindingModels] = useState([]);
  const [bindingMaxN, setBindingMaxN] = useState(MAX_IMAGE_N);
  const [previewValue, setPreviewValue] = useState('');
  const [previewN, setPreviewN] = useState(1);
  const [previewGroupRatio, setPreviewGroupRatio] = useState(1);

  useEffect(() => {
    const nextConfig = normalizeImagePricing(options.ImagePricing);
    const profileIds = Object.keys(nextConfig.profiles);
    setConfig(nextConfig);
    setSelectedProfileId((current) =>
      current && nextConfig.profiles[current] ? current : profileIds[0] || '',
    );
  }, [options.ImagePricing]);

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
    () => validateImagePricing(config, t),
    [config, t],
  );
  const boundModelsForSelectedProfile = useMemo(
    () =>
      Object.entries(config.model_bindings)
        .filter(([, binding]) => binding.profile === selectedProfileId)
        .map(([model]) => model),
    [config.model_bindings, selectedProfileId],
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
  const preview = useMemo(
    () =>
      selectedProfile
        ? calculateImagePricingPreview({
            profile: selectedProfile,
            rawValue: previewValue,
            n: previewN,
            groupRatio: previewGroupRatio,
          })
        : null,
    [previewGroupRatio, previewN, previewValue, selectedProfile],
  );

  const updateProfile = (updater) => {
    if (!selectedProfileId) return;
    setConfig((current) => {
      const next = cloneConfig(current);
      const profile = next.profiles[selectedProfileId];
      next.profiles[selectedProfileId] =
        typeof updater === 'function' ? updater(profile) : updater;
      return next;
    });
  };

  const updateTier = (index, patch) => {
    updateProfile((profile) => {
      const previousTier = profile.tiers[index];
      const updatesDefault =
        Object.prototype.hasOwnProperty.call(patch, 'key') &&
        previousTier?.key?.toLowerCase() ===
          profile.default_tier?.toLowerCase();
      return {
        ...profile,
        default_tier: updatesDefault ? patch.key : profile.default_tier,
        tiers: profile.tiers.map((tier, tierIndex) =>
          tierIndex === index ? { ...tier, ...patch } : tier,
        ),
      };
    });
  };

  const openCreateModal = () => {
    const id = createProfileId(config.profiles);
    setCreateDraft({ id, name: '', parameter: 'quality' });
    setCreateVisible(true);
  };

  const createProfile = () => {
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
    const tier = createTier(0);
    setConfig((current) => ({
      ...current,
      profiles: {
        ...current.profiles,
        [id]: {
          name,
          parameter: createDraft.parameter,
          default_tier: tier.key,
          tiers: [tier],
        },
      },
    }));
    setSelectedProfileId(id);
    setPreviewValue('');
    setCreateVisible(false);
  };

  const copyProfile = () => {
    if (!selectedProfile) return;
    const id = createProfileId(config.profiles, `${selectedProfileId}-copy`);
    setConfig((current) =>
      copyImagePricingProfile(
        current,
        selectedProfileId,
        id,
        t('{{name}} 副本', { name: selectedProfile.name }),
      ),
    );
    setSelectedProfileId(id);
  };

  const deleteProfile = () => {
    if (!selectedProfileId) return;
    setConfig((current) =>
      deleteImagePricingProfile(current, selectedProfileId),
    );
    const nextProfileId = profileIds.find((id) => id !== selectedProfileId);
    setSelectedProfileId(nextProfileId || '');
  };

  const addTier = () => {
    if (!selectedProfile) return;
    const tier = createTier(selectedProfile.tiers.length);
    const existing = new Set(selectedProfile.tiers.map((item) => item.key));
    let suffix = selectedProfile.tiers.length + 1;
    while (existing.has(tier.key)) {
      tier.key = `tier-${suffix}`;
      tier.upstream_value = tier.key;
      suffix += 1;
    }
    updateProfile((profile) => ({
      ...profile,
      tiers: [...profile.tiers, tier],
    }));
  };

  const deleteTier = (index) => {
    updateProfile((profile) => {
      const removed = profile.tiers[index];
      const tiers = profile.tiers.filter((_, tierIndex) => tierIndex !== index);
      return {
        ...profile,
        tiers,
        default_tier:
          profile.default_tier === removed.key
            ? tiers[0]?.key || ''
            : profile.default_tier,
      };
    });
  };

  const applyBindings = () => {
    if (!selectedProfileId || bindingModels.length === 0) {
      showWarning(t('请先选择模板和至少一个模型'));
      return;
    }
    const maxN = Number(bindingMaxN);
    if (!Number.isInteger(maxN) || maxN < 1 || maxN > MAX_IMAGE_N) {
      showWarning(t('最大张数必须在 1 到 {{max}} 之间', { max: MAX_IMAGE_N }));
      return;
    }
    setConfig((current) =>
      bindImagePricingModels(current, bindingModels, selectedProfileId, maxN),
    );
    setBindingModels([]);
    showSuccess(
      t('已绑定 {{count}} 个模型，保存后生效', {
        count: bindingModels.length,
      }),
    );
  };

  const updateBinding = (model, patch) => {
    setConfig((current) => ({
      ...current,
      model_bindings: {
        ...current.model_bindings,
        [model]: { ...current.model_bindings[model], ...patch },
      },
    }));
  };

  const removeBinding = (model) => {
    setConfig((current) => {
      const next = cloneConfig(current);
      delete next.model_bindings[model];
      return next;
    });
  };

  const save = async () => {
    if (validationErrors.length > 0) {
      showError(validationErrors[0]);
      return;
    }
    setLoading(true);
    try {
      const response = await API.put('/api/option/', {
        key: 'ImagePricing',
        value: JSON.stringify(config),
      });
      if (!response.data?.success) {
        showError(response.data?.message || t('保存失败'));
        return;
      }
      showSuccess(t('保存成功'));
      await refresh();
    } catch (error) {
      showError(error.message || t('保存失败'));
    } finally {
      setLoading(false);
    }
  };

  const tierColumns = [
    {
      title: t('客户端值'),
      dataIndex: 'key',
      width: 150,
      render: (_, record, index) => (
        <Input
          value={record.key}
          placeholder={t('例如 low')}
          onChange={(value) => updateTier(index, { key: value })}
        />
      ),
    },
    {
      title: t('上游值'),
      dataIndex: 'upstream_value',
      width: 150,
      render: (_, record, index) => (
        <Input
          value={record.upstream_value}
          placeholder={t('例如 low')}
          onChange={(value) => updateTier(index, { upstream_value: value })}
        />
      ),
    },
    {
      title: t('别名'),
      dataIndex: 'aliases',
      width: 220,
      render: (_, record, index) => (
        <TagInput
          value={record.aliases}
          placeholder={t('输入后回车，例如 auto')}
          onChange={(value) => updateTier(index, { aliases: value })}
          style={{ width: '100%' }}
        />
      ),
    },
    {
      title: t('单张美元价'),
      dataIndex: 'unit_price',
      width: 150,
      render: (_, record, index) => (
        <InputNumber
          min={0}
          precision={8}
          value={Number.isFinite(record.unit_price) ? record.unit_price : null}
          prefix={<DollarSign size={13} />}
          onChange={(value) => updateTier(index, { unit_price: value })}
          style={{ width: '100%' }}
        />
      ),
    },
    {
      title: t('操作'),
      width: 64,
      render: (_, record, index) => (
        <Button
          aria-label={t('删除档位')}
          theme='borderless'
          type='danger'
          icon={<Trash2 size={15} />}
          disabled={selectedProfile?.tiers.length <= 1}
          onClick={() => deleteTier(index)}
        />
      ),
    },
  ];

  const bindingColumns = [
    {
      title: t('公开模型名称'),
      dataIndex: 'model',
      width: 260,
      render: (model) => <Text copyable={{ content: model }}>{model}</Text>,
    },
    {
      title: t('计价模板'),
      dataIndex: 'profile',
      width: 220,
      render: (profileId, record) => (
        <Select
          value={profileId}
          optionList={profileIds.map((id) => ({
            value: id,
            label: config.profiles[id]?.name || id,
          }))}
          onChange={(value) => updateBinding(record.model, { profile: value })}
          style={{ width: '100%' }}
        />
      ),
    },
    {
      title: t('最大张数'),
      dataIndex: 'max_n',
      width: 130,
      render: (maxN, record) => (
        <InputNumber
          min={1}
          max={MAX_IMAGE_N}
          precision={0}
          value={maxN}
          onChange={(value) => updateBinding(record.model, { max_n: value })}
          style={{ width: '100%' }}
        />
      ),
    },
    {
      title: t('操作'),
      width: 64,
      render: (_, record) => (
        <Button
          aria-label={t('解除绑定')}
          theme='borderless'
          type='danger'
          icon={<Unlink size={15} />}
          onClick={() => removeBinding(record.model)}
        />
      ),
    },
  ];

  const bindingRows = Object.entries(config.model_bindings)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([model, binding]) => ({ key: model, model, ...binding }));

  return (
    <div className='space-y-5'>
      <div className='flex flex-col md:flex-row md:items-center justify-between gap-3'>
        <div>
          <Title heading={5}>{t('图片参数计价')}</Title>
          <Text type='tertiary'>
            {t(
              '按质量、尺寸或分辨率中的一个参数选择单张价格，n 仅作为张数乘数。',
            )}
          </Text>
        </div>
        <Button
          type='primary'
          icon={<Save size={16} />}
          loading={loading}
          onClick={save}
        >
          {t('保存图片计价配置')}
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
            <Text strong>{t('计价模板')}</Text>
            <Space spacing={4}>
              <Button
                aria-label={t('新增模板')}
                theme='borderless'
                icon={<Plus size={16} />}
                onClick={openCreateModal}
              />
              <Button
                aria-label={t('复制模板')}
                theme='borderless'
                icon={<Copy size={16} />}
                disabled={!selectedProfile}
                onClick={copyProfile}
              />
              <Popconfirm
                title={t('删除模板会同时解除使用它的模型绑定，确定继续吗？')}
                onConfirm={deleteProfile}
              >
                <Button
                  aria-label={t('删除模板')}
                  theme='borderless'
                  type='danger'
                  icon={<Trash2 size={16} />}
                  disabled={!selectedProfile}
                />
              </Popconfirm>
            </Space>
          </div>
          {profileIds.length === 0 ? (
            <Empty
              title={t('暂无图片计价模板')}
              description={t('新增模板后即可配置档位并绑定公开模型。')}
            />
          ) : (
            <div className='space-y-2'>
              {profileIds.map((id) => {
                const profile = config.profiles[id];
                const count = Object.values(config.model_bindings).filter(
                  (binding) => binding.profile === id,
                ).length;
                const selected = id === selectedProfileId;
                return (
                  <button
                    key={id}
                    type='button'
                    className='w-full text-left px-3 py-2 border rounded-md'
                    style={{
                      borderColor: selected
                        ? 'var(--semi-color-primary)'
                        : 'var(--semi-color-border)',
                      background: selected
                        ? 'var(--semi-color-primary-light-default)'
                        : 'transparent',
                    }}
                    onClick={() => {
                      setSelectedProfileId(id);
                      setPreviewValue('');
                    }}
                  >
                    <div className='font-medium truncate'>{profile.name}</div>
                    <div className='flex items-center gap-2 mt-1 text-xs text-gray-500'>
                      <span>{profile.parameter}</span>
                      <span>·</span>
                      <span>
                        {t('{{count}} 档', { count: profile.tiers.length })}
                      </span>
                      <span>·</span>
                      <span>{t('{{count}} 个模型', { count })}</span>
                    </div>
                  </button>
                );
              })}
            </div>
          )}
        </div>

        <div className='min-w-0'>
          {!selectedProfile ? (
            <Empty
              title={t('选择或新增计价模板')}
              description={t('每个模板只使用一个计价参数。')}
            />
          ) : (
            <div className='space-y-5'>
              <div
                className='border rounded-md p-4'
                style={{ borderColor: 'var(--semi-color-border)' }}
              >
                <div className='grid grid-cols-1 md:grid-cols-3 gap-3'>
                  <div>
                    <Text strong>{t('模板名称')}</Text>
                    <Input
                      className='mt-2'
                      value={selectedProfile.name}
                      onChange={(value) =>
                        updateProfile((profile) => ({
                          ...profile,
                          name: value,
                        }))
                      }
                    />
                  </div>
                  <div>
                    <Text strong>{t('计价参数')}</Text>
                    <Select
                      className='mt-2 w-full'
                      value={selectedProfile.parameter}
                      optionList={[
                        { value: 'quality', label: 'quality' },
                        { value: 'size', label: 'size' },
                        { value: 'resolution', label: 'resolution' },
                      ]}
                      onChange={(value) =>
                        updateProfile((profile) => ({
                          ...profile,
                          parameter: value,
                        }))
                      }
                    />
                  </div>
                  <div>
                    <Text strong>{t('默认档位')}</Text>
                    <Select
                      className='mt-2 w-full'
                      value={selectedProfile.default_tier}
                      optionList={selectedProfile.tiers.map((tier) => ({
                        value: tier.key,
                        label: tier.key || t('未命名档位'),
                      }))}
                      onChange={(value) =>
                        updateProfile((profile) => ({
                          ...profile,
                          default_tier: value,
                        }))
                      }
                    />
                  </div>
                </div>

                <div className='flex items-center justify-between mt-5 mb-2'>
                  <div>
                    <Text strong>{t('价格档位')}</Text>
                    <div className='text-xs text-gray-500 mt-1'>
                      {t('客户端值和别名不区分大小写，上游值按原样透传。')}
                    </div>
                  </div>
                  <Button icon={<Plus size={15} />} onClick={addTier}>
                    {t('新增档位')}
                  </Button>
                </div>
                <Table
                  columns={tierColumns}
                  dataSource={selectedProfile.tiers.map((tier, index) => ({
                    ...tier,
                    rowKey: `${selectedProfileId}-${index}`,
                  }))}
                  rowKey='rowKey'
                  pagination={false}
                  size='small'
                  scroll={{ x: 760 }}
                />
              </div>

              <div
                className='border rounded-md p-4'
                style={{ borderColor: 'var(--semi-color-border)' }}
              >
                <Text strong>{t('实时费用预览')}</Text>
                <div className='grid grid-cols-1 sm:grid-cols-3 gap-3 mt-3'>
                  <div>
                    <Text type='tertiary'>{selectedProfile.parameter}</Text>
                    <Select
                      className='mt-1 w-full'
                      value={previewValue}
                      optionList={[
                        { value: '', label: t('未传（使用默认档位）') },
                        ...selectedProfile.tiers.flatMap((tier) => [
                          { value: tier.key, label: tier.key || '-' },
                          ...(tier.aliases || []).map((alias) => ({
                            value: alias,
                            label: `${alias} (${t('别名')})`,
                          })),
                        ]),
                      ]}
                      onChange={setPreviewValue}
                    />
                  </div>
                  <div>
                    <Text type='tertiary'>n</Text>
                    <InputNumber
                      className='mt-1 w-full'
                      min={1}
                      max={MAX_IMAGE_N}
                      precision={0}
                      value={previewN}
                      onChange={setPreviewN}
                    />
                  </div>
                  <div>
                    <Text type='tertiary'>{t('分组倍率')}</Text>
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
                    <div className='flex flex-wrap items-center gap-2'>
                      <Tag color='blue'>{preview.effective_value}</Tag>
                      <span>
                        {formatUSD(preview.unit_price)} × {preview.n} ×{' '}
                        {preview.group_ratio} ={' '}
                        <strong>{formatUSD(preview.total)}</strong>
                      </span>
                      <Text type='tertiary'>
                        {preview.source === 'default'
                          ? t('命中默认档位')
                          : preview.source === 'alias'
                            ? t('通过别名命中')
                            : t('直接命中')}
                      </Text>
                    </div>
                  ) : (
                    <Text type='danger'>{t('当前参数无法命中价格档位')}</Text>
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
          <Link2 size={16} />
          <Text strong>{t('模型绑定')}</Text>
        </div>
        <Text type='tertiary'>
          {t(
            '公开模型名称可自定义；计价在模型映射前完成，解绑后恢复原模型价格或倍率。',
          )}
        </Text>
        <div className='grid grid-cols-1 md:grid-cols-[minmax(0,1fr)_150px_auto] gap-3 mt-3'>
          <Select
            multiple
            allowCreate
            filter={selectFilter}
            autoClearSearchValue={false}
            searchPosition='dropdown'
            value={bindingModels}
            optionList={modelOptions}
            placeholder={t('选择或输入多个公开模型名称')}
            onChange={setBindingModels}
          />
          <InputNumber
            min={1}
            max={MAX_IMAGE_N}
            precision={0}
            value={bindingMaxN}
            prefix={t('最大 n')}
            onChange={setBindingMaxN}
            style={{ width: '100%' }}
          />
          <Button
            type='primary'
            icon={<Link2 size={15} />}
            disabled={!selectedProfile}
            onClick={applyBindings}
          >
            {t('批量绑定')}
          </Button>
        </div>
        {selectedProfile && boundModelsForSelectedProfile.length > 0 ? (
          <div className='text-xs text-gray-500 mt-2'>
            {t('当前模板已绑定 {{count}} 个模型', {
              count: boundModelsForSelectedProfile.length,
            })}
          </div>
        ) : null}
        <Table
          className='mt-3'
          columns={bindingColumns}
          dataSource={bindingRows}
          pagination={bindingRows.length > 10 ? { pageSize: 10 } : false}
          size='small'
          scroll={{ x: 720 }}
          empty={<Empty description={t('暂无模型绑定')} />}
        />
      </div>

      <Modal
        title={t('新增图片计价模板')}
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
              placeholder={t('例如 adobe-quality-v1')}
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
              placeholder={t('例如 ADOBE 质量按张')}
              onChange={(value) =>
                setCreateDraft((current) => ({ ...current, name: value }))
              }
            />
          </div>
          <div>
            <Text strong>{t('计价参数')}</Text>
            <Select
              className='mt-2 w-full'
              value={createDraft.parameter}
              optionList={[
                { value: 'quality', label: 'quality' },
                { value: 'size', label: 'size' },
                { value: 'resolution', label: 'resolution' },
              ]}
              onChange={(value) =>
                setCreateDraft((current) => ({
                  ...current,
                  parameter: value,
                }))
              }
            />
          </div>
        </div>
      </Modal>
    </div>
  );
}
