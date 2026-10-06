-- Unmuting chat audits 'chat_unmute', which the enum never contained.
ALTER TYPE public.gd_moderation_log_action ADD VALUE IF NOT EXISTS 'chat_unmute';
