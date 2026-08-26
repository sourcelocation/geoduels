alter table maps
  add column if not exists difficulty text not null default 'normal',
  add column if not exists thumbnail_variant integer not null default 1,
  add column if not exists published_at timestamptz,
  add column if not exists play_count integer not null default 0,
  add column if not exists favorite_count integer not null default 0,
  add column if not exists comment_count integer not null default 0,
  add column if not exists trending_score double precision not null default 0,
  add column if not exists official_region_type text not null default '',
  add column if not exists official_region_code text not null default '';

alter table maps drop constraint if exists maps_visibility_check;
alter table maps add constraint maps_visibility_check
  check (visibility in ('private', 'unlisted', 'public'));

alter table maps drop constraint if exists maps_difficulty_check;
alter table maps add constraint maps_difficulty_check
  check (difficulty in ('easy', 'normal', 'hard'));

alter table maps drop constraint if exists maps_thumbnail_variant_check;
alter table maps add constraint maps_thumbnail_variant_check
  check (thumbnail_variant between 1 and 5);

create index if not exists idx_maps_public_trending
  on maps(trending_score desc, published_at desc)
  where archived_at is null and published_at is not null and status = 'ready';

create index if not exists idx_maps_public_popular
  on maps((play_count + favorite_count * 3) desc, published_at desc)
  where archived_at is null and published_at is not null and status = 'ready';

create index if not exists idx_maps_public_new
  on maps(published_at desc)
  where archived_at is null and published_at is not null and status = 'ready';

create table if not exists map_favorites (
  map_id text not null references maps(map_key) on delete cascade,
  user_id text not null references users(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (map_id, user_id)
);

create index if not exists idx_map_favorites_user_created
  on map_favorites(user_id, created_at desc);

create table if not exists map_comments (
  id text primary key,
  map_id text not null references maps(map_key) on delete cascade,
  parent_id text references map_comments(id) on delete cascade,
  user_id text not null references users(id) on delete cascade,
  body text not null,
  status text not null default 'visible',
  deleted_by text references users(id) on delete set null,
  deleted_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint map_comments_status_check check (status in ('visible', 'deleted', 'moderated')),
  constraint map_comments_body_check check (length(body) between 1 and 1000)
);

create index if not exists idx_map_comments_map_created
  on map_comments(map_id, created_at asc);

create index if not exists idx_map_comments_user_created
  on map_comments(user_id, created_at desc);

create table if not exists map_stats_daily (
  map_id text not null references maps(map_key) on delete cascade,
  day date not null,
  plays integer not null default 0,
  favorites integer not null default 0,
  comments integer not null default 0,
  unique_players integer not null default 0,
  unique_favoriters integer not null default 0,
  unique_commenters integer not null default 0,
  primary key (map_id, day)
);

create table if not exists map_daily_users (
  map_id text not null references maps(map_key) on delete cascade,
  day date not null,
  user_id text not null references users(id) on delete cascade,
  played boolean not null default false,
  favorited boolean not null default false,
  commented boolean not null default false,
  primary key (map_id, day, user_id)
);

create table if not exists map_revision_country_stats (
  map_revision_id text not null references map_revisions(id) on delete cascade,
  country text not null,
  location_count integer not null,
  primary key (map_revision_id, country)
);
