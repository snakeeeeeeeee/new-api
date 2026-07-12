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

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { useTranslation } from 'react-i18next';
import { initVChartSemiTheme } from '@visactor/vchart-semi-theme';
import { Banner, Button, TabPane, Tabs, Typography } from '@douyinfe/semi-ui';
import { BarChart3, ListOrdered, RefreshCw, WalletCards } from 'lucide-react';
import { API, showError } from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import FundingRecords from './FundingRecords';
import UsageOverview from './UsageOverview';
import UsageRanking from './UsageRanking';
import UsageStatsDetails from './UsageStatsDetails';
import UsageStatsFilters from './UsageStatsFilters';
import {
  buildUsageStatsParams,
  createDefaultFilters,
  filtersCacheKey,
} from './utils';

const { Text, Title } = Typography;

const UsageStatsPage = () => {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [draftFilters, setDraftFilters] = useState(createDefaultFilters);
  const [appliedFilters, setAppliedFilters] = useState(createDefaultFilters);
  const [activeTab, setActiveTab] = useState('overview');
  const [rankingMode, setRankingMode] = useState('total');
  const [fundingMode, setFundingMode] = useState('recharge');
  const [sectionData, setSectionData] = useState({});
  const [sectionLoading, setSectionLoading] = useState({});
  const [sectionErrors, setSectionErrors] = useState({});
  const [userState, setUserState] = useState({
    record: null,
    source: 'all',
    data: null,
    loading: false,
  });
  const [fundingState, setFundingState] = useState({
    record: null,
    mode: 'recharge',
    data: null,
    loading: false,
  });
  const cacheRef = useRef(new Map());
  const requestVersionRef = useRef(0);
  const appliedKey = useMemo(
    () => filtersCacheKey(appliedFilters),
    [appliedFilters],
  );
  const currentSection = activeTab === 'funding' ? fundingMode : 'usage';

  useEffect(() => {
    initVChartSemiTheme({ isWatchingThemeSwitch: true });
  }, []);

  const fetchSection = useCallback(
    async (section, options = {}, force = false) => {
      const params = buildUsageStatsParams(appliedFilters);
      if (!params) return;
      params.section = section;
      if (section === 'recharge') {
        params.recharge_page = options.page || 1;
        params.recharge_page_size = options.pageSize || 20;
      } else if (section === 'subscription_purchase') {
        params.subscription_purchase_page = options.page || 1;
        params.subscription_purchase_page_size = options.pageSize || 20;
      }
      const cacheKey = JSON.stringify(params);
      if (!force && cacheRef.current.has(cacheKey)) {
        setSectionData((current) => ({
          ...current,
          [section]: cacheRef.current.get(cacheKey),
        }));
        return;
      }

      const requestVersion = requestVersionRef.current;
      setSectionLoading((current) => ({ ...current, [section]: true }));
      setSectionErrors((current) => ({ ...current, [section]: '' }));
      try {
        const response = await API.get('/api/log/usage_stats', { params });
        const { success, message, data } = response.data;
        if (!success) throw new Error(message || t('加载失败'));
        if (requestVersion !== requestVersionRef.current) return;
        cacheRef.current.set(cacheKey, data);
        setSectionData((current) => ({ ...current, [section]: data }));
      } catch (error) {
        if (requestVersion !== requestVersionRef.current) return;
        const message = error.message || t('加载失败');
        setSectionErrors((current) => ({ ...current, [section]: message }));
        showError(message);
      } finally {
        if (requestVersion === requestVersionRef.current) {
          setSectionLoading((current) => ({ ...current, [section]: false }));
        }
      }
    },
    [appliedFilters, t],
  );

  useEffect(() => {
    fetchSection(currentSection);
  }, [appliedKey, currentSection, fetchSection]);

  const closeDetails = () => {
    setUserState({ record: null, source: 'all', data: null, loading: false });
    setFundingState({
      record: null,
      mode: 'recharge',
      data: null,
      loading: false,
    });
  };

  const applyFilters = (nextFilters = draftFilters) => {
    if (!buildUsageStatsParams(nextFilters)) {
      showError(t('请选择时间范围'));
      return;
    }
    requestVersionRef.current += 1;
    cacheRef.current.clear();
    setSectionData({});
    setSectionErrors({});
    closeDetails();
    setAppliedFilters({
      ...nextFilters,
      dateRange: [...nextFilters.dateRange],
    });
  };

  const resetFilters = () => {
    const defaults = createDefaultFilters();
    setDraftFilters(defaults);
    applyFilters(defaults);
  };

  const currentPageOptions = () => {
    const pageData =
      currentSection === 'recharge'
        ? sectionData.recharge?.recharge_ranking
        : sectionData.subscription_purchase?.subscription_purchase_ranking;
    return {
      page: pageData?.page || 1,
      pageSize: pageData?.page_size || 20,
    };
  };

  const refreshCurrentSection = () =>
    fetchSection(
      currentSection,
      currentSection === 'usage' ? {} : currentPageOptions(),
      true,
    );

  const loadUserDetail = async (record, source) => {
    const params = buildUsageStatsParams(appliedFilters);
    if (!record || !params) return;
    setUserState({ record, source, data: null, loading: true });
    params.section = 'usage';
    params.user_id = record.user_id;
    params.billing_source = source;
    try {
      const response = await API.get('/api/log/usage_stats', { params });
      const { success, message, data } = response.data;
      if (!success) throw new Error(message || t('加载失败'));
      setUserState({ record, source, data, loading: false });
    } catch (error) {
      showError(error.message || t('加载失败'));
      setUserState((current) => ({ ...current, loading: false }));
    }
  };

  const loadFundingDetail = async (record, mode, page = 1, pageSize = 20) => {
    const params = buildUsageStatsParams(appliedFilters);
    if (!record || !params) return;
    setFundingState((current) => ({
      ...current,
      record,
      mode,
      loading: true,
    }));
    params.section = mode;
    if (mode === 'recharge') {
      params.recharge_user_id = record.user_id;
      params.recharge_detail_page = page;
      params.recharge_detail_page_size = pageSize;
    } else {
      params.subscription_purchase_user_id = record.user_id;
      params.subscription_purchase_detail_page = page;
      params.subscription_purchase_detail_page_size = pageSize;
    }
    try {
      const response = await API.get('/api/log/usage_stats', { params });
      const { success, message, data } = response.data;
      if (!success) throw new Error(message || t('加载失败'));
      setFundingState({ record, mode, data, loading: false });
    } catch (error) {
      showError(error.message || t('加载失败'));
      setFundingState((current) => ({ ...current, loading: false }));
    }
  };

  const usageData = sectionData.usage;
  const activeData = sectionData[currentSection];
  const activeLoading = !!sectionLoading[currentSection];
  const activeError = sectionErrors[currentSection];

  return (
    <div className='mt-[60px] overflow-x-hidden px-2 pb-8'>
      <div className='mx-auto flex w-full max-w-[1680px] flex-col gap-4'>
        <header className='flex flex-col gap-1 md:flex-row md:items-end md:justify-between'>
          <div>
            <Title heading={4} className='!mb-0'>
              {t('用量统计')}
            </Title>
            <Text type='tertiary'>{t('运营数据')}</Text>
          </div>
          {activeData?.generated_at && (
            <Text type='tertiary' size='small'>
              {t('数据已更新')}
            </Text>
          )}
        </header>

        <UsageStatsFilters
          value={draftFilters}
          onChange={setDraftFilters}
          onApply={() => applyFilters()}
          onRefresh={refreshCurrentSection}
          onReset={resetFilters}
          loading={activeLoading}
          canRefresh={!!activeData}
        />

        <Tabs
          activeKey={activeTab}
          onChange={setActiveTab}
          type='button'
          className='usage-stats-primary-tabs'
        >
          <TabPane
            itemKey='overview'
            tab={
              <span className='inline-flex items-center gap-2'>
                <BarChart3 size={16} />
                {t('概览')}
              </span>
            }
          />
          <TabPane
            itemKey='ranking'
            tab={
              <span className='inline-flex items-center gap-2'>
                <ListOrdered size={16} />
                {t('消费排行')}
              </span>
            }
          />
          <TabPane
            itemKey='funding'
            tab={
              <span className='inline-flex items-center gap-2'>
                <WalletCards size={16} />
                {t('资金记录')}
              </span>
            }
          />
        </Tabs>

        {activeError && !activeData && (
          <Banner
            type='danger'
            fullMode={false}
            title={t('加载失败')}
            description={activeError}
            closeIcon={null}
            action={
              <Button
                icon={<RefreshCw size={16} />}
                onClick={refreshCurrentSection}
              >
                {t('重试')}
              </Button>
            }
          />
        )}

        {activeTab === 'overview' && (
          <UsageOverview data={usageData} loading={activeLoading} />
        )}
        {activeTab === 'ranking' && (
          <UsageRanking
            data={usageData}
            loading={activeLoading}
            mode={rankingMode}
            onModeChange={setRankingMode}
            onUserSelect={loadUserDetail}
            isMobile={isMobile}
          />
        )}
        {activeTab === 'funding' && (
          <FundingRecords
            mode={fundingMode}
            onModeChange={setFundingMode}
            data={sectionData[fundingMode]}
            loading={activeLoading}
            isMobile={isMobile}
            onPageChange={(page) =>
              fetchSection(fundingMode, {
                page,
                pageSize:
                  (fundingMode === 'recharge'
                    ? sectionData.recharge?.recharge_ranking?.page_size
                    : sectionData.subscription_purchase
                        ?.subscription_purchase_ranking?.page_size) || 20,
              })
            }
            onPageSizeChange={(pageSize) =>
              fetchSection(fundingMode, { page: 1, pageSize })
            }
            onUserSelect={loadFundingDetail}
          />
        )}
      </div>

      <UsageStatsDetails
        userState={userState}
        onCloseUser={() =>
          setUserState({
            record: null,
            source: 'all',
            data: null,
            loading: false,
          })
        }
        fundingState={fundingState}
        onCloseFunding={() =>
          setFundingState({
            record: null,
            mode: 'recharge',
            data: null,
            loading: false,
          })
        }
        onFundingPageChange={(page) => {
          const detailPage =
            fundingState.mode === 'recharge'
              ? fundingState.data?.recharge_details
              : fundingState.data?.subscription_purchase_details;
          loadFundingDetail(
            fundingState.record,
            fundingState.mode,
            page,
            detailPage?.page_size || 20,
          );
        }}
        onFundingPageSizeChange={(pageSize) =>
          loadFundingDetail(fundingState.record, fundingState.mode, 1, pageSize)
        }
        isMobile={isMobile}
      />
    </div>
  );
};

export default UsageStatsPage;
