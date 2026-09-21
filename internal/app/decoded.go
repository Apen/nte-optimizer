package app

import (
	"time"

	"nte-optimizer/internal/decoded"
	"nte-optimizer/internal/nte"
)

type AccountImportSummary struct {
	HasImport         bool      `json:"has_import"`
	ImportedAt        time.Time `json:"imported_at,omitempty"`
	SourceGeneratedAt time.Time `json:"source_generated_at,omitempty"`
	Characters        int       `json:"characters"`
	Modules           int       `json:"modules"`
	Cartridges        int       `json:"cartridges"`
	Weapons           int       `json:"weapons"`
	Warnings          []string  `json:"warnings,omitempty"`
}

func ImportDecodedWithSummary(sourceDir, projectDir string) (AccountImportSummary, error) {
	result, err := decoded.ImportDirectory(sourceDir)
	if err != nil {
		return AccountImportSummary{}, err
	}
	if err := ensureWorkspace(projectDir); err != nil {
		return AccountImportSummary{}, err
	}
	if err := writeAccountData(projectDir, result.Inventory, result.State); err != nil {
		return AccountImportSummary{}, err
	}
	return accountImportSummary(result.Inventory, result.State, result.Warnings), nil
}

func LoadAccountImportSummary(projectDir string) (AccountImportSummary, error) {
	inventory, state, loaded, err := loadAccountData(projectDir)
	if err != nil || !loaded {
		return AccountImportSummary{}, err
	}
	return accountImportSummary(inventory, state, nil), nil
}

func accountImportSummary(inventory nte.Inventory, state decoded.State, warnings []string) AccountImportSummary {
	return AccountImportSummary{
		HasImport: true, ImportedAt: state.ImportedAt, SourceGeneratedAt: state.SourceGeneratedAt,
		Characters: len(state.Characters), Modules: len(inventory.Modules),
		Cartridges: len(inventory.Cartridges), Weapons: len(state.Weapons), Warnings: warnings,
	}
}
