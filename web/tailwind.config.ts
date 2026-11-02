import type { Config } from 'tailwindcss';

const token = (name: string) => `rgb(var(--oa-${name}) / <alpha-value>)`;

export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    // Spacing scale from the design system: 4 / 8 / 12 / 16 / 24 / 32 / 48 (+ utility steps).
    extend: {
      colors: {
        background: token('background'),
        surface: {
          DEFAULT: token('surface'),
          raised: token('surface-raised'),
          sunken: token('surface-sunken'),
        },
        border: { DEFAULT: token('border'), strong: token('border-strong') },
        fg: { DEFAULT: token('fg'), muted: token('fg-muted'), subtle: token('fg-subtle') },
        primary: { DEFAULT: token('primary'), fg: token('primary-fg') },
        accent: { DEFAULT: token('accent'), fg: token('accent-fg') },
        success: token('success'),
        warning: token('warning'),
        danger: token('danger'),
      },
      fontFamily: {
        sans: ['Inter', 'ui-sans-serif', 'system-ui', '-apple-system', 'Segoe UI', 'sans-serif'],
        mono: ['"JetBrains Mono"', 'ui-monospace', 'SFMono-Regular', 'Menlo', 'monospace'],
      },
      fontSize: {
        caption: ['0.75rem', { lineHeight: '1rem' }],
        body: ['0.875rem', { lineHeight: '1.375rem' }],
        heading: ['1.25rem', { lineHeight: '1.75rem', fontWeight: '600' }],
        display: ['2.5rem', { lineHeight: '1.1', fontWeight: '700', letterSpacing: '-0.02em' }],
      },
      borderRadius: { DEFAULT: '0.5rem', lg: '0.75rem', xl: '1rem' },
      boxShadow: {
        elevated: '0 8px 30px rgb(0 0 0 / 0.35)',
        focus: '0 0 0 2px rgb(var(--oa-background)), 0 0 0 4px rgb(var(--oa-primary))',
      },
      keyframes: {
        'fade-in': { from: { opacity: '0' }, to: { opacity: '1' } },
        'slide-in-right': { from: { transform: 'translateX(100%)' }, to: { transform: 'translateX(0)' } },
        'scale-in': { from: { opacity: '0', transform: 'scale(.96)' }, to: { opacity: '1', transform: 'scale(1)' } },
        'toast-in': { from: { opacity: '0', transform: 'translateY(8px)' }, to: { opacity: '1', transform: 'translateY(0)' } },
        eq: { '0%,100%': { transform: 'scaleY(.3)' }, '50%': { transform: 'scaleY(1)' } },
      },
      animation: {
        'fade-in': 'fade-in 150ms ease-out',
        'slide-in-right': 'slide-in-right 200ms ease-out',
        'scale-in': 'scale-in 150ms ease-out',
        'toast-in': 'toast-in 200ms ease-out',
        eq: 'eq 900ms ease-in-out infinite',
      },
    },
  },
  plugins: [],
} satisfies Config;
