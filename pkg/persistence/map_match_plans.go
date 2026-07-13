package persistence

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/pkg/contracts"
)

func (s *pgStore) PrepareMatchPlan(ctx context.Context, found *contracts.MatchFound) error {
	if found == nil || strings.TrimSpace(found.MatchID) == "" {
		return errors.New("match required")
	}
	if len(found.PlannedRounds) > 0 && found.ResolvedMap.MapID != "" {
		return nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `select round_index,lat,lng,coalesce(country,''),pano_id,heading,pitch,map_id::text from match_round_plans where match_id=$1 order by round_index`, found.MatchID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var p contracts.PlannedRound
		var mapID string
		if err := rows.Scan(&p.RoundIndex, &p.Location.Lat, &p.Location.Lng, &p.Location.Country, &p.Location.PanoID, &p.Location.Heading, &p.Location.Pitch, &mapID); err != nil {
			rows.Close()
			return err
		}
		found.PlannedRounds = append(found.PlannedRounds, p)
		found.ResolvedMap.MapID = mapID
	}
	rows.Close()
	if len(found.PlannedRounds) > 0 {
		_ = tx.QueryRow(ctx, `select display_name from maps where id=$1`, found.ResolvedMap.MapID).Scan(&found.ResolvedMap.DisplayName)
		found.Config.MapID = found.ResolvedMap.MapID
		found.Config.MapName = found.ResolvedMap.DisplayName
		applyPlayRegionConfig(ctx, tx, found.ResolvedMap.MapID, &found.Config)
		return tx.Commit(ctx)
	}
	cfg := contracts.NormalizeMatchConfig(found.Config)
	mapID := cfg.MapID
	if found.Mode == contracts.ModeDuel && !found.Unranked && strings.TrimSpace(found.SourcePartyID) == "" {
		resolved, err := s.ResolveGameplayMapID(found.Mode, cfg.Ruleset, "")
		if err != nil {
			return err
		}
		mapID = resolved
	}
	canonicalMapID, _, err := resolveMapIdentity(ctx, tx, mapID)
	if err != nil {
		return fmt.Errorf("selected map unavailable: %w", err)
	}
	mapID = canonicalMapID
	var owner, visibility, status, displayName string
	var count int
	err = tx.QueryRow(ctx, `select coalesce(owner_user_id::text,''),visibility,status,display_name,location_count from maps where id=$1 and archived_at is null for share`, mapID).Scan(&owner, &visibility, &status, &displayName, &count)
	if err != nil {
		return fmt.Errorf("selected map unavailable: %w", err)
	}
	if status != "ready" {
		return errors.New("selected map is not ready")
	}
	if !selectedMapAccessible(owner, found.MapAccessUserID, visibility) {
		return errors.New("selected map is not accessible")
	}
	requiredRounds := plannedRoundCount
	if found.Mode == contracts.ModeFreeForAll || found.Mode == contracts.ModeSingleplayer {
		requiredRounds = minMapLocations
	}
	if count < requiredRounds {
		return errors.New("selected map has too few locations")
	}
	pivot := deterministicPivot(found.MatchID, mapID)
	selected, err := selectPlanRows(ctx, tx, mapID, pivot, requiredRounds)
	if err != nil {
		return err
	}
	if len(selected) < requiredRounds {
		return errors.New("selected map has too few locations")
	}
	for i, row := range selected {
		if _, err := tx.Exec(ctx, `insert into match_round_plans(match_id,round_index,map_id,lat,lng,country,pano_id,heading,pitch) values($1,$2,$3,$4,$5,$6,$7,$8,$9) on conflict(match_id,round_index) do nothing`, found.MatchID, i, mapID, row.Lat, row.Lng, row.Country, row.PanoID, row.Heading, row.Pitch); err != nil {
			return err
		}
		found.PlannedRounds = append(found.PlannedRounds, contracts.PlannedRound{RoundIndex: i, Location: row.LocationPoint})
	}
	found.ResolvedMap = contracts.ResolvedMap{MapID: mapID, DisplayName: displayName}
	found.Config.MapID = mapID
	found.Config.MapName = displayName
	found.Config.MapKey = ""
	applyPlayRegionConfig(ctx, tx, mapID, &found.Config)
	if err := incrementMapPlayStats(ctx, tx, mapID, found.Players); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// applyPlayRegionConfig sets the auto-zoom flag and precomputed play-region
// bounds on the match config so the client can zoom the minimap when the map
// author opted in. Missing bounds simply leave the config untouched.
func applyPlayRegionConfig(ctx context.Context, tx pgx.Tx, mapID string, cfg *contracts.MatchConfig) {
	if cfg == nil || strings.TrimSpace(mapID) == "" {
		return
	}
	var autoZoom bool
	var minLat, maxLat, minLng, maxLng pgtype.Int4
	if err := tx.QueryRow(ctx, `select auto_zoom_play_region,bounds_min_lat_e7,bounds_max_lat_e7,bounds_min_lng_e7,bounds_max_lng_e7 from maps where id=$1`, mapID).
		Scan(&autoZoom, &minLat, &maxLat, &minLng, &maxLng); err != nil {
		return
	}
	if !autoZoom {
		return
	}
	if !(minLat.Valid && maxLat.Valid && minLng.Valid && maxLng.Valid) {
		return
	}
	cfg.AutoZoomPlayRegion = true
	cfg.PlayRegionBounds = &contracts.PlayRegionBounds{
		MinLat: float64(minLat.Int32) / 1e7,
		MaxLat: float64(maxLat.Int32) / 1e7,
		MinLng: float64(minLng.Int32) / 1e7,
		MaxLng: float64(maxLng.Int32) / 1e7,
	}
}

func selectedMapAccessible(ownerUserID, accessUserID, visibility string) bool {
	if ownerUserID == "" || ownerUserID == accessUserID {
		return true
	}
	switch strings.TrimSpace(strings.ToLower(visibility)) {
	case "public", "unlisted":
		return true
	default:
		return false
	}
}

type plannedLocation struct {
	contracts.LocationPoint
}

func selectPlanRows(ctx context.Context, tx pgx.Tx, mapID string, pivot int32, limit int) ([]plannedLocation, error) {
	query := func(op string, n int) ([]plannedLocation, error) {
		rows, err := tx.Query(ctx, `select lat_e7::float8/10000000.0,lng_e7::float8/10000000.0,coalesce(country,''),pano_id,heading_cdeg::float8/100.0,pitch_cdeg::float8/100.0 from locations where map_storage_id=(select storage_id from maps where id=$1) and rand_key_i `+op+` $2 order by rand_key_i asc limit $3`, mapID, pivot, n)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []plannedLocation{}
		for rows.Next() {
			var p plannedLocation
			if err := rows.Scan(&p.Lat, &p.Lng, &p.Country, &p.PanoID, &p.Heading, &p.Pitch); err != nil {
				return nil, err
			}
			out = append(out, p)
		}
		return out, rows.Err()
	}
	out, err := query(">=", limit)
	if err != nil {
		return nil, err
	}
	if len(out) < limit {
		rest, err := query("<", limit-len(out))
		if err != nil {
			return nil, err
		}
		out = append(out, rest...)
	}
	return out, nil
}
func deterministicPivot(matchID, mapID string) int32 {
	sum := sha256.Sum256([]byte(matchID + ":" + mapID))
	return int32(binary.BigEndian.Uint32(sum[:4]) >> 8)
}

func incrementMapPlayStats(ctx context.Context, tx pgx.Tx, mapID string, players []string) error {
	if _, err := tx.Exec(ctx, `update maps set play_count=play_count+1, updated_at=now() where id=$1`, mapID); err != nil {
		return err
	}
	for _, userID := range players {
		if strings.TrimSpace(userID) == "" {
			continue
		}
		if err := markMapDailyUser(ctx, tx, mapID, userID, "played"); err != nil {
			return err
		}
	}
	return refreshMapTrendingScore(ctx, tx, mapID)
}
