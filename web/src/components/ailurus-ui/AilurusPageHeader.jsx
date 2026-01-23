/*
 * AilurusPageHeader - Page Header Component
 *
 * A beautiful page header with the Ailurus aesthetic:
 * - Animated title and description
 * - Action buttons area
 * - Breadcrumb support
 * - Gradient underline accent
 */

import { motion } from 'framer-motion';
import { forwardRef } from 'react';
import clsx from 'clsx';
import { springConfig, staggerContainer, staggerItem } from './motion';

/**
 * AilurusPageHeader - Page header with title, description and actions
 *
 * @param {object} props
 * @param {string} props.title - Page title
 * @param {string} props.description - Optional description
 * @param {React.ReactNode} props.actions - Action buttons
 * @param {React.ReactNode} props.breadcrumb - Breadcrumb navigation
 * @param {React.ReactNode} props.icon - Optional icon
 * @param {boolean} props.withDivider - Show bottom divider
 */
const AilurusPageHeader = forwardRef(function AilurusPageHeader(
  {
    title,
    description,
    actions,
    breadcrumb,
    icon,
    withDivider = true,
    className,
    ...props
  },
  ref
) {
  return (
    <motion.div
      ref={ref}
      className={clsx('relative mb-6', className)}
      variants={staggerContainer}
      initial="initial"
      animate="animate"
      {...props}
    >
      {/* Breadcrumb */}
      {breadcrumb && (
        <motion.div
          className="mb-3"
          variants={staggerItem}
        >
          {breadcrumb}
        </motion.div>
      )}

      {/* Main header content */}
      <div className="flex items-start justify-between gap-4">
        {/* Left: Title and description */}
        <div className="flex-1">
          <div className="flex items-center gap-3">
            {icon && (
              <motion.div
                className="w-10 h-10 rounded-xl bg-ailurus-rust-500/20 flex items-center justify-center text-ailurus-rust-500 dark:text-ailurus-rust-400"
                variants={staggerItem}
                whileHover={{ scale: 1.1, rotate: 5 }}
                transition={springConfig.bouncy}
              >
                {icon}
              </motion.div>
            )}
            <motion.h1
              className="ailurus-heading text-2xl font-bold text-gray-900 dark:text-ailurus-cream"
              variants={staggerItem}
            >
              {title}
            </motion.h1>
          </div>

          {description && (
            <motion.p
              className="mt-2 text-sm text-gray-600 dark:text-ailurus-cream/60 max-w-2xl"
              variants={staggerItem}
            >
              {description}
            </motion.p>
          )}
        </div>

        {/* Right: Actions */}
        {actions && (
          <motion.div
            className="flex items-center gap-3 flex-shrink-0"
            variants={staggerItem}
          >
            {actions}
          </motion.div>
        )}
      </div>

      {/* Divider with gradient */}
      {withDivider && (
        <motion.div
          className="mt-5 h-px bg-gradient-to-r from-ailurus-rust-500/50 via-transparent to-transparent"
          initial={{ scaleX: 0, originX: 0 }}
          animate={{ scaleX: 1 }}
          transition={{ delay: 0.3, duration: 0.5, ease: 'easeOut' }}
        />
      )}
    </motion.div>
  );
});

// ==================== Breadcrumb ====================
export const AilurusBreadcrumb = forwardRef(function AilurusBreadcrumb(
  { items, className, ...props },
  ref
) {
  return (
    <nav
      ref={ref}
      className={clsx('flex items-center gap-2 text-sm', className)}
      {...props}
    >
      {items.map((item, index) => (
        <div key={index} className="flex items-center gap-2">
          {index > 0 && (
            <span className="text-gray-300 dark:text-ailurus-cream/30">/</span>
          )}
          {item.href ? (
            <motion.a
              href={item.href}
              className="text-gray-500 dark:text-ailurus-cream/60 hover:text-ailurus-rust-500 dark:hover:text-ailurus-rust-400 transition-colors"
              whileHover={{ x: 2 }}
              transition={springConfig.snappy}
            >
              {item.label}
            </motion.a>
          ) : (
            <span className="text-gray-900 dark:text-ailurus-cream">{item.label}</span>
          )}
        </div>
      ))}
    </nav>
  );
});

// ==================== Section Header ====================
export const AilurusSectionHeader = forwardRef(function AilurusSectionHeader(
  {
    title,
    description,
    actions,
    className,
    ...props
  },
  ref
) {
  return (
    <motion.div
      ref={ref}
      className={clsx('flex items-center justify-between mb-4', className)}
      initial={{ opacity: 0, y: 10 }}
      animate={{ opacity: 1, y: 0 }}
      transition={springConfig.snappy}
      {...props}
    >
      <div>
        <h3 className="text-lg font-semibold text-gray-900 dark:text-ailurus-cream">{title}</h3>
        {description && (
          <p className="text-sm text-gray-500 dark:text-ailurus-cream/50 mt-0.5">{description}</p>
        )}
      </div>
      {actions && (
        <div className="flex items-center gap-2">{actions}</div>
      )}
    </motion.div>
  );
});

export default AilurusPageHeader;
