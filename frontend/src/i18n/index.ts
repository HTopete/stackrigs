import en from './en.json';
import es from './es.json';

export type Locale = 'en' | 'es';

export const defaultLocale: Locale = 'en';

export const supportedLocales: Locale[] = ['en', 'es'];

export const localeNames: Record<Locale, string> = {
  en: 'English',
  es: 'Español',
};

type TranslationDict = typeof en;

const translations: Record<Locale, TranslationDict> = { en, es };

/**
 * Detect locale from a URL pathname.
 * Expects paths like /es/about or /en/explore.
 * Falls back to defaultLocale when no prefix matches.
 */
export function detectLocaleFromPath(pathname: string): Locale {
  const segment = pathname.split('/').filter(Boolean)[0];
  if (segment && supportedLocales.includes(segment as Locale)) {
    return segment as Locale;
  }
  return defaultLocale;
}

/**
 * Detect locale from the Accept-Language header.
 * Returns the first supported locale found, or the default.
 */
export function detectLocaleFromHeader(acceptLanguage: string | null | undefined): Locale {
  if (!acceptLanguage) return defaultLocale;

  const preferred = acceptLanguage
    .split(',')
    .map((part) => {
      const [lang, q] = part.trim().split(';q=');
      return { lang: lang.trim().toLowerCase(), quality: q ? parseFloat(q) : 1 };
    })
    .sort((a, b) => b.quality - a.quality);

  for (const { lang } of preferred) {
    const short = lang.substring(0, 2) as Locale;
    if (supportedLocales.includes(short)) {
      return short;
    }
  }

  return defaultLocale;
}

/**
 * Access a nested translation value using a dot-separated key path.
 *
 * Example:
 *   t('en', 'nav.home')          => "Home"
 *   t('es', 'buildLog.status.building') => "En construccion"
 */
export function t(locale: Locale, keyPath: string): string {
  const dict = translations[locale] ?? translations[defaultLocale];
  const keys = keyPath.split('.');
  let current: unknown = dict;

  for (const key of keys) {
    if (current === null || current === undefined || typeof current !== 'object') {
      return keyPath; // fallback: return the key itself
    }
    current = (current as Record<string, unknown>)[key];
  }

  if (typeof current === 'string') {
    return current;
  }

  return keyPath;
}

/**
 * Create a scoped translator bound to a specific locale.
 *
 * Example:
 *   const t = useTranslations('es');
 *   t('nav.home') => "Inicio"
 */
export function useTranslations(locale: Locale) {
  return (keyPath: string): string => t(locale, keyPath);
}

/**
 * Get the full translations object for a locale.
 * Useful when you need to pass the entire dictionary to a component.
 */
export function getTranslations(locale: Locale): TranslationDict {
  return translations[locale] ?? translations[defaultLocale];
}

/**
 * Build a localized path. English paths have no prefix (default locale).
 *
 * Examples:
 *   localePath('en', '/about')  => "/about"
 *   localePath('es', '/about')  => "/es/about"
 */
export function localePath(locale: Locale, path: string): string {
  const clean = path.startsWith('/') ? path : `/${path}`;
  if (locale === defaultLocale) {
    return clean;
  }
  return `/${locale}${clean}`;
}

/**
 * Remove locale prefix from a path.
 *
 * Examples:
 *   stripLocale('/es/about') => "/about"
 *   stripLocale('/about')    => "/about"
 */
export function stripLocale(pathname: string): string {
  for (const locale of supportedLocales) {
    const prefix = `/${locale}/`;
    if (pathname.startsWith(prefix)) {
      return pathname.slice(prefix.length - 1);
    }
    if (pathname === `/${locale}`) {
      return '/';
    }
  }
  return pathname;
}
