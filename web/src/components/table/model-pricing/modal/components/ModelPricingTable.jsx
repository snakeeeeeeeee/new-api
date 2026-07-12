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

import React from 'react';
import { Card, Avatar, Typography, Table, Tag } from '@douyinfe/semi-ui';
import { IconCoinMoneyStroked } from '@douyinfe/semi-icons';
import {
  calculateModelPrice,
  formatRatioLabel,
  getModelPriceItems,
} from '../../../../../helpers';

const { Text } = Typography;

const buildTierRange = (tiers, index, t) => {
  const current = tiers[index];
  const previous = index > 0 ? tiers[index - 1] : null;
  if (index === 0) return `≤${current.up_to_inclusive}`;
  if (current.up_to_inclusive === null) {
    return `>${previous?.up_to_inclusive}`;
  }
  return `>${previous?.up_to_inclusive} ${t('且')} ≤${current.up_to_inclusive}`;
};

const ModelPricingTable = ({
  modelData,
  groupRatio,
  groupRatioDetails = {},
  currency,
  siteDisplayType,
  tokenUnit,
  displayPrice,
  showRatio,
  usableGroup,
  autoGroups = [],
  t,
}) => {
  const modelEnableGroups = Array.isArray(modelData?.enable_groups)
    ? modelData.enable_groups
    : [];
  const autoChain = autoGroups.filter((g) => modelEnableGroups.includes(g));
  const renderGroupPriceTable = () => {
    // 仅展示模型可用的分组：模型 enable_groups 与用户可用分组的交集

    const availableGroups = Object.keys(usableGroup || {})
      .filter((g) => g !== '')
      .filter((g) => g !== 'auto')
      .filter((g) => modelEnableGroups.includes(g));

    // 准备表格数据
    const tableData = availableGroups.map((group) => {
      const priceData = modelData
        ? calculateModelPrice({
            record: modelData,
            selectedGroup: group,
            groupRatio,
            tokenUnit,
            displayPrice,
            currency,
            quotaDisplayType: siteDisplayType,
          })
        : { inputPrice: '-', outputPrice: '-', price: '-' };

      // 获取分组倍率
      const groupRatioValue =
        groupRatio && groupRatio[group] !== undefined ? groupRatio[group] : 1;
      const groupRatioDetail =
        modelData?.group_ratio_details?.[group] || groupRatioDetails[group];

      const tiers = modelData?.token_tier_pricing?.rule?.tiers || [];
      const tierPrices = tiers.map((tier, index) => {
        if (index === 0 || tier.use_base_price) {
          return {
            range: buildTierRange(tiers, index, t),
            items: getModelPriceItems(priceData, t, siteDisplayType),
          };
        }
        const inputPrice = Number(tier.prices?.input || 0);
        const syntheticRecord = {
          ...modelData,
          model_ratio: inputPrice / 2,
          completion_ratio:
            inputPrice > 0 ? Number(tier.prices?.output || 0) / inputPrice : 0,
          cache_ratio:
            inputPrice > 0
              ? Number(tier.prices?.cached_input || 0) / inputPrice
              : 0,
          create_cache_ratio:
            inputPrice > 0
              ? Number(tier.prices?.cache_write || 0) / inputPrice
              : 0,
        };
        const tierPriceData = calculateModelPrice({
          record: syntheticRecord,
          selectedGroup: group,
          groupRatio,
          tokenUnit,
          displayPrice,
          currency,
          quotaDisplayType: siteDisplayType,
        });
        return {
          range: buildTierRange(tiers, index, t),
          items: getModelPriceItems(tierPriceData, t, siteDisplayType),
        };
      });

      return {
        key: group,
        group: group,
        ratio: priceData.usedGroupRatio ?? groupRatioValue,
        ratioDetail: groupRatioDetail,
        isDynamicRouteMaximum: priceData.isDynamicRouteMaximum,
        billingType:
          modelData?.quota_type === 0
            ? t('按量计费')
            : modelData?.quota_type === 1
              ? t('按次计费')
              : '-',
        priceItems: getModelPriceItems(priceData, t, siteDisplayType),
        tierPrices,
      };
    });

    // 定义表格列
    const columns = [
      {
        title: t('分组'),
        dataIndex: 'group',
        render: (text) => (
          <Tag color='white' size='small' shape='circle'>
            {text}
            {t('分组')}
          </Tag>
        ),
      },
    ];

    // 如果显示倍率，添加倍率列
    if (showRatio) {
      columns.push({
        title: t('倍率'),
        dataIndex: 'ratio',
        render: (text, record) => (
          <Tag color='white' size='small' shape='circle'>
            {record?.isDynamicRouteMaximum ? (
              `${formatRatioLabel(text)}x`
            ) : record?.ratioDetail?.has_ratio_override ? (
              <span style={{ display: 'inline-flex', gap: 4 }}>
                <span style={{ textDecoration: 'line-through' }}>
                  {formatRatioLabel(record.ratioDetail.original_ratio)}x
                </span>
                <span>{formatRatioLabel(record.ratioDetail.ratio)}x</span>
              </span>
            ) : (
              `${formatRatioLabel(text)}x`
            )}
          </Tag>
        ),
      });
    }

    // 添加计费类型列
    columns.push({
      title: t('计费类型'),
      dataIndex: 'billingType',
      render: (text) => {
        let color = 'white';
        if (text === t('按量计费')) color = 'violet';
        else if (text === t('按次计费')) color = 'teal';
        return (
          <Tag color={color} size='small' shape='circle'>
            {text || '-'}
          </Tag>
        );
      },
    });

    columns.push({
      title: siteDisplayType === 'TOKENS' ? t('计费摘要') : t('价格摘要'),
      dataIndex: 'priceItems',
      render: (items, record) => (
        <div className='space-y-1'>
          {items.map((item) => (
            <div key={item.key}>
              <div className='font-semibold text-orange-600'>
                {item.label} {item.value}
              </div>
              <div className='text-xs text-gray-500'>{item.suffix}</div>
            </div>
          ))}
          {record.tierPrices.length > 0 ? (
            <div
              className='mt-3 pt-3 space-y-3'
              style={{ borderTop: '1px solid var(--semi-color-border)' }}
            >
              <div className='text-xs font-medium text-gray-700'>
                {t('阶梯计价 · {{count}}档', {
                  count: record.tierPrices.length,
                })}
              </div>
              {record.tierPrices.map((tier, index) => (
                <div key={`${record.key}-${index}`}>
                  <div className='flex items-center gap-2 mb-1'>
                    <Tag size='small'>{tier.range}</Tag>
                    {index === 0 ? (
                      <span className='text-xs text-gray-500'>
                        {t('基础价格')}
                      </span>
                    ) : null}
                  </div>
                  <div className='grid grid-cols-1 sm:grid-cols-2 gap-x-3 gap-y-1'>
                    {tier.items.map((item) => (
                      <div key={item.key} className='text-xs text-gray-600'>
                        {item.label} {item.value}
                        {item.suffix}
                      </div>
                    ))}
                  </div>
                </div>
              ))}
              <div className='text-xs text-gray-500 leading-relaxed'>
                {t(
                  '档位由单次请求总输入 Token 数决定；命中更高档位后，本次请求全部输入和输出 Token 均按该档位计费。',
                )}
              </div>
            </div>
          ) : null}
        </div>
      ),
    });

    return (
      <Table
        dataSource={tableData}
        columns={columns}
        pagination={false}
        size='small'
        bordered={false}
        className='!rounded-lg'
      />
    );
  };

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='orange' className='mr-2 shadow-md'>
          <IconCoinMoneyStroked size={16} />
        </Avatar>
        <div>
          <Text className='text-lg font-medium'>{t('分组价格')}</Text>
          <div className='text-xs text-gray-600'>
            {t('不同用户分组的价格信息')}
          </div>
        </div>
      </div>
      {autoChain.length > 0 && (
        <div className='flex flex-wrap items-center gap-1 mb-4'>
          <span className='text-sm text-gray-600'>{t('auto分组调用链路')}</span>
          <span className='text-sm'>→</span>
          {autoChain.map((g, idx) => (
            <React.Fragment key={g}>
              <Tag color='white' size='small' shape='circle'>
                {g}
                {t('分组')}
              </Tag>
              {idx < autoChain.length - 1 && <span className='text-sm'>→</span>}
            </React.Fragment>
          ))}
        </div>
      )}
      {renderGroupPriceTable()}
    </Card>
  );
};

export default ModelPricingTable;
