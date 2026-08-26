-- Bring a legacy version-63 installation to the shared GeoDuels v2 schema at
-- version 2000. Fresh installations reach 2000 through db/migrations instead.

-- Admin badge grants are recorded in moderation_log. Keep this enum extension
-- in the same forward migration as the grant implementation so legacy
-- databases accept the audit row before the endpoint is enabled.
alter type gd_moderation_log_action add value if not exists 'badge_granted';

do $migration$
begin
  if exists (
    select 1
    from information_schema.columns
    where table_schema = 'public'
      and table_name = 'user_badges'
      and column_name = 'badge_season_id'
  ) then
    alter table users drop column if exists selected_badge_season_id;

    alter table user_badges
      add column level smallint not null default 1,
      add column extra smallint,
      add column updated_at timestamptz not null default now();

    create temporary table legacy_top_finish_badges on commit drop as
    with historical as (
      select ub.user_id, min(coalesce(ub.rank, 100))::smallint as best_rank
      from user_badges ub
      where ub.badge_code = 10
      group by ub.user_id
      union all
      select ranked.user_id, min(ranked.rank)::smallint
      from (
        select r.user_id,
          row_number() over (
            partition by r.season_id
            order by r.mmr desc, r.updated_at asc, r.user_id asc
          )::int as rank
        from ranks r
        join users u on u.id = r.user_id
        where r.mode = 'duel'
          and r.season_id <> coalesce(
            (select value_json->>'activeSeasonId' from site_settings where key = 'ranked_season'),
            's2'
          )
          and coalesce(u.account_type, 'registered') <> 'guest'
          and not coalesce(
            u.banned_at is not null
              and (u.ban_expires_at is null or u.ban_expires_at > now()),
            false
          )
      ) ranked
      where ranked.rank between 1 and 100
      group by ranked.user_id
    )
    select user_id, min(best_rank)::smallint as best_rank
    from historical
    group by user_id;

    delete from user_badges where badge_code = 10;
    insert into user_badges(user_id, badge_code, level, extra, awarded_at, updated_at)
    select user_id, 10, 1, best_rank, now(), now()
    from legacy_top_finish_badges;

    alter table user_badges
      drop constraint if exists user_badges_user_badge_code_season_key,
      drop constraint if exists user_badges_pkey,
      drop column badge_season_id,
      drop column rank,
      add constraint user_badges_pkey primary key (user_id, badge_code),
      add constraint user_badges_level_check check (level >= 1),
      add constraint user_badges_extra_check check (extra is null or extra >= 0);

    update users u
    set selected_badge_code = null
    where selected_badge_code is not null
      and not exists (
        select 1 from user_badges ub
        where ub.user_id = u.id and ub.badge_code = u.selected_badge_code
      );
  end if;
end
$migration$;

create index if not exists idx_user_badges_user_updated
  on user_badges(user_id, updated_at desc);

-- Match navigation intent is durable match metadata, independent of the map
-- selected in config_json. Party routes retain the stable party ID and resolve
-- the current invite code per viewer at read time.
alter table match_sessions
  add column if not exists return_target_kind text not null default 'home',
  add column if not exists return_target_map_id uuid,
  add column if not exists return_target_party_id uuid;

alter table match_sessions
  add constraint match_sessions_return_target_kind_check
    check (return_target_kind in ('home', 'map', 'party')),
  add constraint match_sessions_return_target_map_fk
    foreign key (return_target_map_id) references maps(id) on delete set null,
  add constraint match_sessions_return_target_party_fk
    foreign key (return_target_party_id) references parties(id) on delete set null;

create index if not exists idx_match_sessions_return_target_party
  on match_sessions(return_target_party_id)
  where return_target_party_id is not null;

-- Migrate gameplay map settings from ranked/singleplayer slots to one map per
-- ruleset. Fresh installations receive this final shape directly from the v2
-- schema migration; legacy installations are normalized during the cutover.
update site_settings
set value_json = jsonb_strip_nulls(jsonb_build_object(
        'movingMapId', coalesce(
          nullif(value_json->>'movingMapId', ''),
          nullif(value_json->>'rankedMovingMapId', ''),
          nullif(value_json->>'singleplayerMovingMapId', ''),
          'a-source-world'
        ),
        'noMoveMapId', coalesce(
          nullif(value_json->>'noMoveMapId', ''),
          nullif(value_json->>'rankedNmpzMapId', ''),
          'a-location-world'
        ),
        'nmpzMapId', coalesce(
          nullif(value_json->>'nmpzMapId', ''),
          nullif(value_json->>'singleplayerNmpzMapId', ''),
          nullif(value_json->>'rankedNmpzMapId', ''),
          'a-location-world'
        )
      )),
    updated_at = now()
where key = 'gameplay_map_settings';
