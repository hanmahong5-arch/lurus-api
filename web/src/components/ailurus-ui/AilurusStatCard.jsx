/*
 * AilurusStatCard - Animated Statistics Card Component
 *
 * A beautiful stat card with the Ailurus aesthetic:
 * - Glassmorphic background with luminous shadows
 * - Animated number counting on mount
 * - Hover effects with spring physics
 * - Multiple variants for different metrics
 */

import { motion, useMotionValue, useTransform, animate } from 'framer-motion';
import { forwardRef, useEffect, useState } from 'react';
import clsx from 'clsx';
import { springConfig, cardVariants } from './motion';

/**
 * AilurusStatCard - Stats display card with animated values
 *
 * @param {object} props
 * @param {string} props.title - Metric title
 * @param {number|string} props.value - The value to display
 * @param {string} props.suffix - Optional suffix (e.g., "USD", "%")
 * @param {string} props.prefix - Optional prefix (e.g., "$")
 * @param {React.ReactNode} props.icon - Icon element
 * @param {string} props.trend - Trend direction: 'up' | 'down' | 'neutral'
 * @param {string} props.trendValue - Trend percentage or value
 * @param {string} props.variant - Color variant: 'default' | 'rust' | 'teal' | 'purple'
 * @param {boolean} props.animate - Whether to animate the number on mount
 * @param {number} props.delay - Animation delay in seconds
 */
const AilurusStatCard = forwardRef(function AilurusStatCard(
  {
    title,
    value,
    suffix = '',
    prefix = '',
    icon,
    trend,
    trendValue,
    variant = 'default',
    animate: shouldAnimate = true,
    delay = 0,
    className,
    onClick,
    ...props
  },
  ref
) {
  const [displayValue, setDisplayValue] = useState(shouldAnimate ? 0 : value);

  // Animate number counting
  useEffect(() => {
    if (shouldAnimate && typeof value === 'number') {
      const controls = animate(0, value, {
        duration: 1.5,
        delay: delay,
        ease: [0.25, 0.1, 0.25, 1],
        onUpdate: (latest) => setDisplayValue(Math.round(latest)),
      });
      return () => controls.stop();
    } else {
      setDisplayValue(value);
    }
  }, [value, shouldAnimate, delay]);

  // Variant styles - now with light theme support
  const variantStyles = {
    default: {
      iconBg: 'bg-ailurus-rust-500/20',
      iconColor: 'text-ailurus-rust-500 dark:text-ailurus-rust-400',
      shadow: 'shadow-sm dark:shadow-ailurus-rust/10',
      border: 'border-gray-200 dark:border-white/5',
    },
    rust: {
      iconBg: 'bg-ailurus-rust-500/20',
      iconColor: 'text-ailurus-rust-500 dark:text-ailurus-rust-400',
      shadow: 'shadow-md shadow-ailurus-rust-200 dark:shadow-ailurus-rust',
      border: 'border-ailurus-rust-200 dark:border-ailurus-rust-500/20',
    },
    teal: {
      iconBg: 'bg-ailurus-teal-500/20',
      iconColor: 'text-ailurus-teal-600 dark:text-ailurus-teal-400',
      shadow: 'shadow-md shadow-ailurus-teal-200 dark:shadow-ailurus-teal',
      border: 'border-ailurus-teal-200 dark:border-ailurus-teal-500/20',
    },
    purple: {
      iconBg: 'bg-ailurus-purple-500/20',
      iconColor: 'text-ailurus-purple-600 dark:text-ailurus-purple-400',
      shadow: 'shadow-md shadow-ailurus-purple-200 dark:shadow-ailurus-purple',
      border: 'border-ailurus-purple-200 dark:border-ailurus-purple-500/20',
    },
  };

  const styles = variantStyles[variant] || variantStyles.default;

  // Trend colors
  const trendColors = {
    up: 'text-green-400',
    down: 'text-red-400',
    neutral: 'text-gray-400',
  };

  // Trend icons
  const TrendIcon = () => {
    if (trend === 'up') {
      return (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 10l7-7m0 0l7 7m-7-7v18" />
        </svg>
      );
    }
    if (trend === 'down') {
      return (
        <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 14l-7 7m0 0l-7-7m7 7V3" />
        </svg>
      );
    }
    return null;
  };

  return (
    <motion.div
      ref={ref}
      className={clsx(
        // Glassmorphism base
        'relative overflow-hidden',
        'backdrop-blur-xl',
        'bg-white dark:bg-white/[0.03]',
        'border',
        styles.border,
        'rounded-2xl',
        // Padding
        'p-5',
        // Shadow
        styles.shadow,
        // Cursor
        onClick && 'cursor-pointer',
        className
      )}
      variants={cardVariants}
      initial="initial"
      animate="animate"
      whileHover={onClick ? "hover" : undefined}
      whileTap={onClick ? "tap" : undefined}
      onClick={onClick}
      custom={delay}
      {...props}
    >
      {/* Gradient overlay on hover */}
      <motion.div
        className="absolute inset-0 bg-gradient-to-br from-white/5 to-transparent opacity-0"
        whileHover={{ opacity: 1 }}
        transition={{ duration: 0.3 }}
      />

      <div className="relative z-10">
        {/* Header with icon and title */}
        <div className="flex items-center justify-between mb-4">
          <span className="text-sm font-medium text-gray-600 dark:text-ailurus-cream/60">{title}</span>
          {icon && (
            <motion.div
              className={clsx(
                'w-10 h-10 rounded-xl flex items-center justify-center',
                styles.iconBg
              )}
              whileHover={{ scale: 1.1, rotate: 5 }}
              transition={springConfig.bouncy}
            >
              <span className={styles.iconColor}>{icon}</span>
            </motion.div>
          )}
        </div>

        {/* Value */}
        <div className="flex items-baseline gap-2">
          <motion.span
            className="text-3xl font-bold text-gray-900 dark:text-ailurus-cream"
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: delay + 0.2, ...springConfig.snappy }}
          >
            {prefix}
            {typeof displayValue === 'number'
              ? displayValue.toLocaleString()
              : displayValue}
            {suffix}
          </motion.span>

          {/* Trend indicator */}
          {trend && trendValue && (
            <motion.div
              className={clsx(
                'flex items-center gap-1 text-sm font-medium',
                trendColors[trend]
              )}
              initial={{ opacity: 0, x: -10 }}
              animate={{ opacity: 1, x: 0 }}
              transition={{ delay: delay + 0.4, ...springConfig.snappy }}
            >
              <TrendIcon />
              <span>{trendValue}</span>
            </motion.div>
          )}
        </div>
      </div>

      {/* Decorative glow */}
      <div
        className={clsx(
          'absolute -bottom-4 -right-4 w-24 h-24 rounded-full blur-2xl opacity-20',
          variant === 'rust' && 'bg-ailurus-rust-500',
          variant === 'teal' && 'bg-ailurus-teal-500',
          variant === 'purple' && 'bg-ailurus-purple-500',
          variant === 'default' && 'bg-ailurus-rust-500'
        )}
      />
    </motion.div>
  );
});

