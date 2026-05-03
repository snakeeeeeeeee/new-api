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

import React, { useEffect, useRef, useState } from 'react';
import {
  Avatar,
  Typography,
  Tag,
  Card,
  Button,
  Banner,
  Skeleton,
  Form,
  Space,
  Row,
  Col,
  Spin,
  Tooltip,
  Tabs,
  TabPane,
} from '@douyinfe/semi-ui';
import { SiAlipay, SiWechat, SiStripe } from 'react-icons/si';
import {
  CreditCard,
  Coins,
  Wallet,
  BarChart2,
  TrendingUp,
  Receipt,
  Sparkles,
} from 'lucide-react';
import { IconGift } from '@douyinfe/semi-icons';
import { useMinimumLoadingTime } from '../../hooks/common/useMinimumLoadingTime';
import { getCurrencyConfig } from '../../helpers/render';
import { normalizeSubscriptionQuotaSummary } from '../../helpers';
import SubscriptionPlansCard from './SubscriptionPlansCard';

const { Text } = Typography;

const RechargeCard = ({
  t,
  enableOnlineTopUp,
  enableStripeTopUp,
  enableCreemTopUp,
  creemProducts,
  creemPreTopUp,
  presetAmounts,
  selectedPreset,
  selectPresetAmount,
  formatLargeNumber,
  priceRatio,
  topUpCount,
  minTopUp,
  renderQuotaWithAmount,
  getAmount,
  setTopUpCount,
  setSelectedPreset,
  renderAmount,
  amountLoading,
  payMethods,
  preTopUp,
  paymentLoading,
  payWay,
  redemptionCode,
  setRedemptionCode,
  topUp,
  isSubmitting,
  topUpLink,
  openTopUpLink,
  userState,
  renderQuota,
  statusLoading,
  topupInfo,
  onOpenHistory,
  enableWaffoTopUp,
  waffoTopUp,
  waffoPayMethods,
  subscriptionLoading = false,
  subscriptionPlans = [],
  billingPreference,
  onChangeBillingPreference,
  activeSubscriptions = [],
  allSubscriptions = [],
  reloadSubscriptionSelf,
}) => {
  const onlineFormApiRef = useRef(null);
  const redeemFormApiRef = useRef(null);
  const initialTabSetRef = useRef(false);
  const showAmountSkeleton = useMinimumLoadingTime(amountLoading);
  const [activeTab, setActiveTab] = useState('topup');
  const shouldShowSubscription =
    !subscriptionLoading && subscriptionPlans.length > 0;

  useEffect(() => {
    if (initialTabSetRef.current) return;
    if (subscriptionLoading) return;
    setActiveTab('topup');
    initialTabSetRef.current = true;
  }, [shouldShowSubscription, subscriptionLoading]);

  useEffect(() => {
    if (!shouldShowSubscription && activeTab !== 'topup') {
      setActiveTab('topup');
    }
  }, [shouldShowSubscription, activeTab]);

  const subscriptionQuotaSummary = normalizeSubscriptionQuotaSummary(
    userState?.user?.subscription_quota,
  );
  const subscriptionQuotaLabel = subscriptionQuotaSummary.hasActive
    ? subscriptionQuotaSummary.hasUnlimited
      ? subscriptionQuotaSummary.hasLimited
        ? `${t('不限')} + ${renderQuota(subscriptionQuotaSummary.amountRemain)}`
        : t('不限')
      : renderQuota(subscriptionQuotaSummary.amountRemain)
    : '--';
  const subscriptionRemainPercent = subscriptionQuotaSummary.hasLimited
    ? subscriptionQuotaSummary.remainPercent
    : 0;

  const topupContent = (
    <Space vertical style={{ width: '100%' }}>
      {/* 统计数据 */}
      <Card
        className='!rounded-xl w-full'
        cover={
          <div
            className='relative overflow-hidden'
            style={{
              background:
                'linear-gradient(135deg, #f8fafc 0%, #eef8f2 48%, #f5f2ff 100%)',
              borderBottom: '1px solid rgba(15, 23, 42, 0.08)',
            }}
          >
            <div
              className='pointer-events-none absolute inset-x-0 top-0 h-1'
              style={{
                background:
                  'linear-gradient(90deg, #10b981 0%, #38bdf8 52%, #8b5cf6 100%)',
              }}
            />
            <div className='relative z-10 p-5 sm:p-6'>
              <div className='mb-5 flex items-center justify-between'>
                <Text strong style={{ color: '#0f172a', fontSize: '16px' }}>
                  {t('账户统计')}
                </Text>
              </div>

              {/* 统计数据 */}
              <div className='grid grid-cols-2 gap-x-6 gap-y-6 sm:grid-cols-4 sm:gap-x-8'>
                {/* 当前余额 */}
                <div className='flex min-h-[88px] flex-col justify-between text-center sm:text-left'>
                  <div className='flex h-[54px] flex-col justify-start'>
                    <div
                      className='break-words text-xl font-semibold leading-none tabular-nums sm:text-2xl'
                      style={{ color: '#111827' }}
                    >
                      {renderQuota(userState?.user?.quota)}
                    </div>
                  </div>
                  <div className='flex h-6 items-center justify-center text-sm sm:justify-start'>
                    <Wallet
                      size={14}
                      className='mr-1'
                      style={{ color: '#64748b' }}
                    />
                    <Text
                      style={{
                        color: '#64748b',
                        fontSize: '12px',
                      }}
                    >
                      {t('当前余额')}
                    </Text>
                  </div>
                </div>

                {/* 历史消耗 */}
                <div className='flex min-h-[88px] flex-col justify-between text-center sm:text-left'>
                  <div className='flex h-[54px] flex-col justify-start'>
                    <div
                      className='break-words text-xl font-semibold leading-none tabular-nums sm:text-2xl'
                      style={{ color: '#111827' }}
                    >
                      {renderQuota(userState?.user?.used_quota)}
                    </div>
                  </div>
                  <div className='flex h-6 items-center justify-center text-sm sm:justify-start'>
                    <TrendingUp
                      size={14}
                      className='mr-1'
                      style={{ color: '#64748b' }}
                    />
                    <Text
                      style={{
                        color: '#64748b',
                        fontSize: '12px',
                      }}
                    >
                      {t('历史消耗')}
                    </Text>
                  </div>
                </div>

                {/* 请求次数 */}
                <div className='flex min-h-[88px] flex-col justify-between text-center sm:text-left'>
                  <div className='flex h-[54px] flex-col justify-start'>
                    <div
                      className='break-words text-xl font-semibold leading-none tabular-nums sm:text-2xl'
                      style={{ color: '#111827' }}
                    >
                      {userState?.user?.request_count || 0}
                    </div>
                  </div>
                  <div className='flex h-6 items-center justify-center text-sm sm:justify-start'>
                    <BarChart2
                      size={14}
                      className='mr-1'
                      style={{ color: '#64748b' }}
                    />
                    <Text
                      style={{
                        color: '#64748b',
                        fontSize: '12px',
                      }}
                    >
                      {t('请求次数')}
                    </Text>
                  </div>
                </div>

                {/* 订阅额度 */}
                <div className='flex min-h-[88px] flex-col justify-between text-center sm:text-left'>
                  <div className='flex h-[54px] flex-col justify-start'>
                    {subscriptionQuotaSummary.hasActive ? (
                      <div className='mx-auto flex w-max max-w-full min-w-0 flex-col justify-start sm:mx-0'>
                        <div
                          className='whitespace-nowrap text-center text-lg font-semibold leading-none tabular-nums sm:text-left sm:text-xl'
                          style={{ color: '#111827' }}
                          title={
                            subscriptionQuotaSummary.hasUnlimited
                              ? subscriptionQuotaLabel
                              : `${renderQuota(subscriptionQuotaSummary.amountRemain)} / ${renderQuota(subscriptionQuotaSummary.amountTotal)}`
                          }
                        >
                          {subscriptionQuotaSummary.hasUnlimited
                            ? subscriptionQuotaLabel
                            : `${renderQuota(subscriptionQuotaSummary.amountRemain)} / ${renderQuota(subscriptionQuotaSummary.amountTotal)}`}
                        </div>
                        {subscriptionQuotaSummary.hasLimited ? (
                          <div
                            className='relative mt-2 h-1.5 w-full overflow-hidden rounded-full'
                            aria-label='subscription quota remaining'
                            style={{
                              background:
                                'linear-gradient(90deg, #ef4444 0%, #ef4444 33.333%, #f59e0b 33.333%, #f59e0b 66.666%, #10b981 66.666%, #10b981 100%)',
                            }}
                          >
                            <div
                              className='absolute inset-y-0 right-0 bg-slate-200/90'
                              style={{
                                width: `${100 - Math.min(100, Math.max(0, subscriptionRemainPercent))}%`,
                              }}
                            />
                            <div
                              className='absolute top-[1px] bottom-[1px] w-px rounded-full bg-white/90 shadow-[0_0_0_1px_rgba(15,23,42,0.08)]'
                              style={{ left: '33.333%' }}
                            />
                            <div
                              className='absolute top-[1px] bottom-[1px] w-px rounded-full bg-white/90 shadow-[0_0_0_1px_rgba(15,23,42,0.08)]'
                              style={{ left: '66.666%' }}
                            />
                          </div>
                        ) : null}
                      </div>
                    ) : (
                      <div
                        className='text-xl font-semibold leading-none sm:text-2xl'
                        style={{ color: '#111827' }}
                      >
                        --
                      </div>
                    )}
                  </div>
                  <div className='flex h-6 items-center justify-center text-sm sm:justify-start'>
                    <Sparkles
                      size={14}
                      className='mr-1'
                      style={{ color: '#64748b' }}
                    />
                    <Text
                      style={{
                        color: '#64748b',
                        fontSize: '12px',
                      }}
                    >
                      {t('订阅额度')}
                    </Text>
                  </div>
                </div>
              </div>
            </div>
          </div>
        }
      >
        {/* 在线充值表单 */}
        {statusLoading ? (
          <div className='py-8 flex justify-center'>
            <Spin size='large' />
          </div>
        ) : enableOnlineTopUp || enableStripeTopUp || enableCreemTopUp || enableWaffoTopUp ? (
          <Form
            getFormApi={(api) => (onlineFormApiRef.current = api)}
            initValues={{ topUpCount: topUpCount }}
          >
            <div className='space-y-6'>
              {(enableOnlineTopUp || enableStripeTopUp || enableWaffoTopUp) && (
                <Row gutter={12}>
                  <Col xs={24} sm={24} md={24} lg={10} xl={10}>
                    <Form.InputNumber
                      field='topUpCount'
                      label={t('充值额度')}
                      disabled={!enableOnlineTopUp && !enableStripeTopUp && !enableWaffoTopUp}
                      placeholder={
                        t('充值额度，最低 ') + renderQuotaWithAmount(minTopUp)
                      }
                      value={topUpCount}
                      min={minTopUp}
                      max={999999999}
                      step={1}
                      precision={0}
                      onChange={async (value) => {
                        if (value && value >= 1) {
                          setTopUpCount(value);
                          setSelectedPreset(null);
                          await getAmount(value);
                        }
                      }}
                      onBlur={(e) => {
                        const value = parseInt(e.target.value);
                        if (!value || value < 1) {
                          setTopUpCount(1);
                          getAmount(1);
                        }
                      }}
                      formatter={(value) => (value ? `${value}` : '')}
                      parser={(value) =>
                        value ? parseInt(value.replace(/[^\d]/g, '')) : 0
                      }
                      suffix='$'
                      extraText={
                        <Skeleton
                          loading={showAmountSkeleton}
                          active
                          placeholder={
                            <Skeleton.Title
                              style={{
                                width: 120,
                                height: 20,
                                borderRadius: 6,
                              }}
                            />
                          }
                        >
                          <Text type='secondary' className='text-red-600'>
                            {t('实付金额：')}
                            <span style={{ color: 'red' }}>
                              {renderAmount()}
                            </span>
                          </Text>
                        </Skeleton>
                      }
                      style={{ width: '100%' }}
                    />
                  </Col>
                  {payMethods && payMethods.filter(m => m.type !== 'waffo').length > 0 && (
                  <Col xs={24} sm={24} md={24} lg={14} xl={14}>
                    <Form.Slot label={t('选择支付方式')}>
                        <Space wrap>
                          {payMethods.filter(m => m.type !== 'waffo').map((payMethod) => {
                            const minTopupVal = Number(payMethod.min_topup) || 0;
                            const isStripe = payMethod.type === 'stripe';
                            const disabled =
                              (!enableOnlineTopUp && !isStripe) ||
                              (!enableStripeTopUp && isStripe) ||
                              minTopupVal > Number(topUpCount || 0);

                            const buttonEl = (
                              <Button
                                key={payMethod.type}
                                theme='outline'
                                type='tertiary'
                                onClick={() => preTopUp(payMethod.type)}
                                disabled={disabled}
                                loading={
                                  paymentLoading && payWay === payMethod.type
                                }
                                icon={
                                  payMethod.type === 'alipay' ? (
                                    <SiAlipay size={18} color='#1677FF' />
                                  ) : payMethod.type === 'wxpay' ? (
                                    <SiWechat size={18} color='#07C160' />
                                  ) : payMethod.type === 'stripe' ? (
                                    <SiStripe size={18} color='#635BFF' />
                                  ) : (
                                    <CreditCard
                                      size={18}
                                      color={
                                        payMethod.color ||
                                        'var(--semi-color-text-2)'
                                      }
                                    />
                                  )
                                }
                                className='!rounded-lg !px-4 !py-2'
                              >
                                {payMethod.name}
                              </Button>
                            );

                            return disabled &&
                              minTopupVal > Number(topUpCount || 0) ? (
                              <Tooltip
                                content={
                                  t('此支付方式最低充值金额为') +
                                  ' ' +
                                  minTopupVal
                                }
                                key={payMethod.type}
                              >
                                {buttonEl}
                              </Tooltip>
                            ) : (
                              <React.Fragment key={payMethod.type}>
                                {buttonEl}
                              </React.Fragment>
                            );
                          })}
                        </Space>
                    </Form.Slot>
                  </Col>
                  )}
                </Row>
              )}

              {(enableOnlineTopUp || enableStripeTopUp || enableWaffoTopUp) && (
                <Form.Slot
                  label={
                    <div className='flex items-center gap-2'>
                      <span>{t('选择充值额度')}</span>
                      {(() => {
                        const { symbol, rate, type } = getCurrencyConfig();
                        if (type === 'USD') return null;

                        return (
                          <span
                            style={{
                              color: 'var(--semi-color-text-2)',
                              fontSize: '12px',
                              fontWeight: 'normal',
                            }}
                          >
                            (1 $ = {rate.toFixed(2)} {symbol})
                          </span>
                        );
                      })()}
                    </div>
                  }
                >
                  <div className='grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-2'>
                    {presetAmounts.map((preset, index) => {
                      const discount =
                        preset.discount || topupInfo?.discount?.[preset.value] || 1.0;
                      const originalPrice = preset.value * priceRatio;
                      const discountedPrice = originalPrice * discount;
                      const hasDiscount = discount < 1.0;
                      const actualPayCNY = discountedPrice;

                      // 根据当前货币类型换算显示金额和数量
                      const { symbol, rate, type } = getCurrencyConfig();

                      let displayValue = preset.value; // 显示的数量

                      if (type === 'CNY') {
                        // 数量转CNY展示
                        displayValue = preset.value * rate;
                      } else if (type === 'CUSTOM') {
                        // 数量转自定义货币展示，支付金额始终按人民币显示
                        displayValue = preset.value * rate;
                      }

                      return (
                        <Card
                          key={index}
                          style={{
                            cursor: 'pointer',
                            border:
                              selectedPreset === preset.value
                                ? '2px solid var(--semi-color-primary)'
                                : '1px solid var(--semi-color-border)',
                            height: '100%',
                            width: '100%',
                          }}
                          bodyStyle={{ padding: '12px' }}
                          onClick={() => {
                            selectPresetAmount(preset);
                            onlineFormApiRef.current?.setValue(
                              'topUpCount',
                              preset.value,
                            );
                          }}
                        >
                          <div style={{ textAlign: 'center' }}>
                            <Typography.Title
                              heading={6}
                              style={{ margin: '0 0 8px 0' }}
                            >
                              <Coins size={18} />
                              {formatLargeNumber(displayValue)} {symbol}
                              {hasDiscount && (
                                <Tag style={{ marginLeft: 4 }} color='green'>
                                  {t('折').includes('off')
                                    ? ((1 - parseFloat(discount)) * 100).toFixed(1)
                                    : (discount * 10).toFixed(1)}
                                  {t('折')}
                                </Tag>
                              )}
                            </Typography.Title>
                            <div
                              style={{
                                color: 'var(--semi-color-text-2)',
                                fontSize: '12px',
                                margin: '4px 0',
                              }}
                            >
                              {t('实付')} ¥{actualPayCNY.toFixed(2)}
                            </div>
                          </div>
                        </Card>
                      );
                    })}
                  </div>
                </Form.Slot>
              )}

              {/* Waffo 充值区域 */}
              {enableWaffoTopUp &&
                waffoPayMethods &&
                waffoPayMethods.length > 0 && (
                  <Form.Slot label={t('Waffo 充值')}>
                    <Space wrap>
                      {waffoPayMethods.map((method, index) => (
                        <Button
                          key={index}
                          theme='outline'
                          type='tertiary'
                          onClick={() => waffoTopUp(index)}
                          loading={paymentLoading}
                          icon={
                            method.icon ? (
                              <img
                                src={method.icon}
                                alt={method.name}
                                style={{
                                  width: 36,
                                  height: 36,
                                  objectFit: 'contain',
                                }}
                              />
                            ) : (
                              <CreditCard
                                size={18}
                                color='var(--semi-color-text-2)'
                              />
                            )
                          }
                          className='!rounded-lg !px-4 !py-2'
                        >
                          {method.name}
                        </Button>
                      ))}
                    </Space>
                  </Form.Slot>
                )}

              {/* Creem 充值区域 */}
              {enableCreemTopUp && creemProducts.length > 0 && (
                <Form.Slot label={t('Creem 充值')}>
                  <div className='grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3'>
                    {creemProducts.map((product, index) => (
                      <Card
                        key={index}
                        onClick={() => creemPreTopUp(product)}
                        className='cursor-pointer !rounded-2xl transition-all hover:shadow-md border-gray-200 hover:border-gray-300'
                        bodyStyle={{ textAlign: 'center', padding: '16px' }}
                      >
                        <div className='font-medium text-lg mb-2'>
                          {product.name}
                        </div>
                        <div className='text-sm text-gray-600 mb-2'>
                          {t('充值额度')}: {product.quota}
                        </div>
                        <div className='text-lg font-semibold text-blue-600'>
                          {product.currency === 'EUR' ? '€' : '$'}
                          {product.price}
                        </div>
                      </Card>
                    ))}
                  </div>
                </Form.Slot>
              )}
            </div>
          </Form>
        ) : (
          <Banner
            type='info'
            description={t(
              '管理员未开启在线充值功能，请联系管理员开启或使用兑换码充值。',
            )}
            className='!rounded-xl'
            closeIcon={null}
          />
        )}
      </Card>

      {/* 兑换码充值 */}
      <Card
        className='!rounded-xl w-full'
        title={
          <Text type='tertiary' strong>
            {t('兑换码充值')}
          </Text>
        }
      >
        <Form
          getFormApi={(api) => (redeemFormApiRef.current = api)}
          initValues={{ redemptionCode: redemptionCode }}
        >
          <Form.Input
            field='redemptionCode'
            noLabel={true}
            placeholder={t('请输入兑换码')}
            value={redemptionCode}
            onChange={(value) => setRedemptionCode(value)}
            prefix={<IconGift />}
            suffix={
              <div className='flex items-center gap-2'>
                <Button
                  type='primary'
                  theme='solid'
                  onClick={topUp}
                  loading={isSubmitting}
                >
                  {t('兑换额度')}
                </Button>
              </div>
            }
            showClear
            style={{ width: '100%' }}
            extraText={
              topUpLink && (
                <Text type='tertiary'>
                  {t('在找兑换码？')}
                  <Text
                    type='secondary'
                    underline
                    className='cursor-pointer'
                    onClick={openTopUpLink}
                  >
                    {t('购买兑换码')}
                  </Text>
                </Text>
              )
            }
          />
        </Form>
      </Card>
    </Space>
  );

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      {/* 卡片头部 */}
      <div className='flex items-center justify-between mb-4'>
        <div className='flex items-center'>
          <Avatar size='small' color='blue' className='mr-3 shadow-md'>
            <CreditCard size={16} />
          </Avatar>
          <div>
            <Typography.Text className='text-lg font-medium'>
              {t('账户充值')}
            </Typography.Text>
            <div className='text-xs'>{t('多种充值方式，安全便捷')}</div>
          </div>
        </div>
        <Button
          icon={<Receipt size={16} />}
          theme='solid'
          onClick={onOpenHistory}
        >
          {t('账单')}
        </Button>
      </div>

      {shouldShowSubscription ? (
        <Tabs type='card' activeKey={activeTab} onChange={setActiveTab}>
          <TabPane
            tab={
              <div className='flex items-center gap-2'>
                <Wallet size={16} />
                {t('额度充值')}
              </div>
            }
            itemKey='topup'
          >
            <div className='py-2'>{topupContent}</div>
          </TabPane>
          <TabPane
            tab={
              <div className='flex items-center gap-2'>
                <Sparkles size={16} />
                {t('订阅套餐')}
              </div>
            }
            itemKey='subscription'
          >
            <div className='py-2'>
              <SubscriptionPlansCard
                t={t}
                loading={subscriptionLoading}
                plans={subscriptionPlans}
                payMethods={payMethods}
                enableOnlineTopUp={enableOnlineTopUp}
                enableStripeTopUp={enableStripeTopUp}
                enableCreemTopUp={enableCreemTopUp}
                billingPreference={billingPreference}
                onChangeBillingPreference={onChangeBillingPreference}
                activeSubscriptions={activeSubscriptions}
                allSubscriptions={allSubscriptions}
                reloadSubscriptionSelf={reloadSubscriptionSelf}
                withCard={false}
              />
            </div>
          </TabPane>
        </Tabs>
      ) : (
        topupContent
      )}
    </Card>
  );
};

export default RechargeCard;
