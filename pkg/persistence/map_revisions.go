package persistence

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/entityid"
)

// ReplaceMapLocations atomically replaces the current dataset for an official
// map. Match plans contain copied coordinates, so active matches do not depend
// on retaining old map datasets.
func (s *pgStore) ReplaceMapLocations(mapKey, displayName string, dataset []byte) (MapImportSummary, error) {
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
	if _, err := tx.Exec(ctx, `
		insert into maps(id,map_key,display_name,status,visibility,location_count,content_hash,created_at,updated_at)
		values($1,$2,$3,'processing','public',0,$4,now(),now())
		on conflict(map_key) do update set display_name=excluded.display_name,status='processing',updated_at=now()
	`, mapID, mapKey, displayName, digest[:]); err != nil {
		return MapImportSummary{}, err
	}
	var mapStorageID int32
	if err := tx.QueryRow(ctx, `select id::text,storage_id from maps where map_key=$1 for update`, mapKey).Scan(&mapID, &mapStorageID); err != nil {
		return MapImportSummary{}, err
	}
	if _, err := tx.Exec(ctx, `delete from locations where map_storage_id=$1`, mapStorageID); err != nil {
		return MapImportSummary{}, err
	}
	block := make([][]any, 0, len(rows))
	for _, row := range rows {
		block = append(block, []any{mapStorageID, row.LatE7, row.LngE7, row.Country, row.PanoID, row.HeadingCDeg, row.PitchCDeg, row.RandKey})
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"locations"},
		[]string{"map_storage_id", "lat_e7", "lng_e7", "country", "pano_id", "heading_cdeg", "pitch_cdeg", "rand_key_i"},
		pgx.CopyFromRows(block)); err != nil {
		return MapImportSummary{}, err
	}
	if _, err := tx.Exec(ctx, `delete from map_country_stats where map_id=$1`, mapID); err != nil {
		return MapImportSummary{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into map_country_stats(map_id,country,location_count)
		select $1,coalesce(nullif(country,''),'Unknown'),count(*)::int
		from locations where map_storage_id=$2
		group by coalesce(nullif(country,''),'Unknown')
	`, mapID, mapStorageID); err != nil {
		return MapImportSummary{}, err
	}
	if _, err := tx.Exec(ctx, `
		update maps
		set status='ready',location_count=$2,content_hash=$3,rejected_location_count=0,updated_at=now()
		where id=$1
	`, mapID, len(rows), digest[:]); err != nil {
		return MapImportSummary{}, err
	}
	if _, err := tx.Exec(ctx, `insert into map_aliases(alias,map_id) values($1,$2) on conflict(alias) do update set map_id=excluded.map_id`, mapKey, mapID); err != nil {
		return MapImportSummary{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MapImportSummary{}, err
	}
	return MapImportSummary{MapID: mapID, MapKey: mapKey, LocationCount: len(rows), DisplayName: displayName}, nil
}
