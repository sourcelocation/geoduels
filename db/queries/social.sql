-- name: AcceptFriendRequest :one
update friend_requests set status='accepted',responded_at=now() where id=$1 and recipient_user_id=$2 and status='pending' and expires_at>now() returning sender_user_id::text;

-- name: AddUserBlock :exec
insert into user_blocks(blocker_user_id,blocked_user_id) values($1,$2) on conflict do nothing;

-- name: CanSendFriendRequest :one
select exists(select 1 from users where id=$2 and account_type='registered' and social_requests_enabled) and not exists(select 1 from user_blocks where (blocker_user_id=$1 and blocked_user_id=$2) or (blocker_user_id=$2 and blocked_user_id=$1));

-- name: CancelFriendRequest :execrows
update friend_requests set status='cancelled',responded_at=now() where id=$1 and sender_user_id=$2 and status='pending';

-- name: CancelPairFriendRequests :exec
update friend_requests set status='cancelled',responded_at=now() where status='pending' and least(sender_user_id,recipient_user_id)=least(sqlc.arg(user_a)::uuid,sqlc.arg(user_b)::uuid) and greatest(sender_user_id,recipient_user_id)=greatest(sqlc.arg(user_a)::uuid,sqlc.arg(user_b)::uuid);

-- name: CountFriends :one
select count(*)::int from friendships where user_id_low=$1 or user_id_high=$1;

-- name: DeclineFriendRequest :execrows
update friend_requests set status='declined',responded_at=now() where id=$1 and recipient_user_id=$2 and status='pending';

-- name: FindCrossedFriendRequest :one
select id::text from friend_requests where sender_user_id=$2 and recipient_user_id=$1 and status='pending' and expires_at>now() for update;

-- name: FriendRequestSender :one
select sender_user_id::text from friend_requests where id=$1 and recipient_user_id=$2;

-- name: InsertFriendCode :exec
insert into friend_codes(code,user_id,expires_at) values($1,$2,$3);

-- name: InsertFriendship :exec
insert into friendships(user_id_low,user_id_high,created_from_request_id) values(least(sqlc.arg(user_a)::uuid,sqlc.arg(user_b)::uuid),greatest(sqlc.arg(user_a)::uuid,sqlc.arg(user_b)::uuid),sqlc.arg(created_from_request_id)) on conflict do nothing;

-- name: InsertUserEvent :exec
insert into user_events(user_id,sequence,type,payload_json) values(sqlc.arg(user_id),sqlc.arg(event_sequence),sqlc.arg(event_type),sqlc.arg(payload_json)::jsonb);

-- name: ListFriends :many
with friend_ids as (select case when user_id_low=$1 then user_id_high else user_id_low end user_id,created_at from friendships where user_id_low=$1 or user_id_high=$1)
select u.id::text user_id,coalesce(nullif(u.display_name,''),u.id::text)::text display_name,coalesce(u.avatar_url,'') avatar_url,coalesce(r.mmr,1000)::int mmr,(case when u.social_presence_visible then u.last_seen_at end)::timestamptz last_seen_at from friend_ids f join users u on u.id=f.user_id left join ranks r on r.user_id=u.id and r.mode='duel' where u.account_type='registered' order by f.created_at desc limit $2;

-- name: ListIncomingFriendRequests :many
select fr.id::text request_id,u.id::text user_id,coalesce(nullif(u.display_name,''),u.id::text) display_name,coalesce(u.avatar_url,'') avatar_url,coalesce(r.mmr,1000)::int mmr,u.last_seen_at,fr.created_at,fr.expires_at from friend_requests fr join users u on u.id=fr.sender_user_id left join ranks r on r.user_id=u.id and r.mode='duel' where fr.recipient_user_id=$1 and fr.status='pending' and fr.expires_at>now() order by fr.created_at desc limit $2;

-- name: ListOutgoingFriendRequests :many
select fr.id::text request_id,u.id::text user_id,coalesce(nullif(u.display_name,''),u.id::text) display_name,coalesce(u.avatar_url,'') avatar_url,coalesce(r.mmr,1000)::int mmr,u.last_seen_at,fr.created_at,fr.expires_at from friend_requests fr join users u on u.id=fr.recipient_user_id left join ranks r on r.user_id=u.id and r.mode='duel' where fr.sender_user_id=$1 and fr.status='pending' and fr.expires_at>now() order by fr.created_at desc limit $2;

