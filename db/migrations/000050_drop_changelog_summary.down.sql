alter table changelog_posts
  add column if not exists summary text not null default '';
