import { BuildRotation } from './build-rotation'
import { BuildBuffs } from './build-buffs'
import { useEffect, useState } from 'react'
import { Ban, LockKeyhole, Save } from 'lucide-react'
import { CartridgePieceCard, ModulePieceCard } from './equipment-cards'
import { ConsoleGrid } from './console-grid'
import { DamagePreview } from './damage-preview'
import { BuildRankingTable, ScoreDetailsDialog, buildResultSignature, limitBuildResults } from './build-ranking'
import { StatsComparison } from './stats-comparison'
import { AdvancedResultDetails } from './advanced-result-details'
import { OptimizationLogDialog } from './optimization-log-dialog'
import { AccountBuild } from './result-summary'
import { Button } from './ui/button'
import { Card, CardContent } from './ui/card'
import { formatRanking } from '../lib/format'
import { t } from '../i18n'
import type { Result, SavedBuild, WorkspaceCharacter, OptimizationLog, TargetGoal, OptimizedModule } from '../types'

function EmptyGear({label}:{label:string}) { return <div className="grid min-h-28 place-items-center rounded-xl border border-dashed border-slate-700 text-sm text-slate-500">{label}</div> }

export function ResultView({result,savedBuild,characters,optimizationLog,profileName,onEquip,pinned,excluded,onPin,onExclude,goals,disabledGoals,mainStats}:{result:Result;savedBuild?:SavedBuild;characters:WorkspaceCharacter[];optimizationLog?:OptimizationLog;profileName:string;onEquip:(result:Result)=>void;pinned:string[];excluded:string[];onPin:(id:string)=>void;onExclude:(id:string)=>void;goals:TargetGoal[];disabledGoals:string[];mainStats:string[]}) {
  const [rank,setRank]=useState(0)
  const [tab,setTab]=useState('stats')
  const allRanked=[result,...(result.alternatives||[]).map(item=>({...result,solution:item.solution,modules:item.modules,weapon:item.weapon,weapon_conditional_note:item.weapon_conditional_note,cartridge:item.cartridge,cartridge_breakdown:item.cartridge_breakdown,stats:item.stats,buff_effects:item.buff_effects,set:item.set,goals:item.goals,conditional_goals:item.conditional_goals,damage:item.damage}))]
  const seenBuilds=new Set<string>()
  const ranked=allRanked.filter(build=>{const signature=buildResultSignature(build);if(seenBuilds.has(signature))return false;seenBuilds.add(signature);return true}).sort((left,right)=>(right.solution.ranking?.score??right.solution.score)-(left.solution.ranking?.score??left.solution.score))
  useEffect(()=>setRank(savedBuild?findSavedBuildRank(ranked,savedBuild,result):0),[result,savedBuild])
  const shown=ranked[rank]||ranked[0]
  return <section className="results-section grid gap-5" aria-label={t('results_aria')}>
    {ranked.length>1&&<BuildRankingTable builds={limitBuildResults(ranked)} active={rank} onSelect={setRank} goals={goals} disabledGoals={disabledGoals} mainStats={mainStats}/>}
    <div className="selected-build-summary">{shown.character && <img src={`/game_ui/characters/${shown.character.characterId}.png`} alt=""/>}<div className="selected-build-identity"><p className="eyebrow">{t('selected_build_rank',{number:String(rank+1).padStart(2,'0')})}</p><strong>{shown.character?.name || `${t('build')} #${String(rank+1).padStart(2,'0')}`}</strong><span>{profileName}</span></div><div className="selected-build-score"><small>{t('ranking')}</small><strong>{formatRanking(shown.solution.ranking?.score ?? shown.solution.score)}</strong></div><div className="selected-build-actions"><ScoreDetailsDialog build={shown} buildNumber={rank+1}/><AdvancedResultDetails result={shown}/>{(optimizationLog?.entries.length||0)>0&&<OptimizationLogDialog log={optimizationLog!}/>}</div></div>
    <Card><CardContent className="grid gap-5 pt-5"><p className="eyebrow">{t('build_section')}</p><div className="result-toolbar"><div className="result-tabs" role="group" aria-label={t('build_details')}>{[['stats','tab_stats'],['damage','tab_damage'],['buffs','tab_buffs'],['rotation','tab_rotation'],['equipment','tab_equipment']].map(([id,key])=><button key={id} aria-pressed={tab===id} onClick={()=>setTab(id)}>{t(key)}{id==='equipment'&&<span>{shown.modules.length+(shown.cartridge?1:0)+(shown.weapon?1:0)}</span>}</button>)}</div></div>
      {tab==='stats'&&<StatsComparison result={shown}/>}
      {tab==='buffs'&&<BuildBuffs result={shown}/>}
      {tab==='rotation'&&<BuildRotation result={shown} characters={characters}/>}
      {tab==='damage'&&(shown.damage&&shown.damage.status!=='unavailable'?<DamagePreview result={shown}/>:<EmptyGear label={t('damage_unavailable')}/>)}
    {tab==='equipment'&&<Card><CardContent className="pt-5"><div className="grid items-start gap-6 xl:grid-cols-3"><AccountBuild result={shown} characters={characters}/><ConsoleGrid result={shown}/><Cartridge result={shown}/></div><div className="mt-6 border-t border-slate-800 pt-5"><h2 className="mb-1 text-xl font-bold">{t('modules_to_equip')}</h2><p className="mb-4 text-sm text-slate-400">{t('modules_to_equip_description')}</p><div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{shown.modules.map((entry,index)=><ModuleCard key={entry.module.local_id} entry={entry} index={index} result={shown} pinned={pinned.includes(entry.module.local_id)} excluded={excluded.includes(entry.module.local_id)} onPin={()=>onPin(entry.module.local_id)} onExclude={()=>onExclude(entry.module.local_id)}/>)}</div></div></CardContent></Card>}
      <div className="flex justify-end border-t border-slate-800 pt-5"><Button onClick={()=>onEquip(shown)}><Save className="mr-2 size-4"/>{t('equip_build',{number:rank+1})}</Button></div>
    </CardContent></Card>
  </section>
}

