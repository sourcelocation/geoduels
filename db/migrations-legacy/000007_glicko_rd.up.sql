alter table ranks
  add column if not exists rd double precision not null default 350;

update ranks
set rd = 350
where rd is null;
