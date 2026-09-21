import { AssetImage } from './equipment-cards'
import { Card, CardContent } from './ui/card'
import { t } from '../i18n'
import { formatRanking } from '../lib/format'
import type { Result } from '../types'

export function ScoreSummary({ result }: { result: Result }) {
  return <div className="score-strip">{result.solution.ranking && <ScoreMetric title={t('score_ranking')} value={result.solution.ranking.score} primary />}<ScoreMetric title={t('score_equipment')} value={result.solution.score} /><ScoreMetric title={t('score_modules')} value={result.solution.module_score} /><ScoreMetric title={t('score_cartridge')} value={result.solution.cartridge_score} /><ScoreMetric title={t('score_set_bonus')} value={result.solution.set_bonus_score} /></div>
}

function ScoreMetric({ title, value, primary = false }: { title: string; value: number; primary?: boolean }) {
  return <div className={`rounded-xl border p-4 ${primary ? 'border-pink-500 bg-pink-950/60' : 'border-slate-700 bg-slate-950/60'}`}><p className="text-xs font-bold text-slate-400">{title}</p><strong className={`mt-1 block text-3xl tabular-nums ${primary ? 'text-pink-200' : 'text-white'}`}>{formatRanking(value)}</strong></div>
}

export function AccountBuild({ result }: { result: Result }) {
  if (!result.character) return null
  const ascension = result.character.breakthroughLevel >= 6 ? t('ascension_max') : t('ascension', { level: result.character.breakthroughLevel })
  return <Card><CardContent className="grid gap-4 pt-5 md:grid-cols-2"><div className="flex items-center gap-3"><AssetImage source={`/game_ui/characters/${result.character.characterId}.png`} label={result.character.name} className="size-16 rounded-xl object-cover object-top" /><Info title={t('account_build')} value={result.character.name} detail={`${t('level')} ${result.character.level} · ${ascension} · ${t('awakenings').slice(0, -1)} ${result.character.awakenLevel}`} /></div><div className="flex items-center gap-3">{result.weapon && <AssetImage source={`/game_ui/forks/${result.weapon.forkId}.png`} label={result.weapon.name || result.weapon.forkId} className="size-16 rounded-xl object-contain" />}<Info title={t('arc_counted')} value={result.weapon?.name || t('unknown_arc')} detail={result.weapon ? `${t('level')} ${result.weapon.level} · ${t('upgrade')} ${result.weapon.star}` : undefined} /></div></CardContent></Card>
}

function Info({ title, value, detail }: { title: string; value: string; detail?: string }) {
  return <div><p className="text-xs text-slate-500">{title}</p><strong>{value}</strong>{detail && <p className="text-xs text-slate-400">{detail}</p>}</div>
}
