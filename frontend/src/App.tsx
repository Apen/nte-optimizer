import { useEffect, useMemo, useRef, useState } from 'react'
import { Ban, CheckCircle2, LockKeyhole, Play, Save, Square, X } from 'lucide-react'
import { AccountImportStatus, BuildWorkspace, CheckForUpdate, EquipmentCatalog, EquipBuildResult, LastOptimizationLog, Localization, OptimizeFlexibleSelection, Profiles, ResetProfileStrategy, SaveProfileSettings, SavedBuildResult, SetCharacterPriority, StopOptimization, Target } from '../wailsjs/go/main/DesktopApp'
import { AppSidebar, MobileNavigation, type Page } from './components/app-navigation'
import { CartridgePieceCard, ModulePieceCard } from './components/equipment-cards'
import { ConsoleGrid } from './components/console-grid'
import { SearchStatus, StatsEditor, weightForGoal } from './components/optimizer-configuration'
import { OptimizationLogDialog } from './components/optimization-log-dialog'
import { DamagePreview } from './components/damage-preview'
import { BuildRankingTable } from './components/build-ranking'
import { StatsComparison } from './components/stats-comparison'
import { AdvancedResultDetails } from './components/advanced-result-details'
import { UpdateDialog, type UpdateInfo } from './components/update-dialog'
import { AccountBuild } from './components/result-summary'
import { Button } from './components/ui/button'
import { Card, CardContent } from './components/ui/card'
import { ImportPage } from './pages/import-page'
import { CharactersPage } from './pages/characters-page'
import { CharacterStatePage } from './pages/character-state-page'
import { ArcsPage, CartridgesPage, ModulesPage, ResourcesPage } from './pages/inventory-pages'
import { formatRanking } from './lib/format'
import { PresentationProvider } from './presentation'
import type { AccountImportSummary, BuildWorkspace as Workspace, EquipmentCatalog as Catalog, LocalizationCatalog, OptimizationLog, OptimizedModule, Profile, Result, SearchProgress, TargetGoal, TargetPreset } from './types'
import { applyLocalization, initialLocale, setActiveLocale, t, type Locale } from './i18n'

function displayGoal(goal:TargetGoal) { return goal.percent ? goal.minimum*100 : goal.minimum }

