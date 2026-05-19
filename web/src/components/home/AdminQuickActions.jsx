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
import React, { useState, useEffect } from 'react';
import { Card, Button, Typography, Tag, Space } from '@douyinfe/semi-ui';
import { IconPlus, IconKey, IconList, IconSetting } from '@douyinfe/semi-icons';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { API } from '../../helpers';

const { Text } = Typography;

const AdminQuickActions = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [stats, setStats] = useState(null);

  useEffect(() => {
    const fetchStats = async () => {
      try {
        const [chRes, tkRes, mdRes] = await Promise.all([
          API.get('/api/channel/?p=0&size=1'),
          API.get('/api/token/?p=0&size=1'),
          API.get('/api/models/?p=0&size=1'),
        ]);
        setStats({
          channels: chRes?.data?.total ?? 0,
          tokens: tkRes?.data?.total ?? 0,
          models: mdRes?.data?.total ?? 0,
        });
      } catch {
        // silently ignore
      }
    };
    fetchStats();
  }, []);

  return (
    <Card
      className='mb-6'
      bodyStyle={{ padding: '16px 20px' }}
      style={{
        borderRadius: '12px',
        border: '1px solid var(--semi-color-border)',
      }}
    >
      <div className='flex flex-col md:flex-row md:items-center md:justify-between gap-3'>
        <div className='flex flex-wrap gap-2'>
          <Button
            icon={<IconPlus />}
            theme='light'
            type='primary'
            size='small'
            onClick={() => navigate('/console/channel')}
          >
            {t('admin_action_add_channel')}
          </Button>
          <Button
            icon={<IconKey />}
            theme='light'
            type='primary'
            size='small'
            onClick={() => navigate('/console/token')}
          >
            {t('admin_action_create_token')}
          </Button>
          <Button
            icon={<IconList />}
            theme='light'
            type='tertiary'
            size='small'
            onClick={() => navigate('/console/log')}
          >
            {t('admin_action_view_logs')}
          </Button>
          <Button
            icon={<IconSetting />}
            theme='light'
            type='tertiary'
            size='small'
            onClick={() => navigate('/console/setting')}
          >
            {t('admin_action_settings')}
          </Button>
        </div>

        {stats && (
          <Space spacing={8}>
            <Tag color='green' shape='circle' size='small'>
              {stats.channels} {t('admin_stat_channels')}
            </Tag>
            <Tag color='blue' shape='circle' size='small'>
              {stats.models} {t('admin_stat_models')}
            </Tag>
            <Tag color='cyan' shape='circle' size='small'>
              {stats.tokens} {t('admin_stat_tokens')}
            </Tag>
          </Space>
        )}
      </div>
    </Card>
  );
};

export default AdminQuickActions;
