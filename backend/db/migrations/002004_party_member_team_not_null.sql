-- +goose Up
-- party_members.team_id is assigned at insert time (owner -> 'a', joiners -> smaller
-- team) and SetPartyMemberTeam validates 'a'/'b', so NULL has no meaning. Backfill
-- legacy rows and enforce NOT NULL so sqlc generates the plain enum type instead of
-- a Null wrapper that row-mapping helpers silently drop.
UPDATE party_members SET team_id = 'a' WHERE team_id IS NULL;
ALTER TABLE party_members ALTER COLUMN team_id SET DEFAULT 'a';
ALTER TABLE party_members ALTER COLUMN team_id SET NOT NULL;

-- +goose Down
ALTER TABLE party_members ALTER COLUMN team_id DROP NOT NULL;
ALTER TABLE party_members ALTER COLUMN team_id DROP DEFAULT;
