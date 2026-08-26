alter table users
  add column if not exists selected_badge_code smallint,
  add column if not exists selected_badge_season_id text not null default '';

update users
set
  selected_badge_code = case
    when selected_badge_id = 'discord-member' then 1
    when selected_badge_id = 'geoduels-team' then 2
    when selected_badge_id like 'season-%-top-100' then 10
    else null
  end,
  selected_badge_season_id = case
    when selected_badge_id like 'season-%-top-100'
      then substring(selected_badge_id from '^season-(.*)-top-100$')
    else ''
  end
where selected_badge_id is not null
  and selected_badge_code is null;

alter table user_badges
  add column if not exists badge_code smallint,
  add column if not exists badge_season_id text not null default '';

update user_badges
set
  badge_code = case
    when badge_id = 'discord-member' then 1
    when badge_id = 'geoduels-team' then 2
    when badge_id like 'season-%-top-100' or kind = 'season_rank' then 10
    else null
  end,
  badge_season_id = case
    when coalesce(season_id, '') <> '' then season_id
    when badge_id like 'season-%-top-100' then substring(badge_id from '^season-(.*)-top-100$')
    else ''
  end
where badge_code is null;

delete from user_badges
where badge_code is null;

alter table user_badges
  alter column badge_code set not null;

update users u
set
  selected_badge_code = null,
  selected_badge_season_id = ''
where selected_badge_code is not null
  and not exists (
    select 1
    from user_badges ub
    where ub.user_id = u.id
      and ub.badge_code = u.selected_badge_code
      and ub.badge_season_id = u.selected_badge_season_id
  );

create unique index if not exists user_badges_user_badge_code_season_key
on user_badges(user_id, badge_code, badge_season_id);

alter table user_badges
  drop constraint if exists user_badges_pkey;

alter table user_badges
  add primary key using index user_badges_user_badge_code_season_key;

alter table users
  drop column if exists selected_badge_id;

alter table user_badges
  drop column if exists badge_id,
  drop column if exists kind,
  drop column if exists label,
  drop column if exists description,
  drop column if exists image_url,
  drop column if exists season_id;
