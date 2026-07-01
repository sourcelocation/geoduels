alter table maps
  add column if not exists thumbnail_key text not null default 'generic/variant-1';

update maps
set thumbnail_key = 'generic/variant-' || greatest(1, least(5, thumbnail_variant))::text
where thumbnail_key = ''
   or thumbnail_key ~ '^variant-[1-5]$';

alter table maps drop constraint if exists maps_thumbnail_key_check;
alter table maps add constraint maps_thumbnail_key_check
  check (thumbnail_key ~ '^[a-z0-9][a-z0-9-]*/[a-z0-9][a-z0-9-]*$');
