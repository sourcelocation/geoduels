package moderation

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"geoduels/internal/audit"
	"geoduels/internal/jobs"
	"geoduels/internal/storekit"
	"geoduels/pkg/contracts"
	db "geoduels/pkg/persistence/sqlc/db"
	pkgstaff "geoduels/pkg/staff"
)

const (
	modeDuel   = "duel"
	initialMMR = 1000
)

// Store persists moderation data. WithinTx supplies a store whose writes
// commit together, or roll back when the callback returns an error.
type Store interface {
	WithinTx(ctx context.Context, fn func(Store) error) error
	RecordAudit(ctx context.Context, entry audit.Entry) (int64, error)

	// Review
	SearchSubjects(ctx context.Context, viewer pkgstaff.Actor, query string, limit int) ([]contracts.AdminPlayerSummary, error)
	GetSubject(ctx context.Context, viewer pkgstaff.Actor, userID string) (SubjectDetail, error)
	ListSignals(ctx context.Context, subjectID string, limit int) ([]contracts.ModerationSignalSummary, error)
	ListAudit(ctx context.Context, subjectID string, limit int) ([]contracts.ModerationAuditLogEntry, error)

	// Reports
	ReportEligibility(ctx context.Context, matchID, reporterID, subjectID string) (ReportEligibility, error)
	SaveReport(ctx context.Context, report PlayerReport) (contracts.ModerationSignalCreated, error)

	// Enforcement
	ActiveSeasonID(ctx context.Context) (string, error)
	ApplyCheatingBan(ctx context.Context, userID, reason, actorID string) (string, error)
	HasRelatedCheater(ctx context.Context, userID, registrationIP string) (bool, error)
	NotifyCheatingBan(ctx context.Context, userID, reason string, logID int64) error
	RefundCandidates(ctx context.Context, cheaterID string) ([]RefundCandidate, error)
	LockRefundRating(ctx context.Context, userID, seasonID string) (RatingState, bool, error)
	SaveRefund(ctx context.Context, award RefundAward) (bool, error)
	SetBan(ctx context.Context, userID, reason, actorID string, banned bool) error
	SetMute(ctx context.Context, userID, kind, reason, actorID string, until time.Time, muted bool) error
	ClearReporterMute(ctx context.Context, userID string) error
	Pardon(ctx context.Context, cutoff time.Time, actorID string) (CommunityPardonSummary, error)
	PreviewPardon(ctx context.Context, cutoff time.Time) (CommunityPardonSummary, error)
	AddIPBan(ctx context.Context, ipAddress, reason, actorID string) error
	RemoveIPBan(ctx context.Context, ipAddress string) error
	ListIPBans(ctx context.Context, limit int) ([]SignupIPBan, error)
	IsSignupIPBanned(ctx context.Context, ipAddress string) (bool, error)

	// Integrity
	MatchPlayerIDs(ctx context.Context, matchID string) ([]string, error)
	RecentGuessEvents(ctx context.Context, userID string) ([]RiskGuessEvent, error)
	PlayerContext(ctx context.Context, userID, seasonID string) (rating int, rankedGames int, err error)
	RecordSignal(ctx context.Context, matchID string, signal RiskSignal, occurredAt time.Time) error

	// Warnings
	UserExists(ctx context.Context, userID string) (bool, error)
	EvidenceSentBy(ctx context.Context, messageID, userID string) (bool, error)
	ResetNickname(ctx context.Context, userID string) (previous, reset string, err error)
	InsertWarning(ctx context.Context, warning Warning) error
	ListWarnings(ctx context.Context, userID string) ([]Warning, error)
	AcknowledgeWarning(ctx context.Context, userID string, id int64) error
	WithdrawWarning(ctx context.Context, userID string, id int64, actorID string) error
	NotifyWarning(ctx context.Context, userID string, id int64, payload map[string]any) error
}

// PGStore persists moderation data using the pool, or the transaction bound
// by WithinTx.
type PGStore struct {
	pool *pgxpool.Pool
	tx   pgx.Tx
	jobs jobs.Enqueuer
}

func NewPGStore(pool *pgxpool.Pool, enqueuer jobs.Enqueuer) *PGStore {
	return &PGStore{pool: pool, jobs: enqueuer}
}

// q selects the transaction when present and the pool otherwise.
func (a *PGStore) q() *db.Queries {
	if a.tx != nil {
		return db.New(a.tx)
	}
	return db.New(a.pool)
}

// conn is the transaction when present and the pool otherwise.
func (a *PGStore) conn() db.DBTX {
	if a.tx != nil {
		return a.tx
	}
	return a.pool
}

func (a *PGStore) requireTx() (pgx.Tx, error) {
	if a.tx == nil {
		return nil, errors.New("moderation store: operation requires a transaction")
	}
	return a.tx, nil
}

// WithinTx runs fn against a copy bound to the current transaction.
func (a *PGStore) WithinTx(ctx context.Context, fn func(Store) error) error {
	return storekit.WithTx(ctx, a.pool, func(tx pgx.Tx) error {
		bound := *a
		bound.tx = tx
		return fn(&bound)
	})
}

func (a *PGStore) RecordAudit(ctx context.Context, entry audit.Entry) (int64, error) {
	return audit.Record(ctx, a.conn(), entry)
}

var _ Store = (*PGStore)(nil)
