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
import { Button, Card, DatePicker, Space, Typography } from '@douyinfe/semi-ui';
import {
  API,
  renderPaymentAmount,
  renderQuota,
  renderQuotaWithAmount,
  showError,
} from '../../helpers';

const { Text } = Typography;

const sevenDaysAgo = () => new Date(Date.now() - 7 * 24 * 60 * 60 * 1000);
const nowDate = () => new Date();
const toTimestamp = (value) => Math.floor(new Date(value).getTime() / 1000);

const parseDateRange = (range) => {
  if (!Array.isArray(range) || range.length !== 2 || !range[0] || !range[1]) {
    return [toTimestamp(sevenDaysAgo()), toTimestamp(nowDate())];
  }
  return [toTimestamp(range[0]), toTimestamp(range[1])];
};

const InviteCommissionSummary = ({ t }) => {
  const [dateRange, setDateRange] = useState([sevenDaysAgo(), nowDate()]);
  const [report, setReport] = useState(null);
  const [loading, setLoading] = useState(false);

  const loadReport = async () => {
    const [start, end] = parseDateRange(dateRange);
    setLoading(true);
    try {
      const res = await API.get(
        `/api/invite_commission/self/report?start_timestamp=${start}&end_timestamp=${end}`,
      );
      if (res.data?.success) {
        setReport(res.data.data);
      } else {
        showError(res.data?.message || t('查询失败'));
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadReport();
  }, []);

  const summary = report?.summary || {};
  const services = report?.services || [];
  const serviceMap = useMemo(() => {
    const map = {};
    services.forEach((item) => {
      map[item.service] = item;
    });
    return map;
  }, [services]);

  const metrics = [
    [t('一级邀请人数'), summary.level1_invitee_count || 0],
    [t('二级邀请人数'), summary.level2_invitee_count || 0],
    [t('钱包充值额度'), renderQuotaWithAmount(summary.wallet_recharge_amount || 0)],
    [t('真实充值金额'), renderPaymentAmount(summary.wallet_recharge_money || 0)],
    [t('兑换码兑换额度'), renderQuota(summary.redemption_quota || 0)],
    [t('订阅购买金额'), renderPaymentAmount(summary.subscription_purchase_money || 0)],
    [t('GPT 消费'), renderQuota(serviceMap.gpt?.total_consumption_quota || 0)],
    [t('Claude 消费'), renderQuota(serviceMap.claude?.total_consumption_quota || 0)],
    [t('Gemini 消费'), renderQuota(serviceMap.gemini?.total_consumption_quota || 0)],
    [t('预估返佣'), renderQuota(summary.estimated_commission_quota || 0, 4)],
  ];

  return (
    <Card
      className='!rounded-xl w-full'
      title={<Text type='tertiary'>{t('邀请返佣')}</Text>}
    >
      <Space vertical align='start' style={{ width: '100%' }}>
        <div className='flex flex-col sm:flex-row gap-2 w-full'>
          <DatePicker
            type='dateTimeRange'
            value={dateRange}
            onChange={setDateRange}
            style={{ width: 320, maxWidth: '100%' }}
          />
          <Button loading={loading} onClick={loadReport}>
            {t('查询')}
          </Button>
        </div>
        <div className='grid grid-cols-2 md:grid-cols-5 gap-3 w-full'>
          {metrics.map(([label, value]) => (
            <div
              key={label}
              className='rounded-lg border border-[var(--semi-color-border)] p-3 min-w-0'
            >
              <Text type='tertiary' size='small' ellipsis={{ showTooltip: true }}>
                {label}
              </Text>
              <div className='mt-1 font-semibold truncate' title={String(value)}>
                {value}
              </div>
            </div>
          ))}
        </div>
      </Space>
    </Card>
  );
};

export default InviteCommissionSummary;
