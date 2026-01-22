/*
 * Ailurus Motion System
 *
 * Framer-motion variants and utilities for consistent animations
 * across the Ailurus Design System.
 *
 * Core principle: "Refuse instant changes" - everything must
 * have elastic entrance animations and interaction feedback.
 */

// ==================== Spring Configurations ====================
// Physics-based spring configs for natural motion

export const springConfig = {
  // Default spring - balanced and smooth
  default: {
    type: 'spring',
    stiffness: 400,
    damping: 30,
  },
  // Snappy spring - quick response
  snappy: {
    type: 'spring',
    stiffness: 500,
    damping: 25,
  },
  // Soft spring - gentle and elastic
  soft: {
    type: 'spring',
    stiffness: 200,
    damping: 20,
  },
  // Bouncy spring - playful with overshoot
  bouncy: {
    type: 'spring',
    stiffness: 300,
    damping: 15,
  },
};

// ==================== Entrance Variants ====================
// Use these for elements appearing on the page

export const fadeIn = {
  initial: { opacity: 0 },
  animate: {
    opacity: 1,
    transition: { duration: 0.4, ease: [0.16, 1, 0.3, 1] }
  },
  exit: {
    opacity: 0,
    transition: { duration: 0.2 }
  },
};

export const slideUp = {
  initial: { opacity: 0, y: 20 },
  animate: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.5,
      ease: [0.16, 1, 0.3, 1]
    }
  },
  exit: {
    opacity: 0,
    y: 10,
    transition: { duration: 0.2 }
  },
};

export const slideDown = {
  initial: { opacity: 0, y: -20 },
  animate: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.5,
      ease: [0.16, 1, 0.3, 1]
    }
  },
  exit: {
    opacity: 0,
    y: -10,
    transition: { duration: 0.2 }
  },
};

export const scaleIn = {
  initial: { opacity: 0, scale: 0.9 },
  animate: {
    opacity: 1,
    scale: 1,
    transition: springConfig.default
  },
  exit: {
    opacity: 0,
    scale: 0.95,
    transition: { duration: 0.15 }
  },
};

export const bounceIn = {
  initial: { opacity: 0, scale: 0.3 },
  animate: {
    opacity: 1,
    scale: 1,
    transition: springConfig.bouncy
  },
  exit: {
    opacity: 0,
    scale: 0.5,
    transition: { duration: 0.2 }
  },
};

// ==================== Stagger Container Variants ====================
// Use these for lists and grids of items

export const staggerContainer = {
  initial: {},
  animate: {
    transition: {
      staggerChildren: 0.05,
      delayChildren: 0.1,
    },
  },
  exit: {
    transition: {
      staggerChildren: 0.03,
      staggerDirection: -1,
    },
  },
};

export const staggerContainerSlow = {
  initial: {},
  animate: {
    transition: {
      staggerChildren: 0.1,
      delayChildren: 0.2,
    },
  },
};

export const staggerItem = {
  initial: { opacity: 0, y: 16 },
  animate: {
    opacity: 1,
    y: 0,
    transition: {
      duration: 0.5,
      ease: [0.16, 1, 0.3, 1],
    }
  },
  exit: {
    opacity: 0,
    y: 8,
    transition: { duration: 0.15 }
  },
};

// ==================== Hover Variants ====================
// Use these for interactive elements

export const hoverScale = {
  scale: 1.02,
  transition: springConfig.snappy,
};

export const hoverScaleSmall = {
  scale: 1.01,
  transition: springConfig.snappy,
};

export const hoverLift = {
  y: -4,
  transition: springConfig.default,
};

// ==================== Tap Variants ====================
// Use these for button press feedback

export const tapScale = {
  scale: 0.98,
  transition: springConfig.snappy,
};

export const tapBounce = {
  scale: 0.95,
  transition: springConfig.bouncy,
};

// ==================== Button Variants ====================
// Combined hover and tap for buttons

export const buttonVariants = {
  initial: { opacity: 0, y: 10 },
  animate: {
    opacity: 1,
    y: 0,
    transition: springConfig.default
  },
  hover: {
    scale: 1.02,
    y: -1,
    transition: springConfig.snappy
  },
  tap: {
    scale: 0.98,
    y: 0,
    transition: springConfig.snappy
  },
};

