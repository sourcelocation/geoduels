-- name: ActivePartyChatTeam :one
SELECT ms.match_id AS match_id, coalesce(mp.team_id, '') AS team_id FROM parties p JOIN match_sessions ms ON ms.match_id=coalesce(p.active_match_id,p.started_match_id) JOIN match_participants mp ON mp.match_id=ms.match_id AND mp.user_id=sqlc.arg(user_id) WHERE p.id=sqlc.arg(party_id) AND ms.mode='team_duel' AND ms.state='live';

-- name: ActiveRestriction :one
SELECT CASE WHEN banned_at IS NOT NULL AND (ban_expires_at IS NULL OR ban_expires_at > now()) THEN CASE WHEN ban_expires_at IS NULL THEN 'permanent_ban' ELSE 'temporary_ban' END ELSE 'chat_mute' END AS action_type, CASE WHEN banned_at IS NOT NULL AND (ban_expires_at IS NULL OR ban_expires_at > now()) THEN 'ban' ELSE 'chat_mute' END AS reason_code, CASE WHEN banned_at IS NOT NULL AND (ban_expires_at IS NULL OR ban_expires_at > now()) THEN coalesce(ban_reason,'') ELSE coalesce(chat_mute_reason,'') END AS reason_note, coalesce(CASE WHEN banned_at IS NOT NULL AND (ban_expires_at IS NULL OR ban_expires_at > now()) THEN ban_expires_at ELSE chat_mute_expires_at END, '0001-01-01 00:00:00+00'::timestamptz) AS ends_at FROM users WHERE id=$1 AND ((banned_at IS NOT NULL AND (ban_expires_at IS NULL OR ban_expires_at > now())) OR (chat_muted_at IS NOT NULL AND (chat_mute_expires_at IS NULL OR chat_mute_expires_at > now())));

-- name: ChatTeamForMatch :one
SELECT coalesce(team_id, '') FROM match_participants WHERE match_id=$1 AND user_id=$2;

-- name: EnsureConversation :exec
INSERT INTO chat_conversations (id, scope_kind, scope_id) VALUES (sqlc.arg(conversation_id), sqlc.arg(scope_kind)::gd_chat_scope, sqlc.arg(scope_id)::uuid) ON CONFLICT (id) DO NOTHING;

-- name: InsertMessage :exec
INSERT INTO chat_messages (id, conversation_id, team_match_id, sender_user_id, sender_display_name, kind, body, emote, audience, team_id, created_at)
VALUES (sqlc.arg(message_id), sqlc.arg(conversation_id), sqlc.narg(team_match_id), sqlc.arg(sender_user_id), sqlc.arg(sender_display_name), sqlc.arg(kind), sqlc.arg(body), sqlc.arg(emote), sqlc.arg(audience), sqlc.narg(team_id), sqlc.arg(created_at)) ON CONFLICT (id) DO NOTHING;

-- name: ListMessages :many
SELECT m.id, c.scope_kind || ':' || c.scope_id::text AS conversation_id, m.team_match_id AS match_id, m.sender_user_id, m.sender_display_name, m.kind AS kind, coalesce(m.body, '') AS body, coalesce(m.emote, '') AS emote, m.audience AS audience, m.team_id AS team_id, m.created_at
FROM chat_messages m JOIN chat_conversations c ON c.id=m.conversation_id WHERE m.conversation_id=sqlc.arg(conversation_id) AND (sqlc.arg(viewer_user_id)='' OR m.audience='all' OR EXISTS (SELECT 1 FROM match_participants mp WHERE mp.match_id=m.team_match_id AND mp.user_id=nullif(sqlc.arg(viewer_user_id),'')::uuid AND mp.team_id::text=m.team_id::text)) ORDER BY m.created_at ASC LIMIT sqlc.arg(row_limit);
