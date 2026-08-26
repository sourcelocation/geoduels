-- OAuth account discovery compares normalized emails. Enforce the same rule on
-- canonical user emails so case or surrounding whitespace cannot create two
-- owners through another write path.
create unique index if not exists idx_users_normalized_email_unique
  on users (lower(btrim(email)))
  where email is not null and btrim(email) <> '';

create index if not exists idx_user_identities_normalized_email
  on user_identities (lower(btrim(email)), user_id)
  where email is not null and btrim(email) <> '';
