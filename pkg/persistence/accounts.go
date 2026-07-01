package persistence

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"geoduels/pkg/contentfilter"
)

var ErrNicknameTaken = errors.New("nickname already taken")

func chooseProviderIdentityUser(existingProviderUserID, existingEmailUserID, existingEmailAccountType, linkUserID, linkAccountType string) (string, bool) {
	if existingProviderUserID != "" {
		return existingProviderUserID, false
	}
	if existingEmailUserID != "" {
		return existingEmailUserID, existingEmailAccountType == "guest"
	}
	if linkUserID != "" && linkAccountType != "" {
		return linkUserID, linkAccountType == "guest"
	}
	return newUserID(), false
}

func chooseGoogleIdentityUser(existingGoogleUserID, existingEmailUserID, existingEmailAccountType, linkUserID, linkAccountType string) (string, bool) {
	return chooseProviderIdentityUser(existingGoogleUserID, existingEmailUserID, existingEmailAccountType, linkUserID, linkAccountType)
}

func providerUsesAccountEmail(provider string) bool {
	return provider == IdentityProviderGoogle || provider == IdentityProviderDiscord
}

func isSyntheticOAuthEmail(email string) bool {
	email = strings.TrimSpace(strings.ToLower(email))
	return strings.HasSuffix(email, "@oauth.invalid") || strings.HasSuffix(email, ".oauth.invalid")
}

func providerAccountEmail(provider, email string) any {
	email = strings.TrimSpace(email)
	if providerUsesAccountEmail(provider) && email != "" && !isSyntheticOAuthEmail(email) {
		return email
	}
	return nil
}

func (s *pgStore) UpsertGoogleIdentity(googleSub, email, googleName, avatarURL, linkUserID string) (Identity, error) {
	return s.UpsertProviderIdentity(IdentityProviderGoogle, googleSub, email, googleName, avatarURL, linkUserID)
}

