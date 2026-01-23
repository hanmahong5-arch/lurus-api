/*
 * AilurusTabs - Animated Tabs Component
 *
 * A beautiful tabs component with the Ailurus aesthetic:
 * - Animated active indicator with spring physics
 * - Glassmorphic styling
 * - Multiple variants (underline, pills, cards)
 * - Keyboard navigation support
 */

import { motion } from 'framer-motion';
import { forwardRef, useState, useRef, useEffect } from 'react';
import clsx from 'clsx';
import { springConfig } from './motion';

/**
 * AilurusTabs - Tab navigation component
 *
 * @param {object} props
 * @param {Array} props.items - Tab items: { key, label, icon, disabled, content }
 * @param {string} props.activeKey - Currently active tab key
 * @param {function} props.onChange - Tab change handler
 * @param {string} props.variant - Tab style: 'underline' | 'pills' | 'cards'
 * @param {string} props.size - Tab size: 'sm' | 'md' | 'lg'
 */
const AilurusTabs = forwardRef(function AilurusTabs(
  {
    items = [],
    activeKey,
    defaultActiveKey,
    onChange,
    variant = 'underline',
    size = 'md',
    className,
    tabBarExtraContent,
    ...props
  },
  ref
) {
  const [active, setActive] = useState(activeKey || defaultActiveKey || items[0]?.key);
  const [indicatorStyle, setIndicatorStyle] = useState({});
  const tabsRef = useRef([]);

  // Sync with controlled activeKey
  useEffect(() => {
    if (activeKey !== undefined) {
      setActive(activeKey);
    }
  }, [activeKey]);

  // Update indicator position
  useEffect(() => {
    const activeIndex = items.findIndex((item) => item.key === active);
    const activeTab = tabsRef.current[activeIndex];

    if (activeTab && variant === 'underline') {
      setIndicatorStyle({
        left: activeTab.offsetLeft,
        width: activeTab.offsetWidth,
      });
    }
  }, [active, items, variant]);

  const handleTabClick = (key) => {
    const item = items.find((i) => i.key === key);
    if (item?.disabled) return;

    if (activeKey === undefined) {
      setActive(key);
    }
    onChange?.(key);
  };

  // Size classes
  const sizeClasses = {
    sm: 'text-xs px-3 py-1.5',
    md: 'text-sm px-4 py-2',
    lg: 'text-base px-5 py-2.5',
  };

  // Variant styles
  const renderTabs = () => {
    switch (variant) {
      case 'pills':
        return (
          <div className="flex items-center gap-2 p-1 bg-gray-100 dark:bg-white/5 rounded-xl">
            {items.map((item, index) => (
              <motion.button
                key={item.key}
                ref={(el) => (tabsRef.current[index] = el)}
                className={clsx(
                  'relative flex items-center gap-2 rounded-lg font-medium transition-colors',
                  sizeClasses[size],
                  item.disabled && 'opacity-50 cursor-not-allowed',
                  active === item.key
                    ? 'text-gray-900 dark:text-ailurus-cream'
                    : 'text-gray-600 dark:text-ailurus-cream/60 hover:text-gray-900 dark:hover:text-ailurus-cream'
                )}
                onClick={() => handleTabClick(item.key)}
                disabled={item.disabled}
                whileHover={!item.disabled ? { scale: 1.02 } : undefined}
                whileTap={!item.disabled ? { scale: 0.98 } : undefined}
                transition={springConfig.snappy}
              >
                {active === item.key && (
                  <motion.div
                    className="absolute inset-0 bg-ailurus-rust-500/20 rounded-lg"
                    layoutId="pillsIndicator"
                    transition={springConfig.snappy}
                  />
                )}
                <span className="relative z-10 flex items-center gap-2">
                  {item.icon}
                  {item.label}
                </span>
              </motion.button>
            ))}
          </div>
        );

      case 'cards':
        return (
          <div className="flex items-center gap-3">
            {items.map((item, index) => (
              <motion.button
                key={item.key}
                ref={(el) => (tabsRef.current[index] = el)}
                className={clsx(
                  'relative flex items-center gap-2 rounded-xl border font-medium transition-all',
                  sizeClasses[size],
                  item.disabled && 'opacity-50 cursor-not-allowed',
                  active === item.key
                    ? 'bg-ailurus-rust-500/20 border-ailurus-rust-500/40 text-gray-900 dark:text-ailurus-cream shadow-ailurus-rust'
                    : 'bg-gray-100 dark:bg-white/5 border-gray-200 dark:border-white/10 text-gray-600 dark:text-ailurus-cream/60 hover:bg-gray-200 dark:hover:bg-white/10 hover:text-gray-900 dark:hover:text-ailurus-cream'
                )}
                onClick={() => handleTabClick(item.key)}
                disabled={item.disabled}
                whileHover={!item.disabled ? { scale: 1.03, y: -2 } : undefined}
                whileTap={!item.disabled ? { scale: 0.98 } : undefined}
                transition={springConfig.snappy}
              >
                {item.icon}
                {item.label}
              </motion.button>
            ))}
          </div>
        );

      case 'underline':
      default:
        return (
          <div className="relative">
            <div className="flex items-center gap-1 border-b border-gray-200 dark:border-white/10">
              {items.map((item, index) => (
                <motion.button
                  key={item.key}
                  ref={(el) => (tabsRef.current[index] = el)}
                  className={clsx(
                    'relative flex items-center gap-2 font-medium transition-colors',
                    sizeClasses[size],
                    item.disabled && 'opacity-50 cursor-not-allowed',
                    active === item.key
                      ? 'text-ailurus-rust-500 dark:text-ailurus-rust-400'
                      : 'text-gray-600 dark:text-ailurus-cream/60 hover:text-gray-900 dark:hover:text-ailurus-cream'
                  )}
                  onClick={() => handleTabClick(item.key)}
                  disabled={item.disabled}
                  whileHover={!item.disabled ? { y: -1 } : undefined}
                  transition={springConfig.snappy}
                >
                  {item.icon}
                  {item.label}
                </motion.button>
              ))}
            </div>

            {/* Animated underline indicator */}
            <motion.div
              className="absolute bottom-0 h-0.5 bg-gradient-to-r from-ailurus-rust-500 to-ailurus-rust-400 rounded-full"
              initial={false}
              animate={indicatorStyle}
              transition={springConfig.snappy}
            />
          </div>
        );
    }
  };

  // Get active content
  const activeItem = items.find((item) => item.key === active);

  return (
    <div ref={ref} className={className} {...props}>
      {/* Tab bar */}
      <div className="flex items-center justify-between mb-4">
        {renderTabs()}
        {tabBarExtraContent && (
          <div className="ml-4">{tabBarExtraContent}</div>
        )}
      </div>

      {/* Tab content */}
      {activeItem?.content && (
        <motion.div
          key={active}
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -10 }}
          transition={springConfig.snappy}
        >
          {activeItem.content}
        </motion.div>
      )}
    </div>
  );
});

// ==================== Tab Pane (for composition) ====================
export const AilurusTabPane = ({ children, tab, ...props }) => {
  return children;
};

export default AilurusTabs;
