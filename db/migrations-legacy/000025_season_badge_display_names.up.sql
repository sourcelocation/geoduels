update user_badges
set
  label = 'Season 1 #' || rank::text,
  description = 'Finished #' || rank::text || ' in Season 1.'
where kind = 'season_rank'
  and season_id = 's2'
  and rank is not null;

update user_badges
set
  label = 'Season 2 #' || rank::text,
  description = 'Finished #' || rank::text || ' in Season 2.'
where kind = 'season_rank'
  and season_id = 's2.5'
  and rank is not null;
