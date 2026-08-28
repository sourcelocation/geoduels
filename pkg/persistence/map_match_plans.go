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
	db "geoduels/pkg/persistence/sqlc/db"
)

func (s *DB) PrepareMatchPlan(ctx context.Context, found *contracts.MatchFound) error {
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
	q := db.New(tx)
	rows, err := q.ListMatchRoundPlans(ctx, mustMapUUID(found.MatchID))
	if err != nil {
		return err
	}
	for _, row := range rows {
		var p contracts.PlannedRound
		mapID := row.MapID
		p.RoundIndex = int(row.RoundIndex)
		p.Location.Lat, p.Location.Lng, p.Location.Country = row.Lat, row.Lng, row.Country
		if row.PanoID.Valid {
			v := row.PanoID.String
			p.Location.PanoID = &v
		}
		if row.Heading.Valid {
			v := row.Heading.Float64
			p.Location.Heading = &v
		}
		if row.Pitch.Valid {
			v := row.Pitch.Float64
			p.Location.Pitch = &v
		}
		found.PlannedRounds = append(found.PlannedRounds, p)
		found.ResolvedMap.MapID = mapID
	}
	if len(found.PlannedRounds) > 0 {
		id, err := profileUUID(found.ResolvedMap.MapID)
		if err == nil {
			if name, nameErr := q.MapDisplayName(ctx, id); nameErr == nil {
				found.ResolvedMap.DisplayName = name
			}
		}
		found.Config.MapID = found.ResolvedMap.MapID
		found.Config.MapName = found.ResolvedMap.DisplayName
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
	mapUUID, err := profileUUID(mapID)
	if err != nil {
		return fmt.Errorf("selected map unavailable: %w", err)
	}
	selectedMap, err := q.SelectedMap(ctx, mapUUID)
	if err == nil {
		owner, _ = selectedMap.Coalesce.(string)
		visibility, status, displayName, count = string(selectedMap.Visibility), string(selectedMap.Status), selectedMap.DisplayName, int(selectedMap.LocationCount)
	}
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
		if err := q.InsertMatchRoundPlan(ctx, db.InsertMatchRoundPlanParams{MatchID: mustMapUUID(found.MatchID), RoundIndex: int32(i), MapID: mapUUID, Lat: row.Lat, Lng: row.Lng, Country: pgtype.Text{String: row.Country, Valid: true}, PanoID: pgtype.Text{String: valueOrEmpty(row.PanoID), Valid: row.PanoID != nil}, Heading: pgtype.Float8{Float64: valueOrZero(row.Heading), Valid: row.Heading != nil}, Pitch: pgtype.Float8{Float64: valueOrZero(row.Pitch), Valid: row.Pitch != nil}}); err != nil {
			return err
		}
		found.PlannedRounds = append(found.PlannedRounds, contracts.PlannedRound{RoundIndex: i, Location: row.LocationPoint})
	}
	found.ResolvedMap = contracts.ResolvedMap{MapID: mapID, DisplayName: displayName}
	found.Config.MapID = mapID
	found.Config.MapName = displayName
	found.Config.MapKey = ""
	if err := incrementMapPlayStats(ctx, tx, mapID, found.Players); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
func valueOrZero(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func selectPlanRows(ctx context.Context, tx pgx.Tx, mapID string, pivot int32, limit int) ([]plannedLocation, error) {
	q := db.New(tx)
	uuid, err := profileUUID(mapID)
	if err != nil {
		return nil, err
	}
	query := func(op string, n int) ([]plannedLocation, error) {
		var rows []db.SelectPlanRowsGERow
		var err error
		params := db.SelectPlanRowsGEParams{ID: uuid, RandKeyI: pivot, Limit: int32(n)}
		if op == ">=" {
			rows, err = q.SelectPlanRowsGE(ctx, params)
		} else {
			var r []db.SelectPlanRowsLTRow
			r, err = q.SelectPlanRowsLT(ctx, db.SelectPlanRowsLTParams{ID: uuid, RandKeyI: params.RandKeyI, Limit: params.Limit})
			for _, x := range r {
				rows = append(rows, db.SelectPlanRowsGERow{Column1: x.Column1, Column2: x.Column2, Country: x.Country, PanoID: x.PanoID, Column5: x.Column5, Column6: x.Column6})
			}
		}
		if err != nil {
			return nil, err
		}
		out := []plannedLocation{}
		for _, row := range rows {
			var p plannedLocation
			p.Lat, p.Lng, p.Country = float64(row.Column1)/1e7, float64(row.Column2)/1e7, row.Country
			if row.PanoID.Valid {
				panoID := row.PanoID.String
				p.PanoID = &panoID
			}
			heading, pitch := float64(row.Column5)/100, float64(row.Column6)/100
			p.Heading, p.Pitch = &heading, &pitch
			out = append(out, p)
		}
		return out, nil
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
	id, err := profileUUID(mapID)
	if err != nil {
		return err
	}
	if err := db.New(tx).IncrementMapPlay(ctx, id); err != nil {
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
