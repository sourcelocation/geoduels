alter table lobbies
  drop constraint if exists lobbies_mode_check;

alter table lobbies
  add constraint lobbies_mode_check check (mode in ('duel', 'team_duel', 'free_for_all'));

alter table lobby_members
  add column if not exists team_id text;

alter table lobby_members
  drop constraint if exists lobby_members_team_id_check;

alter table lobby_members
  add constraint lobby_members_team_id_check check (team_id is null or team_id in ('a', 'b'));
