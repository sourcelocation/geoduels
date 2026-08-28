package preferences

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakeStore struct {
	value UserPreferences
	err   error
}

func (f *fakeStore) GetUserPreferences(context.Context, string) (UserPreferences, error) {
	return f.value, f.err
}
func (f *fakeStore) UpdateUserPreferences(_ context.Context, _ string, version int, value json.RawMessage, revision int64) (UserPreferences, error) {
	if revision != f.value.Revision {
		return UserPreferences{}, ErrRevisionConflict
	}
	f.value = UserPreferences{SchemaVersion: version, Preferences: value, Revision: revision + 1}
	return f.value, nil
}

func TestUpdateRejectsUnsupportedVersion(t *testing.T) {
	store := &fakeStore{}
	_, err := NewService(store).Update(context.Background(), "u", 2, json.RawMessage(`{"version":2}`), 0)
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("err = %v, want unsupported version", err)
	}
}

func TestUpdatePreservesRevisionConflict(t *testing.T) {
	store := &fakeStore{value: UserPreferences{Revision: 2}}
	_, err := NewService(store).Update(context.Background(), "u", 1, json.RawMessage(`{"version":1}`), 1)
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("err = %v, want revision conflict", err)
	}
}
