import { Info } from 'lucide-react'

import { currentIntlLocale, t } from '../i18n'
import { formatRanking, formatStat } from '../lib/format'
import { usePresentation } from '../presentation'
import type { Result } from '../types'
import { ArcEffectDescription, GameEffectText, hasUnresolvedEffectParameters } from './equipment-cards'
import { Button } from './ui/button'
import { Dialog, DialogContent, DialogTrigger } from './ui/dialog'

export function AdvancedResultDetails({ result }: { result: Result }) {
  const { stats: labels, stat_sources: sourceNames, fork_effects: forkEffects, fork_effect_parameters: forkEffectParameters, set_effects: setEffects } = usePresentation()
  const weaponEffectKey = result.weapon ? `${result.weapon.forkId}_${result.weapon.star}` : undefined
  const weaponEffectDescription = weaponEffectKey ? forkEffects?.[weaponEffectKey] : undefined
  const canReplaceWeaponEffectStats = Boolean(weaponEffectDescription) && !hasUnresolvedEffectParameters(weaponEffectDescription, weaponEffectKey ? forkEffectParameters?.[weaponEffectKey] : undefined)
  const activeSetEffects = [2, 4]
    .filter(count => (result.solution.set_matched_count || 0) >= count)
    .map(count => ({ count, description: setEffects?.[`${result.set.id}_${count}`] }))
  const canReplaceSetEffectStats = activeSetEffects.length > 0 && activeSetEffects.every(effect =>
    Boolean(effect.description) && !hasUnresolvedEffectParameters(effect.description),
  )
  const visibleSources = Object.entries(result.stats.sources).filter(([source]) =>
    (!canReplaceWeaponEffectStats || (source !== 'weapon_conditional' && source !== 'weapon_permanent')) &&
    (!canReplaceSetEffectStats || (source !== 'set' && source !== 'set_conditional')),
  )
  return <Dialog>
    <DialogTrigger asChild><Button variant="secondary"><Info aria-hidden="true" className="mr-2 size-4" />{t('understand_result')}</Button></DialogTrigger>
    <DialogContent title={t('understand_result')} className="max-h-[92vh] w-[min(1100px,96vw)]">
      <h2 className="pr-10 text-xl font-bold">{t('understand_result')}</h2>
      <div className="understand-result-content mt-4">
        <div className="grid gap-3 sm:grid-cols-3">
          <Detail title={t('weighted_module_score')} value={formatRanking(result.solution.module_score)} />
          <Detail title={t('weighted_cartridge_score')} value={formatRanking(result.solution.cartridge_score)} />
          <Detail title={t('set_bonus_score_detail')} value={formatRanking(result.solution.set_bonus_score)} />
          <Detail title={t('combinations_detail')} value={result.solution.visited_states.toLocaleString(currentIntlLocale())} />
          <Detail title={t('solution_quality_detail')} value={result.solution.complete ? t('best_solution_confirmed_detail') : t('best_solution_found_detail')} />
          <Detail title={t('protected_pieces_detail')} value={result.include_equipped ? t('protection_ignored_detail') : String(result.excluded_equipped)} />
        </div>
        {result.stats.completeness === 'partial' && <p className="rounded-lg border border-amber-900/60 bg-amber-950/30 p-3 text-xs text-amber-200">{t('estimated_stats')}</p>}
        <div className="grid gap-2">
          {visibleSources.map(([source, values]) => <div className="grid gap-2 rounded-lg bg-slate-950 p-3 text-xs md:grid-cols-[190px_1fr]" key={source}>
            <b>{sourceNames[source] || t('other_bonus_fallback')}</b>
            <span className="text-slate-400">{Object.entries(values).map(([key, value]) => `${labels[key] || key} ${formatStat(key, value)}`).join(' · ') || '—'}</span>
          </div>)}
          {result.weapon && <div className="rounded-lg bg-slate-950 p-3 text-xs">
            <b>{t('arc_effect')}</b>
            <ArcEffectDescription forkID={result.weapon.forkId} star={result.weapon.star} />
          </div>}
          {activeSetEffects.some(effect => effect.description) && <div className="rounded-lg bg-slate-950 p-3 text-xs">
            <b>{result.set.name || t('set_bonus_score_detail')}</b>
            {activeSetEffects.map(effect => effect.description && <GameEffectText key={effect.count} text={effect.description} className="mt-2 text-slate-400" />)}
          </div>}
        </div>
      </div>
    </DialogContent>
  </Dialog>
}

function Detail({ title, value }: { title: string; value: string }) {
  return <div><p className="text-xs text-slate-500">{title}</p><strong>{value}</strong></div>
}
