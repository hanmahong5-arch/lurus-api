import React from 'react';
import { Empty, Button, Typography } from '@douyinfe/semi-ui';
import {
  IllustrationNoAccess,
  IllustrationNoAccessDark,
} from '@douyinfe/semi-illustrations';
import { useTranslation } from 'react-i18next';

// Terminal page reached when zita-bootstrap returns 403 (user.Status !=
// UserStatusEnabled). The visitor holds a valid platform identity but
// their newhub account is suspended/disabled — sending them back through
// /login would loop because bootstrap rejects again on every retry.
// Surface the state explicitly with a contact path instead.
const AccountDisabled = () => {
  const { t } = useTranslation();
  return (
    <div className='flex justify-center items-center min-h-screen p-8 bg-gray-50'>
      <Empty
        image={<IllustrationNoAccess style={{ width: 220, height: 220 }} />}
        darkModeImage={
          <IllustrationNoAccessDark style={{ width: 220, height: 220 }} />
        }
        title={
          <Typography.Title heading={4}>
            {t('账户已被禁用')}
          </Typography.Title>
        }
        description={
          <div className='max-w-md text-center'>
            <Typography.Text>
              {t(
                '您的账户当前处于禁用状态，无法访问此服务。如需恢复访问，请联系系统管理员。',
              )}
            </Typography.Text>
          </div>
        }
      >
        <div className='mt-4 flex gap-3 justify-center'>
          <Button
            theme='solid'
            onClick={() => {
              window.location.href = 'mailto:support@lurus.cn';
            }}
          >
            {t('联系管理员')}
          </Button>
          <Button
            onClick={() => {
              window.location.href = '/api/v2/auth/zita-logout';
            }}
          >
            {t('切换账号')}
          </Button>
        </div>
      </Empty>
    </div>
  );
};

export default AccountDisabled;
