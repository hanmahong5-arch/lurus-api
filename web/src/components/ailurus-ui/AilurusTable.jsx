/*
 * AilurusTable - Animated Table Component
 *
 * A beautiful table with the Ailurus aesthetic:
 * - Glassmorphic styling with luminous effects
 * - Row hover animations
 * - Staggered row entrance
 * - Responsive design
 */

import { motion } from 'framer-motion';
import { forwardRef } from 'react';
import clsx from 'clsx';
import { springConfig, staggerContainer } from './motion';

/**
 * AilurusTable - Data table with animations
 *
 * @param {object} props
 * @param {Array} props.columns - Column definitions: { key, title, dataIndex, render, width, align }
 * @param {Array} props.data - Data rows
 * @param {string} props.rowKey - Key field for row identification
 * @param {boolean} props.loading - Loading state
 * @param {boolean} props.striped - Striped rows
 * @param {boolean} props.hoverable - Hover effects
 * @param {function} props.onRowClick - Row click handler
 * @param {React.ReactNode} props.emptyText - Empty state content
 */
const AilurusTable = forwardRef(function AilurusTable(
  {
    columns = [],
    data = [],
    rowKey = 'id',
    loading = false,
    striped = false,
    hoverable = true,
    onRowClick,
    emptyText = '暂无数据',
    className,
    ...props
  },
  ref
) {
  // Row animation variants
  const rowVariants = {
    initial: { opacity: 0, y: 10 },
    animate: (i) => ({
      opacity: 1,
      y: 0,
      transition: {
        delay: i * 0.05,
        ...springConfig.snappy,
      },
    }),
  };

  // Loading skeleton row
  const SkeletonRow = () => (
    <tr>
      {columns.map((col, i) => (
        <td key={i} className="px-4 py-3">
          <div className="h-4 bg-gray-200 dark:bg-white/10 rounded animate-pulse" />
        </td>
      ))}
    </tr>
  );

  // Empty state
  const EmptyState = () => (
    <tr>
      <td colSpan={columns.length} className="px-4 py-12 text-center">
        <div className="flex flex-col items-center text-gray-400 dark:text-ailurus-cream/40">
          <svg
            className="w-12 h-12 mb-3"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={1.5}
              d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
            />
          </svg>
          <span className="text-sm">{emptyText}</span>
        </div>
      </td>
    </tr>
  );

  return (
    <div
      ref={ref}
      className={clsx(
        'overflow-hidden rounded-xl',
        'border border-gray-200 dark:border-white/10',
        'bg-gray-50/50 dark:bg-white/[0.02]',
        'backdrop-blur-sm',
        className
      )}
      {...props}
    >
      <div className="overflow-x-auto">
        <table className="w-full">
          {/* Table Header */}
          <thead>
            <tr className="border-b border-gray-200 dark:border-white/10 bg-gray-100/50 dark:bg-white/[0.03]">
              {columns.map((col) => (
                <th
                  key={col.key || col.dataIndex}
                  className={clsx(
                    'px-4 py-3 text-left text-xs font-semibold uppercase tracking-wider text-gray-700 dark:text-ailurus-cream/70',
                    col.align === 'center' && 'text-center',
                    col.align === 'right' && 'text-right'
                  )}
                  style={{ width: col.width }}
                >
                  {col.title}
                </th>
              ))}
            </tr>
          </thead>

          {/* Table Body */}
          <tbody>
            {loading ? (
              // Loading skeletons
              [...Array(5)].map((_, i) => <SkeletonRow key={i} />)
            ) : data.length === 0 ? (
              // Empty state
              <EmptyState />
            ) : (
              // Data rows
              data.map((row, rowIndex) => (
                <motion.tr
                  key={row[rowKey] || rowIndex}
                  className={clsx(
                    'border-b border-gray-100 dark:border-white/5 transition-colors',
                    striped && rowIndex % 2 === 1 && 'bg-gray-50 dark:bg-white/[0.02]',
                    hoverable && 'hover:bg-gray-100 dark:hover:bg-white/[0.05]',
                    onRowClick && 'cursor-pointer'
                  )}
                  variants={rowVariants}
                  initial="initial"
                  animate="animate"
                  custom={rowIndex}
                  onClick={() => onRowClick?.(row, rowIndex)}
                  whileHover={
                    hoverable
                      ? {
                          x: 2,
                          transition: springConfig.snappy,
                        }
                      : undefined
                  }
                >
                  {columns.map((col) => (
                    <td
                      key={col.key || col.dataIndex}
                      className={clsx(
                        'px-4 py-3 text-sm text-gray-900 dark:text-ailurus-cream/80',
                        col.align === 'center' && 'text-center',
                        col.align === 'right' && 'text-right'
                      )}
                    >
                      {col.render
                        ? col.render(row[col.dataIndex], row, rowIndex)
                        : row[col.dataIndex]}
                    </td>
                  ))}
                </motion.tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
});

// ==================== Table Cell Components ====================

export const AilurusTableTag = forwardRef(function AilurusTableTag(
  { children, variant = 'default', className, ...props },
  ref
) {
  const variants = {
    default: 'bg-gray-100 dark:bg-white/10 text-gray-700 dark:text-ailurus-cream/80',
    success: 'bg-green-100 dark:bg-green-500/20 text-green-700 dark:text-green-400',
    warning: 'bg-yellow-100 dark:bg-yellow-500/20 text-yellow-700 dark:text-yellow-400',
    danger: 'bg-red-100 dark:bg-red-500/20 text-red-700 dark:text-red-400',
    info: 'bg-ailurus-teal-100 dark:bg-ailurus-teal-500/20 text-ailurus-teal-700 dark:text-ailurus-teal-400',
    rust: 'bg-ailurus-rust-100 dark:bg-ailurus-rust-500/20 text-ailurus-rust-700 dark:text-ailurus-rust-400',
    purple: 'bg-ailurus-purple-100 dark:bg-ailurus-purple-500/20 text-ailurus-purple-700 dark:text-ailurus-purple-400',
  };

  return (
    <span
      ref={ref}
      className={clsx(
        'inline-flex items-center px-2.5 py-0.5 rounded-md text-xs font-medium',
        variants[variant],
        className
      )}
      {...props}
    >
      {children}
    </span>
  );
});

export const AilurusTableAvatar = forwardRef(function AilurusTableAvatar(
  { src, alt, name, size = 'md', className, ...props },
  ref
) {
  const sizes = {
    sm: 'w-6 h-6 text-xs',
    md: 'w-8 h-8 text-sm',
    lg: 'w-10 h-10 text-base',
  };

  const initials = name
    ?.split(' ')
    .map((n) => n[0])
    .join('')
    .toUpperCase()
    .slice(0, 2);

  return (
    <div
      ref={ref}
      className={clsx(
        'rounded-full flex items-center justify-center overflow-hidden',
        'bg-ailurus-rust-500/20 text-ailurus-rust-400 font-medium',
        sizes[size],
        className
      )}
      {...props}
    >
      {src ? (
        <img src={src} alt={alt || name} className="w-full h-full object-cover" />
      ) : (
        initials
      )}
    </div>
  );
});

export const AilurusTableActions = forwardRef(function AilurusTableActions(
  { children, className, ...props },
  ref
) {
  return (
    <div
      ref={ref}
      className={clsx('flex items-center gap-2', className)}
      {...props}
    >
      {children}
    </div>
  );
});

export const AilurusTableActionButton = forwardRef(function AilurusTableActionButton(
  { children, variant = 'default', onClick, className, ...props },
  ref
) {
  const variants = {
    default: 'text-gray-500 dark:text-ailurus-cream/60 hover:text-gray-900 dark:hover:text-ailurus-cream hover:bg-gray-100 dark:hover:bg-white/10',
    danger: 'text-red-400/60 hover:text-red-600 dark:hover:text-red-400 hover:bg-red-50 dark:hover:bg-red-500/10',
    primary: 'text-ailurus-rust-500/60 dark:text-ailurus-rust-400/60 hover:text-ailurus-rust-600 dark:hover:text-ailurus-rust-400 hover:bg-ailurus-rust-50 dark:hover:bg-ailurus-rust-500/10',
  };

  return (
    <motion.button
      ref={ref}
      className={clsx(
        'p-1.5 rounded-lg transition-colors',
        variants[variant],
        className
      )}
      onClick={onClick}
      whileHover={{ scale: 1.1 }}
      whileTap={{ scale: 0.95 }}
      transition={springConfig.snappy}
      {...props}
    >
      {children}
    </motion.button>
  );
});

export default AilurusTable;
