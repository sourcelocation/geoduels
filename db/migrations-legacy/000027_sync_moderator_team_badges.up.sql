insert into user_badges(user_id, badge_id, kind, label, description, image_url)
select
  id,
  'geoduels-team',
  'special',
  'GeoDuels Team',
  'An exclusive medal for GeoDuels moderators and team members.',
  '/badges/team-badge.v1.png'
from users
where coalesce(is_moderator, false) = true
on conflict (user_id, badge_id) do nothing;

update users
set selected_badge_id = null
where selected_badge_id = 'geoduels-team'
  and coalesce(is_moderator, false) = false;

delete from user_badges ub
using users u
where ub.user_id = u.id
  and ub.badge_id = 'geoduels-team'
  and coalesce(u.is_moderator, false) = false;
