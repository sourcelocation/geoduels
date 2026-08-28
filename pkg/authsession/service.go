package authsession

import (
	"context"
	"geoduels/pkg/contracts"
	"time"
)

type Store interface {
	CreateAuthSession(string, string, time.Time, contracts.AuthSessionParams) (contracts.RefreshTokenRecord, error)
	GetAuthSessionByRefreshToken(string) (contracts.RefreshTokenRecord, bool, error)
	RotateAuthSession(string, string, string, time.Time, time.Time) (contracts.RefreshTokenRecord, bool, error)
	RevokeAuthSession(string) error
	RevokeAuthSessionsForUser(string) error
}
type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }
func (s *Service) Create(ctx context.Context, a, b string, t time.Time, p contracts.AuthSessionParams) (contracts.RefreshTokenRecord, error) {
	_ = ctx
	return s.store.CreateAuthSession(a, b, t, p)
}
func (s *Service) Get(ctx context.Context, h string) (contracts.RefreshTokenRecord, bool, error) {
	_ = ctx
	return s.store.GetAuthSessionByRefreshToken(h)
}
func (s *Service) Rotate(ctx context.Context, a, b, c string, t, u time.Time) (contracts.RefreshTokenRecord, bool, error) {
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