export const buttonVariantsSubtle = {
  hover: {
    scale: 1.01,
    transition: springConfig.snappy
  },
  tap: {
    scale: 0.99,
    transition: springConfig.snappy
  },
};

// ==================== Card Variants ====================
// For glass panel cards with hover effects

export const cardVariants = {
  initial: { opacity: 0, y: 20, scale: 0.98 },
  animate: {
    opacity: 1,
    y: 0,
    scale: 1,
    transition: {
      duration: 0.5,
      ease: [0.16, 1, 0.3, 1],
    }
  },
  hover: {
    y: -4,
    scale: 1.01,
    transition: springConfig.default
  },
  tap: {
    scale: 0.99,
    transition: springConfig.snappy
  },
  exit: {
    opacity: 0,
    y: 10,
    scale: 0.98,
    transition: { duration: 0.2 }
  },
};

// ==================== Modal Variants ====================
// For dialog/modal overlays

export const modalOverlayVariants = {
  initial: { opacity: 0 },
  animate: {
    opacity: 1,
    transition: { duration: 0.2 }
  },
  exit: {
    opacity: 0,
    transition: { duration: 0.15 }
  },
};

export const modalContentVariants = {
  initial: { opacity: 0, scale: 0.95, y: 20 },
  animate: {
    opacity: 1,
    scale: 1,
    y: 0,
    transition: springConfig.default
  },
  exit: {
    opacity: 0,
    scale: 0.95,
    y: 10,
    transition: { duration: 0.15 }
  },
};

// ==================== Input Focus Variants ====================
// For form inputs when focused

export const inputFocusVariants = {
  unfocused: {
    boxShadow: '0 0 0 0px rgba(194, 94, 0, 0)',
    borderColor: 'rgba(255, 255, 255, 0.1)',
  },
  focused: {
    boxShadow: '0 0 0 3px rgba(194, 94, 0, 0.15)',
    borderColor: '#E67E22',
    transition: springConfig.snappy,
  },
};

// ==================== Page Transition Variants ====================
// For full page transitions

export const pageVariants = {
  initial: { opacity: 0, x: -10 },
  animate: {
    opacity: 1,
    x: 0,
    transition: {
      duration: 0.4,
      ease: [0.16, 1, 0.3, 1],
    }
  },
  exit: {
    opacity: 0,
    x: 10,
    transition: { duration: 0.2 }
  },
};

// ==================== Utility Functions ====================

/**
 * Creates a stagger delay for individual items in a list
 * @param {number} index - The item's index in the list
 * @param {number} baseDelay - Base delay in seconds (default: 0.05)
 * @returns {object} Transition object with calculated delay
 */
export const getStaggerDelay = (index, baseDelay = 0.05) => ({
  delay: index * baseDelay,
});

/**
 * Combines multiple variants into one
 * @param {...object} variants - Variant objects to merge
 * @returns {object} Merged variant object
 */
export const combineVariants = (...variants) => {
  return variants.reduce((acc, variant) => ({
    ...acc,
    ...variant,
  }), {});
};

// ==================== Default Export ====================
// Export all variants as a single object for convenience

const ailurusMotion = {
  spring: springConfig,
  variants: {
    fadeIn,
    slideUp,
    slideDown,
    scaleIn,
    bounceIn,
    staggerContainer,
    staggerContainerSlow,
    staggerItem,
    button: buttonVariants,
    buttonSubtle: buttonVariantsSubtle,
    card: cardVariants,
    modalOverlay: modalOverlayVariants,
    modalContent: modalContentVariants,
    inputFocus: inputFocusVariants,
    page: pageVariants,
  },
  hover: {
    scale: hoverScale,
    scaleSmall: hoverScaleSmall,
    lift: hoverLift,
  },
  tap: {
    scale: tapScale,
    bounce: tapBounce,
  },
  utils: {
    getStaggerDelay,
    combineVariants,
  },
};

export default ailurusMotion;
