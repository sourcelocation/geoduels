alter table users
  add column if not exists nickname_claimed_at timestamptz;

with ranked_candidates as (
  select
    id,
    row_number() over (
      partition by lower(display_name)
      order by created_at asc, id asc
    ) as claim_order
  from users
  where account_type = 'registered'
    and display_name ~ '^[A-Za-z0-9._]{2,14}$'
    and position('..' in display_name) = 0
    and position('__' in display_name) = 0
)
update users u
set nickname_claimed_at = now()
from ranked_candidates candidate
where u.id = candidate.id
  and candidate.claim_order = 1;

create unique index if not exists users_claimed_nickname_unique
  on users (lower(display_name))
  where account_type = 'registered'
    and nickname_claimed_at is not null;

alter table users
  drop constraint if exists users_display_name_max_length_check;

alter table users
  add constraint users_claimed_nickname_format_check
  check (
    nickname_claimed_at is null
    or (
      account_type = 'registered'
      and display_name ~ '^[A-Za-z0-9._]{2,14}$'
      and position('..' in display_name) = 0
      and position('__' in display_name) = 0
    )
  );

alter table users
  drop column if exists onboarded_at;
