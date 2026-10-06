// Package moderation handles player conduct: reports and integrity signals,
// staff review, enforcement (bans, mutes, pardons, signup IP blocks, rating
// refunds) and warnings. Staff operations check their own capabilities.
package moderation

import (
	"errors"
	"time"
)

// ErrNotFound marks a missing subject or record.
var ErrNotFound = errors.New("subject not found")

// Service applies moderation policies and coordinates persistence. The risk
// detector is an external dependency, independent of database transactions.
type Service struct {
	store Store
	risk  RiskEngine
	clock func() time.Time
}

func NewService(store Store, risk RiskEngine) *Service {
	return &Service{store: store, risk: risk, clock: time.Now}
}

func (s *Service) now() time.Time { return s.clock().UTC() }
