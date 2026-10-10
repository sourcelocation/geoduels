package maps

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/internal/storekit"
	"geoduels/pkg/contracts"
	"geoduels/pkg/matchkind"
	db "geoduels/pkg/persistence/sqlc/db"
)

// MatchPlanRequest asks for a new match's map and round plan.
type MatchPlanRequest struct {
	MatchID string
	Spec    matchkind.Spec
	Config  contracts.MatchConfig
	// MapAccessUserID is whose private maps the match may play.
	MapAccessUserID string
	Players         []string
}

// MatchPlan is the map a match plays; its rounds are in match_round_plans.
type MatchPlan struct {
	Config contracts.MatchConfig
	MapID  string
}

// PlanMatchTx picks a new match's map and writes its round plan in the caller's transaction, so the
// plan exists exactly when the match does. Ranked kinds play the ruleset's rotation map; the others
// play the map in their config, or the rotation map when none was chosen.
func (s *PGStore) PlanMatchTx(ctx context.Context, tx pgx.Tx, req MatchPlanRequest) (MatchPlan, error) {
	if strings.TrimSpace(req.MatchID) == "" {
		return MatchPlan{}, errors.New("match required")
	}
	q := db.New(tx)
	cfg := contracts.NormalizeMatchConfig(req.Config)
	mapID := strings.TrimSpace(cfg.MapID)
	if req.Spec.Maps == matchkind.MapsRotation || mapID == "" {
		resolved, err := s.ResolveGameplayMapID(req.Spec.Mode, cfg.Ruleset, "")
		if err != nil {
			return MatchPlan{}, err
		}
		mapID = resolved
	}
	canonicalMapID, _, err := resolveMapIdentity(ctx, tx, mapID)
	if err != nil {
		return MatchPlan{}, fmt.Errorf("selected map unavailable: %w", err)
	}
	mapID = canonicalMapID
	mapUUID, err := storekit.ProfileUUID(mapID)
	if err != nil {
		return MatchPlan{}, fmt.Errorf("selected map unavailable: %w", err)
	}
	selectedMap, err := q.SelectedMap(ctx, mapUUID)
	if err != nil {
		return MatchPlan{}, fmt.Errorf("selected map unavailable: %w", err)
	}
	if string(selectedMap.Status) != "ready" {
		return MatchPlan{}, errors.New("selected map is not ready")
	}
	if !selectedMapAccessible(storekit.UUIDVal(selectedMap.OwnerUserID), req.MapAccessUserID, string(selectedMap.Visibility)) {
		return MatchPlan{}, errors.New("selected map is not accessible")
	}
	requiredRounds := plannedRoundCount
	if req.Spec.Mode == contracts.ModeFreeForAll || req.Spec.Mode == contracts.ModeSingleplayer {
		requiredRounds = minMapLocations
	}
	if int(selectedMap.LocationCount) < requiredRounds {
		return MatchPlan{}, errors.New("selected map has too few locations")
	}
	selected, err := selectPlanRows(ctx, tx, mapID, deterministicPivot(req.MatchID, mapID), requiredRounds)
	if err != nil {
		return MatchPlan{}, err
	}
	if len(selected) < requiredRounds {
		return MatchPlan{}, errors.New("selected map has too few locations")
	}
	for i, row := range selected {
		if err := q.InsertMatchRoundPlan(ctx, db.InsertMatchRoundPlanParams{MatchID: mustMapUUID(req.MatchID), RoundIndex: int32(i), MapID: mapUUID, Lat: row.Lat, Lng: row.Lng, Country: pgtype.Text{String: row.Country, Valid: true}, PanoID: pgtype.Text{String: valueOrEmpty(row.PanoID), Valid: row.PanoID != nil}, Heading: pgtype.Float8{Float64: valueOrZero(row.Heading), Valid: row.Heading != nil}, Pitch: pgtype.Float8{Float64: valueOrZero(row.Pitch), Valid: row.Pitch != nil}}); err != nil {
			return MatchPlan{}, err
		}
	}
	cfg.MapID = mapID
	cfg.MapName = selectedMap.DisplayName
	cfg.MapKey = ""
	if err := incrementMapPlayStats(ctx, tx, mapID, req.Players); err != nil {
		return MatchPlan{}, err
	}
	return MatchPlan{Config: cfg, MapID: mapID}, nil
}

// MatchRounds returns a match's planned round locations, in order.
func (s *PGStore) MatchRounds(ctx context.Context, matchID string) ([]contracts.LocationPoint, error) {
	id, err := storekit.ProfileUUID(matchID)
	if err != nil {
		return nil, err
	}
	rows, err := db.New(s.pool).ListMatchRoundPlans(ctx, id)
	if err != nil {
		return nil, err
	}
	rounds := make([]contracts.LocationPoint, 0, len(rows))
	for _, row := range rows {
		point := contracts.LocationPoint{Lat: row.Lat, Lng: row.Lng, Country: row.Country}
		if row.PanoID.Valid {
			v := row.PanoID.String
			point.PanoID = &v
		}
		if row.Heading.Valid {
			v := row.Heading.Float64
			point.Heading = &v
		}
		if row.Pitch.Valid {
			v := row.Pitch.Float64
			point.Pitch = &v
		}
		rounds = append(rounds, point)
	}
	if len(rounds) == 0 {
		return nil, errors.New("match has no round plan")
	}
	return rounds, nil
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
	uuid, err := storekit.ProfileUUID(mapID)
	if err != nil {
		return nil, err
	}
	query := func(op string, n int) ([]plannedLocation, error) {
		var rows []db.SelectPlanRowsGERow
		var err error
		params := db.SelectPlanRowsGEParams{MapID: uuid, MinRandKey: pivot, RowLimit: int32(n)}
		if op == ">=" {
			rows, err = q.SelectPlanRowsGE(ctx, params)
		} else {
			var r []db.SelectPlanRowsLTRow
			r, err = q.SelectPlanRowsLT(ctx, db.SelectPlanRowsLTParams{MapID: uuid, MaxRandKey: params.MinRandKey, RowLimit: params.RowLimit})
			for _, x := range r {
				rows = append(rows, db.SelectPlanRowsGERow{LatE7: x.LatE7, LngE7: x.LngE7, Country: x.Country, PanoID: x.PanoID, HeadingCdeg: x.HeadingCdeg, PitchCdeg: x.PitchCdeg})
			}
		}
		if err != nil {
			return nil, err
		}
		out := []plannedLocation{}
		for _, row := range rows {
			var p plannedLocation
			p.Lat, p.Lng, p.Country = float64(row.LatE7)/1e7, float64(row.LngE7)/1e7, row.Country
			if row.PanoID.Valid {
				panoID := row.PanoID.String
				p.PanoID = &panoID
			}
			heading, pitch := float64(row.HeadingCdeg.Int16)/100, float64(row.PitchCdeg.Int16)/100
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
	id, err := storekit.ProfileUUID(mapID)
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