export function App() {
  const [profiles,setProfiles]=useState<Profile[]>([])
  const [workspace,setWorkspace]=useState<Workspace>({characters:[]})
  const [catalog,setCatalog]=useState<Catalog>({modules:[],cartridges:[],arcs:[],resources:[]})
  const [selectedCharacter,setSelectedCharacter]=useState(0)
  const [profile,setProfile]=useState('')
  const [searchMode,setSearchMode]=useState<'fast'|'beta'>('fast')
  const [locale,setLocale]=useState<Locale>(()=>initialLocale(localStorage.getItem('nte-optimizer-locale')))
  const [presentation,setPresentation]=useState<LocalizationCatalog>({locale,ui:{},stats:{},qualities:{},geometries:{},stat_sources:{},damage:{},abilities:{}})
  const [preset,setPreset]=useState<TargetPreset>()
  const [goals,setGoals]=useState<Record<string,number>>({})
	const [maximums,setMaximums]=useState<Record<string,number>>({})
  const [tolerances,setTolerances]=useState<Record<string,number>>({})
  const [strictMinimums,setStrictMinimums]=useState<Record<string,number>>({})
  const [disabledGoals,setDisabledGoals]=useState<string[]>([])
  const [strictGoals,setStrictGoals]=useState<string[]>([])
  const [weightEdits,setWeightEdits]=useState<Record<string,{main_stats:string[];weights:Record<string,number>}>>({})
  const [running,setRunning]=useState(false)
  const [stopping,setStopping]=useState(false)
  const [status,setStatus]=useState('status_loading')
  const [progress,setProgress]=useState<SearchProgress>()
  const [result,setResult]=useState<Result>()
  const [pinnedModules,setPinnedModules]=useState<string[]>([])
  const [excludedModules,setExcludedModules]=useState<string[]>([])
  const [page,setPage]=useState<Page>('characters')
  const [importSummary,setImportSummary]=useState<AccountImportSummary>({has_import:false,characters:0,modules:0,cartridges:0,weapons:0})
  const [saveToast,setSaveToast]=useState('')
  const [optimizationLog,setOptimizationLog]=useState<OptimizationLog>()
  const [update,setUpdate]=useState<UpdateInfo>()
  const toastTimer=useRef<ReturnType<typeof setTimeout>|undefined>(undefined)
  const selectionTouched=useRef(false)
  const selectedProfile=useMemo(()=>profiles.find(item=>item.id===profile),[profiles,profile])
  const weights=useMemo(()=>weightEdits[profile]||{main_stats:selectedProfile?.main_stats||Object.keys(selectedProfile?.main_weights||{}),weights:selectedProfile?.weights||selectedProfile?.sub_weights||{}},[weightEdits,profile,selectedProfile])
  const importances=useMemo(()=>Object.fromEntries((preset?.goals||[]).map(goal=>[goal.property_id,weightForGoal(goal.property_id,weights)])),[preset,weights])
  const compatibleProfiles=useMemo(()=>profiles.filter(item=>item.character_id===selectedCharacter),[profiles,selectedCharacter])
  const availableMainStats=useMemo(()=>Array.from(new Set(catalog.cartridges.flatMap(item=>item.main_stats.map(stat=>stat.property_id)))),[catalog.cartridges])
  const changeWeight=(id:string,value:number)=>setWeightEdits(current=>({...current,[profile]:{...weights,weights:{...weights.weights,[id]:value}}}))
  const changeMainStats=(main_stats:string[])=>setWeightEdits(current=>({...current,[profile]:{...weights,main_stats}}))
  const showSaveToast=(message:string)=>{if(toastTimer.current)clearTimeout(toastTimer.current);setSaveToast(message);toastTimer.current=setTimeout(()=>setSaveToast(''),2200)}
  const persistSettings=async(nextGoals=goals,nextMaximums=maximums,nextTolerances=tolerances,nextDisabled=disabledGoals,nextStrict=strictGoals,nextMinimums=strictMinimums)=>{if(!profile)return;try{const savedGoals=Object.fromEntries((preset?.goals||[]).map(goal=>[goal.property_id,{target:(nextGoals[goal.property_id]||0)/(goal.percent?100:1),maximum:(nextMaximums[goal.property_id]||0)/(goal.percent?100:1),minimum:nextStrict.includes(goal.property_id)?(nextMinimums[goal.property_id]||0)/(goal.percent?100:1):undefined,tolerance:(nextTolerances[goal.property_id]||0)/100,strict_minimum:nextStrict.includes(goal.property_id),disabled:nextDisabled.includes(goal.property_id)}]));await SaveProfileSettings(profile,{...weights,goals:savedGoals});setProfiles(items=>items.map(item=>item.id===profile?{...item,main_stats:weights.main_stats,weights:weights.weights,saved_goals:savedGoals}:item));showSaveToast(t('profile_saved',{name:selectedProfile?.name||profile}))}catch(error){setStatus(String(error))}}
  const saveSettings=()=>persistSettings()
  const removeGoal=(id:string)=>{const next=disabledGoals.includes(id)?disabledGoals:[...disabledGoals,id];setDisabledGoals(next);void persistSettings(goals,maximums,tolerances,next)}
  const changeGoal=(id:string,value:number)=>setGoals(current=>({...current,[id]:value}))
  const resetWeights=async()=>{try{await ResetProfileStrategy(profile);const refreshed=await Profiles() as Profile[];setProfiles(refreshed);setWeightEdits(current=>{const next={...current};delete next[profile];return next});resetTargets();setStatus(t('status_reset',{name:selectedProfile?.name||profile}))}catch(error){setStatus(String(error))}}

  useEffect(() => { Profiles().then((items:Profile[]) => { setProfiles(items); setStatus(t('status_ready')) }).catch(error=>setStatus(String(error))) },[])
  useEffect(() => { let active=true;BuildWorkspace(locale).then((value:Workspace)=>{if(active)setWorkspace(value)}).catch(error=>{if(active)setStatus(String(error))});return()=>{active=false} },[locale])
  useEffect(() => { if(selectionTouched.current&&selectedCharacter)return;const next=workspace.characters.find(character=>profiles.some(item=>item.character_id===character.character_id))?.character_id||workspace.characters[0]?.character_id||0;if(next)setSelectedCharacter(next) },[workspace.characters,profiles,selectedCharacter])
  useEffect(() => { let active=true;EquipmentCatalog(locale).then((value:Catalog)=>{if(active)setCatalog(value)}).catch(error=>{if(active)setStatus(String(error))});return()=>{active=false} },[locale])
  useEffect(() => { AccountImportStatus().then((value:AccountImportSummary)=>setImportSummary(value)).catch(error=>setStatus(String(error))) },[])
  useEffect(() => { CheckForUpdate().then((value:UpdateInfo)=>{if(value.available)setUpdate(value)}).catch(()=>{}) },[])
  useEffect(() => { let active=true;setActiveLocale(locale);localStorage.setItem('nte-optimizer-locale',locale);Localization(locale).then((value:LocalizationCatalog)=>{if(!active)return;applyLocalization(locale,value);setPresentation(value)}).catch(error=>{if(active)setStatus(String(error))});return()=>{active=false} },[locale])
  useEffect(() => { const matches=profiles.filter(item=>item.character_id===selectedCharacter); setProfile(current=>matches.some(item=>item.id===current)?current:(matches[0]?.id||'')); if(selectedCharacter&&matches.length===0)setStatus(t('status_no_profile')) },[selectedCharacter,profiles])
  useEffect(() => { if(!profile){setPreset(undefined);setGoals({});setMaximums({});setDisabledGoals([]);setStrictGoals([]);setStrictMinimums({});return};let active=true;Target(profile).then((value:TargetPreset)=>{if(!active)return;const saved=profiles.find(item=>item.id===profile);setPreset(value);setGoals(Object.fromEntries(value.goals.map(goal=>[goal.property_id,saved?.saved_goals?.[goal.property_id]?.target!=null?saved.saved_goals[goal.property_id].target*(goal.percent?100:1):displayGoal(goal)])));setMaximums(Object.fromEntries(value.goals.map(goal=>[goal.property_id,saved?.saved_goals?.[goal.property_id]?.maximum!=null?(saved.saved_goals[goal.property_id].maximum||0)*(goal.percent?100:1):goal.maximum!=null?(goal.percent?goal.maximum*100:goal.maximum):0])));setTolerances(Object.fromEntries(value.goals.map(goal=>[goal.property_id,saved?.saved_goals?.[goal.property_id]?.tolerance!=null?saved.saved_goals[goal.property_id].tolerance*100:5])));setDisabledGoals(value.goals.filter(goal=>saved?.saved_goals?.[goal.property_id]?.disabled).map(goal=>goal.property_id));setStrictGoals(value.goals.filter(goal=>saved?.saved_goals?.[goal.property_id]?.strict_minimum).map(goal=>goal.property_id));setStrictMinimums(Object.fromEntries(value.goals.map(goal=>{const entry=saved?.saved_goals?.[goal.property_id];const amount=entry?.minimum??(entry?.strict_minimum?(entry.target*(1-(entry.tolerance||0))):0);return [goal.property_id,amount*(goal.percent?100:1)]}))) }).catch(error=>{if(active)setStatus(String(error))});return()=>{active=false} },[profile,profiles])
  useEffect(() => { setResult(undefined) }, [profile, goals, maximums, tolerances, disabledGoals, strictGoals, strictMinimums, weights, pinnedModules, excludedModules])
  useEffect(() => window.runtime.EventsOn('optimizer:progress',(value:SearchProgress)=>setProgress(value)),[])
  useEffect(()=>()=>{if(toastTimer.current)clearTimeout(toastTimer.current)},[])

  function resetTargets(save=false) { if(preset){const nextGoals=Object.fromEntries(preset.goals.map(goal=>[goal.property_id,displayGoal(goal)])),nextMaximums=Object.fromEntries(preset.goals.map(goal=>[goal.property_id,goal.maximum!=null?(goal.percent?goal.maximum*100:goal.maximum):0])),nextTolerances=Object.fromEntries(preset.goals.map(goal=>[goal.property_id,5]));setGoals(nextGoals);setMaximums(nextMaximums);setTolerances(nextTolerances);setDisabledGoals([]);setStrictGoals([]);setStrictMinimums({});if(save)void persistSettings(nextGoals,nextMaximums,nextTolerances,[],[],{}) } }
  async function optimize() {
    setRunning(true); setStopping(false); setResult(undefined); setProgress({visited:0,total:0,elapsed_ms:0,candidates:0,workers:0}); setStatus(t('status_searching'))
    try {
      const tuned=Object.fromEntries((preset?.goals||[]).filter(goal=>!disabledGoals.includes(goal.property_id)).map(goal=>[goal.property_id,{target:(goals[goal.property_id]||0)/(goal.percent?100:1),maximum:(maximums[goal.property_id]||0)/(goal.percent?100:1),tolerance:(tolerances[goal.property_id]||0)/100,importance:importances[goal.property_id]??1,minimum:strictGoals.includes(goal.property_id)?(strictMinimums[goal.property_id]||0)/(goal.percent?100:1):0,strict_minimum:strictGoals.includes(goal.property_id)}]))
      const value=await OptimizeFlexibleSelection(profile,false,locale,searchMode,tuned,pinnedModules,excludedModules,weights) as Result
      setResult(value); setStatus(value.solution.complete?t('status_optimal'):t('status_approximate'))
    } catch(error) { setStatus(String(error)) } finally { try{setOptimizationLog(await LastOptimizationLog() as OptimizationLog)}catch{}setRunning(false);setStopping(false) }
  }
  async function stop() { setStopping(true); setStatus(t('status_stop_requested')); await StopOptimization() }
  async function saveCharacterOrder(ids:number[]) { try{setWorkspace(await SetCharacterPriority(ids,locale) as Workspace)}catch(error){setStatus(String(error))} }
  async function moveCharacter(characterID:number,direction:-1|1) { const ids=workspace.characters.map(character=>character.character_id),index=ids.indexOf(characterID),target=index+direction;if(index<0||target<0||target>=ids.length)return;[ids[index],ids[target]]=[ids[target],ids[index]];await saveCharacterOrder(ids) }
  async function equipResult(build=result) { if(!build?.character)return;try{setWorkspace(await EquipBuildResult(build,locale) as Workspace);setStatus(t('status_equipped',{name:build.character.name}))}catch(error){setStatus(String(error))} }
  async function openCharacterBuild(id:number) { selectCharacter(id);setPage('builds');const character=workspace.characters.find(item=>item.character_id===id);if(!character?.build){setResult(undefined);return}try{setResult(await SavedBuildResult(id) as Result);setStatus(t('status_build_loaded',{name:character.name}))}catch(error){setResult(undefined);setStatus(t('status_load_error',{error:String(error)}))} }
  async function refreshImportedData(summary:AccountImportSummary) { const [nextWorkspace,nextCatalog]=await Promise.all([BuildWorkspace(locale),EquipmentCatalog(locale)]);setWorkspace(nextWorkspace as Workspace);setCatalog(nextCatalog as Catalog);setImportSummary(summary);setResult(undefined) }

  const togglePinned=(id:string)=>{setExcludedModules(items=>items.filter(item=>item!==id));setPinnedModules(items=>items.includes(id)?items.filter(item=>item!==id):[...items,id])}
  const toggleExcluded=(id:string)=>{setPinnedModules(items=>items.filter(item=>item!==id));setExcludedModules(items=>items.includes(id)?items.filter(item=>item!==id):[...items,id])}
  const selectCharacter=(id:number)=>{selectionTouched.current=true;setSelectedCharacter(id);setResult(undefined);setPinnedModules([]);setExcludedModules([])}
  return <PresentationProvider catalog={presentation}><div className="app-shell">{update&&<UpdateDialog update={update} onClose={()=>setUpdate(undefined)}/>}<AppSidebar page={page} onPage={setPage} characters={workspace.characters} catalog={catalog} importSummary={importSummary} locale={locale} onLocaleChange={setLocale}/><main className="app-main">
    <MobileNavigation page={page} onPage={setPage} locale={locale} onLocaleChange={setLocale}/>
    <p className="mb-3 text-right text-xs text-slate-500" role="status" aria-live="polite">{status}</p>
    {page==='characters'?<CharactersPage characters={workspace.characters} selected={selectedCharacter} onSelect={selectCharacter} onMove={moveCharacter} onReorder={ids=>void saveCharacterOrder(ids)} onBuild={openCharacterBuild} onState={id=>{selectCharacter(id);setPage('character-state')}}/>:page==='character-state'?<CharacterStatePage characterID={selectedCharacter} locale={locale} onBack={()=>setPage('characters')}/>:page==='cartridges'?<CartridgesPage items={catalog.cartridges} characters={workspace.characters}/>:page==='modules'?<ModulesPage items={catalog.modules} characters={workspace.characters}/>:page==='arcs'?<ArcsPage items={catalog.arcs}/>:page==='resources'?<ResourcesPage items={catalog.resources}/>:page==='import'?<ImportPage summary={importSummary} locale={locale} onImported={refreshImportedData}/>:<>
    <fieldset disabled={running} className="configuration-panel mb-3">
      <div className="config-title-row"><p className="eyebrow">{t('configuration')}</p>{(optimizationLog?.entries.length||0)>0&&<OptimizationLogDialog log={optimizationLog!}/>}</div>
      <div className="strategy-field">{workspace.characters.find(character=>character.character_id===selectedCharacter)&&<img className="strategy-character-image" src={`/game_ui/characters/${selectedCharacter}.png`} alt=""/>}<Field label={t('profile')}><select disabled={!compatibleProfiles.length} value={profile} onChange={e=>setProfile(e.target.value)}>{compatibleProfiles.length?compatibleProfiles.map(item=><option key={item.id} value={item.id}>{item.name||item.id}</option>):<option>{t('no_profile')}</option>}</select></Field><Field label={t('search_method')}><select value={searchMode} onChange={event=>{setSearchMode(event.target.value as 'fast'|'beta');setResult(undefined)}}><option value="fast">{t('search_fast')}</option><option value="beta">{t('search_beta')}</option></select></Field><Button className="strategy-search-button" disabled={!profile} onClick={optimize}><Play className="mr-2 size-4"/>{t('find_best_build')}</Button></div>
    </fieldset>
    {(pinnedModules.length>0||excludedModules.length>0)&&<Card className="mb-5 border-amber-900/60"><CardContent className="flex flex-wrap items-center gap-2 pt-5"><div className="mr-2"><strong>{t('piece_constraints')}</strong><p className="text-xs text-slate-500">{t('constraints_next_calculation')}</p></div>{pinnedModules.map(id=><ConstraintChip key={id} id={id} kind="locked" onRemove={()=>togglePinned(id)}/>) }{excludedModules.map(id=><ConstraintChip key={id} id={id} kind="excluded" onRemove={()=>toggleExcluded(id)}/>) }<Button className="ml-auto" variant="secondary" onClick={()=>{setPinnedModules([]);setExcludedModules([])}}><X className="mr-2 size-3"/>{t('clear_all')}</Button></CardContent></Card>}
    <fieldset disabled={running}><StatsEditor preset={preset} disabledGoals={disabledGoals} strictGoals={strictGoals} strictMinimums={strictMinimums} goals={goals} maximums={maximums} weights={weights} availableMain={availableMainStats} onGoal={changeGoal} onMaximum={(id,value)=>setMaximums(current=>({...current,[id]:value}))} onMinimum={(id,value)=>{setStrictMinimums(current=>({...current,[id]:value}));setStrictGoals(items=>value>0?(items.includes(id)?items:[...items,id]):items.filter(item=>item!==id))}} onWeight={changeWeight} onMainStats={changeMainStats} onRemove={removeGoal} onReset={resetWeights} onSave={saveSettings}/></fieldset>
    {running&&<div className="search-stop-row"><Button variant="destructive" disabled={stopping} onClick={stop}><Square className="mr-2 size-4"/>{stopping?t('stop_loading'):t('stop_and_keep')}</Button></div>}
    {running&&<SearchStatus progress={progress}/>}
    {result&&<ResultView result={result} onEquip={equipResult} pinned={pinnedModules} excluded={excludedModules} onPin={togglePinned} onExclude={toggleExcluded} goals={preset?.goals||[]} disabledGoals={disabledGoals} mainStats={weights.main_stats}/>}
    </>}
  </main>{saveToast&&<div className="save-toast fixed bottom-6 right-6 z-[100] flex items-center gap-3 rounded-xl border border-emerald-700 bg-emerald-950/95 px-4 py-3 text-sm font-semibold text-emerald-100 shadow-2xl shadow-black/50" role="status" aria-live="polite"><CheckCircle2 className="size-5 text-emerald-400"/><span>{saveToast}</span></div>}</div></PresentationProvider>
}

