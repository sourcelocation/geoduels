package maps

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/gameplay"
	"geoduels/pkg/matchkind"
)

type requiredMap struct {
	key         string
	displayName string
	mode        matchkind.Mode
	ruleset     gameplay.Ruleset
}

// requiredPlayableMaps lists the (mode, ruleset) pairs a development
// environment cannot play without; each is served by the bundled sample
// dataset.
var requiredPlayableMaps = []requiredMap{
	{key: MapKeyMoving, displayName: "Sample World", mode: matchkind.ModeDuel, ruleset: gameplay.RulesetMoving},
	{key: MapKeyNMPZ, displayName: "Sample Varied World", mode: matchkind.ModeDuel, ruleset: gameplay.RulesetNoMove},
	{key: MapKeyNMPZ, displayName: "Sample Varied World", mode: matchkind.ModeSingleplayer, ruleset: gameplay.RulesetNMPZ},
}

// EnsurePlayableMaps imports the sample dataset for each required playable
// map that has no configured gameplay map yet. Existing maps are never
// replaced. Only development nodes enable this (DEV_MAP_DATASET); production
// maps are provisioned through imports.
func (s *PGStore) EnsurePlayableMaps(datasetPath string) error {
	dataset, err := os.ReadFile(datasetPath)
	if err != nil {
		return fmt.Errorf("read sample map dataset: %w", err)
	}
	for _, required := range requiredPlayableMaps {
		created, err := ensurePlayableMap(s, s, required, dataset)
		if err != nil {
			return fmt.Errorf("ensure %s map: %w", required.key, err)
		}
		if created {
			log.Printf("created development map %s from %s", required.key, datasetPath)
		} else {
			log.Printf("development map %s already configured; skipped", required.key)
		}
	}
	return nil
}

// mapResolver is the configured-map lookup EnsurePlayableMaps depends on;
// the maps feature store owns dataset imports.
type mapResolver interface {
	ResolveGameplayMapID(mode matchkind.Mode, ruleset gameplay.Ruleset, requestedMapID string) (string, error)
}

type mapDatasetStore interface {
	ReplaceMapLocations(mapKey, displayName string, dataset []byte) (ImportSummary, error)
}

func ensurePlayableMap(resolver mapResolver, datasets mapDatasetStore, required requiredMap, dataset []byte) (bool, error) {
	if _, err := resolver.ResolveGameplayMapID(required.mode, required.ruleset, ""); err == nil {
		return false, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("check configured map: %w", err)
	}
	if _, err := datasets.ReplaceMapLocations(required.key, required.displayName, dataset); err != nil {
		return false, fmt.Errorf("import sample dataset: %w", err)
	}
	return true, nil
}
