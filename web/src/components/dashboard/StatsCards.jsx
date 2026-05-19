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
import { Card, Avatar, Skeleton, Progress } from '@douyinfe/semi-ui';
import { VChart } from '@visactor/react-vchart';
import { useTranslation } from 'react-i18next';
import { renderQuota } from '../../helpers/render';

/**
 * Determine progress bar color based on usage percentage.
 * <=50% green, 50-80% yellow, >80% red.
 */
const getUsageColor = (percent) => {
  if (percent > 80) return 'var(--semi-color-danger)';
  if (percent > 50) return 'var(--semi-color-warning)';
  return 'var(--semi-color-success)';
};

const StatsCards = ({
  groupedStatsData,
  loading,
  getTrendSpec,
  CARD_PROPS,
  CHART_CONFIG,
  usageGauge,
}) => {
  const { t } = useTranslation();

  return (
    <div className='mb-4' aria-live='polite'>
      <div className='grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4'>
        {groupedStatsData.map((group, idx) => (
          <Card
            key={idx}
            {...CARD_PROPS}
            className={`${group.color} border-0 !rounded-2xl w-full`}
            title={group.title}
          >
            <div className='space-y-4'>
              {group.items.map((item, itemIdx) => (
                <div
                  key={itemIdx}
                  className='flex items-center justify-between cursor-pointer'
                  onClick={item.onClick}
                >
                  <div className='flex items-center'>
                    <Avatar
                      className='mr-3'
                      size='small'
                      color={item.avatarColor}
                    >
                      {item.icon}
                    </Avatar>
                    <div>
                      <div className='text-xs text-gray-500'>{item.title}</div>
                      <div className='text-lg font-semibold'>
                        <Skeleton
                          loading={loading}
                          active
                          placeholder={
                            <Skeleton.Paragraph
                              active
                              rows={1}
                              style={{
                                width: '65px',
                                height: '24px',
                                marginTop: '4px',
                              }}
                            />
                          }
                        >
                          {item.value}
                        </Skeleton>
                      </div>
                    </div>
                  </div>
                  {(loading ||
                    (item.trendData && item.trendData.length > 0)) && (
                    <div className='w-24 h-10'>
                      <VChart
                        spec={getTrendSpec(item.trendData, item.trendColor)}
                        option={CHART_CONFIG}
                      />
                    </div>
                  )}
                </div>
              ))}

              {/* Usage progress bar — only in the first card (account data) */}
              {idx === 0 && (
                <div className='pt-2 border-t border-gray-200/50'>
                  <Skeleton
                    loading={loading}
                    active
                    placeholder={
                      <Skeleton.Paragraph
                        active
                        rows={2}
                        style={{
                          width: '100%',
                          height: '12px',
                          marginTop: '4px',
                        }}
                      />
                    }
                  >
                    {usageGauge && usageGauge.totalQuota > 0 ? (
                      <>
                        <div className='flex justify-between items-center mb-1'>
                          <span className='text-xs text-gray-500'>
                            {t('额度已使用')} {usageGauge.usagePercent}%
                          </span>
                          <span
                            className='text-xs font-mono font-semibold'
                            style={{
                              color: getUsageColor(usageGauge.usagePercent),
                            }}
                          >
                            {renderQuota(usageGauge.usedQuota)} /{' '}
                            {renderQuota(usageGauge.totalQuota)}
                          </span>
                        </div>
                        <Progress
                          percent={usageGauge.usagePercent}
                          showInfo={false}
                          stroke={getUsageColor(usageGauge.usagePercent)}
                          size='small'
                          style={{ height: 6 }}
                        />
                        <div className='text-xs text-gray-400 mt-1'>
                          {t('本月已使用')} {renderQuota(usageGauge.usedQuota)}{' '}
                          / {renderQuota(usageGauge.totalQuota)}
                        </div>
                      </>
                    ) : (
                      <div className='text-xs text-gray-400 text-center py-1'>
                        {t('暂无用量数据')}
                      </div>
                    )}
                  </Skeleton>
                </div>
              )}
            </div>
          </Card>
        ))}
      </div>
    </div>
  );
};

export default StatsCards;
