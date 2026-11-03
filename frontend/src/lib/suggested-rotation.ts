import characters from '../../../data/game/characters/decode.json'
import rotations from '../../../data/game/combat/rotations.json'
import type { Result } from '../types'
import { planBurstSetup, type RotationStep } from './burst-setup'

type TemplateStep = {
  id: string
  actor: string | number
  label?: string
  prefer?: number[]
  consumes?: string[]
  requires?: string[]
  requires_reaction?: boolean
}

type Template = {
  reaction?: string
  required_steps: string[]
  steps: TemplateStep[]
}

type SuggestedStep = {
  id: string
  characterID: number
  label?: string
  buff?: RotationStep
}

const templates: Record<string, Template> = rotations.characters
const esperTypes: Record<string, { esper_type: string }> = characters.characters

export function suggestRotation(result: Result) {
  const activeID = result.character?.characterId
  const template = activeID === undefined ? undefined : templates[activeID]
  if (!template || activeID === undefined) return undefined

  const allies = (result.team_allies ?? []).map(ally => ally.character_id)
  const assigned = template.steps.map(step => {
    const actor = step.actor
    const characterID = actor === 'active' ? activeID
      : typeof actor === 'number' ? allies.find(id => id === actor)
      : [...(step.prefer ?? []), ...allies].find(id => allies.includes(id) && esperTypes[id]?.esper_type === actor)
    return { ...step, characterID }
  })
  const available = new Set(assigned.filter(step => step.characterID !== undefined).map(step => step.id))
  const reactionReady = template.required_steps.every(id => available.has(id))
  const playable = assigned.filter(step => (step.requires ?? []).every(id => available.has(id))
    && (reactionReady || !step.requires_reaction))
  const remainingEffects = (result.buff_effects ?? []).flatMap(effect => {
    const action = effect.rotation_action ?? effect.trigger
    const sources = effect.source_ids?.length ? effect.source_ids : [effect.character_id]
    if (sources.some(id => playable.some(step => step.characterID === id && step.consumes?.includes(action)))) return []
    return [effect]
  })
  const steps = playable.flatMap<SuggestedStep>(step => {
    if (step.actor === 'buffs') {
      return planBurstSetup(remainingEffects).map(buff => ({
        id: `buff:${buff.characterID}:${buff.action}`, characterID: buff.characterID, buff,
      }))
    }
    if (step.characterID === undefined) return []
    return [{ id: step.id, characterID: step.characterID, label: step.label }]
  })

  return { steps, missingReaction: reactionReady ? undefined : template.reaction }
}
