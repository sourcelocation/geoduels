alter table maps
  add column if not exists official_at timestamptz,
  add column if not exists official_by text references users(id) on delete set null;

insert into site_settings(key, value_json, updated_at)
values(
  'gameplay_map_settings',
  jsonb_build_object(
    'rankedMovingMapId', 'a-source-world',
    'rankedNmpzMapId', 'a-location-world',
    'singleplayerMovingMapId', 'a-source-world',
    'singleplayerNmpzMapId', 'a-location-world'
  ),
  now()
)
on conflict (key) do nothing;
