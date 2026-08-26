package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"geoduels/pkg/contracts"
)

func (s *pgStore) ListUserRoles() ([]UserRoleGrant, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `
		select u.id, coalesce(nullif(u.display_name, ''), u.id::text), coalesce(u.email, ''),
			case when u.is_admin then 'admin' else 'moderator' end,
			coalesce(last_grant.actor_user_id::text, ''), coalesce(last_grant.created_at, u.created_at),
			null::timestamptz, coalesce(last_grant.reason, '')
		from users u
		left join lateral (
			select actor_user_id, created_at, reason
			from moderation_log
			where subject_user_id = u.id and action = 'role_granted'
			order by created_at desc, id desc limit 1
		) last_grant on true
		where u.is_admin or u.is_moderator
		order by u.is_admin desc, coalesce(last_grant.created_at, u.created_at) desc
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
		insert into moderation_log(subject_user_id, actor_user_id, action, reason, metadata)
		values($1, nullif($2, '')::uuid, 'role_granted', nullif($3, ''), jsonb_build_object('role', $4::text))
	`, userID, strings.TrimSpace(grantedBy), strings.TrimSpace(reason), role); err != nil {
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
	tag, err := tx.Exec(ctx, `
		update users
		set is_admin = case when $2 = 'admin' then false else is_admin end,
			is_moderator = case when $2 = 'admin' then false when is_admin then true else false end
		where id = $1
	`, userID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	var hasTeamRole bool
	if err := tx.QueryRow(ctx, `select is_admin or is_moderator from users where id = $1`, userID).Scan(&hasTeamRole); err != nil {
		return err
	}
	if !hasTeamRole {
		if err := removeGeoDuelsTeamBadgeTx(ctx, tx, userID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		insert into moderation_log(subject_user_id, actor_user_id, action, reason, metadata)
		values($1, nullif($2, '')::uuid, 'role_revoked', nullif($3, ''), jsonb_build_object('role', $4::text))
	`, userID, strings.TrimSpace(revokedBy), strings.TrimSpace(reason), role); err != nil {
		return err
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
			u.ban_expires_at,
			u.chat_muted_at,
			coalesce(u.chat_mute_reason, ''),
			u.chat_mute_expires_at,
			u.report_muted_at,
			coalesce(u.report_mute_reason, ''),
			u.report_mute_expires_at,
			coalesce(latest_session.ip_address, '')
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
		var bannedAt, banExpiresAt, chatMutedAt, chatMutedUntil, reportMutedAt, reportMutedUntil *time.Time
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
			&banExpiresAt,
			&chatMutedAt,
			&item.ChatMuteReason,
			&chatMutedUntil,
			&reportMutedAt,
			&item.ReportMuteReason,
			&reportMutedUntil,
			&item.LastIPAddress,
		); err != nil {
			return nil, err
		}
		if bannedAt != nil {
			item.BannedAt = *bannedAt
		}
		if banExpiresAt != nil {
			item.BanExpiresAt = *banExpiresAt
		}
		if chatMutedAt != nil {
			item.ChatMutedAt = *chatMutedAt
		}
		if chatMutedUntil != nil {
			item.ChatMutedUntil = *chatMutedUntil
		}
		if reportMutedAt != nil {
			item.ReportMutedAt = *reportMutedAt
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
	var bannedAt, banExpiresAt, chatMutedAt, chatMutedUntil, reportMutedAt, reportMutedUntil *time.Time
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
			u.ban_expires_at,
			u.chat_muted_at,
			coalesce(u.chat_mute_reason, ''),
			u.chat_mute_expires_at,
			u.report_muted_at,
			coalesce(u.report_mute_reason, ''),
			u.report_mute_expires_at,
			coalesce(latest_session.ip_address, '')
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
		&banExpiresAt,
		&chatMutedAt,
		&item.ChatMuteReason,
		&chatMutedUntil,
		&reportMutedAt,
		&item.ReportMuteReason,
		&reportMutedUntil,
		&item.LastIPAddress,
	)
	if err != nil {
		return AdminPlayerSummary{}, err
	}
	if bannedAt != nil {
		item.BannedAt = *bannedAt
	}
	if banExpiresAt != nil {
		item.BanExpiresAt = *banExpiresAt
	}
	if chatMutedAt != nil {
		item.ChatMutedAt = *chatMutedAt
	}
	if chatMutedUntil != nil {
		item.ChatMutedUntil = *chatMutedUntil
	}
	if reportMutedAt != nil {
		item.ReportMutedAt = *reportMutedAt
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
	stats, err := s.adminPlayerStats(ctx, userID)
	if err != nil {
		return AdminPlayerDetail{}, err
	}
	applyAdminPlayerStats(&player, stats)
	return AdminPlayerDetail{Player: player}, nil
}

func applyAdminPlayerStats(player *AdminPlayerSummary, stats AdminPlayerStats) {
	player.TrackedMatches = stats.TotalMatches
	player.RankedMatches = stats.RankedMatches
	player.DuelMatches = stats.DuelMatches
	player.SingleplayerRuns = stats.SingleplayerRuns
	player.Losses = stats.Losses
	if stats.Wins > player.Wins {
		player.Wins = stats.Wins
	}
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

func (s *pgStore) SetPlayerBan(userID, reason, actorUserID string, banned bool) error {
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
	} else {
		if _, err := tx.Exec(ctx, `
			update oauth_identity_bans
			set revoked_at = coalesce(revoked_at, now())
			where banned_user_id = $1
			  and revoked_at is null
		`, userID); err != nil {
			return err
		}
	}
	action := "unban"
	if banned {
		action = "permanent_ban"
	}
	var logID int64
	if err := tx.QueryRow(ctx, `
		insert into moderation_log(subject_user_id, actor_user_id, action, reason)
		values($1, nullif($2, '')::uuid, $3, nullif($4, ''))
		returning id
	`, userID, strings.TrimSpace(actorUserID), action, strings.TrimSpace(reason)).Scan(&logID); err != nil {
		return err
	}
	if err := notifyAccountEnforcement(ctx, tx, userID, action, reason, logID, nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const communityPardonCandidatesSQL = `
	with latest_ban as (
		select distinct on (subject_user_id) subject_user_id, created_at
		from moderation_log
		where action in ('permanent_ban', 'temporary_ban')
		order by subject_user_id, created_at desc, id desc
	), latest_unban as (
		select distinct on (subject_user_id) subject_user_id, created_at
		from moderation_log
		where action = 'unban'
		order by subject_user_id, created_at desc, id desc
	)
	select u.id, coalesce(lb.created_at, u.banned_at) as sanction_started_at
	from users u
	left join latest_ban lb on lb.subject_user_id = u.id
	left join latest_unban lu on lu.subject_user_id = u.id
	where u.banned_at is not null
	  and (u.ban_expires_at is null or u.ban_expires_at > now())
	  and coalesce(lb.created_at, u.banned_at) < $1
	  and (lb.subject_user_id is null or lu.created_at is null or lu.created_at < lb.created_at)
`

func (s *pgStore) PreviewCommunityPardon(olderThan time.Duration) (CommunityPardonSummary, error) {
	if olderThan <= 0 {
		olderThan = 7 * 24 * time.Hour
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cutoff := time.Now().Add(-olderThan)
	var eligible int
	if err := s.pool.QueryRow(ctx, `select count(*) from (`+communityPardonCandidatesSQL+`) candidates`, cutoff).Scan(&eligible); err != nil {
		return CommunityPardonSummary{}, err
	}
	return CommunityPardonSummary{Eligible: eligible, Cutoff: cutoff}, nil
}

func (s *pgStore) PardonBannedPlayers(olderThan time.Duration, actorUserID string) (CommunityPardonSummary, error) {
	if olderThan <= 0 {
		olderThan = 7 * 24 * time.Hour
	}
	actorUserID = strings.TrimSpace(actorUserID)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CommunityPardonSummary{}, err
	}
	defer tx.Rollback(ctx)

	cutoff := time.Now().Add(-olderThan)
	rows, err := tx.Query(ctx, `
		with candidates as (`+communityPardonCandidatesSQL+`)
		update users u
		set banned_at = null, ban_reason = null, ban_expires_at = null
	from candidates c
	where u.id = c.id
	returning u.id
	`, cutoff)
	if err != nil {
		return CommunityPardonSummary{}, err
	}
	var userIDs []string
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			rows.Close()
			return CommunityPardonSummary{}, err
		}
		userIDs = append(userIDs, userID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return CommunityPardonSummary{}, err
	}
	rows.Close()

	for _, userID := range userIDs {
		if _, err := tx.Exec(ctx, `
			update oauth_identity_bans
			set revoked_at = coalesce(revoked_at, now())
			where banned_user_id = $1 and revoked_at is null
		`, userID); err != nil {
			return CommunityPardonSummary{}, err
		}
		var logID int64
		if err := tx.QueryRow(ctx, `
			insert into moderation_log(subject_user_id, actor_user_id, action, reason, metadata)
			values($1, nullif($2, '')::uuid, 'unban', 'v2 community pardon',
				jsonb_build_object('release', 'v2', 'policy', 'active ban older than 7 days'))
			returning id
		`, userID, actorUserID).Scan(&logID); err != nil {
			return CommunityPardonSummary{}, err
		}
		if err := notifyAccountEnforcement(ctx, tx, userID, "unban", "v2 community pardon", logID, nil); err != nil {
			return CommunityPardonSummary{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return CommunityPardonSummary{}, err
	}
	return CommunityPardonSummary{Eligible: len(userIDs), Pardoned: len(userIDs), Cutoff: cutoff}, nil
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
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
		update users set report_muted_at = null, report_mute_reason = null, report_mute_expires_at = null where id = $1
	`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	if _, err := tx.Exec(ctx, `insert into moderation_log(subject_user_id, action) values($1, 'report_unmute')`, userID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *pgStore) SetPlayerMute(userID, kind, reason, actorUserID string, until time.Time, muted bool) error {
	userID = strings.TrimSpace(userID)
	kind = strings.ToLower(strings.TrimSpace(kind))
	if userID == "" {
		return errors.New("user id required")
	}
	if kind != "chat" && kind != "report" {
		return errors.New("unsupported mute kind")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var tag pgconn.CommandTag
	if muted {
		if until.IsZero() {
			until = time.Now().Add(7 * 24 * time.Hour)
		}
		if kind == "chat" {
			tag, err = tx.Exec(ctx, `update users set chat_muted_at=now(), chat_mute_reason=nullif($2,''), chat_mute_expires_at=$3 where id=$1`, userID, strings.TrimSpace(reason), until)
		} else {
			tag, err = tx.Exec(ctx, `update users set report_muted_at=now(), report_mute_reason=nullif($2,''), report_mute_expires_at=$3 where id=$1`, userID, strings.TrimSpace(reason), until)
		}
	} else if kind == "chat" {
		tag, err = tx.Exec(ctx, `update users set chat_muted_at=null, chat_mute_reason=null, chat_mute_expires_at=null where id=$1`, userID)
	} else {
		tag, err = tx.Exec(ctx, `update users set report_muted_at=null, report_mute_reason=null, report_mute_expires_at=null where id=$1`, userID)
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("user not found")
	}
	action := kind + "_unmute"
	var expiresAt any
	if muted {
		action = kind + "_mute"
		expiresAt = until
	}
	if _, err := tx.Exec(ctx, `
		insert into moderation_log(subject_user_id, actor_user_id, action, reason, expires_at)
		values($1, nullif($2,'')::uuid, $3, nullif($4,''), $5)
	`, userID, strings.TrimSpace(actorUserID), action, strings.TrimSpace(reason), expiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
