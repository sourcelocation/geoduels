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
		RankedMovingMapID:       contracts.MapKeyMoving,
		RankedNMPZMapID:         contracts.MapKeyNMPZ,
		SingleplayerMovingMapID: contracts.MapKeyMoving,
		SingleplayerNMPZMapID:   contracts.MapKeyNMPZ,
	}
}

func normalizeGameplayMapSettings(settings contracts.GameplayMapSettings) contracts.GameplayMapSettings {
	defaults := defaultGameplayMapSettings()
	if strings.TrimSpace(settings.RankedMovingMapID) == "" {
		settings.RankedMovingMapID = defaults.RankedMovingMapID
	}
	if strings.TrimSpace(settings.RankedNMPZMapID) == "" {
		settings.RankedNMPZMapID = defaults.RankedNMPZMapID
	}
	if strings.TrimSpace(settings.SingleplayerMovingMapID) == "" {
		settings.SingleplayerMovingMapID = defaults.SingleplayerMovingMapID
	}
	if strings.TrimSpace(settings.SingleplayerNMPZMapID) == "" {
		settings.SingleplayerNMPZMapID = defaults.SingleplayerNMPZMapID
	}
	return settings
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
	var settings contracts.GameplayMapSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return defaultGameplayMapSettings(), nil
	}
	return normalizeGameplayMapSettings(settings), nil
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
	required := minMapLocations
	if strings.HasPrefix(field, "ranked") {
		required = plannedRoundCount
	}
	if err := s.ensureReadyMapTx(ctx, tx, mapID, required); err != nil {
		return contracts.CustomMap{}, err
	}
	settings, err := s.gameplayMapSettings(ctx, tx)
	if err != nil {
		return contracts.CustomMap{}, err
	}
	switch field {
	case "rankedMovingMapId":
		settings.RankedMovingMapID = mapID
	case "rankedNmpzMapId":
		settings.RankedNMPZMapID = mapID
	case "singleplayerMovingMapId":
		settings.SingleplayerMovingMapID = mapID
	case "singleplayerNmpzMapId":
		settings.SingleplayerNMPZMapID = mapID
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
	case "ranked_moving", "ranked-moving", "rankedmovingmapid":
		return "rankedMovingMapId", nil
	case "ranked_nmpz", "ranked-nmpz", "rankednmpzmapid":
		return "rankedNmpzMapId", nil
	case "singleplayer_moving", "singleplayer-moving", "singleplayermovingmapid":
		return "singleplayerMovingMapId", nil
	case "singleplayer_nmpz", "singleplayer-nmpz", "singleplayernmpzmapid":
		return "singleplayerNmpzMapId", nil
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
	ruleset = contracts.NormalizeRuleset(ruleset)
	if mode == contracts.ModeSingleplayer {
		if ruleset == contracts.RulesetNMPZ {
			return s.ResolveGameplayMapID(mode, ruleset, settings.SingleplayerNMPZMapID)
		}
		return s.ResolveGameplayMapID(mode, ruleset, settings.SingleplayerMovingMapID)
	}
	if mode == contracts.ModeDuel {
		if ruleset == contracts.RulesetNMPZ {
			return s.ResolveGameplayMapID(mode, ruleset, settings.RankedNMPZMapID)
		}
		return s.ResolveGameplayMapID(mode, ruleset, settings.RankedMovingMapID)
	}
	return requestedMapID, nil
}
