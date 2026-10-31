export const APP_CONFIG = {
  brand: {
    name: import.meta.env.VITE_APP_BRAND_NAME || 'barter',
    mark: '$',
    tagline: import.meta.env.VITE_APP_BRAND_TAGLINE || '// a marketplace for makers',
  },
} as const
