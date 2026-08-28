package main

import (
	"context"
	"encoding/json"
	"errors"

	"geoduels/pkg/persistence"
	preferencesdomain "geoduels/pkg/preferences"
)

type preferenceStoreAdapter struct {
	store persistence.PreferenceRepository
}

func (a preferenceStoreAdapter) GetUserPreferences(ctx context.Context, userID string) (preferencesdomain.UserPreferences, error) {
	value, err := a.store.GetUserPreferences(ctx, userID)
	return preferencesdomain.UserPreferences{SchemaVersion: value.SchemaVersion, Preferences: value.Preferences, Revision: value.Revision}, err
}

func (a preferenceStoreAdapter) UpdateUserPreferences(ctx context.Context, userID string, version int, value json.RawMessage, revision int64) (preferencesdomain.UserPreferences, error) {
	result, err := a.store.UpdateUserPreferences(ctx, userID, version, value, revision)
	if errors.Is(err, persistence.ErrPreferenceRevisionConflict) {
		return preferencesdomain.UserPreferences{}, preferencesdomain.ErrRevisionConflict
	}
	return preferencesdomain.UserPreferences{SchemaVersion: result.SchemaVersion, Preferences: result.Preferences, Revision: result.Revision}, err
}

func newPreferencesService(store persistence.PreferenceRepository) *preferencesdomain.Service {
	return preferencesdomain.NewService(preferenceStoreAdapter{store: store})
}
