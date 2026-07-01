package persistence

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *pgStore) GetFinalMatchSnapshot(matchID string) ([]byte, bool, error) {
	if matchID == "" {
		return nil, false, errors.New("matchID required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	row := s.pool.QueryRow(ctx, `
		select replay_zstd, coalesce(replay_codec, 0), coalesce(replay_uncompressed_bytes, 0),
		       replay_sha256, replay_json::text
		from match_history
		where match_id = $1
		  and (replay_expires_at is null or replay_expires_at > now())
		limit 1
	`, matchID)
	var compressed, expectedHash []byte
	var codec, uncompressedBytes int
	var legacy *string
	if err := row.Scan(&compressed, &codec, &uncompressedBytes, &expectedHash, &legacy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if len(compressed) == 0 {
		if legacy == nil {
			return nil, false, nil
		}
		return []byte(*legacy), true, nil
	}
	raw, err := decompressReplay(compressed, codec, uncompressedBytes)
	if err != nil {
		return nil, false, err
	}
	if len(expectedHash) == sha256.Size {
		sum := sha256.Sum256(raw)
		if !equalBytes(sum[:], expectedHash) {
			return nil, false, errors.New("replay checksum mismatch")
		}
	}
	return raw, true, nil
}

func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}

func (s *pgStore) ListPlayerMatchHistory(userID string, limit int) ([]MatchHistorySummary, error) {
	page, err := s.ListPlayerMatchHistoryPage(userID, limit, time.Time{}, "", false)
	return page.Matches, err
}

func (s *pgStore) ListPlayerMatchHistoryPage(userID string, limit int, beforeEndedAt time.Time, beforeMatchID string, rankedOnly bool) (MatchHistoryPage, error) {
	if userID == "" {
		return MatchHistoryPage{}, errors.New("userID required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	query := `
		select
			h.match_id, h.mode, h.started_at, h.ended_at,
			coalesce(h.winner_user_id::text, ''),
			case
				when h.mode = 'singleplayer' then 'completed'
				when h.winner_user_id is null then 'draw'
				when h.winner_user_id = p.user_id then 'win'
				else 'loss'
			end,
			coalesce(h.ranked, false) and h.mode = 'duel',
			coalesce(p.final_ranked_delta, 0),
			coalesce(p.total_score, 0),
			coalesce(opponent.user_id, ''),
			coalesce(opponent.display_name, '')
		from match_players p
		join match_history h on h.match_id = p.match_id
		left join lateral (
			select
				op.user_id::text as user_id,
				coalesce(nullif(op.display_name, ''), nullif(u.display_name, ''), op.user_id::text) as display_name
			from match_players op
			left join users u on u.id = op.user_id
			where op.match_id = p.match_id
			  and op.user_id <> p.user_id
			order by op.total_score desc, op.user_id
			limit 1
		) opponent on true
		where p.user_id = $1
	`
	args := []any{userID, limit + 1}
	if rankedOnly {
		query += ` and h.mode = 'duel' and coalesce(h.ranked, false)`
	}
	if !beforeEndedAt.IsZero() && beforeMatchID != "" {
		query += ` and (p.ended_at, p.match_id) < ($3, $4::uuid)`
		args = append(args, beforeEndedAt, beforeMatchID)
	}
	query += `
		order by p.ended_at desc, p.match_id desc
		limit $2
	`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return MatchHistoryPage{}, err
	}
	defer rows.Close()
	out := make([]MatchHistorySummary, 0, limit+1)
	for rows.Next() {
		var item MatchHistorySummary
		if err := rows.Scan(
			&item.MatchID,
			&item.Mode,
			&item.StartedAt,
			&item.EndedAt,
			&item.WinnerUserID,
			&item.Outcome,
			&item.Ranked,
			&item.RatingDelta,
			&item.TotalScore,
			&item.OpponentUserID,
			&item.OpponentDisplayName,
		); err != nil {
			return MatchHistoryPage{}, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return MatchHistoryPage{}, err
	}
	page := MatchHistoryPage{Matches: out}
	if len(out) > limit {
		page.HasMore = true
		page.Matches = out[:limit]
		last := page.Matches[len(page.Matches)-1]
		page.NextEndedAt = last.EndedAt
		page.NextMatchID = last.MatchID
	}
	return page, nil
}

func (s *pgStore) PlayerParticipatedInMatch(userID, matchID string) (bool, error) {
	if userID == "" || matchID == "" {
		return false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var exists bool
	err := s.pool.QueryRow(ctx, `
		select exists (
			select 1
			from match_players
			where user_id = $1 and match_id = $2
		)
	`, userID, matchID).Scan(&exists)
	return exists, err
}
