package persistence

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/contracts"
)

func (s *pgStore) GetRuntimeMatch(ctx context.Context, matchID string) (RuntimeMatch, bool, error) {
	if matchID == "" {
		return RuntimeMatch{}, false, errors.New("matchID required")
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	row := s.pool.QueryRow(ctx, `
		select
			id,
			state,
			owner_epoch,
			started_at,
			coalesce(ended_at, '0001-01-01 00:00:00+00'::timestamptz)
		from runtime_matches
		where id = $1
	`, matchID)
	var out RuntimeMatch
	if err := row.Scan(&out.MatchID, &out.State, &out.OwnerEpoch, &out.StartedAt, &out.EndedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RuntimeMatch{}, false, nil
		}
		return RuntimeMatch{}, false, err
	}
	return out, true, nil
}

func (s *pgStore) RecordRuntimeMatch(ctx context.Context, matchID, state string, ownerEpoch int64, terminal bool) error {
	if matchID == "" {
		return errors.New("matchID required")
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	if terminal {
		_, err := s.pool.Exec(ctx, `
			insert into runtime_matches(id, state, owner_epoch, started_at, ended_at)
			values($1,$2,$3,now(),now())
			on conflict (id) do update set
				state = excluded.state,
				owner_epoch = excluded.owner_epoch,
				ended_at = now()
		`, matchID, state, ownerEpoch)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		insert into runtime_matches(id, state, owner_epoch, started_at)
		values($1,$2,$3,now())
		on conflict (id) do update set
			state = excluded.state,
			owner_epoch = excluded.owner_epoch
	`, matchID, state, ownerEpoch)
	return err
}

func (s *pgStore) ExpireStaleRuntimeMatches(ctx context.Context, mode string, olderThan time.Duration) error {
	if mode == "" || olderThan <= 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
		update runtime_matches
		set state = $1,
			ended_at = now()
		where state = $2
		  and exists (
		    select 1 from match_sessions ms
		    where ms.match_id=runtime_matches.id and ms.mode=$3
		  )
		  and started_at < now() - $4::interval
		  and ended_at is null
	`, string(contracts.MatchEnded), string(contracts.MatchLive), mode, olderThan.String())
	return err
}
