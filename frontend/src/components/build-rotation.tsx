import { t } from '../i18n'
import { planBurstSetup } from '../lib/burst-setup'
import { usePresentation } from '../presentation'
import type { Result, WorkspaceCharacter } from '../types'
import { suggestRotation } from '../lib/suggested-rotation'

export function BuildRotation({ result, characters }: { result: Result; characters: WorkspaceCharacter[] }) {
  const presentation = usePresentation()
  const suggestion = suggestRotation(result)
  if (suggestion) return <section className="grid gap-3 text-sm">
    <div>
      <h3 className="font-bold">{t('rotation_suggested_title')}</h3>
      <p className="mt-1 text-slate-400">{t('rotation_suggested_note')}</p>
      {suggestion.missingReaction && <p className="mt-1 text-amber-200">
        {t('rotation_missing_reaction', { reaction: t(suggestion.missingReaction) })}
      </p>}
    </div>
    <ol className="divide-y divide-slate-800 rounded-xl border border-slate-800">
      {suggestion.steps.map((step, index) => {
        const name = step.characterID === result.character?.characterId ? result.character.name
          : characters.find(character => character.character_id === step.characterID)?.name ?? step.buff?.name
        return <li key={step.id} className="flex items-start gap-3 p-3">
          <span className="grid size-7 shrink-0 place-items-center rounded-full bg-slate-800 font-semibold">{index + 1}</span>
          <img className="size-9 shrink-0 rounded-lg object-cover" src={`/game_ui/characters/${step.characterID}.png`} alt="" />
          <div>
            <strong>{name}</strong>
            <p className="text-slate-300">{t(step.buff ? `rotation_action_${step.buff.action}` : step.label ?? 'rotation_action_skill')}</p>
            {step.buff && <p className="text-xs text-slate-400">{step.buff.effects.map(effect =>
              effect.equipment_name || presentation.abilities?.[effect.id] || t(`team_source_${effect.kind}`),
            ).join(' · ')}</p>}
          </div>
        </li>
      })}
    </ol>
  </section>

  const steps = planBurstSetup(result.buff_effects ?? [])
  if (!steps.length) return <p className="text-sm text-slate-400">
    {t(result.team_allies?.length && !result.buff_effects ? 'build_buffs_old_result' : 'rotation_no_setup')}
  </p>

  return <section className="grid gap-3 text-sm">
    <div>
      <h3 className="font-bold">{t('rotation_burst_title')}</h3>
      <p className="mt-1 text-slate-400">{t('rotation_burst_assumptions')}</p>
    </div>
    <ol className="divide-y divide-slate-800 rounded-xl border border-slate-800">
      {steps.map((step, index) => <li key={`${step.characterID}:${step.action}`} className="flex gap-3 p-3">
        <span className="grid size-7 shrink-0 place-items-center rounded-full bg-slate-800 font-semibold">{index + 1}</span>
        <img className="size-9 shrink-0 rounded-lg object-cover" src={`/game_ui/characters/${step.characterID}.png`} alt="" />
        <div className="min-w-0">
          <strong>{step.name} · {t(`rotation_action_${step.action}`)}</strong>
          <ul className="mt-1 grid gap-1 text-xs text-slate-400">
            {step.effects.map(effect => <li key={effect.id}>
              {effect.equipment_name || presentation.abilities?.[effect.id] || t(`team_source_${effect.kind}`)}
              {effect.duration_seconds ? ` · ${t('buff_duration_seconds', { seconds: effect.duration_seconds })}` : ''}
              {effect.trigger !== step.action ? ` · ${t(`team_trigger_${effect.trigger}`)}` : ''}
            </li>)}
          </ul>
        </div>
      </li>)}
      <li className="flex items-start gap-3 bg-emerald-950/25 p-3">
        <span className="grid size-7 shrink-0 place-items-center rounded-full bg-emerald-900 font-semibold">{steps.length + 1}</span>
        <div>
          <strong>{result.character?.name ?? t('team_active_badge')} · {t('rotation_action_ultimate')}</strong>
          <p className="mt-1 text-xs text-slate-400">{t('rotation_burst_finish')}</p>
        </div>
      </li>
    </ol>
  </section>
}
