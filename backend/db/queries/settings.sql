-- name: GetSetting :one
SELECT value_json FROM site_settings WHERE key=$1;

-- name: SetSetting :exec
INSERT INTO site_settings(key,value_json,updated_at) VALUES(sqlc.arg(setting_key),convert_from(sqlc.arg(value_json), 'UTF8')::jsonb,now()) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,updated_at=now();

-- name: DeleteSetting :exec
DELETE FROM site_settings WHERE key = $1;
