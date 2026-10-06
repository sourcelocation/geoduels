-- name: CreateChangelogPost :one
INSERT INTO changelog_posts(slug,title,markdown,published,updated_at) VALUES($1,$2,$3,$4,now()) RETURNING id,slug,title,markdown,published,created_at,updated_at;

-- name: GetChangelogPost :one
SELECT id,slug,title,markdown,published,created_at,updated_at FROM changelog_posts WHERE slug=sqlc.arg(slug) AND (sqlc.arg(include_unpublished)::boolean=false OR published=true);

-- name: ListChangelogPosts :many
SELECT id,slug,title,markdown,published,created_at,updated_at FROM changelog_posts WHERE ($1::boolean OR published=true) ORDER BY updated_at DESC,id DESC;

-- name: UpdateChangelogPost :one
UPDATE changelog_posts SET slug=$2,title=$3,markdown=$4,published=$5,updated_at=now() WHERE id=$1 RETURNING id,slug,title,markdown,published,created_at,updated_at;
