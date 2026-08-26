package main

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/contracts"
	"geoduels/pkg/persistence"
)

type bootstrapStore struct {
	resolveErr error
	imports    []string
}

func (s *bootstrapStore) ResolveGameplayMapID(contracts.MatchMode, contracts.GameRuleset, string) (string, error) {
	return "map-id", s.resolveErr
}

func (s *bootstrapStore) ReplaceMapLocations(mapKey, _ string, _ []byte) (persistence.MapImportSummary, error) {
	s.imports = append(s.imports, mapKey)
	return persistence.MapImportSummary{}, nil
}

func TestEnsureDevelopmentMapSkipsConfiguredMap(t *testing.T) {
	store := &bootstrapStore{}
	created, err := ensureDevelopmentMap(store, requiredDevelopmentMaps[0], []byte("[]"))
	if err != nil || created || len(store.imports) != 0 {
		t.Fatalf("created=%v imports=%v err=%v", created, store.imports, err)
	}
}

func TestEnsureDevelopmentMapImportsMissingMap(t *testing.T) {
	store := &bootstrapStore{resolveErr: pgx.ErrNoRows}
	created, err := ensureDevelopmentMap(store, requiredDevelopmentMaps[0], []byte("[]"))
	if err != nil || !created || len(store.imports) != 1 || store.imports[0] != contracts.MapKeyMoving {
		t.Fatalf("created=%v imports=%v err=%v", created, store.imports, err)
	}
}

func TestEnsureDevelopmentMapDoesNotMaskDatabaseFailure(t *testing.T) {
	store := &bootstrapStore{resolveErr: errors.New("database offline")}
	created, err := ensureDevelopmentMap(store, requiredDevelopmentMaps[0], []byte("[]"))
	if err == nil || created || len(store.imports) != 0 {
		t.Fatalf("created=%v imports=%v err=%v", created, store.imports, err)
	}
}
