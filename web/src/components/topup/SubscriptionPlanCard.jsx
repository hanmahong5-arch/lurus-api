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
import { Card, Button, Tag, Typography } from '@douyinfe/semi-ui';
import { IconTick } from '@douyinfe/semi-icons';
import { renderQuota } from '../../helpers';

const { Text } = Typography;

const SubscriptionPlanCard = ({ plan, isCurrent, onSelect }) => {
  const { t } = useTranslation();

  // Format currency symbol
  const getCurrencySymbol = (currency) => {
    const symbols = {
      CNY: '¥',
      USD: '$',
      EUR: '€',
    };
    return symbols[currency] || currency;
  };

  // Calculate daily equivalent price
  const getDailyPrice = () => {
    if (!plan.price || !plan.days) return 0;
    return (plan.price / plan.days).toFixed(2);
  };

  // Get plan features list
  const getFeatures = () => {
    const features = [];

    if (plan.daily_quota > 0) {
      features.push({
        icon: 'quota',
        text: t('每日额度') + ': ' + renderQuota(plan.daily_quota),
      });
    }

    if (plan.total_quota > 0) {
      features.push({
        icon: 'total',
        text: t('总额度') + ': ' + renderQuota(plan.total_quota),
      });
    }

    if (plan.base_group) {
      features.push({
        icon: 'group',
        text: t('专属分组') + ': ' + plan.base_group,
      });
    }

    features.push({
      icon: 'duration',
      text: t('有效期') + ': ' + plan.days + ' ' + t('天'),
    });

    return features;
  };

  // Determine card style based on plan type
  const isRecommended = plan.code === 'monthly' || plan.recommended;
  const isPremium = plan.code === 'yearly' || plan.premium;

  return (
    <Card
      className={`relative transition-all hover:shadow-lg ${
        isCurrent
          ? 'border-2 border-green-500 dark:border-green-400'
          : isRecommended
          ? 'border-2 border-primary'
          : ''
      }`}
      bodyStyle={{ padding: '24px' }}
    >
      {/* Badge */}
      {isCurrent && (
        <div className="absolute -top-3 left-1/2 -translate-x-1/2">
          <Tag color="green" size="small">
            {t('当前套餐')}
          </Tag>
        </div>
      )}
      {!isCurrent && isRecommended && (
        <div className="absolute -top-3 left-1/2 -translate-x-1/2">
          <Tag color="blue" size="small">
            {t('推荐')}
          </Tag>
        </div>
      )}
      {!isCurrent && isPremium && (
        <div className="absolute -top-3 left-1/2 -translate-x-1/2">
          <Tag color="orange" size="small">
            {t('超值')}
          </Tag>
        </div>
      )}

      {/* Plan Name */}
      <div className="text-center mb-4 pt-2">
        <h3 className="text-xl font-bold">{plan.name}</h3>
        {plan.description && (
          <Text type="secondary" size="small" className="mt-1 block">
            {plan.description}
          </Text>
        )}
      </div>

      {/* Price */}
      <div className="text-center mb-6">
        <div className="flex items-baseline justify-center gap-1">
          <span className="text-lg text-gray-500">
            {getCurrencySymbol(plan.currency)}
          </span>
          <span className="text-4xl font-bold text-primary">{plan.price}</span>
        </div>
        <Text type="tertiary" size="small" className="mt-1 block">
          {getCurrencySymbol(plan.currency)}
          {getDailyPrice()}/{t('天')}
        </Text>
      </div>

      {/* Features */}
      <div className="space-y-3 mb-6">
        {getFeatures().map((feature, index) => (
          <div key={index} className="flex items-center gap-2">
            <IconTick className="text-green-500 flex-shrink-0" size="small" />
            <Text size="small">{feature.text}</Text>
          </div>
        ))}
      </div>

      {/* Action Button */}
      <Button
        theme={isCurrent ? 'light' : 'solid'}
        type={isCurrent ? 'tertiary' : 'primary'}
        block
        onClick={onSelect}
        disabled={isCurrent}
      >
        {isCurrent ? t('当前套餐') : t('立即订阅')}
      </Button>
    </Card>
  );
};

export default SubscriptionPlanCard;
