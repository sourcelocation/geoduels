package authsession

import (
	"context"
	"time"
)

// Record is a refresh session: the hash of its current refresh token and the device it was issued to.
type Record struct {
	ID               string
	UserID           string
	RefreshTokenHash string
	ExpiresAt        time.Time
	CreatedAt        time.Time
	LastUsedAt       time.Time
	RevokedAt        *time.Time
	UserAgent        string
	IPAddress        string
}

// Params is the device a new session is issued to.
type Params struct {
	UserAgent string
	IPAddress string
}

type Store interface {
	CreateAuthSession(string, string, time.Time, Params) (Record, error)
	GetAuthSessionByRefreshToken(string) (Record, bool, error)
	RotateAuthSession(string, string, string, time.Time, time.Time) (Record, bool, error)
	RevokeAuthSession(string) error
	RevokeAuthSessionsForUser(string) error
}
type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }
func (s *Service) Create(ctx context.Context, a, b string, t time.Time, p Params) (Record, error) {
	_ = ctx
	return s.store.CreateAuthSession(a, b, t, p)
}
func (s *Service) Get(ctx context.Context, h string) (Record, bool, error) {
	_ = ctx
	return s.store.GetAuthSessionByRefreshToken(h)
}
func (s *Service) Rotate(ctx context.Context, a, b, c string, t, u time.Time) (Record, bool, error) {
	_ = ctx
	return s.store.RotateAuthSession(a, b, c, t, u)
}
func (s *Service) Revoke(ctx context.Context, id string) error {
	_ = ctx
	return s.store.RevokeAuthSession(id)
}
func (s *Service) RevokeAll(ctx context.Context, id string) error {
	_ = ctx
	return s.store.RevokeAuthSessionsForUser(id)
}
