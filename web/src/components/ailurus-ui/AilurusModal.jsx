/*
 * AilurusModal - Animated Modal Component
 *
 * A beautiful modal with the Ailurus aesthetic:
 * - Glassmorphic backdrop
 * - Spring-based entrance/exit animations
 * - Luminous shadow effects
 * - Multiple sizes and variants
 */

import { motion, AnimatePresence } from 'framer-motion';
import { forwardRef, useEffect } from 'react';
import clsx from 'clsx';
import { springConfig, modalOverlayVariants, modalContentVariants } from './motion';
import AilurusButton from './AilurusButton';

/**
 * AilurusModal - Modal dialog with animations
 *
 * @param {object} props
 * @param {boolean} props.visible - Whether the modal is visible
 * @param {function} props.onClose - Close handler
 * @param {string} props.title - Modal title
 * @param {React.ReactNode} props.children - Modal content
 * @param {React.ReactNode} props.footer - Footer content (or use okText/cancelText)
 * @param {string} props.okText - OK button text
 * @param {string} props.cancelText - Cancel button text
 * @param {function} props.onOk - OK button handler
 * @param {boolean} props.okLoading - OK button loading state
 * @param {boolean} props.okDisabled - OK button disabled state
 * @param {string} props.size - Modal size: 'sm' | 'md' | 'lg' | 'xl' | 'full'
 * @param {boolean} props.closable - Show close button
 * @param {boolean} props.maskClosable - Close on mask click
 */
const AilurusModal = forwardRef(function AilurusModal(
  {
    visible,
    onClose,
    title,
    children,
    footer,
    okText = '确定',
    cancelText = '取消',
    onOk,
    okLoading = false,
    okDisabled = false,
    size = 'md',
    closable = true,
    maskClosable = true,
    className,
    ...props
  },
  ref
) {
  // Lock body scroll when modal is open
  useEffect(() => {
    if (visible) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }
    return () => {
      document.body.style.overflow = '';
    };
  }, [visible]);

  // Handle escape key
  useEffect(() => {
    const handleEscape = (e) => {
      if (e.key === 'Escape' && visible && closable) {
        onClose?.();
      }
    };
    window.addEventListener('keydown', handleEscape);
    return () => window.removeEventListener('keydown', handleEscape);
  }, [visible, closable, onClose]);

  // Size classes
  const sizeClasses = {
    sm: 'max-w-sm',
    md: 'max-w-lg',
    lg: 'max-w-2xl',
    xl: 'max-w-4xl',
    full: 'max-w-[90vw] max-h-[90vh]',
  };

  // Default footer
  const defaultFooter = (
    <div className="flex items-center justify-end gap-3">
      <AilurusButton variant="ghost" onClick={onClose}>
        {cancelText}
      </AilurusButton>
      <AilurusButton
        variant="primary"
        onClick={onOk}
        loading={okLoading}
        disabled={okDisabled}
      >
        {okText}
      </AilurusButton>
    </div>
  );

  return (
    <AnimatePresence>
      {visible && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          {/* Backdrop */}
          <motion.div
            className="absolute inset-0 bg-black/60 backdrop-blur-sm"
            variants={modalOverlayVariants}
            initial="initial"
            animate="animate"
            exit="exit"
            onClick={maskClosable ? onClose : undefined}
          />

          {/* Modal content */}
          <motion.div
            ref={ref}
            className={clsx(
              // Base styles
              'relative w-full',
              sizeClasses[size],
              // Glassmorphism
              'backdrop-blur-2xl',
              'bg-ailurus-obsidian/90',
              'border border-white/10',
              'rounded-2xl',
              // Shadow
              'shadow-[0_25px_80px_rgba(194,94,0,0.15),inset_0_1px_0_rgba(255,255,255,0.05)]',
              // Overflow
              'overflow-hidden',
              className
            )}
            variants={modalContentVariants}
            initial="initial"
            animate="animate"
            exit="exit"
            onClick={(e) => e.stopPropagation()}
            {...props}
          >
            {/* Header */}
            {(title || closable) && (
              <div className="flex items-center justify-between px-6 py-4 border-b border-white/5">
                {title && (
                  <h2 className="text-lg font-semibold text-ailurus-cream">
                    {title}
                  </h2>
                )}
                {closable && (
                  <motion.button
                    className="p-2 rounded-lg text-ailurus-cream/60 hover:text-ailurus-cream hover:bg-white/5 transition-colors"
                    onClick={onClose}
                    whileHover={{ scale: 1.1 }}
                    whileTap={{ scale: 0.95 }}
                    transition={springConfig.snappy}
                  >
                    <svg
                      className="w-5 h-5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M6 18L18 6M6 6l12 12"
                      />
                    </svg>
                  </motion.button>
                )}
              </div>
            )}

            {/* Body */}
            <div className="px-6 py-5 max-h-[60vh] overflow-y-auto">
              {children}
            </div>

            {/* Footer */}
            {(footer !== null) && (
              <div className="px-6 py-4 border-t border-white/5 bg-white/[0.02]">
                {footer !== undefined ? footer : defaultFooter}
              </div>
            )}

            {/* Decorative corner glow */}
            <div className="absolute -top-20 -right-20 w-40 h-40 bg-ailurus-rust-500/20 rounded-full blur-3xl pointer-events-none" />
          </motion.div>
        </div>
      )}
    </AnimatePresence>
  );
});

// ==================== Confirm Modal ====================
export const AilurusConfirmModal = forwardRef(function AilurusConfirmModal(
  {
    visible,
    onClose,
    onConfirm,
    title = '确认',
    content,
    confirmText = '确定',
    cancelText = '取消',
    type = 'warning', // 'warning' | 'danger' | 'info'
    loading = false,
    ...props
  },
  ref
) {
  const iconColors = {
    warning: 'text-yellow-400 bg-yellow-400/20',
    danger: 'text-red-400 bg-red-400/20',
    info: 'text-ailurus-teal-400 bg-ailurus-teal-400/20',
  };

  const buttonVariants = {
    warning: 'primary',
    danger: 'danger',
    info: 'teal',
  };

  return (
    <AilurusModal
      ref={ref}
      visible={visible}
      onClose={onClose}
      size="sm"
      footer={null}
      {...props}
    >
      <div className="text-center py-4">
        {/* Icon */}
        <motion.div
          className={clsx(
            'w-14 h-14 rounded-full mx-auto mb-4 flex items-center justify-center',
            iconColors[type]
          )}
          initial={{ scale: 0 }}
          animate={{ scale: 1 }}
          transition={springConfig.bouncy}
        >
          {type === 'warning' && (
            <svg className="w-7 h-7" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          )}
          {type === 'danger' && (
            <svg className="w-7 h-7" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
          )}
          {type === 'info' && (
            <svg className="w-7 h-7" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          )}
        </motion.div>

        {/* Title */}
        <h3 className="text-lg font-semibold text-ailurus-cream mb-2">{title}</h3>

        {/* Content */}
        {content && (
          <p className="text-sm text-ailurus-cream/60 mb-6">{content}</p>
        )}

        {/* Actions */}
        <div className="flex items-center justify-center gap-3">
          <AilurusButton variant="ghost" onClick={onClose}>
            {cancelText}
          </AilurusButton>
          <AilurusButton
            variant={buttonVariants[type]}
            onClick={onConfirm}
            loading={loading}
          >
            {confirmText}
          </AilurusButton>
        </div>
      </div>
    </AilurusModal>
  );
});

export default AilurusModal;
