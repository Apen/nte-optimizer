import { currentIntlLocale, t } from '../i18n'
import { formatStat } from '../lib/format'
import { usePresentation } from '../presentation'
import type { ObservedPanelStats, PanelBonus, PotentialHousingEffect, StatSummary } from '../types'
import { Card, CardContent, CardHeader } from './ui/card'

// These inputs are already represented by the final panel values below.
const panelStatComponents = new Set([
  'AtkBase', 'AtkUp', 'AtkAdd',
  'DefBase', 'DefUp', 'DefAdd',
  'HPMaxBase', 'HPMaxUp', 'HPMaxAdd', 'HPUp',
])

const roundedPanelStats = new Set(['HPFinal', 'AtkFinal', 'DefFinal', 'Endurance', 'MagBase', 'UnbalIntensityBase'])

export function CurrentGameStats({
  observedPanelStats,
  calculatedStats,
  possiblePanelBonuses,
  potentialHousingEffects,
}: {
  observedPanelStats?: ObservedPanelStats
  calculatedStats?: StatSummary
  possiblePanelBonuses?: PanelBonus[]
  potentialHousingEffects?: PotentialHousingEffect[]
}) {
  const { stats: labels } = usePresentation()
  if (!observedPanelStats && !calculatedStats) {
    return <Card><CardContent className="py-8 text-center text-sm text-slate-500">{t('no_stats_available')}</CardContent></Card>
  }

  const observedEntries: [string, number][] = []
  if (observedPanelStats) {
    const candidates: [string, number | undefined][] = [
      ['HPFinal', observedPanelStats.maxHp],
      ['AtkFinal', observedPanelStats.attack],
      ['DefFinal', observedPanelStats.defense],
      ['Endurance', observedPanelStats.endurance],
      ['CritBase', observedPanelStats.critRate],
      ['CritDamageBase', observedPanelStats.critDamage],
      ['ChargeGetEfficiencyBase', observedPanelStats.chargeEfficiency],
      ['MagBase', observedPanelStats.cycleIntensity],
      ['UnbalIntensityBase', observedPanelStats.breakIntensity],
      ['DamageUpGeneralBase', observedPanelStats.universalDamageBonus],
      ['ElementalDMGBonus', observedPanelStats.elementalDamageBonus],
    ]
    for (const [property, value] of candidates) {
      if (value !== undefined && Number.isFinite(value)) observedEntries.push([property, value])
    }
  }
  const calculatedEntries: [string, number][] = Object.entries(calculatedStats?.derived || {})
    .filter(([property, value]) => Number.isFinite(value) && !panelStatComponents.has(property))
  const entries: [string, number][] = (observedPanelStats ? observedEntries : calculatedEntries)
    .sort(([left], [right]) => (labels[left] || left).localeCompare(labels[right] || right, currentIntlLocale()))
  const panelBonusProperties = new Set((possiblePanelBonuses || []).map(effect => effect.property_id))
  const unmatchedHousingEffects = (potentialHousingEffects || []).filter(effect => !panelBonusProperties.has(effect.panel_property))
  const formatPanelValue = (property: string, value: number) => roundedPanelStats.has(property)
    ? Math.round(value).toLocaleString(currentIntlLocale())
    : formatStat(property, value)

  return <Card>
    <CardHeader>
      <div>
        <h2 className="text-xl font-bold">{t('current_stats')}</h2>
        <p className="mt-1 text-sm text-slate-400">{t(observedPanelStats ? 'observed_panel_stats_description' : 'current_stats_description')}</p>
      </div>
    </CardHeader>
    <CardContent>
      {!observedPanelStats && calculatedStats?.completeness === 'partial' && <p className="mb-4 rounded-lg border border-amber-900/60 bg-amber-950/30 p-3 text-xs text-amber-200">{t('estimated_stats')}</p>}
      <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
        {entries.map(([key, value]) => <div className="rounded-lg border border-slate-800 bg-slate-950/60 px-3 py-2" key={key}>
          <span className="block text-xs text-slate-500">{labels[key] || key}</span>
          <strong className="block text-lg tabular-nums text-slate-100">{roundedPanelStats.has(key) ? Math.round(value).toLocaleString(currentIntlLocale()) : formatStat(key, value)}</strong>
        </div>)}
      </div>
      {possiblePanelBonuses && possiblePanelBonuses.length > 0 && <aside className="mt-4 rounded-lg border border-amber-900/60 bg-amber-950/20 p-3">
        <h3 className="text-sm font-semibold text-amber-200">{t('possible_panel_bonuses')}</h3>
        <p className="mt-1 text-xs text-amber-100/80">{t('possible_panel_bonuses_note')}</p>
        <ul className="mt-3 grid gap-2 sm:grid-cols-2">
          {possiblePanelBonuses.map(effect => <li className="rounded-md border border-amber-900/40 bg-slate-950/40 px-3 py-2 text-sm" key={effect.property_id}>
            <strong>{labels[effect.property_id] || effect.property_id}</strong>
            <span className="ml-2 tabular-nums">+{formatStat(effect.property_id, effect.difference)}</span>
            <small className="mt-1 block text-xs text-slate-400">{t('panel_bonus_comparison', {
              panel: formatPanelValue(effect.property_id, effect.observed_value),
              calculated: formatPanelValue(effect.property_id, effect.calculated_value),
            })}</small>
          </li>)}
        </ul>
      </aside>}
      {unmatchedHousingEffects.length > 0 && <aside className="mt-4 rounded-lg border border-amber-900/60 bg-amber-950/20 p-3">
        <h3 className="text-sm font-semibold text-amber-200">{t('possible_housing_modifiers')}</h3>
        <p className="mt-1 text-xs text-amber-100/80">{t('possible_housing_modifiers_note')}</p>
        <ul className="mt-3 grid gap-2 sm:grid-cols-2">
          {unmatchedHousingEffects.map(effect => <li className="rounded-md border border-amber-900/40 bg-slate-950/40 px-3 py-2 text-sm" key={effect.modifier_id}>
            <strong>{labels[effect.panel_property] || effect.property_id}</strong>
            <span className="ml-2 tabular-nums">+{formatStat(effect.property_id, effect.value)}</span>
            <small className="mt-1 block text-xs text-slate-400">{t('housing_modifier_evidence', { matching: effect.matching_characters, observed: effect.observed_characters })}</small>
          </li>)}
        </ul>
      </aside>}
    </CardContent>
  </Card>
}
