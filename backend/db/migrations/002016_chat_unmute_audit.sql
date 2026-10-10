-- +goose Up
-- Unmuting chat audits 'chat_unmute', which the enum never contained.
ALTER TYPE public.gd_moderation_log_action ADD VALUE IF NOT EXISTS 'chat_unmute';

-- +goose Down
-- PostgreSQL can't remove an enum value; 'chat_unmute' stays, unused.
