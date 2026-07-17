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
  Button,
  Empty,
  Form,
  ImagePreview,
  Pagination,
  Select,
  SideSheet,
  Space,
  Table,
  Tabs,
  Tag,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  Copy,
  Download,
  ExternalLink,
  FileSpreadsheet,
  Grid3X3,
  Image as ImageIcon,
  List,
  RefreshCcw,
  Video,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import {
  API,
  copy,
  downloadTextAsFile,
  isAdmin,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { DATE_RANGE_PRESETS } from '../../constants/console.constants';
import ResourceCenterDocs from './docs/ResourceCenterDocs';
import WebhookTab from './webhooks/WebhookTab';

const { Text, Title } = Typography;
const DIRECT_DOWNLOAD_LIMIT = 20;

const assetTypeOptions = [
  { value: '', label: '全部' },
  { value: 'image', label: '图片' },
  { value: 'video', label: '视频' },
  { value: 'audio', label: '音频' },
];

const statusOptions = [
  { value: '', label: '全部状态' },
  { value: 'available', label: '可用' },
  { value: 'blocked', label: '已屏蔽' },
  { value: 'deleted', label: '已删除' },
  { value: 'unavailable', label: '不可用' },
];

function assetTypeLabel(type, t) {
  switch (type) {
    case 'image':
      return t('图片');
    case 'video':
      return t('视频');
    case 'audio':
      return t('音频');
    default:
      return t('文件');
  }
}

function assetTypeIcon(type) {
  if (type === 'video') return <Video size={14} />;
  return <ImageIcon size={14} />;
}

function statusTag(status, t) {
  switch (status) {
    case 'available':
      return <Tag color='green'>{t('可用')}</Tag>;
    case 'blocked':
      return <Tag color='red'>{t('已屏蔽')}</Tag>;
    case 'deleted':
      return <Tag color='grey'>{t('已删除')}</Tag>;
    case 'unavailable':
      return <Tag color='orange'>{t('不可用')}</Tag>;
    default:
      return <Tag>{status || t('未知')}</Tag>;
  }
}

function buildDownloadName(asset) {
  if (asset.filename) return asset.filename;
  const ext = asset.asset_type === 'video' ? 'mp4' : 'png';
  return `${asset.asset_id}.${ext}`;
}

function triggerDownload(url, filename) {
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.target = '_blank';
  a.rel = 'noreferrer';
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
}

function AssetPreview({ asset, className = '' }) {
  if (!asset) return null;
  if (asset.asset_type === 'video') {
    return (
      <video
        src={asset.url}
        poster={asset.thumbnail_url || undefined}
        className={`w-full h-full object-cover bg-black ${className}`}
        controls
        preload='metadata'
      />
    );
  }
  if (asset.asset_type === 'image') {
    return (
      <img
        src={asset.thumbnail_url || asset.url}
        alt={asset.filename || asset.asset_id}
        className={`w-full h-full object-cover ${className}`}
        loading='lazy'
      />
    );
  }
  return (
    <div className={`flex items-center justify-center bg-gray-50 ${className}`}>
      <ImageIcon size={32} />
    </div>
  );
}

export default function AssetsPage() {
  const { t } = useTranslation();
  const adminUser = isAdmin();
  const [formApi, setFormApi] = useState(null);
  const [activeMainTab, setActiveMainTab] = useState('assets');
  const [assets, setAssets] = useState([]);
  const [loading, setLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [total, setTotal] = useState(0);
  const [assetType, setAssetType] = useState('');
  const [viewMode, setViewMode] = useState('grid');
  const [selectedRowKeys, setSelectedRowKeys] = useState([]);
  const [detailAsset, setDetailAsset] = useState(null);
  const [previewImage, setPreviewImage] = useState('');

  const now = new Date();
  const zeroNow = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const formInitValues = {
    asset_type: '',
    status: '',
    task_id: '',
    model: '',
    channel_id: '',
    user_id: '',
    keyword: '',
    dateRange: [
      timestamp2string(zeroNow.getTime() / 1000),
      timestamp2string(now.getTime() / 1000 + 3600),
    ],
  };

  const selectedAssets = useMemo(
    () => assets.filter((asset) => selectedRowKeys.includes(asset.asset_id)),
    [assets, selectedRowKeys],
  );

  const getQueryParams = () => {
    const values = formApi ? formApi.getValues() : {};
    let startTimestamp = '';
    let endTimestamp = '';
    if (
      values.dateRange &&
      Array.isArray(values.dateRange) &&
      values.dateRange.length === 2
    ) {
      startTimestamp = parseInt(Date.parse(values.dateRange[0]) / 1000);
      endTimestamp = parseInt(Date.parse(values.dateRange[1]) / 1000);
    }
    return {
      asset_type: assetType || values.asset_type || '',
      status: values.status || '',
      task_id: values.task_id || '',
      model: values.model || '',
      channel_id: values.channel_id || '',
      user_id: values.user_id || '',
      keyword: values.keyword || '',
      start_timestamp: startTimestamp || '',
      end_timestamp: endTimestamp || '',
    };
  };

  const loadAssets = async (page = activePage, size = pageSize) => {
    setLoading(true);
    try {
      const endpoint = adminUser ? '/api/assets/' : '/api/assets/self';
      const params = new URLSearchParams({
        p: String(page),
        page_size: String(size),
      });
      const queryParams = getQueryParams();
      Object.entries(queryParams).forEach(([key, value]) => {
        if (value !== undefined && value !== null && String(value) !== '') {
          params.set(key, String(value));
        }
      });
      const res = await API.get(`${endpoint}?${params.toString()}`);
      const { success, data, message } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setAssets(data.items || []);
      setTotal(data.total || 0);
      setActivePage(data.page || page);
      setPageSize(data.page_size || size);
      setSelectedRowKeys([]);
    } catch (error) {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadAssets(1, pageSize);
  }, [assetType]);

  const exportCsv = async () => {
    const endpoint = adminUser
      ? '/api/assets/export'
      : '/api/assets/self/export';
    const params = new URLSearchParams();
    const queryParams = getQueryParams();
    Object.entries(queryParams).forEach(([key, value]) => {
      if (value !== undefined && value !== null && String(value) !== '') {
        params.set(key, String(value));
      }
    });
    const res = await API.get(`${endpoint}?${params.toString()}`);
    downloadTextAsFile(res.data, `assets-${Date.now()}.csv`);
    showSuccess(t('已导出 CSV'));
  };

  const copySelectedUrls = async () => {
    const urls = selectedAssets.map((asset) => asset.url).filter(Boolean);
    if (urls.length === 0) return;
    if (await copy(urls.join('\n'))) {
      showSuccess(t('已复制链接'));
    }
  };

  const downloadSelected = () => {
    if (selectedAssets.length === 0) return;
    if (selectedAssets.length > DIRECT_DOWNLOAD_LIMIT) {
      showError(t('选中资源过多，请导出 CSV 后使用下载工具处理'));
      return;
    }
    selectedAssets.forEach((asset) => {
      triggerDownload(asset.url, buildDownloadName(asset));
    });
  };

  const columns = [
    {
      title: t('资源'),
      dataIndex: 'url',
      render: (_, record) => (
        <Space>
          <div className='w-14 h-14 rounded-md overflow-hidden border border-solid border-semi-color-border'>
            <AssetPreview asset={record} />
          </div>
          <div className='min-w-0'>
            <div className='flex items-center gap-2'>
              <Tag prefixIcon={assetTypeIcon(record.asset_type)}>
                {assetTypeLabel(record.asset_type, t)}
              </Tag>
              {statusTag(record.status, t)}
            </div>
            <Text size='small' ellipsis={{ showTooltip: true }}>
              {record.asset_id}
            </Text>
          </div>
        </Space>
      ),
    },
    {
      title: t('模型'),
      dataIndex: 'model',
      render: (text) => text || '-',
    },
    {
      title: t('任务 ID'),
      dataIndex: 'task_id',
      render: (text) => <Text copyable>{text}</Text>,
    },
    {
      title: t('时间'),
      dataIndex: 'created_at',
      render: (time) => timestamp2string(time),
    },
    adminUser && {
      title: t('用户'),
      dataIndex: 'username',
      render: (_, record) => record.username || record.user_id || '-',
    },
    {
      title: t('操作'),
      dataIndex: 'actions',
      render: (_, record) => (
        <Space>
          <Tooltip content={t('预览')}>
            <Button
              icon={<Grid3X3 size={14} />}
              size='small'
              onClick={() => setDetailAsset(record)}
            />
          </Tooltip>
          <Tooltip content={t('复制链接')}>
            <Button
              icon={<Copy size={14} />}
              size='small'
              onClick={() =>
                copy(record.url).then(
                  (ok) => ok && showSuccess(t('已复制链接')),
                )
              }
            />
          </Tooltip>
          <Tooltip content={t('打开')}>
            <Button
              icon={<ExternalLink size={14} />}
              size='small'
              onClick={() => window.open(record.url, '_blank', 'noreferrer')}
            />
          </Tooltip>
        </Space>
      ),
    },
  ].filter(Boolean);

  const renderAssetsTab = () => (
    <div className='flex flex-col gap-3'>
      <div className='flex flex-col md:flex-row md:items-center md:justify-between gap-2'>
        <div>
          <Title heading={5} className='!mb-1'>
            {t('资源列表')}
          </Title>
          <Text type='tertiary'>
            {t('集中查看、筛选和导出异步生成的图片与视频资源')}
          </Text>
        </div>
        <Space wrap>
          <Button
            icon={<RefreshCcw size={14} />}
            onClick={() => loadAssets(1, pageSize)}
            loading={loading}
          >
            {t('刷新')}
          </Button>
          <Button icon={<FileSpreadsheet size={14} />} onClick={exportCsv}>
            {t('导出 CSV')}
          </Button>
        </Space>
      </div>

      <Form
        initValues={formInitValues}
        getFormApi={setFormApi}
        onSubmit={() => loadAssets(1, pageSize)}
        allowEmpty
        autoComplete='off'
        layout='vertical'
      >
        <div className='grid grid-cols-1 md:grid-cols-2 xl:grid-cols-6 gap-2'>
          <div className='xl:col-span-2'>
            <Form.DatePicker
              field='dateRange'
              className='w-full'
              type='dateTimeRange'
              placeholder={[t('开始时间'), t('结束时间')]}
              showClear
              pure
              size='small'
              presets={DATE_RANGE_PRESETS.map((preset) => ({
                text: t(preset.text),
                start: preset.start(),
                end: preset.end(),
              }))}
            />
          </div>
          <Form.Input
            field='task_id'
            placeholder={t('任务 ID')}
            showClear
            pure
            size='small'
          />
          <Form.Input
            field='model'
            placeholder={t('模型')}
            showClear
            pure
            size='small'
          />
          <Form.Input
            field='keyword'
            placeholder={t('关键词')}
            showClear
            pure
            size='small'
          />
          <Form.Select
            field='status'
            placeholder={t('状态')}
            showClear
            pure
            size='small'
          >
            {statusOptions.map((option) => (
              <Select.Option key={option.value} value={option.value}>
                {t(option.label)}
              </Select.Option>
            ))}
          </Form.Select>
          {adminUser && (
            <>
              <Form.Input
                field='user_id'
                placeholder={t('用户 ID')}
                showClear
                pure
                size='small'
              />
              <Form.Input
                field='channel_id'
                placeholder={t('渠道 ID')}
                showClear
                pure
                size='small'
              />
            </>
          )}
        </div>
        <div className='flex justify-between items-center mt-2 gap-2 flex-wrap'>
          <Tabs
            type='button'
            activeKey={assetType}
            onChange={(key) => setAssetType(key)}
            tabList={assetTypeOptions.slice(0, 3).map((option) => ({
              tab: t(option.label),
              itemKey: option.value,
            }))}
          />
          <Space wrap>
            <Button htmlType='submit' type='tertiary' loading={loading}>
              {t('查询')}
            </Button>
            <Button
              type='tertiary'
              onClick={() => {
                formApi?.reset();
                setAssetType('');
                setTimeout(() => loadAssets(1, pageSize), 100);
              }}
            >
              {t('重置')}
            </Button>
            <Button
              icon={
                viewMode === 'grid' ? <List size={14} /> : <Grid3X3 size={14} />
              }
              onClick={() =>
                setViewMode(viewMode === 'grid' ? 'table' : 'grid')
              }
            >
              {viewMode === 'grid' ? t('表格') : t('网格')}
            </Button>
          </Space>
        </div>
      </Form>

      <div className='flex items-center justify-between gap-2 min-h-[36px]'>
        <Text type='tertiary'>
          {selectedAssets.length > 0
            ? t('已选择 {{count}} 个资源', { count: selectedAssets.length })
            : t('共 {{count}} 个资源', { count: total })}
        </Text>
        <Space wrap>
          <Button
            icon={<Copy size={14} />}
            disabled={selectedAssets.length === 0}
            onClick={copySelectedUrls}
          >
            {t('复制链接')}
          </Button>
          <Button
            icon={<Download size={14} />}
            disabled={selectedAssets.length === 0}
            onClick={downloadSelected}
          >
            {t('下载选中')}
          </Button>
        </Space>
      </div>

      {viewMode === 'grid' ? (
        <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-3'>
          {assets.map((asset) => (
            <div
              key={asset.asset_id}
              className={`border border-solid rounded-lg overflow-hidden bg-white dark:bg-semi-color-bg-1 ${
                selectedRowKeys.includes(asset.asset_id)
                  ? 'border-semi-color-primary'
                  : 'border-semi-color-border'
              }`}
            >
              <button
                type='button'
                className='w-full aspect-[4/3] border-0 p-0 cursor-pointer bg-transparent'
                onClick={() => setDetailAsset(asset)}
              >
                <AssetPreview asset={asset} />
              </button>
              <div className='p-3 flex flex-col gap-2'>
                <div className='flex items-center justify-between gap-2'>
                  <Tag prefixIcon={assetTypeIcon(asset.asset_type)}>
                    {assetTypeLabel(asset.asset_type, t)}
                  </Tag>
                  {statusTag(asset.status, t)}
                </div>
                <Text ellipsis={{ showTooltip: true }} strong>
                  {asset.model || asset.asset_id}
                </Text>
                <Text
                  size='small'
                  type='tertiary'
                  ellipsis={{ showTooltip: true }}
                >
                  {asset.task_id}
                </Text>
                <Text size='small' type='tertiary'>
                  {timestamp2string(asset.created_at)}
                </Text>
                <div className='flex items-center justify-between'>
                  <label className='flex items-center gap-2 text-sm'>
                    <input
                      type='checkbox'
                      checked={selectedRowKeys.includes(asset.asset_id)}
                      onChange={(e) => {
                        setSelectedRowKeys((current) =>
                          e.target.checked
                            ? [...current, asset.asset_id]
                            : current.filter((key) => key !== asset.asset_id),
                        );
                      }}
                    />
                    {t('选择')}
                  </label>
                  <Space>
                    <Button
                      icon={<Copy size={14} />}
                      size='small'
                      onClick={() =>
                        copy(asset.url).then(
                          (ok) => ok && showSuccess(t('已复制链接')),
                        )
                      }
                    />
                    <Button
                      icon={<ExternalLink size={14} />}
                      size='small'
                      onClick={() =>
                        window.open(asset.url, '_blank', 'noreferrer')
                      }
                    />
                  </Space>
                </div>
              </div>
            </div>
          ))}
          {!loading && assets.length === 0 && (
            <div className='col-span-full py-12'>
              <Empty description={t('暂无资源')} />
            </div>
          )}
        </div>
      ) : (
        <Table
          columns={columns}
          dataSource={assets}
          rowKey='asset_id'
          loading={loading}
          pagination={false}
          rowSelection={{
            selectedRowKeys,
            onChange: setSelectedRowKeys,
          }}
        />
      )}

      <div className='flex justify-end'>
        <Pagination
          currentPage={activePage}
          pageSize={pageSize}
          total={total}
          showSizeChanger
          pageSizeOptions={[10, 20, 50, 100]}
          onPageChange={(page) => loadAssets(page, pageSize)}
          onPageSizeChange={(size) => loadAssets(1, size)}
        />
      </div>
    </div>
  );

  return (
    <div className='p-2'>
      <div className='flex flex-col gap-3'>
        <div>
          <Title heading={4} className='!mb-1'>
            {t('资源管理中心')}
          </Title>
          <Text type='tertiary'>
            {t('管理异步任务生成的图片、视频和 Webhook 通知')}
          </Text>
        </div>
        <Tabs
          type='line'
          activeKey={activeMainTab}
          onChange={setActiveMainTab}
          tabList={[
            { tab: t('资源列表'), itemKey: 'assets' },
            { tab: 'Webhook', itemKey: 'webhooks' },
            { tab: t('使用文档'), itemKey: 'docs' },
          ]}
        />
        {activeMainTab === 'assets' && renderAssetsTab()}
        {activeMainTab === 'webhooks' && <WebhookTab />}
        {activeMainTab === 'docs' && (
          <ResourceCenterDocs
            onOpenWebhook={() => setActiveMainTab('webhooks')}
          />
        )}
      </div>

      <SideSheet
        placement='right'
        title={t('资源详情')}
        visible={!!detailAsset}
        onCancel={() => setDetailAsset(null)}
        width='min(560px, 100vw)'
        footer={null}
      >
        {detailAsset && (
          <div className='flex flex-col gap-3'>
            <div className='w-full aspect-video rounded-lg overflow-hidden border border-solid border-semi-color-border'>
              {detailAsset.asset_type === 'image' ? (
                <button
                  type='button'
                  className='w-full h-full border-0 p-0 cursor-pointer bg-transparent'
                  onClick={() => setPreviewImage(detailAsset.url)}
                >
                  <AssetPreview asset={detailAsset} />
                </button>
              ) : (
                <AssetPreview asset={detailAsset} />
              )}
            </div>
            <Space wrap>
              <Button
                icon={<Copy size={14} />}
                onClick={() =>
                  copy(detailAsset.url).then(
                    (ok) => ok && showSuccess(t('已复制链接')),
                  )
                }
              >
                {t('复制链接')}
              </Button>
              <Button
                icon={<ExternalLink size={14} />}
                onClick={() =>
                  window.open(detailAsset.url, '_blank', 'noreferrer')
                }
              >
                {t('打开资源')}
              </Button>
              <Button
                icon={<Download size={14} />}
                onClick={() =>
                  triggerDownload(
                    detailAsset.url,
                    buildDownloadName(detailAsset),
                  )
                }
              >
                {t('下载')}
              </Button>
            </Space>
            {[
              [t('资源 ID'), detailAsset.asset_id],
              [t('任务 ID'), detailAsset.task_id],
              [t('类型'), assetTypeLabel(detailAsset.asset_type, t)],
              [t('状态'), detailAsset.status],
              [t('模型'), detailAsset.model || '-'],
              [t('平台'), detailAsset.platform || '-'],
              [t('渠道 ID'), detailAsset.channel_id || '-'],
              [t('生成时间'), timestamp2string(detailAsset.created_at)],
              [t('URL'), detailAsset.url],
            ].map(([label, value]) => (
              <div
                key={label}
                className='flex gap-3 border-0 border-b border-solid border-semi-color-border pb-2'
              >
                <Text type='tertiary' className='w-24 shrink-0'>
                  {label}
                </Text>
                <Text
                  copyable={label === t('URL') || label === t('任务 ID')}
                  ellipsis={{ showTooltip: true }}
                >
                  {value}
                </Text>
              </div>
            ))}
            {detailAsset.metadata &&
              Object.keys(detailAsset.metadata).length > 0 && (
                <pre className='text-xs p-3 rounded-md bg-semi-color-fill-0 overflow-auto'>
                  {JSON.stringify(detailAsset.metadata, null, 2)}
                </pre>
              )}
          </div>
        )}
      </SideSheet>

      <ImagePreview
        src={previewImage}
        visible={!!previewImage}
        onVisibleChange={(visible) => {
          if (!visible) setPreviewImage('');
        }}
      />
    </div>
  );
}
