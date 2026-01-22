/*
 * AilurusAuthLayout - Authentication Page Layout
 *
 * A beautiful authentication layout implementing the Ailurus aesthetic:
 * - Dark forest gradient background
 * - Subtle noise texture
 * - Animated glassmorphic card
 * - Luminous orange glow effects
 *
 * Use this as a wrapper for login/register forms.
 */

import { motion, AnimatePresence } from 'framer-motion';
import { forwardRef } from 'react';
import clsx from 'clsx';
import { springConfig, staggerContainer, staggerItem } from './motion';

/**
 * AilurusAuthLayout - Full-page auth layout with animated background
 *
 * @param {object} props
 * @param {React.ReactNode} props.children - Form content
 * @param {string} props.logo - Logo image URL
 * @param {string} props.title - Page title (e.g., "Welcome back")
 * @param {string} props.subtitle - Subtitle text
 * @param {string} props.systemName - System/App name
 */
const AilurusAuthLayout = forwardRef(function AilurusAuthLayout(
  {
    children,
    logo,
    title,
    subtitle,
    systemName,
    className,
    ...props
  },
  ref
) {
  return (
    <div
      ref={ref}
      className={clsx(
        // Full screen container
        'min-h-screen w-full',
        'flex items-center justify-center',
        'p-4 sm:p-6 lg:p-8',
        // Dark forest gradient background
        'bg-gradient-to-br from-ailurus-obsidian via-ailurus-forest to-ailurus-obsidian-900',
        // Position for pseudo-elements
        'relative overflow-hidden',
        className
      )}
      {...props}
    >
      {/* Noise texture overlay */}
      <div
        className="absolute inset-0 pointer-events-none z-0 opacity-[0.03]"
        style={{
          backgroundImage: `url("data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.8' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noise)'/%3E%3C/svg%3E")`,
        }}
      />

      {/* Animated background blur balls */}
      <motion.div
        className="absolute top-[-20%] right-[-10%] w-[500px] h-[500px] rounded-full bg-ailurus-rust-500/20 blur-[120px]"
        animate={{
          scale: [1, 1.1, 1],
          opacity: [0.2, 0.3, 0.2],
        }}
        transition={{
          duration: 8,
          repeat: Infinity,
          ease: 'easeInOut',
        }}
      />
      <motion.div
        className="absolute bottom-[-30%] left-[-20%] w-[600px] h-[600px] rounded-full bg-ailurus-teal-500/10 blur-[150px]"
        animate={{
          scale: [1, 1.15, 1],
          opacity: [0.1, 0.15, 0.1],
        }}
        transition={{
          duration: 10,
          repeat: Infinity,
          ease: 'easeInOut',
          delay: 1,
        }}
      />
      <motion.div
        className="absolute top-[40%] left-[10%] w-[300px] h-[300px] rounded-full bg-ailurus-purple-500/10 blur-[100px]"
        animate={{
          scale: [1, 1.2, 1],
          opacity: [0.1, 0.15, 0.1],
        }}
        transition={{
          duration: 12,
          repeat: Infinity,
          ease: 'easeInOut',
          delay: 2,
        }}
      />

      {/* Main content container */}
      <motion.div
        className="relative z-10 w-full max-w-md"
        initial="initial"
        animate="animate"
        variants={staggerContainer}
      >
        {/* Logo and system name */}
        {(logo || systemName) && (
          <motion.div
            className="flex items-center justify-center gap-3 mb-8"
            variants={staggerItem}
          >
            {logo && (
              <motion.img
                src={logo}
                alt="Logo"
                className="h-12 w-12 rounded-2xl shadow-ailurus-rust"
                whileHover={{ scale: 1.05, rotate: 5 }}
                transition={springConfig.bouncy}
              />
            )}
            {systemName && (
              <span className="ailurus-heading text-2xl font-bold text-ailurus-cream">
                {systemName}
              </span>
            )}
          </motion.div>
        )}

        {/* Glass card container */}
        <motion.div
          className={clsx(
            // Glassmorphism
            'backdrop-blur-xl',
            'bg-white/[0.03]',
            'border border-white/[0.08]',
            'rounded-3xl',
            // Luminous shadow
            'shadow-[0_20px_60px_rgba(194,94,0,0.15),inset_0_1px_0_rgba(255,255,255,0.08)]',
            // Padding
            'p-8 sm:p-10'
          )}
          variants={staggerItem}
        >
          {/* Title section */}
          {(title || subtitle) && (
            <motion.div
              className="text-center mb-8"
              variants={staggerItem}
            >
              {title && (
                <h1 className="ailurus-heading text-2xl font-bold text-ailurus-cream mb-2">
                  {title}
                </h1>
              )}
              {subtitle && (
                <p className="text-semi-color-text-2 text-sm">
                  {subtitle}
                </p>
              )}
            </motion.div>
          )}

          {/* Form content */}
          <motion.div variants={staggerItem}>
            {children}
          </motion.div>
        </motion.div>
      </motion.div>
    </div>
  );
});

