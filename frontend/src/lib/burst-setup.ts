import type { TeamPreview } from '../types'

type Effect = TeamPreview['conditional'][number]
type Action = 'ultimate' | 'skill' | 'redirect_press'

export type RotationStep = {
  characterID: number
  name: string
  action: Action
  effects: Effect[]
  shortestDuration?: number
}

export function planBurstSetup(effects: Effect[]): RotationStep[] {
  const steps: RotationStep[] = []
  const ordered = effects.filter(effect => effect.included).sort((left, right) =>
    (left.source_ids?.length ?? 1) - (right.source_ids?.length ?? 1),
  )
  for (const effect of ordered) {
    const action = effect.rotation_action ?? effect.trigger
    if (action !== 'ultimate' && action !== 'skill' && action !== 'redirect_press') continue
    const sources = effect.source_ids?.length ? effect.source_ids : [effect.character_id]
    let step = steps.find(candidate => candidate.action === action && sources.includes(candidate.characterID))
    if (!step) {
      step = { characterID: sources[0], name: effect.source_names?.[0] ?? '', action, effects: [] }
      steps.push(step)
    }
    step.effects.push(effect)
    if (effect.duration_seconds !== undefined && effect.duration_seconds > 0) {
      step.shortestDuration = Math.min(step.shortestDuration ?? Infinity, effect.duration_seconds)
    }
  }
  return steps.sort((left, right) => (right.shortestDuration ?? 0) - (left.shortestDuration ?? 0))
}
