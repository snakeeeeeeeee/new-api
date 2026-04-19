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

import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';

export const useInviteCodesData = () => {
  const { t } = useTranslation();
  const [inviteCodes, setInviteCodes] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [inviteCodeCount, setInviteCodeCount] = useState(0);
  const [showEdit, setShowEdit] = useState(false);
  const [editingInviteCode, setEditingInviteCode] = useState({ id: undefined });
  const [formApi, setFormApi] = useState(null);

  const formInitValues = {
    searchKeyword: '',
  };

  const getFormValues = () => {
    const formValues = formApi ? formApi.getValues() : {};
    return {
      searchKeyword: formValues.searchKeyword || '',
    };
  };

  const loadInviteCodes = async (page = 1, currentPageSize = pageSize) => {
    setLoading(true);
    try {
      const res = await API.get(
        `/api/invite_code/?p=${page}&page_size=${currentPageSize}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        setInviteCodes(data.items || []);
        setActivePage(data.page || 1);
        setInviteCodeCount(data.total || 0);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  const searchInviteCodes = async (page = 1, currentPageSize = pageSize) => {
    const { searchKeyword } = getFormValues();
    if (!searchKeyword) {
      await loadInviteCodes(page, currentPageSize);
      return;
    }
    setLoading(true);
    try {
      const res = await API.get(
        `/api/invite_code/search?keyword=${encodeURIComponent(searchKeyword)}&p=${page}&page_size=${currentPageSize}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        setInviteCodes(data.items || []);
        setActivePage(data.page || 1);
        setInviteCodeCount(data.total || 0);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
    setLoading(false);
  };

  const refresh = async (page = activePage) => {
    const { searchKeyword } = getFormValues();
    if (searchKeyword) {
      await searchInviteCodes();
    } else {
      await loadInviteCodes(page, pageSize);
    }
  };

  const handlePageChange = (page) => {
    setActivePage(page);
    const { searchKeyword } = getFormValues();
    if (searchKeyword) {
      searchInviteCodes(page, pageSize);
    } else {
      loadInviteCodes(page, pageSize);
    }
  };

  const handlePageSizeChange = (size) => {
    setPageSize(size);
    setActivePage(1);
    const { searchKeyword } = getFormValues();
    if (searchKeyword) {
      searchInviteCodes(1, size);
    } else {
      loadInviteCodes(1, size);
    }
  };

  const updateInviteCodeStatus = async (record, status) => {
    try {
      const res = await API.put('/api/invite_code/', {
        id: record.id,
        owner_user_id: record.owner_user_id,
        target_group: record.target_group,
        reward_quota_per_use: record.reward_quota_per_use,
        reward_total_uses: record.reward_total_uses,
        status,
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('操作成功完成！'));
        await refresh();
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
  };

  const deleteInviteCode = async (record) => {
    try {
      const res = await API.delete(`/api/invite_code/${record.id}`);
      const { success, message } = res.data;
      if (success) {
        showSuccess(t('操作成功完成！'));
        await refresh(activePage);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    }
  };

  useEffect(() => {
    loadInviteCodes(1, pageSize);
  }, []);

  return {
    inviteCodes,
    loading,
    activePage,
    pageSize,
    inviteCodeCount,
    showEdit,
    editingInviteCode,
    setEditingInviteCode,
    setShowEdit,
    refresh,
    formInitValues,
    setFormApi,
    searchInviteCodes,
    handlePageChange,
    handlePageSizeChange,
    updateInviteCodeStatus,
    deleteInviteCode,
    t,
  };
};
