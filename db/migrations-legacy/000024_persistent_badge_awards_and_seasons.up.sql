create table if not exists user_badges (
  user_id text not null references users(id) on delete cascade,
  badge_id text not null,
  kind text not null,
  label text not null,
  description text not null default '',
  image_url text not null default '',
  season_id text,
  rank integer,
  awarded_at timestamptz not null default now(),
  primary key (user_id, badge_id)
);

create index if not exists idx_user_badges_user_awarded
on user_badges(user_id, awarded_at desc);

insert into user_badges(user_id, badge_id, kind, label, description, image_url)
select
  ui.user_id,
  'discord-member',
  'community',
  'Discord Member',
  'Awarded for linking Discord to your GeoDuels account.',
  '/badges/discord-badge.v1.png'
from user_identities ui
where ui.provider = 'discord'
on conflict (user_id, badge_id) do nothing;

insert into site_settings(key, value_json, updated_at)
values('ranked_season', jsonb_build_object('activeSeasonId', 's2'), now())
on conflict (key) do nothing;
