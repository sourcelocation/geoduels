package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"

	"geoduels/pkg/contracts"
)

func (s *pgStore) ListUserRoles() ([]UserRoleGrant, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `
		select
			ur.user_id,
			coalesce(nullif(u.display_name, ''), u.id::text),
			coalesce(u.email, ''),
			ur.role,
			coalesce(ur.granted_by::text, ''),
			ur.granted_at,
			ur.revoked_at,
			coalesce(ur.reason, '')
		from user_roles ur
		left join users u on u.id = ur.user_id
		where ur.revoked_at is null
		order by
			case ur.role when 'admin' then 0 when 'moderator' then 1 else 2 end,
			ur.granted_at desc
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserRoleGrant{}
	for rows.Next() {
		var item UserRoleGrant
		var revokedAt *time.Time
		if err := rows.Scan(&item.UserID, &item.DisplayName, &item.Email, &item.Role, &item.GrantedBy, &item.GrantedAt, &revokedAt, &item.Reason); err != nil {
			return nil, err
		}
		if revokedAt != nil {
			item.RevokedAt = *revokedAt
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func normalizeAdminRole(role string) (string, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	switch role {
	case "admin", "moderator":
		return role, nil
	default:
		return "", errors.New("unsupported role")
	}
}

func (s *pgStore) GrantUserRole(userID, role, grantedBy, reason string) error {
	userID = strings.TrimSpace(userID)
	role, err := normalizeAdminRole(role)
	if err != nil {
		return err
	}
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
		set is_admin = case when $2 = 'admin' then true else is_admin end,
			is_moderator = case when $2 in ('admin', 'moderator') then true else is_moderator end
		where id = $1
	`, userID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	if _, err := tx.Exec(ctx, `
		insert into user_roles(user_id, role, granted_by, granted_at, reason)
		values($1, $2, nullif($3, '')::uuid, now(), nullif($4, ''))
		on conflict (user_id, role) where revoked_at is null do update set
			granted_by = excluded.granted_by,
			reason = excluded.reason
	`, userID, role, strings.TrimSpace(grantedBy), strings.TrimSpace(reason)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *pgStore) RevokeUserRole(userID, role, revokedBy, reason string) error {
	userID = strings.TrimSpace(userID)
	role, err := normalizeAdminRole(role)
	if err != nil {
		return err
	}
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
	if _, err := tx.Exec(ctx, `
		update user_roles
		set revoked_at = coalesce(revoked_at, now()),
			reason = coalesce(nullif($3, ''), reason)
		where user_id = $1 and role = $2 and revoked_at is null
	`, userID, role, strings.TrimSpace(reason)); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		update users
		set is_admin = case when $2 = 'admin' then false else is_admin end,
			is_moderator = case
				when not exists (
					select 1 from user_roles
					where user_id = $1 and role in ('admin', 'moderator') and revoked_at is null
				) then false
				else is_moderator
			end
		where id = $1
	`, userID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	var hasTeamRole bool
	if err := tx.QueryRow(ctx, `
		select exists (
			select 1
			from user_roles
			where user_id = $1 and role in ('admin', 'moderator') and revoked_at is null
		)
	`, userID).Scan(&hasTeamRole); err != nil {
		return err
	}
	if !hasTeamRole {
		if err := removeGeoDuelsTeamBadgeTx(ctx, tx, userID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *pgStore) SearchPlayers(query string, limit int) ([]AdminPlayerSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	pattern := "%"
	trimmed := strings.TrimSpace(query)
	if trimmed != "" {
		pattern = "%" + strings.ToLower(trimmed) + "%"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	seasonID, err := s.activeSeasonID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		select
			u.id,
			coalesce(u.email, ''),
			coalesce(nullif(u.display_name, ''), ui.provider_name, u.id::text),
			coalesce(u.avatar_url, ui.avatar_url, ''),
			coalesce(r.mmr, $3),
				coalesce(us.games_played, 0),
				coalesce(us.wins, 0),
				coalesce(rs.games_played, 0),
				coalesce(u.account_type = 'guest', false),
				coalesce(u.is_admin, false),
				coalesce(u.is_moderator, false),
				coalesce(u.banned_at is not null and (u.ban_expires_at is null or u.ban_expires_at > now()), false),
				coalesce(u.ban_reason, ''),
			u.banned_at,
			coalesce(latest_session.ip_address, ''),
			rep.muted_until
		from users u
		left join lateral (
			select provider_name, avatar_url
			from user_identities
			where user_id = u.id and provider = 'google'
			order by created_at asc
			limit 1
		) ui on true
		left join lateral (
			select ip_address
			from auth_sessions
			where user_id = u.id and coalesce(ip_address, '') <> ''
			order by last_used_at desc, created_at desc
			limit 1
		) latest_session on true
		left join ranks r on r.user_id = u.id and r.mode = $1 and r.season_id = $2
		left join user_stats us on us.user_id = u.id
		left join ranked_stats rs on rs.user_id = u.id and rs.mode = $1 and rs.season_id = $2
		left join moderation_reporter_state rep on rep.user_id = u.id
		where $4 = '%%'
		   or lower(u.id::text) like $4
		   or lower(coalesce(u.email, '')) like $4
		   or lower(coalesce(u.display_name, ui.provider_name, '')) like $4
		   or exists (
			select 1
			from user_identity_history ih
			where ih.user_id = u.id
			  and (
				lower(ih.provider) like $4
				or lower(ih.provider_user_id) like $4
				or lower(coalesce(ih.email, '')) like $4
				or lower(coalesce(ih.provider_name, '')) like $4
			  )
		   )
		order by u.created_at desc, u.id desc
		limit $5
	`, modeDuel, seasonID, initialMMR, pattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]AdminPlayerSummary, 0, limit)
	for rows.Next() {
		var item AdminPlayerSummary
		var bannedAt *time.Time
		var reportMutedUntil *time.Time
		if err := rows.Scan(
			&item.UserID,
			&item.Email,
			&item.DisplayName,
			&item.AvatarURL,
			&item.MMR,
			&item.GamesPlayed,
			&item.Wins,
			&item.RankedGamesPlayed,
			&item.IsGuest,
			&item.IsAdmin,
			&item.IsModerator,
			&item.IsBanned,
			&item.BanReason,
			&bannedAt,
			&item.LastIPAddress,
			&reportMutedUntil,
		); err != nil {
			return nil, err
		}
		if bannedAt != nil {
			item.BannedAt = *bannedAt
		}
		if reportMutedUntil != nil {
			item.ReportMutedUntil = *reportMutedUntil
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := s.populateAdminPlayerIdentities(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *pgStore) getAdminPlayerSummary(ctx context.Context, userID string) (AdminPlayerSummary, error) {
	var item AdminPlayerSummary
	var bannedAt *time.Time
	var reportMutedUntil *time.Time
	seasonID, err := s.activeSeasonID(ctx)
	if err != nil {
		return item, err
	}
	err = s.pool.QueryRow(ctx, `
		select
			u.id,
			coalesce(u.email, ''),
			coalesce(nullif(u.display_name, ''), ui.provider_name, u.id::text),
			coalesce(u.avatar_url, ui.avatar_url, ''),
			coalesce(r.mmr, $4),
			coalesce(us.games_played, 0),
			coalesce(us.wins, 0),
			coalesce(rs.games_played, 0),
			coalesce(u.account_type = 'guest', false),
			coalesce(u.is_admin, false),
			coalesce(u.is_moderator, false),
				coalesce(u.banned_at is not null and (u.ban_expires_at is null or u.ban_expires_at > now()), false),
			coalesce(u.ban_reason, ''),
			u.banned_at,
			coalesce(latest_session.ip_address, ''),
			rep.muted_until
		from users u
		left join lateral (
			select provider_name, avatar_url
			from user_identities
			where user_id = u.id and provider = 'google'
			order by created_at asc
			limit 1
		) ui on true
		left join lateral (
			select ip_address
			from auth_sessions
			where user_id = u.id and coalesce(ip_address, '') <> ''
			order by last_used_at desc, created_at desc
			limit 1
		) latest_session on true
		left join ranks r on r.user_id = u.id and r.mode = $2 and r.season_id = $3
		left join user_stats us on us.user_id = u.id
		left join ranked_stats rs on rs.user_id = u.id and rs.mode = $2 and rs.season_id = $3
		left join moderation_reporter_state rep on rep.user_id = u.id
		where u.id = $1
	`, userID, modeDuel, seasonID, initialMMR).Scan(
		&item.UserID,
		&item.Email,
		&item.DisplayName,
		&item.AvatarURL,
		&item.MMR,
		&item.GamesPlayed,
		&item.Wins,
		&item.RankedGamesPlayed,
		&item.IsGuest,
		&item.IsAdmin,
		&item.IsModerator,
		&item.IsBanned,
		&item.BanReason,
		&bannedAt,
		&item.LastIPAddress,
		&reportMutedUntil,
	)
	if err != nil {
		return AdminPlayerSummary{}, err
	}
	if bannedAt != nil {
		item.BannedAt = *bannedAt
	}
	if reportMutedUntil != nil {
		item.ReportMutedUntil = *reportMutedUntil
	}
	items := []AdminPlayerSummary{item}
	if err := s.populateAdminPlayerIdentities(ctx, items); err != nil {
		return AdminPlayerSummary{}, err
	}
	return items[0], nil
}

func (s *pgStore) GetAdminPlayerDetail(userID string) (AdminPlayerDetail, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return AdminPlayerDetail{}, errors.New("userID required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	player, err := s.getAdminPlayerSummary(ctx, userID)
	if err != nil {
		return AdminPlayerDetail{}, err
	}
	var stats AdminPlayerStats
	var eloHistory []AdminPlayerEloPoint
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var queryErr error
		stats, queryErr = s.adminPlayerStats(groupCtx, userID)
		return queryErr
	})
	group.Go(func() error {
		var queryErr error
		eloHistory, queryErr = s.adminPlayerEloHistory(groupCtx, userID, 7)
		return queryErr
	})
	if err := group.Wait(); err != nil {
		return AdminPlayerDetail{}, err
	}
	return AdminPlayerDetail{
		Player:     player,
		Stats:      stats,
		EloHistory: eloHistory,
	}, nil
}

func (s *pgStore) adminPlayerStats(ctx context.Context, userID string) (AdminPlayerStats, error) {
	var stats AdminPlayerStats
	err := s.pool.QueryRow(ctx, `
		select
			count(*)::int,
			count(*) filter (where h.ranked)::int,
			count(*) filter (where h.mode = $2)::int,
			count(*) filter (where h.mode = 'singleplayer')::int,
			count(*) filter (where h.winner_user_id = $1)::int,
			count(*) filter (where h.mode = $2 and h.winner_user_id is not null and h.winner_user_id <> $1)::int
		from match_history h
		join match_players p on p.match_id = h.match_id
		where p.user_id = $1
	`, userID, modeDuel).Scan(
		&stats.TotalMatches,
		&stats.RankedMatches,
		&stats.DuelMatches,
		&stats.SingleplayerRuns,
		&stats.Wins,
		&stats.Losses,
	)
	return stats, err
}

func (s *pgStore) adminPlayerEloHistory(ctx context.Context, userID string, days int) ([]AdminPlayerEloPoint, error) {
	if days <= 0 {
		days = 7
	}
	since := time.Now().AddDate(0, 0, -days)
	rows, err := s.pool.Query(ctx, `
		with ranked_matches as (
			select
				h.ended_at,
				coalesce(p.rating_after, p.rating_before, p.mmr)::int as rating_after,
				coalesce(p.final_ranked_delta, 0)::int as delta
			from match_history h
			join match_players p on p.match_id = h.match_id
			where p.user_id = $1
			  and h.mode = $2
			  and h.ranked
			  and h.ended_at >= $3
		),
		latest_per_day as (
			select distinct on (date_trunc('day', ended_at))
				date_trunc('day', ended_at) as day,
				rating_after,
				sum(delta) over (partition by date_trunc('day', ended_at))::int as delta,
				count(*) over (partition by date_trunc('day', ended_at))::int as played,
				ended_at
			from ranked_matches
			order by date_trunc('day', ended_at), ended_at desc
		)
		select day, rating_after, delta, played
		from latest_per_day
		order by day asc
	`, userID, modeDuel, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminPlayerEloPoint{}
	for rows.Next() {
		var point AdminPlayerEloPoint
		if err := rows.Scan(&point.Date, &point.MMR, &point.Delta, &point.Played); err != nil {
			return nil, err
		}
		out = append(out, point)
	}
	return out, rows.Err()
}

func (s *pgStore) populateAdminPlayerIdentities(ctx context.Context, players []AdminPlayerSummary) error {
	if len(players) == 0 {
		return nil
	}
	userIDs := make([]string, 0, len(players))
	byUserID := make(map[string]int, len(players))
	for i := range players {
		userIDs = append(userIDs, players[i].UserID)
		byUserID[players[i].UserID] = i
	}
	rows, err := s.pool.Query(ctx, `
		select
			user_id,
			provider,
			provider_user_id,
			coalesce(email, ''),
			coalesce(provider_name, ''),
			last_seen_at,
			deleted_at
		from user_identity_history
		where user_id = any($1)
		order by user_id, provider, deleted_at nulls first, last_seen_at desc
	`, userIDs)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var userID string
		var identity contracts.AdminUserIdentity
		var deletedAt *time.Time
		if err := rows.Scan(
			&userID,
			&identity.Provider,
			&identity.ProviderUserID,
			&identity.Email,
			&identity.ProviderName,
			&identity.LastSeenAt,
			&deletedAt,
		); err != nil {
			return err
		}
		if deletedAt != nil {
			identity.DeletedAt = *deletedAt
		}
		if idx, ok := byUserID[userID]; ok {
			players[idx].Identities = append(players[idx].Identities, identity)
		}
	}
	return rows.Err()
}

func (s *pgStore) SetPlayerBan(userID, reason string, banned bool) error {
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
	var bannedAt any
	var banReason any
	if banned {
		bannedAt = time.Now()
		if strings.TrimSpace(reason) != "" {
			banReason = strings.TrimSpace(reason)
		}
	}
	tag, err := tx.Exec(ctx, `
		update users
		set banned_at = $2,
			ban_reason = $3,
			ban_expires_at = null
		where id = $1
	`, userID, bannedAt, banReason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	if banned {
		if err := banUserOAuthIdentities(ctx, tx, userID, strings.TrimSpace(reason), ""); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			insert into enforcement_actions(target_user_id, action_type, reason_code, reason_note)
			values($1, 'permanent_ban', 'manual', nullif($2, ''))
		`, userID, strings.TrimSpace(reason)); err != nil {
			return err
		}
	} else {
		if _, err := tx.Exec(ctx, `
			update oauth_identity_bans
			set revoked_at = coalesce(revoked_at, now())
			where banned_user_id = $1
			  and revoked_at is null
		`, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			insert into enforcement_actions(target_user_id, action_type, reason_code, reason_note)
			values($1, 'unban', 'manual', nullif($2, ''))
		`, userID, strings.TrimSpace(reason)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func banUserOAuthIdentities(ctx context.Context, tx pgx.Tx, userID, reason, actorUserID string) error {
	reason = strings.TrimSpace(reason)
	actorUserID = strings.TrimSpace(actorUserID)
	_, err := tx.Exec(ctx, `
		insert into oauth_identity_bans(provider, provider_user_id, banned_user_id, reason, created_by, created_at, revoked_at)
		select provider, provider_user_id, $1, nullif($2, ''), nullif($3, '')::uuid, now(), null
		from (
			select provider, provider_user_id
			from user_identity_history
			where user_id = $1
			union
			select provider, provider_user_id
			from user_identities
			where user_id = $1
		) identities
		on conflict (provider, provider_user_id) do update set
			banned_user_id = excluded.banned_user_id,
			reason = excluded.reason,
			created_by = excluded.created_by,
			created_at = now(),
			revoked_at = null
	`, userID, reason, actorUserID)
	return err
}

func (s *pgStore) ClearReporterMute(userID string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return errors.New("user id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tag, err := s.pool.Exec(ctx, `
		update moderation_reporter_state
		set muted_until = null,
			report_weight = greatest(report_weight, 0.05),
			updated_at = now()
		where user_id = $1
	`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		_, err = s.pool.Exec(ctx, `
			insert into moderation_reporter_state(user_id, muted_until, report_weight, updated_at)
			values($1, null, 1, now())
		`, userID)
	}
	return err
}
