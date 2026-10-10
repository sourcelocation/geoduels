-- +goose Up
-- Remember the explicitly selected party without reviving older memberships.
CREATE TABLE current_parties (
    user_id uuid PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    party_id uuid NOT NULL REFERENCES parties(id) ON DELETE CASCADE
);

-- Legacy clients did not record a selection; seed the most recently joined
-- available party once. Future selections are written by create/join.
INSERT INTO current_parties (user_id, party_id)
SELECT DISTINCT ON (m.user_id) m.user_id, m.party_id
FROM party_members m JOIN parties p ON p.id = m.party_id
WHERE m.left_at IS NULL AND p.state IN ('open', 'in_match', 'started')
  AND (p.state <> 'open' OR p.expires_at > now())
ORDER BY m.user_id, m.joined_at DESC, p.created_at DESC, p.id;

-- +goose Down
DROP TABLE current_parties;
