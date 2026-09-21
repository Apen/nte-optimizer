import { currentIntlLocale, t } from '../i18n'

const percentStats = new Set(['CritBase','CritDamageBase','DamageUpGeneralBase','DamageUpIncantationBase','DamageUpChaosBase','DamageUpCosmosBase','DamageUpLakshanaBase','DamageUpNatureBase','DamageUpPsycheBase','DamageUpPsychicallyBase','AtkUp','HPMaxUp','HPUp','DefUp','HealUp','ShieldEfficiency','DefenseIgnoreBase','ChargeGetEfficiencyBase','ResistanceIgnoreChaosBase'])

export function formatStat(key: string, value: number) {
  if (percentStats.has(key)) return `${(value * 100).toFixed(1)} %`
  return Number.isInteger(value) ? value.toLocaleString(currentIntlLocale()) : value.toFixed(1)
}

export function formatRanking(value: number) {
  return value.toLocaleString(currentIntlLocale(), { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

export function formatDateTime(value?: string) {
  if (!value) return t('unknown_date')
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return t('unknown_date')
  return new Intl.DateTimeFormat(currentIntlLocale(), { dateStyle: 'short', timeStyle: 'short' }).format(date)
}
