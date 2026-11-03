import { t } from '../i18n'
import { formatStat } from '../lib/format'
import { usePresentation } from '../presentation'
import type { Result, TeamPreview } from '../types'
import { GameEffectText } from './equipment-cards'

type Effect = TeamPreview['conditional'][number]

export function BuildBuffs({ result }: { result: Result }) {
  const presentation = usePresentation()
  const effects = result.buff_effects ?? []
  const personal = Object.entries(result.stats.sources).filter(([source, stats]) =>
    source.endsWith('_conditional') && Object.values(stats).some(value => value !== 0),
  )
  const included = effects.filter(effect => effect.included)
  const informational = effects.filter(effect => !effect.included)
  const formatBonus = (property: string, value: number) =>
    `+${formatStat(property, value)} ${presentation.stats[property] ?? property}`

  function row(effect: Effect) {
    const source = effect.equipment_name || presentation.abilities?.[effect.id] || t(`team_source_${effect.kind}`)
    const description = effect.kind === 'set'
      ? presentation.set_effects?.[`${effect.id}_${effect.pieces}`]
      : t(`team_effect_${effect.detail}`)
    const stats = effect.bonus ? [effect.bonus] : effect.projected_stats ?? []
    return <li key={`${effect.character_id}:${effect.id}`} className="grid gap-1 py-2 sm:grid-cols-2">
      <div>
        <b>{[...(effect.source_names ?? []), source].filter(Boolean).join(' · ')}</b>
        <p className="text-slate-400">{t(`team_trigger_${effect.trigger}`)}</p>
      </div>
      <div>
        {stats.length > 0 && <p className="font-semibold text-emerald-200">
          {stats.map((stat, index) => <span key={index}>{index > 0 && ' · '}{formatBonus(stat.property_id, stat.value)}</span>)}
          {effect.duration_seconds ? ` · ${t('buff_duration_seconds', { seconds: effect.duration_seconds })}` : ''}
        </p>}
        {description && (effect.kind !== 'skill' || stats.length === 0) && <GameEffectText text={description}
          parameters={presentation.fork_effect_parameters?.[`${effect.id}_${effect.star}`]} className="text-slate-400" />}
        {effect.non_stacking && <p className="text-amber-200">{t('team_effect_non_stacking')}</p>}
      </div>
    </li>
  }

  return <div className="grid gap-4 text-sm">
    {!result.buff_effects && !!result.team_allies?.length && <p className="text-slate-400">{t('build_buffs_old_result')}</p>}
    <section>
      <h3 className="font-bold">{t('build_buffs_included')}</h3>
      {!personal.length && !included.length && <p className="mt-1 text-slate-400">{t('build_buffs_none')}</p>}
      <ul className="divide-y divide-slate-800">
        {personal.map(([source, stats]) => <li key={source} className="grid gap-1 py-2 sm:grid-cols-2">
          <div>
            <b>{presentation.stat_sources[source] || t('other_bonus_fallback')}</b>
            <p className="text-slate-400">{source === 'weapon_conditional' ? result.weapon?.name : source === 'set_conditional' ? result.set.name : undefined}</p>
          </div>
          <p className="text-emerald-200">{Object.entries(stats).filter(([, value]) => value !== 0).map(([property, value]) => formatBonus(property, value)).join(' · ')}</p>
        </li>)}
        {included.map(row)}
      </ul>
    </section>
    {informational.length > 0 && <section>
      <h3 className="font-bold">{t('build_buffs_informational')}</h3>
      <ul className="divide-y divide-slate-800">{informational.map(row)}</ul>
    </section>}
  </div>
}
