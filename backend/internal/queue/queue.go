// Package queue holds the players looking for a ranked duel and decides who plays whom. A player
// queues for one or more variants of a ranked duel; each pairing takes both players out of every
// variant at once.
package queue

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"geoduels/internal/storekit"
	"geoduels/pkg/contracts"
	db "geoduels/pkg/persistence/sqlc/db"
)

// Variant is one ranked queue: a ruleset with street names shown or hidden.
type Variant = string

const (
	Moving       Variant = "moving"
	NoMove       Variant = "no_move"
	NMPZ         Variant = "nmpz"
	MovingHidden Variant = "moving_hidden"
	NoMoveHidden Variant = "no_move_hidden"
	NMPZHidden   Variant = "nmpz_hidden"
)

// Ranked lists the variants a player can queue for, in the order they are matched.
var Ranked = []Variant{Moving, NoMoveHidden}

// ParseVariants reads a comma-separated list of variants, keeping the ranked ones.
func ParseVariants(raw string) []Variant {
	out := []Variant{}
	seen := map[Variant]bool{}
	for _, part := range strings.Split(raw, ",") {
		variant := Variant(strings.ToLower(strings.TrimSpace(part)))
		if !isRanked(variant) || seen[variant] {
			continue
		}
		seen[variant] = true
		out = append(out, variant)
	}
	if len(out) == 0 {
		return []Variant{Moving}
	}
	return out
}

func isRanked(variant Variant) bool {
	for _, ranked := range Ranked {
		if ranked == variant {
			return true
		}
	}
	return false
}

// Config is the match config a variant plays.
func Config(variant Variant) contracts.MatchConfig {
	cfg := contracts.MatchConfig{
		Ruleset:        contracts.RulesetMoving,
		StreetNames:    contracts.StreetNamesShown,
		MultiplierMode: contracts.MultiplierIndividual,
	}
	switch variant {
	case NoMove, NoMoveHidden:
		cfg.Ruleset = contracts.RulesetNoMove
	case NMPZ, NMPZHidden:
		cfg.Ruleset = contracts.RulesetNMPZ
	}
	switch variant {
	case MovingHidden, NoMoveHidden, NMPZHidden:
		cfg.StreetNames = contracts.StreetNamesHidden
	}
	return contracts.NormalizeMatchConfig(cfg)
}

// freshFor is how long a ticket counts after its player's socket last refreshed it.
const freshFor = 30 * time.Second

// Ticket is a player waiting in one variant.
type Ticket struct {
	UserID   string
	Variant  Variant
	MMR      int
	JoinedAt time.Time
}

type PGStore struct {
	pool *pgxpool.Pool
	db   *db.Queries
}

func NewPGStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool, db: db.New(pool)}
}

// Join queues the player for these variants only, at the given rating.
func (s *PGStore) Join(ctx context.Context, userID string, variants []Variant, mmr int) error {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	return storekit.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.db.WithTx(tx)
		if err := q.DeleteQueueTickets(ctx, id); err != nil {
			return err
		}
		for _, variant := range variants {
			if err := q.UpsertQueueTicket(ctx, db.UpsertQueueTicketParams{UserID: id, Variant: variant, Mmr: int32(mmr)}); err != nil {
				return err
			}
		}
		return nil
	})
}

// Touch keeps the player's tickets fresh. It reports whether they are still queued: false once a
// pairing took them.
func (s *PGStore) Touch(ctx context.Context, userID string) (bool, error) {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return false, err
	}
	n, err := s.db.TouchQueueTickets(ctx, id)
	return n > 0, err
}

// Leave takes the player out of every variant.
func (s *PGStore) Leave(ctx context.Context, userID string) error {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	return s.db.DeleteQueueTickets(ctx, id)
}

// LeaveTx takes the player out of every variant in the caller's transaction.
func LeaveTx(ctx context.Context, tx pgx.Tx, userID string) error {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	return db.New(tx).DeleteQueueTickets(ctx, id)
}

// LockTx takes the matchmaker's lock for the caller's transaction, reporting whether it got it.
// Only one matchmaking pass runs at a time, whichever process starts it.
func LockTx(ctx context.Context, tx pgx.Tx) (bool, error) {
	return db.New(tx).TryMatchmakerLock(ctx)
}

// TicketsTx lists the fresh tickets, oldest first, and drops the stale ones.
func TicketsTx(ctx context.Context, tx pgx.Tx) ([]Ticket, error) {
	q := db.New(tx)
	if err := q.DeleteStaleQueueTickets(ctx, freshFor.Seconds()); err != nil {
		return nil, err
	}
	rows, err := q.ListQueueTickets(ctx, freshFor.Seconds())
	if err != nil {
		return nil, err
	}
	out := make([]Ticket, len(rows))
	for i, row := range rows {
		out[i] = Ticket{UserID: storekit.UUIDVal(row.UserID), Variant: row.Variant, MMR: int(row.Mmr), JoinedAt: row.JoinedAt.Time}
	}
	return out, nil
}

// TakeTx takes both players out of every variant, reporting whether both were still queued.
func TakeTx(ctx context.Context, tx pgx.Tx, a, b string) (bool, error) {
	rows, err := db.New(tx).TakeQueueTickets(ctx, storekit.UUIDList([]string{a, b}))
	if err != nil {
		return false, err
	}
	taken := map[string]bool{}
	for _, row := range rows {
		taken[storekit.UUIDVal(row.UserID)] = true
	}
	return taken[a] && taken[b], nil
}
