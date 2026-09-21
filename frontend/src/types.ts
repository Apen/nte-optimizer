export type Profile = { id: string; character_id:number; name: string; grid_id: string; preferred_sets?: { set_id:string; priority:number }[]; main_weights?: Record<string,number>; sub_weights?: Record<string,number>; main_stats?:string[]; weights?:Record<string,number>; saved_goals?:Record<string,{target:number;maximum?:number;tolerance:number;strict_minimum?:boolean;disabled?:boolean}>; search_mode?:string; caps?: Record<string,{soft_cap:number;hard_cap:number;after_soft_scale:number}>; console_trait?: {area:number;property_id:string;value_per_module:number} }
export type SavedBuild = { character_id:number; profile_id:string; module_ids:string[]; cartridge_id?:string; stats?:Record<string,number>; updated_at:string }
export type WorkspaceCharacter = { character_id:number; name:string; level:number; awaken_level:number; priority:number; build?:SavedBuild }
export type BuildWorkspace = { characters:WorkspaceCharacter[] }
export type TargetGoal = { property_id: string; label: string; minimum: number; maximum?: number; percent: boolean }
export type TargetPreset = { name: string; goals: TargetGoal[] }
export type Cell = { x: number; y: number }
export type Placement = { module_id: string; cells: Cell[] }
export type Stat = { property_id: string; value: number; percent: boolean }
export type OptimizedModule = {
  module: { local_id: string; game_item_id?: string; geometry: string; main_stats: Stat[]; sub_stats: Stat[]; equipped_character_id?: number }
  breakdown: { total: number; contributions?: { property_id:string; weight:number; score:number }[] }
  equipped_character_name?: string
}
export type StatSummary = { completeness: string; missing_sources: string[]; sources: Record<string, Record<string, number>>; derived: Record<string, number>; derived_conditional: Record<string, number> }
export type DamageResult = { instance_id:string; name:string; category:string; non_crit:number; crit:number; expected:number; total:number; factors:{coefficient:number;crit_rate:number;crit_damage:number;damage_zone:number;defense_zone:number;resistance_zone:number}; warnings?:string[] }
export type DamageGroup = { id:string;name:string;description:string;category:string;build_damage:number;current_damage?:number;build_non_crit:number;current_non_crit?:number;build_crit:number;current_crit?:number;instances:number;max_stacks?:number;build_max_tick?:number;current_max_tick?:number;build_max_crit_tick?:number;current_max_crit_tick?:number }
export type DamageAnalysis = { status:string; enemy:{level:number;resistance:number;approximation?:string}; build:DamageResult[]; current?:DamageResult[]; groups?:DamageGroup[]; missing_inputs?:string[]; catalog_source:string }
export type GoalProgress = TargetGoal & { current: number; missing: number; progress: number; reached: boolean }
export type Result = {
  optimization_mode: string
  profile_id: string
  grid: { width: number; height: number; playable: Cell[] }
  modules: OptimizedModule[]
  cartridge?: { local_id: string; game_item_id: string; set_name: string; main_stats: Stat[]; sub_stats: Stat[]; equipped_character_id?:number }
  cartridge_breakdown?: { total: number; contributions?: { property_id:string; weight:number; score:number }[] }
  stats: StatSummary
  current_stats?: StatSummary
  set: { id: string; name: string; required_geometries: string[] }
  target?: { name: string }
  goals?: GoalProgress[]
  current_goals?: GoalProgress[]
  conditional_goals?: GoalProgress[]
  current_conditional_goals?: GoalProgress[]
  character?: { characterId: number; name: string; level: number; breakthroughLevel: number; awakenLevel: number }
  weapon?: { forkId: string; name: string; level: number; breakthrough: number; star: number }
  excluded_equipped: number
  weapon_conditional_note?: string
  include_equipped: boolean
  cartridge_equipped_character_name?: string
  alternatives?: OptimizationAlternative[]
  damage?: DamageAnalysis
  solution: OptimizationSolution
}
export type OptimizationSolution = { ranking?: { score:number; equipment:number; objectives:number; structure:number; contributions?: {property_id:string; source?:string; input_property_id?:string; value:number; target:number; importance:number; points:number}[] }; score: number; module_score: number; set_bonus_score: number; cartridge_score: number; selected_set_id?: string; set_matched_count?: number; complete: boolean; visited_states: number; placements: Placement[] }
export type OptimizationAlternative = { solution:OptimizationSolution; modules:OptimizedModule[]; cartridge?:Result['cartridge']; cartridge_breakdown?:Result['cartridge_breakdown']; stats:StatSummary; set:Result['set']; goals?:GoalProgress[]; conditional_goals?:GoalProgress[]; damage?:DamageAnalysis }
export type SearchProgress = { visited: number; total?: number; elapsed_ms: number; candidates: number; workers?:number; pruned_branches?:number; finished?:boolean }
export type OptimizationLog = { started_at:string; finished_at?:string; status:string; entries:{level:string;stage:string;message:string}[] }
export type InventoryModule = { local_id:string; game_item_id?:string; quality:string; set_id:string; set_name:string; geometry:string; area:number; level:number; main_stats:Stat[]; sub_stats:Stat[]; locked?:boolean; equipped_character_id?:number; equipped_placement?:{row:number;column:number} }
export type InventoryCartridge = { local_id:string; game_item_id?:string; set_id:string; set_name:string; quality:string; level:number; main_stats:Stat[]; sub_stats:Stat[]; locked?:boolean; equipped_character_id?:number }
export type InventoryArc = { id:{solt:number;serial:number}; forkId:string; name:string; quality:string; level:number; breakthrough:number; star:number; equippedCharacterId?:number; equipped_character_name?:string }
export type InventoryResource = { id:{solt:number;serial:number}; itemId:string; name:string; quality?:string; quantity:number }
export type EquipmentCatalog = { modules:InventoryModule[]; cartridges:InventoryCartridge[]; arcs:InventoryArc[]; resources:InventoryResource[] }
export type LocalizationCatalog = { locale:string; ui:Record<string,string>; stats:Record<string,string>; qualities:Record<string,string>; geometries:Record<string,string>; stat_sources:Record<string,string>; damage:Record<string,{name:string;description?:string}>; abilities?:Record<string,string> }
export type CharacterSkill = { abilityId:string; category:string; level:number }
export type CurrentCharacter = { characterId:number; name:string; codename:string; level:number; breakthroughLevel:number; awakenLevel:number; skills?:CharacterSkill[]; stats?:{maxHp?:number}; savedState?:{healthRatio?:number}; observedAtLogin?:{loaded:boolean;activeAwakeningLevels?:number[];activeAwakeningBuffs?:string[];activeEquipmentBuffs?:{equipmentId:number;buff:string;setId:string;setName:string;pieces:number;effect:string}[];activeWeaponBuffs?:{forkId:string;buff:string;star:number}[]} }
export type CharacterGameSnapshot = { character:CurrentCharacter; weapon?:InventoryArc; cartridges:InventoryCartridge[]; modules:InventoryModule[]; stats?:StatSummary; imported_at?:string }
export type AccountImportSummary = { has_import:boolean; imported_at?:string; source_generated_at?:string; characters:number; modules:number; cartridges:number; weapons:number; warnings?:string[] }
