import { useState } from 'react'
import { ChevronRight, Info, RotateCcw, X } from 'lucide-react'

import { Card, CardContent } from './ui/card'
import { Progress } from './ui/progress'
import { currentIntlLocale, t } from '../i18n'
import { usePresentation } from '../presentation'
import type { SearchProgress, TargetGoal } from '../types'

type WeightSettings = { main_stats: string[]; weights: Record<string, number> }

export function weightForGoal(property: string, settings: Pick<WeightSettings, 'weights'>) {
  const families: Record<string, string[]> = {
    AtkFinal: ['AtkUp', 'AtkAdd'],
    HPFinal: ['HPMaxUp', 'HPMaxAdd'],
    DefFinal: ['DefUp', 'DefAdd'],
  }
  return Math.max(0, ...(families[property] || [property]).map(key => settings.weights[key] || 0))
}

export function normalizeGoalWeights(settings: WeightSettings): WeightSettings {
  const weights = { ...settings.weights }
  const legacyAliases: Record<string, string> = {
    AtkBase: 'AtkUp',
    HPMaxBase: 'HPMaxUp',
    HPUp: 'HPMaxUp',
    DefBase: 'DefUp',
  }
  for (const [legacy, supported] of Object.entries(legacyAliases)) {
    const value = weights[legacy]
    if (value === undefined) continue
    weights[supported] = Math.max(weights[supported] || 0, value)
    delete weights[legacy]
  }
  return { ...settings, weights }
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
  return <Card className="mb-5 border-slate-700"><CardContent className="grid gap-3 pt-5"><div className="flex flex-wrap items-center justify-between gap-3"><div><strong>{t('search_best_build')}</strong><p className="text-xs text-slate-500">{t('workers_candidates', { workers: progress?.workers || 1, candidates: (progress?.candidates || 0).toLocaleString(currentIntlLocale()), rate: Math.round(rate).toLocaleString(currentIntlLocale()), pruned: (progress?.pruned_branches || 0).toLocaleString(currentIntlLocale()) })}</p></div><span className="text-sm tabular-nums text-slate-400">{progressLabel}{total > visited && !progress?.pruned_branches ? ` · ${t('no_cuts_eta', { eta })}` : ''}</span></div><Progress value={percent} /></CardContent></Card>
}

type StatsEditorProps = {
  goalDefinitions: TargetGoal[]
  availableGoals: TargetGoal[]
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
  onAddGoal: (goal: TargetGoal) => void
  onReset: () => Promise<void>
  onSave: () => Promise<void>
}

export function StatsEditor({ goalDefinitions, availableGoals, disabledGoals, strictGoals, strictMinimums, goals, maximums, weights, availableMain, onGoal, onMaximum, onMinimum, onWeight, onMainStats, onRemove, onAddGoal, onReset, onSave }: StatsEditorProps) {
  const visibleGoals = goalDefinitions.filter(goal => !disabledGoals.includes(goal.property_id))
  const visibleGoalIDs = new Set(visibleGoals.map(goal => goal.property_id))
  const selectableGoals = availableGoals.filter(goal => !visibleGoalIDs.has(goal.property_id)).sort((a, b) => a.label.localeCompare(b.label, currentIntlLocale()))
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const activeConstraints = visibleGoals.reduce((count, goal) => count + Number(strictGoals.includes(goal.property_id)) + Number((maximums[goal.property_id] || 0) > 0), 0)
  return <section className="configuration-panel goals-panel" onBlur={event => { if (!event.relatedTarget || !event.currentTarget.contains(event.relatedTarget as Node)) void onSave() }}>
    <div className="section-heading"><p className="eyebrow">{t('stats_section')}</p><button className="text-action" onClick={() => void onReset()}><RotateCcw className="size-3.5" />{t('reset')}</button></div>
    <MainStatSelector selected={weights.main_stats} available={availableMain} onChange={onMainStats} />
    <div className="goal-matrix stats-matrix">
      <div className="goal-table-heading"><span>{t('statistic')}</span><span className="weight-heading"><span>{t('weight')}</span><button type="button" aria-label={t('weight_help')} title={t('weight_help')}><Info className="size-3" aria-hidden="true" /></button></span><span>{t('objective_label')}</span></div>
      {visibleGoals.map(goal => <StatConfigRow key={goal.property_id} goal={goal} value={goals[goal.property_id] || 0} weight={weightForGoal(goal.property_id, weights)} onGoal={value => onGoal(goal.property_id, value)} onWeight={value => onWeight(weightKeyForGoal(goal.property_id, weights), value)} onRemove={() => onRemove(goal.property_id)} />)}
      {visibleGoals.length === 0 && <p className="py-8 text-center text-sm text-slate-500">{t('no_stats')}</p>}
    </div>
    <div className="goal-add-toolbar"><span className="field-caption">{t('stats_objectives')}</span><select aria-label={t('add_objective_stat')} value="" onChange={event => { const goal = selectableGoals.find(item => item.property_id === event.target.value); if (goal) onAddGoal(goal) }}><option value="">{t('add_objective_stat')}</option>{selectableGoals.map(goal => <option key={goal.property_id} value={goal.property_id}>{goal.label}</option>)}</select></div>
    <button type="button" className="advanced-constraints-toggle" aria-expanded={advancedOpen} aria-controls="strict-constraints-panel" onClick={() => setAdvancedOpen(open => !open)}>
      <ChevronRight aria-hidden="true" className="advanced-constraints-chevron size-4" />
      <span className="advanced-constraints-title">{t('advanced_constraints')}</span>
      {activeConstraints > 0 ? <span className="advanced-constraints-count">{t('active_constraints_count', { count: activeConstraints })}</span> : <span className="advanced-constraints-description">{t('strict_limits_description')}</span>}
    </button>
    <div id="strict-constraints-panel" className="advanced-constraints-fields" aria-label={t('advanced_constraints')} hidden={!advancedOpen}>
      <div className="advanced-constraints-heading"><span>{t('statistic')}</span><span>{t('min_strict')}</span><span>{t('max_strict')}</span></div>
      {visibleGoals.map(goal => <StrictConstraintRow key={goal.property_id} goal={goal} minimumValue={strictGoals.includes(goal.property_id) ? strictMinimums[goal.property_id] || 0 : 0} maximumValue={maximums[goal.property_id] || 0} onMinimum={value => onMinimum(goal.property_id, value)} onMaximum={value => onMaximum(goal.property_id, value)} />)}
    </div>
  </section>
}

