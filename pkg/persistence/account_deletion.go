package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *pgStore) DeleteAccount(userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("userID required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var isBanned bool
	var banReason string
	if err := tx.QueryRow(ctx, `
		select coalesce(banned_at is not null and (ban_expires_at is null or ban_expires_at > now()), false), coalesce(ban_reason, '')
		from users
		where id = $1
	`, userID).Scan(&isBanned, &banReason); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("user not found")
		}
		return err
	}
	if isBanned {
		if strings.TrimSpace(banReason) == "" {
			banReason = "account deleted while banned"
		}
		if err := banUserOAuthIdentities(ctx, tx, userID, banReason, ""); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		update auth_sessions
		set revoked_at = coalesce(revoked_at, now())
		where user_id = $1 and revoked_at is null
	`, userID); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `
		select provider_user_id
		from user_identities
		where user_id = $1
		  and provider = $2
	`, userID, IdentityProviderDiscord)
	if err != nil {
		return err
	}
	for rows.Next() {
		var discordUserID string
		if err := rows.Scan(&discordUserID); err != nil {
			rows.Close()
			return err
		}
		if err := enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionCleanupRoles, discordUserID); err != nil {
			rows.Close()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if _, err := tx.Exec(ctx, `
		insert into user_identity_history(user_id, provider, provider_user_id, email, provider_name, first_seen_at, last_seen_at, deleted_at)
		select user_id, provider, provider_user_id, email, provider_name, created_at, last_seen_at, now()
		from user_identities
		where user_id = $1
		on conflict (user_id, provider, provider_user_id) do update set
			email = excluded.email,
			provider_name = excluded.provider_name,
			last_seen_at = greatest(user_identity_history.last_seen_at, excluded.last_seen_at),
			deleted_at = coalesce(user_identity_history.deleted_at, excluded.deleted_at)
	`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		delete from user_identities
		where user_id = $1
	`, userID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		update users
		set email = null,
			display_name = 'Deleted player',
			avatar_url = null,
			nickname_claimed_at = null,
			account_type = 'guest',
			is_admin = false,
			is_moderator = false,
			deleted_at = coalesce(deleted_at, now())
		where id = $1
	`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return tx.Commit(ctx)
}

func (s *pgStore) DeleteGuestAccountsOlderThan(ttl time.Duration, limit int) (int, error) {
	if ttl <= 0 || limit <= 0 {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `
		with batch as materialized (
			select id
			from users
			where account_type = 'guest'
			  and deleted_at is null
			  and created_at < now() - ($1::double precision * interval '1 second')
			order by created_at asc
			limit $2
		),
		del_ranked as (
			delete from ranked_stats
			using batch
			where ranked_stats.user_id = batch.id
		),
		del_ranks as (
			delete from ranks
			using batch
			where ranks.user_id = batch.id
		),
		del_stats as (
			delete from user_stats
			using batch
			where user_stats.user_id = batch.id
		)
		delete from users
		using batch
		where users.id = batch.id
		returning users.id
	`, ttl.Seconds(), limit)
	if err != nil {
		return 0, err
	}
	deleted := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		deleted++
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return deleted, nil
}
