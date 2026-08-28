package persistence

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	db "geoduels/pkg/persistence/sqlc/db"
	"time"
)

const storageCleanupAdvisoryLock int64 = 0x47444d41494e54

func (s *DB) CleanupStorage(n int) (StorageCleanupResult, error) {
	if n <= 0 {
		n = 1000
	}
	if n > 10000 {
		n = 10000
	}
	ctx, c := context.WithTimeout(context.Background(), 30*time.Second)
	defer c()
	var o StorageCleanupResult
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		q := s.db.WithTx(tx)
		ok, e := q.TryAdvisoryLock(ctx, storageCleanupAdvisoryLock)
		if e != nil || !ok {
			return e
		}
		xs, e := q.ListLegacyReplays(ctx, int32(min(n, 1000)))
		if e != nil {
			return e
		}
		for _, x := range xs {
			z, h, e := compressReplay([]byte(x.ReplayJson))
			if e != nil {
				return e
			}
			t, e := q.CompressReplay(ctx, db.CompressReplayParams{MatchID: x.MatchID, ReplayZstd: z, ReplayCodec: pgtype.Int2{Int16: replayCodecZstd, Valid: true}, ReplaySchemaVersion: pgtype.Int2{Int16: replaySchemaVersion, Valid: true}, ReplayUncompressedBytes: pgtype.Int4{Int32: int32(len(x.ReplayJson)), Valid: true}, ReplaySha256: h[:]})
			if e != nil {
				return e
			}
			o.ReplaysCompressed += t.RowsAffected()
		}
		rs := []struct {
			p *int64
			f func(context.Context, int32) (pgconn.CommandTag, error)
		}{{&o.ExpiredReplays, q.DeleteExpiredReplays}, {&o.MatchPlans, q.DeleteMatchPlans}, {&o.MatchSessions, q.DeleteMatchSessions}, {&o.RuntimeMatches, q.DeleteRuntimeMatches}, {&o.ChatMessages, q.DeleteChatMessages}, {&o.ChatConversations, q.DeleteChatConversations}, {&o.AuthSessions, q.DeleteAuthSessions}, {&o.Parties, q.DeleteParties}, {&o.MapUploadEvents, q.DeleteMapUploadEvents}, {&o.MapDailyUsers, q.DeleteMapDailyUsers}, {&o.UserNotifications, q.DeleteUserNotifications}, {&o.NotificationOutbox, q.DeleteNotificationOutbox}, {&o.DiscordSyncOutbox, q.DeleteDiscordSyncOutbox}}
		for _, r := range rs {
			t, e := r.f(ctx, int32(n))
			if e != nil {
				return e
			}
			*r.p = t.RowsAffected()
		}
		_, e = q.DeleteUserEvents(ctx, int32(n))
		return e
	})
	return o, err
}
func (s *DB) ReconcileStaleMatchSessions(g time.Duration, n int) (int64, error) {
	if g <= 0 {
		g = 5 * time.Minute
	}
	if n <= 0 {
		n = 1000
	}
	ctx, c := context.WithTimeout(context.Background(), 15*time.Second)
	defer c()
	var out int64
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		q := s.db.WithTx(tx)
		ids, e := q.ListStaleMatchSessionIDs(ctx, db.ListStaleMatchSessionIDsParams{Column1: pgtype.Interval{Microseconds: g.Microseconds(), Valid: true}, Limit: int32(min(n, 10000))})
		if e != nil {
			return e
		}
		out = int64(len(ids))
		if len(ids) == 0 {
			return nil
		}
		if e = q.EndMatchSessions(ctx, ids); e != nil {
			return e
		}
		if e = q.EndRuntimeMatches(ctx, ids); e != nil {
			return e
		}
		if e = q.ReopenPartiesForEndedSessions(ctx, ids); e != nil {
			return e
		}
		e = q.ResetPartyMembersForEndedSessions(ctx, ids)
		return e
	})
	return out, err
}

var _ StorageMaintenance = (*DB)(nil)
