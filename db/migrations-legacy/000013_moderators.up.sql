alter table users
  add column if not exists is_moderator boolean not null default false;
