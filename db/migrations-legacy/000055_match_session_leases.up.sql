-- Durable ownership lease for live match cleanup.
alter table match_sessions
  add column if not exists lease_expires_at timestamptz;

update match_sessions
set lease_expires_at = now() + interval '30 minutes'
where state = 'live'
  and lease_expires_at is null;

create index if not exists idx_match_sessions_expired_lease
  on match_sessions(lease_expires_at, match_id)
  where state = 'live';