function Field({label,children}:{label:string;children:React.ReactNode}) { return <label className="grid gap-1.5 text-xs font-medium text-slate-400"><span>{label}</span>{children}</label> }

function ConstraintChip({id,kind,onRemove}:{id:string;kind:'locked'|'excluded';onRemove:()=>void}) { return <button onClick={onRemove} title={t('remove_constraint')} className={`flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-bold ${kind==='locked'?'border-emerald-700 bg-emerald-950/40 text-emerald-300':'border-red-800 bg-red-950/40 text-red-300'}`}>{kind==='locked'?<LockKeyhole className="size-3"/>:<Ban className="size-3"/>}{kind==='locked'?t('locked_state'):t('excluded_state')} · {id}<X className="size-3 opacity-60"/></button> }

function EmptyGear({label}:{label:string}) { return <div className="grid min-h-28 place-items-center rounded-xl border border-dashed border-slate-700 text-sm text-slate-500">{label}</div> }

function ResultView({result,onEquip,pinned,excluded,onPin,onExclude,goals,disabledGoals,mainStats}:{result:Result;onEquip:(result:Result)=>void;pinned:string[];excluded:string[];onPin:(id:string)=>void;onExclude:(id:string)=>void;goals:TargetGoal[];disabledGoals:string[];mainStats:string[]}) {
  const [rank,setRank]=useState(0)
  const [tab,setTab]=useState('stats')
  useEffect(()=>setRank(0),[result])
  const allRanked=[result,...(result.alternatives||[]).map(item=>({...result,solution:item.solution,modules:item.modules,cartridge:item.cartridge,cartridge_breakdown:item.cartridge_breakdown,stats:item.stats,set:item.set,goals:item.goals,conditional_goals:item.conditional_goals,damage:item.damage}))]
  const seenBuilds=new Set<string>()
  const ranked=allRanked.filter(build=>{const signature=buildResultSignature(build);if(seenBuilds.has(signature))return false;seenBuilds.add(signature);return true})
  const shown=ranked[rank]||ranked[0]
  return <section className="results-section grid gap-5" aria-label={t('results_aria')}>
    {ranked.length>1&&<BuildRankingTable builds={ranked.slice(0,50)} active={rank} onSelect={setRank} goals={goals} disabledGoals={disabledGoals} mainStats={mainStats}/>}
    <Card><CardContent className="grid gap-5 pt-5"><p className="eyebrow">{t('build_section')}</p><div className="result-toolbar"><div className="result-tabs" role="group" aria-label={t('build_details')}>{[['stats','tab_stats'],['damage','tab_damage'],['equipment','tab_equipment']].map(([id,key])=><button key={id} aria-pressed={tab===id} onClick={()=>setTab(id)}>{t(key)}{id==='equipment'&&<span>{shown.modules.length+1}</span>}</button>)}</div></div>
      {tab==='stats'&&<><StatsComparison result={shown}/><AdvancedResultDetails result={shown}/></>}
      {tab==='damage'&&(shown.damage&&shown.damage.status!=='unavailable'?<DamagePreview result={shown}/>:<EmptyGear label={t('damage_unavailable')}/>)}
      {tab==='equipment'&&<Card><CardContent className="pt-5"><div className="grid items-start gap-6 xl:grid-cols-3"><AccountBuild result={shown}/><ConsoleGrid result={shown}/><Cartridge result={shown}/></div><div className="mt-6 border-t border-slate-800 pt-5"><h2 className="mb-1 text-xl font-bold">{t('modules_to_equip')}</h2><p className="mb-4 text-sm text-slate-400">{t('modules_to_equip_description')}</p><div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">{shown.modules.map((entry,index)=><ModuleCard key={entry.module.local_id} entry={entry} index={index} result={shown} pinned={pinned.includes(entry.module.local_id)} excluded={excluded.includes(entry.module.local_id)} onPin={()=>onPin(entry.module.local_id)} onExclude={()=>onExclude(entry.module.local_id)}/>)}</div></div></CardContent></Card>}
      <div className="flex justify-end border-t border-slate-800 pt-5"><Button onClick={()=>onEquip(shown)}><Save className="mr-2 size-4"/>{t('equip_build',{number:rank+1})}</Button></div>
    </CardContent></Card>
  </section>
}

function buildResultSignature(build:Result) {
  const stats=Object.entries(build.stats.derived).sort(([a],[b])=>a.localeCompare(b)).map(([key,value])=>`${key}:${value.toFixed(6)}`).join('|')
  return `${build.solution.selected_set_id||build.set.id||''}|${stats}`
}

function WeightedScore({value,label}:{value:number;label:string}) { return <span className="rounded-lg border border-pink-800 bg-pink-950/40 px-3 py-2 text-right" title={t('stat_relevance_description')}><small className="block text-[9px] font-black uppercase tracking-wider text-pink-300">{label}</small><strong className="text-lg tabular-nums text-pink-100">{formatRanking(value)}</strong></span> }
function Cartridge({result}:{result:Result}) { if(!result.cartridge)return null; const owner=result.cartridge.equipped_character_id?{id:result.cartridge.equipped_character_id,name:result.cartridge_equipped_character_name||result.character?.name||t('owner_unknown')}:undefined; return <section><h2 className="mb-3 font-bold">{t('selected_cartridge')}</h2><CartridgePieceCard item={result.cartridge} title={result.set.name||result.cartridge.set_name} owner={owner?'':t('available_state')} ownerCharacter={owner} className="border-violet-800" trailing={<><WeightedScore value={result.cartridge_breakdown?.total||0} label={t('stat_relevance')}/><b className="badge-ok">{t('set_active')}</b></>}/></section> }

function ModuleCard({entry,index,result,pinned=false,excluded=false,onPin,onExclude}:{entry:OptimizedModule;index:number;result:Result;pinned?:boolean;excluded?:boolean;onPin?:()=>void;onExclude?:()=>void}) { const required=new Set(result.set.required_geometries); const owner=entry.module.equipped_character_id?{id:entry.module.equipped_character_id,name:entry.equipped_character_name||result.character?.name||t('owner_unknown')}:undefined; const label=t('module_label',{number:index+1}); return <ModulePieceCard item={entry.module} title={label} owner={owner?'':t('available_state')} ownerCharacter={owner} className={pinned?'border-emerald-600':excluded?'border-red-700 opacity-70':''} leading={<span className="grid size-8 place-items-center rounded-full bg-orange-500 font-black">{index+1}</span>} trailing={<><WeightedScore value={entry.breakdown.total} label={t('stat_relevance')}/>{required.has(entry.module.geometry)&&<span className="badge-ok">{t('piece_set')}</span>}</>} actions={onPin&&onExclude?<><Button variant="secondary" className={pinned?'border-emerald-600 text-emerald-300':''} onClick={onPin}><LockKeyhole className="mr-2 size-3"/>{pinned?t('locked_state'):t('lock')}</Button><Button variant="secondary" className={excluded?'border-red-700 text-red-300':''} onClick={onExclude}><Ban className="mr-2 size-3"/>{excluded?t('excluded_state'):t('exclude')}</Button></>:undefined}/> }
