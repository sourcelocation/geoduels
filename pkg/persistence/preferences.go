package persistence

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
)

var ErrPreferenceRevisionConflict = errors.New("preference revision conflict")

type UserPreferences struct {
	SchemaVersion int             `json:"schemaVersion"`
	Preferences   json.RawMessage `json:"preferences"`
	Revision      int64           `json:"revision"`
}

func (s *pgStore) GetUserPreferences(userID string) (UserPreferences, error) {
	var result UserPreferences
	err := s.pool.QueryRow(context.Background(), `
		select schema_version, preferences_json, revision
		from user_preferences
		where user_id=$1
	`, userID).Scan(&result.SchemaVersion, &result.Preferences, &result.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		result.SchemaVersion = 1
		result.Preferences = json.RawMessage(`{}`)
		return result, nil
	}
	return result, err
}

func (s *pgStore) UpdateUserPreferences(userID string, schemaVersion int, preferences json.RawMessage, expectedRevision int64) (UserPreferences, error) {
	var result UserPreferences
	err := s.pool.QueryRow(context.Background(), `
		insert into user_preferences(user_id, schema_version, preferences_json, revision, updated_at)
		values($1, $2, $3::jsonb, 1, now())
		on conflict(user_id) do update
		set schema_version=excluded.schema_version,
		    preferences_json=excluded.preferences_json,
		    revision=user_preferences.revision+1,
		    updated_at=now()
		where user_preferences.revision=$4
		returning schema_version, preferences_json, revision
	`, userID, schemaVersion, string(preferences), expectedRevision).Scan(
		&result.SchemaVersion,
		&result.Preferences,
		&result.Revision,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return UserPreferences{}, ErrPreferenceRevisionConflict
	}
	return result, err
}