-- name: ListPartyInvitations :many
select pi.id::text invitation_id,p.id::text party_id,p.invite_code,p.mode,count(pm.user_id)::int member_count,u.id::text inviter_id,u.display_name,coalesce(u.avatar_url,'') avatar_url,pi.created_at,pi.expires_at from party_invitations pi join parties p on p.id=pi.party_id join users u on u.id=pi.inviter_user_id left join party_members pm on pm.party_id=p.id and pm.left_at is null where pi.recipient_user_id=$1 and pi.status='pending' and pi.expires_at>now() and p.state='open' and p.expires_at>now() group by pi.id,p.id,u.id order by pi.created_at desc limit $2;

-- name: ListRecentPlayers :many
with recent as (select mp2.user_id,max(h.ended_at) shared_at from match_players mine join match_history h on h.match_id=mine.match_id join match_players mp2 on mp2.match_id=h.match_id and mp2.user_id<>sqlc.arg(self_user_id) where mine.user_id=sqlc.arg(self_user_id) and h.ended_at is not null group by mp2.user_id)
select u.id::text user_id,u.display_name,coalesce(u.avatar_url,'') avatar_url,coalesce(r.mmr,1000)::int mmr,u.last_seen_at,recent.shared_at::timestamptz shared_at from recent join users u on u.id=recent.user_id left join ranks r on r.user_id=u.id and r.mode='duel' where u.account_type='registered' and u.nickname_claimed_at is not null and u.social_discoverable and not exists(select 1 from friendships f where f.user_id_low=least(sqlc.arg(self_user_id)::uuid,u.id) and f.user_id_high=greatest(sqlc.arg(self_user_id)::uuid,u.id)) and not exists(select 1 from friend_requests fr where fr.status='pending' and least(fr.sender_user_id,fr.recipient_user_id)=least(sqlc.arg(self_user_id)::uuid,u.id) and greatest(fr.sender_user_id,fr.recipient_user_id)=greatest(sqlc.arg(self_user_id)::uuid,u.id)) and not exists(select 1 from user_blocks b where (b.blocker_user_id=sqlc.arg(self_user_id) and b.blocked_user_id=u.id) or (b.blocker_user_id=u.id and b.blocked_user_id=sqlc.arg(self_user_id))) order by recent.shared_at desc limit sqlc.arg(row_limit);

-- name: ListUserEvents :many
select sequence,type,payload_json::text payload,created_at from user_events where user_id=$1 and sequence>$2 order by sequence limit $3;

-- name: MarkFriendRequestNotificationRead :exec
update user_notifications set read_at=coalesce(read_at,now()) where user_id=$1 and dedupe_key='friend_request:'||sqlc.arg(request_id);

-- name: MarkPartyInvitationNotificationRead :exec
update user_notifications set read_at=coalesce(read_at,now()) where user_id=$1 and dedupe_key='party_invitation:'||sqlc.arg(invitation_id);

-- name: NextUserEventSequence :one
insert into user_event_sequences(user_id,sequence) values($1,1) on conflict(user_id) do update set sequence=user_event_sequences.sequence+1 returning sequence;

-- name: PartyInvitationEligibility :one
select p.invite_code,p.mode,count(pm.user_id)::int member_count from parties p join party_members self on self.party_id=p.id and self.user_id=sqlc.arg(inviter_user_id) and self.left_at is null left join party_members pm on pm.party_id=p.id and pm.left_at is null where p.id=sqlc.arg(party_id) and p.state='open' and p.expires_at>now() and exists(select 1 from friendships f where f.user_id_low=least(sqlc.arg(inviter_user_id)::uuid,sqlc.arg(recipient_user_id)::uuid) and f.user_id_high=greatest(sqlc.arg(inviter_user_id)::uuid,sqlc.arg(recipient_user_id)::uuid)) and exists(select 1 from users u where u.id=sqlc.arg(recipient_user_id) and u.social_party_invites_enabled) and not exists(select 1 from user_blocks b where (b.blocker_user_id=sqlc.arg(inviter_user_id) and b.blocked_user_id=sqlc.arg(recipient_user_id)) or (b.blocker_user_id=sqlc.arg(recipient_user_id) and b.blocked_user_id=sqlc.arg(inviter_user_id))) group by p.id;

