drop table if exists map_comment_likes;

alter table map_comments
  drop constraint if exists map_comments_like_count_check,
  drop column if exists like_count;
