import { createContext, useContext, type ReactNode } from 'react'

import type { LocalizationCatalog } from './types'

const emptyCatalog: LocalizationCatalog = {
  locale: 'en',
  ui: {},
  stats: {},
  qualities: {},
  geometries: {},
  stat_sources: {},
  damage: {},
  abilities: {},
}

const PresentationContext = createContext<LocalizationCatalog>(emptyCatalog)

export function PresentationProvider({ catalog, children }: { catalog: LocalizationCatalog; children: ReactNode }) {
  return <PresentationContext.Provider value={catalog}>{children}</PresentationContext.Provider>
}

export function usePresentation() {
  return useContext(PresentationContext)
}

export function presentationName(values: Record<string, string> | undefined, key: string, fallback = key) {
  return values?.[key] || fallback
}
