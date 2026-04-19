import React, { useEffect, useState } from 'react';
import { Empty, Modal, Table, Toast } from '@douyinfe/semi-ui';
import { API } from '../../../helpers';

const InviteDetailsModal = ({
  visible,
  onCancel,
  t,
  title,
  endpoint,
  columns,
  emptyText,
}) => {
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);

  const loadItems = async (currentPage, currentPageSize) => {
    setLoading(true);
    try {
      const res = await API.get(
        `${endpoint}?p=${currentPage}&page_size=${currentPageSize}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        setItems(data?.items || []);
        setTotal(data?.total || 0);
      } else {
        Toast.error({ content: message || t('加载失败') });
      }
    } catch (error) {
      Toast.error({ content: t('加载失败') });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!visible) {
      setPage(1);
      setPageSize(10);
      setItems([]);
      setTotal(0);
      return;
    }
    loadItems(page, pageSize);
  }, [visible, page, pageSize, endpoint]);

  return (
    <Modal
      title={title}
      visible={visible}
      onCancel={onCancel}
      footer={null}
      width={960}
      centered
    >
      <Table
        rowKey={(record) => record.id || record.user_id || record.code}
        columns={columns}
        dataSource={items}
        loading={loading}
        pagination={{
          currentPage: page,
          pageSize,
          total,
          pageSizeOpts: [10, 20, 50],
          showSizeChanger: true,
          onPageChange: setPage,
          onPageSizeChange: (size) => {
            setPageSize(size);
            setPage(1);
          },
        }}
        scroll={{ x: 'max-content' }}
        empty={<Empty description={emptyText} style={{ padding: 24 }} />}
      />
    </Modal>
  );
};

export default InviteDetailsModal;
