/*
 * AilurusInput - Animated Input Component
 *
 * An input with the Ailurus aesthetic:
 * - Glassmorphic background
 * - Luminous focus ring (rust orange glow)
 * - Smooth focus/blur animations
 * - Floating label support
 */

import { motion, AnimatePresence } from 'framer-motion';
import { forwardRef, useState, useId } from 'react';
import clsx from 'clsx';
import { springConfig } from './motion';

/**
 * AilurusInput - An input with animated focus states
 *
 * @param {object} props
 * @param {string} props.label - Input label
 * @param {string} props.placeholder - Placeholder text
 * @param {string} props.error - Error message
 * @param {string} props.hint - Hint text below input
 * @param {string} props.size - Input size: 'sm' | 'md' | 'lg'
 * @param {boolean} props.disabled - Disable input
 * @param {React.ReactNode} props.leftIcon - Icon on the left
 * @param {React.ReactNode} props.rightIcon - Icon on the right
 * @param {boolean} props.floating - Use floating label style
 */
const AilurusInput = forwardRef(function AilurusInput(
  {
    label,
    placeholder,
    error,
    hint,
    size = 'md',
    disabled = false,
    leftIcon,
    rightIcon,
    floating = false,
    className,
    wrapperClassName,
    type = 'text',
    value,
    defaultValue,
    onChange,
    onFocus,
    onBlur,
    ...props
  },
  ref
) {
  const id = useId();
  const [isFocused, setIsFocused] = useState(false);
  const [hasValue, setHasValue] = useState(!!value || !!defaultValue);

  // Size classes
  const sizeClasses = {
    sm: 'px-3 py-1.5 text-sm',
    md: 'px-4 py-2.5 text-base',
    lg: 'px-5 py-3 text-lg',
  };

  const iconSizeClasses = {
    sm: 'w-4 h-4',
    md: 'w-5 h-5',
    lg: 'w-6 h-6',
  };

  // Handle focus
  const handleFocus = (e) => {
    setIsFocused(true);
    onFocus?.(e);
  };

  // Handle blur
  const handleBlur = (e) => {
    setIsFocused(false);
    onBlur?.(e);
  };

  // Handle change to track if has value (for floating label)
  const handleChange = (e) => {
    setHasValue(!!e.target.value);
    onChange?.(e);
  };

  // Animation variants for the focus ring glow
  const glowVariants = {
    unfocused: {
      boxShadow: '0 0 0 0px rgba(194, 94, 0, 0)',
    },
    focused: {
      boxShadow: '0 0 0 3px rgba(194, 94, 0, 0.15)',
      transition: springConfig.snappy,
    },
    error: {
      boxShadow: '0 0 0 3px rgba(239, 68, 68, 0.15)',
      transition: springConfig.snappy,
    },
  };

  // Floating label animation
  const floatingLabelVariants = {
    default: {
      top: '50%',
      y: '-50%',
      scale: 1,
      color: 'var(--semi-color-text-2)',
    },
    active: {
      top: '0',
      y: '-50%',
      scale: 0.85,
      color: error ? 'rgb(239, 68, 68)' : 'var(--ailurus-rust-500)',
      transition: springConfig.snappy,
    },
  };

  const showFloatingLabel = floating && (isFocused || hasValue);

  return (
    <div className={clsx('relative', wrapperClassName)}>
      {/* Standard label (non-floating) */}
      {label && !floating && (
        <motion.label
          htmlFor={id}
          className={clsx(
            'block mb-2 text-sm font-medium',
            error ? 'text-red-500' : 'text-semi-color-text-0'
          )}
          initial={{ opacity: 0, y: -4 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.2 }}
        >
          {label}
        </motion.label>
      )}

      {/* Input wrapper */}
      <motion.div
        className="relative"
        variants={glowVariants}
        initial="unfocused"
        animate={error ? 'error' : isFocused ? 'focused' : 'unfocused'}
      >
        {/* Left icon */}
        {leftIcon && (
          <div
            className={clsx(
              'absolute left-3 top-1/2 -translate-y-1/2',
              'text-semi-color-text-2',
              'pointer-events-none',
              iconSizeClasses[size]
            )}
          >
            {leftIcon}
          </div>
        )}

        {/* Floating label */}
        {floating && label && (
          <motion.label
            htmlFor={id}
            className={clsx(
              'absolute left-4 pointer-events-none',
              'px-1 bg-semi-color-bg-0',
              'text-sm font-medium origin-left',
              'z-10'
            )}
            variants={floatingLabelVariants}
            initial="default"
            animate={showFloatingLabel ? 'active' : 'default'}
          >
            {label}
          </motion.label>
        )}

        {/* The input element */}
        <input
          ref={ref}
          id={id}
          type={type}
          value={value}
          defaultValue={defaultValue}
          disabled={disabled}
          placeholder={floating ? (isFocused ? placeholder : '') : placeholder}
          onChange={handleChange}
          onFocus={handleFocus}
          onBlur={handleBlur}
          className={clsx(
            // Base styles
            'w-full rounded-xl outline-none',
            'transition-colors duration-200',
            // Background and border
            'bg-white/5 dark:bg-white/5',
            'border',
            error
              ? 'border-red-500'
              : isFocused
                ? 'border-ailurus-rust-500'
                : 'border-white/10 dark:border-white/10',
            // Text
            'text-semi-color-text-0',
            'placeholder:text-semi-color-text-2',
            // Size
            sizeClasses[size],
            // Icons padding
            leftIcon && 'pl-10',
            rightIcon && 'pr-10',
            // Disabled
            disabled && 'opacity-50 cursor-not-allowed bg-semi-color-fill-0',
            // Custom class
            className
          )}
          {...props}
        />

        {/* Right icon */}
        {rightIcon && (
          <div
            className={clsx(
              'absolute right-3 top-1/2 -translate-y-1/2',
              'text-semi-color-text-2',
              iconSizeClasses[size]
            )}
          >
            {rightIcon}
          </div>
        )}

        {/* Focus line animation at bottom */}
        <motion.div
          className="absolute bottom-0 left-0 right-0 h-0.5 bg-ailurus-rust-500 rounded-full"
          initial={{ scaleX: 0 }}
          animate={{ scaleX: isFocused ? 1 : 0 }}
          transition={springConfig.snappy}
          style={{ originX: 0.5 }}
        />
      </motion.div>

      {/* Error message */}
      <AnimatePresence>
        {error && (
          <motion.p
            className="mt-1.5 text-sm text-red-500"
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.2 }}
          >
            {error}
          </motion.p>
        )}
      </AnimatePresence>

      {/* Hint text */}
      {hint && !error && (
        <p className="mt-1.5 text-sm text-semi-color-text-2">{hint}</p>
      )}
    </div>
  );
});

