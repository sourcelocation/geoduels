package persistence

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

func mu(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	e := u.Scan(s)
	return u, e
}
func (s *DB) GetFinalMatchSnapshot(id string) ([]byte, bool, error) {
	if id == "" {
		return nil, false, errors.New("matchID required")
	}
	u, e := mu(id)
	if e != nil {
		return nil, false, e
	}
	r, e := s.db.GetFinalMatchSnapshot(context.Background(), u)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if e != nil {
		return nil, false, e
	}
	if len(r.ReplayZstd) == 0 {
		if r.ReplayJson == "" {
			return nil, false, nil
		}
		return []byte(r.ReplayJson), true, nil
	}
	raw, e := decompressReplay(r.ReplayZstd, int(r.ReplayCodec), int(r.ReplayUncompressedBytes))
	if e != nil {
		return nil, false, e
	}
	if len(r.ReplaySha256) == sha256.Size {
		h := sha256.Sum256(raw)
		if !equalBytes(h[:], r.ReplaySha256) {
			return nil, false, errors.New("replay checksum mismatch")
		}
	}
	return raw, true, nil
}
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var d byte
	for i := range a {
		d |= a[i] ^ b[i]
	}
	return d == 0
}
func (s *DB) ListPlayerMatchHistory(id string, l int) ([]MatchHistorySummary, error) {
	p, e := s.ListPlayerMatchHistoryPage(id, l, time.Time{}, "", false)
	return p.Matches, e
}
func (s *DB) ListPlayerMatchHistoryPage(id string, l int, b time.Time, bid string, rk bool) (MatchHistoryPage, error) {
	if id == "" {
		return MatchHistoryPage{}, errors.New("userID required")
	}
	if l <= 0 {
		l = 20
	}
	if l > 100 {
		l = 100
	}
	u, e := mu(id)
	if e != nil {
		return MatchHistoryPage{}, e
	}
	var rs []db.ListPlayerMatchHistoryBasicRow
	if rk {
		xs, ee := s.db.ListPlayerMatchHistoryRanked(context.Background(), db.ListPlayerMatchHistoryRankedParams{UserID: u, Limit: int32(l + 1)})
		e = ee
		for _, x := range xs {
			rs = append(rs, db.ListPlayerMatchHistoryBasicRow{MatchID: x.MatchID, Mode: x.Mode, StartedAt: x.StartedAt, EndedAt: x.EndedAt, WinnerUserID: x.WinnerUserID, Outcome: x.Outcome, Ranked: x.Ranked, RankedDelta: x.RankedDelta, TotalScore: x.TotalScore, OpponentUserID: x.OpponentUserID, OpponentDisplayName: x.OpponentDisplayName})
		}
	} else if !b.IsZero() && bid != "" {
		v, e2 := mu(bid)
		if e2 != nil {
			return MatchHistoryPage{}, e2
		}
		xs, ee := s.db.ListPlayerMatchHistoryBefore(context.Background(), db.ListPlayerMatchHistoryBeforeParams{UserID: u, Limit: int32(l + 1), CursorEndedAt: pgtype.Timestamptz{Time: b, Valid: true}, CursorMatchID: v})
		e = ee
		for _, x := range xs {
			rs = append(rs, db.ListPlayerMatchHistoryBasicRow{MatchID: x.MatchID, Mode: x.Mode, StartedAt: x.StartedAt, EndedAt: x.EndedAt, WinnerUserID: x.WinnerUserID, Outcome: x.Outcome, Ranked: x.Ranked, RankedDelta: x.RankedDelta, TotalScore: x.TotalScore, OpponentUserID: x.OpponentUserID, OpponentDisplayName: x.OpponentDisplayName})
		}
	} else {
		rs, e = s.db.ListPlayerMatchHistoryBasic(context.Background(), db.ListPlayerMatchHistoryBasicParams{UserID: u, Limit: int32(l + 1)})
	}
	if e != nil {
		return MatchHistoryPage{}, e
	}
	o := make([]MatchHistorySummary, 0, len(rs))
	for _, x := range rs {
		o = append(o, MatchHistorySummary{MatchID: fmt.Sprintf("%x", x.MatchID.Bytes), Mode: string(x.Mode), StartedAt: x.StartedAt.Time, EndedAt: x.EndedAt.Time, WinnerUserID: fmt.Sprint(x.WinnerUserID), Outcome: x.Outcome, Ranked: x.Ranked.Bool, RatingDelta: int(x.RankedDelta), TotalScore: int(x.TotalScore), OpponentUserID: x.OpponentUserID, OpponentDisplayName: fmt.Sprint(x.OpponentDisplayName)})
	}
	p := MatchHistoryPage{Matches: o}
	if len(o) > l {
		p.HasMore = true
		p.Matches = o[:l]
		p.NextEndedAt = p.Matches[l-1].EndedAt
		p.NextMatchID = p.Matches[l-1].MatchID
	}
	return p, nil
}
func (s *DB) PlayerParticipatedInMatch(a, b string) (bool, error) {
	if a == "" || b == "" {
		return false, nil
	}
	u, e := mu(a)
	if e != nil {
		return false, e
	}
	m, e := mu(b)
	if e != nil {
		return false, e
	}
	return s.db.PlayerParticipatedInMatch(context.Background(), db.PlayerParticipatedInMatchParams{UserID: u, MatchID: m})
}
