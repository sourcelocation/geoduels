-- UpsertPartyInvitation uses ON CONFLICT (party_id, recipient_user_id) WHERE status='pending'.
-- The original active index was not unique, so every invite failed at planning time.
drop index if exists public.idx_party_invitations_active;
create unique index idx_party_invitations_active on public.party_invitations using btree (party_id, recipient_user_id) where (status = 'pending'::public.gd_social_request_status);
