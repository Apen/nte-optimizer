import { AssetImage, OwnerBadge, type Owner } from './equipment-cards'
import { Card, CardContent } from './ui/card'
import { t } from '../i18n'
import type { Result } from '../types'

export function AccountBuild({ result, characters }: { result: Result; characters: { character_id: number; name: string }[] }) {
  if (!result.character) return null
  const ascension = result.character.breakthroughLevel >= 6 ? t('ascension_max') : t('ascension', { level: result.character.breakthroughLevel })
  const equippedCharacter = result.weapon?.equippedCharacterId ? characters.find(character => character.character_id === result.weapon?.equippedCharacterId) : undefined
  const arcOwner: Owner | undefined = result.weapon?.equippedCharacterId ? { id: result.weapon.equippedCharacterId, name: equippedCharacter?.name || t('owner_unknown') } : undefined
  return <Card><CardContent className="grid gap-4 pt-5"><div className="flex items-center gap-3"><AssetImage source={`/game_ui/characters/${result.character.characterId}.png`} label={result.character.name} className="size-16 rounded-xl object-cover object-top" /><Info title={t('account_build')} value={result.character.name} detail={`${t('level')} ${result.character.level} · ${ascension} · ${t('awakenings').slice(0, -1)} ${result.character.awakenLevel}`} /></div><div className="flex items-center gap-3 border-t border-slate-800 pt-4">{result.weapon && <AssetImage source={`/game_ui/forks/${result.weapon.forkId}.png`} label={result.weapon.name || result.weapon.forkId} className="size-16 rounded-xl object-contain" />}<Info title={t('arc_counted')} value={result.weapon?.name || t('unknown_arc')} detail={result.weapon ? `${t('level')} ${result.weapon.level} · ${t('upgrade')} ${result.weapon.star}` : undefined} ownerCharacter={arcOwner} /></div></CardContent></Card>
}

function Info({ title, value, detail, ownerCharacter }: { title: string; value: string; detail?: string; ownerCharacter?: Owner }) {
  return <div><p className="text-xs text-slate-500">{title}</p><strong>{value}</strong>{detail && <p className="text-xs text-slate-400">{detail}</p>}{ownerCharacter && <OwnerBadge owner={ownerCharacter} />}</div>
}