function findSavedBuildRank(builds:Result[],savedBuild:SavedBuild,savedResult:Result) {
  const savedModules=[...savedBuild.module_ids].sort()
  const savedBuildRank=builds.findIndex(build=>{
    const moduleIDs=build.modules.map(entry=>entry.module.local_id).sort()
    return moduleIDs.length===savedModules.length&&moduleIDs.every((id,index)=>id===savedModules[index])
      &&(build.cartridge?.local_id||'')===(savedBuild.cartridge_id||'')
      &&(build.solution.selected_weapon_id||'')===(savedBuild.arc_id||'')
  })
  if(savedBuildRank>=0)return savedBuildRank
  return Math.max(0,builds.findIndex(build=>buildResultSignature(build)===buildResultSignature(savedResult)))
}

function WeightedScore({value,label}:{value:number;label:string}) { return <span className="weighted-score" title={t('stat_relevance_description')}><small className="block">{label}</small><strong className="text-lg tabular-nums">{formatRanking(value)}</strong></span> }
function Cartridge({result}:{result:Result}) { if(!result.cartridge)return null; const owner=result.cartridge.equipped_character_id?{id:result.cartridge.equipped_character_id,name:result.cartridge_equipped_character_name||result.character?.name||t('owner_unknown')}:undefined; return <section><h2 className="mb-3 font-bold">{t('selected_cartridge')}</h2><CartridgePieceCard item={result.cartridge} details={result.cartridge.simulated_main_stat_level?[t('cartridge_simulation_detail',{target:result.cartridge.simulated_main_stat_level})]:undefined} meta={result.cartridge.simulated_main_stat_level?t('cartridge_upgrade_required',{current:result.cartridge.level||0,target:result.cartridge.simulated_main_stat_level}):undefined} title={result.set.name||result.cartridge.set_name} owner={owner?'':t('available_state')} ownerCharacter={owner} className="border-violet-800" trailing={<><WeightedScore value={result.cartridge_breakdown?.total||0} label={t('stat_relevance')}/><b className="badge-ok">{t('set_active')}</b></>}/></section> }

function ModuleCard({entry,index,result,pinned=false,excluded=false,onPin,onExclude}:{entry:OptimizedModule;index:number;result:Result;pinned?:boolean;excluded?:boolean;onPin?:()=>void;onExclude?:()=>void}) { const required=new Set(result.set.required_geometries); const owner=entry.module.equipped_character_id?{id:entry.module.equipped_character_id,name:entry.equipped_character_name||result.character?.name||t('owner_unknown')}:undefined; const label=t('module_label',{number:index+1}); return <ModulePieceCard item={entry.module} title={label} owner={owner?'':t('available_state')} ownerCharacter={owner} className={pinned?'border-emerald-600':excluded?'border-red-700 opacity-70':''} leading={<span className="grid size-8 place-items-center rounded-full bg-orange-500 font-black">{index+1}</span>} trailing={<><WeightedScore value={entry.breakdown.total} label={t('stat_relevance')}/>{required.has(entry.module.geometry)&&<span className="badge-ok">{t('piece_set')}</span>}</>} actions={onPin&&onExclude?<><Button variant="secondary" className={pinned?'border-emerald-600 text-emerald-300':''} onClick={onPin}><LockKeyhole className="mr-2 size-3"/>{pinned?t('locked_state'):t('lock')}</Button><Button variant="secondary" className={excluded?'border-red-700 text-red-300':''} onClick={onExclude}><Ban className="mr-2 size-3"/>{excluded?t('excluded_state'):t('exclude')}</Button></>:undefined}/> }
