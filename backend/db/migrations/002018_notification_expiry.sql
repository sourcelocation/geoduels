-- +goose Up
-- A friend request or party invitation is announced only while it can still be
-- answered: its notification expires with it, or ended when it was answered.
UPDATE public.user_notifications n
SET expires_at = CASE WHEN r.status = 'pending' THEN r.expires_at ELSE coalesce(r.responded_at, now()) END
FROM public.friend_requests r
WHERE n.type = 'friend_request_received' AND n.expires_at IS NULL AND n.dedupe_key = 'friend_request:' || r.id::text;

UPDATE public.user_notifications n
SET expires_at = CASE WHEN i.status = 'pending' THEN i.expires_at ELSE coalesce(i.responded_at, now()) END
FROM public.party_invitations i
WHERE n.type = 'party_invitation_received' AND n.expires_at IS NULL AND n.dedupe_key = 'party_invitation:' || i.id::text;

-- What remains announces a request or invitation that no longer exists. Its row
-- was written in the same transaction as the notification, so none is in flight.
UPDATE public.user_notifications
SET expires_at = created_at
WHERE type IN ('friend_request_received', 'party_invitation_received') AND expires_at IS NULL;

-- +goose Down
-- Nothing set expires_at before this migration.
UPDATE public.user_notifications
SET expires_at = NULL
WHERE type IN ('friend_request_received', 'party_invitation_received');
