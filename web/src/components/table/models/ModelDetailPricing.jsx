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
import { Card, Tag, Typography } from '@douyinfe/semi-ui';
import { AlertTriangle, DollarSign, GitBranch } from 'lucide-react';

const { Text, Title } = Typography;

const PRICING_SOURCE_CONFIG = {
  explicit: { color: 'green', iconColor: '#16a34a' },
  family_fallback: { color: 'blue', iconColor: '#2563eb' },
  none: { color: 'red', iconColor: '#dc2626' },
};

const ModelDetailPricing = ({ model, pricingMap, t }) => {
  if (!model) return null;

  const info = pricingMap?.[model.model_name];
  const source = info?.source || 'none';
  const config = PRICING_SOURCE_CONFIG[source] || PRICING_SOURCE_CONFIG.none;

  return (
    <Card bodyStyle={{ padding: '16px' }} className='rounded-xl'>
      <div className='flex items-center gap-2 mb-4'>
        <Title heading={6} className='!mb-0'>
          {t('定价信息')}
        </Title>
        <Tag size='small' shape='circle' color={config.color}>
          {source === 'explicit'
            ? t('显式配置')
            : source === 'family_fallback'
              ? t('自动定价')
              : t('未定价')}
        </Tag>
      </div>

      {source === 'explicit' && (
        <div className='space-y-3'>
          <div className='flex items-center gap-3 p-3 rounded-lg bg-[var(--semi-color-fill-0)]'>
            <DollarSign size={20} color={config.iconColor} />
            <div>
              <div className='text-xs text-[var(--semi-color-text-2)]'>
                {t('倍率')}
              </div>
              <div className='text-lg font-semibold'>
                {info.ratio?.toFixed(3)}
              </div>
            </div>
          </div>
        </div>
      )}

      {source === 'family_fallback' && (
        <div className='space-y-3'>
          <div className='flex items-center gap-3 p-3 rounded-lg bg-[var(--semi-color-fill-0)]'>
            <GitBranch size={20} color={config.iconColor} />
            <div className='flex-1'>
              <div className='text-xs text-[var(--semi-color-text-2)]'>
                {t('模型族')}
              </div>
              <div className='text-sm font-medium'>{info.family}</div>
            </div>
          </div>

          <div className='grid grid-cols-2 gap-3'>
            <div className='p-3 rounded-lg bg-[var(--semi-color-fill-0)]'>
              <div className='text-xs text-[var(--semi-color-text-2)]'>
                {t('基础倍率')}
              </div>
              <div className='text-lg font-semibold'>
                {info.markup
                  ? (info.ratio / info.markup).toFixed(3)
                  : info.ratio?.toFixed(3)}
              </div>
            </div>
            <div className='p-3 rounded-lg bg-[var(--semi-color-fill-0)]'>
              <div className='text-xs text-[var(--semi-color-text-2)]'>
                {t('加价倍数')}
              </div>
              <div className='text-lg font-semibold'>
                {info.markup ? `×${info.markup}` : '-'}
              </div>
            </div>
          </div>

          <div className='flex items-center gap-3 p-3 rounded-lg bg-[var(--semi-color-fill-0)]'>
            <DollarSign size={20} color={config.iconColor} />
            <div>
              <div className='text-xs text-[var(--semi-color-text-2)]'>
                {t('最终倍率')}
              </div>
              <div className='text-lg font-semibold'>
                ≈ {info.ratio?.toFixed(3)}
              </div>
            </div>
          </div>
        </div>
      )}

      {source === 'none' && (
        <div className='flex items-start gap-3 p-4 rounded-lg bg-[var(--semi-color-warning-light-default)]'>
          <AlertTriangle
            size={20}
            color={config.iconColor}
            className='flex-shrink-0 mt-0.5'
          />
          <div>
            <div className='text-sm font-medium mb-1'>
              {t('未配置倍率，调用将失败')}
            </div>
            <Text type='secondary' size='small'>
              {t(
                '该模型尚未配置定价信息，需要设置显式倍率或确保其所属模型族已配置基础倍率。',
              )}
            </Text>
          </div>
        </div>
      )}
    </Card>
  );
};

export default ModelDetailPricing;