// ==================== Auth Divider ====================
export const AilurusAuthDivider = ({ text = 'or' }) => {
  return (
    <div className="relative my-6">
      <div className="absolute inset-0 flex items-center">
        <div className="w-full border-t border-white/10" />
      </div>
      <div className="relative flex justify-center text-sm">
        <span className="px-4 text-semi-color-text-2 bg-transparent">
          {text}
        </span>
      </div>
    </div>
  );
};

// ==================== OAuth Button ====================
export const AilurusOAuthButton = forwardRef(function AilurusOAuthButton(
  {
    icon,
    provider,
    onClick,
    loading = false,
    disabled = false,
    className,
    ...props
  },
  ref
) {
  return (
    <motion.button
      ref={ref}
      type="button"
      disabled={disabled || loading}
      onClick={onClick}
      className={clsx(
        // Base styles
        'w-full h-12 flex items-center justify-center gap-3',
        'rounded-xl',
        'font-medium text-sm',
        // Glass effect
        'bg-white/5 backdrop-blur-sm',
        'border border-white/10',
        // Text
        'text-ailurus-cream',
        // Transitions (non-transform, motion handles transform)
        'transition-colors duration-200',
        // Hover
        'hover:bg-white/8 hover:border-white/15',
        // Disabled
        (disabled || loading) && 'opacity-50 cursor-not-allowed',
        className
      )}
      whileHover={!disabled && !loading ? { scale: 1.01, y: -1 } : undefined}
      whileTap={!disabled && !loading ? { scale: 0.99 } : undefined}
      transition={springConfig.snappy}
      {...props}
    >
      {loading ? (
        <motion.div
          className="w-5 h-5 border-2 border-white/20 border-t-white rounded-full"
          animate={{ rotate: 360 }}
          transition={{ duration: 1, repeat: Infinity, ease: 'linear' }}
        />
      ) : (
        <>
          {icon && <span className="w-5 h-5 flex items-center justify-center">{icon}</span>}
          <span>Continue with {provider}</span>
        </>
      )}
    </motion.button>
  );
});

// ==================== Auth Link ====================
export const AilurusAuthLink = ({ children, to, className, ...props }) => {
  return (
    <motion.a
      href={to}
      className={clsx(
        'text-ailurus-rust-400 hover:text-ailurus-rust-300',
        'font-medium',
        'transition-colors duration-200',
        className
      )}
      whileHover={{ scale: 1.02 }}
      transition={springConfig.snappy}
      {...props}
    >
      {children}
    </motion.a>
  );
};

// ==================== Auth Footer ====================
export const AilurusAuthFooter = ({ children, className }) => {
  return (
    <motion.div
      className={clsx(
        'mt-6 text-center text-sm text-semi-color-text-2',
        className
      )}
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ delay: 0.5 }}
    >
      {children}
    </motion.div>
  );
};

export default AilurusAuthLayout;
