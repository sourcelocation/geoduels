package moderation

import (
	"context"
	"errors"
	"strings"
	"time"

	"geoduels/internal/audit"
	pkgstaff "geoduels/pkg/staff"
)

// CheatingBanSummary reports the outcome of a cheating ban.
type CheatingBanSummary struct {
	UserID         string           `json:"userId"`
	Reason         string           `json:"reason,omitempty"`
	Refunds        EloRefundSummary `json:"refunds"`
	IPSignupBanned bool             `json:"ipSignupBanned"`
}

// EloRefundSummary counts rating refunds issued during enforcement.
type EloRefundSummary struct {
	RefundsIssued int `json:"refundsIssued"`
	TotalRefunded int `json:"totalRefunded"`
}

// CommunityPardonSummary reports a pardon preview or execution.
type CommunityPardonSummary struct {
	Eligible int       `json:"eligible"`
	Pardoned int       `json:"pardoned"`
	Cutoff   time.Time `json:"cutoff"`
}

// SignupIPBan is an active IP signup block.
type SignupIPBan struct {
	ID        int64     `json:"id"`
	IPAddress string    `json:"ipAddress"`
	Reason    string    `json:"reason,omitempty"`
	CreatedBy string    `json:"createdBy,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Service) BanCheater(ctx context.Context, actor pkgstaff.Actor, userID, reason string) (CheatingBanSummary, error) {
	if err := actor.RequireCap(pkgstaff.CapEnforceBans); err != nil {
		return CheatingBanSummary{}, err
	}
	var summary CheatingBanSummary
	err := s.store.WithinTx(ctx, func(store Store) error {
		userID, reason = strings.TrimSpace(userID), strings.TrimSpace(reason)
		if userID == "" {
			return errors.New("userID required")
		}
		if reason == "" {
			reason = "cheating"
		}
		registrationIP, err := store.ApplyCheatingBan(ctx, userID, reason, actor.ID)
		if err != nil {
			return err
		}
		refunds, err := s.refundVictims(ctx, store, userID, reason)
		if err != nil {
			return err
		}
		logID, err := store.RecordAudit(ctx, audit.Entry{ActorID: actor.ID, SubjectID: userID, Action: audit.ActionPermanentBan, Reason: reason,
			Metadata: map[string]any{"refundsIssued": refunds.RefundsIssued, "totalRefunded": refunds.TotalRefunded}})
		if err != nil {
			return err
		}
		if err := store.NotifyCheatingBan(ctx, userID, reason, logID); err != nil {
			return err
		}
		summary = CheatingBanSummary{UserID: userID, Reason: reason, Refunds: refunds}
		if registrationIP != "" {
			related, err := store.HasRelatedCheater(ctx, userID, registrationIP)
			if err != nil {
				return err
			}
			if ShouldBanRegistrationIP(registrationIP, related) {
				if err := store.AddIPBan(ctx, registrationIP, "Automatic signup ban: repeated cheating bans from registration IP", actor.ID); err != nil {
					return err
				}
				summary.IPSignupBanned = true
			}
		}
		return nil
	})
	if err != nil {
		return CheatingBanSummary{}, err
	}
	return summary, nil
}

func (s *Service) SetBan(ctx context.Context, actor pkgstaff.Actor, userID, reason string, banned bool) error {
	if err := actor.RequireAny(pkgstaff.CapReviewReports, pkgstaff.CapManageAccess); err != nil {
		return err
	}
	return s.store.WithinTx(ctx, func(store Store) error {
		return store.SetBan(ctx, userID, reason, actor.ID, banned)
	})
}

func (s *Service) SetMute(ctx context.Context, actor pkgstaff.Actor, userID, kind, reason string, until time.Time, muted bool) error {
	if err := actor.RequireCap(pkgstaff.CapReviewReports); err != nil {
		return err
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind != "chat" && kind != "report" {
		return errors.New("unsupported mute kind")
	}
	if muted && until.IsZero() {
		until = s.now().Add(7 * 24 * time.Hour)
	}
	return s.store.WithinTx(ctx, func(store Store) error {
		return store.SetMute(ctx, userID, kind, reason, actor.ID, until, muted)
	})
}

func (s *Service) ClearReporterMute(ctx context.Context, actor pkgstaff.Actor, userID string) error {
	if err := actor.RequireAny(pkgstaff.CapReviewReports, pkgstaff.CapManageAccess); err != nil {
		return err
	}
	return s.store.WithinTx(ctx, func(store Store) error {
		return store.ClearReporterMute(ctx, userID)
	})
}

func (s *Service) PreviewPardon(ctx context.Context, actor pkgstaff.Actor, olderThan time.Duration) (CommunityPardonSummary, error) {
	if err := actor.RequireCap(pkgstaff.CapManageAccess); err != nil {
		return CommunityPardonSummary{}, err
	}
	return s.store.PreviewPardon(ctx, pardonCutoff(s.now(), olderThan))
}

func (s *Service) Pardon(ctx context.Context, actor pkgstaff.Actor, olderThan time.Duration) (CommunityPardonSummary, error) {
	if err := actor.RequireCap(pkgstaff.CapManageAccess); err != nil {
		return CommunityPardonSummary{}, err
	}
	var summary CommunityPardonSummary
	err := s.store.WithinTx(ctx, func(store Store) error {
		var err error
		summary, err = store.Pardon(ctx, pardonCutoff(s.now(), olderThan), actor.ID)
		return err
	})
	if err != nil {
		return CommunityPardonSummary{}, err
	}
	return summary, nil
}

func (s *Service) AddIPBan(ctx context.Context, actor pkgstaff.Actor, ipAddress, reason string) error {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return err
	}
	return s.store.WithinTx(ctx, func(store Store) error {
		return store.AddIPBan(ctx, ipAddress, reason, actor.ID)
	})
}

func (s *Service) RemoveIPBan(ctx context.Context, actor pkgstaff.Actor, ipAddress string) error {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return err
	}
	return s.store.WithinTx(ctx, func(store Store) error {
		return store.RemoveIPBan(ctx, ipAddress)
	})
}

func (s *Service) ListIPBans(ctx context.Context, actor pkgstaff.Actor, limit int) ([]SignupIPBan, error) {
	if err := actor.RequireCap(pkgstaff.CapManageConfig); err != nil {
		return nil, err
	}
	return s.store.ListIPBans(ctx, limit)
}

// IsSignupIPBanned gates guest and OAuth signup on blocked IPs.
func (s *Service) IsSignupIPBanned(ctx context.Context, ipAddress string) (bool, error) {
	return s.store.IsSignupIPBanned(ctx, ipAddress)
}
