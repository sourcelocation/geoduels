/** @type {import('tailwindcss').Config} */

/* GeoDuels owns this theme. Do not use `extend`: extending Tailwind defaults
 * makes the design language effectively unbounded. */
module.exports = {
  content: ['./pages/**/*.{js,ts,jsx,tsx}', './components/**/*.{js,ts,jsx,tsx}', './features/**/*.{js,ts,jsx,tsx}'],
  theme: {
    fontFamily: {
      /* One Google family (Outfit) serves body, display, and HUD type;
       * the roles differ only by weight tokens. */
      body: ['var(--font-body)', 'system-ui', 'sans-serif'],
      display: ['var(--font-body)', 'system-ui', 'sans-serif'],
      hud: ['var(--font-body)', 'system-ui', 'sans-serif'],
      mono: ['ui-monospace', 'SFMono-Regular', 'Menlo', 'Monaco', 'Consolas', 'monospace']
    },
    fontSize: {
      body: ['1rem', { lineHeight: '1.5rem' }],
      'body-sm': ['0.875rem', { lineHeight: '1.25rem' }],
      label: ['0.75rem', { lineHeight: '1rem', letterSpacing: '0.075em' }],
      caption: ['0.6875rem', { lineHeight: '1rem', letterSpacing: '0.06em' }],
      'heading-sm': ['1.125rem', { lineHeight: '1.5rem' }],
      'heading-md': ['1.5rem', { lineHeight: '1.875rem' }],
      'heading-lg': ['1.875rem', { lineHeight: '2.25rem' }],
      'display-md': ['2.25rem', { lineHeight: '2.5rem' }],
      'display-lg': ['3rem', { lineHeight: '1' }],
      'hud-label': ['0.6875rem', { lineHeight: '1rem', letterSpacing: '0.12em' }],
      'hud-value': ['1.5rem', { lineHeight: '1', letterSpacing: '-0.03em' }],
      /* Gameplay countdown numerals own a screen-centre scale distinct from
       * document display type. */
      'hud-countdown': ['9.375rem', { lineHeight: '1' }],
      'hud-countdown-lg': ['12.5rem', { lineHeight: '1' }]
    },
    fontWeight: { regular: '400', medium: '500', semibold: '600', strong: '700' },
    colors: {
      transparent: 'transparent', current: 'currentColor', inherit: 'inherit',
      'surface-page': 'var(--gd-surface-page)', 'surface-panel': 'var(--gd-surface-panel)',
      'surface-raised': 'var(--gd-surface-raised)', 'surface-grouped': 'var(--gd-surface-grouped)',
      'surface-fill': 'var(--gd-surface-fill)', 'surface-inset': 'var(--gd-surface-inset)',
      'surface-overlay': 'var(--gd-surface-overlay)', 'content-primary': 'var(--gd-content-primary)',
      'content-secondary': 'var(--gd-content-secondary)', 'content-inverse': 'var(--gd-content-inverse)',
      'content-on-action': 'var(--gd-content-on-action)', 'content-on-danger': 'var(--gd-content-on-danger)',
      'border-default': 'var(--gd-border-default)', 'border-strong': 'var(--gd-border-strong)',
      'border-focus': 'var(--gd-border-focus)', 'action-primary': 'rgb(var(--gd-action-primary) / <alpha-value>)',
      'action-primary-hover': 'rgb(var(--gd-action-primary-hover) / <alpha-value>)',
      'action-secondary': 'var(--gd-action-secondary)', 'action-danger': 'rgb(var(--gd-action-danger) / <alpha-value>)',
      'status-success': 'rgb(var(--gd-status-success) / <alpha-value>)', 'status-warning': 'rgb(var(--gd-status-warning) / <alpha-value>)',
      'status-danger': 'rgb(var(--gd-status-danger) / <alpha-value>)', 'status-info': 'rgb(var(--gd-status-info) / <alpha-value>)',
      scrim: 'var(--gd-scrim)', 'hud-surface': 'var(--gd-hud-surface)', 'hud-border': 'var(--gd-hud-border)',
      'brand-blue': 'rgb(var(--gd-accent-blue) / <alpha-value>)', 'brand-blue-hover': 'rgb(var(--gd-accent-blue-hover) / <alpha-value>)',
      'brand-pink': 'rgb(var(--gd-accent-pink) / <alpha-value>)', 'brand-pink-soft': 'rgb(var(--gd-accent-pink-soft) / <alpha-value>)',
      'brand-orange': 'rgb(var(--gd-accent-orange) / <alpha-value>)'
    },
    spacing: {
      px: '1px', 0: '0px', 0.5: '0.125rem', 1: '0.25rem', 1.5: '0.375rem', 2: '0.5rem', 2.5: '0.625rem',
      3: '0.75rem', 3.5: '0.875rem', 4: '1rem', 4.5: '1.125rem', 5: '1.25rem', 6: '1.5rem', 7: '1.75rem',
      8: '2rem', 9: '2.25rem', 10: '2.5rem', 11: '2.75rem', 12: '3rem', 14: '3.5rem', 16: '4rem',
      20: '5rem', 24: '6rem', 28: '7rem', 32: '8rem', 36: '9rem', 40: '10rem', 44: '11rem', 48: '12rem',
      52: '13rem', 56: '14rem', 60: '15rem', 64: '16rem', 72: '18rem', 80: '20rem', 96: '24rem', auto: 'auto'
    },
    borderRadius: { sm: 'var(--gd-radius-sm)', md: 'var(--gd-radius-md)', lg: 'var(--gd-radius-lg)', xl: 'var(--gd-radius-xl)', '2xl': 'var(--gd-radius-2xl)', full: 'var(--gd-radius-full)' },
    boxShadow: {
      none: 'none', inner: 'inset 0 2px 4px rgb(2 6 11 / 18%)',
      'elev-1': 'var(--gd-shadow-1)', 'elev-2': 'var(--gd-shadow-2)', 'elev-3': 'var(--gd-shadow-3)', 'elev-4': 'var(--gd-shadow-4)'
    },
    dropShadow: {
      none: '0 0 0 transparent',
      sm: '0 1px 2px rgb(2 6 11 / 18%)',
      md: '0 4px 8px rgb(2 6 11 / 24%)',
      lg: '0 8px 16px rgb(2 6 11 / 30%)'
    },
    opacity: { 0: '0', 5: '0.05', 10: '0.1', 15: '0.15', 20: '0.2', 25: '0.25', 30: '0.3', 35: '0.35', 40: '0.4', 50: '0.5', 60: '0.6', 70: '0.7', 75: '0.75', 80: '0.8', 85: '0.85', 90: '0.9', 95: '0.95', 100: '1' },
    zIndex: { base: '0', underlay: '1', content: '10', sticky: '20', 'game-controls': '40', game: '50', 'game-overlay': '100', modal: '1000', dialog: '1200', 'modal-critical': '2100', popover: '2200', tooltip: '2300', 'range-track': '0', 'range-thumb': '1' },
    borderWidth: { DEFAULT: '1px', 0: '0px', 2: '2px', 3: '3px', 4: '4px', 6: '6px' },
    letterSpacing: { body: '0', heading: '-0.025em', 'display-tight': '-0.03em', label: '0.08em', wide: '0.05em', eyebrow: '0.12em', 'eyebrow-wide': '0.16em', 'eyebrow-strong': '0.18em', control: '0.14em', 'control-wide': '0.15em', display: '0.1em', 'display-wide': '0.2em', 'display-max': '0.26em' },
    lineHeight: { collapsed: '1', heading: '1.25', label: '1.25rem', body: '1.5rem', prose: '1.625rem', 'prose-lg': '1.75rem' },
    transitionDuration: { instant: '75ms', fast: '150ms', normal: '200ms', slow: '300ms', emphasis: '500ms', dramatic: '700ms' },
    transitionTimingFunction: { standard: 'cubic-bezier(0.4, 0, 0.2, 1)', emphasized: 'cubic-bezier(0.16, 1, 0.3, 1)', gameplay: 'cubic-bezier(0.22, 1, 0.36, 1)' },
    screens: { sm: '640px', md: '768px', lg: '1024px', xl: '1280px' },
    backdropBlur: { md: '12px', hud: '10px' }, blur: { md: '12px' },
    backgroundImage: {
      'gradient-to-r': 'linear-gradient(to right, var(--tw-gradient-stops))', 'gradient-to-b': 'linear-gradient(to bottom, var(--tw-gradient-stops))',
      'gradient-to-br': 'linear-gradient(to bottom right, var(--tw-gradient-stops))', 'gradient-to-t': 'linear-gradient(to top, var(--tw-gradient-stops))'
    },
    keyframes: {
      spin: { to: { transform: 'rotate(360deg)' } },
      pulse: { '50%': { opacity: '0.5' } },
      countdownPulse: { '0%, 100%': { transform: 'scale(1)', opacity: '0.95' }, '50%': { transform: 'scale(1.08)', opacity: '1' } },
      scorePop: { '0%': { transform: 'translateY(12px) scale(0.9)', opacity: '0' }, '60%': { transform: 'translateY(0) scale(1.06)', opacity: '1' }, '100%': { transform: 'translateY(0) scale(1)', opacity: '1' } },
      damageTravel: { '0%': { transform: 'translateY(14px) scale(0.9)', opacity: '0' }, '35%': { transform: 'translateY(0) scale(1.08)', opacity: '1' }, '100%': { transform: 'translateY(-4px) scale(1)', opacity: '1' } },
      overlayFade: { '0%': { opacity: '0', transform: 'translateY(8px)' }, '100%': { opacity: '1', transform: 'translateY(0)' } },
      hudSlideIn: { '0%': { opacity: '0', transform: 'translateY(-10px)' }, '100%': { opacity: '1', transform: 'translateY(0)' } },
      timerCritical: { '0%': { transform: 'scale(1)' }, '85%': { transform: 'scale(1.055)' }, '100%': { transform: 'scale(1)' } },
      lobbyAurora: { '0%': { transform: 'translate3d(-3%, 0, 0) scale(1)', opacity: '0.55' }, '50%': { transform: 'translate3d(3%, 2%, 0) scale(1.06)', opacity: '0.85' }, '100%': { transform: 'translate3d(-3%, 0, 0) scale(1)', opacity: '0.55' } },
      lobbyFloat: { '0%, 100%': { transform: 'translateY(0)' }, '50%': { transform: 'translateY(-10px)' } }
    },
    animation: { spin: 'spin 1s linear infinite', pulse: 'pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite', countdownPulse: 'countdownPulse 1s cubic-bezier(0.4, 0, 0.2, 1) infinite', scorePop: 'scorePop 360ms cubic-bezier(0.16, 1, 0.3, 1) both', damageTravel: 'damageTravel 620ms cubic-bezier(0.22, 1, 0.36, 1) both', overlayFade: 'overlayFade 250ms ease-out both', hudSlideIn: 'hudSlideIn 220ms ease-out both', timerCritical: 'timerCritical 1s cubic-bezier(0.32, 0, 0.68, 1) infinite', lobbyAurora: 'lobbyAurora 15s ease-in-out infinite', lobbyFloat: 'lobbyFloat 8s ease-in-out infinite' }
  },
  plugins: []
};
