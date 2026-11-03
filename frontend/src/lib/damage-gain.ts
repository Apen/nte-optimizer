import type { DamageGroup } from '../types'

// Give each comparable action one vote, regardless of damage scale or hit count.
export function averageDamageGain(groups?: DamageGroup[]): number | undefined {
  let total = 0
  let count = 0
  for (const group of groups ?? []) {
    const current = group.current_max_tick ?? group.current_damage
    const build = group.build_max_tick ?? group.build_damage
    if (current === undefined || current <= 0 || !Number.isFinite(current) || !Number.isFinite(build)) continue
    total += build / current - 1
    count++
  }
  return count > 0 ? total / count : undefined
}
