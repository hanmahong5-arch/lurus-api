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

import React, { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Table,
  Button,
  Card,
  Space,
  Tag,
  Input,
  Select,
  Modal,
  Form,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconSearch,
  IconPlus,
  IconRefresh,
} from '@douyinfe/semi-icons';
import { API, showError, showSuccess } from '../../helpers';

const Subscription = () => {
  const { t } = useTranslation();
  const [subscriptions, setSubscriptions] = useState([]);
  const [loading, setLoading] = useState(false);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [total, setTotal] = useState(0);
  const [statusFilter, setStatusFilter] = useState('');
  const [userIdFilter, setUserIdFilter] = useState('');
  const [showGrantModal, setShowGrantModal] = useState(false);
  const [grantForm, setGrantForm] = useState({
    user_id: '',
    plan_code: 'monthly',
    days: 0,
  });
  const [plans, setPlans] = useState([]);

  // Fetch subscriptions
  const fetchSubscriptions = useCallback(async () => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        page: page.toString(),
        page_size: pageSize.toString(),
      });
      if (statusFilter) params.append('status', statusFilter);
      if (userIdFilter) params.append('user_id', userIdFilter);

      const res = await API.get(`/api/subscription/admin/all?${params}`);
      if (res.data.success) {
        setSubscriptions(res.data.data.subscriptions || []);
        setTotal(res.data.data.total || 0);
      } else {
        showError(res.data.message);
      }
    } catch (e) {
      showError(e.message || 'Failed to fetch subscriptions');
    }
    setLoading(false);
  }, [page, pageSize, statusFilter, userIdFilter]);

  // Fetch plans
  const fetchPlans = async () => {
    try {
      const res = await API.get('/api/subscription/plans');
      if (res.data.success) {
        setPlans(res.data.data || []);
      }
    } catch (e) {
      // ignore
    }
  };

  useEffect(() => {
    fetchSubscriptions();
    fetchPlans();
  }, [fetchSubscriptions]);

  // Grant subscription
  const handleGrant = async () => {
    if (!grantForm.user_id) {
      showError(t('请输入用户ID'));
      return;
    }
    try {
      const res = await API.post('/api/subscription/admin/grant', {
        user_id: parseInt(grantForm.user_id),
        plan_code: grantForm.plan_code,
        days: grantForm.days || 0,
      });
      if (res.data.success) {
        showSuccess(t('订阅已赠送'));
        setShowGrantModal(false);
        setGrantForm({ user_id: '', plan_code: 'monthly', days: 0 });
        fetchSubscriptions();
      } else {
        showError(res.data.message);
      }
    } catch (e) {
      showError(e.message || 'Failed to grant subscription');
    }
  };

  // Activate subscription
  const handleActivate = async (id) => {
    try {
      const res = await API.post(`/api/subscription/admin/${id}/activate`);
      if (res.data.success) {
        showSuccess(t('订阅已激活'));
        fetchSubscriptions();
      } else {
        showError(res.data.message);
      }
    } catch (e) {
      showError(e.message || 'Failed to activate subscription');
    }
  };

  // Expire subscription
  const handleExpire = async (id) => {
    Modal.confirm({
      title: t('确认过期'),
      content: t('确定要将此订阅标记为过期吗？用户将失去订阅权益。'),
      onOk: async () => {
        try {
          const res = await API.post(`/api/subscription/admin/${id}/expire`);
          if (res.data.success) {
            showSuccess(t('订阅已过期'));
            fetchSubscriptions();
          } else {
            showError(res.data.message);
          }
        } catch (e) {
          showError(e.message || 'Failed to expire subscription');
        }
      },
    });
  };

  const getStatusColor = (status) => {
    const colorMap = {
      active: 'green',
      pending: 'yellow',
      expired: 'grey',
      cancelled: 'red',
    };
    return colorMap[status] || 'grey';
  };

  const getStatusText = (status) => {
    const textMap = {
      active: t('激活'),
      pending: t('待支付'),
      expired: t('已过期'),
      cancelled: t('已取消'),
    };
    return textMap[status] || status;
  };

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: t('用户ID'), dataIndex: 'user_id', width: 80 },
    { title: t('计划'), dataIndex: 'plan_name', width: 120 },
    {
      title: t('状态'),
      dataIndex: 'status',
      width: 100,
      render: (status) => (
        <Tag color={getStatusColor(status)}>{getStatusText(status)}</Tag>
      ),
    },
    {
      title: t('开始时间'),
      dataIndex: 'started_at',
      width: 160,
      render: (time) => (time ? new Date(time).toLocaleString() : '-'),
    },
    {
      title: t('到期时间'),
      dataIndex: 'expires_at',
      width: 160,
      render: (time) => (time ? new Date(time).toLocaleString() : '-'),
    },
    { title: t('支付方式'), dataIndex: 'payment_method', width: 100 },
    {
      title: t('金额'),
      dataIndex: 'amount',
      width: 100,
      render: (amount, record) =>
        amount > 0 ? `${record.currency} ${amount}` : t('免费'),
    },
    {
      title: t('操作'),
      width: 150,
      render: (_, record) => (
        <Space>
          {record.status === 'pending' && (
            <Button size='small' onClick={() => handleActivate(record.id)}>
              {t('激活')}
            </Button>
          )}
          {record.status === 'active' && (
            <Button
              size='small'
              type='danger'
              onClick={() => handleExpire(record.id)}
            >
              {t('过期')}
            </Button>
          )}
        </Space>
      ),
    },
  ];

  return (
    <div className='px-2'>
      <Card>
        <Typography.Title heading={4}>{t('订阅管理')}</Typography.Title>

        {/* Filters */}
        <Space className='mb-4'>
          <Input
            prefix={<IconSearch />}
            placeholder={t('用户ID')}
            value={userIdFilter}
            onChange={(v) => setUserIdFilter(v)}
            style={{ width: 150 }}
          />
          <Select
            placeholder={t('状态')}
            value={statusFilter}
            onChange={(v) => setStatusFilter(v)}
            style={{ width: 120 }}
            optionList={[
              { value: '', label: t('全部') },
              { value: 'active', label: t('激活') },
              { value: 'pending', label: t('待支付') },
              { value: 'expired', label: t('已过期') },
              { value: 'cancelled', label: t('已取消') },
            ]}
          />
          <Button icon={<IconRefresh />} onClick={fetchSubscriptions}>
            {t('刷新')}
          </Button>
          <Button
            icon={<IconPlus />}
            type='primary'
            onClick={() => setShowGrantModal(true)}
          >
            {t('赠送订阅')}
          </Button>
        </Space>

        {/* Table */}
        <Table
          columns={columns}
          dataSource={subscriptions}
          loading={loading}
          pagination={{
            currentPage: page,
            pageSize: pageSize,
            total: total,
            onPageChange: setPage,
            onPageSizeChange: setPageSize,
            showSizeChanger: true,
          }}
          rowKey='id'
        />
      </Card>

      {/* Grant Modal */}
      <Modal
        title={t('赠送订阅')}
        visible={showGrantModal}
        onOk={handleGrant}
        onCancel={() => setShowGrantModal(false)}
      >
        <Form>
          <Form.Input
            field='user_id'
            label={t('用户ID')}
            value={grantForm.user_id}
            onChange={(v) => setGrantForm({ ...grantForm, user_id: v })}
            placeholder={t('输入用户ID')}
          />
          <Form.Select
            field='plan_code'
            label={t('订阅计划')}
            value={grantForm.plan_code}
            onChange={(v) => setGrantForm({ ...grantForm, plan_code: v })}
            optionList={plans.map((p) => ({ value: p.code, label: p.name }))}
          />
          <Form.InputNumber
            field='days'
            label={t('自定义天数')}
            value={grantForm.days}
            onChange={(v) => setGrantForm({ ...grantForm, days: v })}
            placeholder={t('留空使用计划默认天数')}
          />
        </Form>
      </Modal>
    </div>
  );
};

export default Subscription;
