package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/pkg/contracts"
	db "geoduels/pkg/persistence/sqlc/db"
)

func (s *DB) FinalizeMatch(snap contracts.MatchSnapshot, ownerEpoch int64) (contracts.MatchSnapshot, error) {
	if strings.TrimSpace(snap.MatchID) == "" {
		return snap, errors.New("match id required")
	}
	var matchUUID pgtype.UUID
	if err := matchUUID.Scan(snap.MatchID); err != nil || !matchUUID.Valid {
		return snap, errors.New("invalid match id")
	}
	var result duelResult
	if snap.Mode == contracts.ModeDuel {
		var err error
		result, err = newDuelResult(&snap)
		if err != nil {
			return snap, err
		}
	}
	replay, err := finalReplaySnapshotJSON(snap)
	if err != nil {
		return snap, err
	}
	compressedReplay, replayHash, err := compressReplay(replay)
	if err != nil {
		return snap, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return snap, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)

	sessionState, err := q.LockMatchSessionState(ctx, matchUUID)
	if errors.Is(err, pgx.ErrNoRows) {
		return snap, errors.New("match session missing")
	}
	if err != nil {
		return snap, err
	}
	if sessionState == db.GdMatchSessionStateEnded {
		if err := applyPersistedRatingsToSnapshot(ctx, tx, q, &snap); err != nil {
			return snap, err
		}
		if err := tx.Commit(ctx); err != nil {
			return snap, err
		}
		return snap, nil
	}

	// Guarantee the users rows referenced by match_players (and the ranked
	// bookkeeping tables) exist before we write anything that points at them.
	// A player can reach finalize without a users row: GetProfile never inserts
	// one, and the guest-cleanup job can hard-delete a guest mid-match. Without
	// this, recordMatchHistory's match_players insert fails the user_id foreign
	// key and the match retries finalize forever.
	if err := ensureMatchUsersTx(ctx, tx, q, snap); err != nil {
		return snap, err
	}
	if err := recordMatchHistory(
		ctx,
		q,
		matchUUID,
		snap,
		compressedReplay,
		replayHash[:],
		len(replay),
	); err != nil {
		return snap, err
	}
	if snap.Mode == contracts.ModeDuel {
		if err := finalizeDuelResultTx(ctx, tx, q, &snap, &result); err != nil {
			return snap, err
		}
	}
	if err := completeMatchSessionTx(ctx, q, matchUUID); err != nil {
		return snap, err
	}
	if err := recordRuntimeMatchEndedTx(ctx, q, matchUUID, ownerEpoch); err != nil {
		return snap, err
	}
	if err := tx.Commit(ctx); err != nil {
		return snap, err
	}
	if snap.Mode == contracts.ModeDuel && !snap.Unranked {
		go func(matchID string) {
			_ = s.EvaluateAutoCheatBansForMatch(matchID)
		}(snap.MatchID)
	}
	return snap, nil
}

