import type { DamageGroup } from '../types'

// Give each comparable action one vote, regardless of damage scale or hit count.
export function averageDamageGain(groups?: DamageGroup[]): number | undefined {
  const scenarioCounts = new Map<string, number>()
  for (const { action_condition } of groups ?? []) {
    if (action_condition) {
      const key = action_condition.exclusive_group
      scenarioCounts.set(key, (scenarioCounts.get(key) ?? 0) + 1)
    }
  }
  let total = 0
  let count = 0
  for (const group of groups ?? []) {
    // An unknown buff state leaves both scenarios visible but gives neither a score.
    if (group.action_condition && (scenarioCounts.get(group.action_condition.exclusive_group) ?? 0) > 1) continue
    const current = group.current_max_tick ?? group.current_damage
    const build = group.build_max_tick ?? group.build_damage
    if (current === undefined || current <= 0 || !Number.isFinite(current) || !Number.isFinite(build)) continue
    total += build / current - 1
    count++
  }
  return count > 0 ? total / count : undefined
}
