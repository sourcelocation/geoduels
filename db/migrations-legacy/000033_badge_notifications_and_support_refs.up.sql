create table if not exists support_donation_refs (
  ref text primary key,
  user_id text not null references users(id) on delete cascade,
  completed_at timestamptz,
  created_at timestamptz not null default now()
);

create index if not exists idx_support_donation_refs_user_created
on support_donation_refs(user_id, created_at desc);

insert into user_badges(user_id, badge_code)
select u.id, 6
from users u
join ranks r on r.user_id = u.id
where r.mode = 'duel'
  and r.season_id = coalesce(
    nullif((select value_json->>'activeSeasonId' from site_settings where key = 'ranked_season'), ''),
    's2'
  )
  and r.mmr >= 1000
  and coalesce(u.account_type, 'registered') <> 'guest'
on conflict (user_id, badge_code, badge_season_id) do nothing;

insert into user_badges(user_id, badge_code)
select u.id, 7
from users u
join ranks r on r.user_id = u.id
where r.mode = 'duel'
  and r.season_id = coalesce(
    nullif((select value_json->>'activeSeasonId' from site_settings where key = 'ranked_season'), ''),
    's2'
  )
  and r.mmr >= 1500
  and coalesce(u.account_type, 'registered') <> 'guest'
on conflict (user_id, badge_code, badge_season_id) do nothing;

insert into user_badges(user_id, badge_code)
select u.id, 8
from users u
join ranks r on r.user_id = u.id
where r.mode = 'duel'
  and r.season_id = coalesce(
    nullif((select value_json->>'activeSeasonId' from site_settings where key = 'ranked_season'), ''),
    's2'
  )
  and r.mmr >= 2000
  and coalesce(u.account_type, 'registered') <> 'guest'
on conflict (user_id, badge_code, badge_season_id) do nothing;
