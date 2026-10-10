package matches

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"geoduels/internal/storekit"
	"geoduels/pkg/contracts"
	"geoduels/pkg/matchkind"
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

// FinalMatchSnapshot returns a recorded match's final snapshot, with its kind: a match recorded
// before kinds existed gets the kind its history row describes.
func (s *PGStore) FinalMatchSnapshot(id string) (*contracts.MatchSnapshot, bool, error) {
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
	raw := r.ReplayJson
	if len(r.ReplayZstd) > 0 {
		raw, e = decompressReplay(r.ReplayZstd, int(r.ReplayCodec), int(r.ReplayUncompressedBytes))
		if e != nil {
			return nil, false, e
		}
		if len(r.ReplaySha256) == sha256.Size {
			h := sha256.Sum256(raw)
			if !equalBytes(h[:], r.ReplaySha256) {
				return nil, false, errors.New("replay checksum mismatch")
			}
		}
	}
	if len(raw) == 0 {
		return nil, false, nil
	}
	var snap contracts.MatchSnapshot
	if e := json.Unmarshal(raw, &snap); e != nil {
		return nil, false, e
	}
	if snap.Kind == "" {
		snap.Kind = matchkind.FromHistory(contracts.MatchMode(r.Mode), r.Ranked)
	}
	return &snap, true, nil
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
func (s *PGStore) ListPlayerMatchHistory(id string, l int) ([]MatchHistorySummary, error) {
	p, e := s.ListPlayerMatchHistoryPage(id, l, time.Time{}, "", false)
	return p.Matches, e
}
func (s *PGStore) ListPlayerMatchHistoryPage(id string, l int, b time.Time, bid string, rk bool) (MatchHistoryPage, error) {
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
	var cursorEndedAt pgtype.Timestamptz
	if !b.IsZero() {
		cursorEndedAt = pgtype.Timestamptz{Time: b, Valid: true}
	}
	var cursorMatchID pgtype.UUID
	if bid != "" {
		cursorMatchID, e = mu(bid)
		if e != nil {
			return MatchHistoryPage{}, e
		}
	}
	rs, e := s.db.ListPlayerMatchHistoryPage(context.Background(), db.ListPlayerMatchHistoryPageParams{
		UserID:        u,
		RankedOnly:    rk,
		CursorEndedAt: cursorEndedAt,
		CursorMatchID: cursorMatchID,
		RowLimit:      int32(l + 1),
	})
	if e != nil {
		return MatchHistoryPage{}, e
	}
	o := make([]MatchHistorySummary, 0, len(rs))
	for _, x := range rs {
		o = append(o, MatchHistorySummary{MatchID: storekit.UUIDVal(x.MatchID), Mode: string(x.Mode), StartedAt: x.StartedAt.Time, EndedAt: x.EndedAt.Time, WinnerUserID: storekit.UUIDVal(x.WinnerUserID), Outcome: x.Outcome, Ranked: x.Ranked.Bool, RatingDelta: int(x.RankedDelta), TotalScore: int(x.TotalScore), OpponentUserID: storekit.UUIDVal(x.OpponentUserID), OpponentDisplayName: storekit.TextVal(x.OpponentDisplayName)})
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
func (s *PGStore) PlayerParticipatedInMatch(a, b string) (bool, error) {
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
