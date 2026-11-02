// Explicit bridge declarations share application DTOs until Wails generation is available.
import type { AccountBuffPage, AccountImportSummary, BuildWorkspace as Workspace, CharacterGameSnapshot, EquipmentCatalog as Catalog, GoalTuning, LocalizationCatalog, OptimizationLog, Profile, ProfileSettings, Result, ScanLogEvent, TargetPreset, UpdateInfo } from '../../../src/types'
export function Profiles(): Promise<Profile[]>;
export function SaveProfileStrategy(profileID:string,settings:ProfileSettings):Promise<ProfileSettings>;
export function SaveProfileSettings(profileID:string,settings:ProfileSettings):Promise<ProfileSettings>;
export function ResetProfileStrategy(profileID:string):Promise<void>;
export function BuildWorkspace(language:string):Promise<Workspace>;
export function EquipmentCatalog(language:string):Promise<Catalog>;
export function Localization(language:string):Promise<LocalizationCatalog>;
export function CharacterGameState(characterID:number,language:string):Promise<CharacterGameSnapshot>;
export function EquipBuild(profileID:string,characterID:number,moduleIDs:string[],cartridgeID:string,stats:Record<string,number>,language:string):Promise<Workspace>;
export function EquipBuildResult(result:Result,language:string):Promise<Workspace>;
export function SavedBuildResult(characterID:number):Promise<Result>;
export function SetCharacterPriority(priority:number[],language:string):Promise<Workspace>;
export function Target(profileID:string):Promise<TargetPreset>;
export function AccountImportStatus():Promise<AccountImportSummary>;
export function CheckForUpdate():Promise<UpdateInfo>;
export function OpenDownloadURL(url:string):Promise<void>;
export function ScanAndImportAccount(language:string,seconds:number):Promise<AccountImportSummary>;
export function RecoverAndScanAccount(language:string,seconds:number):Promise<AccountImportSummary>;
export function LastScanLog():Promise<ScanLogEvent[]>;
export function OptimizeFlexibleSelection(profileID:string,ignorePriority:boolean,language:string,mode:string,goals:Record<string,GoalTuning>,pinnedModuleIDs:string[],excludedModuleIDs:string[],weights:ProfileSettings,arcForkID:string):Promise<Result>;
export function StopOptimization():Promise<boolean>;
export function LastOptimizationLog():Promise<OptimizationLog>;
export function AccountBuffs(language:string):Promise<AccountBuffPage>;
