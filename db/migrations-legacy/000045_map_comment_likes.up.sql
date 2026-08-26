alter table map_comments
  add column if not exists like_count integer not null default 0;

alter table map_comments drop constraint if exists map_comments_like_count_check;
alter table map_comments add constraint map_comments_like_count_check
  check (like_count >= 0);

create table if not exists map_comment_likes (
  comment_id text not null references map_comments(id) on delete cascade,
  user_id text not null references users(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (comment_id, user_id)
);

create index if not exists idx_map_comment_likes_user_created
  on map_comment_likes(user_id, created_at desc);
