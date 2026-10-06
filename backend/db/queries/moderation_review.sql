-- name: AdminPlayerIdentities :many
select user_id, provider, provider_user_id, coalesce(email, ''), coalesce(provider_name, ''), last_seen_at, deleted_at
from user_identity_history where user_id = any($1::uuid[])
order by user_id, provider, deleted_at nulls first, last_seen_at desc;

-- name: AdminPlayerStats :one
select count(*)::int as total_matches,
       count(*) filter (where h.ranked)::int as ranked_matches,
       count(*) filter (where h.mode = $2)::int as duel_matches,
       count(*) filter (where h.mode = 'singleplayer')::int as singleplayer_runs,
       count(*) filter (where h.winner_user_id = $1)::int as wins,
       count(*) filter (where h.mode = $2 and h.winner_user_id is not null and h.winner_user_id <> $1)::int as losses
from match_history h join match_players p on p.match_id = h.match_id where p.user_id = $1;

-- name: SearchAdminPlayers :many
select
    u.id as user_id,
    case when sqlc.arg(include_sensitive)::boolean then coalesce(u.email, '') else '' end as email,
    gd_display_name(u.display_name,ui.provider_name,u.id) as display_name,
    coalesce(u.avatar_url, ui.avatar_url, '') as avatar_url,
    coalesce(r.mmr, sqlc.arg(default_mmr)::int) as mmr,
    coalesce(us.games_played, 0) as games_played,
    coalesce(us.wins, 0) as wins,
    coalesce(rs.games_played, 0) as ranked_games_played,
    gd_is_guest(u.id) as is_guest,
    gd_is_admin(u.id) as is_admin,
    gd_is_judge(u.id) as is_moderator,
    gd_is_banned(u.id) as is_banned,
    coalesce(u.ban_reason, '') as ban_reason,
    u.banned_at,
    u.ban_expires_at,
    u.chat_muted_at,
    coalesce(u.chat_mute_reason, '') as chat_mute_reason,
    u.chat_mute_expires_at,
    u.report_muted_at,
    coalesce(u.report_mute_reason, '') as report_mute_reason,
    u.report_mute_expires_at,
    case when sqlc.arg(include_sensitive)::boolean then coalesce(latest_session.ip_address, '') else '' end as last_ip_address
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
left join ranks r on r.user_id = u.id and r.mode = sqlc.arg(mode) and r.season_id = sqlc.arg(season_id)
left join user_stats us on us.user_id = u.id
left join ranked_stats rs on rs.user_id = u.id and rs.mode = sqlc.arg(mode) and rs.season_id = sqlc.arg(season_id)
where (sqlc.arg(search) = '%%'
       or lower(u.id::text) like sqlc.arg(search)
       or lower(coalesce(u.email, '')) like sqlc.arg(search)
       or lower(coalesce(u.display_name, ui.provider_name, '')) like sqlc.arg(search)
       or exists (
        select 1
        from user_identity_history ih
        where ih.user_id = u.id
          and (
            lower(ih.provider::text) like sqlc.arg(search)
            or lower(ih.provider_user_id) like sqlc.arg(search)
            or lower(coalesce(ih.email, '')) like sqlc.arg(search)
            or lower(coalesce(ih.provider_name, '')) like sqlc.arg(search)
          )
       ))
  and (sqlc.arg(creator_id)::uuid is null or u.id = sqlc.arg(creator_id)::uuid)
order by u.created_at desc, u.id desc
limit sqlc.arg(row_limit);
