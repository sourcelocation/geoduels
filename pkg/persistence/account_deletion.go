package persistence

import (
	"context"
	"errors"
	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"strings"
	"time"
)

func deletionUUID(s string) (pgtype.UUID, error) { var u pgtype.UUID; return u, u.Scan(s) }
func (s *DB) DeleteAccount(userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("userID required")
	}
	ctx, c := context.WithTimeout(context.Background(), 4*time.Second)
	defer c()
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	id, e := deletionUUID(userID)
	if e != nil {
		return e
	}
	u, e := q.GetDeletionUser(ctx, id)
	if e != nil {
		if errors.Is(e, pgx.ErrNoRows) {
			return errors.New("user not found")
		}
		return e
	}
	if banned, ok := u.Coalesce.(bool); ok && banned {
		r := strings.TrimSpace(u.BanReason)
		if r == "" {
			r = "account deleted while banned"
		}
		if e = db.New(tx).BanUserOAuthIdentities(ctx, db.BanUserOAuthIdentitiesParams{BannedUserID: id, Column2: r}); e != nil {
			return e
		}
	}
	if e = q.RevokeDeletionSessions(ctx, id); e != nil {
		return e
	}
	xs, e := q.ListDeletionDiscordIdentities(ctx, db.ListDeletionDiscordIdentitiesParams{UserID: id, Provider: db.GdOauthProvider(IdentityProviderDiscord)})
	if e != nil {
		return e
	}
	for _, x := range xs {
		if e = enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionCleanupRoles, x); e != nil {
			return e
		}
	}
	if e = q.ArchiveDeletionIdentities(ctx, id); e != nil {
		return e
	}
	if e = q.DeleteDeletionIdentities(ctx, id); e != nil {
		return e
	}
	if _, e = q.AnonymizeDeletedUser(ctx, id); e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (s *DB) DeleteGuestAccountsOlderThan(ttl time.Duration, limit int) (int, error) {
	if ttl <= 0 || limit <= 0 {
		return 0, nil
	}
	ctx, c := context.WithTimeout(context.Background(), 30*time.Second)
	defer c()
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback(ctx)
	xs, e := db.New(tx).DeleteOldGuestAccounts(ctx, db.DeleteOldGuestAccountsParams{TtlSeconds: ttl.Seconds(), AccountLimit: int32(limit)})
	if e != nil {
		return 0, e
	}
	if e = tx.Commit(ctx); e != nil {
		return 0, e
	}
	return len(xs), nil
}
