package maps

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/gameplay"
	"geoduels/pkg/matchkind"
	db "geoduels/pkg/persistence/sqlc/db"
)

// Gameplay map configuration: which map answers each mode/ruleset slot.
// The maps feature owns catalog CRUD; this stays with persistence because
// match coordination, parties, and match planning resolve maps at runtime.

const gameplayMapSettingsKey = "gameplay_map_settings"

func defaultGameplayMapSettings() GameplaySettings {
	return GameplaySettings{
		MovingMapID: MapKeyMoving,
		NoMoveMapID: MapKeyNMPZ,
		NMPZMapID:   MapKeyNMPZ,
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

func normalizeGameplayMapSettings(settings GameplaySettings) GameplaySettings {
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

func decodeGameplayMapSettings(raw string) GameplaySettings {
	var legacy legacyGameplayMapSettings
	if err := json.Unmarshal([]byte(raw), &legacy); err != nil {
		return defaultGameplayMapSettings()
	}
	return normalizeGameplayMapSettings(GameplaySettings{
		MovingMapID: firstNonEmpty(legacy.MovingMapID, legacy.RankedMovingMapID, legacy.SingleplayerMovingMapID),
		// Ranked NMPZ became ranked No Move; keep that map for the no-move slot.
		NoMoveMapID: firstNonEmpty(legacy.NoMoveMapID, legacy.RankedNMPZMapID),
		NMPZMapID:   firstNonEmpty(legacy.NMPZMapID, legacy.SingleplayerNMPZMapID, legacy.RankedNMPZMapID),
	})
}

func (s *PGStore) GetGameplayMapSettings() (GameplaySettings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	return s.gameplayMapSettings(ctx, s.pool)
}

func (s *PGStore) gameplayMapSettings(ctx context.Context, q db.DBTX) (GameplaySettings, error) {
	raw, err := db.New(q).GetGameplayMapSettings(ctx, gameplayMapSettingsKey)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return defaultGameplayMapSettings(), nil
		}
		return GameplaySettings{}, err
	}
	return decodeGameplayMapSettings(string(raw)), nil
}

func (s *PGStore) ResolveGameplayMapID(mode matchkind.Mode, ruleset gameplay.Ruleset, requestedMapID string) (string, error) {
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
	switch gameplay.NormalizeRuleset(ruleset) {
	case gameplay.RulesetNoMove:
		return s.ResolveGameplayMapID(mode, ruleset, settings.NoMoveMapID)
	case gameplay.RulesetNMPZ:
		return s.ResolveGameplayMapID(mode, ruleset, settings.NMPZMapID)
	default:
		return s.ResolveGameplayMapID(mode, ruleset, settings.MovingMapID)
	}
}
