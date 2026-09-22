package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	appservice "nte-optimizer/internal/app"
	"nte-optimizer/internal/buildinfo"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scannerlauncher"
	"nte-optimizer/internal/target"
	"nte-optimizer/internal/updatecheck"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type DesktopApp struct {
	ctx         context.Context
	projectDir  string
	stateDir    string
	service     appservice.OptimizerService
	searchMu    sync.Mutex
	searchStop  context.CancelFunc
	searchID    uint64
	scanMu      sync.Mutex
	scanStop    context.CancelFunc
	scanID      uint64
	buildMu     sync.Mutex
	logMu       sync.Mutex
	lastLog     OptimizationLog
	logProgress optimizer.SearchProgress
}

func NewDesktopApp(projectDir string) *DesktopApp {
	return NewDesktopAppWithStateDir(projectDir, projectDir)
}

func NewDesktopAppWithStateDir(projectDir, stateDir string) *DesktopApp {
	return &DesktopApp{
		projectDir: projectDir,
		stateDir:   stateDir,
		service:    appservice.NewOptimizerService(filepath.Join(projectDir, "data")),
	}
}

func (a *DesktopApp) startup(ctx context.Context) { a.ctx = ctx }

func (a *DesktopApp) shutdown(context.Context) {
	a.StopScan()
	a.StopOptimization()
}

func (a *DesktopApp) Profiles() ([]appservice.ProfileSummary, error) {
	profiles, err := a.service.Profiles()
	if err != nil {
		return nil, err
	}
	overrides, err := appservice.LoadStrategyOverrides(a.stateDir)
	if err != nil {
		return nil, err
	}
	for index := range profiles {
		if settings, ok := overrides.Profiles[profiles[index].ID]; ok {
			profiles[index].MainStats = settings.MainStats
			profiles[index].Weights = settings.Weights
			profiles[index].SavedGoals = settings.Goals
		}
	}
	return profiles, nil
}

func (a *DesktopApp) SaveProfileSettings(profileID string, settings appservice.WeightOverrides) (appservice.WeightOverrides, error) {
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	return appservice.SaveProfileSettings(a.stateDir, profileID, settings)
}

func (a *DesktopApp) SaveProfileStrategy(profileID string, settings appservice.WeightOverrides) (appservice.WeightOverrides, error) {
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	profiles, err := a.service.Profiles()
	if err != nil {
		return appservice.WeightOverrides{}, err
	}
	found := false
	for _, profile := range profiles {
		if profile.ID == profileID {
			found = true
			break
		}
	}
	if !found {
		return appservice.WeightOverrides{}, fmt.Errorf("unknown profile %q", profileID)
	}
	return appservice.SaveProfileStrategy(a.stateDir, profileID, settings)
}

func (a *DesktopApp) ResetProfileStrategy(profileID string) error {
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	return appservice.DeleteProfileStrategy(a.stateDir, profileID)
}

func (a *DesktopApp) BuildWorkspace(language string) (appservice.BuildWorkspace, error) {
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	return appservice.LoadBuildWorkspace(a.stateDir, a.service.DataDir, language)
}

func (a *DesktopApp) EquipmentCatalog(language string) (appservice.EquipmentCatalog, error) {
	return appservice.LoadEquipmentCatalog(a.stateDir, a.service.DataDir, language)
}

func (a *DesktopApp) Localization(language string) (ntelocale.PresentationCatalog, error) {
	return ntelocale.LoadPresentation(a.service.DataDir, language)
}

func (a *DesktopApp) CharacterGameState(characterID int, language string) (appservice.CharacterGameState, error) {
	return appservice.LoadCharacterGameState(a.stateDir, a.service.DataDir, characterID, language)
}

func (a *DesktopApp) EquipBuild(profileID string, characterID int, moduleIDs []string, cartridgeID string, stats map[string]float64, language string) (appservice.BuildWorkspace, error) {
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	if _, err := appservice.SaveLocalBuild(a.stateDir, profileID, characterID, moduleIDs, cartridgeID, stats); err != nil {
		return appservice.BuildWorkspace{}, err
	}
	return appservice.LoadBuildWorkspace(a.stateDir, a.service.DataDir, language)
}

func (a *DesktopApp) EquipBuildResult(result appservice.OptimizationResult, language string) (appservice.BuildWorkspace, error) {
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	if _, err := appservice.SaveOptimizationResult(a.stateDir, result); err != nil {
		return appservice.BuildWorkspace{}, err
	}
	return appservice.LoadBuildWorkspace(a.stateDir, a.service.DataDir, language)
}

func (a *DesktopApp) SavedBuildResult(characterID int) (*appservice.OptimizationResult, error) {
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	return appservice.SavedOptimizationResult(a.stateDir, characterID)
}

func (a *DesktopApp) SetCharacterPriority(priority []int, language string) (appservice.BuildWorkspace, error) {
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	if _, err := appservice.SaveCharacterPriority(a.stateDir, priority); err != nil {
		return appservice.BuildWorkspace{}, err
	}
	return appservice.LoadBuildWorkspace(a.stateDir, a.service.DataDir, language)
}

func (a *DesktopApp) Target(profileID string) (target.BuildTarget, error) {
	return a.service.Target(profileID)
}

