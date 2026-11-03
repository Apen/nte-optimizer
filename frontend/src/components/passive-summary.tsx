import { Check } from 'lucide-react'

import { t } from '../i18n'
import { usePresentation } from '../presentation'

export function PassiveSummary({ references, cards = false }: { references?: string[]; cards?: boolean }) {
  const { abilities } = usePresentation()
  if (!references?.length) return cards ? <p className="text-xs text-slate-400">{t('no_active_passives')}</p> : null

  const labels = references.map((reference, index) => {
    const number = /_Passive_?(\d+)$/i.exec(reference)?.[1]
    return abilities?.[reference] || t('numbered_passive', { number: number || index + 1 })
  })
  if (!cards) return <span title={t('active_passives')}>
    <Check className="mr-1 inline size-3 align-[-0.125em]" aria-hidden="true" />
    {labels.join(', ')}
  </span>

  return <section aria-label={t('active_passives')}>
    <h2 className="mb-2 text-xs font-semibold text-slate-300">{t('active_passives')}</h2>
    <ul className="grid grid-cols-2 gap-2 lg:grid-cols-4">
      {labels.map((label, index) => <li key={index} className="flex items-center gap-2 rounded-xl border border-emerald-500/25 bg-emerald-950/30 p-3 text-sm text-emerald-200">
        <Check className="size-4 shrink-0" aria-hidden="true" />
        {label}
      </li>)}
    </ul>
  </section>
}
