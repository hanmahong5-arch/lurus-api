/*
 * AilurusStatsCards - Ailurus Styled Statistics Cards
 *
 * A beautiful stats display with the Ailurus aesthetic:
 * - Glassmorphic card backgrounds
 * - Animated number counting
 * - Mini trend charts
 * - Staggered entrance animations
 */

import React from 'react';
import { motion } from 'framer-motion';
import { VChart } from '@visactor/react-vchart';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import clsx from 'clsx';
import { springConfig, staggerContainer, staggerItem } from '../ailurus-ui/motion';
import { AilurusButton } from '../ailurus-ui';

/**
 * Single stat item with animation
 */
const StatItem = ({
  item,
  loading,
  getTrendSpec,
  CHART_CONFIG,
  index,
  t,
  navigate,
}) => {
  // Variant colors for icons based on avatar color
  const avatarColorMap = {
    amber: 'bg-ailurus-rust-500/20 text-ailurus-rust-400',
    blue: 'bg-ailurus-teal-500/20 text-ailurus-teal-400',
    green: 'bg-green-500/20 text-green-400',
    purple: 'bg-ailurus-purple-500/20 text-ailurus-purple-400',
    cyan: 'bg-cyan-500/20 text-cyan-400',
    red: 'bg-red-500/20 text-red-400',
    orange: 'bg-orange-500/20 text-orange-400',
  };

  const iconClasses = avatarColorMap[item.avatarColor] || avatarColorMap.amber;

  return (
    <motion.div
      className={clsx(
        'group flex items-center justify-between p-3 rounded-xl',
        'bg-white/[0.02] hover:bg-white/[0.05]',
        'border border-transparent hover:border-white/10',
        'transition-all cursor-pointer'
      )}
      variants={staggerItem}
      onClick={item.onClick}
      whileHover={{ x: 4 }}
      transition={springConfig.snappy}
    >
      {/* Left: Icon and text */}
      <div className="flex items-center gap-3">
        {/* Animated icon container */}
        <motion.div
          className={clsx(
            'w-10 h-10 rounded-xl flex items-center justify-center',
            iconClasses
          )}
          whileHover={{ scale: 1.1, rotate: 5 }}
          transition={springConfig.bouncy}
        >
          {item.icon}
        </motion.div>

        <div>
          <div className="text-xs text-ailurus-cream/50">{item.title}</div>
          <div className="text-lg font-semibold text-ailurus-cream">
            {loading ? (
              <div className="h-6 w-16 bg-white/10 rounded animate-pulse" />
            ) : (
              <motion.span
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: index * 0.1, ...springConfig.snappy }}
              >
                {item.value}
              </motion.span>
            )}
          </div>
        </div>
      </div>

      {/* Right: Trend chart or action */}
      {item.title === t('当前余额') ? (
        <motion.button
          className={clsx(
            'px-3 py-1.5 rounded-full text-xs font-medium',
            'bg-ailurus-rust-500/20 text-ailurus-rust-400',
            'hover:bg-ailurus-rust-500/30',
            'border border-ailurus-rust-500/30',
            'transition-colors'
          )}
          onClick={(e) => {
            e.stopPropagation();
            navigate('/console/topup');
          }}
          whileHover={{ scale: 1.05 }}
          whileTap={{ scale: 0.95 }}
          transition={springConfig.snappy}
        >
          {t('充值')}
        </motion.button>
      ) : (
        (loading || (item.trendData && item.trendData.length > 0)) && (
          <div className="w-24 h-10 opacity-80 group-hover:opacity-100 transition-opacity">
            <VChart
              spec={getTrendSpec(item.trendData, item.trendColor)}
              option={CHART_CONFIG}
            />
          </div>
        )
      )}
    </motion.div>
  );
};

/**
 * Group card containing multiple stat items
 */
const StatsGroupCard = ({
  group,
  loading,
  getTrendSpec,
  CHART_CONFIG,
  groupIndex,
  t,
  navigate,
}) => {
  // Map group colors to Ailurus variants
  const variantMap = {
    'bg-amber-50': {
      border: 'border-ailurus-rust-500/20',
      glow: 'shadow-ailurus-rust/10',
      titleColor: 'text-ailurus-rust-400',
    },
    'bg-blue-50': {
      border: 'border-ailurus-teal-500/20',
      glow: 'shadow-ailurus-teal/10',
      titleColor: 'text-ailurus-teal-400',
    },
    'bg-green-50': {
      border: 'border-green-500/20',
      glow: 'shadow-green-500/10',
      titleColor: 'text-green-400',
    },
    'bg-purple-50': {
      border: 'border-ailurus-purple-500/20',
      glow: 'shadow-ailurus-purple/10',
      titleColor: 'text-ailurus-purple-400',
    },
  };

  const variant = variantMap[group.color] || variantMap['bg-amber-50'];

  return (
    <motion.div
      className={clsx(
        // Glassmorphism base
        'relative overflow-hidden',
        'backdrop-blur-xl',
        'bg-white/[0.03]',
        'border',
        variant.border,
        'rounded-2xl',
        // Padding
        'p-4',
        // Shadow
        variant.glow
      )}
      variants={staggerItem}
      whileHover={{
        y: -4,
        transition: springConfig.snappy,
      }}
    >
      {/* Card title */}
      <div className="flex items-center gap-2 mb-4 pb-3 border-b border-white/5">
        <h3 className={clsx('text-sm font-semibold', variant.titleColor)}>
          {group.title}
        </h3>
      </div>

      {/* Stats items */}
      <motion.div
        className="space-y-2"
        variants={staggerContainer}
        initial="initial"
        animate="animate"
      >
        {group.items.map((item, itemIdx) => (
          <StatItem
            key={itemIdx}
            item={item}
            loading={loading}
            getTrendSpec={getTrendSpec}
            CHART_CONFIG={CHART_CONFIG}
            index={groupIndex * 3 + itemIdx}
            t={t}
            navigate={navigate}
          />
        ))}
      </motion.div>

      {/* Decorative gradient glow */}
      <div
        className={clsx(
          'absolute -bottom-6 -right-6 w-24 h-24 rounded-full blur-2xl opacity-20',
          group.color?.includes('amber') && 'bg-ailurus-rust-500',
          group.color?.includes('blue') && 'bg-ailurus-teal-500',
          group.color?.includes('green') && 'bg-green-500',
          group.color?.includes('purple') && 'bg-ailurus-purple-500'
        )}
      />
    </motion.div>
  );
};

/**
 * Main StatsCards component
 */
const AilurusStatsCards = ({
  groupedStatsData,
  loading,
  getTrendSpec,
  CARD_PROPS,
  CHART_CONFIG,
}) => {
  const navigate = useNavigate();
  const { t } = useTranslation();

  return (
    <motion.div
      className="mb-6"
      variants={staggerContainer}
      initial="initial"
      animate="animate"
    >
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {groupedStatsData.map((group, idx) => (
          <StatsGroupCard
            key={idx}
            group={group}
            loading={loading}
            getTrendSpec={getTrendSpec}
            CHART_CONFIG={CHART_CONFIG}
            groupIndex={idx}
            t={t}
            navigate={navigate}
          />
        ))}
      </div>
    </motion.div>
  );
};

export default AilurusStatsCards;
