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
  Card,
  DatePicker,
  Divider,
  Input,
  InputNumber,
  Modal,
  Select,
  SideSheet,
  Space,
  Switch,
  Table,
  TabPane,
  Tabs,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { IconHelpCircle } from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import {
  API,
  renderPaymentAmount,
  renderQuota,
  renderQuotaWithAmount,
  showError,
  showSuccess,
} from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Text, Title } = Typography;

const bpsToPercent = (bps) => Number((Number(bps || 0) / 100).toFixed(4));
const percentToBps = (value) => Math.round(Number(value || 0) * 100);
const nowDate = () => new Date();
const sevenDaysAgo = () => new Date(Date.now() - 7 * 24 * 60 * 60 * 1000);
const toTimestamp = (value) => Math.floor(new Date(value).getTime() / 1000);

const defaultServiceCategories = [
  { service: 'gpt', label: 'GPT', remark: '' },
  { service: 'claude', label: 'Claude', remark: '' },
  { service: 'gemini', label: 'Gemini', remark: '' },
  { service: 'other', label: 'Other', remark: '' },
];

const defaultSettings = {
  default_level1_rate_bps: 500,
  default_level2_rate_bps: 150,
  service_categories: defaultServiceCategories,
  subscription_tiers: [
    { start_percent: 0, end_percent: 33, rate_bps: 1500 },
    { start_percent: 33, end_percent: 66, rate_bps: 750 },
    { start_percent: 66, end_percent: 100, rate_bps: 0 },
  ],
  group_profit_rules: [],
};

const normalizeCommissionService = (value) =>
  String(value || '')
    .trim()
    .toLowerCase();

const buildCommissionServiceOptions = (settings, currentService) => {
  const source =
    settings?.service_categories?.length > 0
      ? settings.service_categories
      : defaultServiceCategories;
  const options = source
    .map((category) => ({
      value: normalizeCommissionService(category.service),
      label: String(category.label || category.service || '').trim(),
    }))
    .filter((option) => option.value);
  const normalized = normalizeCommissionService(currentService);
  if (!normalized || options.some((option) => option.value === normalized)) {
    return options;
  }
  return [...options, { label: normalized.toUpperCase(), value: normalized }];
};

const MetricCard = ({ label, value, color = 'var(--semi-color-text-0)' }) => (
  <Card className='!rounded-lg' bodyStyle={{ padding: 14 }}>
    <Text type='tertiary' size='small'>
      {label}
    </Text>
    <div
      className='mt-2 text-xl font-semibold truncate'
      style={{ color }}
      title={String(value)}
    >
      {value}
    </div>
  </Card>
);

const FormulaCode = ({ children }) => (
  <div className='rounded-lg border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] px-3 py-2 font-mono text-xs leading-6 whitespace-pre-wrap'>
    {children}
  </div>
);

const FlowNode = ({ title, value, tone = 'default' }) => {
  const toneClass =
    tone === 'success'
      ? 'border-[var(--semi-color-success)] bg-[var(--semi-color-success-light-default)]'
      : tone === 'warning'
        ? 'border-[var(--semi-color-warning)] bg-[var(--semi-color-warning-light-default)]'
        : 'border-[var(--semi-color-border)] bg-[var(--semi-color-bg-1)]';
  return (
    <div className={`rounded-lg border px-3 py-2 min-w-[140px] ${toneClass}`}>
      <div className='text-xs text-[var(--semi-color-text-2)]'>{title}</div>
      <div className='mt-1 text-sm font-semibold text-[var(--semi-color-text-0)]'>
        {value}
      </div>
    </div>
  );
};

const FlowArrow = () => (
  <div className='hidden md:flex items-center text-[var(--semi-color-text-2)] px-1'>
    →
  </div>
);

const renderGroupTypeTag = (value, t) => {
  if (value === 'aggregate') {
    return <Tag color='violet'>{t('聚合分组')}</Tag>;
  }
  if (value === 'ratio') {
    return <Tag color='teal'>{t('倍率分组')}</Tag>;
  }
  return <Tag color='blue'>{t('普通分组')}</Tag>;
};

const getQuotaPerUnitText = () => {
  const value = Number(localStorage.getItem('quota_per_unit') || 0);
  return Number.isFinite(value) && value > 0 ? value : 500000;
};

const formatRawQuota = (value, digits = 4) => {
  const numberValue = Number(value || 0);
  if (!Number.isFinite(numberValue)) {
    return '0';
  }
  return numberValue.toFixed(digits).replace(/\.?0+$/, '');
};

