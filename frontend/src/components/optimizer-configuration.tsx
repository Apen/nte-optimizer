import { useId } from 'react'
import { RotateCcw, TriangleAlert, X } from 'lucide-react'

import { Card, CardContent } from './ui/card'
import { Progress } from './ui/progress'
import { currentIntlLocale, t } from '../i18n'
import { usePresentation } from '../presentation'
import type { SearchProgress, TargetGoal, TargetPreset } from '../types'

type WeightSettings = { main_stats: string[]; weights: Record<string, number> }

export function weightForGoal(property: string, settings: Pick<WeightSettings, 'weights'>) {
  const families: Record<string, string[]> = {
    AtkFinal: ['AtkBase', 'AtkUp', 'AtkAdd'],
    HPFinal: ['HPMaxBase', 'HPMaxUp', 'HPMaxAdd', 'HPUp'],
    DefFinal: ['DefBase', 'DefUp', 'DefAdd'],
  }
  return Math.max(0, ...(families[property] || [property]).map(key => settings.weights[key] || 0))
}

export function SearchStatus({ progress }: { progress?: SearchProgress }) {
  const elapsed = Math.max((progress?.elapsed_ms || 0) / 1000, .001)
  const visited = progress?.visited || 0
  const total = progress?.total || 0
  const percent = progress?.finished ? 100 : total > 0 ? Math.min(100, visited / total * 100) : undefined
  const rate = visited / elapsed
  const remaining = total > visited ? (total - visited) / Math.max(rate, 1) : 0
  const eta = remaining >= 3600 ? `${(remaining / 3600).toFixed(1)} ${t('hour_short')}` : remaining >= 60 ? `${Math.ceil(remaining / 60)} ${t('minute_short')}` : `${Math.ceil(remaining)} ${t('second_short')}`
  const elapsedLabel = elapsed.toFixed(1).replace('.', currentIntlLocale() === 'fr-FR' ? ',' : '.')
  const progressLabel = total > 0 ? t('tested_progress', { visited: visited.toLocaleString(currentIntlLocale()), total: total.toLocaleString(currentIntlLocale()), elapsed: elapsedLabel }) : t('tested_progress_short', { visited: visited.toLocaleString(currentIntlLocale()), elapsed: elapsedLabel })
  return <Card className="mb-5 border-pink-900/60"><CardContent className="grid gap-3 pt-5"><div className="flex flex-wrap items-center justify-between gap-3"><div><strong>{t('search_best_build')}</strong><p className="text-xs text-slate-500">{t('workers_candidates', { workers: progress?.workers || 1, candidates: (progress?.candidates || 0).toLocaleString(currentIntlLocale()), rate: Math.round(rate).toLocaleString(currentIntlLocale()), pruned: (progress?.pruned_branches || 0).toLocaleString(currentIntlLocale()) })}</p></div><span className="text-sm tabular-nums text-slate-400">{progressLabel}{total > visited && !progress?.pruned_branches ? ` · ${t('no_cuts_eta', { eta })}` : ''}</span></div><Progress value={percent} /></CardContent></Card>
}

type StatsEditorProps = {
  preset?: TargetPreset
  disabledGoals: string[]
  strictGoals: string[]
  strictMinimums: Record<string, number>
  goals: Record<string, number>
  maximums: Record<string, number>
  weights: WeightSettings
  availableMain: string[]
  onGoal: (id: string, value: number) => void
  onMaximum: (id: string, value: number) => void
  onMinimum: (id: string, value: number) => void
  onWeight: (id: string, value: number) => void
  onMainStats: (ids: string[]) => void
  onRemove: (id: string) => void
  onReset: () => Promise<void>
  onSave: () => Promise<void>
}

export function StatsEditor({ preset, disabledGoals, strictGoals, strictMinimums, goals, maximums, weights, availableMain, onGoal, onMaximum, onMinimum, onWeight, onMainStats, onRemove, onReset, onSave }: StatsEditorProps) {
  const visibleGoals = (preset?.goals || []).filter(goal => !disabledGoals.includes(goal.property_id))
  return <section className="configuration-panel goals-panel" onBlur={() => void onSave()}>
    <div className="section-heading"><p className="eyebrow">{t('stats_section')}</p><button className="text-action" onClick={() => void onReset()}><RotateCcw className="size-3.5" />{t('reset')}</button></div>
    <MainStatSelector selected={weights.main_stats} available={availableMain} onChange={onMainStats} />
    <div className="goal-matrix stats-matrix"><div className="goal-table-heading"><span>{t('statistic')}</span><span>{t('weight')}</span><span>{t('objective_label')}</span><span>{t('min_strict')}</span><span>{t('max_strict')}</span></div>{visibleGoals.map(goal => <StatConfigRow key={goal.property_id} goal={goal} value={goals[goal.property_id] || 0} maximumValue={maximums[goal.property_id] || 0} minimumValue={strictGoals.includes(goal.property_id) ? strictMinimums[goal.property_id] || 0 : 0} weight={weightForGoal(goal.property_id, weights)} onGoal={value => onGoal(goal.property_id, value)} onMaximum={value => onMaximum(goal.property_id, value)} onMinimum={value => onMinimum(goal.property_id, value)} onWeight={value => onWeight(weightKeyForGoal(goal.property_id, weights), value)} onRemove={() => onRemove(goal.property_id)} />)}{visibleGoals.length === 0 && <p className="py-8 text-center text-sm text-slate-500">{t('no_stats')}</p>}</div>
  </section>
}

