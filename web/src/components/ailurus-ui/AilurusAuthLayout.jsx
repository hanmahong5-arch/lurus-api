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
    backgroundImage,
    className,
    ...props
  },
  ref
) {
  // Default background images - stunning abstract/tech visuals
  const defaultBgDark = 'https://images.unsplash.com/photo-1639322537228-f710d846310a?w=1920&q=80';
  const defaultBgLight = 'https://images.unsplash.com/photo-1557683316-973673baf926?w=1920&q=80';

  return (
    <div
      ref={ref}
      className={clsx(
        // Full screen container
        'min-h-screen w-full',
        'flex items-center justify-center',
        'p-4 sm:p-6 lg:p-8',
        // Theme-aware gradient background
        // Light mode: soft cream/warm gradient
        'bg-gradient-to-br from-gray-50 via-orange-50/30 to-gray-100',
        // Dark mode: forest gradient
        'dark:from-ailurus-obsidian dark:via-ailurus-forest dark:to-ailurus-obsidian-900',
        // Position for pseudo-elements
        'relative overflow-hidden',
        className
      )}
      {...props}
    >
      {/* Background image layer */}
      <div
        className="absolute inset-0 z-0"
        style={{
          backgroundImage: `url(${backgroundImage || defaultBgDark})`,
          backgroundSize: 'cover',
          backgroundPosition: 'center',
          backgroundRepeat: 'no-repeat',
        }}
      />
      {/* Dark overlay for better contrast */}
      <div className="absolute inset-0 z-0 bg-ailurus-forest/85 dark:bg-ailurus-obsidian/80 backdrop-blur-sm" />
      {/* Light mode: use light background image */}
      <div
        className="absolute inset-0 z-0 dark:hidden"
        style={{
          backgroundImage: `url(${defaultBgLight})`,
          backgroundSize: 'cover',
          backgroundPosition: 'center',
          backgroundRepeat: 'no-repeat',
        }}
      />
      <div className="absolute inset-0 z-0 bg-white/75 dark:hidden backdrop-blur-sm" />
      {/* Noise texture overlay */}
      <div
        className="absolute inset-0 pointer-events-none z-0 opacity-[0.03]"
        style={{
          backgroundImage: `url("data:image/svg+xml,%3Csvg viewBox='0 0 256 256' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='noise'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.8' numOctaves='4' stitchTiles='stitch'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23noise)'/%3E%3C/svg%3E")`,
        }}
      />

      {/* Aurora gradient animated background blur balls */}
      {/* Teal bubble - top left */}
      <motion.div
        className="absolute top-[-15%] left-[-10%] w-[450px] h-[450px] rounded-full bg-ailurus-teal-500/25 blur-[100px]"
        animate={{
          scale: [1, 1.15, 1],
          opacity: [0.2, 0.35, 0.2],
          x: [0, 20, 0],
          y: [0, -15, 0],
        }}
        transition={{
          duration: 10,
          repeat: Infinity,
          ease: 'easeInOut',
        }}
      />
      {/* Purple bubble - center right */}
      <motion.div
        className="absolute top-[30%] right-[-15%] w-[500px] h-[500px] rounded-full bg-ailurus-purple-500/20 blur-[120px]"
        animate={{
          scale: [1, 1.1, 1],
          opacity: [0.15, 0.28, 0.15],
          x: [0, -25, 0],
          y: [0, 20, 0],
        }}
        transition={{
          duration: 12,
          repeat: Infinity,
          ease: 'easeInOut',
          delay: 0.5,
        }}
      />
      {/* Rust bubble - bottom center */}
      <motion.div
        className="absolute bottom-[-20%] left-[20%] w-[550px] h-[550px] rounded-full bg-ailurus-rust-500/18 blur-[130px]"
        animate={{
          scale: [1, 1.12, 1],
          opacity: [0.12, 0.22, 0.12],
          x: [0, 30, 0],
          y: [0, -25, 0],
        }}
        transition={{
          duration: 14,
          repeat: Infinity,
          ease: 'easeInOut',
          delay: 1,
        }}
      />
      {/* Small accent bubble - floating */}
      <motion.div
        className="absolute top-[60%] left-[5%] w-[200px] h-[200px] rounded-full bg-ailurus-teal-400/15 blur-[60px]"
        animate={{
          scale: [1, 1.3, 1],
          opacity: [0.1, 0.2, 0.1],
          y: [0, -30, 0],
        }}
        transition={{
          duration: 8,
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
              <span className="ailurus-heading text-2xl font-bold text-gray-900 dark:ailurus-aurora-text">
                {systemName}
              </span>
            )}
          </motion.div>
        )}

        {/* Glass card container with Aurora border */}
        <motion.div
          className={clsx(
            // Glassmorphism - theme aware
            'backdrop-blur-xl',
            // Light mode: white glass with subtle shadow
            'bg-white/90 border border-gray-200/50',
            'shadow-[0_20px_60px_rgba(0,0,0,0.08)]',
            // Dark mode: dark glass with aurora glow
            'dark:bg-white/[0.04] dark:border-transparent',
            'dark:shadow-[0_20px_60px_rgba(139,92,246,0.12),0_0_40px_rgba(6,182,212,0.08),inset_0_1px_0_rgba(255,255,255,0.1)]',
            'rounded-3xl',
            // Padding
            'p-8 sm:p-10',
            // Position for aurora border
            'relative overflow-hidden'
          )}
          variants={staggerItem}
        >
          {/* Aurora gradient border for dark mode */}
          <div className="absolute inset-0 rounded-3xl p-[1px] pointer-events-none dark:block hidden">
            <div
              className="absolute inset-0 rounded-3xl opacity-40"
              style={{
                background: 'linear-gradient(135deg, rgba(6,182,212,0.5) 0%, rgba(139,92,246,0.5) 50%, rgba(194,94,0,0.5) 100%)',
                WebkitMask: 'linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0)',
                WebkitMaskComposite: 'xor',
                maskComposite: 'exclude',
                padding: '1px',
              }}
            />
          </div>
          {/* Title section */}
          {(title || subtitle) && (
            <motion.div
              className="text-center mb-8"
              variants={staggerItem}
            >
              {title && (
                <h1 className="ailurus-heading text-2xl font-bold text-gray-900 dark:text-ailurus-cream mb-2">
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
        <div className="w-full border-t border-gray-200 dark:border-white/10" />
      </div>
      <div className="relative flex justify-center text-sm">
        <span className="px-4 text-semi-color-text-2 bg-white dark:bg-transparent">
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
        // Glass effect - theme aware
        // Light mode
        'bg-gray-100 border border-gray-200',
        'text-gray-700',
        'hover:bg-gray-200 hover:border-gray-300',
        // Dark mode
        'dark:bg-white/5 dark:backdrop-blur-sm',
        'dark:border-white/10',
        'dark:text-ailurus-cream',
        'dark:hover:bg-white/8 dark:hover:border-white/15',
        // Transitions (non-transform, motion handles transform)
        'transition-colors duration-200',
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