func finalizeDuelResultTx(ctx context.Context, tx pgx.Tx, q *db.Queries, snap *contracts.MatchSnapshot, result *duelResult) error {
	if snap == nil || result == nil {
		return errors.New("duel result is required")
	}
	p1 := &result.participants[0]
	p2 := &result.participants[1]
	seasonID := strings.TrimSpace(snap.SeasonID)
	if seasonID == "" {
		var err error
		seasonID, err = activeSeasonIDTx(ctx, tx)
		if err != nil {
			return err
		}
	}

	locked, err := q.LockDuelRatings(ctx, db.LockDuelRatingsParams{
		User1ID:  p1.userID,
		User2ID:  p2.userID,
		Mode:     db.GdMatchMode(modeDuel),
		SeasonID: seasonID,
	})
	if err != nil {
		return err
	}
	participants := []*duelParticipantResult{p1, p2}
	if len(locked) != len(participants) {
		return errors.New("duel player rating state is missing")
	}
	for i, row := range locked {
		current := participants[i]
		if row.ID.Bytes != current.userID.Bytes {
			return errors.New("locked duel players are out of order")
		}
		current.guest = row.IsGuest
		current.rating.MMR = int(row.Mmr)
		current.rating.RD = row.Rd
		current.rating.UpdatedAt = row.UpdatedAt.Time
	}

	privatePartyMatch, err := matchBelongsToPartyTx(ctx, q, snap.MatchID)
	if err != nil {
		return err
	}
	ratedMatch := !snap.Unranked && !privatePartyMatch && (!p1.guest || !p2.guest)
	now := time.Now()
	if ratedMatch {
		p1.update, p2.update = CalculateDuelRatingUpdates(p1.rating, p2.rating, result.ratingWinner(), now)
		if err := q.UpdateDuelRanks(ctx, db.UpdateDuelRanksParams{
			UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
			Mode:      db.GdMatchMode(modeDuel),
			SeasonID:  seasonID,
			P1UserID:  p1.userID, P1Mmr: int32(p1.update.MMR), P1Rd: p1.update.RD, P1Apply: !p1.guest,
			P2UserID: p2.userID, P2Mmr: int32(p2.update.MMR), P2Rd: p2.update.RD, P2Apply: !p2.guest,
		}); err != nil {
			return err
		}
	}
	if err := q.AddDuelUserStats(ctx, db.AddDuelUserStatsParams{
		P1UserID: p1.userID, P1Won: p1.won,
		P2UserID: p2.userID, P2Won: p2.won,
	}); err != nil {
		return err
	}
	if ratedMatch {
		if err := q.AddDuelRankedStats(ctx, db.AddDuelRankedStatsParams{
			Mode:     db.GdMatchMode(modeDuel),
			SeasonID: seasonID,
			P1UserID: p1.userID, P1Won: p1.won, P1Apply: !p1.guest,
			P2UserID: p2.userID, P2Won: p2.won, P2Apply: !p2.guest,
		}); err != nil {
			return err
		}
		if err := q.SetMatchPlayerRatingDeltas(ctx, db.SetMatchPlayerRatingDeltasParams{
			MatchID:  chatUUID(snap.MatchID),
			P1UserID: p1.userID, P1RatingBefore: int32(p1.rating.MMR), P1RatingAfter: int32(p1.update.MMR), P1Apply: !p1.guest,
			P2UserID: p2.userID, P2RatingBefore: int32(p2.rating.MMR), P2RatingAfter: int32(p2.update.MMR), P2Apply: !p2.guest,
		}); err != nil {
			return err
		}
	}
	if ratedMatch && !p1.guest {
		if err := awardEloBadgesTx(ctx, tx, p1.userIDText, p1.update.MMR); err != nil {
			return err
		}
	}
	if ratedMatch && !p2.guest {
		if err := awardEloBadgesTx(ctx, tx, p2.userIDText, p2.update.MMR); err != nil {
			return err
		}
	}
	if ratedMatch {
		fast5000 := rankedSpeedrunnerUsers(*snap)
		if fast5000[p1.userIDText] && !p1.guest {
			if _, err := awardBadgeTx(ctx, tx, p1.userIDText, "speedrunner"); err != nil {
				return err
			}
		}
		if fast5000[p2.userIDText] && !p2.guest {
			if _, err := awardBadgeTx(ctx, tx, p2.userIDText, "speedrunner"); err != nil {
				return err
			}
		}
	}
	if ratedMatch && !p1.guest {
		p1.player.MMR = p1.update.MMR
		p1.player.RatingRD = p1.update.RD
		snap.Players[p1.userIDText] = p1.player
	}
	if ratedMatch && !p2.guest {
		p2.player.MMR = p2.update.MMR
		p2.player.RatingRD = p2.update.RD
		snap.Players[p2.userIDText] = p2.player
	}
	return nil
}