function MainStatSelector({ selected, available, onChange }: { selected: string[]; available: string[]; onChange: (ids: string[]) => void }) {
  const { stats: labels } = usePresentation()
  const choices = available.filter(id => !selected.includes(id)).sort((a, b) => (labels[a] || a).localeCompare(labels[b] || b, currentIntlLocale()))
  return <div className="main-stat-selector"><span className="field-caption">{t('stats_main')}</span><div className="main-stat-tags">{selected.map(id => <button key={id} onClick={() => onChange(selected.filter(value => value !== id))}>{labels[id] || id}<X className="size-3" /></button>)}</div>{choices.length > 0 && <select aria-label={t('add_main_stat')} value="" onChange={event => { if (event.target.value) onChange([...selected, event.target.value]) }}><option value="">{t('add_option')}</option>{choices.map(id => <option key={id} value={id}>{labels[id] || id}</option>)}</select>}</div>
}

export function weightKeyForGoal(property: string, settings: Pick<WeightSettings, 'weights'>) {
  const families: Record<string, string[]> = {
    AtkFinal: ['AtkUp', 'AtkAdd'],
    HPFinal: ['HPMaxUp', 'HPMaxAdd'],
    DefFinal: ['DefUp', 'DefAdd'],
  }
  return (families[property] || [property]).sort((a, b) => (settings.weights[b] || 0) - (settings.weights[a] || 0))[0]
}

function StatConfigRow({ goal, value, weight, onGoal, onWeight, onRemove }: { goal: TargetGoal; value: number; weight: number; onGoal: (value: number) => void; onWeight: (value: number) => void; onRemove: () => void }) {
  const { stats: labels } = usePresentation()
  const name = labels[goal.property_id] || goal.label
  const reference = t('reference', { value: `${goal.percent ? goal.minimum * 100 : goal.minimum}${goal.percent ? ' %' : ''}` })
  return <div className="goal-row"><div className="goal-name flex items-start justify-between gap-2"><span><strong className="block" title={reference}>{name}</strong></span><button className="rounded-md p-1 text-slate-500 hover:bg-red-950 hover:text-red-300" onClick={onRemove} title={t('remove_goal', { name })} aria-label={t('remove_goal', { name })}><X className="size-4" /></button></div>
    <div className="goal-field goal-weight-field"><span className="mobile-label">{t('weight')}</span><div className="number-field"><input aria-label={t('weight_for', { name })} type="number" min={0} max={10} step={0.05} value={weight} onChange={event => onWeight(Math.min(10, Math.max(0, Number(event.target.value))))} /></div>{weight === 0 && <small>{t('zero_weight_goal_hint')}</small>}</div>
    <StatNumber label={t('objective_label')} name={name} goal={goal} value={value} onChange={onGoal} />
  </div>
}

function StrictConstraintRow({ goal, minimumValue, maximumValue, onMinimum, onMaximum }: { goal: TargetGoal; minimumValue: number; maximumValue: number; onMinimum: (value: number) => void; onMaximum: (value: number) => void }) {
  const { stats: labels } = usePresentation()
  const name = labels[goal.property_id] || goal.label
  return <div className="advanced-constraint-row"><strong>{name}</strong><StatNumber label={t('min_strict')} name={name} goal={goal} value={minimumValue} placeholder="—" onChange={onMinimum} /><StatNumber label={t('max_strict')} name={name} goal={goal} value={maximumValue} placeholder="—" onChange={onMaximum} /></div>
}

function StatNumber({ label, name, goal, value, placeholder, onChange }: { label: string; name: string; goal: TargetGoal; value: number; placeholder?: string; onChange: (value: number) => void }) {
  const shown = value ? (goal.percent ? Number(value.toFixed(1)) : Number(value.toFixed(0))) : ''
  return <label className="goal-field"><span className="mobile-label">{label}</span><div className="number-field"><input aria-label={`${label} ${name}`} type="number" min={0} step={goal.percent ? .1 : 1} value={shown} placeholder={placeholder} onChange={event => onChange(Math.max(0, Number(event.target.value)))} /><span>{goal.percent ? '%' : t('points_short')}</span></div></label>
}
