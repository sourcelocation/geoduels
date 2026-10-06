package moderation

import (
	"context"

	"geoduels/pkg/contracts"
	pkgstaff "geoduels/pkg/staff"
)

// SubjectDetail is a player read shape already filtered for the viewer's role.
type SubjectDetail struct {
	Player contracts.AdminPlayerSummary `json:"player"`
}

func requireReviewer(actor pkgstaff.Actor) error {
	return actor.RequireAny(pkgstaff.CapReviewReports, pkgstaff.CapManageAccess)
}

func (s *Service) SearchSubjects(ctx context.Context, actor pkgstaff.Actor, query string, limit int) ([]contracts.AdminPlayerSummary, error) {
	if err := requireReviewer(actor); err != nil {
		return nil, err
	}
	return s.store.SearchSubjects(ctx, actor, query, limit)
}

func (s *Service) GetSubject(ctx context.Context, actor pkgstaff.Actor, userID string) (SubjectDetail, error) {
	if err := requireReviewer(actor); err != nil {
		return SubjectDetail{}, err
	}
	return s.store.GetSubject(ctx, actor, userID)
}

func (s *Service) SubjectProfile(ctx context.Context, actor pkgstaff.Actor, userID string) (contracts.ModerationSubjectProfile, error) {
	if err := requireReviewer(actor); err != nil {
		return contracts.ModerationSubjectProfile{}, err
	}
	detail, err := s.store.GetSubject(ctx, actor, userID)
	if err != nil {
		return contracts.ModerationSubjectProfile{}, err
	}
	signals, err := s.store.ListSignals(ctx, userID, 100)
	if err != nil {
		return contracts.ModerationSubjectProfile{}, err
	}
	log, err := s.store.ListAudit(ctx, userID, 100)
	if err != nil {
		return contracts.ModerationSubjectProfile{}, err
	}
	return contracts.ModerationSubjectProfile{Player: detail.Player, Signals: signals, Log: log}, nil
}

func (s *Service) ListSignals(ctx context.Context, actor pkgstaff.Actor, limit int) ([]contracts.ModerationSignalSummary, error) {
	if err := requireReviewer(actor); err != nil {
		return nil, err
	}
	return s.store.ListSignals(ctx, "", limit)
}

func (s *Service) ListAudit(ctx context.Context, actor pkgstaff.Actor, limit int) ([]contracts.ModerationAuditLogEntry, error) {
	if err := requireReviewer(actor); err != nil {
		return nil, err
	}
	return s.store.ListAudit(ctx, "", limit)
}
