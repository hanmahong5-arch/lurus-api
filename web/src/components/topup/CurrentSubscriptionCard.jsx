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
import { useTranslation } from 'react-i18next';
import { Card, Progress, Tag, Button, Typography } from '@douyinfe/semi-ui';
import { IconRefresh } from '@douyinfe/semi-icons';
import { renderQuota } from '../../helpers';

const { Text } = Typography;

const CurrentSubscriptionCard = ({
  subscription,
  quotaInfo,
  daysRemaining,
  hasActive,
  onCancelRenewal,
  onRefresh,
}) => {
  const { t } = useTranslation();

  // Calculate daily quota usage percentage
  const getDailyUsagePercent = () => {
    if (!quotaInfo?.daily_quota || quotaInfo.daily_quota <= 0) return 0;
    const used = quotaInfo.daily_used || 0;
    const total = quotaInfo.daily_quota;
    return Math.min(100, (used / total) * 100);
  };

  // Get status color
  const getStatusColor = () => {
    if (!hasActive) return 'grey';
    if (daysRemaining <= 3) return 'orange';
    return 'green';
  };

  // Get status text
  const getStatusText = () => {
    if (!hasActive) return t('未订阅');
    if (daysRemaining <= 0) return t('即将过期');
    if (daysRemaining <= 3) return t('即将到期');
    return t('生效中');
  };

  if (!hasActive) {
    return (
      <Card
        className="bg-gradient-to-r from-gray-50 to-gray-100 dark:from-gray-800 dark:to-gray-900"
        bodyStyle={{ padding: '24px' }}
      >
        <div className="flex items-center justify-between">
          <div>
            <div className="flex items-center gap-2">
              <Text strong className="text-lg">{t('当前订阅')}</Text>
              <Tag color="grey" size="small">{t('未订阅')}</Tag>
            </div>
            <Text type="secondary" className="mt-2 block">
              {t('订阅套餐可享受每日固定额度和专属分组优先调用')}
            </Text>
          </div>
          <Button
            icon={<IconRefresh />}
            theme="borderless"
            onClick={onRefresh}
          />
        </div>
      </Card>
    );
  }

  return (
    <Card
      className="bg-gradient-to-r from-blue-50 to-indigo-50 dark:from-blue-900/20 dark:to-indigo-900/20 border-blue-200 dark:border-blue-800"
      bodyStyle={{ padding: '24px' }}
    >
      <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-4">
        {/* Left: Subscription Info */}
        <div className="flex-1">
          <div className="flex items-center gap-2 mb-3">
            <Text strong className="text-lg">{t('当前订阅')}</Text>
            <Tag color={getStatusColor()} size="small">
              {getStatusText()}
            </Tag>
            <Button
              icon={<IconRefresh />}
              theme="borderless"
              size="small"
              onClick={onRefresh}
            />
          </div>

          <div className="space-y-2">
            <div className="flex items-center gap-4">
              <span className="text-gray-500">{t('套餐名称')}:</span>
              <span className="font-medium">{subscription?.plan_name}</span>
            </div>

            <div className="flex items-center gap-4">
              <span className="text-gray-500">{t('到期时间')}:</span>
              <span className="font-medium">
                {subscription?.expires_at
                  ? new Date(subscription.expires_at).toLocaleDateString()
                  : '-'}
              </span>
              {daysRemaining > 0 && (
                <Tag color="blue" size="small">
                  {t('剩余')} {daysRemaining} {t('天')}
                </Tag>
              )}
            </div>

            {subscription?.base_group && (
              <div className="flex items-center gap-4">
                <span className="text-gray-500">{t('专属分组')}:</span>
                <Tag color="cyan" size="small">{subscription.base_group}</Tag>
                {subscription?.fallback_group && subscription.fallback_group !== subscription.base_group && (
                  <span className="text-xs text-gray-400">
                    ({t('超额降级')}: {subscription.fallback_group})
                  </span>
                )}
              </div>
            )}
          </div>
        </div>

        {/* Right: Daily Quota Usage */}
        {quotaInfo?.daily_quota > 0 && (
          <div className="md:w-64 bg-white/50 dark:bg-gray-800/50 rounded-lg p-4">
            <div className="flex items-center justify-between mb-2">
              <Text type="secondary" size="small">{t('今日额度')}</Text>
              <Text size="small">
                {renderQuota(quotaInfo.daily_used || 0)} / {renderQuota(quotaInfo.daily_quota)}
              </Text>
            </div>
            <Progress
              percent={getDailyUsagePercent()}
              showInfo={false}
              stroke={getDailyUsagePercent() > 80 ? '#f97316' : '#3b82f6'}
              style={{ height: 8 }}
            />
            <div className="flex justify-between mt-2 text-xs text-gray-500">
              <span>{t('已用')} {getDailyUsagePercent().toFixed(0)}%</span>
              <span>{t('每日')} 00:00 {t('重置')}</span>
            </div>
          </div>
        )}
      </div>

      {/* Actions */}
      {subscription?.auto_renew && (
        <div className="mt-4 pt-4 border-t border-blue-200 dark:border-blue-800">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Tag color="green" size="small">{t('自动续费已开启')}</Tag>
              <Text type="secondary" size="small">
                {t('到期后将自动续费')}
              </Text>
            </div>
            <Button
              type="tertiary"
              size="small"
              onClick={onCancelRenewal}
            >
              {t('取消自动续费')}
            </Button>
          </div>
        </div>
      )}
    </Card>
  );
};

export default CurrentSubscriptionCard;
