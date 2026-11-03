package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	appservice "nte-optimizer/internal/app"
	"nte-optimizer/internal/apperrors"
	"nte-optimizer/internal/buildinfo"
	ntelocale "nte-optimizer/internal/locale"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scanlog"
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
	logID       uint64
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
			profiles[index].ArcForkID = settings.ArcForkID
			profiles[index].TeamAllies = settings.TeamAllies
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
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	value, err := appservice.LoadCharacterGameState(a.stateDir, a.service.DataDir, characterID, language)
	return value, apperrors.ForClient(err)
}

func (a *DesktopApp) AccountBuffs(language string) (appservice.AccountBuffPage, error) {
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	value, err := appservice.LoadAccountBuffPage(a.stateDir, a.service.DataDir, language)
	return value, apperrors.ForClient(err)
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
	value, err := a.scanAndImportAccount(language, seconds, false)
	return value, scanClientError(err)
}

// RecoverAndScanAccount explicitly stops all Pktmon collection before starting
// a new guided scan. The UI warns that this can stop another tool's capture.
func (a *DesktopApp) RecoverAndScanAccount(language string, seconds int) (appservice.AccountImportSummary, error) {
	value, err := a.scanAndImportAccount(language, seconds, true)
	return value, scanClientError(err)
}

func (a *DesktopApp) scanAndImportAccount(language string, seconds int, stopPktmonFirst bool) (appservice.AccountImportSummary, error) {
	if _, err := ntelocale.Load(a.service.DataDir, language); err != nil {
		return appservice.AccountImportSummary{}, err
	}
	a.scanMu.Lock()
	if a.scanStop != nil {
		a.scanMu.Unlock()
		return appservice.AccountImportSummary{}, scannerlauncher.ErrScanAlreadyRunning
	}
	a.scanMu.Unlock()
	releaseScanLock, err := scannerlauncher.AcquireScanLock(a.stateDir)
	if err != nil {
		if errors.Is(err, scannerlauncher.ErrScanAlreadyRunning) {
			return appservice.AccountImportSummary{}, scannerlauncher.ErrScanAlreadyRunning
		}
		return appservice.AccountImportSummary{}, err
	}
	defer releaseScanLock()

	scanContext, stop := context.WithCancel(a.ctx)
	a.scanMu.Lock()
	if a.scanStop != nil {
		a.scanMu.Unlock()
		stop()
		return appservice.AccountImportSummary{}, scannerlauncher.ErrScanAlreadyRunning
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

	logPath := scanlog.Path(a.stateDir)
	if err := scanlog.Start(logPath, seconds); err != nil {
		return appservice.AccountImportSummary{}, fmt.Errorf("start local scan diagnostic: %w", err)
	}
	if stopPktmonFirst {
		if err := scanlog.Append(logPath, "pktmon_recovery", "requested", nil); err != nil {
			return appservice.AccountImportSummary{}, fmt.Errorf("record Pktmon recovery request: %w", err)
		}
	}
	var scanErr error
	if stopPktmonFirst {
		scanErr = scannerlauncher.RunContextAfterPktmonRecovery(scanContext, a.projectDir, a.stateDir, seconds)
	} else {
		scanErr = scannerlauncher.RunContext(scanContext, a.projectDir, a.stateDir, seconds)
	}
	if scanErr != nil {
		events, readErr := scanlog.Read(logPath)
		if readErr != nil || !hasScanFailure(events) {
			_ = scanlog.AppendWithCode(logPath, "scanner", "failed", scannerlauncher.ErrorCode(scanErr), nil)
		}
		return appservice.AccountImportSummary{}, scanErr
	}
	_ = scanlog.Append(logPath, "import", "started", nil)
	a.buildMu.Lock()
	defer a.buildMu.Unlock()
	summary, err := appservice.ImportDecodedWithSummary(scannerlauncher.OutputDir(a.stateDir), a.stateDir, a.service.DataDir)
	if err != nil {
		_ = scanlog.AppendWithCode(logPath, "import", "failed", "import_failed", nil)
		return summary, err
	}
	_ = scanlog.Append(logPath, "import", "complete", map[string]int{"characters": summary.Characters, "modules": summary.Modules, "cartridges": summary.Cartridges, "arcs": summary.Weapons})
	return summary, nil
}

func hasScanFailure(events []scanlog.Event) bool {
	for _, event := range events {
		if event.Status == "failed" || event.Status == "incomplete" || event.Status == "already_running" {
			return true
		}
	}
	return false
}

func (a *DesktopApp) LastScanLog() ([]scanlog.Event, error) {
	return scanlog.Read(scanlog.Path(a.stateDir))
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

func (a *DesktopApp) OptimizeFlexibleSelection(profileID string, ignorePriority bool, language, mode string, goals map[string]appservice.GoalTuning, pinnedModuleIDs, excludedModuleIDs []string, weights *appservice.WeightOverrides, arcForkID string) (appservice.OptimizationResult, error) {
	logID := a.beginOptimizationLog(profileID, mode, goals, pinnedModuleIDs, excludedModuleIDs, weights)
	service, err := a.optimizerService(profileID, ignorePriority)
	if err != nil {
		a.finishOptimizationLog(logID, appservice.OptimizationResult{}, err)
		return appservice.OptimizationResult{}, apperrors.ForClient(err)
	}
	service.WeightOverrides = weights
	service.PinnedModuleIDs = stringSet(pinnedModuleIDs)
	service.ExcludedModuleIDs = stringSet(excludedModuleIDs)
	result, err := a.runFlexibleOptimization(service, profileID, true, language, mode, goals, arcForkID, logID)
	a.finishOptimizationLog(logID, result, err)
	return result, apperrors.ForClient(err)
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
	service.ReservedModuleIDs, service.ReservedCartridgeIDs, service.ReservedArcIDs, err = appservice.HigherPriorityReservationsWithArcs(a.stateDir, characterID)
	a.buildMu.Unlock()
	return service, err
}

func (a *DesktopApp) runFlexibleOptimization(service appservice.OptimizerService, profileID string, includeEquipped bool, language, mode string, goals map[string]appservice.GoalTuning, arcForkID string, logID uint64) (appservice.OptimizationResult, error) {
	return a.runSearch(service, logID, func(ctx context.Context, service appservice.OptimizerService) (appservice.OptimizationResult, error) {
		return service.OptimizeFlexibleProjectWithArc(ctx, a.stateDir, profileID, includeEquipped, language, mode, goals, arcForkID)
	})
}

func (a *DesktopApp) runSearch(service appservice.OptimizerService, logID uint64, run func(context.Context, appservice.OptimizerService) (appservice.OptimizationResult, error)) (appservice.OptimizationResult, error) {
	searchContext, stop := context.WithCancel(a.ctx)
	a.searchMu.Lock()
	a.logMu.Lock()
	stale := a.logID != logID
	a.logMu.Unlock()
	if stale {
		a.searchMu.Unlock()
		stop()
		return appservice.OptimizationResult{}, context.Canceled
	}
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
		if a.recordOptimizationProgress(logID, progress) {
			runtime.EventsEmit(a.ctx, "optimizer:progress", progress)
		}
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

func scanClientError(err error) error {
	if err == nil {
		return nil
	}
	code := "scan_failed"
	if errors.Is(err, scannerlauncher.ErrScanAlreadyRunning) {
		code = "scan_busy"
	}
	var diagnostic *scannerlauncher.DiagnosticError
	if errors.As(err, &diagnostic) {
		switch diagnostic.Code {
		case "state_not_writable", "output_not_writable":
			code = "scan_diagnostic_unavailable"
		}
	}
	return apperrors.ForClient(apperrors.New(code, err))
}

func (a *DesktopApp) PreviewTeam(profileID, language, arcForkID string, allies []appservice.TeamAlly) (appservice.TeamPreview, error) {
	service, err := a.optimizerService(profileID, false)
	if err != nil {
		return appservice.TeamPreview{}, apperrors.ForClient(err)
	}
	preview, err := service.PreviewTeam(a.stateDir, profileID, language, arcForkID, allies)
	return preview, apperrors.ForClient(err)
}