// ==================== Textarea ====================
export const AilurusTextarea = forwardRef(function AilurusTextarea(
  {
    label,
    error,
    hint,
    rows = 4,
    disabled = false,
    className,
    wrapperClassName,
    ...props
  },
  ref
) {
  const id = useId();
  const [isFocused, setIsFocused] = useState(false);

  const glowVariants = {
    unfocused: {
      boxShadow: '0 0 0 0px rgba(194, 94, 0, 0)',
    },
    focused: {
      boxShadow: '0 0 0 3px rgba(194, 94, 0, 0.15)',
      transition: springConfig.snappy,
    },
    error: {
      boxShadow: '0 0 0 3px rgba(239, 68, 68, 0.15)',
      transition: springConfig.snappy,
    },
  };

  return (
    <div className={clsx('relative', wrapperClassName)}>
      {label && (
        <label
          htmlFor={id}
          className={clsx(
            'block mb-2 text-sm font-medium',
            error ? 'text-red-500' : 'text-semi-color-text-0'
          )}
        >
          {label}
        </label>
      )}

      <motion.div
        variants={glowVariants}
        initial="unfocused"
        animate={error ? 'error' : isFocused ? 'focused' : 'unfocused'}
      >
        <textarea
          ref={ref}
          id={id}
          rows={rows}
          disabled={disabled}
          onFocus={() => setIsFocused(true)}
          onBlur={() => setIsFocused(false)}
          className={clsx(
            'w-full rounded-xl outline-none',
            'px-4 py-3 text-base',
            'resize-y min-h-[100px]',
            'transition-colors duration-200',
            'bg-white/5 dark:bg-white/5',
            'border',
            error
              ? 'border-red-500'
              : isFocused
                ? 'border-ailurus-rust-500'
                : 'border-white/10',
            'text-semi-color-text-0',
            'placeholder:text-semi-color-text-2',
            disabled && 'opacity-50 cursor-not-allowed',
            className
          )}
          {...props}
        />
      </motion.div>

      <AnimatePresence>
        {error && (
          <motion.p
            className="mt-1.5 text-sm text-red-500"
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
          >
            {error}
          </motion.p>
        )}
      </AnimatePresence>

      {hint && !error && (
        <p className="mt-1.5 text-sm text-semi-color-text-2">{hint}</p>
      )}
    </div>
  );
});

export default AilurusInput;