const InviteCommissionFormulaSheet = ({ visible, onClose, report, t }) => {
  const isMobile = useIsMobile();
  if (!report) {
    return null;
  }

  const summary = report.summary || {};
  const effective = report.effective || {};
  const settings = report.settings || defaultSettings;
  const quotaPerUnit = getQuotaPerUnitText();
  const commissionEnabled = !!effective.enabled;
  const level1Rate = commissionEnabled ? bpsToPercent(effective.level1_rate_bps) : 0;
  const level2Rate = commissionEnabled ? bpsToPercent(effective.level2_rate_bps) : 0;
  const walletCommission = Number(summary.wallet_commission_quota || 0);
  const subscriptionCommission = Number(summary.subscription_commission_quota || 0);
  const estimatedCommission = Number(summary.estimated_commission_quota || 0);

  const serviceFormulaColumns = [
    { title: t('服务'), dataIndex: 'label' },
    {
      title: t('消费额度'),
      dataIndex: 'total_consumption_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('毛利额度'),
      dataIndex: 'gross_profit_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('理论返佣'),
      dataIndex: 'theoretical_commission_quota',
      render: (value) => renderQuota(value || 0, 4),
    },
    {
      title: t('保护上限'),
      dataIndex: 'profit_cap_commission_quota',
      render: (value) => renderQuota(value || 0, 4),
    },
    {
      title: t('预估返佣'),
      dataIndex: 'estimated_commission_quota',
      render: (value) => renderQuota(value || 0, 4),
    },
  ];

  const levelFormulaColumns = [
    { title: t('层级'), dataIndex: 'level', render: (value) => `${value}${t('级')}` },
    { title: t('人数'), dataIndex: 'invitee_count' },
    {
      title: t('层级比例'),
      dataIndex: 'rate_bps',
      render: (value) => `${bpsToPercent(value)}%`,
    },
    {
      title: t('消费额度'),
      dataIndex: 'total_consumption_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('预估返佣'),
      dataIndex: 'estimated_commission_quota',
      render: (value) => renderQuota(value || 0, 4),
    },
  ];

  return (
    <SideSheet
      placement='right'
      visible={visible}
      onCancel={onClose}
      title={t('返佣计算说明')}
      width={isMobile ? '100%' : 760}
      bodyStyle={{ padding: 0 }}
    >
      <div className='p-4 space-y-4'>
        <Card className='!rounded-lg' bodyStyle={{ padding: 14 }}>
          <div className='grid grid-cols-1 md:grid-cols-[1fr_auto_1fr_auto_1fr] gap-2 items-stretch'>
            <FlowNode
              title={t('邀请关系')}
              value={`${t('一级')} ${summary.level1_invitee_count || 0} / ${t('二级')} ${summary.level2_invitee_count || 0}`}
            />
            <FlowArrow />
            <FlowNode
              title={t('消费来源')}
              value={`${t('钱包')} ${renderQuota(summary.wallet_consumption_quota || 0)} + ${t('订阅')} ${renderQuota(summary.subscription_consumption_quota || 0)}`}
            />
            <FlowArrow />
            <FlowNode
              title={t('毛利 / 预估返佣')}
              value={`${renderQuota(summary.gross_profit_quota || 0)} / ${renderQuota(estimatedCommission, 4)}`}
              tone='success'
            />
          </div>
        </Card>

        <Card className='!rounded-lg' title={t('核心公式')}>
          <Space vertical align='start' style={{ width: '100%' }}>
            <FormulaCode>
              {`${t('理论返佣')} = ${t('消费额度')} × ${t('分组最高返佣比例或订阅档位比例')} × ${t('层级返佣比例')}\n${t('毛利额度')} = ${t('消费额度')} × ${t('分组毛利率')}\n${t('利润保护上限')} = ${t('毛利额度')} × ${t('利润分成上限')} × ${t('层级返佣比例')}\n${t('最终返佣')} = min(${t('理论返佣')}, ${t('利润保护上限')})`}
            </FormulaCode>
            <FormulaCode>
              {`${t('展示金额')} = ${t('额度')} / ${quotaPerUnit}\n${t('当前预估返佣')} = ${formatRawQuota(estimatedCommission)} / ${quotaPerUnit} = ${renderQuota(estimatedCommission, 4)}`}
            </FormulaCode>
            <Text type='tertiary'>
              {t(
                '充值、兑换码和订阅购买金额只用于报表展示；新消费日志按写入时的分组利润快照计算，缺失快照的老日志只展示消费不参与预估返佣。',
              )}
            </Text>
          </Space>
        </Card>

        <Card className='!rounded-lg' title={t('本次报表代入值')}>
          <div className='grid grid-cols-1 md:grid-cols-3 gap-3'>
            <FlowNode
              title={t('钱包消费返佣')}
              value={`${formatRawQuota(walletCommission)} ${t('额度')} / ${renderQuota(walletCommission, 4)}`}
            />
            <FlowNode
              title={t('订阅消费返佣')}
              value={`${formatRawQuota(subscriptionCommission)} ${t('额度')} / ${renderQuota(subscriptionCommission, 4)}`}
            />
            <FlowNode
              title={t('合计预估返佣')}
              value={`${formatRawQuota(estimatedCommission)} ${t('额度')} / ${renderQuota(estimatedCommission, 4)}`}
              tone='success'
            />
          </div>
        </Card>

        <Card className='!rounded-lg' title={t('当前使用规则')}>
          <div className='grid grid-cols-1 md:grid-cols-2 gap-3'>
            <div className='rounded-lg border border-[var(--semi-color-border)] p-3'>
              <Text strong>{t('层级比例')}</Text>
              <div className='mt-2 flex flex-wrap gap-2'>
                <Tag color='blue'>{`${t('一级')} ${level1Rate}%`}</Tag>
                <Tag color='violet'>{`${t('二级')} ${level2Rate}%`}</Tag>
                <Tag color={effective.enabled ? 'green' : 'grey'}>
                  {effective.enabled ? t('返佣启用') : t('返佣关闭')}
                </Tag>
                {!effective.use_user_config && (
                  <Tag color='amber'>{t('未添加代理配置')}</Tag>
                )}
              </div>
              {!effective.enabled && (
                <Text type='tertiary' size='small'>
                  {t('未启用代理返佣时，邀请、充值和消费仍展示，但预估返佣按 0 计算。')}
                </Text>
              )}
            </div>
            <div className='rounded-lg border border-[var(--semi-color-border)] p-3'>
              <Text strong>{t('订阅用量档位')}</Text>
              <div className='mt-2 flex flex-wrap gap-2'>
                {(settings.subscription_tiers || []).map((tier, index) => (
                  <Tag key={index} color='teal'>
                    {`${tier.start_percent}-${tier.end_percent}% = ${bpsToPercent(tier.rate_bps)}%`}
                  </Tag>
                ))}
              </div>
            </div>
          </div>
        </Card>

        <Table
          title={() => t('层级计算拆分')}
          columns={levelFormulaColumns}
          dataSource={report.levels || []}
          pagination={false}
          rowKey='level'
          size='small'
        />

        <Table
          title={() => t('服务计算拆分')}
          columns={serviceFormulaColumns}
          dataSource={report.services || []}
          pagination={false}
          rowKey='service'
          size='small'
        />
      </div>
    </SideSheet>
  );
};

const parseDateRange = (range) => {
  if (!Array.isArray(range) || range.length !== 2 || !range[0] || !range[1]) {
    return [toTimestamp(sevenDaysAgo()), toTimestamp(nowDate())];
  }
  return [toTimestamp(range[0]), toTimestamp(range[1])];
};

