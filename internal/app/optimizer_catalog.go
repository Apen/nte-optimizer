package app

import (
	"fmt"
	"sync"

	"nte-optimizer/internal/datafiles"
	"nte-optimizer/internal/optimizer"
	"nte-optimizer/internal/scoring"
)

type optimizerCatalog struct {
	references scoring.References
	shapes     optimizer.ShapeCatalog
	grids      optimizer.GridCatalog
	sets       optimizer.SetCatalog
	config     optimizerDataConfig
}

type optimizerCatalogCache struct {
	once    sync.Once
	catalog optimizerCatalog
	err     error
}

func NewOptimizerService(dataDir string) OptimizerService {
	return OptimizerService{
		DataDir:    dataDir,
		catalogs:   &optimizerCatalogCache{},
		baseStats:  &characterBaseStatsCache{},
		blueprints: optimizer.NewBlueprintCache(),
	}
}

func (s OptimizerService) loadOptimizerCatalog() (optimizerCatalog, error) {
	if s.catalogs == nil {
		return readOptimizerCatalog(s.DataDir)
	}
	s.catalogs.once.Do(func() {
		s.catalogs.catalog, s.catalogs.err = readOptimizerCatalog(s.DataDir)
	})
	return s.catalogs.catalog, s.catalogs.err
}

func readOptimizerCatalog(dataDir string) (optimizerCatalog, error) {
	layout := datafiles.New(dataDir)
	var result optimizerCatalog
	if err := readJSON(layout.References(), &result.references); err != nil {
		return optimizerCatalog{}, err
	}
	var err error
	result.shapes, err = optimizer.LoadShapeCatalog(layout.Shapes())
	if err != nil {
		return optimizerCatalog{}, err
	}
	result.grids, err = optimizer.LoadGridCatalog(layout.Grids())
	if err != nil {
		return optimizerCatalog{}, err
	}
	result.sets, err = optimizer.LoadSetCatalog(layout.Sets())
	if err != nil {
		return optimizerCatalog{}, err
	}
	if err := readJSON(layout.Config(), &result.config); err != nil {
		return optimizerCatalog{}, err
	}
	if result.config.Optimize.TopPerGeometry <= 0 || result.config.Optimize.TopPerSet < 0 || result.config.Optimize.TimeoutSeconds <= 0 {
		return optimizerCatalog{}, fmt.Errorf("data/optimizer/config.json optimize values are invalid")
	}
	return result, nil
}
