package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"geoduels/pkg/contracts"
)

const gameplayMapSettingsKey = "gameplay_map_settings"

func defaultGameplayMapSettings() contracts.GameplayMapSettings {
	return contracts.GameplayMapSettings{
		MovingMapID: contracts.MapKeyMoving,
		NoMoveMapID: contracts.MapKeyNMPZ,
		NMPZMapID:   contracts.MapKeyNMPZ,
	}
}

type legacyGameplayMapSettings struct {
	MovingMapID             string `json:"movingMapId"`
	NoMoveMapID             string `json:"noMoveMapId"`
	NMPZMapID               string `json:"nmpzMapId"`
	RankedMovingMapID       string `json:"rankedMovingMapId"`
	RankedNMPZMapID         string `json:"rankedNmpzMapId"`
	SingleplayerMovingMapID string `json:"singleplayerMovingMapId"`
	SingleplayerNMPZMapID   string `json:"singleplayerNmpzMapId"`
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeGameplayMapSettings(settings contracts.GameplayMapSettings) contracts.GameplayMapSettings {
	defaults := defaultGameplayMapSettings()
	if strings.TrimSpace(settings.MovingMapID) == "" {
		settings.MovingMapID = defaults.MovingMapID
	}
	if strings.TrimSpace(settings.NoMoveMapID) == "" {
		settings.NoMoveMapID = defaults.NoMoveMapID
	}
	if strings.TrimSpace(settings.NMPZMapID) == "" {
		settings.NMPZMapID = defaults.NMPZMapID
	}
	return settings
}

func decodeGameplayMapSettings(raw string) contracts.GameplayMapSettings {
	var legacy legacyGameplayMapSettings
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
		return defaultGameplayMapSettings()
	}
	return normalizeGameplayMapSettings(contracts.GameplayMapSettings{
		MovingMapID: firstNonEmpty(legacy.MovingMapID, legacy.RankedMovingMapID, legacy.SingleplayerMovingMapID),
		// Ranked NMPZ became ranked No Move; keep that map for the no-move slot.
		NoMoveMapID: firstNonEmpty(legacy.NoMoveMapID, legacy.RankedNMPZMapID),
		NMPZMapID:   firstNonEmpty(legacy.NMPZMapID, legacy.SingleplayerNMPZMapID, legacy.RankedNMPZMapID),
	})
}

func (s *pgStore) GetGameplayMapSettings() (contracts.GameplayMapSettings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return s.gameplayMapSettings(ctx, s.pool)
}

func (s *pgStore) gameplayMapSettings(ctx context.Context, q seasonQuerier) (contracts.GameplayMapSettings, error) {
	var raw string
	err := q.QueryRow(ctx, `
		select value_json::text
		from site_settings
		where key = $1
	`, gameplayMapSettingsKey).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return defaultGameplayMapSettings(), nil
		}
		return contracts.GameplayMapSettings{}, err
	}
	return decodeGameplayMapSettings(raw), nil
}

func (s *pgStore) SetMapOfficial(adminUserID, mapID string, official bool) (contracts.CustomMap, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	mapID = strings.TrimSpace(mapID)
	if mapID == "" {
		return contracts.CustomMap{}, errors.New("map required")
	}
	canonicalID, _, err := resolveMapIdentity(ctx, s.pool, mapID)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	mapID = canonicalID
	var tag pgconn.CommandTag
	if official {
		if err := s.ensureReadyMap(ctx, mapID, minMapLocations); err != nil {
			return contracts.CustomMap{}, err
		}
		tag, err = s.pool.Exec(ctx, `
			update maps
			set official_at=coalesce(official_at, now()), official_by=$2, published_at=coalesce(published_at, now()), visibility='public', updated_at=now()
			where id=$1 and archived_at is null
		`, mapID, strings.TrimSpace(adminUserID))
	} else {
		tag, err = s.pool.Exec(ctx, `
			update maps
			set official_at=null, official_by=null, updated_at=now()
			where id=$1 and archived_at is null
		`, mapID)
	}
	if err != nil {
		return contracts.CustomMap{}, err
	}
	if tag.RowsAffected() == 0 {
		return contracts.CustomMap{}, pgx.ErrNoRows
	}
	details, _, err := s.GetMap(adminUserID, mapID)
	return details.Map, err
}

