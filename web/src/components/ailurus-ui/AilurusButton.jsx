/*
 * AilurusButton - Animated Button Component
 *
 * A button with the Ailurus aesthetic:
 * - Spring-based hover and tap animations (physical bounce feel)
 * - Luminous colored shadows (never black)
 * - Gradient backgrounds
 * - "No instant changes" - everything animates smoothly
 */

import { motion } from 'framer-motion';
import { forwardRef } from 'react';
import clsx from 'clsx';
import { springConfig } from './motion';

/**
 * AilurusButton - A button with spring-based animations
 *
 * @param {object} props
 * @param {string} props.variant - Button style: 'primary' | 'secondary' | 'ghost' | 'teal' | 'purple' | 'danger'
 * @param {string} props.size - Button size: 'xs' | 'sm' | 'md' | 'lg' | 'xl'
 * @param {boolean} props.fullWidth - Make button full width
 * @param {boolean} props.loading - Show loading state
 * @param {boolean} props.disabled - Disable button
 * @param {React.ReactNode} props.leftIcon - Icon on the left
 * @param {React.ReactNode} props.rightIcon - Icon on the right
 * @param {string} props.className - Additional CSS classes
 * @param {React.ReactNode} props.children - Button content
 */
const AilurusButton = forwardRef(function AilurusButton(
  {
    variant = 'primary',
    size = 'md',
    fullWidth = false,
    loading = false,
    disabled = false,
    leftIcon,
    rightIcon,
    className,
    children,
    type = 'button',
    ...props
  },
  ref
) {
  // Size classes
  const sizeClasses = {
    xs: 'px-2.5 py-1 text-xs gap-1',
    sm: 'px-3 py-1.5 text-sm gap-1.5',
    md: 'px-4 py-2 text-sm gap-2',
    lg: 'px-5 py-2.5 text-base gap-2',
    xl: 'px-6 py-3 text-lg gap-2.5',
  };

  // Icon size classes
  const iconSizeClasses = {
    xs: 'w-3 h-3',
    sm: 'w-3.5 h-3.5',
    md: 'w-4 h-4',
    lg: 'w-5 h-5',
    xl: 'w-6 h-6',
  };

  // Variant styles with luminous shadows
  // All variants now support both light and dark themes
  const variantClasses = {
    primary: clsx(
      'bg-ailurus-gradient-rust',
      'text-white', // White text works on rust gradient in both themes
      'border-transparent',
      'shadow-ailurus-rust',
      'hover:shadow-ailurus-rust-lg',
    ),
    secondary: clsx(
      'bg-gray-100 dark:bg-white/5',
      'backdrop-blur-xl',
      'text-semi-color-text-0',
      'border border-gray-200 dark:border-white/10',
      'hover:bg-gray-200 dark:hover:bg-white/8',
      'hover:border-gray-300 dark:hover:border-white/15',
    ),
    ghost: clsx(
      'bg-transparent',
      'text-semi-color-text-0',
      'border-transparent',
      'hover:bg-gray-100 dark:hover:bg-white/5',
    ),
    teal: clsx(
      'bg-ailurus-gradient-teal',
      'text-white',
      'border-transparent',
      'shadow-ailurus-teal',
      'hover:shadow-ailurus-teal-lg',
    ),
    purple: clsx(
      'bg-ailurus-gradient-purple',
      'text-white',
      'border-transparent',
      'shadow-ailurus-purple',
      'hover:shadow-ailurus-purple-lg',
    ),
    danger: clsx(
      'bg-gradient-to-r from-red-500 to-red-600',
      'text-white',
      'border-transparent',
      'shadow-[0_4px_16px_rgba(239,68,68,0.25)]',
      'hover:shadow-[0_8px_32px_rgba(239,68,68,0.35)]',
    ),
  };

  // Disabled styles
  const disabledClasses = clsx(
    'opacity-50',
    'cursor-not-allowed',
    'pointer-events-none',
  );

  // Spring animation variants for hover and tap
  const buttonVariants = {
    initial: { scale: 1, y: 0 },
    hover: {
      scale: 1.02,
      y: -1,
      transition: springConfig.snappy,
    },
    tap: {
      scale: 0.98,
      y: 0,
      transition: springConfig.snappy,
    },
  };

  // Loading spinner component
  const LoadingSpinner = () => (
    <motion.svg
      className={clsx('animate-spin', iconSizeClasses[size])}
      viewBox="0 0 24 24"
      fill="none"
      initial={{ opacity: 0, scale: 0.8 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={springConfig.snappy}
    >
      <circle
        className="opacity-25"
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        strokeWidth="4"
      />
      <path
        className="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
      />
    </motion.svg>
  );

  return (
    <motion.button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      className={clsx(
        // Base styles
        'relative',
        'inline-flex items-center justify-center',
        'font-medium',
        'rounded-xl',
        'border',
        'outline-none',
        'select-none',
        // Focus ring
        'focus-visible:ring-2 focus-visible:ring-ailurus-rust-500 focus-visible:ring-offset-2',
        'focus-visible:ring-offset-semi-color-bg-0',
        // Transition for colors (motion handles transform)
        'transition-colors duration-200',
        // Size
        sizeClasses[size],
        // Full width
        fullWidth && 'w-full',
        // Variant
        variantClasses[variant],
        // Disabled
        (disabled || loading) && disabledClasses,
        // Custom classes
        className
      )}
      variants={buttonVariants}
      initial="initial"
      whileHover={!disabled && !loading ? 'hover' : undefined}
      whileTap={!disabled && !loading ? 'tap' : undefined}
      {...props}
    >
      {/* Loading state */}
      {loading && <LoadingSpinner />}

      {/* Left icon */}
      {!loading && leftIcon && (
        <motion.span
          className={clsx('flex-shrink-0', iconSizeClasses[size])}
          initial={{ opacity: 0, x: -4 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ delay: 0.05 }}
        >
          {leftIcon}
        </motion.span>
      )}

      {/* Button text */}
      <span className={loading ? 'opacity-0' : undefined}>
        {children}
      </span>

      {/* Right icon */}
      {!loading && rightIcon && (
        <motion.span
          className={clsx('flex-shrink-0', iconSizeClasses[size])}
          initial={{ opacity: 0, x: 4 }}
          animate={{ opacity: 1, x: 0 }}
          transition={{ delay: 0.05 }}
        >
          {rightIcon}
        </motion.span>
      )}

      {/* Shimmer effect on hover for primary buttons */}
      {variant === 'primary' && (
        <motion.div
          className="absolute inset-0 rounded-xl overflow-hidden pointer-events-none"
          initial={{ opacity: 0 }}
          whileHover={{ opacity: 1 }}
        >
          <div className="absolute inset-0 ailurus-shimmer" />
        </motion.div>
      )}
    </motion.button>
  );
});

// ==================== Button Group ====================
export const AilurusButtonGroup = forwardRef(function AilurusButtonGroup(
  { className, children, vertical = false, ...props },
  ref
) {
  return (
    <div
      ref={ref}
      className={clsx(
        'inline-flex',
        vertical ? 'flex-col' : 'flex-row',
        // Connect buttons visually
        '[&>*:not(:first-child):not(:last-child)]:rounded-none',
        vertical
          ? '[&>*:first-child]:rounded-b-none [&>*:last-child]:rounded-t-none'
          : '[&>*:first-child]:rounded-r-none [&>*:last-child]:rounded-l-none',
        // Remove double borders
        vertical
          ? '[&>*:not(:first-child)]:-mt-px'
          : '[&>*:not(:first-child)]:-ml-px',
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
});

// ==================== Icon Button ====================
export const AilurusIconButton = forwardRef(function AilurusIconButton(
  {
    variant = 'ghost',
    size = 'md',
    className,
    children,
    'aria-label': ariaLabel,
    ...props
  },
  ref
) {
  // Square size classes for icon buttons
  const iconButtonSizes = {
    xs: 'w-6 h-6',
    sm: 'w-8 h-8',
    md: 'w-10 h-10',
    lg: 'w-12 h-12',
    xl: 'w-14 h-14',
  };

  return (
    <AilurusButton
      ref={ref}
      variant={variant}
      className={clsx(
        '!p-0',
        iconButtonSizes[size],
        'flex items-center justify-center',
        className
      )}
      aria-label={ariaLabel}
      {...props}
    >
      {children}
    </AilurusButton>
  );
});

export default AilurusButton;
