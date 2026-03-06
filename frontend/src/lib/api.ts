// API base URL — set via PUBLIC_API_URL env var at build time.
// In production: https://api.stackrigs.com
// In dev: empty string (proxied by Astro dev server or same-origin)
export const API_BASE = import.meta.env.PUBLIC_API_URL || '';