const normalizeSettingsForSave = (settings) => ({
  default_level1_rate_bps: Number(settings.default_level1_rate_bps || 0),
  default_level2_rate_bps: Number(settings.default_level2_rate_bps || 0),
  service_categories: (settings.service_categories || []).map((item) => ({
    service: normalizeCommissionService(item.service),
    label: String(item.label || '').trim(),
    remark: String(item.remark || '').trim(),
  })),
  subscription_tiers: (settings.subscription_tiers || []).map((item) => ({
    start_percent: Number(item.start_percent || 0),
    end_percent: Number(item.end_percent || 0),
    rate_bps: Number(item.rate_bps || 0),
  })),
  group_profit_rules: settings.group_profit_rules || [],
});

const InviteCommission = () => {
  const { t } = useTranslation();
  const [dateRange, setDateRange] = useState([sevenDaysAgo(), nowDate()]);
  const [userKeyword, setUserKeyword] = useState('');
  const [userOptions, setUserOptions] = useState([]);
  const [ownerUserId, setOwnerUserId] = useState();
  const [report, setReport] = useState(null);
  const [reportLoading, setReportLoading] = useState(false);
  const [formulaSheetVisible, setFormulaSheetVisible] = useState(false);
  const [settings, setSettings] = useState(defaultSettings);
  const [settingsLoading, setSettingsLoading] = useState(false);
  const [configKeyword, setConfigKeyword] = useState('');
  const [configs, setConfigs] = useState([]);
  const [configLoading, setConfigLoading] = useState(false);
  const [configModalVisible, setConfigModalVisible] = useState(false);
  const [configEditing, setConfigEditing] = useState(false);
  const [configUserKeyword, setConfigUserKeyword] = useState('');
  const [configUserOptions, setConfigUserOptions] = useState([]);
  const [configUserId, setConfigUserId] = useState();
  const [configForm, setConfigForm] = useState({
    enabled: true,
    level1_rate_bps: 500,
    level2_rate_bps: 150,
    remark: '',
  });
  const [groupRuleKeyword, setGroupRuleKeyword] = useState('');
  const [groupRules, setGroupRules] = useState([]);
  const [groupRulesLoading, setGroupRulesLoading] = useState(false);
  const [groupRuleModalVisible, setGroupRuleModalVisible] = useState(false);
  const [groupRuleForm, setGroupRuleForm] = useState({
    group: '',
    service: 'gpt',
    profit_rate_bps: 3000,
    max_commission_rate_bps: 1500,
    profit_share_rate_bps: 6000,
    profit_protection_enabled: true,
    remark: '',
  });

  const searchUsers = async (keyword = userKeyword) => {
    const normalized = String(keyword || '').trim();
    if (!normalized) {
      showError(t('请输入用户 ID 或用户名'));
      return;
    }
    const res = await API.get(
      `/api/user/search?keyword=${encodeURIComponent(normalized)}&p=1&page_size=20`,
    );
    if (!res.data?.success) {
      showError(res.data?.message || t('搜索失败'));
      return;
    }
    const items = res.data?.data?.items || [];
    const options = items.map((user) => ({
      label: `#${user.id} ${user.username || ''}`,
      value: user.id,
    }));
    setUserOptions(options);
    if (options.length > 0 && !ownerUserId) {
      setOwnerUserId(options[0].value);
    }
  };

  const loadReport = async () => {
    if (!ownerUserId) {
      showError(t('请选择用户'));
      return;
    }
    const [start, end] = parseDateRange(dateRange);
    setReportLoading(true);
    try {
      const res = await API.get(
        `/api/invite_commission/admin/report?owner_user_id=${ownerUserId}&start_timestamp=${start}&end_timestamp=${end}`,
      );
      if (res.data?.success) {
        setReport(res.data.data);
      } else {
        showError(res.data?.message || t('查询失败'));
      }
    } finally {
      setReportLoading(false);
    }
  };

  const loadSettings = async () => {
    setSettingsLoading(true);
    try {
      const res = await API.get('/api/invite_commission/settings');
      if (res.data?.success) {
        setSettings(res.data.data || defaultSettings);
      }
    } finally {
      setSettingsLoading(false);
    }
  };

  const saveSettings = async () => {
    setSettingsLoading(true);
    try {
      const res = await API.put(
        '/api/invite_commission/settings',
        normalizeSettingsForSave(settings),
      );
      if (res.data?.success) {
        setSettings(res.data.data);
        showSuccess(t('保存成功'));
      } else {
        showError(res.data?.message || t('保存失败'));
      }
    } finally {
      setSettingsLoading(false);
    }
  };

  const loadConfigs = async () => {
    setConfigLoading(true);
    try {
      const res = await API.get(
        `/api/invite_commission/user_configs?keyword=${encodeURIComponent(configKeyword)}&p=1&page_size=50`,
      );
      if (res.data?.success) {
        setConfigs(res.data?.data?.items || []);
      }
    } finally {
      setConfigLoading(false);
    }
  };

  const searchConfigUsers = async (keyword = configUserKeyword) => {
    const normalized = String(keyword || '').trim();
    if (!normalized) {
      showError(t('请输入用户 ID 或用户名'));
      return;
    }
    const res = await API.get(
      `/api/user/search?keyword=${encodeURIComponent(normalized)}&p=1&page_size=20`,
    );
    if (!res.data?.success) {
      showError(res.data?.message || t('搜索失败'));
      return;
    }
    const options = (res.data?.data?.items || []).map((user) => ({
      label: `#${user.id} ${user.username || ''}`,
      value: user.id,
    }));
    setConfigUserOptions(options);
    if (options.length > 0) {
      setConfigUserId(options[0].value);
    }
  };

  const openCreateUserConfig = () => {
    setConfigEditing(false);
    setConfigUserKeyword('');
    setConfigUserOptions([]);
    setConfigUserId(undefined);
    setConfigForm({
      enabled: true,
      level1_rate_bps: Number(settings.default_level1_rate_bps || 0),
      level2_rate_bps: Number(settings.default_level2_rate_bps || 0),
      remark: '',
    });
    setConfigModalVisible(true);
  };

  const saveUserConfig = async () => {
    if (!configUserId) {
      showError(t('请选择用户'));
      return;
    }
    const res = await API.put(
      `/api/invite_commission/user_configs/${configUserId}`,
      configForm,
    );
    if (res.data?.success) {
      showSuccess(t('保存成功'));
      setConfigModalVisible(false);
      await loadConfigs();
    } else {
      showError(res.data?.message || t('保存失败'));
    }
  };

  const loadGroupProfitRules = async (keyword = groupRuleKeyword) => {
    setGroupRulesLoading(true);
    try {
      const res = await API.get(
        `/api/invite_commission/group_profit_rules?keyword=${encodeURIComponent(keyword || '')}`,
      );
      if (res.data?.success) {
        setGroupRules(res.data.data || []);
      } else {
        showError(res.data?.message || t('加载分组利润规则失败'));
      }
    } finally {
      setGroupRulesLoading(false);
    }
  };

  const openGroupRuleModal = (record) => {
    const fallbackService =
      normalizeCommissionService(settings.service_categories?.[0]?.service) || 'other';
    setGroupRuleForm({
      group: record.group,
      service: record.service || fallbackService,
      profit_rate_bps: Number(record.profit_rate_bps || 0),
      max_commission_rate_bps: Number(record.max_commission_rate_bps || 0),
      profit_share_rate_bps: Number(record.profit_share_rate_bps || 6000),
      profit_protection_enabled: record.configured
        ? !!record.profit_protection_enabled
        : true,
      remark: record.remark || '',
    });
    setGroupRuleModalVisible(true);
  };

  const saveGroupProfitRule = async () => {
    const res = await API.put('/api/invite_commission/group_profit_rules', groupRuleForm);
    if (res.data?.success) {
      showSuccess(t('保存成功'));
      setGroupRuleModalVisible(false);
      await Promise.all([loadGroupProfitRules(), loadSettings()]);
    } else {
      showError(res.data?.message || t('保存失败'));
    }
  };

  const clearGroupProfitRule = (record) => {
    Modal.confirm({
      title: t('清除分组利润规则'),
      content: t('清除后该分组的新消费日志仍会写快照，但未配置利润规则时预估返佣为 0。'),
      okText: t('清除'),
      cancelText: t('取消'),
      onOk: async () => {
        const res = await API.delete(
          `/api/invite_commission/group_profit_rules?group=${encodeURIComponent(record.group)}`,
        );
        if (res.data?.success) {
          showSuccess(t('清除成功'));
          await Promise.all([loadGroupProfitRules(), loadSettings()]);
        } else {
          showError(res.data?.message || t('清除失败'));
        }
      },
    });
  };

  useEffect(() => {
    loadSettings();
    loadConfigs();
    loadGroupProfitRules('');
  }, []);

  const summary = report?.summary || {};
  const reportMetrics = useMemo(
    () => [
      [t('邀请人数'), summary.invitee_count || 0],
      [t('钱包充值额度'), renderQuotaWithAmount(summary.wallet_recharge_amount || 0)],
      [t('真实充值金额'), renderPaymentAmount(summary.wallet_recharge_money || 0)],
      [t('兑换码兑换额度'), renderQuota(summary.redemption_quota || 0)],
      [t('订阅购买金额'), renderPaymentAmount(summary.subscription_purchase_money || 0)],
      [t('总消费额度'), renderQuota(summary.total_consumption_quota || 0)],
      [t('订阅消费额度'), renderQuota(summary.subscription_consumption_quota || 0)],
      [t('上游成本额度'), renderQuota(summary.upstream_cost_quota || 0)],
      [t('毛利额度'), renderQuota(summary.gross_profit_quota || 0)],
      [
        t('理论返佣'),
        renderQuota(summary.theoretical_commission_quota || 0, 4),
      ],
      [
        t('保护上限'),
        renderQuota(summary.profit_cap_commission_quota || 0, 4),
      ],
      [
        t('压低返佣'),
        renderQuota(summary.profit_protection_reduced_quota || 0, 4),
      ],
      [
        t('缺失快照消费'),
        renderQuota(summary.missing_profit_snapshot_quota || 0),
        'var(--semi-color-warning)',
      ],
      [
        t('预估返佣'),
        renderQuota(summary.estimated_commission_quota || 0, 4),
        'var(--semi-color-success)',
      ],
    ],
    [summary, t],
  );

  const levelColumns = [
    { title: t('层级'), dataIndex: 'level', render: (value) => `${value}${t('级')}` },
    { title: t('人数'), dataIndex: 'invitee_count' },
    {
      title: t('层级比例'),
      dataIndex: 'rate_bps',
      render: (value) => `${bpsToPercent(value)}%`,
    },
    {
      title: t('钱包充值'),
      dataIndex: 'wallet_recharge_amount',
      render: (value) => renderQuotaWithAmount(value || 0),
    },
    {
      title: t('兑换码'),
      dataIndex: 'redemption_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('消费额度'),
      dataIndex: 'total_consumption_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('毛利额度'),
      dataIndex: 'gross_profit_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('保护压低'),
      dataIndex: 'profit_protection_reduced_quota',
      render: (value) => renderQuota(value || 0, 4),
    },
    {
      title: t('预估返佣'),
      dataIndex: 'estimated_commission_quota',
      render: (value) => renderQuota(value || 0, 4),
    },
  ];

  const serviceColumns = [
    { title: t('服务'), dataIndex: 'label' },
    { title: t('请求数'), dataIndex: 'request_count' },
    {
      title: t('钱包消费'),
      dataIndex: 'wallet_consumption_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('订阅消费'),
      dataIndex: 'subscription_consumption_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('预估返佣'),
      dataIndex: 'estimated_commission_quota',
      render: (value) => renderQuota(value || 0, 4),
    },
    {
      title: t('缺失快照'),
      dataIndex: 'missing_profit_snapshot_quota',
      render: (value) => renderQuota(value || 0),
    },
  ];

  const inviteeColumns = [
    { title: t('用户ID'), dataIndex: 'user_id' },
    { title: t('用户名'), dataIndex: 'username' },
    { title: t('层级'), dataIndex: 'level', render: (value) => `${value}${t('级')}` },
    {
      title: t('钱包充值'),
      dataIndex: 'wallet_recharge_amount',
      render: (value) => renderQuotaWithAmount(value || 0),
    },
    {
      title: t('兑换码'),
      dataIndex: 'redemption_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('订阅购买'),
      dataIndex: 'subscription_purchase_money',
      render: (value) => renderPaymentAmount(value || 0),
    },
    {
      title: t('消费额度'),
      dataIndex: 'total_consumption_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('毛利额度'),
      dataIndex: 'gross_profit_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('预估返佣'),
      dataIndex: 'estimated_commission_quota',
      render: (value) => renderQuota(value || 0, 4),
    },
  ];

  const modelColumns = [
    { title: t('模型'), dataIndex: 'model_name' },
    { title: t('服务'), dataIndex: 'service_label' },
    { title: t('请求数'), dataIndex: 'request_count' },
    {
      title: t('消费额度'),
      dataIndex: 'total_consumption_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('预估返佣'),
      dataIndex: 'estimated_commission_quota',
      render: (value) => renderQuota(value || 0, 4),
    },
  ];

  const groupColumns = [
    { title: t('分组'), dataIndex: 'group' },
    {
      title: t('类型'),
      dataIndex: 'type',
      render: (value) => renderGroupTypeTag(value, t),
    },
    { title: t('服务'), dataIndex: 'service_label' },
    { title: t('请求数'), dataIndex: 'request_count' },
    {
      title: t('消费额度'),
      dataIndex: 'total_consumption_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('成本额度'),
      dataIndex: 'upstream_cost_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('毛利额度'),
      dataIndex: 'gross_profit_quota',
      render: (value) => renderQuota(value || 0),
    },
    {
      title: t('理论返佣'),
      dataIndex: 'theoretical_commission_quota',
      render: (value) => renderQuota(value || 0, 4),
    },
    {
      title: t('保护上限'),
      dataIndex: 'profit_cap_commission_quota',
      render: (value) => renderQuota(value || 0, 4),
    },
    {
      title: t('预估返佣'),
      dataIndex: 'estimated_commission_quota',
      render: (value) => renderQuota(value || 0, 4),
    },
    {
      title: t('缺失快照'),
      dataIndex: 'missing_profit_snapshot_quota',
      render: (value) => renderQuota(value || 0),
    },
  ];

  const editUserConfig = (record) => {
    const option = {
      label: `#${record.user_id} ${record.username || ''}`,
      value: record.user_id,
    };
    setConfigEditing(true);
    setConfigUserOptions([option]);
    setConfigUserKeyword(record.username || String(record.user_id));
    setConfigUserId(record.user_id);
    setConfigForm({
      enabled: !!record.enabled,
      level1_rate_bps: Number(record.level1_rate_bps || 0),
      level2_rate_bps: Number(record.level2_rate_bps || 0),
      remark: record.remark || '',
    });
    setConfigModalVisible(true);
  };

  const configColumns = [
    { title: t('用户ID'), dataIndex: 'user_id' },
    { title: t('用户名'), dataIndex: 'username' },
    {
      title: t('状态'),
      dataIndex: 'enabled',
      render: (value) => (
        <Tag color={value ? 'green' : 'grey'}>
          {value ? t('启用返佣') : t('停止返佣')}
        </Tag>
      ),
    },
    {
      title: t('一级比例'),
      dataIndex: 'level1_rate_bps',
      render: (value) => `${bpsToPercent(value)}%`,
    },
    {
      title: t('二级比例'),
      dataIndex: 'level2_rate_bps',
      render: (value) => `${bpsToPercent(value)}%`,
    },
    { title: t('备注'), dataIndex: 'remark' },
    {
      title: t('操作'),
      dataIndex: 'action',
      width: 100,
      render: (_, record) => (
        <Button size='small' type='tertiary' onClick={() => editUserConfig(record)}>
          {t('编辑')}
        </Button>
      ),
    },
  ];

  const groupRuleColumns = [
    { title: t('分组'), dataIndex: 'group' },
    {
      title: t('类型'),
      dataIndex: 'type',
      width: 110,
      render: (value) => renderGroupTypeTag(value, t),
    },
    {
      title: t('配置状态'),
      dataIndex: 'configured',
      width: 110,
      render: (value) => (
        <Tag color={value ? 'green' : 'grey'}>
          {value ? t('已配置') : t('未配置')}
        </Tag>
      ),
    },
    {
      title: t('服务'),
      dataIndex: 'service',
      width: 100,
      render: (value) => String(value || 'other').toUpperCase(),
    },
    {
      title: t('毛利率'),
      dataIndex: 'profit_rate_bps',
      width: 110,
      render: (value) => `${bpsToPercent(value)}%`,
    },
    {
      title: t('最高返佣'),
      dataIndex: 'max_commission_rate_bps',
      width: 110,
      render: (value) => `${bpsToPercent(value)}%`,
    },
    {
      title: t('利润分成上限'),
      dataIndex: 'profit_share_rate_bps',
      width: 130,
      render: (value) => `${bpsToPercent(value)}%`,
    },
    {
      title: t('利润保护'),
      dataIndex: 'profit_protection_enabled',
      width: 110,
      render: (value) => (
        <Tag color={value ? 'green' : 'amber'}>
          {value ? t('开启') : t('关闭')}
        </Tag>
      ),
    },
    { title: t('备注'), dataIndex: 'remark' },
    {
      title: t('操作'),
      dataIndex: 'action',
      width: 150,
      fixed: 'right',
      render: (_, record) => (
        <Space>
          <Button size='small' type='tertiary' onClick={() => openGroupRuleModal(record)}>
            {record.configured ? t('编辑') : t('配置')}
          </Button>
          <Button
            size='small'
            type='danger'
            theme='borderless'
            disabled={!record.configured}
            onClick={() => clearGroupProfitRule(record)}
          >
            {t('清除')}
          </Button>
        </Space>
      ),
    },
  ];

  const updateTier = (index, patch) => {
    const list = [...(settings.subscription_tiers || [])];
    list[index] = { ...list[index], ...patch };
    setSettings({ ...settings, subscription_tiers: list });
  };

  const updateServiceCategory = (index, patch) => {
    const list = [...(settings.service_categories || [])];
    list[index] = { ...list[index], ...patch };
    setSettings({ ...settings, service_categories: list });
  };

  const addServiceCategory = () => {
    setSettings({
      ...settings,
      service_categories: [
        ...(settings.service_categories || []),
        { service: '', label: '', remark: '' },
      ],
    });
  };

  const removeServiceCategory = (index) => {
    setSettings({
      ...settings,
      service_categories: (settings.service_categories || []).filter(
        (_, itemIndex) => itemIndex !== index,
      ),
    });
  };

  return (
    <div className='mt-[60px] px-2'>
      <Card className='!rounded-xl'>
        <div className='mb-4'>
          <Title heading={4}>{t('邀请返佣管理')}</Title>
          <Text type='tertiary'>
            {t('按当前邀请关系查询充值、兑换、消费和预估返佣，不自动结算。')}
          </Text>
        </div>

        <Tabs type='card' keepDOM>
          <TabPane tab={t('返佣报表')} itemKey='report'>
            <Space vertical align='start' style={{ width: '100%' }}>
              <div className='flex flex-col md:flex-row gap-2 w-full'>
                <Input
                  value={userKeyword}
                  onChange={setUserKeyword}
                  placeholder={t('用户 ID / 用户名')}
                  style={{ maxWidth: 260 }}
                />
                <Button onClick={() => searchUsers()}>{t('搜索用户')}</Button>
                <Select
                  value={ownerUserId}
                  onChange={setOwnerUserId}
                  optionList={userOptions}
                  placeholder={t('选择邀请人')}
                  style={{ width: 260 }}
                  filter
                />
                <DatePicker
                  type='dateTimeRange'
                  value={dateRange}
                  onChange={setDateRange}
                  style={{ width: 360, maxWidth: '100%' }}
                />
                <Button type='primary' loading={reportLoading} onClick={loadReport}>
                  {t('查询')}
                </Button>
                <Button
                  icon={<IconHelpCircle />}
                  disabled={!report}
                  onClick={() => setFormulaSheetVisible(true)}
                >
                  {t('计算说明')}
                </Button>
              </div>

              {report && (
                <>
                  {!report.effective?.enabled && (
                    <div className='rounded-lg border border-[var(--semi-color-warning-light-default)] bg-[var(--semi-color-warning-light-default)] px-3 py-2 w-full'>
                      <Text type='warning'>
                        {t(
                          '该用户未启用代理返佣，当前只展示邀请、充值和消费统计，预估返佣按 0 计算。请先在“用户配置”中新增并启用代理配置。',
                        )}
                      </Text>
                    </div>
                  )}
                  <div className='grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 w-full'>
                    {reportMetrics.map(([label, value, color]) => (
                      <MetricCard key={label} label={label} value={value} color={color} />
                    ))}
                  </div>
                  <Divider margin={12} />
                  <Table
                    title={() => t('层级汇总')}
                    columns={levelColumns}
                    dataSource={report.levels || []}
                    pagination={false}
                    rowKey='level'
                    size='small'
                  />
                  <Table
                    title={() => t('服务汇总')}
                    columns={serviceColumns}
                    dataSource={report.services || []}
                    pagination={false}
                    rowKey='service'
                    size='small'
                  />
                  <Table
                    title={() => t('分组利润汇总')}
                    columns={groupColumns}
                    dataSource={report.groups || []}
                    rowKey='group'
                    size='small'
                    pagination={{ pageSize: 10 }}
                    scroll={{ x: 1200 }}
                  />
                  <Table
                    title={() => t('被邀请用户明细')}
                    columns={inviteeColumns}
                    dataSource={report.invitees || []}
                    rowKey='user_id'
                    size='small'
                    pagination={{ pageSize: 10 }}
                  />
                  <Table
                    title={() => t('模型明细')}
                    columns={modelColumns}
                    dataSource={report.models || []}
                    rowKey={(record) => `${record.service}-${record.model_name}`}
                    size='small'
                    pagination={{ pageSize: 10 }}
                  />
                  <InviteCommissionFormulaSheet
                    visible={formulaSheetVisible}
                    onClose={() => setFormulaSheetVisible(false)}
                    report={report}
                    t={t}
                  />
                </>
              )}
            </Space>
          </TabPane>

          <TabPane tab={t('用户配置')} itemKey='users'>
            <Space vertical align='start' style={{ width: '100%' }}>
              <div className='rounded-lg border border-[var(--semi-color-border)] bg-[var(--semi-color-fill-0)] px-3 py-2 w-full'>
                <Text type='secondary'>
                  {t(
                    '只有被添加到这里且启用返佣的用户，才会计算预估返佣；未配置或关闭时仍展示邀请、充值和消费统计，但预估返佣为 0。',
                  )}
                </Text>
              </div>
              <div className='flex flex-col md:flex-row gap-2 w-full'>
                <Button type='primary' onClick={openCreateUserConfig}>
                  {t('新增代理配置')}
                </Button>
                <Input
                  value={configKeyword}
                  onChange={setConfigKeyword}
                  placeholder={t('搜索已配置用户')}
                  style={{ maxWidth: 260 }}
                />
                <Button loading={configLoading} onClick={loadConfigs}>
                  {t('刷新')}
                </Button>
              </div>
              <Modal
                title={configEditing ? t('编辑代理配置') : t('新增代理配置')}
                visible={configModalVisible}
                onCancel={() => setConfigModalVisible(false)}
                onOk={saveUserConfig}
                okText={t('保存')}
                cancelText={t('取消')}
                style={{ maxWidth: 'calc(100vw - 32px)' }}
              >
                <Space vertical align='start' style={{ width: '100%' }}>
                  <div className='grid grid-cols-1 md:grid-cols-[1fr_auto] gap-2 w-full'>
                    <Input
                      value={configUserKeyword}
                      onChange={setConfigUserKeyword}
                      placeholder={t('用户 ID / 用户名')}
                      disabled={configEditing}
                    />
                    <Button
                      onClick={() => searchConfigUsers()}
                      disabled={configEditing}
                    >
                      {t('搜索用户')}
                    </Button>
                  </div>
                  <Select
                    value={configUserId}
                    onChange={setConfigUserId}
                    optionList={configUserOptions}
                    placeholder={t('选择代理用户')}
                    disabled={configEditing}
                    filter
                    style={{ width: '100%' }}
                  />
                  <div className='rounded-lg border border-[var(--semi-color-border)] p-3 w-full'>
                    <div className='flex items-center gap-2'>
                      <Switch
                        checked={configForm.enabled}
                        onChange={(checked) =>
                          setConfigForm({ ...configForm, enabled: checked })
                        }
                      />
                      <Text strong>
                        {configForm.enabled ? t('启用返佣') : t('停止返佣')}
                      </Text>
                    </div>
                    <Text type='tertiary' size='small'>
                      {t(
                        '关闭后该用户的报表仍展示邀请人数、充值和消费，但预估返佣按 0 计算。',
                      )}
                    </Text>
                  </div>
                  <div className='grid grid-cols-1 md:grid-cols-2 gap-3 w-full'>
                    <div>
                      <Text type='tertiary' size='small'>
                        {t('一级返佣比例')}
                      </Text>
                      <InputNumber
                        value={bpsToPercent(configForm.level1_rate_bps)}
                        suffix='%'
                        min={0}
                        onChange={(value) =>
                          setConfigForm({
                            ...configForm,
                            level1_rate_bps: percentToBps(value),
                          })
                        }
                        style={{ width: '100%' }}
                      />
                    </div>
                    <div>
                      <Text type='tertiary' size='small'>
                        {t('二级返佣比例')}
                      </Text>
                      <InputNumber
                        value={bpsToPercent(configForm.level2_rate_bps)}
                        suffix='%'
                        min={0}
                        onChange={(value) =>
                          setConfigForm({
                            ...configForm,
                            level2_rate_bps: percentToBps(value),
                          })
                        }
                        style={{ width: '100%' }}
                      />
                    </div>
                  </div>
                  <TextArea
                    value={configForm.remark}
                    onChange={(value) =>
                      setConfigForm({ ...configForm, remark: value })
                    }
                    placeholder={t('备注')}
                    autosize
                    style={{ width: '100%' }}
                  />
                </Space>
              </Modal>
              <Table
                columns={configColumns}
                dataSource={configs}
                rowKey='user_id'
                loading={configLoading}
                pagination={{ pageSize: 10 }}
                size='small'
              />
            </Space>
          </TabPane>

          <TabPane tab={t('规则配置')} itemKey='settings'>
            <Space vertical align='start' style={{ width: '100%' }}>
              <div className='grid grid-cols-1 md:grid-cols-2 gap-3 w-full'>
                <Card className='!rounded-lg' title={t('新增代理默认层级比例')}>
                  <Text type='tertiary' size='small'>
                    {t('用于新增代理配置时预填一级 / 二级比例；未添加或未启用的用户不会产生预估返佣。')}
                  </Text>
                  <Space>
                    <InputNumber
                      value={bpsToPercent(settings.default_level1_rate_bps)}
                      suffix='%'
                      min={0}
                      onChange={(value) =>
                        setSettings({
                          ...settings,
                          default_level1_rate_bps: percentToBps(value),
                        })
                      }
                    />
                    <InputNumber
                      value={bpsToPercent(settings.default_level2_rate_bps)}
                      suffix='%'
                      min={0}
                      onChange={(value) =>
                        setSettings({
                          ...settings,
                          default_level2_rate_bps: percentToBps(value),
                        })
                      }
                    />
                  </Space>
                </Card>
              </div>

              <Card className='!rounded-lg w-full' title={t('服务分类')}>
                <Space vertical align='start' style={{ width: '100%' }}>
                  <Text type='tertiary' size='small'>
                    {t(
                      '先维护 GPT、Claude、Gemini、DeepSeek 等服务分类；分组利润规则里的服务归类会从这里下拉选择。',
                    )}
                  </Text>
                  {(settings.service_categories || []).map((category, index) => (
                    <div
                      key={index}
                      className='grid grid-cols-1 md:grid-cols-[160px_160px_1fr_auto] gap-2 w-full'
                    >
                      <Input
                        value={category.service}
                        placeholder={t('服务标识，例如 deepseek')}
                        onChange={(value) =>
                          updateServiceCategory(index, {
                            service: normalizeCommissionService(value),
                          })
                        }
                      />
                      <Input
                        value={category.label}
                        placeholder={t('显示名称')}
                        onChange={(value) =>
                          updateServiceCategory(index, { label: value })
                        }
                      />
                      <Input
                        value={category.remark}
                        placeholder={t('备注')}
                        onChange={(value) =>
                          updateServiceCategory(index, { remark: value })
                        }
                      />
                      <Button
                        type='danger'
                        theme='borderless'
                        onClick={() => removeServiceCategory(index)}
                      >
                        {t('删除')}
                      </Button>
                    </div>
                  ))}
                  <Button onClick={addServiceCategory}>{t('新增服务分类')}</Button>
                </Space>
              </Card>

              <Card className='!rounded-lg w-full' title={t('订阅用量档位')}>
                <Space vertical align='start'>
                  {(settings.subscription_tiers || []).map((tier, index) => (
                    <Space key={index}>
                      <InputNumber
                        value={tier.start_percent}
                        suffix='%'
                        min={0}
                        onChange={(value) => updateTier(index, { start_percent: value })}
                      />
                      <Text>-</Text>
                      <InputNumber
                        value={tier.end_percent}
                        suffix='%'
                        min={0}
                        onChange={(value) => updateTier(index, { end_percent: value })}
                      />
                      <InputNumber
                        value={bpsToPercent(tier.rate_bps)}
                        suffix='%'
                        min={0}
                        onChange={(value) =>
                          updateTier(index, { rate_bps: percentToBps(value) })
                        }
                      />
                    </Space>
                  ))}
                </Space>
              </Card>

              <Card className='!rounded-lg w-full' title={t('分组利润规则')}>
                <Space vertical align='start' style={{ width: '100%' }}>
                  <Text type='tertiary' size='small'>
                    {t(
                      '只展示可配置分组；来源包含分组倍率、渠道分组和聚合分组，default 和 UserGroup-* 会自动过滤。未配置分组的新消费会写入快照，但预估返佣为 0。',
                    )}
                  </Text>
                  <div className='flex flex-col md:flex-row gap-2 w-full'>
                    <Input
                      value={groupRuleKeyword}
                      onChange={setGroupRuleKeyword}
                      placeholder={t('搜索分组')}
                      style={{ maxWidth: 260 }}
                    />
                    <Button
                      loading={groupRulesLoading}
                      onClick={() => loadGroupProfitRules()}
                    >
                      {t('搜索 / 刷新')}
                    </Button>
                  </div>
                  <Table
                    columns={groupRuleColumns}
                    dataSource={groupRules}
                    rowKey='group'
                    loading={groupRulesLoading}
                    pagination={{ pageSize: 10 }}
                    size='small'
                    scroll={{ x: 1200 }}
                  />
                </Space>
              </Card>

              <Modal
                title={`${t('配置分组利润规则')} - ${groupRuleForm.group}`}
                visible={groupRuleModalVisible}
                onCancel={() => setGroupRuleModalVisible(false)}
                onOk={saveGroupProfitRule}
                okText={t('保存')}
                cancelText={t('取消')}
                style={{ maxWidth: 'calc(100vw - 32px)' }}
              >
                <Space vertical align='start' style={{ width: '100%' }}>
                  <div className='grid grid-cols-1 md:grid-cols-2 gap-3 w-full'>
                    <div>
                      <Text type='tertiary' size='small'>
                        {t('服务归类')}
                      </Text>
                      <Select
                        value={groupRuleForm.service}
                        filter
                        placeholder={t('选择服务分类')}
                        onChange={(value) =>
                          setGroupRuleForm({
                            ...groupRuleForm,
                            service: normalizeCommissionService(value),
                          })
                        }
                        optionList={buildCommissionServiceOptions(
                          settings,
                          groupRuleForm.service,
                        )}
                        style={{ width: '100%' }}
                      />
                      <Text type='tertiary' size='small'>
                        {t('需要新服务时，先在上方“服务分类”中新增，然后这里选择。')}
                      </Text>
                    </div>
                    <div>
                      <Text type='tertiary' size='small'>
                        {t('毛利率')}
                      </Text>
                      <InputNumber
                        value={bpsToPercent(groupRuleForm.profit_rate_bps)}
                        suffix='%'
                        min={0}
                        max={100}
                        onChange={(value) =>
                          setGroupRuleForm({
                            ...groupRuleForm,
                            profit_rate_bps: percentToBps(value),
                          })
                        }
                        style={{ width: '100%' }}
                      />
                    </div>
                    <div>
                      <Text type='tertiary' size='small'>
                        {t('最高返佣比例')}
                      </Text>
                      <InputNumber
                        value={bpsToPercent(groupRuleForm.max_commission_rate_bps)}
                        suffix='%'
                        min={0}
                        max={100}
                        onChange={(value) =>
                          setGroupRuleForm({
                            ...groupRuleForm,
                            max_commission_rate_bps: percentToBps(value),
                          })
                        }
                        style={{ width: '100%' }}
                      />
                    </div>
                    <div>
                      <Text type='tertiary' size='small'>
                        {t('利润分成上限')}
                      </Text>
                      <InputNumber
                        value={bpsToPercent(groupRuleForm.profit_share_rate_bps)}
                        suffix='%'
                        min={0}
                        max={100}
                        onChange={(value) =>
                          setGroupRuleForm({
                            ...groupRuleForm,
                            profit_share_rate_bps: percentToBps(value),
                          })
                        }
                        style={{ width: '100%' }}
                      />
                    </div>
                  </div>
                  <div className='rounded-lg border border-[var(--semi-color-border)] p-3 w-full'>
                    <div className='flex items-center gap-2'>
                      <Switch
                        checked={groupRuleForm.profit_protection_enabled}
                        onChange={(checked) =>
                          setGroupRuleForm({
                            ...groupRuleForm,
                            profit_protection_enabled: checked,
                          })
                        }
                      />
                      <Text strong>
                        {groupRuleForm.profit_protection_enabled
                          ? t('开启利润保护')
                          : t('关闭利润保护')}
                      </Text>
                    </div>
                    <Text type='tertiary' size='small'>
                      {t(
                        '开启后最终返佣取理论返佣与毛利分成上限的较小值；关闭后按理论返佣计算。',
                      )}
                    </Text>
                  </div>
                  <FormulaCode>
                    {`${t('理论返佣')} = ${t('消费额度')} × ${t('最高返佣比例')} × ${t('层级比例')}\n${t('毛利额度')} = ${t('消费额度')} × ${t('毛利率')}\n${t('利润保护上限')} = ${t('毛利额度')} × ${t('利润分成上限')} × ${t('层级比例')}\n${t('最终返佣')} = min(${t('理论返佣')}, ${t('利润保护上限')})`}
                  </FormulaCode>
                  <TextArea
                    value={groupRuleForm.remark}
                    onChange={(value) =>
                      setGroupRuleForm({ ...groupRuleForm, remark: value })
                    }
                    placeholder={t('备注')}
                    autosize
                    style={{ width: '100%' }}
                  />
                </Space>
              </Modal>

              <Button type='primary' loading={settingsLoading} onClick={saveSettings}>
                {t('保存规则配置')}
              </Button>
            </Space>
          </TabPane>
        </Tabs>
      </Card>
    </div>
  );
};

export default InviteCommission;
