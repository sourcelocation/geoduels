package preferences

import (
	"context"
	"encoding/json"
	"errors"
)

const SupportedSchemaVersion = 1

var ErrRevisionConflict = errors.New("preference revision conflict")
var ErrUnsupportedVersion = errors.New("unsupported preference version")

type UserPreferences struct {
	SchemaVersion int
	Preferences   json.RawMessage
	Revision      int64
}

// Store is the persistence capability required by the preferences use cases.
type Store interface {
	GetUserPreferences(context.Context, string) (UserPreferences, error)
	UpdateUserPreferences(context.Context, string, int, json.RawMessage, int64) (UserPreferences, error)
}

type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }

func (s *Service) Get(ctx context.Context, userID string) (UserPreferences, error) {
	return s.store.GetUserPreferences(ctx, userID)
}

func (s *Service) Update(ctx context.Context, userID string, version int, value json.RawMessage, revision int64) (UserPreferences, error) {
	if version != SupportedSchemaVersion {
		return UserPreferences{}, ErrUnsupportedVersion
	}
	result, err := s.store.UpdateUserPreferences(ctx, userID, version, value, revision)
	if errors.Is(err, ErrRevisionConflict) {
		return UserPreferences{}, ErrRevisionConflict
	}
	return result, err
}
