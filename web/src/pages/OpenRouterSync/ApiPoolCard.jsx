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
import React, { useEffect, useState, useCallback } from 'react';
import { Card, Tag, Table, Typography, Space, Button } from '@douyinfe/semi-ui';
import { API } from '../../helpers/api';

const { Text, Title } = Typography;

const STATUS_TAG = {
  enabled: { color: 'green', label: '启用' },
  cooling: { color: 'orange', label: '冷却中' },
  permanent_disabled: { color: 'red', label: '永久禁用' },
};

const CHANNEL_STATUS_TAG = {
  enabled: { color: 'green', label: '启用' },
  auto_disabled: { color: 'orange', label: '自动禁用（待恢复）' },
  manually_disabled: { color: 'red', label: '手动禁用' },
};

function formatRemaining(seconds) {
  if (!seconds || seconds <= 0) return '即将恢复';
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  return `${h}h ${m}m`;
}

const ApiPoolCard = () => {
  const [data, setData] = useState([]);
  const [loading, setLoading] = useState(false);

  const reload = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/openrouter-sync/api-pool');
      if (res.data?.success) setData(res.data.data || []);
    } catch (e) {
      // soft-fail; the card just shows empty
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    reload();
    const id = setInterval(reload, 10000);
    return () => clearInterval(id);
  }, [reload]);

  if (!data || data.length === 0) {
    return (
      <Card style={{ marginBottom: 12 }}>
        <Title heading={5}>API Pool 状态</Title>
        <Text type='tertiary'>
          当前没有启用 multi-key 的 OpenRouter 渠道。在渠道管理中把渠道设为多
          key 模式后，本卡片会展示池内每个 key 的状态。
        </Text>
      </Card>
    );
  }

  const keyColumns = [
    { title: '#', dataIndex: 'index', width: 50 },
    {
      title: 'Key',
      dataIndex: 'key_prefix',
      render: (v) => <Text code>{v}</Text>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      render: (s) => {
        const t = STATUS_TAG[s] || { color: 'grey', label: s };
        return <Tag color={t.color}>{t.label}</Tag>;
      },
    },
    {
      title: '冷却剩余',
      dataIndex: 'cooldown_seconds_remaining',
      render: (s, row) => (row.status === 'cooling' ? formatRemaining(s) : '—'),
    },
    {
      title: '原因',
      dataIndex: 'disable_reason',
      render: (r) =>
        r ? (
          <Text
            size='small'
            type='tertiary'
            ellipsis={{ showTooltip: true }}
            style={{ maxWidth: 240 }}
          >
            {r}
          </Text>
        ) : (
          '—'
        ),
    },
  ];

  return (
    <Card style={{ marginBottom: 12 }}>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          marginBottom: 8,
        }}
      >
        <Title heading={5} style={{ margin: 0 }}>
          API Pool 状态
        </Title>
        <Button size='small' onClick={reload} loading={loading}>
          刷新
        </Button>
      </div>
      <Text type='tertiary' size='small'>
        每 10 秒自动刷新。冷却中的 key 由 reaper 在到期后自动恢复。
      </Text>

      {data.map((ch) => {
        const ct = CHANNEL_STATUS_TAG[ch.status] || {
          color: 'grey',
          label: ch.status,
        };
        return (
          <Card
            key={ch.channel_id}
            style={{ marginTop: 8 }}
            bodyStyle={{ padding: 12 }}
            title={
              <Space>
                <Text strong>{ch.channel_name || `#${ch.channel_id}`}</Text>
                <Text type='tertiary' size='small'>
                  (渠道 #{ch.channel_id})
                </Text>
                <Tag color={ct.color}>{ct.label}</Tag>
              </Space>
            }
            headerLine={false}
          >
            <Space style={{ marginBottom: 8 }}>
              <Tag color='green'>启用 {ch.enabled_count}</Tag>
              <Tag color='orange'>冷却 {ch.cooling_count}</Tag>
              <Tag color='red'>永久禁用 {ch.permanent_disabled_count}</Tag>
              <Text type='tertiary' size='small'>
                共 {ch.key_count}
              </Text>
            </Space>
            <Table
              columns={keyColumns}
              dataSource={ch.keys}
              rowKey='index'
              pagination={false}
              size='small'
            />
          </Card>
        );
      })}
    </Card>
  );
};

export default ApiPoolCard;
