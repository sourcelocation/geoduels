package matches

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/contracts"
)

// Store is the match lifecycle and history: starting matches, the node side of running them,
// finalizing them, and reading them back.
type Store interface {
	Start(ctx context.Context, p StartParams) error
	StartTx(ctx context.Context, tx pgx.Tx, p StartParams) error
	EndMatch(ctx context.Context, matchID, outcome string) (bool, error)
	PickUp(ctx context.Context, node Node, run func(PickedUp) error) (string, error)
	NodeOpenMatches(ctx context.Context, node Node) (map[string]bool, error)
	FinalizeMatch(snap contracts.MatchSnapshot) (contracts.MatchSnapshot, error)
	MatchSessionStatus(ctx context.Context, matchID string) (contracts.MatchSessionStatus, error)
	GetSession(ctx context.Context, matchID string) (Session, bool, error)
	ActiveMatchForUser(ctx context.Context, userID string) (ActiveMatch, bool, error)
	UsersInLiveMatches(ctx context.Context, userIDs []string) (map[string]bool, error)
	FinalMatchSnapshot(matchID string) (*contracts.MatchSnapshot, bool, error)
	ListPlayerMatchHistory(userID string, limit int) ([]MatchHistorySummary, error)
	ListPlayerMatchHistoryPage(userID string, limit int, beforeEndedAt time.Time, beforeMatchID string, rankedOnly bool) (MatchHistoryPage, error)
	PlayerParticipatedInMatch(userID, matchID string) (bool, error)
}

// TxStep runs inside another module's transaction.
type TxStep = func(ctx context.Context, tx pgx.Tx) error

var _ Store = (*PGStore)(nil)
