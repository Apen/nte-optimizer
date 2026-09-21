import type { LocalizationCatalog } from './types'

export type Locale = 'fr' | 'en'

export const defaultLocale: Locale = 'en'

let activeLocale: Locale = defaultLocale
let ui: Record<string, string> = {}

export function initialLocale(stored: string | null): Locale {
  return stored === 'fr' || stored === 'en' ? stored : defaultLocale
}

export function applyLocalization(locale: Locale, catalog: LocalizationCatalog) {
  activeLocale = locale
  ui = catalog.ui || {}
}

export function setActiveLocale(locale: Locale) {
  activeLocale = locale
}

export function currentIntlLocale() {
  return activeLocale === 'fr' ? 'fr-FR' : 'en-US'
}

export function t(key: string, values: Record<string, string | number> = {}) {
  const template = ui[key] || key
  return template.replace(/\{(\w+)\}/g, (_, name: string) => String(values[name] ?? `{${name}}`))
}
