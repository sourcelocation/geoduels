insert into site_settings(key, value_json, updated_at)
values (
  'discord_integration',
  jsonb_build_object(
    'guildId', '',
    'joinsChannelId', '',
    'elo1000RoleId', '',
    'elo1500RoleId', '',
    'elo2000RoleId', '',
    'reconcileIntervalMinutes', 15
  ),
  now()
)
on conflict (key) do nothing;
