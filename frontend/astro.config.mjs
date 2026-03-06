import { defineConfig } from 'astro/config';
import preact from '@astrojs/preact';

export default defineConfig({
  output: 'hybrid',
  integrations: [preact()],
  i18n: {
    defaultLocale: 'en',
    locales: ['en', 'es'],
    prefixDefaultLocale: false,
    fallback: { es: 'en' }
  },
  prefetch: {
    prefetchAll: false,
    defaultStrategy: 'viewport'
  },
  experimental: {
    clientPrerender: true
  }
});