func (s *pgStore) SetGameplayMapRole(adminUserID, mapID, role string) (contracts.CustomMap, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	mapID = strings.TrimSpace(mapID)
	canonicalID, _, err := resolveMapIdentity(ctx, s.pool, mapID)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	mapID = canonicalID
	field, err := gameplayMapRoleField(role)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	defer tx.Rollback(ctx)
	if err := s.ensureReadyMapTx(ctx, tx, mapID, plannedRoundCount); err != nil {
		return contracts.CustomMap{}, err
	}
	settings, err := s.gameplayMapSettings(ctx, tx)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	switch field {
	case "movingMapId":
		settings.MovingMapID = mapID
	case "noMoveMapId":
		settings.NoMoveMapID = mapID
	case "nmpzMapId":
		settings.NMPZMapID = mapID
	}
	payload, err := json.Marshal(normalizeGameplayMapSettings(settings))
	if err != nil {
		return contracts.CustomMap{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into site_settings(key, value_json, updated_at)
		values($1, $2::jsonb, now())
		on conflict (key) do update set value_json=excluded.value_json, updated_at=now()
	`, gameplayMapSettingsKey, string(payload)); err != nil {
		return contracts.CustomMap{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return contracts.CustomMap{}, err
	}
	details, _, err := s.GetMap(adminUserID, mapID)
	return details.Map, err
}

func (s *pgStore) ensureReadyMap(ctx context.Context, mapID string, requiredLocations int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	return s.ensureReadyMapTx(ctx, tx, mapID, requiredLocations)
}

func (s *pgStore) ensureReadyMapTx(ctx context.Context, tx pgx.Tx, mapID string, requiredLocations int) error {
	var status string
	var count int
	if err := tx.QueryRow(ctx, `
		select status,location_count
		from maps
		where id=$1 and archived_at is null
		for share
	`, mapID).Scan(&status, &count); err != nil {
		return err
	}
	if status != "ready" {
		return errors.New("selected map is not ready")
	}
	if count < requiredLocations {
		return errors.New("selected map has too few locations")
	}
	return nil
}

func gameplayMapRoleField(role string) (string, error) {
	switch strings.TrimSpace(strings.ToLower(role)) {
	case "moving", "movingmapid",
		"ranked_moving", "ranked-moving", "rankedmovingmapid",
		"singleplayer_moving", "singleplayer-moving", "singleplayermovingmapid":
		return "movingMapId", nil
	case "no_move", "nomove", "no-move", "nomovemapid",
		"ranked_nmpz", "ranked-nmpz", "rankednmpzmapid",
		"ranked_no_move", "ranked-no-move", "rankednomovemapid":
		return "noMoveMapId", nil
	case "nmpz", "nmpzmapid",
		"singleplayer_nmpz", "singleplayer-nmpz", "singleplayernmpzmapid":
		return "nmpzMapId", nil
	default:
		return "", errors.New("unsupported map role")
	}
}

func (s *pgStore) ResolveGameplayMapID(mode contracts.MatchMode, ruleset contracts.GameRuleset, requestedMapID string) (string, error) {
	requestedMapID = strings.TrimSpace(requestedMapID)
	if requestedMapID != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		id, _, err := resolveMapIdentity(ctx, s.pool, requestedMapID)
		return id, err
	}
	settings, err := s.GetGameplayMapSettings()
	if err != nil {
		return "", err
	}
	switch contracts.NormalizeRuleset(ruleset) {
	case contracts.RulesetNoMove:
		return s.ResolveGameplayMapID(mode, ruleset, settings.NoMoveMapID)
	case contracts.RulesetNMPZ:
		return s.ResolveGameplayMapID(mode, ruleset, settings.NMPZMapID)
	default:
		return s.ResolveGameplayMapID(mode, ruleset, settings.MovingMapID)
	}
}
