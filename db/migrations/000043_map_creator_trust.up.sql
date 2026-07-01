alter table users
  add column if not exists map_creator_tier smallint not null default 0,
  add column if not exists map_creator_tier_override smallint,
  add column if not exists map_creator_qualified_favorites integer not null default 0,
  add column if not exists map_creator_qualified_maps integer not null default 0,
  add column if not exists map_creator_trust_updated_at timestamptz;

alter table users drop constraint if exists users_map_creator_tier_check;
alter table users add constraint users_map_creator_tier_check
  check (map_creator_tier between 0 and 2);

alter table users drop constraint if exists users_map_creator_tier_override_check;
alter table users add constraint users_map_creator_tier_override_check
  check (map_creator_tier_override is null or map_creator_tier_override between 0 and 2);

alter table map_upload_events
  add column if not exists location_count integer not null default 0;

alter table map_upload_events drop constraint if exists map_upload_events_location_count_check;
alter table map_upload_events add constraint map_upload_events_location_count_check
  check (location_count >= 0);

