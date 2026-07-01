package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ResolveLegacyEntityID preserves links and administrative references created
// before entity keys were migrated to UUIDs.
func (s *pgStore) ResolveLegacyEntityID(entityType, legacyID string) (string, bool, error) {
	entityType = strings.TrimSpace(entityType)
	legacyID = strings.TrimSpace(legacyID)
	if entityType == "" || legacyID == "" {
		return "", false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var id string
	err := s.pool.QueryRow(ctx, `
		select entity_id::text
		from legacy_id_aliases
		where entity_type=$1 and legacy_id=$2
	`, entityType, legacyID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}
