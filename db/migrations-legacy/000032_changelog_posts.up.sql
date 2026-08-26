create table if not exists changelog_posts (
  id bigserial primary key,
  slug text not null unique,
  title text not null,
  markdown text not null default '',
  published boolean not null default true,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create index if not exists changelog_posts_published_updated_idx
  on changelog_posts(published, updated_at desc);

insert into changelog_posts(slug, title, markdown, published)
select
  'geoduels-v1-1',
  coalesce(nullif(value_json->>'title', ''), 'GeoDuels v1.1'),
  coalesce(nullif(value_json->>'markdown', ''), ''),
  true
from site_settings
where key = 'lobby_changelog'
  and not exists (select 1 from changelog_posts);