func (s *pgStore) UpsertProviderIdentity(provider, providerUserID, email, providerName, avatarURL, linkUserID string) (Identity, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" {
		return Identity{}, errors.New("provider required")
	}
	if providerUserID == "" {
		return Identity{}, errors.New("provider subject required")
	}
	if email == "" {
		email = providerUserID + "@oauth.invalid"
	}
	if providerName == "" {
		providerName = providerUserID
	}
	if banned, _, err := s.IsProviderIdentityBanned(provider, providerUserID); err != nil {
		return Identity{}, err
	} else if banned {
		return Identity{}, errors.New("provider identity banned")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Identity{}, err
	}
	defer tx.Rollback(ctx)
	seasonID, err := activeSeasonIDTx(ctx, tx)
	if err != nil {
		return Identity{}, err
	}

	var existingProviderUserID string
	row := tx.QueryRow(ctx, `
		select user_id
		from user_identities
		where provider = $1 and provider_user_id = $2
	`, provider, providerUserID)
	if err := row.Scan(&existingProviderUserID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, err
	}
	var previousLinkedProviderUserID string
	if provider == IdentityProviderDiscord && linkUserID != "" {
		if err := tx.QueryRow(ctx, `
			select provider_user_id
			from user_identities
			where user_id = $1 and provider = $2
		`, linkUserID, provider).Scan(&previousLinkedProviderUserID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, err
		}
	}
	var existingEmailUserID string
	var existingEmailAccountType string
	if providerUsesAccountEmail(provider) && existingProviderUserID == "" && email != "" && !isSyntheticOAuthEmail(email) {
		row = tx.QueryRow(ctx, `
			select id, account_type
			from users
			where lower(email) = lower($1)
			order by case when account_type = 'registered' then 0 else 1 end, created_at asc
			limit 1
		`, email)
		if err := row.Scan(&existingEmailUserID, &existingEmailAccountType); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, err
		}
	}
	var linkAccountType string
	if existingProviderUserID == "" && existingEmailUserID == "" && linkUserID != "" {
		row = tx.QueryRow(ctx, `
			select account_type
			from users
			where id = $1
		`, linkUserID)
		if err := row.Scan(&linkAccountType); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, err
		}
	}
	userID, _ := chooseProviderIdentityUser(existingProviderUserID, existingEmailUserID, existingEmailAccountType, linkUserID, linkAccountType)
	userEmail := providerAccountEmail(provider, email)

	if _, err := tx.Exec(ctx, `
		insert into users (id, email, display_name, avatar_url, account_type)
		values ($1, $2, $3, $4, 'registered')
		on conflict (id) do update set
			email = coalesce(excluded.email, users.email),
			display_name = case
				when users.account_type = 'guest' then excluded.display_name
				when users.nickname_claimed_at is not null and nullif(users.display_name, '') is not null then users.display_name
				else excluded.display_name
			end,
			avatar_url = excluded.avatar_url,
			account_type = 'registered'
	`, userID, userEmail, providerName, nullable(avatarURL)); err != nil {
		return Identity{}, err
	}
	if existingProviderUserID != "" {
		if _, err := tx.Exec(ctx, `
			insert into user_identities(user_id, provider, provider_user_id, email, provider_name, avatar_url, last_seen_at)
			values($1, $2, $3, $4, $5, $6, now())
			on conflict (provider, provider_user_id) do update set
				user_id = excluded.user_id,
				email = excluded.email,
				provider_name = excluded.provider_name,
				avatar_url = case
					when excluded.avatar_url is null then user_identities.avatar_url
					when excluded.avatar_url = '' then user_identities.avatar_url
					else excluded.avatar_url
				end,
				last_seen_at = now()
		`, userID, provider, providerUserID, email, providerName, nullable(avatarURL)); err != nil {
			return Identity{}, err
		}
	} else {
		if _, err := tx.Exec(ctx, `
			insert into user_identities(user_id, provider, provider_user_id, email, provider_name, avatar_url, last_seen_at)
			values($1, $2, $3, $4, $5, $6, now())
			on conflict (user_id, provider) do update set
				provider_user_id = excluded.provider_user_id,
				email = excluded.email,
				provider_name = excluded.provider_name,
				avatar_url = case
					when excluded.avatar_url is null then user_identities.avatar_url
					when excluded.avatar_url = '' then user_identities.avatar_url
					else excluded.avatar_url
				end,
				last_seen_at = now()
		`, userID, provider, providerUserID, email, providerName, nullable(avatarURL)); err != nil {
			return Identity{}, err
		}
	}
	if err := recordUserIdentityHistory(ctx, tx, userID, provider, providerUserID, email, providerName); err != nil {
		return Identity{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into ranks (user_id, mode, mmr, season_id)
		values ($1, $2, $4, $3)
		on conflict (user_id, mode, season_id) do nothing
	`, userID, modeDuel, seasonID, initialMMR); err != nil {
		return Identity{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into user_stats (user_id, games_played, wins)
		values ($1, 0, 0)
		on conflict (user_id) do nothing
	`, userID); err != nil {
		return Identity{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into ranked_stats (user_id, mode, season_id, games_played, wins)
		values ($1, $2, $3, 0, 0)
		on conflict (user_id, mode, season_id) do nothing
	`, userID, modeDuel, seasonID); err != nil {
		return Identity{}, err
	}
	if provider == IdentityProviderDiscord {
		if previousLinkedProviderUserID != "" && previousLinkedProviderUserID != providerUserID {
			if err := enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionCleanupRoles, previousLinkedProviderUserID); err != nil {
				return Identity{}, err
			}
		}
		if err := enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionSync, providerUserID); err != nil {
			return Identity{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Identity{}, err
	}
	return s.GetIdentity(userID)
}

func (s *pgStore) LinkProviderIdentity(provider, providerUserID, email, providerName, avatarURL, linkUserID string) (Identity, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	providerUserID = strings.TrimSpace(providerUserID)
	linkUserID = strings.TrimSpace(linkUserID)
	if provider == "" {
		return Identity{}, errors.New("provider required")
	}
	if providerUserID == "" {
		return Identity{}, errors.New("provider subject required")
	}
	if linkUserID == "" {
		return Identity{}, errors.New("link user required")
	}
	if email == "" {
		email = providerUserID + "@oauth.invalid"
	}
	if providerName == "" {
		providerName = providerUserID
	}
	if banned, _, err := s.IsProviderIdentityBanned(provider, providerUserID); err != nil {
		return Identity{}, err
	} else if banned {
		return Identity{}, errors.New("provider identity banned")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Identity{}, err
	}
	defer tx.Rollback(ctx)
	seasonID, err := activeSeasonIDTx(ctx, tx)
	if err != nil {
		return Identity{}, err
	}

	var linkAccountType string
	if err := tx.QueryRow(ctx, `
		select account_type
		from users
		where id = $1
	`, linkUserID).Scan(&linkAccountType); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, errors.New("link user not found")
		}
		return Identity{}, err
	}

	var existingProviderUserID string
	err = tx.QueryRow(ctx, `
		select user_id
		from user_identities
		where provider = $1 and provider_user_id = $2
	`, provider, providerUserID).Scan(&existingProviderUserID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Identity{}, err
	}
	if existingProviderUserID != "" && existingProviderUserID != linkUserID {
		return Identity{}, errors.New("provider identity already linked")
	}
	var previousLinkedProviderUserID string
	if provider == IdentityProviderDiscord {
		if err := tx.QueryRow(ctx, `
			select provider_user_id
			from user_identities
			where user_id = $1 and provider = $2
		`, linkUserID, provider).Scan(&previousLinkedProviderUserID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, err
		}
	}
	if providerUsesAccountEmail(provider) && email != "" && !isSyntheticOAuthEmail(email) {
		var existingEmailUserID string
		err = tx.QueryRow(ctx, `
			select id
			from users
			where lower(email) = lower($1)
			limit 1
		`, email).Scan(&existingEmailUserID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, err
		}
		if existingEmailUserID != "" && existingEmailUserID != linkUserID {
			return Identity{}, errors.New("provider identity already linked")
		}
	}

	userEmail := providerAccountEmail(provider, email)
	if _, err := tx.Exec(ctx, `
		insert into users (id, email, display_name, avatar_url, account_type)
		values ($1, $2, $3, $4, 'registered')
		on conflict (id) do update set
			email = coalesce(excluded.email, users.email),
			display_name = case
				when users.account_type = 'guest' then excluded.display_name
				when users.nickname_claimed_at is not null and nullif(users.display_name, '') is not null then users.display_name
				else excluded.display_name
			end,
			avatar_url = excluded.avatar_url,
			account_type = 'registered'
	`, linkUserID, userEmail, providerName, nullable(avatarURL)); err != nil {
		return Identity{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into user_identities(user_id, provider, provider_user_id, email, provider_name, avatar_url, last_seen_at)
		values($1, $2, $3, $4, $5, $6, now())
		on conflict (user_id, provider) do update set
			provider_user_id = excluded.provider_user_id,
			email = excluded.email,
			provider_name = excluded.provider_name,
			avatar_url = case
				when excluded.avatar_url is null then user_identities.avatar_url
				when excluded.avatar_url = '' then user_identities.avatar_url
				else excluded.avatar_url
			end,
			last_seen_at = now()
	`, linkUserID, provider, providerUserID, email, providerName, nullable(avatarURL)); err != nil {
		return Identity{}, err
	}
	if err := recordUserIdentityHistory(ctx, tx, linkUserID, provider, providerUserID, email, providerName); err != nil {
		return Identity{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into ranks (user_id, mode, mmr, season_id)
		values ($1, $2, $4, $3)
		on conflict (user_id, mode, season_id) do nothing
	`, linkUserID, modeDuel, seasonID, initialMMR); err != nil {
		return Identity{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into ranked_stats (user_id, mode, season_id, games_played, wins)
		values ($1, $2, $3, 0, 0)
		on conflict (user_id, mode, season_id) do nothing
	`, linkUserID, modeDuel, seasonID); err != nil {
		return Identity{}, err
	}
	if provider == IdentityProviderDiscord {
		if previousLinkedProviderUserID != "" && previousLinkedProviderUserID != providerUserID {
			if err := enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionCleanupRoles, previousLinkedProviderUserID); err != nil {
				return Identity{}, err
			}
		}
		if err := enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionSync, providerUserID); err != nil {
			return Identity{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Identity{}, err
	}
	return s.GetIdentity(linkUserID)
}

func (s *pgStore) GoogleIdentityExists(googleSub string) (bool, error) {
	return s.ProviderIdentityExists(IdentityProviderGoogle, googleSub)
}

func (s *pgStore) ProviderIdentityExists(provider, providerUserID string) (bool, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	if provider == "" || strings.TrimSpace(providerUserID) == "" {
		return false, errors.New("provider and subject required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var exists bool
	if err := s.pool.QueryRow(ctx, `
		select exists(
			select 1 from user_identities
			where provider = $1 and provider_user_id = $2
		)
	`, provider, providerUserID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *pgStore) IsProviderIdentityBanned(provider, providerUserID string) (bool, string, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	providerUserID = strings.TrimSpace(providerUserID)
	if provider == "" || providerUserID == "" {
		return false, "", errors.New("provider and subject required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var reason string
	err := s.pool.QueryRow(ctx, `
		select coalesce(reason, '')
		from oauth_identity_bans
		where provider = $1
		  and provider_user_id = $2
		  and revoked_at is null
		limit 1
	`, provider, providerUserID).Scan(&reason)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, "", nil
		}
		return false, "", err
	}
	return true, reason, nil
}

func recordUserIdentityHistory(ctx context.Context, tx pgx.Tx, userID, provider, providerUserID, email, providerName string) error {
	_, err := tx.Exec(ctx, `
		insert into user_identity_history(user_id, provider, provider_user_id, email, provider_name, first_seen_at, last_seen_at, deleted_at)
		values($1, $2, $3, $4, $5, now(), now(), null)
		on conflict (user_id, provider, provider_user_id) do update set
			email = excluded.email,
			provider_name = excluded.provider_name,
			last_seen_at = now(),
			deleted_at = null
	`, userID, provider, providerUserID, email, providerName)
	return err
}

func enqueueDiscordSyncTx(ctx context.Context, tx pgx.Tx, action, discordUserID string) error {
	action = strings.TrimSpace(action)
	discordUserID = strings.TrimSpace(discordUserID)
	if action == "" || discordUserID == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `
		insert into discord_sync_outbox(action, discord_user_id)
		values($1, $2)
		on conflict (action, discord_user_id) where processed_at is null do update set
			next_attempt_at = least(discord_sync_outbox.next_attempt_at, excluded.next_attempt_at),
			last_error = null
	`, action, discordUserID)
	return err
}

func enqueueDiscordSyncForUserTx(ctx context.Context, tx pgx.Tx, userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
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
	var discordUserIDs []string
	for rows.Next() {
		var discordUserID string
		if err := rows.Scan(&discordUserID); err != nil {
			rows.Close()
			return err
		}
		discordUserIDs = append(discordUserIDs, discordUserID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, discordUserID := range discordUserIDs {
		if err := enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionSync, discordUserID); err != nil {
			return err
		}
	}
	return nil
}

func (s *pgStore) UnlinkProviderIdentity(userID, provider string) (Identity, error) {
	userID = strings.TrimSpace(userID)
	provider = strings.TrimSpace(strings.ToLower(provider))
	if userID == "" || provider == "" {
		return Identity{}, errors.New("user and provider required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Identity{}, err
	}
	defer tx.Rollback(ctx)

	var providerCount int
	if err := tx.QueryRow(ctx, `
		select count(*)
		from user_identities
		where user_id = $1
	`, userID).Scan(&providerCount); err != nil {
		return Identity{}, err
	}
	if providerCount <= 1 {
		return Identity{}, errors.New("cannot unlink the last sign-in method")
	}
	var unlinkedProviderUserID string
	if provider == IdentityProviderDiscord {
		if err := tx.QueryRow(ctx, `
			select provider_user_id
			from user_identities
			where user_id = $1 and provider = $2
		`, userID, provider).Scan(&unlinkedProviderUserID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, err
		}
	}
	tag, err := tx.Exec(ctx, `
		delete from user_identities
		where user_id = $1 and provider = $2
	`, userID, provider)
	if err != nil {
		return Identity{}, err
	}
	if tag.RowsAffected() == 0 {
		return Identity{}, errors.New("provider is not linked")
	}
	if _, err := tx.Exec(ctx, `
		update user_identity_history
		set deleted_at = coalesce(deleted_at, now())
		where user_id = $1
		  and provider = $2
		  and deleted_at is null
	`, userID, provider); err != nil {
		return Identity{}, err
	}
	if provider == IdentityProviderGoogle {
		if _, err := tx.Exec(ctx, `
			update users
			set email = null
			where id = $1
			  and not exists (
				select 1 from user_identities
				where user_id = $1 and provider = 'google'
			  )
		`, userID); err != nil {
			return Identity{}, err
		}
	}
	if provider == IdentityProviderDiscord {
		if err := enqueueDiscordSyncTx(ctx, tx, DiscordSyncActionCleanupRoles, unlinkedProviderUserID); err != nil {
			return Identity{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Identity{}, err
	}
	return s.GetIdentity(userID)
}

func (s *pgStore) CreateGuestIdentity() (Identity, error) {
	userID := newUserID()
	if err := s.UpsertUser(userID, "", "Guest"); err != nil {
		return Identity{}, err
	}
	return s.GetIdentity(userID)
}

func (s *pgStore) GetIdentity(sub string) (Identity, error) {
	if sub == "" {
		return Identity{}, errors.New("subject required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	row := s.pool.QueryRow(ctx, `
		select
			u.id,
			coalesce(u.email, ui.email, ''),
			coalesce(ui.provider_name, ''),
			coalesce(u.avatar_url, ui.avatar_url, ''),
			coalesce(u.account_type = 'registered' and u.nickname_claimed_at is null, false) as nickname_required,
				coalesce(nullif(u.display_name, ''), ui.provider_name, u.id::text),
				u.account_type,
				coalesce(u.is_admin, false),
				coalesce(u.is_moderator, false),
				coalesce(u.banned_at is not null and (u.ban_expires_at is null or u.ban_expires_at > now()), false),
				coalesce(u.ban_reason, '')
		from users u
		left join lateral (
			select email, provider_name, avatar_url
			from user_identities
			where user_id = u.id
			  and provider in ('discord', 'google')
			order by case provider when 'discord' then 0 when 'google' then 1 else 2 end, created_at asc
			limit 1
		) ui on true
		where u.id = $1
	`, sub)
	var out Identity
	if err := row.Scan(
		&out.Sub,
		&out.Email,
		&out.GoogleName,
		&out.AvatarURL,
		&out.NicknameRequired,
		&out.DisplayName,
		&out.AccountType,
		&out.IsAdmin,
		&out.IsModerator,
		&out.IsBanned,
		&out.BanReason,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Identity{}, errors.New("identity not found")
		}
		return Identity{}, err
	}
	out.ProviderName = out.GoogleName
	out.LinkedProviders, _ = s.userProviders(ctx, sub)
	out.AuthMigrationRequired = false
	out.RecoveryAvailable = false
	return out, nil
}

func (s *pgStore) userProviders(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.pool.Query(ctx, `
		select provider
		from user_identities
		where user_id = $1
		order by case provider when 'discord' then 0 when 'google' then 1 else 2 end, provider
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var providers []string
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	return providers, rows.Err()
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func (s *pgStore) SetNickname(sub, displayName string) error {
	if sub == "" {
		return errors.New("subject required")
	}
	if displayName == "" {
		return errors.New("display name required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tag, err := s.pool.Exec(ctx, `
		update users
		set display_name = $2,
			nickname_claimed_at = coalesce(nickname_claimed_at, now())
		where id = $1
		  and coalesce(account_type, 'registered') <> 'guest'
	`, sub, displayName)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "users_claimed_nickname_unique" {
			return ErrNicknameTaken
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *pgStore) SuggestNickname(sub, displayName string) (string, error) {
	base := contentfilter.NicknameSuggestionBase(displayName)
	if _, err := contentfilter.ValidateNickname(base); err != nil {
		base = "Player"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	available := func(candidate string) (bool, error) {
		var taken bool
		err := s.pool.QueryRow(ctx, `
			select exists(
				select 1
				from users
				where id <> $1
				  and account_type = 'registered'
				  and nickname_claimed_at is not null
				  and lower(display_name) = lower($2)
			)
		`, sub, candidate).Scan(&taken)
		return !taken, err
	}
	if ok, err := available(base); err != nil {
		return "", err
	} else if ok {
		return base, nil
	}
	prefix := base
	if len(prefix) > contentfilter.MaxNicknameLength-4 {
		prefix = prefix[:contentfilter.MaxNicknameLength-4]
	}
	for range 32 {
		value, err := rand.Int(rand.Reader, big.NewInt(9000))
		if err != nil {
			return "", err
		}
		candidate := fmt.Sprintf("%s%04d", prefix, value.Int64()+1000)
		if ok, err := available(candidate); err != nil {
			return "", err
		} else if ok {
			return candidate, nil
		}
	}
	return "", errors.New("nickname suggestion unavailable")
}

func (s *pgStore) SetUserAdmin(userID string, isAdmin bool) error {
	if userID == "" {
		return errors.New("user id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		update users
		set is_admin = $2,
			is_moderator = case when $2 then true else is_moderator end
		where id = $1
	`, userID, isAdmin)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	if isAdmin {
		if _, err := tx.Exec(ctx, `
			insert into user_roles(user_id, role, granted_at, reason)
			values($1, 'admin', now(), 'legacy admin toggle')
			on conflict (user_id, role) where revoked_at is null do nothing
		`, userID); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(ctx, `
			update user_roles
			set revoked_at = coalesce(revoked_at, now()), reason = coalesce(nullif(reason, ''), 'legacy admin toggle')
			where user_id = $1 and role = 'admin' and revoked_at is null
		`, userID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *pgStore) SetUserModerator(userID string, isModerator bool) error {
	if userID == "" {
		return errors.New("user id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		update users
		set is_moderator = $2
		where id = $1
	`, userID, isModerator)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	if isModerator {
		if _, err := tx.Exec(ctx, `
			insert into user_roles(user_id, role, granted_at, reason)
			values($1, 'moderator', now(), 'legacy moderator toggle')
			on conflict (user_id, role) where revoked_at is null do nothing
		`, userID); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(ctx, `
			update user_roles
			set revoked_at = coalesce(revoked_at, now()), reason = coalesce(nullif(reason, ''), 'legacy moderator toggle')
			where user_id = $1 and role = 'moderator' and revoked_at is null
		`, userID); err != nil {
			return err
		}
		if err := removeGeoDuelsTeamBadgeTx(ctx, tx, userID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
