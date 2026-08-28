-- name: ResolveMapIdentity :one
SELECT m.id::text AS id, COALESCE((SELECT a.alias::text FROM map_aliases a WHERE a.map_id=m.id ORDER BY a.created_at,a.alias LIMIT 1),m.id::text)::text AS key
FROM maps m WHERE m.id::text=$1::text OR EXISTS(SELECT 1 FROM map_aliases a WHERE a.map_id=m.id AND a.alias=$1::text) LIMIT 1;
