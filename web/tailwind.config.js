/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

export default {
  content: ['./index.html', './src/**/*.{js,jsx,ts,tsx}'],
  theme: {
    // ========================= Ailurus Design System =========================
    // The Red Panda aesthetic: High-End Comfort meets Cyberpunk Forest
    colors: {
      // Ailurus Primary Palette (Red Panda Fur)
      'ailurus-rust': {
        DEFAULT: '#C25E00',
        50: '#FFF7ED',
        100: '#FFEDD5',
        200: '#FED7AA',
        300: '#FDBA74',
        400: '#FB923C',
        500: '#E67E22',
        600: '#C25E00',
        700: '#9A3412',
        800: '#7C2D12',
        900: '#431407',
      },
      // Ailurus Secondary (Darkness - The Forest)
      'ailurus-obsidian': {
        DEFAULT: '#1A1A1A',
        50: '#404040',
        100: '#363636',
        200: '#2D2D2D',
        300: '#242424',
        400: '#1F1F1F',
        500: '#1A1A1A',
        600: '#141414',
        700: '#0F0F0F',
        800: '#0A0A0A',
        900: '#050505',
      },
      'ailurus-forest': {
        DEFAULT: '#0F172A',
        50: '#1E293B',
        100: '#1A2438',
        200: '#162033',
        300: '#131C2E',
        400: '#101829',
        500: '#0F172A',
        600: '#0C1322',
        700: '#090F1A',
        800: '#060A12',
        900: '#03050A',
      },
      // Ailurus Accents (Face/Paws)
      'ailurus-cream': {
        DEFAULT: '#FDFBF7',
        50: '#FFFFFF',
        100: '#FEFEFE',
        200: '#FDFCFA',
        300: '#FDFBF7',
        400: '#FAF5ED',
        500: '#F7EFE3',
      },
      // Tech Layer Accents
      'ailurus-teal': {
        DEFAULT: '#06B6D4',
        50: '#ECFEFF',
        100: '#CFFAFE',
        200: '#A5F3FC',
        300: '#67E8F9',
        400: '#22D3EE',
        500: '#06B6D4',
        600: '#0891B2',
        700: '#0E7490',
        800: '#155E75',
        900: '#164E63',
      },
      'ailurus-purple': {
        DEFAULT: '#8B5CF6',
        50: '#F5F3FF',
        100: '#EDE9FE',
        200: '#DDD6FE',
        300: '#C4B5FD',
        400: '#A78BFA',
        500: '#8B5CF6',
        600: '#7C3AED',
        700: '#6D28D9',
        800: '#5B21B6',
        900: '#4C1D95',
      },
      // Standard Semi UI color mappings
      'semi-color-white': 'var(--semi-color-white)',
      'semi-color-black': 'var(--semi-color-black)',
      'semi-color-primary': 'var(--semi-color-primary)',
      'semi-color-primary-hover': 'var(--semi-color-primary-hover)',
      'semi-color-primary-active': 'var(--semi-color-primary-active)',
      'semi-color-primary-disabled': 'var(--semi-color-primary-disabled)',
      'semi-color-primary-light-default':
        'var(--semi-color-primary-light-default)',
      'semi-color-primary-light-hover': 'var(--semi-color-primary-light-hover)',
      'semi-color-primary-light-active':
        'var(--semi-color-primary-light-active)',
      'semi-color-secondary': 'var(--semi-color-secondary)',
      'semi-color-secondary-hover': 'var(--semi-color-secondary-hover)',
      'semi-color-secondary-active': 'var(--semi-color-secondary-active)',
      'semi-color-secondary-disabled': 'var(--semi-color-secondary-disabled)',
      'semi-color-secondary-light-default':
        'var(--semi-color-secondary-light-default)',
      'semi-color-secondary-light-hover':
        'var(--semi-color-secondary-light-hover)',
      'semi-color-secondary-light-active':
        'var(--semi-color-secondary-light-active)',
      'semi-color-tertiary': 'var(--semi-color-tertiary)',
      'semi-color-tertiary-hover': 'var(--semi-color-tertiary-hover)',
      'semi-color-tertiary-active': 'var(--semi-color-tertiary-active)',
      'semi-color-tertiary-light-default':
        'var(--semi-color-tertiary-light-default)',
      'semi-color-tertiary-light-hover':
        'var(--semi-color-tertiary-light-hover)',
      'semi-color-tertiary-light-active':
        'var(--semi-color-tertiary-light-active)',
      'semi-color-default': 'var(--semi-color-default)',
      'semi-color-default-hover': 'var(--semi-color-default-hover)',
      'semi-color-default-active': 'var(--semi-color-default-active)',
      'semi-color-info': 'var(--semi-color-info)',
      'semi-color-info-hover': 'var(--semi-color-info-hover)',
      'semi-color-info-active': 'var(--semi-color-info-active)',
      'semi-color-info-disabled': 'var(--semi-color-info-disabled)',
      'semi-color-info-light-default': 'var(--semi-color-info-light-default)',
      'semi-color-info-light-hover': 'var(--semi-color-info-light-hover)',
      'semi-color-info-light-active': 'var(--semi-color-info-light-active)',
      'semi-color-success': 'var(--semi-color-success)',
      'semi-color-success-hover': 'var(--semi-color-success-hover)',
      'semi-color-success-active': 'var(--semi-color-success-active)',
      'semi-color-success-disabled': 'var(--semi-color-success-disabled)',
      'semi-color-success-light-default':
        'var(--semi-color-success-light-default)',
      'semi-color-success-light-hover': 'var(--semi-color-success-light-hover)',
      'semi-color-success-light-active':
        'var(--semi-color-success-light-active)',
      'semi-color-danger': 'var(--semi-color-danger)',
      'semi-color-danger-hover': 'var(--semi-color-danger-hover)',
      'semi-color-danger-active': 'var(--semi-color-danger-active)',
      'semi-color-danger-light-default':
        'var(--semi-color-danger-light-default)',
      'semi-color-danger-light-hover': 'var(--semi-color-danger-light-hover)',
      'semi-color-danger-light-active': 'var(--semi-color-danger-light-active)',
      'semi-color-warning': 'var(--semi-color-warning)',
      'semi-color-warning-hover': 'var(--semi-color-warning-hover)',
      'semi-color-warning-active': 'var(--semi-color-warning-active)',
      'semi-color-warning-light-default':
        'var(--semi-color-warning-light-default)',
      'semi-color-warning-light-hover': 'var(--semi-color-warning-light-hover)',
      'semi-color-warning-light-active':
        'var(--semi-color-warning-light-active)',
      'semi-color-focus-border': 'var(--semi-color-focus-border)',
      'semi-color-disabled-text': 'var(--semi-color-disabled-text)',
      'semi-color-disabled-border': 'var(--semi-color-disabled-border)',
      'semi-color-disabled-bg': 'var(--semi-color-disabled-bg)',
      'semi-color-disabled-fill': 'var(--semi-color-disabled-fill)',
      'semi-color-shadow': 'var(--semi-color-shadow)',
      'semi-color-link': 'var(--semi-color-link)',
      'semi-color-link-hover': 'var(--semi-color-link-hover)',
      'semi-color-link-active': 'var(--semi-color-link-active)',
      'semi-color-link-visited': 'var(--semi-color-link-visited)',
      'semi-color-border': 'var(--semi-color-border)',
      'semi-color-nav-bg': 'var(--semi-color-nav-bg)',
      'semi-color-overlay-bg': 'var(--semi-color-overlay-bg)',
      'semi-color-fill-0': 'var(--semi-color-fill-0)',
      'semi-color-fill-1': 'var(--semi-color-fill-1)',
      'semi-color-fill-2': 'var(--semi-color-fill-2)',
      'semi-color-bg-0': 'var(--semi-color-bg-0)',
      'semi-color-bg-1': 'var(--semi-color-bg-1)',
      'semi-color-bg-2': 'var(--semi-color-bg-2)',
      'semi-color-bg-3': 'var(--semi-color-bg-3)',
      'semi-color-bg-4': 'var(--semi-color-bg-4)',
      'semi-color-text-0': 'var(--semi-color-text-0)',
      'semi-color-text-1': 'var(--semi-color-text-1)',
      'semi-color-text-2': 'var(--semi-color-text-2)',
      'semi-color-text-3': 'var(--semi-color-text-3)',
      'semi-color-highlight-bg': 'var(--semi-color-highlight-bg)',
      'semi-color-highlight': 'var(--semi-color-highlight)',
      'semi-color-data-0': 'var(--semi-color-data-0)',
      'semi-color-data-1': 'var(--semi-color-data-1)',
      'semi-color-data-2': 'var(--semi-color-data-2)',
      'semi-color-data-3': 'var(--semi-color-data-3)',
      'semi-color-data-4': 'var(--semi-color-data-4)',
      'semi-color-data-5': 'var(--semi-color-data-5)',
      'semi-color-data-6': 'var(--semi-color-data-6)',
      'semi-color-data-7': 'var(--semi-color-data-7)',
      'semi-color-data-8': 'var(--semi-color-data-8)',
      'semi-color-data-9': 'var(--semi-color-data-9)',
      'semi-color-data-10': 'var(--semi-color-data-10)',
      'semi-color-data-11': 'var(--semi-color-data-11)',
      'semi-color-data-12': 'var(--semi-color-data-12)',
      'semi-color-data-13': 'var(--semi-color-data-13)',
      'semi-color-data-14': 'var(--semi-color-data-14)',
      'semi-color-data-15': 'var(--semi-color-data-15)',
      'semi-color-data-16': 'var(--semi-color-data-16)',
      'semi-color-data-17': 'var(--semi-color-data-17)',
      'semi-color-data-18': 'var(--semi-color-data-18)',
      'semi-color-data-19': 'var(--semi-color-data-19)',
    },
    extend: {
      borderRadius: {
        'semi-border-radius-extra-small':
          'var(--semi-border-radius-extra-small)',
        'semi-border-radius-small': 'var(--semi-border-radius-small)',
        'semi-border-radius-medium': 'var(--semi-border-radius-medium)',
        'semi-border-radius-large': 'var(--semi-border-radius-large)',
        'semi-border-radius-circle': 'var(--semi-border-radius-circle)',
        'semi-border-radius-full': 'var(--semi-border-radius-full)',
      },
      // ==================== Ailurus Animations ====================
      animation: {
        // Entrance animations
        'ailurus-fade-in': 'ailurusFadeIn 0.4s cubic-bezier(0.16, 1, 0.3, 1)',
        'ailurus-slide-up': 'ailurusSlideUp 0.5s cubic-bezier(0.16, 1, 0.3, 1)',
        'ailurus-slide-down': 'ailurusSlideDown 0.5s cubic-bezier(0.16, 1, 0.3, 1)',
        'ailurus-scale-in': 'ailurusScaleIn 0.4s cubic-bezier(0.34, 1.56, 0.64, 1)',
        'ailurus-bounce-in': 'ailurusBounceIn 0.6s cubic-bezier(0.34, 1.56, 0.64, 1)',
        // Micro-interaction animations
        'ailurus-pulse-glow': 'ailurusPulseGlow 2s ease-in-out infinite',
        'ailurus-shimmer': 'ailurusShimmer 2.5s ease-in-out infinite',
        // Stagger cascade effect (use with animation-delay utilities)
        'ailurus-cascade': 'ailurusCascade 0.5s cubic-bezier(0.16, 1, 0.3, 1) forwards',
      },
      keyframes: {
        ailurusFadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        },
        ailurusSlideUp: {
          '0%': { opacity: '0', transform: 'translateY(20px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        ailurusSlideDown: {
          '0%': { opacity: '0', transform: 'translateY(-20px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
        ailurusScaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.9)' },
          '100%': { opacity: '1', transform: 'scale(1)' },
        },
        ailurusBounceIn: {
          '0%': { opacity: '0', transform: 'scale(0.3)' },
          '50%': { transform: 'scale(1.05)' },
          '70%': { transform: 'scale(0.95)' },
          '100%': { opacity: '1', transform: 'scale(1)' },
        },
        ailurusPulseGlow: {
          '0%, 100%': { boxShadow: '0 0 20px rgba(194, 94, 0, 0.3)' },
          '50%': { boxShadow: '0 0 40px rgba(230, 126, 34, 0.5)' },
        },
        ailurusShimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' },
        },
        ailurusCascade: {
          '0%': { opacity: '0', transform: 'translateY(16px)' },
          '100%': { opacity: '1', transform: 'translateY(0)' },
        },
      },
      // ==================== Luminous Shadows (Colored Glows) ====================
      boxShadow: {
        // Rust/Orange glows (primary)
        'ailurus-rust-sm': '0 2px 8px rgba(194, 94, 0, 0.15)',
        'ailurus-rust': '0 4px 16px rgba(194, 94, 0, 0.2)',
        'ailurus-rust-lg': '0 8px 32px rgba(194, 94, 0, 0.25)',
        'ailurus-rust-xl': '0 12px 48px rgba(230, 126, 34, 0.3)',
        // Teal glows (tech accent)
        'ailurus-teal-sm': '0 2px 8px rgba(6, 182, 212, 0.15)',
        'ailurus-teal': '0 4px 16px rgba(6, 182, 212, 0.2)',
        'ailurus-teal-lg': '0 8px 32px rgba(6, 182, 212, 0.25)',
        // Purple glows (tech accent)
        'ailurus-purple-sm': '0 2px 8px rgba(139, 92, 246, 0.15)',
        'ailurus-purple': '0 4px 16px rgba(139, 92, 246, 0.2)',
        'ailurus-purple-lg': '0 8px 32px rgba(139, 92, 246, 0.25)',
        // Glass panel shadow
        'ailurus-glass': '0 8px 32px rgba(0, 0, 0, 0.12), inset 0 1px 0 rgba(255, 255, 255, 0.1)',
        'ailurus-glass-lg': '0 16px 48px rgba(0, 0, 0, 0.15), inset 0 1px 0 rgba(255, 255, 255, 0.1)',
        // Inner glow for 3D glass effect
        'ailurus-inner-glow': 'inset 0 1px 0 rgba(255, 255, 255, 0.1), inset 0 -1px 0 rgba(0, 0, 0, 0.1)',
      },
      // ==================== Background Gradients ====================
      backgroundImage: {
        // Primary rust gradients
        'ailurus-gradient-rust': 'linear-gradient(135deg, #C25E00 0%, #E67E22 100%)',
        'ailurus-gradient-rust-hover': 'linear-gradient(135deg, #D96E10 0%, #F08C30 100%)',
        // Dark forest backgrounds
        'ailurus-gradient-forest': 'linear-gradient(to bottom right, #1A1A1A 0%, #0F172A 100%)',
        'ailurus-gradient-forest-radial': 'radial-gradient(ellipse at top, #1E293B 0%, #0F172A 70%)',
        // Tech accent gradients
        'ailurus-gradient-teal': 'linear-gradient(135deg, #06B6D4 0%, #0891B2 100%)',
        'ailurus-gradient-purple': 'linear-gradient(135deg, #8B5CF6 0%, #7C3AED 100%)',
        // Shimmer effect background
        'ailurus-shimmer': 'linear-gradient(90deg, transparent 0%, rgba(255,255,255,0.1) 50%, transparent 100%)',
      },
      // ==================== Backdrop Blur (Glassmorphism) ====================
      backdropBlur: {
        'ailurus-glass': '20px',
        'ailurus-glass-heavy': '40px',
      },
      // ==================== Transition Timing Functions ====================
      transitionTimingFunction: {
        'ailurus-spring': 'cubic-bezier(0.34, 1.56, 0.64, 1)',
        'ailurus-smooth': 'cubic-bezier(0.16, 1, 0.3, 1)',
        'ailurus-bounce': 'cubic-bezier(0.68, -0.55, 0.265, 1.55)',
      },
    },
  },
  plugins: [],
};