func applyPersistedRatingsToSnapshot(ctx context.Context, tx pgx.Tx, q *db.Queries, snap *contracts.MatchSnapshot) error {
	if snap == nil || snap.Mode != contracts.ModeDuel {
		return nil
	}
	seasonID := strings.TrimSpace(snap.SeasonID)
	if seasonID == "" {
		var err error
		seasonID, err = activeSeasonIDTx(ctx, tx)
		if err != nil {
			return err
		}
	}
	userIDs := make([]string, 0, len(snap.Players))
	for userID := range snap.Players {
		userIDs = append(userIDs, userID)
	}
	encodedIDs, err := json.Marshal(userIDs)
	if err != nil {
		return err
	}
	rows, err := q.MatchPlayerPersistedRatings(ctx, db.MatchPlayerPersistedRatingsParams{
		DefaultRd:   initialRatingRD,
		Mode:        db.GdMatchMode(modeDuel),
		SeasonID:    seasonID,
		MatchID:     chatUUID(snap.MatchID),
		UserIdsJson: encodedIDs,
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		userID := uuidVal(row.UserID)
		player, ok := snap.Players[userID]
		if !ok {
			continue
		}
		player.MMR = int(row.Mmr)
		player.RatingRD = row.Rd
		snap.Players[userID] = player
	}
	return nil
}

func completeMatchSessionTx(ctx context.Context, q *db.Queries, matchID pgtype.UUID) error {
	if err := q.CompleteMatchSession(ctx, matchID); err != nil {
		return err
	}
	if err := q.ReopenPartiesAfterMatch(ctx, matchID); err != nil {
		return err
	}
	return q.ResetPartyMembersAfterMatch(ctx, matchID)
}

func recordRuntimeMatchEndedTx(ctx context.Context, q *db.Queries, matchID pgtype.UUID, ownerEpoch int64) error {
	return q.RecordRuntimeMatchEnded(ctx, db.RecordRuntimeMatchEndedParams{
		ID:         matchID,
		State:      db.GdRuntimeState(contracts.MatchEnded),
		OwnerEpoch: ownerEpoch,
	})
}

func rankedSpeedrunnerUsers(snap contracts.MatchSnapshot) map[string]bool {
	out := map[string]bool{}
	for _, round := range snap.RoundResults {
		if round == nil {
			continue
		}
		for userID, result := range round.Players {
			if result.Score >= 5000 && result.GuessMS > 0 && result.GuessMS < 30000 {
				out[userID] = true
			}
		}
	}
	return out
}

func matchBelongsToPartyTx(ctx context.Context, q *db.Queries, matchID string) (bool, error) {
	if matchID == "" {
		return false, nil
	}
	matchUUID, err := chatUUIDErr(matchID)
	if err != nil {
		return false, nil
	}
	exists, err := q.MatchBelongsToParty(ctx, matchUUID)
	if err != nil {
		return false, err
	}
	return exists.Bool, nil
}

// ensureMatchUsersTx upserts the users row (and the season-scoped ranked
// bookkeeping rows) for every player in the snapshot so that subsequent inserts
// into user-referencing tables satisfy their foreign keys. It is idempotent and
// safe to call for any mode. Blank user ids are skipped rather than inserted —
// they would fail the uuid cast and never belong in a match snapshot.
func ensureMatchUsersTx(ctx context.Context, tx pgx.Tx, q *db.Queries, snap contracts.MatchSnapshot) error {
	if len(snap.Players) == 0 {
		return nil
	}
	seasonID := strings.TrimSpace(snap.SeasonID)
	if seasonID == "" {
		var err error
		seasonID, err = activeSeasonIDTx(ctx, tx)
		if err != nil {
			return err
		}
	}
	type playerSeed struct {
		UserID      string `json:"user_id"`
		DisplayName string `json:"display_name"`
	}
	seeds := make([]playerSeed, 0, len(snap.Players))
	for _, p := range snap.Players {
		userID := strings.TrimSpace(p.UserID)
		if userID == "" {
			continue
		}
		name := p.DisplayName
		if name == "" {
			name = userID
		}
		seeds = append(seeds, playerSeed{UserID: userID, DisplayName: name})
	}
	if len(seeds) == 0 {
		return nil
	}
	encodedSeeds, err := json.Marshal(seeds)
	if err != nil {
		return err
	}
	if err := q.EnsureMatchUsers(ctx, encodedSeeds); err != nil {
		return err
	}
	if err := q.EnsureMatchRanks(ctx, db.EnsureMatchRanksParams{Mode: db.GdMatchMode(modeDuel), Mmr: int32(initialMMR), SeasonID: seasonID, PlayerIdsJson: encodedSeeds}); err != nil {
		return err
	}
	if err := q.EnsureMatchUserStats(ctx, encodedSeeds); err != nil {
		return err
	}
	return q.EnsureMatchRankedStats(ctx, db.EnsureMatchRankedStatsParams{Mode: db.GdMatchMode(modeDuel), SeasonID: seasonID, PlayerIdsJson: encodedSeeds})
}

func recordMatchHistory(
	ctx context.Context,
	q *db.Queries,
	matchUUID pgtype.UUID,
	snap contracts.MatchSnapshot,
	compressedReplay []byte,
	replayHash []byte,
	replayUncompressedBytes int,
) error {
	if len(snap.Players) == 0 {
		return nil
	}
	startedAt := snapshotStartedAt(snap)
	endedAt := time.Now()
	winner := snapshotWinner(snap)
	privatePartyMatch, err := matchBelongsToPartyTx(ctx, q, snap.MatchID)
	if err != nil {
		return err
	}
	ranked := snap.Mode == contracts.ModeDuel && !snap.Unranked && !privatePartyMatch
	sourceKind := "queue"
	var sourcePartyID any
	if snap.Mode == contracts.ModeSingleplayer {
		sourceKind = "solo"
	} else if privatePartyMatch {
		sourceKind = "party"
		partyID, err := q.GetMatchSourcePartyID(ctx, matchUUID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if partyID.Valid {
			sourcePartyID = uuidVal(partyID)
		}
		if sourcePartyID == nil {
			foundPartyID, err := q.FindPartyIDByMatchID(ctx, matchUUID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if foundPartyID.Valid {
				sourcePartyID = uuidVal(foundPartyID)
			}
		}
	}
	ruleset := string(contracts.NormalizeRuleset(snap.Config.Ruleset))
	mapID := snap.Config.MapID
	if planMapID, err := q.GetMatchRoundPlanMapID(ctx, matchUUID); err == nil && planMapID.Valid {
		mapID = uuidVal(planMapID)
	}
	if err := q.UpsertMatchHistory(ctx, db.UpsertMatchHistoryParams{
		MatchID:                 matchUUID,
		Mode:                    db.GdMatchMode(snap.Mode),
		StartedAt:               pgtype.Timestamptz{Time: startedAt, Valid: true},
		EndedAt:                 pgtype.Timestamptz{Time: endedAt, Valid: true},
		WinnerUserID:            winner,
		Ranked:                  ranked,
		SourceKind:              db.GdMatchSource(sourceKind),
		SourcePartyID:           sourcePartyID,
		Ruleset:                 ruleset,
		MapID:                   mapID,
		ReplayZstd:              compressedReplay,
		ReplayCodec:             pgtype.Int2{Int16: replayCodecZstd, Valid: true},
		ReplaySchemaVersion:     pgtype.Int2{Int16: replaySchemaVersion, Valid: true},
		ReplayUncompressedBytes: pgtype.Int4{Int32: int32(replayUncompressedBytes), Valid: true},
		ReplaySha256:            replayHash,
		ReplayRetentionDays:     replayRetentionDays,
		RoundCount:              int16(len(snap.RoundResults)),
	}); err != nil {
		return err
	}
	type matchPlayerRecord struct {
		UserID      string  `json:"user_id"`
		DisplayName string  `json:"display_name"`
		MMR         int     `json:"mmr"`
		HP          int     `json:"hp"`
		RatingRD    float64 `json:"rating_rd"`
		TotalScore  int     `json:"total_score"`
	}
	playerRecords := make([]matchPlayerRecord, 0, len(snap.Players))
	for userID, player := range snap.Players {
		if strings.TrimSpace(userID) == "" {
			// A blank user id never has a users row and would fail the uuid
			// cast; skip it so the rest of the match still records.
			continue
		}
		displayName := player.DisplayName
		if displayName == "" {
			displayName = userID
		}
		totalScore := 0
		for _, round := range snap.RoundResults {
			if round != nil {
				totalScore += round.Players[userID].Score
			}
		}
		playerRecords = append(playerRecords, matchPlayerRecord{
			UserID:      userID,
			DisplayName: displayName,
			MMR:         player.MMR,
			HP:          player.HP,
			RatingRD:    clampRatingRD(player.RatingRD),
			TotalScore:  totalScore,
		})
	}
	encodedPlayers, err := json.Marshal(playerRecords)
	if err != nil {
		return err
	}
	if err := q.UpsertMatchPlayers(ctx, db.UpsertMatchPlayersParams{
		MatchID:     matchUUID,
		PlayersJson: encodedPlayers,
		EndedAt:     pgtype.Timestamptz{Time: endedAt, Valid: true},
	}); err != nil {
		return err
	}
	if snap.Mode == contracts.ModeSingleplayer && len(snap.RoundResults) == 5 && strings.TrimSpace(mapID) != "" {
		rulesetCode := 0
		if contracts.NormalizeRuleset(snap.Config.Ruleset) == contracts.RulesetNMPZ {
			rulesetCode = 1
		}
		if err := q.UpsertPlayerMapBests(ctx, db.UpsertPlayerMapBestsParams{
			MapID:       chatUUID(mapID),
			Ruleset:     int16(rulesetCode),
			MatchID:     matchUUID,
			AchievedAt:  pgtype.Timestamptz{Time: endedAt, Valid: true},
			PlayersJson: encodedPlayers,
		}); err != nil {
			return err
		}
	}
	type rankedGuessRecord struct {
		UserID     string  `json:"user_id"`
		Round      int     `json:"round_number"`
		Score      int     `json:"score"`
		GuessMS    int64   `json:"guess_ms"`
		Evidence   float64 `json:"evidence"`
		OccurredAt string  `json:"occurred_at"`
	}
	guessRecords := make([]rankedGuessRecord, 0, len(snap.RoundResults)*len(snap.Players))
	for _, round := range snap.RoundResults {
		if round == nil {
			continue
		}
		for userID, result := range round.Players {
			if strings.TrimSpace(userID) == "" {
				continue
			}
			if snap.Mode == contracts.ModeDuel && !snap.Unranked && !privatePartyMatch && result.GuessMS > 0 {
				occurredAt := endedAt
				if result.GuessUnixMS > 0 {
					occurredAt = time.UnixMilli(result.GuessUnixMS)
				}
				guessRecords = append(guessRecords, rankedGuessRecord{
					UserID:     userID,
					Round:      round.RoundNumber,
					Score:      result.Score,
					GuessMS:    min(result.GuessMS, int64(2147483647)),
					Evidence:   guessEvidence(result.Score, result.GuessMS),
					OccurredAt: occurredAt.UTC().Format(time.RFC3339Nano),
				})
			}
		}
	}
	if len(guessRecords) > 0 {
		encodedGuesses, err := json.Marshal(guessRecords)
		if err != nil {
			return err
		}
		if err := q.UpsertRankedGuessEvents(ctx, db.UpsertRankedGuessEventsParams{
			MatchID:    matchUUID,
			EventsJson: encodedGuesses,
		}); err != nil {
			return err
		}
	}
	return nil
}

func finalReplaySnapshotJSON(snap contracts.MatchSnapshot) ([]byte, error) {
	snap.CurrentRound = nil
	snap.RoundMSLeft = 0
	snap.PhaseEndsAt = 0
	snap.PhaseStartedAt = 0
	snap.EventSequence = 0
	snap.ServerUnixMS = 0
	snap.GraceWindowSec = 0
	for id, player := range snap.Players {
		player.Finalized = false
		player.LastGuessLat = 0
		player.LastGuessLng = 0
		player.HasGuess = false
		player.Disconnected = false
		player.DisconnectDue = 0
		snap.Players[id] = player
	}
	body, err := json.Marshal(snap)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func guessEvidence(score int, guessMS int64) float64 {
	if score < 4900 || guessMS <= 0 || guessMS > 15000 {
		return 0
	}
	seconds := float64(guessMS) / 1000.0
	scoreExcess := float64(score-4900) / 100.0
	if scoreExcess < 0 {
		scoreExcess = 0
	}
	speed := (15.0 - seconds) / 12.0
	if speed < 0 {
		speed = 0
	}
	if speed > 1.25 {
		speed = 1.25
	}
	evidence := 0.75 + scoreExcess*4.0
	evidence *= speed
	if score >= 5000 && guessMS <= 3000 {
		evidence += 5
	} else if score >= 4990 && guessMS <= 5000 {
		evidence += 3
	} else if score >= 4950 && guessMS <= 5000 {
		evidence += 1.5
	}
	return evidence
}

func snapshotStartedAt(snap contracts.MatchSnapshot) time.Time {
	if len(snap.RoundResults) > 0 {
		first := snap.RoundResults[0]
		if first != nil {
			for _, player := range first.Players {
				if player.GuessUnixMS > 0 {
					t := time.UnixMilli(player.GuessUnixMS - player.GuessMS)
					if !t.IsZero() {
						return t
					}
				}
			}
		}
	}
	if snap.PhaseStartedAt > 0 {
		return time.UnixMilli(snap.PhaseStartedAt)
	}
	return time.Now()
}

func snapshotWinner(snap contracts.MatchSnapshot) string {
	if snap.Mode == contracts.ModeSingleplayer {
		return ""
	}
	winner := ""
	winnerHP := -1
	tie := false
	for userID, player := range snap.Players {
		if player.HP > winnerHP {
			winner = userID
			winnerHP = player.HP
			tie = false
		} else if player.HP == winnerHP {
			tie = true
		}
	}
	if tie {
		return ""
	}
	return winner
}
