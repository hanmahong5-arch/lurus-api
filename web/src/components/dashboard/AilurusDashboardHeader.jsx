/*
 * AilurusDashboardHeader - Ailurus Styled Dashboard Header
 *
 * A beautiful dashboard header with the Ailurus aesthetic:
 * - Animated greeting text with fade-in effect
 * - Glassmorphic action buttons
 * - Spring-based hover animations
 */

import { motion, AnimatePresence } from 'framer-motion';
import { RefreshCw, Search } from 'lucide-react';
import { springConfig, staggerContainer, staggerItem } from '../ailurus-ui/motion';
import { AilurusButton, AilurusIconButton } from '../ailurus-ui';

const AilurusDashboardHeader = ({
  getGreeting,
  greetingVisible,
  showSearchModal,
  refresh,
  loading,
  t,
}) => {
  return (
    <motion.div
      className="flex items-center justify-between mb-6"
      variants={staggerContainer}
      initial="initial"
      animate="animate"
    >
      {/* Greeting text with animated fade */}
      <motion.div variants={staggerItem} className="flex-1">
        <AnimatePresence mode="wait">
          {greetingVisible && (
            <motion.h2
              key="greeting"
              className="text-2xl md:text-3xl font-bold text-semi-color-text-0"
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: -10 }}
              transition={springConfig.snappy}
            >
              {/* Gradient text effect - using theme-aware colors */}
              <span className="bg-gradient-to-r from-ailurus-rust-500 via-ailurus-rust-400 to-ailurus-rust-500 bg-clip-text text-transparent">
                {getGreeting}
              </span>
            </motion.h2>
          )}
        </AnimatePresence>

        {/* Subtle subtitle */}
        <motion.p
          className="text-sm text-semi-color-text-2 mt-1"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.3, ...springConfig.snappy }}
        >
          {t ? t('dashboard.welcomeBack') : 'Welcome back to your dashboard'}
        </motion.p>
      </motion.div>

      {/* Action buttons */}
      <motion.div
        className="flex items-center gap-3"
        variants={staggerItem}
      >
        {/* Search button */}
        <motion.button
          className="group relative p-3 rounded-xl bg-semi-color-fill-0 border border-semi-color-border
                     hover:bg-ailurus-teal-500/20 hover:border-ailurus-teal-500/40
                     transition-colors"
          onClick={showSearchModal}
          whileHover={{ scale: 1.05 }}
          whileTap={{ scale: 0.95 }}
          transition={springConfig.snappy}
        >
          <Search
            size={18}
            className="text-semi-color-text-2 group-hover:text-ailurus-teal-500 transition-colors"
          />
          {/* Glow effect on hover */}
          <motion.div
            className="absolute inset-0 rounded-xl bg-ailurus-teal-500/20 blur-xl opacity-0
                       group-hover:opacity-100 transition-opacity -z-10"
          />
        </motion.button>

        {/* Refresh button */}
        <motion.button
          className="group relative p-3 rounded-xl bg-semi-color-fill-0 border border-semi-color-border
                     hover:bg-ailurus-rust-500/20 hover:border-ailurus-rust-500/40
                     transition-colors disabled:opacity-50"
          onClick={refresh}
          disabled={loading}
          whileHover={{ scale: loading ? 1 : 1.05 }}
          whileTap={{ scale: loading ? 1 : 0.95 }}
          transition={springConfig.snappy}
        >
          <RefreshCw
            size={18}
            className={`text-semi-color-text-2 group-hover:text-ailurus-rust-500 transition-colors
                       ${loading ? 'animate-spin' : ''}`}
          />
          {/* Glow effect on hover */}
          <motion.div
            className="absolute inset-0 rounded-xl bg-ailurus-rust-500/20 blur-xl opacity-0
                       group-hover:opacity-100 transition-opacity -z-10"
          />
        </motion.button>
      </motion.div>
    </motion.div>
  );
};

export default AilurusDashboardHeader;
