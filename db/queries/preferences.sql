-- name: GetUserPreferences :one
SELECT schema_version, preferences_json, revision
FROM user_preferences
WHERE user_id = $1;

-- name: UpsertUserPreferences :one
INSERT INTO user_preferences(user_id, schema_version, preferences_json, revision, updated_at)
VALUES ($1, $2, $3::jsonb, 1, now())
ON CONFLICT (user_id) DO UPDATE
SET schema_version = EXCLUDED.schema_version,
    preferences_json = EXCLUDED.preferences_json,
    revision = user_preferences.revision + 1,
    updated_at = now()
WHERE user_preferences.revision = $4
RETURNING schema_version, preferences_json, revision;
