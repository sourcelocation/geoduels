package maps

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	db "geoduels/pkg/persistence/sqlc/db"
)

// readGameplaySettingsTx reads the configured gameplay map settings inside the
// caller's transaction.
func readGameplaySettingsTx(ctx context.Context, q db.DBTX) (GameplaySettings, error) {
	raw, err := db.New(q).GetGameplayMapSettings(ctx, gameplayMapSettingsKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return defaultGameplayMapSettings(), nil
		}
		return GameplaySettings{}, err
	}
	var legacy struct {
		MovingMapID             string `json:"movingMapId"`
		NoMoveMapID             string `json:"noMoveMapId"`
		NMPZMapID               string `json:"nmpzMapId"`
		RankedMovingMapID       string `json:"rankedMovingMapId"`
		RankedNMPZMapID         string `json:"rankedNmpzMapId"`
		SingleplayerMovingMapID string `json:"singleplayerMovingMapId"`
		SingleplayerNMPZMapID   string `json:"singleplayerNmpzMapId"`
	}
	if json.Unmarshal(raw, &legacy) != nil {
		return defaultGameplayMapSettings(), nil
	}
	firstNonEmpty := func(values ...string) string {
		for _, value := range values {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
		return ""
	}
	return normalizeGameplayMapSettings(GameplaySettings{
		MovingMapID: firstNonEmpty(legacy.MovingMapID, legacy.RankedMovingMapID, legacy.SingleplayerMovingMapID),
		// Ranked NMPZ became ranked No Move; keep that map for the no-move slot.
		NoMoveMapID: firstNonEmpty(legacy.NoMoveMapID, legacy.RankedNMPZMapID),
		NMPZMapID:   firstNonEmpty(legacy.NMPZMapID, legacy.SingleplayerNMPZMapID, legacy.RankedNMPZMapID),
	}), nil
}

func (s *PGStore) SetMapOfficial(adminUserID, mapID string, official bool) (Map, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	mapID = strings.TrimSpace(mapID)
	if mapID == "" {
		return Map{}, errors.New("map required")
	}
	canonicalID, _, err := resolveMapIdentity(ctx, s.pool, mapID)
	if err != nil {
		return Map{}, err
	}
	mapID = canonicalID
	var affected int64
	mapUUID := pgtype.UUID{}
	if err := mapUUID.Scan(mapID); err != nil {
		return Map{}, err
	}
	adminUUID := pgtype.UUID{}
	if strings.TrimSpace(adminUserID) != "" {
		if err := adminUUID.Scan(strings.TrimSpace(adminUserID)); err != nil {
			return Map{}, err
		}
	}
	if official {
		if err := s.ensureReadyMap(ctx, mapID, minMapLocations); err != nil {
			return Map{}, err
		}
		affected, err = s.q().SetMapOfficial(ctx, db.SetMapOfficialParams{MapID: mapUUID, OfficialBy: adminUUID})
	} else {
		affected, err = s.q().ClearMapOfficial(ctx, mapUUID)
	}
	if err != nil {
		return Map{}, err
	}
	if affected == 0 {
		return Map{}, pgx.ErrNoRows
	}
	details, _, err := s.GetMap(adminUserID, mapID)
	return details.Map, err
}

func (s *PGStore) SetGameplayMapRole(adminUserID, mapID, role string) (Map, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	mapID = strings.TrimSpace(mapID)
	canonicalID, _, err := resolveMapIdentity(ctx, s.pool, mapID)
	if err != nil {
		return Map{}, err
	}
	mapID = canonicalID
	field, err := gameplayMapRoleField(role)
	if err != nil {
		return Map{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Map{}, err
	}
	defer tx.Rollback(ctx)
	if err := s.ensureReadyMapTx(ctx, tx, mapID, plannedRoundCount); err != nil {
		return Map{}, err
	}
	settings, err := readGameplaySettingsTx(ctx, tx)
	if err != nil {
		return Map{}, err
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
		return Map{}, err
	}
	if err := db.New(tx).SetGameplayMapSettings(ctx, db.SetGameplayMapSettingsParams{Key: gameplayMapSettingsKey, ValueJson: payload}); err != nil {
		return Map{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Map{}, err
	}
	details, _, err := s.GetMap(adminUserID, mapID)
	return details.Map, err
}

func (s *PGStore) ensureReadyMap(ctx context.Context, mapID string, requiredLocations int) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	return s.ensureReadyMapTx(ctx, tx, mapID, requiredLocations)
}

func (s *PGStore) ensureReadyMapTx(ctx context.Context, tx pgx.Tx, mapID string, requiredLocations int) error {
	id := pgtype.UUID{}
	if err := id.Scan(mapID); err != nil {
		return err
	}
	row, err := db.New(tx).GetReadyMapForShare(ctx, id)
	if err != nil {
		return err
	}
	if string(row.Status) != "ready" {
		return errors.New("selected map is not ready")
	}
	if int(row.LocationCount) < requiredLocations {
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