func (a *DesktopApp) AccountImportStatus() (appservice.AccountImportSummary, error) {
	return appservice.LoadAccountImportSummary(a.stateDir)
}

func (a *DesktopApp) CheckForUpdate() (updatecheck.Result, error) {
	return updatecheck.Check(a.ctx, updatecheck.HTTPClient(), buildinfo.Version)
}

func (a *DesktopApp) OpenDownloadURL(url string) {
	if strings.HasPrefix(url, "https://github.com/Apen/nte-optimizer/") {
		runtime.BrowserOpenURL(a.ctx, url)
	}
}

// ScanAndImportAccount runs only the packet-capture helper as administrator.
// Once its validated JSON export is complete, the normal desktop process reads
// and imports it without retaining elevated privileges.
func (a *DesktopApp) ScanAndImportAccount(language string, seconds int) (appservice.AccountImportSummary, error) {
	if _, err := ntelocale.Load(a.service.DataDir, language); err != nil {
		return appservice.AccountImportSummary{}, err
	}
	scanContext, stop := context.WithCancel(a.ctx)
	a.scanMu.Lock()
	if a.scanStop != nil {
		a.scanStop()
	}
	a.scanID++
	scanID := a.scanID
	a.scanStop = stop
	a.scanMu.Unlock()
	defer func() {
		a.scanMu.Lock()
		if a.scanID == scanID {
			a.scanStop = nil
		}
		a.scanMu.Unlock()
		stop()
	}()
	if err := scannerlauncher.RunContext(scanContext, a.projectDir, a.stateDir, seconds); err != nil {
		return appservice.AccountImportSummary{}, err
	}
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	return appservice.ImportDecodedWithSummary(scannerlauncher.OutputDir(a.stateDir), a.stateDir)
}

func (a *DesktopApp) StopScan() bool {
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	if a.scanStop == nil {
		return false
	}
	a.scanStop()
	return true
}

func (a *DesktopApp) OptimizeFlexibleSelection(profileID string, ignorePriority bool, language, mode string, goals map[string]appservice.GoalTuning, pinnedModuleIDs, excludedModuleIDs []string, weights *appservice.WeightOverrides) (appservice.OptimizationResult, error) {
	a.beginOptimizationLog(profileID, mode, goals, pinnedModuleIDs, excludedModuleIDs, weights)
	service, err := a.optimizerService(profileID, ignorePriority)
	if err != nil {
		a.finishOptimizationLog(appservice.OptimizationResult{}, err)
		return appservice.OptimizationResult{}, err
	}
	service.WeightOverrides = weights
	service.PinnedModuleIDs = stringSet(pinnedModuleIDs)
	service.ExcludedModuleIDs = stringSet(excludedModuleIDs)
	result, err := a.runFlexibleOptimization(service, profileID, true, language, mode, goals)
	a.finishOptimizationLog(result, err)
	return result, err
}

func (a *DesktopApp) optimizerService(profileID string, ignorePriority bool) (appservice.OptimizerService, error) {
	service := a.service
	if ignorePriority {
		return service, nil
	}
	profiles, err := service.Profiles()
	if err != nil {
		return appservice.OptimizerService{}, err
	}
	characterID := 0
	for _, profile := range profiles {
		if profile.ID == profileID {
			characterID = profile.CharacterID
			break
		}
	}
	if characterID == 0 {
		return appservice.OptimizerService{}, fmt.Errorf("unknown character profile %q", profileID)
	}
	a.buildMu.Lock()
	service.ReservedModuleIDs, service.ReservedCartridgeIDs, err = appservice.HigherPriorityReservations(a.stateDir, characterID)
	a.buildMu.Unlock()
	return service, err
}

func (a *DesktopApp) runFlexibleOptimization(service appservice.OptimizerService, profileID string, includeEquipped bool, language, mode string, goals map[string]appservice.GoalTuning) (appservice.OptimizationResult, error) {
	return a.runSearch(service, func(ctx context.Context, service appservice.OptimizerService) (appservice.OptimizationResult, error) {
		return service.OptimizeFlexibleProject(ctx, a.stateDir, profileID, includeEquipped, language, mode, goals)
	})
}

func (a *DesktopApp) runSearch(service appservice.OptimizerService, run func(context.Context, appservice.OptimizerService) (appservice.OptimizationResult, error)) (appservice.OptimizationResult, error) {
	searchContext, stop := context.WithCancel(a.ctx)
	a.searchMu.Lock()
	if a.searchStop != nil {
		a.searchStop()
	}
	a.searchID++
	searchID := a.searchID
	a.searchStop = stop
	a.searchMu.Unlock()
	defer func() {
		a.searchMu.Lock()
		if a.searchID == searchID {
			a.searchStop = nil
		}
		a.searchMu.Unlock()
		stop()
	}()
	service.Progress = func(progress optimizer.SearchProgress) {
		a.logMu.Lock()
		a.logProgress = progress
		a.logMu.Unlock()
		runtime.EventsEmit(a.ctx, "optimizer:progress", progress)
	}
	return run(searchContext, service)
}

func (a *DesktopApp) StopOptimization() bool {
	a.searchMu.Lock()
	defer a.searchMu.Unlock()
	if a.searchStop == nil {
		return false
	}
	a.searchStop()
	return true
}
