package matches

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/pkg/gameplay"
)

type duelOutcome uint8

const (
	duelOutcomeDraw duelOutcome = iota
	duelOutcomePlayer1Win
	duelOutcomePlayer2Win
)

type duelParticipantResult struct {
	userID     pgtype.UUID
	userIDText string
	player     gameplay.PlayerState
	won        bool
	guest      bool
	rating     RatingState
	update     RatingUpdate
}

type duelResult struct {
	outcome      duelOutcome
	participants [2]duelParticipantResult
}

func newDuelResult(snap *gameplay.MatchSnapshot) (duelResult, error) {
	if snap == nil {
		return duelResult{}, fmt.Errorf("duel snapshot is required")
	}
	if len(snap.Players) != 2 {
		return duelResult{}, fmt.Errorf(
			"duel %q has %d players; expected 2",
			snap.MatchID,
			len(snap.Players),
		)
	}

	keys := make([]string, 0, len(snap.Players))
	for key := range snap.Players {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var result duelResult
	for index, key := range keys {
		player := snap.Players[key]
		playerID := strings.TrimSpace(player.UserID)
		if playerID == "" {
			return duelResult{}, fmt.Errorf(
				"duel %q player %d has no user id",
				snap.MatchID,
				index+1,
			)
		}
		if playerID != strings.TrimSpace(key) {
			return duelResult{}, fmt.Errorf(
				"duel %q player map key does not match player user id",
				snap.MatchID,
			)
		}
		var parsed pgtype.UUID
		if err := parsed.Scan(playerID); err != nil || !parsed.Valid {
			return duelResult{}, fmt.Errorf(
				"duel %q player %d has an invalid user id",
				snap.MatchID,
				index+1,
			)
		}
		result.participants[index] = duelParticipantResult{
			userID:     parsed,
			userIDText: playerID,
			player:     player,
			update: RatingUpdate{
				MMR: player.MMR,
				RD:  clampRatingRD(player.RatingRD),
			},
		}
	}

	p1 := &result.participants[0]
	p2 := &result.participants[1]
	switch {
	case p1.player.HP > p2.player.HP:
		result.outcome = duelOutcomePlayer1Win
		p1.won = true
	case p2.player.HP > p1.player.HP:
		result.outcome = duelOutcomePlayer2Win
		p2.won = true
	default:
		result.outcome = duelOutcomeDraw
	}
	return result, nil
}

func (r duelResult) ratingWinner() string {
	switch r.outcome {
	case duelOutcomePlayer1Win:
		return "p1"
	case duelOutcomePlayer2Win:
		return "p2"
	default:
		return ""
	}
}
