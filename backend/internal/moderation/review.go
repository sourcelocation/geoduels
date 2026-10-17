package moderation

import (
	"context"
	"encoding/json"
	"time"

	"geoduels/internal/audit"
	pkgstaff "geoduels/pkg/staff"
)

// PlayerSummary is a player as staff review them. Email, IP and identities are filled only for
// viewers who manage access.
type PlayerSummary struct {
	UserID            string
	Email             string
	DisplayName       string
	AvatarURL         string
	MMR               int
	GamesPlayed       int
	Wins              int
	RankedGamesPlayed int
	TrackedMatches    int
	RankedMatches     int
	DuelMatches       int
	SingleplayerRuns  int
	Losses            int
	IsGuest           bool
	IsAdmin           bool
	IsModerator       bool
	IsBanned          bool
	BanReason         string
	BannedAt          time.Time
	BanExpiresAt      time.Time
	ChatMuteReason    string
	ChatMutedAt       time.Time
	ChatMutedUntil    time.Time
	ReportMuteReason  string
	ReportMutedAt     time.Time
	ReportMutedUntil  time.Time
	LastIPAddress     string
	Identities        []PlayerIdentity
}

// PlayerIdentity is a sign-in identity linked to a player.
type PlayerIdentity struct {
	Provider       string
	ProviderUserID string
	Email          string
	ProviderName   string
	LastSeenAt     time.Time
	DeletedAt      time.Time
}

// Signal is a moderation signal about a player: a report, or a detector's finding.
type Signal struct {
	ID               int64
	SubjectUserID    string
	SubjectName      string
	SignalType       string
	Source           string
	Severity         string
	EvidenceStrength string
	DetectorKey      string
	DetectorVersion  string
	ReasonCode       string
	Score            float64
	RecommendedQueue bool
	ReporterUserID   string
	ReporterName     string
	MatchID          string
	Payload          json.RawMessage
	OccurredAt       time.Time
	CreatedAt        time.Time
	ReviewedAt       time.Time
	ReviewedBy       string
	Outcome          string
}

// SubjectDetail is a player read shape already filtered for the viewer's role.
type SubjectDetail struct {
	Player PlayerSummary
}

// SubjectProfile is everything staff review about one player.
type SubjectProfile struct {
	Player  PlayerSummary
	Signals []Signal
	Log     []audit.LogEntry
}

func requireReviewer(actor pkgstaff.Actor) error {
	return actor.RequireAny(pkgstaff.CapReviewReports, pkgstaff.CapManageAccess)
}

func (s *Service) SearchSubjects(ctx context.Context, actor pkgstaff.Actor, query string, limit int) ([]PlayerSummary, error) {
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

func (s *Service) SubjectProfile(ctx context.Context, actor pkgstaff.Actor, userID string) (SubjectProfile, error) {
	if err := requireReviewer(actor); err != nil {
		return SubjectProfile{}, err
	}
	detail, err := s.store.GetSubject(ctx, actor, userID)
	if err != nil {
		return SubjectProfile{}, err
	}
	signals, err := s.store.ListSignals(ctx, userID, 100)
	if err != nil {
		return SubjectProfile{}, err
	}
	log, err := s.store.ListAudit(ctx, userID, 100)
	if err != nil {
		return SubjectProfile{}, err
	}
	return SubjectProfile{Player: detail.Player, Signals: signals, Log: log}, nil
}

func (s *Service) ListSignals(ctx context.Context, actor pkgstaff.Actor, limit int) ([]Signal, error) {
	if err := requireReviewer(actor); err != nil {
		return nil, err
	}
	return s.store.ListSignals(ctx, "", limit)
}

func (s *Service) ListAudit(ctx context.Context, actor pkgstaff.Actor, limit int) ([]audit.LogEntry, error) {
	if err := requireReviewer(actor); err != nil {
		return nil, err
	}
	return s.store.ListAudit(ctx, "", limit)
}
