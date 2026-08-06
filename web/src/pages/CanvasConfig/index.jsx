import React, { useEffect, useMemo, useState } from 'react';
import {
  Banner,
  Button,
  Select,
  Spin,
  Switch,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { Image, Save, ShieldCheck, Video } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';

const EMPTY_CONFIG = {
  enabled: false,
  image_group: '',
  video_group: '',
  redirect_uris: [],
  image_models: [],
  video_models: [],
};

function ModelPreview({ icon, title, models }) {
  return (
    <div className='min-w-0 border-t border-semi-color-border pt-4'>
      <div className='mb-3 flex items-center gap-2 text-sm font-medium'>
        {icon}
        <span>{title}</span>
        <Tag size='small'>{models.length}</Tag>
      </div>
      {models.length ? (
        <div className='flex flex-wrap gap-2'>
          {models.slice(0, 12).map((model) => (
            <Tag key={model} color='green' type='light'>
              {model}
            </Tag>
          ))}
          {models.length > 12 ? <Tag>+{models.length - 12}</Tag> : null}
        </div>
      ) : (
        <Typography.Text type='tertiary' size='small'>
          当前分组没有匹配模型
        </Typography.Text>
      )}
    </div>
  );
}

export default function CanvasConfigPage() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [config, setConfig] = useState(EMPTY_CONFIG);
  const [groups, setGroups] = useState([]);
  const [redirectText, setRedirectText] = useState('');

  const imageGroup = useMemo(
    () => groups.find((group) => group.name === config.image_group),
    [groups, config.image_group],
  );
  const videoGroup = useMemo(
    () => groups.find((group) => group.name === config.video_group),
    [groups, config.video_group],
  );

  const load = async () => {
    setLoading(true);
    try {
      const response = await API.get('/api/canvas/admin/config');
      if (!response.data.success) {
        showError(response.data.message);
        return;
      }
      const nextConfig = response.data.data?.config || EMPTY_CONFIG;
      setConfig(nextConfig);
      setGroups(response.data.data?.groups || []);
      setRedirectText((nextConfig.redirect_uris || []).join('\n'));
    } catch {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const groupOptions = (kind) =>
    groups.map((group) => {
      const count = group[`${kind}_models`]?.length || 0;
      return {
        value: group.name,
        label: `${group.display_name || group.name} (${count})`,
        disabled: count === 0,
      };
    });

  const save = async () => {
    setSaving(true);
    try {
      const response = await API.put('/api/canvas/admin/config', {
        ...config,
        redirect_uris: redirectText
          .split(/\r?\n/)
          .map((value) => value.trim())
          .filter(Boolean),
      });
      if (!response.data.success) {
        showError(response.data.message);
        return;
      }
      const next = response.data.data;
      setConfig(next.config);
      setGroups(next.groups || []);
      setRedirectText((next.config.redirect_uris || []).join('\n'));
      showSuccess(t('保存成功'));
    } catch (error) {
      showError(error?.response?.data?.message || t('保存失败，请重试'));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className='mx-auto mt-[60px] w-full max-w-5xl px-4 pb-12'>
      <div className='mb-8 flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between'>
        <div>
          <div className='mb-2 flex items-center gap-3'>
            <ShieldCheck size={24} />
            <Typography.Title heading={3} style={{ margin: 0 }}>
              Canvas 配置
            </Typography.Title>
          </div>
          <Typography.Text type='secondary'>
            配置 Infinite Canvas 授权后自动创建的图片与视频专用令牌。
          </Typography.Text>
        </div>
        <Button
          theme='solid'
          type='primary'
          icon={<Save size={16} />}
          loading={saving}
          onClick={save}
        >
          保存配置
        </Button>
      </div>

      <Spin spinning={loading}>
        <div
          className='overflow-hidden rounded-lg border p-5 sm:p-6'
          style={{
            borderColor: 'var(--semi-color-border)',
            background: 'var(--semi-color-bg-0)',
          }}
        >
          <div className='flex items-start justify-between gap-6'>
            <div>
              <Typography.Text strong>启用 Canvas 授权</Typography.Text>
              <div className='mt-1 text-sm text-semi-color-text-2'>
                关闭后不会删除已有令牌，但新的授权和兑换会被拒绝。
              </div>
            </div>
            <Switch
              checked={config.enabled}
              onChange={(enabled) =>
                setConfig((current) => ({ ...current, enabled }))
              }
            />
          </div>
          <div className='my-6 h-px bg-semi-color-border' />
          <div className='grid grid-cols-1 gap-6 md:grid-cols-2'>
            <label className='block'>
              <span className='mb-2 block text-sm font-medium'>
                图片聚合分组
              </span>
              <Select
                className='w-full'
                filter
                value={config.image_group || undefined}
                optionList={groupOptions('image')}
                placeholder='选择包含生图模型的聚合分组'
                onChange={(image_group) =>
                  setConfig((current) => ({ ...current, image_group }))
                }
              />
            </label>
            <label className='block'>
              <span className='mb-2 block text-sm font-medium'>
                视频聚合分组
              </span>
              <Select
                className='w-full'
                filter
                value={config.video_group || undefined}
                optionList={groupOptions('video')}
                placeholder='选择包含视频模型的聚合分组'
                onChange={(video_group) =>
                  setConfig((current) => ({ ...current, video_group }))
                }
              />
            </label>
            <ModelPreview
              icon={<Image size={16} />}
              title='图片 Token 模型白名单'
              models={imageGroup?.image_models || []}
            />
            <ModelPreview
              icon={<Video size={16} />}
              title='视频 Token 模型白名单'
              models={videoGroup?.video_models || []}
            />
          </div>
          <div className='mt-7'>
            <label className='block'>
              <span className='mb-2 block text-sm font-medium'>
                回调 URI 白名单
              </span>
              <TextArea
                autosize={{ minRows: 3, maxRows: 8 }}
                value={redirectText}
                placeholder={
                  '每行一个完整地址\nhttp://localhost:3000/auth/supertoken/callback'
                }
                onChange={setRedirectText}
              />
            </label>
            <div className='mt-2 text-xs text-semi-color-text-2'>
              生产环境仅允许 HTTPS；localhost 和 127.0.0.1 可使用
              HTTP。地址必须精确匹配且不能包含查询参数。
            </div>
          </div>
          {config.enabled &&
          (!config.image_group ||
            !config.video_group ||
            !redirectText.trim()) ? (
            <Banner
              className='mt-6'
              type='warning'
              description='启用授权前必须完成两个分组和至少一个回调 URI。'
            />
          ) : null}
        </div>
      </Spin>
    </div>
  );
}
