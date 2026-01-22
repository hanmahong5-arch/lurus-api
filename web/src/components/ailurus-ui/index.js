/*
 * Ailurus UI Component Library
 *
 * A design system implementing the "Ailurus" aesthetic:
 * - High-End Comfort meets Cyberpunk Forest
 * - Red Panda themed color palette
 * - Glassmorphism + Luminous shadows
 * - Spring-based animations with framer-motion
 *
 * Core Principles:
 * 1. "Refuse instant changes" - everything animates
 * 2. "Luminous Depth" - colored shadows, never black
 * 3. "Organic Texture" - noise overlay, no plastic feel
 */

// ==================== Components ====================
export { default as AilurusCard } from './AilurusCard';
export {
  AilurusCardHeader,
  AilurusCardTitle,
  AilurusCardDescription,
  AilurusCardContent,
  AilurusCardFooter,
  AilurusCardGroup,
} from './AilurusCard';

export { default as AilurusButton } from './AilurusButton';
export { AilurusButtonGroup, AilurusIconButton } from './AilurusButton';

export { default as AilurusInput } from './AilurusInput';
export { AilurusTextarea } from './AilurusInput';

export { default as AilurusAuthLayout } from './AilurusAuthLayout';
export {
  AilurusAuthDivider,
  AilurusOAuthButton,
  AilurusAuthLink,
  AilurusAuthFooter,
} from './AilurusAuthLayout';

export { default as AilurusStatCard } from './AilurusStatCard';
export {
  AilurusStatCardGroup,
  AilurusMiniStatCard,
} from './AilurusStatCard';

export { default as AilurusPageHeader } from './AilurusPageHeader';
export {
  AilurusBreadcrumb,
  AilurusSectionHeader,
} from './AilurusPageHeader';

export { default as AilurusModal } from './AilurusModal';
export { AilurusConfirmModal } from './AilurusModal';

export { default as AilurusTabs } from './AilurusTabs';
export { AilurusTabPane } from './AilurusTabs';

export { default as AilurusTable } from './AilurusTable';
export {
  AilurusTableTag,
  AilurusTableAvatar,
  AilurusTableActions,
  AilurusTableActionButton,
} from './AilurusTable';

// ==================== Motion System ====================
export { default as ailurusMotion } from './motion';
export {
  // Spring configurations
  springConfig,
  // Entrance variants
  fadeIn,
  slideUp,
  slideDown,
  scaleIn,
  bounceIn,
  // Stagger variants
  staggerContainer,
  staggerContainerSlow,
  staggerItem,
  // Interactive variants
  hoverScale,
  hoverScaleSmall,
  hoverLift,
  tapScale,
  tapBounce,
  // Component variants
  buttonVariants,
  buttonVariantsSubtle,
  cardVariants,
  modalOverlayVariants,
  modalContentVariants,
  inputFocusVariants,
  pageVariants,
  // Utilities
  getStaggerDelay,
  combineVariants,
} from './motion';
