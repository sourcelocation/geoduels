package persistence

import (
	"context"
	"encoding/json"
	"errors"

	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrPreferenceRevisionConflict = errors.New("preference revision conflict")

type UserPreferences struct {
	SchemaVersion int             `json:"schemaVersion"`
	Preferences   json.RawMessage `json:"preferences"`
	Revision      int64           `json:"revision"`
}

func (s *DB) GetUserPreferences(ctx context.Context, userID string) (UserPreferences, error) {
	var result UserPreferences
	var id pgtype.UUID
	if err := id.Scan(userID); err != nil {
		return result, err
	}
	row, err := s.db.GetUserPreferences(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		result.SchemaVersion = 1
		result.Preferences = json.RawMessage(`{}`)
		return result, nil
	}
	if err == nil {
		result.SchemaVersion = int(row.SchemaVersion)
		result.Preferences = json.RawMessage(row.PreferencesJson)
		result.Revision = row.Revision
	}
	return result, err
}

func (s *DB) UpdateUserPreferences(ctx context.Context, userID string, schemaVersion int, preferences json.RawMessage, expectedRevision int64) (UserPreferences, error) {
	var result UserPreferences
	var id pgtype.UUID
	if err := id.Scan(userID); err != nil {
		return result, err
	}
	row, err := s.db.UpsertUserPreferences(ctx, db.UpsertUserPreferencesParams{UserID: id, SchemaVersion: int32(schemaVersion), PreferencesJson: preferences, ExpectedRevision: expectedRevision})
	if errors.Is(err, pgx.ErrNoRows) {
		return UserPreferences{}, ErrPreferenceRevisionConflict
	}
	if err == nil {
		result.SchemaVersion = int(row.SchemaVersion)
		result.Preferences = json.RawMessage(row.PreferencesJson)
		result.Revision = row.Revision
	}
	return result, err
}
