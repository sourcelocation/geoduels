update users
set display_name = left(display_name, 14)
where char_length(display_name) > 14;

alter table users
  drop constraint if exists users_display_name_max_length_check;

alter table users
  add constraint users_display_name_max_length_check
  check (onboarded_at is null or char_length(display_name) <= 14);
