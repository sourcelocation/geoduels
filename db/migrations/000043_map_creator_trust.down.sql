alter table map_upload_events
  drop constraint if exists map_upload_events_location_count_check,
  drop column if exists location_count;

alter table users
  drop constraint if exists users_map_creator_tier_override_check,
  drop constraint if exists users_map_creator_tier_check,
  drop column if exists map_creator_trust_updated_at,
  drop column if exists map_creator_qualified_maps,
  drop column if exists map_creator_qualified_favorites,
  drop column if exists map_creator_tier_override,
  drop column if exists map_creator_tier;

