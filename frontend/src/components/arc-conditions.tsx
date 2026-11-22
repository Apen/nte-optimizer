import { t } from '../i18n'
import type { ArcConditionKey, ArcConditionOption, WorkspaceCharacter } from '../types'

export function arcConditionKey(condition: ArcConditionKey) {
  return JSON.stringify([condition.character_id, condition.fork_id, condition.trigger])
}

export function ArcConditions({ options, selected, characters, onChange }: {
  options: ArcConditionOption[]
  selected: Record<string, number>
  characters: WorkspaceCharacter[]
  onChange: (key: string, stacks: number) => void
}) {
  if (!options.length) return null
  return <fieldset className="mt-4 grid gap-2 rounded-lg border border-slate-700 p-3 text-sm">
    <legend className="px-1 font-semibold">{t('arc_conditions_title')}</legend>
    <p className="text-slate-400">{t('arc_conditions_note')}</p>
    {options.map(option => {
      const key = arcConditionKey(option)
      const character = characters.find(item => item.character_id === option.character_id)?.name
      const source = [character, option.arc_name].filter(Boolean).join(' · ')
      const trigger = t(`team_trigger_${option.trigger}`)
      return <label key={key} className="flex flex-wrap items-center gap-2 rounded-md bg-slate-900/70 p-2">
        {option.max_stacks > 1
          ? <select value={selected[key] ?? 0} onChange={event => onChange(key, Number(event.target.value))}>
            <option value={0}>{t('arc_condition_inactive')}</option>
            {Array.from({ length: option.max_stacks }, (_, index) =>
              <option key={index + 1} value={index + 1}>{t('arc_condition_stacks', { count: index + 1 })}</option>,
            )}
          </select>
          : <input type="checkbox" checked={Boolean(selected[key])} onChange={event => onChange(key, Number(event.target.checked))} />}
        <span><strong>{source}</strong> · {trigger}</span>
      </label>
    })}
  </fieldset>
}