// ==================== Stat Card Group ====================
export const AilurusStatCardGroup = forwardRef(function AilurusStatCardGroup(
  { children, columns = 4, className, ...props },
  ref
) {
  const gridCols = {
    1: 'grid-cols-1',
    2: 'grid-cols-1 sm:grid-cols-2',
    3: 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-3',
    4: 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-4',
    5: 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-5',
  };

  return (
    <div
      ref={ref}
      className={clsx('grid gap-4', gridCols[columns] || gridCols[4], className)}
      {...props}
    >
      {children}
    </div>
  );
});

// ==================== Mini Stat Card ====================
export const AilurusMiniStatCard = forwardRef(function AilurusMiniStatCard(
  {
    label,
    value,
    icon,
    variant = 'default',
    className,
    ...props
  },
  ref
) {
  const variantStyles = {
    default: 'bg-gray-50 dark:bg-white/5 border-gray-200 dark:border-white/5',
    rust: 'bg-ailurus-rust-50 dark:bg-ailurus-rust-500/10 border-ailurus-rust-200 dark:border-ailurus-rust-500/20',
    teal: 'bg-ailurus-teal-50 dark:bg-ailurus-teal-500/10 border-ailurus-teal-200 dark:border-ailurus-teal-500/20',
    purple: 'bg-ailurus-purple-50 dark:bg-ailurus-purple-500/10 border-ailurus-purple-200 dark:border-ailurus-purple-500/20',
  };

  const iconColors = {
    default: 'text-gray-500 dark:text-ailurus-cream/60',
    rust: 'text-ailurus-rust-500 dark:text-ailurus-rust-400',
    teal: 'text-ailurus-teal-600 dark:text-ailurus-teal-400',
    purple: 'text-ailurus-purple-600 dark:text-ailurus-purple-400',
  };

  return (
    <motion.div
      ref={ref}
      className={clsx(
        'flex items-center gap-3 p-3 rounded-xl border',
        variantStyles[variant],
        className
      )}
      whileHover={{ scale: 1.02 }}
      transition={springConfig.snappy}
      {...props}
    >
      {icon && (
        <span className={iconColors[variant]}>{icon}</span>
      )}
      <div className="flex flex-col">
        <span className="text-xs text-gray-500 dark:text-ailurus-cream/50">{label}</span>
        <span className="text-sm font-semibold text-gray-900 dark:text-ailurus-cream">{value}</span>
      </div>
    </motion.div>
  );
});

export default AilurusStatCard;
