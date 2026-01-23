/*
 * AilurusCard - Glassmorphic Card Component
 *
 * A reusable card component with the Ailurus aesthetic:
 * - Frosted glass background
 * - Luminous colored shadows (never black)
 * - Spring-based hover animations
 * - Soft geometry (rounded corners)
 */

import { motion } from 'framer-motion';
import { forwardRef } from 'react';
import clsx from 'clsx';
import { cardVariants, springConfig } from './motion';

/**
 * AilurusCard - A glassmorphic card with hover animations
 *
 * @param {object} props
 * @param {string} props.variant - Card style variant: 'default' | 'rust' | 'teal' | 'purple'
 * @param {boolean} props.hoverable - Enable hover lift effect (default: true)
 * @param {boolean} props.clickable - Enable click/tap effect (default: false)
 * @param {string} props.padding - Padding size: 'none' | 'sm' | 'md' | 'lg' | 'xl'
 * @param {string} props.className - Additional CSS classes
 * @param {React.ReactNode} props.children - Card content
 * @param {function} props.onClick - Click handler
 * @param {boolean} props.animate - Enable entrance animation (default: true)
 * @param {number} props.delay - Entrance animation delay in seconds
 */
const AilurusCard = forwardRef(function AilurusCard(
  {
    variant = 'default',
    hoverable = true,
    clickable = false,
    padding = 'md',
    className,
    children,
    onClick,
    animate = true,
    delay = 0,
    ...props
  },
  ref
) {
  // Padding size classes
  const paddingClasses = {
    none: '',
    sm: 'p-3',
    md: 'p-5',
    lg: 'p-6',
    xl: 'p-8',
  };

  // Variant-specific styles for luminous shadows
  const variantStyles = {
    default: {
      base: '',
      hover: 'hover:shadow-ailurus-glass-lg',
    },
    rust: {
      base: 'border-ailurus-rust-500/20',
      hover: 'hover:shadow-ailurus-rust-lg',
      glow: 'shadow-ailurus-rust-sm',
    },
    teal: {
      base: 'border-ailurus-teal-500/20',
      hover: 'hover:shadow-ailurus-teal-lg',
      glow: 'shadow-ailurus-teal-sm',
    },
    purple: {
      base: 'border-ailurus-purple-500/20',
      hover: 'hover:shadow-ailurus-purple-lg',
      glow: 'shadow-ailurus-purple-sm',
    },
  };

  const currentVariant = variantStyles[variant] || variantStyles.default;

  // Animation variants
  const motionVariants = animate ? {
    initial: { opacity: 0, y: 20, scale: 0.98 },
    animate: {
      opacity: 1,
      y: 0,
      scale: 1,
      transition: {
        duration: 0.5,
        ease: [0.16, 1, 0.3, 1],
        delay,
      }
    },
    ...(hoverable && {
      hover: {
        y: -4,
        scale: 1.005,
        transition: springConfig.default,
      },
    }),
    ...(clickable && {
      tap: {
        scale: 0.995,
        y: -2,
        transition: springConfig.snappy,
      },
    }),
  } : {};

  return (
    <motion.div
      ref={ref}
      className={clsx(
        // Base glass panel styles
        'ailurus-glass-panel',
        'overflow-hidden',
        'rounded-2xl',
        // Transition for non-animated hover states
        'transition-all duration-300 ease-ailurus-smooth',
        // Padding
        paddingClasses[padding],
        // Variant-specific styles
        currentVariant.base,
        currentVariant.glow,
        hoverable && currentVariant.hover,
        // Clickable cursor
        clickable && 'cursor-pointer',
        // Custom classes
        className
      )}
      variants={motionVariants}
      initial={animate ? 'initial' : undefined}
      animate={animate ? 'animate' : undefined}
      whileHover={hoverable ? 'hover' : undefined}
      whileTap={clickable ? 'tap' : undefined}
      onClick={onClick}
      {...props}
    >
      {children}
    </motion.div>
  );
});

// ==================== Card Header ====================
export const AilurusCardHeader = forwardRef(function AilurusCardHeader(
  { className, children, ...props },
  ref
) {
  return (
    <div
      ref={ref}
      className={clsx(
        'ailurus-heading',
        'pb-4 mb-4',
        'border-b border-gray-200 dark:border-white/5',
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
});

// ==================== Card Title ====================
export const AilurusCardTitle = forwardRef(function AilurusCardTitle(
  { className, children, size = 'md', ...props },
  ref
) {
  const sizeClasses = {
    sm: 'text-base',
    md: 'text-lg',
    lg: 'text-xl',
    xl: 'text-2xl',
  };

  return (
    <h3
      ref={ref}
      className={clsx(
        'ailurus-heading',
        'font-semibold',
        sizeClasses[size],
        'text-semi-color-text-0',
        className
      )}
      {...props}
    >
      {children}
    </h3>
  );
});

// ==================== Card Description ====================
export const AilurusCardDescription = forwardRef(function AilurusCardDescription(
  { className, children, ...props },
  ref
) {
  return (
    <p
      ref={ref}
      className={clsx(
        'ailurus-body',
        'text-sm',
        'text-semi-color-text-2',
        'mt-1',
        className
      )}
      {...props}
    >
      {children}
    </p>
  );
});

// ==================== Card Content ====================
export const AilurusCardContent = forwardRef(function AilurusCardContent(
  { className, children, ...props },
  ref
) {
  return (
    <div
      ref={ref}
      className={clsx('ailurus-body', className)}
      {...props}
    >
      {children}
    </div>
  );
});

// ==================== Card Footer ====================
export const AilurusCardFooter = forwardRef(function AilurusCardFooter(
  { className, children, ...props },
  ref
) {
  return (
    <div
      ref={ref}
      className={clsx(
        'pt-4 mt-4',
        'border-t border-gray-200 dark:border-white/5',
        'flex items-center gap-3',
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
});

// ==================== Card Group (Stagger Container) ====================
export const AilurusCardGroup = forwardRef(function AilurusCardGroup(
  { className, children, staggerDelay = 0.08, ...props },
  ref
) {
  return (
    <motion.div
      ref={ref}
      className={clsx('grid gap-4', className)}
      initial="initial"
      animate="animate"
      variants={{
        initial: {},
        animate: {
          transition: {
            staggerChildren: staggerDelay,
            delayChildren: 0.1,
          },
        },
      }}
      {...props}
    >
      {children}
    </motion.div>
  );
});

export default AilurusCard;
