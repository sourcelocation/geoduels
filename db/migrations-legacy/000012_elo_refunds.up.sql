create table if not exists elo_refunds (
  id bigserial primary key,
  user_id text not null references users(id) on delete cascade,
  match_id text not null references match_history(match_id) on delete cascade,
  cheater_user_id text not null references users(id) on delete cascade,
  original_delta integer not null,
  refund_delta integer not null,
  reason text not null default 'cheating_verdict',
  created_at timestamptz not null default now(),
  constraint elo_refunds_positive_refund check (refund_delta > 0),
  unique (user_id, match_id, cheater_user_id)
);

create index if not exists idx_elo_refunds_user_created
on elo_refunds(user_id, created_at desc);

create index if not exists idx_elo_refunds_cheater_created
on elo_refunds(cheater_user_id, created_at desc);