function MainStatSelector({ selected, available, onChange }: { selected: string[]; available: string[]; onChange: (ids: string[]) => void }) {
  const { stats: labels } = usePresentation()
  const choices = available.filter(id => !selected.includes(id)).sort((a, b) => (labels[a] || a).localeCompare(labels[b] || b, currentIntlLocale()))
  return <div className="main-stat-selector"><span className="field-caption">{t('stats_main')}</span><div className="main-stat-tags">{selected.map(id => <button key={id} onClick={() => onChange(selected.filter(value => value !== id))}>{labels[id] || id}<X className="size-3" /></button>)}</div>{choices.length > 0 && <select aria-label={t('add_main_stat')} value="" onChange={event => { if (event.target.value) onChange([...selected, event.target.value]) }}><option value="">{t('add_option')}</option>{choices.map(id => <option key={id} value={id}>{labels[id] || id}</option>)}</select>}</div>
}

function weightKeyForGoal(property: string, settings: Pick<WeightSettings, 'weights'>) {
  const families: Record<string, string[]> = {
    AtkFinal: ['AtkBase', 'AtkUp', 'AtkAdd'],
    HPFinal: ['HPMaxBase', 'HPMaxUp', 'HPMaxAdd', 'HPUp'],
    DefFinal: ['DefBase', 'DefUp', 'DefAdd'],
  }
  return (families[property] || [property]).sort((a, b) => (settings.weights[b] || 0) - (settings.weights[a] || 0))[0]
}

function StatConfigRow({ goal, value, maximumValue, minimumValue, weight, onGoal, onMaximum, onMinimum, onWeight, onRemove }: { goal: TargetGoal; value: number; maximumValue: number; minimumValue: number; weight: number; onGoal: (value: number) => void; onMaximum: (value: number) => void; onMinimum: (value: number) => void; onWeight: (value: number) => void; onRemove: () => void }) {
  const { stats: labels } = usePresentation()
  const warningId = useId()
  const name = labels[goal.property_id] || goal.label
  const zeroWeightHint = t('zero_weight_goal_hint')
  return <div className="goal-row"><div className="goal-name flex items-start justify-between gap-2"><span><strong className="block">{name}</strong><small>{t('reference', { value: `${goal.percent ? goal.minimum * 100 : goal.minimum}${goal.percent ? ' %' : ''}` })}</small></span><button className="rounded-md p-1 text-slate-500 hover:bg-red-950 hover:text-red-300" onClick={onRemove} title={t('remove_goal', { name })} aria-label={t('remove_goal', { name })}><X className="size-4" /></button></div>
    <div className="goal-field"><span className="mobile-label">{t('weight')}</span><div className="flex items-center gap-1"><div className="number-field flex-1"><input aria-label={t('weight_for', { name })} type="number" min={0} max={10} step={0.05} value={weight} onChange={event => onWeight(Math.min(10, Math.max(0, Number(event.target.value))))} /></div>{weight === 0 && <span className="group relative inline-flex shrink-0">
      <button type="button" aria-label={t('weight_for', { name })} aria-describedby={warningId} className="rounded p-0.5 text-amber-300 hover:text-amber-200 focus-visible:outline focus-visible:outline-2 focus-visible:outline-amber-300"><TriangleAlert aria-hidden="true" className="size-4" /></button>
      <span id={warningId} role="tooltip" className="pointer-events-none absolute right-0 top-full z-30 mt-1 w-56 rounded-md border border-amber-700/70 bg-slate-950 px-3 py-2 text-left text-xs font-normal leading-relaxed text-amber-100 opacity-0 shadow-lg transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">{zeroWeightHint}</span>
    </span>}</div></div>
    <StatNumber label={t('objective_label')} name={name} goal={goal} value={value} onChange={onGoal} />
    <StatNumber label={t('min_strict')} name={name} goal={goal} value={minimumValue} placeholder="—" onChange={onMinimum} />
    <StatNumber label={t('max_strict')} name={name} goal={goal} value={maximumValue} placeholder="—" onChange={onMaximum} />
  </div>
}

function StatNumber({ label, name, goal, value, placeholder, onChange }: { label: string; name: string; goal: TargetGoal; value: number; placeholder?: string; onChange: (value: number) => void }) {
  const shown = value ? (goal.percent ? Number(value.toFixed(1)) : Number(value.toFixed(0))) : ''
  return <label className="goal-field"><span className="mobile-label">{label}</span><div className="number-field"><input aria-label={`${label} ${name}`} type="number" min={0} step={goal.percent ? .1 : 1} value={shown} placeholder={placeholder} onChange={event => onChange(Math.max(0, Number(event.target.value)))} /><span>{goal.percent ? '%' : t('points_short')}</span></div></label>
}
