-- +goose Up
CREATE TYPE staff_role AS ENUM ('admin', 'judge', 'moderator', 'lanista');
CREATE TABLE user_roles (
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 role staff_role NOT NULL,
 granted_by uuid REFERENCES users(id) ON DELETE SET NULL,
 granted_at timestamptz NOT NULL DEFAULT now(),
 reason text NOT NULL DEFAULT '',
 PRIMARY KEY (user_id, role)
);
INSERT INTO user_roles(user_id, role, reason)
 SELECT id, 'admin', 'Migrated staff access' FROM users WHERE is_admin;
INSERT INTO user_roles(user_id, role, reason)
 SELECT id, role, 'Migrated moderator access' FROM users
 CROSS JOIN unnest(ARRAY['judge','moderator']::staff_role[]) role
 WHERE is_moderator OR is_admin;
ALTER TABLE users DROP COLUMN is_admin, DROP COLUMN is_moderator;

-- +goose Down
-- Roles map back to the two flags; the lanista role has no flag and is lost.
ALTER TABLE users ADD COLUMN is_admin boolean DEFAULT false NOT NULL, ADD COLUMN is_moderator boolean DEFAULT false NOT NULL;
UPDATE users u SET
    is_admin = EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id = u.id AND r.role = 'admin'),
    is_moderator = EXISTS (SELECT 1 FROM user_roles r WHERE r.user_id = u.id AND r.role IN ('judge', 'moderator'));
DROP TABLE user_roles;
DROP TYPE staff_role;
