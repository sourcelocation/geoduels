package persistence

import (
	"context"
	"errors"
	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

func (s *DB) GetRuntimeMatch(ctx context.Context, matchID string) (RuntimeMatch, bool, error) {
	if matchID == "" {
		return RuntimeMatch{}, false, errors.New("matchID required")
	}
	var id pgtype.UUID
	if err := id.Scan(matchID); err != nil {
		return RuntimeMatch{}, false, errors.New("matchID required")
	}
	r, err := s.db.GetRuntimeMatch(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return RuntimeMatch{}, false, nil
	}
	if err != nil {
		return RuntimeMatch{}, false, err
	}
	return RuntimeMatch{MatchID: matchID, State: string(r.State), OwnerEpoch: r.OwnerEpoch, StartedAt: r.StartedAt.Time, EndedAt: r.EndedAt.Time}, true, nil
}

func (s *DB) RecordRuntimeMatch(ctx context.Context, matchID, state string, ownerEpoch int64, terminal bool) error {
	if matchID == "" {
		return errors.New("matchID required")
	}
	var id pgtype.UUID
	if err := id.Scan(matchID); err != nil {
		return errors.New("matchID required")
	}
	if terminal {
		return s.db.RecordRuntimeMatchTerminal(ctx, db.RecordRuntimeMatchTerminalParams{ID: id, State: db.GdRuntimeState(state), OwnerEpoch: ownerEpoch})
	}
	return s.db.RecordRuntimeMatchLive(ctx, db.RecordRuntimeMatchLiveParams{ID: id, State: db.GdRuntimeState(state), OwnerEpoch: ownerEpoch})
}

func (s *DB) ExpireStaleRuntimeMatches(ctx context.Context, mode string, olderThan time.Duration) error {
	if mode == "" || olderThan <= 0 {
		return nil
	}
	return s.db.ExpireStaleRuntimeMatches(ctx, db.ExpireStaleRuntimeMatchesParams{State: db.GdRuntimeStateEnded, State_2: db.GdRuntimeStateLive, Mode: db.GdMatchMode(mode), Column4: pgtype.Interval{Microseconds: olderThan.Microseconds(), Valid: true}})
}
