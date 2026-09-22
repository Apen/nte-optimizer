// Generated-compatible Wails bridge kept explicit because Wails v2 binding
// generation currently fails under Go 1.25.
export function Profiles() {
  return window['go']['main']['DesktopApp']['Profiles']()
}

export function SaveProfileStrategy(profileID, settings) {
  return window['go']['main']['DesktopApp']['SaveProfileStrategy'](profileID, settings)
}

export function SaveProfileSettings(profileID, settings) {
  return window['go']['main']['DesktopApp']['SaveProfileSettings'](profileID, settings)
}

export function ResetProfileStrategy(profileID) {
  return window['go']['main']['DesktopApp']['ResetProfileStrategy'](profileID)
}

export function BuildWorkspace(language) {
  return window['go']['main']['DesktopApp']['BuildWorkspace'](language)
}

export function EquipmentCatalog(language) {
  return window['go']['main']['DesktopApp']['EquipmentCatalog'](language)
}

export function Localization(language) {
  return window['go']['main']['DesktopApp']['Localization'](language)
}

export function CharacterGameState(characterID, language) {
  return window['go']['main']['DesktopApp']['CharacterGameState'](characterID, language)
}

export function EquipBuild(profileID, characterID, moduleIDs, cartridgeID, stats, language) {
  return window['go']['main']['DesktopApp']['EquipBuild'](profileID, characterID, moduleIDs, cartridgeID, stats, language)
}

export function EquipBuildResult(result, language) {
  return window['go']['main']['DesktopApp']['EquipBuildResult'](result, language)
}

export function SavedBuildResult(characterID) {
  return window['go']['main']['DesktopApp']['SavedBuildResult'](characterID)
}

export function SetCharacterPriority(priority, language) {
  return window['go']['main']['DesktopApp']['SetCharacterPriority'](priority, language)
}

export function Target(profileID) {
  return window['go']['main']['DesktopApp']['Target'](profileID)
}

export function AccountImportStatus() {
  return window['go']['main']['DesktopApp']['AccountImportStatus']()
}

export function CheckForUpdate() {
  return window['go']['main']['DesktopApp']['CheckForUpdate']()
}

export function OpenDownloadURL(url) {
  return window['go']['main']['DesktopApp']['OpenDownloadURL'](url)
}

export function ScanAndImportAccount(language, seconds) {
  return window['go']['main']['DesktopApp']['ScanAndImportAccount'](language, seconds)
}

export function OptimizeFlexibleSelection(profileID, ignorePriority, language, mode, goals, pinnedModuleIDs, excludedModuleIDs, weights) {
  return window['go']['main']['DesktopApp']['OptimizeFlexibleSelection'](profileID, ignorePriority, language, mode, goals, pinnedModuleIDs, excludedModuleIDs, weights)
}

export function StopOptimization() {
  return window['go']['main']['DesktopApp']['StopOptimization']()
}

export function LastOptimizationLog() {
  return window['go']['main']['DesktopApp']['LastOptimizationLog']()
}