-- name: Relationship :one
select exists(select 1 from user_blocks b1 where b1.blocker_user_id=$1 and b1.blocked_user_id=$2) blocked_by_viewer,
exists(select 1 from user_blocks b2 where b2.blocker_user_id=$2 and b2.blocked_user_id=$1) blocked_by_target,
exists(select 1 from friendships f where f.user_id_low=least($1::uuid,$2::uuid) and f.user_id_high=greatest($1::uuid,$2::uuid)) friends,
coalesce((select fr1.id::text from friend_requests fr1 where fr1.status='pending' and least(fr1.sender_user_id,fr1.recipient_user_id)=least($1::uuid,$2::uuid) and greatest(fr1.sender_user_id,fr1.recipient_user_id)=greatest($1::uuid,$2::uuid) limit 1),'')::text request_id,
coalesce((select fr2.sender_user_id::text from friend_requests fr2 where fr2.status='pending' and least(fr2.sender_user_id,fr2.recipient_user_id)=least($1::uuid,$2::uuid) and greatest(fr2.sender_user_id,fr2.recipient_user_id)=greatest($1::uuid,$2::uuid) limit 1),'')::text sender_id;

-- name: RemoveFriend :exec
delete from friendships where user_id_low=least(sqlc.arg(user_a)::uuid,sqlc.arg(user_b)::uuid) and user_id_high=greatest(sqlc.arg(user_a)::uuid,sqlc.arg(user_b)::uuid);

-- name: RemoveUserBlock :exec
delete from user_blocks where blocker_user_id=$1 and blocked_user_id=$2;

-- name: ResolveFriendCode :one
select u.id::text user_id,u.display_name,coalesce(u.avatar_url,'') avatar_url,coalesce(r.mmr,1000)::int mmr,u.last_seen_at from friend_codes fc join users u on u.id=fc.user_id left join ranks r on r.user_id=u.id and r.mode='duel' where fc.code=$1 and fc.revoked_at is null and fc.expires_at>now() and u.id<>$2 and u.account_type='registered' and u.social_requests_enabled and not exists(select 1 from user_blocks b where (b.blocker_user_id=$2 and b.blocked_user_id=u.id) or (b.blocker_user_id=u.id and b.blocked_user_id=$2));

-- name: RespondPartyInvitation :one
update party_invitations pi set status=$3,responded_at=now() from parties p where pi.id=$1 and pi.recipient_user_id=$2 and pi.status='pending' and pi.expires_at>now() and p.id=pi.party_id and p.state='open' and p.expires_at>now() returning pi.id::text,p.id::text,p.invite_code,p.mode,pi.expires_at;

-- name: RevokeFriendCodes :exec
update friend_codes set revoked_at=now() where user_id=$1 and revoked_at is null;

-- name: SearchSocialPlayers :many
select u.id::text user_id,u.display_name,coalesce(u.avatar_url,'') avatar_url,coalesce(r.mmr,1000)::int mmr,u.last_seen_at from users u left join ranks r on r.user_id=u.id and r.mode='duel' where u.id<>$1 and u.account_type='registered' and u.nickname_claimed_at is not null and u.social_discoverable and lower(u.display_name) like lower($2)||'%' and not exists(select 1 from user_blocks b where (b.blocker_user_id=$1 and b.blocked_user_id=u.id) or (b.blocker_user_id=u.id and b.blocked_user_id=$1)) order by (lower(u.display_name)=lower($2)) desc,length(u.display_name),lower(u.display_name) limit $3;

-- name: UpsertFriendRequest :one
insert into friend_requests(id,sender_user_id,recipient_user_id,expires_at) values($1,$2,$3,$4) on conflict (least(sender_user_id,recipient_user_id),greatest(sender_user_id,recipient_user_id)) where status='pending' do update set expires_at=greatest(friend_requests.expires_at,excluded.expires_at) returning id::text,created_at,expires_at;

-- name: UpsertPartyInvitation :one
insert into party_invitations(id,party_id,inviter_user_id,recipient_user_id,expires_at) values($1,$2,$3,$4,$5) on conflict(party_id,recipient_user_id) where status='pending' do update set inviter_user_id=excluded.inviter_user_id,expires_at=excluded.expires_at returning id::text,created_at,expires_at;
