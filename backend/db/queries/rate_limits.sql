-- name: HitRateLimit :one
-- Counts a hit in the key's fixed window, opening a new window once the last one is over.
INSERT INTO rate_limit_windows (key, window_started_at, hits)
VALUES (sqlc.arg(key), now(), 1)
ON CONFLICT (key) DO UPDATE SET
    hits = CASE WHEN rate_limit_windows.window_started_at <= now() - (sqlc.arg(window_seconds)::double precision * interval '1 second')
        THEN 1 ELSE rate_limit_windows.hits + 1 END,
    window_started_at = CASE WHEN rate_limit_windows.window_started_at <= now() - (sqlc.arg(window_seconds)::double precision * interval '1 second')
        THEN now() ELSE rate_limit_windows.window_started_at END
RETURNING hits, window_started_at;
