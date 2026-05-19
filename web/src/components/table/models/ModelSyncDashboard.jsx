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
import React, { useMemo } from 'react';
import { Card, Button, Space, Tag, Typography } from '@douyinfe/semi-ui';
import { IconRefresh } from '@douyinfe/semi-icons';

const { Text, Title } = Typography;

const ModelSyncDashboard = ({
  modelCount,
  pricingMap,
  syncAllChannels,
  syncingChannels,
  syncUpstream,
  syncing,
  t,
}) => {
  const stats = useMemo(() => {
    const entries = Object.values(pricingMap || {});
    const explicit = entries.filter((e) => e.source === 'explicit').length;
    const fallback = entries.filter(
      (e) => e.source === 'family_fallback',
    ).length;
    const none = entries.filter((e) => e.source === 'none').length;
    const total = entries.length;
    const priced = explicit + fallback;
    const pct = total > 0 ? ((priced / total) * 100).toFixed(1) : '0';
    return { total, explicit, fallback, none, priced, pct };
  }, [pricingMap]);

  return (
    <Card style={{ marginBottom: 12 }} bodyStyle={{ padding: '12px 20px' }}>
      <div className='flex flex-col md:flex-row items-start md:items-center justify-between gap-3'>
        <div className='flex flex-wrap items-center gap-4'>
          <Title heading={6} style={{ margin: 0 }}>
            {t('模型同步状态')}
          </Title>
          <Space>
            <Text>
              {t('总模型')}: {modelCount || stats.total}
            </Text>
            <Tag color='green' size='small'>
              {t('显式定价')}: {stats.explicit}
            </Tag>
            <Tag color='blue' size='small'>
              {t('自动定价')}: {stats.fallback}
            </Tag>
            {stats.none > 0 && (
              <Tag color='red' size='small'>
                {t('未定价')}: {stats.none}
              </Tag>
            )}
            <Tag
              color={stats.pct === '100.0' ? 'green' : 'orange'}
              size='small'
            >
              {t('覆盖率')}: {stats.pct}%
            </Tag>
          </Space>
        </div>
        <Space>
          <Button
            icon={<IconRefresh />}
            loading={syncingChannels}
            onClick={syncAllChannels}
            size='small'
          >
            {t('同步渠道模型')}
          </Button>
          <Button
            icon={<IconRefresh />}
            loading={syncing}
            onClick={() => syncUpstream?.()}
            size='small'
          >
            {t('同步上游元数据')}
          </Button>
        </Space>
      </div>
    </Card>
  );
};

export default ModelSyncDashboard;
