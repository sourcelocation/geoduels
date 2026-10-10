-- name: EnsureRankedStats :exec
insert into ranked_stats (user_id,mode,season_id,games_played,wins) values ($1,$2,$3,0,0) on conflict (user_id,mode,season_id) do nothing;

-- name: EnsureUserRank :exec
insert into ranks (user_id,mode,mmr,season_id) values ($1,$2,$4,$3) on conflict (user_id,mode,season_id) do nothing;

-- name: EnsureUserStats :exec
insert into user_stats (user_id,games_played,wins) values ($1,0,0) on conflict (user_id) do nothing;

-- name: GetBestWinStreak :one
with season_matches as (
  select h.ended_at,h.match_id,h.winner_user_id=sqlc.arg(user_id)::uuid won
  from match_players mp join match_history h on h.match_id=mp.match_id
  where mp.user_id=sqlc.arg(user_id)::uuid and h.mode='duel' and h.ranked
    and (sqlc.arg(since)::timestamptz is null or h.ended_at >= sqlc.arg(since)::timestamptz)
), streak_groups as (
  select won,count(*) filter(where not won) over(order by ended_at asc,match_id asc) streak_group
  from season_matches
), winning_streaks as (
  select count(*)::int streak from streak_groups where won group by streak_group
)
select coalesce(max(streak),0)::int from winning_streaks;

-- name: GetFlawlessWins :one
select count(*)::int from match_players mp join match_history h on h.match_id=mp.match_id where mp.user_id=$1 and h.mode='duel' and h.winner_user_id=$1 and mp.hp>=6000;

-- name: GetPerfectGuesses :one
select count(*)::int from ranked_guess_events where user_id=$1 and score=5000;

-- name: GetProfile :one
select coalesce(nullif(u.display_name, seed.user_id::text), ui.provider_name, sqlc.arg(user_id)::text) display_name, coalesce(u.avatar_url,ui.avatar_url,'') avatar_url, coalesce(r.mmr,sqlc.arg(default_mmr)::int) mmr, coalesce(r.rd,sqlc.arg(default_rd)::int) rating_rd, greatest(coalesce(us.games_played,0),coalesce(history_stats.games_played,0)) games_played, greatest(coalesce(us.wins,0),coalesce(history_stats.wins,0)) wins, coalesce(rs.games_played,0) ranked_games_played, coalesce(rs.wins,0) ranked_wins, (u.id is not null and gd_is_guest(u.id)) is_guest, gd_is_admin(u.id) is_admin, gd_is_judge(u.id) is_moderator, gd_is_banned(u.id) is_banned, coalesce(u.ban_reason,'') ban_reason, coalesce(u.selected_badge_code,0)::smallint selected_badge_code from (select sqlc.arg(user_id)::uuid user_id) seed left join users u on u.id=seed.user_id left join lateral (select provider_name,avatar_url from user_identities where user_id=seed.user_id and provider='google' order by created_at asc limit 1) ui on true left join ranks r on r.user_id=seed.user_id and r.mode=sqlc.arg(mode) and r.season_id=sqlc.arg(season_id) left join user_stats us on us.user_id=seed.user_id left join lateral (select count(*)::int games_played,count(*) filter(where h.winner_user_id=seed.user_id)::int wins from match_players mp join match_history h on h.match_id=mp.match_id where mp.user_id=seed.user_id and h.mode='duel') history_stats on true left join ranked_stats rs on rs.user_id=seed.user_id and rs.mode=sqlc.arg(mode) and rs.season_id=sqlc.arg(season_id);

-- name: GetPublicProfile :one
select u.id user_id, gd_display_name(u.display_name,ui.provider_name,u.id) display_name, coalesce(u.avatar_url,ui.avatar_url,'') avatar_url, coalesce(r.mmr,sqlc.arg(default_mmr)::int) mmr, coalesce(r.rd,sqlc.arg(default_rd)::int) rating_rd, greatest(coalesce(us.games_played,0),coalesce(history_stats.games_played,0))::int games_played, greatest(coalesce(us.wins,0),coalesce(history_stats.wins,0))::int wins, coalesce(rs.games_played,0)::int ranked_games_played, coalesce(rs.wins,0)::int ranked_wins, coalesce(u.selected_badge_code,0)::smallint selected_badge_code from users u left join lateral (select provider_name,avatar_url from user_identities where user_id=u.id and provider='google' order by created_at asc limit 1) ui on true left join ranks r on r.user_id=u.id and r.mode=sqlc.arg(mode) and r.season_id=sqlc.arg(season_id) left join user_stats us on us.user_id=u.id left join lateral (select count(*)::int games_played,count(*) filter(where h.winner_user_id=u.id)::int wins from match_players mp join match_history h on h.match_id=mp.match_id where mp.user_id=u.id and h.mode='duel') history_stats on true left join ranked_stats rs on rs.user_id=u.id and rs.mode=sqlc.arg(mode) and rs.season_id=sqlc.arg(season_id) where gd_is_registered(u.id) and u.nickname_claimed_at is not null and lower(u.display_name)=lower(sqlc.arg(display_name));

-- name: GetSocialAccount :one
select gd_is_registered(users.id) as has_identity, social_requests_enabled, social_party_invites_enabled from users where id=$1;

-- name: GetSocialSettings :one
select social_discoverable, social_presence_visible, social_requests_enabled, social_party_invites_enabled from users where id=$1;

-- name: ListUserBadges :many
select badge_code,coalesce(level,1)::smallint level,coalesce(extra,0)::smallint extra from user_badges where user_id=$1 order by awarded_at desc,badge_code asc;

-- name: TouchLastSeen :exec
update users set last_seen_at=greatest(coalesce(last_seen_at,$2),$2) where id=$1;

-- name: TouchPresence :exec
insert into presence (user_id, seen_at) values ($1, now())
on conflict (user_id) do update set seen_at = excluded.seen_at;

-- name: CountOnlineUsers :one
-- Players seen within the presence window.
select count(*) from presence where seen_at > now() - (sqlc.arg(window_seconds)::double precision * interval '1 second');

-- name: ListOnlineUsers :many
select user_id from presence
where user_id = any(sqlc.arg(user_ids)::uuid[])
  and seen_at > now() - (sqlc.arg(window_seconds)::double precision * interval '1 second');

-- name: UpdateSelectedBadge :exec
update users set selected_badge_code=nullif(sqlc.arg(badge_code),0) where id=sqlc.arg(user_id);

-- name: UpdateSocialSettings :one
update users set social_discoverable=$2, social_presence_visible=$3, social_requests_enabled=$4, social_party_invites_enabled=$5 where id=$1 and gd_is_registered(users.id) returning social_discoverable, social_presence_visible, social_requests_enabled, social_party_invites_enabled;

-- name: UpsertUser :exec
insert into users (id,email,display_name,avatar_url) values ($1,$2,$3,null) on conflict (id) do update set email=excluded.email, display_name=excluded.display_name;
