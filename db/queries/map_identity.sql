-- name: ResolveMapIdentity :one
SELECT m.id AS id, COALESCE((SELECT a.alias FROM map_aliases a WHERE a.map_id=m.id ORDER BY a.created_at,a.alias LIMIT 1),m.id::text) AS key
FROM maps m
WHERE (sqlc.narg(map_id)::uuid IS NOT NULL AND m.id=sqlc.narg(map_id))
   OR (sqlc.narg(alias)::text IS NOT NULL AND EXISTS(SELECT 1 FROM map_aliases a WHERE a.map_id=m.id AND a.alias=sqlc.narg(alias)))
LIMIT 1;
