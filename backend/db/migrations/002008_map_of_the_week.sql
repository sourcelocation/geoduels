-- +goose Up
CREATE TABLE motw_schedule (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 starts_at timestamptz NOT NULL DEFAULT now(),
 closes_at timestamptz NOT NULL DEFAULT now() + interval '7 days',
 current_award_at timestamptz,
 CHECK(closes_at>starts_at)
);
INSERT INTO motw_schedule(singleton) VALUES(true);
CREATE TABLE motw_nominations (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 cycle_start timestamptz NOT NULL,
 map_id uuid NOT NULL REFERENCES maps(id) ON DELETE CASCADE,
 nominated_by uuid REFERENCES users(id) ON DELETE SET NULL,
 nominated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(cycle_start,map_id)
);
CREATE TABLE motw_likes (
 nomination_id bigint NOT NULL REFERENCES motw_nominations(id) ON DELETE CASCADE,
 user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 PRIMARY KEY(nomination_id,user_id)
);
CREATE TABLE motw_awards (
 cycle_start timestamptz PRIMARY KEY,
 selected_at timestamptz NOT NULL,
 map_id uuid REFERENCES maps(id) ON DELETE SET NULL,
 map_name text NOT NULL,
 creator_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
 source text NOT NULL CHECK(source IN ('nomination','trending')),
 likes integer NOT NULL DEFAULT 0
);
CREATE INDEX motw_awards_map ON motw_awards(map_id,selected_at DESC);

-- +goose Down
DROP TABLE motw_awards;
DROP TABLE motw_likes;
DROP TABLE motw_nominations;
DROP TABLE motw_schedule;
