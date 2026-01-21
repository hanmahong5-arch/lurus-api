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

import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, Collapsible, Typography } from '@douyinfe/semi-ui';
import { IconChevronDown, IconChevronUp } from '@douyinfe/semi-icons';

const { Text, Title } = Typography;

const BillingExplanation = () => {
  const { t } = useTranslation();
  const [expanded, setExpanded] = useState(false);

  return (
    <Card className="mt-6">
      <div
        className="flex items-center justify-between cursor-pointer"
        onClick={() => setExpanded(!expanded)}
      >
        <Title heading={5} className="!mb-0">
          {t('计费说明')}
        </Title>
        {expanded ? (
          <IconChevronUp className="text-gray-400" />
        ) : (
          <IconChevronDown className="text-gray-400" />
        )}
      </div>

      <Collapsible isOpen={expanded}>
        <div className="mt-4 grid grid-cols-1 md:grid-cols-2 gap-6">
          {/* Subscription Billing */}
          <div className="bg-blue-50 dark:bg-blue-900/20 rounded-lg p-4">
            <div className="flex items-center gap-2 mb-3">
              <div className="w-3 h-3 rounded-full bg-blue-500" />
              <Text strong>{t('订阅套餐')}</Text>
            </div>
            <ul className="space-y-2 text-sm text-gray-600 dark:text-gray-300">
              <li className="flex items-start gap-2">
                <span className="text-blue-500">•</span>
                <span>{t('每日重置固定额度，未用完不累计')}</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-blue-500">•</span>
                <span>{t('专属分组优先调用，响应更快')}</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-blue-500">•</span>
                <span>{t('超出每日限额后自动降级到普通分组')}</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-blue-500">•</span>
                <span>{t('适合高频、稳定使用场景')}</span>
              </li>
            </ul>
          </div>

          {/* Pay-as-you-go Billing */}
          <div className="bg-green-50 dark:bg-green-900/20 rounded-lg p-4">
            <div className="flex items-center gap-2 mb-3">
              <div className="w-3 h-3 rounded-full bg-green-500" />
              <Text strong>{t('余额充值')}</Text>
            </div>
            <ul className="space-y-2 text-sm text-gray-600 dark:text-gray-300">
              <li className="flex items-start gap-2">
                <span className="text-green-500">•</span>
                <span>{t('按实际使用量扣费，用多少付多少')}</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-green-500">•</span>
                <span>{t('余额永久有效，不清零、不过期')}</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-green-500">•</span>
                <span>{t('支持多种充值方式和优惠折扣')}</span>
              </li>
              <li className="flex items-start gap-2">
                <span className="text-green-500">•</span>
                <span>{t('适合低频或不规则使用场景')}</span>
              </li>
            </ul>
          </div>
        </div>

        {/* Additional Notes */}
        <div className="mt-4 p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
          <Text strong className="block mb-2">{t('使用须知')}</Text>
          <ul className="space-y-1 text-sm text-gray-500">
            <li>• {t('订阅套餐和余额可同时使用，订阅额度优先消耗')}</li>
            <li>• {t('订阅到期后，如有余额可继续使用按量计费')}</li>
            <li>• {t('每日额度在北京时间 00:00 自动重置')}</li>
            <li>• {t('如有疑问，请联系客服获取帮助')}</li>
          </ul>
        </div>
      </Collapsible>
    </Card>
  );
};

export default BillingExplanation;
