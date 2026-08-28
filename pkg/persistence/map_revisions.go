package persistence

import (
	"context"
	"crypto/sha256"
	"errors"
	"github.com/jackc/pgx/v5/pgtype"
	"strings"
	"time"

	"geoduels/pkg/entityid"
	db "geoduels/pkg/persistence/sqlc/db"
)

// ReplaceMapLocations atomically replaces the current dataset for an official
// map. Match plans contain copied coordinates, so active matches do not depend
// on retaining old map datasets.
func (s *DB) ReplaceMapLocations(mapKey, displayName string, dataset []byte) (MapImportSummary, error) {
	mapKey = strings.TrimSpace(mapKey)
	if mapKey == "" {
		return MapImportSummary{}, errors.New("map key required")
	}
	rows, err := parseMapRows(dataset)
	if err != nil {
		return MapImportSummary{}, err
	}
	if len(rows) == 0 {
		return MapImportSummary{}, errors.New("no valid rows")
	}
	if strings.TrimSpace(displayName) == "" {
		displayName = mapKey
	}
	digest := sha256.Sum256(dataset)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return MapImportSummary{}, err
	}
	defer tx.Rollback(ctx)

	mapID := entityid.Derive("map", mapKey)
	var id pgtype.UUID
	if err := id.Scan(mapID); err != nil {
		return MapImportSummary{}, err
	}
	q := s.maprevisions
	if err := q.WithTx(tx).UpsertMap(ctx, db.UpsertMapParams{ID: id, DisplayName: displayName, ContentHash: digest[:]}); err != nil {
		return MapImportSummary{}, err
	}
	if err := q.WithTx(tx).UpsertMapAlias(ctx, db.UpsertMapAliasParams{Alias: mapKey, MapID: id}); err != nil {
		return MapImportSummary{}, err
	}
	storage, err := q.WithTx(tx).LockMapStorageID(ctx, id)
	if err != nil {
		return MapImportSummary{}, err
	}
	if err := q.WithTx(tx).DeleteMapLocations(ctx, storage); err != nil {
		return MapImportSummary{}, err
	}
	lat, lng, country, pano := make([]int32, len(rows)), make([]int32, len(rows)), make([]string, len(rows)), make([]string, len(rows))
	heading, pitch, rand := make([]int16, len(rows)), make([]int16, len(rows)), make([]int32, len(rows))
	for i, row := range rows {
		h, p := int16(0), int16(0)
		if row.HeadingCDeg != nil {
			h = *row.HeadingCDeg
		}
		if row.PitchCDeg != nil {
			p = *row.PitchCDeg
		}
		lat[i], lng[i], country[i], heading[i], pitch[i], rand[i] = row.LatE7, row.LngE7, row.Country, h, p, row.RandKey
		if row.PanoID != nil {
			pano[i] = *row.PanoID
		}
	}
	if err := q.WithTx(tx).InsertMapLocations(ctx, db.InsertMapLocationsParams{MapStorageID: storage, Column2: lat, Column3: lng, Column4: country, Column5: pano, Column6: heading, Column7: pitch, Column8: rand}); err != nil {
		return MapImportSummary{}, err
	}
	if err := q.WithTx(tx).DeleteMapCountryStats(ctx, id); err != nil {
		return MapImportSummary{}, err
	}
	if err := q.WithTx(tx).InsertMapCountryStats(ctx, db.InsertMapCountryStatsParams{MapID: id, MapStorageID: storage}); err != nil {
		return MapImportSummary{}, err
	}
	if err := q.WithTx(tx).FinalizeMap(ctx, db.FinalizeMapParams{ID: id, LocationCount: int32(len(rows)), ContentHash: digest[:]}); err != nil {
		return MapImportSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MapImportSummary{}, err
	}
	return MapImportSummary{MapID: mapID, MapKey: mapKey, LocationCount: len(rows), DisplayName: displayName}, nil
}
