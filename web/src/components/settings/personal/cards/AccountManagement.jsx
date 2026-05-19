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
import {
  Button,
  Card,
  Input,
  Space,
  Typography,
  Avatar,
} from '@douyinfe/semi-ui';
import { IconKey } from '@douyinfe/semi-icons';
import { UserPlus, ShieldCheck } from 'lucide-react';

const AccountManagement = ({
  t,
  userState,
  status,
  systemToken,
  generateAccessToken,
  handleSystemTokenClick,
}) => {
  return (
    <Card className='!rounded-2xl'>
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='teal' className='mr-3 shadow-md'>
          <UserPlus size={16} />
        </Avatar>
        <div>
          <Typography.Text className='text-lg font-medium'>
            {t('账户管理')}
          </Typography.Text>
          <div className='text-xs text-gray-600'>{t('账户信息和安全设置')}</div>
        </div>
      </div>

      <div className='py-4'>
        <Space vertical className='w-full'>
          {/* Account info */}
          <Card className='!rounded-xl w-full'>
            <div className='grid grid-cols-1 lg:grid-cols-2 gap-4'>
              <div className='flex items-center gap-3'>
                <div className='font-medium text-gray-900'>{t('用户名')}</div>
                <div className='text-sm text-gray-500'>
                  {userState.user?.username || '-'}
                </div>
              </div>
              <div className='flex items-center gap-3'>
                <div className='font-medium text-gray-900'>{t('邮箱')}</div>
                <div className='text-sm text-gray-500'>
                  {userState.user?.email || t('未设置')}
                </div>
              </div>
            </div>
            <div className='mt-3 text-xs text-gray-400'>
              {t('账户信息由身份认证服务管理，如需修改请联系管理员')}
            </div>
          </Card>

          {/* System access token */}
          <Card className='!rounded-xl w-full'>
            <div className='flex flex-col sm:flex-row items-start sm:justify-between gap-4'>
              <div className='flex items-start w-full sm:w-auto'>
                <div className='w-12 h-12 rounded-full bg-slate-100 flex items-center justify-center mr-4 flex-shrink-0'>
                  <IconKey size='large' className='text-slate-600' />
                </div>
                <div className='flex-1'>
                  <Typography.Title heading={6} className='mb-1'>
                    {t('系统访问令牌')}
                  </Typography.Title>
                  <Typography.Text type='tertiary' className='text-sm'>
                    {t('用于API调用的身份验证令牌，请妥善保管')}
                  </Typography.Text>
                  {systemToken && (
                    <div className='mt-3'>
                      <Input
                        readonly
                        value={systemToken}
                        onClick={handleSystemTokenClick}
                        size='large'
                        prefix={<IconKey />}
                      />
                    </div>
                  )}
                </div>
              </div>
              <Button
                type='primary'
                theme='solid'
                onClick={generateAccessToken}
                className='!bg-slate-600 hover:!bg-slate-700 w-full sm:w-auto'
                icon={<IconKey />}
              >
                {systemToken ? t('重新生成') : t('生成令牌')}
              </Button>
            </div>
          </Card>
        </Space>
      </div>
    </Card>
  );
};

export default AccountManagement;
