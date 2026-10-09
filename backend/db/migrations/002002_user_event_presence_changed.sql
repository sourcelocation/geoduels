-- +goose Up
-- Register the presence event type used by friend presence fan-out.
alter type public.gd_user_event_type add value if not exists 'presence.changed';

-- +goose Down
-- PostgreSQL can't remove an enum value; 'presence.changed' stays, unused.
