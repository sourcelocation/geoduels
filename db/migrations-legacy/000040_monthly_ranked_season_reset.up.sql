insert into site_settings(key, value_json, updated_at)
values(
  'ranked_season',
  jsonb_build_object(
    'activeSeasonId', 's2',
    'monthlyResetDay', 1,
    'lastResetAt',
      case
        when now() >= ((date_trunc('month', now() at time zone 'UTC') + interval '21 hours') at time zone 'UTC')
          then to_jsonb((date_trunc('month', now() at time zone 'UTC') + interval '21 hours') at time zone 'UTC')
        else null
      end
  ),
  now()
)
on conflict (key) do update set
  value_json = jsonb_strip_nulls(
    jsonb_build_object(
      'activeSeasonId', coalesce(nullif(site_settings.value_json->>'activeSeasonId', ''), 's2'),
      'monthlyResetDay', coalesce((site_settings.value_json->>'monthlyResetDay')::int, 1),
      'lastResetAt',
        coalesce(
          site_settings.value_json->'lastResetAt',
          case
            when now() >= ((date_trunc('month', now() at time zone 'UTC') + interval '21 hours') at time zone 'UTC')
              then to_jsonb((date_trunc('month', now() at time zone 'UTC') + interval '21 hours') at time zone 'UTC')
            else null
          end
        )
    )
  ),
  updated_at = now();
