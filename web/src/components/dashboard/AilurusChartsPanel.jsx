/*
 * AilurusChartsPanel - Ailurus Styled Charts Panel
 *
 * A beautiful charts panel with the Ailurus aesthetic:
 * - Glassmorphic card background
 * - Animated tab switching
 * - Chart content with smooth transitions
 */

import React from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { PieChart, TrendingUp, BarChart3, Activity } from 'lucide-react';
import { VChart } from '@visactor/react-vchart';
import clsx from 'clsx';
import { springConfig } from '../ailurus-ui/motion';

const AilurusChartsPanel = ({
  activeChartTab,
  setActiveChartTab,
  spec_line,
  spec_model_line,
  spec_pie,
  spec_rank_bar,
  CARD_PROPS,
  CHART_CONFIG,
  FLEX_CENTER_GAP2,
  hasApiInfoPanel,
  t,
}) => {
  // Tab definitions with icons
  const tabs = [
    { key: '1', label: t('消耗分布'), icon: <Activity size={14} /> },
    { key: '2', label: t('消耗趋势'), icon: <TrendingUp size={14} /> },
    { key: '3', label: t('调用次数分布'), icon: <PieChart size={14} /> },
    { key: '4', label: t('调用次数排行'), icon: <BarChart3 size={14} /> },
  ];

  // Get chart spec based on active tab
  const getChartSpec = () => {
    switch (activeChartTab) {
      case '1':
        return spec_line;
      case '2':
        return spec_model_line;
      case '3':
        return spec_pie;
      case '4':
        return spec_rank_bar;
      default:
        return spec_line;
    }
  };

  return (
    <motion.div
      className={clsx(
        // Glassmorphism base - use theme-aware panel
        'ailurus-glass-panel',
        'relative overflow-hidden',
        'border border-semi-color-border',
        'rounded-2xl',
        // Grid span
        hasApiInfoPanel ? 'lg:col-span-3' : ''
      )}
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={springConfig.snappy}
    >
      {/* Header */}
      <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between gap-4 p-4 border-b border-semi-color-border">
        {/* Title */}
        <div className="flex items-center gap-2">
          <motion.div
            className="w-8 h-8 rounded-lg bg-ailurus-rust-500/20 flex items-center justify-center"
            whileHover={{ scale: 1.1, rotate: 5 }}
            transition={springConfig.bouncy}
          >
            <PieChart size={16} className="text-ailurus-rust-500" />
          </motion.div>
          <h3 className="text-sm font-semibold text-semi-color-text-0">
            {t('模型数据分析')}
          </h3>
        </div>

        {/* Tabs */}
        <div className="flex items-center gap-1 p-1 bg-semi-color-fill-0 rounded-xl">
          {tabs.map((tab) => (
            <motion.button
              key={tab.key}
              className={clsx(
                'relative flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors',
                activeChartTab === tab.key
                  ? 'text-semi-color-text-0'
                  : 'text-semi-color-text-2 hover:text-semi-color-text-1'
              )}
              onClick={() => setActiveChartTab(tab.key)}
              whileHover={{ scale: 1.02 }}
              whileTap={{ scale: 0.98 }}
              transition={springConfig.snappy}
            >
              {/* Active indicator */}
              {activeChartTab === tab.key && (
                <motion.div
                  className="absolute inset-0 bg-ailurus-rust-500/20 rounded-lg"
                  layoutId="chartTabIndicator"
                  transition={springConfig.snappy}
                />
              )}
              <span className="relative z-10 flex items-center gap-1.5">
                {tab.icon}
                <span className="hidden sm:inline">{tab.label}</span>
              </span>
            </motion.button>
          ))}
        </div>
      </div>

      {/* Chart content */}
      <div className="h-96 p-4">
        <AnimatePresence mode="wait">
          <motion.div
            key={activeChartTab}
            className="h-full"
            initial={{ opacity: 0, scale: 0.98 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.98 }}
            transition={springConfig.snappy}
          >
            <VChart spec={getChartSpec()} option={CHART_CONFIG} />
          </motion.div>
        </AnimatePresence>
      </div>

      {/* Decorative corner glow */}
      <div className="absolute -top-20 -right-20 w-40 h-40 bg-ailurus-rust-500/10 rounded-full blur-3xl pointer-events-none" />
      <div className="absolute -bottom-20 -left-20 w-40 h-40 bg-ailurus-teal-500/10 rounded-full blur-3xl pointer-events-none" />
    </motion.div>
  );
};

export default AilurusChartsPanel;
