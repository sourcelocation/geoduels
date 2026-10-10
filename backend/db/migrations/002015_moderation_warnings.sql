-- +goose Up
ALTER TYPE public.gd_notification_type ADD VALUE IF NOT EXISTS 'moderation_warning';

CREATE TABLE moderation_warnings (
    id bigint PRIMARY KEY REFERENCES moderation_log(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category text NOT NULL CHECK (category IN ('chat_abuse', 'staff_impersonation', 'inappropriate_nickname', 'other')),
    previous_nickname text NOT NULL DEFAULT '',
    reset_nickname text NOT NULL DEFAULT '',
    evidence_message_id text NOT NULL DEFAULT '',
    acknowledged_at timestamptz,
    withdrawn_at timestamptz,
    withdrawn_by uuid REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX moderation_warnings_user ON moderation_warnings(user_id, id DESC);

-- +goose Down
-- PostgreSQL can't remove an enum value; 'moderation_warning' stays, unused.
DROP TABLE moderation_warnings;
