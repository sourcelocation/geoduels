create table if not exists player_report_reviews (
  id bigserial primary key,
  reported_user_id text not null references users(id) on delete cascade,
  reviewed_by text references users(id) on delete set null,
  report_count integer not null default 0,
  created_at timestamptz not null default now()
);

alter table player_reports
  add column if not exists reviewed_at timestamptz,
  add column if not exists reviewed_by text references users(id) on delete set null;

alter table player_reports
  drop constraint if exists player_reports_reporter_user_id_reported_user_id_key;

create unique index if not exists idx_player_reports_open_unique
on player_reports(reporter_user_id, reported_user_id)
where reviewed_at is null;

create index if not exists idx_player_reports_open_reported
on player_reports(reported_user_id, created_at desc)
where reviewed_at is null;

create index if not exists idx_player_report_reviews_reported
on player_report_reviews(reported_user_id, created_at desc);
