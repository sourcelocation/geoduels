-- name: ClearUserEmailWithoutGoogle :exec
update users set email=null where id=$1 and not exists(select 1 from user_identities where user_id=$1 and provider='google');

-- name: CountUserProviders :one
select count(*)::int from user_identities where user_id=$1;

-- name: DeleteUserProvider :execrows
delete from user_identities where user_id=$1 and provider=$2;

-- name: EnqueueDiscordSync :exec
insert into discord_sync_outbox(action, discord_user_id) values($1,$2)
on conflict (action, discord_user_id) where processed_at is null do update set next_attempt_at=least(discord_sync_outbox.next_attempt_at, excluded.next_attempt_at), last_error=null;

-- name: EnsureAccountRank :exec
insert into ranks(user_id,mode,mmr,season_id) values($1,$2,$4,$3) on conflict(user_id,mode,season_id) do nothing;

-- name: EnsureAccountRankedStats :exec
insert into ranked_stats(user_id,mode,season_id,games_played,wins) values($1,$2,$3,0,0) on conflict(user_id,mode,season_id) do nothing;

-- name: EnsureAccountStats :exec
insert into user_stats(user_id,games_played,wins) values($1,0,0) on conflict(user_id) do nothing;

-- name: FindProviderIdentityUser :one
select user_id from user_identities where provider=$1 and provider_user_id=$2;

-- name: FindProviderUserIdentity :one
select provider_user_id from user_identities where user_id=$1 and provider=$2;

-- name: FindUserByVerifiedEmail :many
select u.id, u.account_type from users u
where u.deleted_at is null and (lower(btrim(u.email)) = lower(btrim($1)) or exists (select 1 from user_identities ui where ui.user_id=u.id and lower(btrim(ui.email))=lower(btrim($1))))
order by u.created_at, u.id limit 2;

-- name: GetUserAccountType :one
select account_type from users where id=$1;

-- name: ListDiscordIdentities :many
select provider_user_id from user_identities where user_id=$1 and provider=$2;

-- name: LockOAuthEmail :exec
select pg_advisory_xact_lock(hashtextextended(lower(btrim($1)), 0));

-- name: MarkIdentityHistoryDeleted :exec
update user_identity_history set deleted_at=coalesce(deleted_at,now()) where user_id=$1 and provider=$2 and deleted_at is null;

-- name: PromoteLinkedUser :exec
insert into users(id,email,display_name,avatar_url,account_type) values($1,$2,$3,$4,'registered') on conflict(id) do update set email=coalesce(excluded.email,users.email),display_name=case when users.account_type='guest' then excluded.display_name when users.nickname_claimed_at is not null and nullif(users.display_name,'') is not null then users.display_name else excluded.display_name end,avatar_url=excluded.avatar_url,account_type='registered';

-- name: RecordUserIdentityHistory :exec
insert into user_identity_history(user_id, provider, provider_user_id, email, provider_name, first_seen_at, last_seen_at, deleted_at)
values($1,$2,$3,$4,$5,now(),now(),null)
on conflict (user_id, provider, provider_user_id) do update set email=excluded.email, provider_name=excluded.provider_name, last_seen_at=now(), deleted_at=null;

-- name: UpsertIdentityByProviderSubject :exec
insert into user_identities(user_id,provider,provider_user_id,email,provider_name,avatar_url,last_seen_at) values($1,$2,$3,$4,$5,$6,now()) on conflict(provider,provider_user_id) do update set user_id=excluded.user_id,email=excluded.email,provider_name=excluded.provider_name,avatar_url=case when excluded.avatar_url is null or excluded.avatar_url='' then user_identities.avatar_url else excluded.avatar_url end,last_seen_at=now();

-- name: UpsertIdentityByUserProvider :exec
insert into user_identities(user_id,provider,provider_user_id,email,provider_name,avatar_url,last_seen_at) values($1,$2,$3,$4,$5,$6,now()) on conflict(user_id,provider) do update set provider_user_id=excluded.provider_user_id,email=excluded.email,provider_name=excluded.provider_name,avatar_url=case when excluded.avatar_url is null or excluded.avatar_url='' then user_identities.avatar_url else excluded.avatar_url end,last_seen_at=now();

-- name: UpsertRegisteredUser :exec
insert into users(id,email,display_name,avatar_url,account_type) values($1,$2,$3,$4,'registered') on conflict(id) do update set email=case when excluded.email is null then users.email when exists(select 1 from users email_owner where email_owner.id<>users.id and email_owner.deleted_at is null and lower(btrim(email_owner.email))=lower(btrim(excluded.email)) union all select 1 from user_identities identity_owner join users identity_user on identity_user.id=identity_owner.user_id where identity_owner.user_id<>users.id and identity_user.deleted_at is null and lower(btrim(identity_owner.email))=lower(btrim(excluded.email))) then users.email else excluded.email end,display_name=case when users.account_type='guest' then excluded.display_name when users.nickname_claimed_at is not null and nullif(users.display_name,'') is not null then users.display_name else excluded.display_name end,avatar_url=excluded.avatar_url,account_type='registered';
