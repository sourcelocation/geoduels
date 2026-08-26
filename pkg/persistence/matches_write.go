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
)

func (s *pgStore) FinalizeMatch(snap contracts.MatchSnapshot, ownerEpoch int64) (contracts.MatchSnapshot, error) {
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

	var sessionState string
	err = tx.QueryRow(ctx, `
		select state
		from match_sessions
		where match_id = $1
		for update
	`, snap.MatchID).Scan(&sessionState)
	if errors.Is(err, pgx.ErrNoRows) {
		return snap, errors.New("match session missing")
	}
	if err != nil {
		return snap, err
	}
	if sessionState == string(contracts.MatchEnded) {
		if err := applyPersistedRatingsToSnapshot(ctx, tx, &snap); err != nil {
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
	if err := ensureMatchUsersTx(ctx, tx, snap); err != nil {
		return snap, err
	}
	if err := recordMatchHistory(
		ctx,
		tx,
		snap.MatchID,
		snap,
		compressedReplay,
		replayHash[:],
		len(replay),
	); err != nil {
		return snap, err
	}
	if snap.Mode == contracts.ModeDuel {
		if err := finalizeDuelResultTx(ctx, tx, &snap, &result); err != nil {
			return snap, err
		}
	}
	if err := completeMatchSessionTx(ctx, tx, snap.MatchID); err != nil {
		return snap, err
	}
	if err := recordRuntimeMatchEndedTx(ctx, tx, snap.MatchID, ownerEpoch); err != nil {
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

func finalizeDuelResultTx(ctx context.Context, tx pgx.Tx, snap *contracts.MatchSnapshot, result *duelResult) error {
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

	rows, err := tx.Query(ctx, `
		select
			u.id,
			u.account_type = 'guest',
			r.mmr,
			r.rd,
			r.updated_at
		from unnest(array[$1::uuid, $2::uuid]) with ordinality requested(user_id, position)
		join users u on u.id = requested.user_id
		join ranks r
		  on r.user_id = u.id
		 and r.mode = $3
		 and r.season_id = $4
		order by requested.position
		for update of r
	`, p1.userID, p2.userID, modeDuel, seasonID)
	if err != nil {
		return err
	}
	defer rows.Close()
	participants := []*duelParticipantResult{p1, p2}
	index := 0
	for rows.Next() {
		if index >= len(participants) {
			return errors.New("duel result contains duplicate users")
		}
		var userID pgtype.UUID
		current := participants[index]
		if err := rows.Scan(
			&userID,
			&current.guest,
			&current.rating.MMR,
			&current.rating.RD,
			&current.rating.UpdatedAt,
		); err != nil {
			return err
		}
		if userID.Bytes != current.userID.Bytes {
			return errors.New("locked duel players are out of order")
		}
		index++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if index != len(participants) {
		return errors.New("duel player rating state is missing")
	}

	privatePartyMatch, err := matchBelongsToPartyTx(ctx, tx, snap.MatchID)
	if err != nil {
		return err
	}
	ratedMatch := !snap.Unranked && !privatePartyMatch && (!p1.guest || !p2.guest)
	now := time.Now()
	if ratedMatch {
		p1.update, p2.update = CalculateDuelRatingUpdates(p1.rating, p2.rating, result.ratingWinner(), now)
		if _, err := tx.Exec(ctx, `
			update ranks r
			set mmr = next.mmr,
				rd = next.rd,
				updated_at = $9
			from (
				values
					($1::uuid, $2::integer, $3::double precision, $4::boolean),
					($5::uuid, $6::integer, $7::double precision, $8::boolean)
			) as next(user_id, mmr, rd, apply)
			where r.user_id = next.user_id
			  and r.mode = $10
			  and r.season_id = $11
			  and next.apply
		`, p1.userID, p1.update.MMR, p1.update.RD, !p1.guest,
			p2.userID, p2.update.MMR, p2.update.RD, !p2.guest,
			now, modeDuel, seasonID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		update user_stats stats
		set games_played = stats.games_played + 1,
			wins = stats.wins + result.won::integer,
			updated_at = now()
		from (
			values
				($1::uuid, $2::boolean),
				($3::uuid, $4::boolean)
		) as result(user_id, won)
		where stats.user_id = result.user_id
	`, p1.userID, p1.won, p2.userID, p2.won); err != nil {
		return err
	}
	if ratedMatch {
		if _, err := tx.Exec(ctx, `
			update ranked_stats stats
			set games_played = stats.games_played + 1,
				wins = stats.wins + result.won::integer,
				updated_at = now()
			from (
				values
					($1::uuid, $2::boolean, $3::boolean),
					($4::uuid, $5::boolean, $6::boolean)
			) as result(user_id, won, apply)
			where stats.user_id = result.user_id
			  and stats.mode = $7
			  and stats.season_id = $8
			  and result.apply
		`, p1.userID, p1.won, !p1.guest,
			p2.userID, p2.won, !p2.guest,
			modeDuel, seasonID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			update match_players players
			set rating_before = result.rating_before,
				rating_after = result.rating_after,
				final_ranked_delta = result.rating_after - result.rating_before
			from (
				values
					($2::uuid, $3::integer, $4::integer, $5::boolean),
					($6::uuid, $7::integer, $8::integer, $9::boolean)
			) as result(user_id, rating_before, rating_after, apply)
			where players.match_id = $1
			  and players.user_id = result.user_id
			  and result.apply
		`, snap.MatchID,
			p1.userID, p1.rating.MMR, p1.update.MMR, !p1.guest,
			p2.userID, p2.rating.MMR, p2.update.MMR, !p2.guest); err != nil {
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

func applyPersistedRatingsToSnapshot(ctx context.Context, tx pgx.Tx, snap *contracts.MatchSnapshot) error {
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
	rows, err := tx.Query(ctx, `
		select
			mp.user_id::text,
			coalesce(mp.rating_after, mp.mmr),
			coalesce(r.rd, mp.rating_rd, $4)
		from match_players mp
		left join ranks r
		  on r.user_id = mp.user_id
		 and r.mode = $2
		 and r.season_id = $3
		where mp.match_id = $1
		  and mp.user_id in (
			select value::uuid from jsonb_array_elements_text($5::jsonb)
		  )
	`, snap.MatchID, modeDuel, seasonID, initialRatingRD, string(encodedIDs))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var userID string
		var mmr int
		var rd float64
		if err := rows.Scan(&userID, &mmr, &rd); err != nil {
			return err
		}
		player, ok := snap.Players[userID]
		if !ok {
			continue
		}
		player.MMR = mmr
		player.RatingRD = rd
		snap.Players[userID] = player
	}
	return rows.Err()
}

func completeMatchSessionTx(ctx context.Context, tx pgx.Tx, matchID string) error {
	if _, err := tx.Exec(ctx, `
		update match_sessions
		set state = 'ended',
			ended_at = coalesce(ended_at, now()),
			lease_expires_at = null,
			updated_at = now()
		where match_id = $1
	`, matchID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		update parties
		set state = 'open',
			last_match_id = $1,
			active_match_id = null,
			started_match_id = null,
			updated_at = now()
		where active_match_id = $1 or started_match_id = $1
	`, matchID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		update party_members pm
		set ready = false
		from match_sessions ms
		where ms.match_id = $1
		  and pm.party_id = ms.source_party_id
	`, matchID); err != nil {
		return err
	}
	return nil
}

func recordRuntimeMatchEndedTx(ctx context.Context, tx pgx.Tx, matchID string, ownerEpoch int64) error {
	_, err := tx.Exec(ctx, `
		insert into runtime_matches(id, state, owner_epoch, started_at, ended_at)
		values($1,$2,$3,now(),now())
		on conflict (id) do update set
			state = excluded.state,
			owner_epoch = excluded.owner_epoch,
			ended_at = now()
	`, matchID, string(contracts.MatchEnded), ownerEpoch)
	return err
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

func matchBelongsToPartyTx(ctx context.Context, tx pgx.Tx, matchID string) (bool, error) {
	if matchID == "" {
		return false, nil
	}
	var exists bool
	if err := tx.QueryRow(ctx, `
		select exists (
			select 1
			from match_sessions
			where match_id = $1
			  and source_kind = 'party'
		) or exists (
			select 1
			from parties
			where active_match_id = $1
			   or started_match_id = $1
			   or last_match_id = $1
		)
	`, matchID).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// ensureMatchUsersTx upserts the users row (and the season-scoped ranked
// bookkeeping rows) for every player in the snapshot so that subsequent inserts
// into user-referencing tables satisfy their foreign keys. It is idempotent and
// safe to call for any mode. Blank user ids are skipped rather than inserted —
// they would fail the uuid cast and never belong in a match snapshot.
func ensureMatchUsersTx(ctx context.Context, tx pgx.Tx, snap contracts.MatchSnapshot) error {
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
	if _, err := tx.Exec(ctx, `
		insert into users (id, email, display_name, avatar_url, account_type)
		select input.user_id, null, input.display_name, null, 'guest'
		from jsonb_to_recordset($1::jsonb)
			as input(user_id uuid, display_name text)
		on conflict (id) do nothing
	`, string(encodedSeeds)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		insert into ranks (user_id, mode, mmr, season_id)
		select input.user_id, $2, $3, $4
		from jsonb_to_recordset($1::jsonb)
			as input(user_id uuid)
		on conflict (user_id, mode, season_id) do nothing
	`, string(encodedSeeds), modeDuel, initialMMR, seasonID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		insert into user_stats (user_id, games_played, wins)
		select input.user_id, 0, 0
		from jsonb_to_recordset($1::jsonb)
			as input(user_id uuid)
		on conflict (user_id) do nothing
	`, string(encodedSeeds)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		insert into ranked_stats (user_id, mode, season_id, games_played, wins)
		select input.user_id, $2, $3, 0, 0
		from jsonb_to_recordset($1::jsonb)
			as input(user_id uuid)
		on conflict (user_id, mode, season_id) do nothing
	`, string(encodedSeeds), modeDuel, seasonID); err != nil {
		return err
	}
	return nil
}

func recordMatchHistory(
	ctx context.Context,
	tx pgx.Tx,
	matchID string,
	snap contracts.MatchSnapshot,
	compressedReplay []byte,
	replayHash []byte,
	replayUncompressedBytes int,
) error {
	if matchID == "" {
		matchID = snap.MatchID
	}
	if matchID == "" || len(snap.Players) == 0 {
		return nil
	}
	startedAt := snapshotStartedAt(snap)
	endedAt := time.Now()
	winner := snapshotWinner(snap)
	privatePartyMatch := false
	if err := tx.QueryRow(ctx, `
		select exists (
			select 1
			from match_sessions
			where match_id = $1
			  and source_kind = 'party'
		) or exists (
			select 1
			from parties
			where active_match_id = $1
			   or started_match_id = $1
			   or last_match_id = $1
		)
	`, matchID).Scan(&privatePartyMatch); err != nil {
		return err
	}
	ranked := snap.Mode == contracts.ModeDuel && !snap.Unranked && !privatePartyMatch
	sourceKind := "queue"
	var sourcePartyID any
	if snap.Mode == contracts.ModeSingleplayer {
		sourceKind = "solo"
	} else if privatePartyMatch {
		sourceKind = "party"
		var partyID string
		if err := tx.QueryRow(ctx, `
			select coalesce(source_party_id::text, '')
			from match_sessions
			where match_id = $1
		`, matchID).Scan(&partyID); err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
		} else if strings.TrimSpace(partyID) != "" {
			sourcePartyID = partyID
		}
		if sourcePartyID == nil {
			if err := tx.QueryRow(ctx, `
				select id
				from parties
				where active_match_id = $1
				   or started_match_id = $1
				   or last_match_id = $1
				limit 1
			`, matchID).Scan(&partyID); err != nil {
				if !errors.Is(err, pgx.ErrNoRows) {
					return err
				}
			} else {
				sourcePartyID = partyID
			}
		}
	}
	ruleset := string(contracts.NormalizeRuleset(snap.Config.Ruleset))
	mapID := snap.Config.MapID
	_ = tx.QueryRow(ctx, `select map_id::text from match_round_plans where match_id=$1 order by round_index limit 1`, matchID).Scan(&mapID)
	if _, err := tx.Exec(ctx, `
		insert into match_history(
			match_id, mode, started_at, ended_at, winner_user_id,
			ranked, source_kind, source_party_id, ruleset, map_id,
			replay_zstd, replay_codec, replay_schema_version, replay_uncompressed_bytes,
			replay_sha256, replay_expires_at, round_count
		)
		values($1,$2,$3,$4,nullif($5,'')::uuid,$6,$7,nullif($8,'')::uuid,nullif($9,'')::gd_ruleset,
		       nullif($10,'')::uuid,$11,$12,$13,$14,$15,
		       $4::timestamptz + make_interval(days => $16::integer),$17)
		on conflict (match_id) do update set
			mode = excluded.mode,
			started_at = excluded.started_at,
			ended_at = excluded.ended_at,
			winner_user_id = excluded.winner_user_id,
			ranked = excluded.ranked,
			source_kind = excluded.source_kind,
			source_party_id = excluded.source_party_id,
			ruleset = excluded.ruleset,
			map_id = excluded.map_id,
			replay_json = null,
			replay_zstd = excluded.replay_zstd,
			replay_codec = excluded.replay_codec,
			replay_schema_version = excluded.replay_schema_version,
			replay_uncompressed_bytes = excluded.replay_uncompressed_bytes,
			replay_sha256 = excluded.replay_sha256,
			replay_expires_at = excluded.replay_expires_at,
			round_count = excluded.round_count
	`, matchID, string(snap.Mode), startedAt, endedAt, winner,
		ranked, sourceKind, sourcePartyID, ruleset, mapID,
		compressedReplay, replayCodecZstd, replaySchemaVersion, replayUncompressedBytes,
		replayHash, replayRetentionDays, len(snap.RoundResults)); err != nil {
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
	if _, err := tx.Exec(ctx, `
		insert into match_players(
			match_id, user_id, display_name, mmr, hp, rating_rd, total_score, ended_at
		)
		select
			$1, input.user_id, input.display_name, input.mmr, input.hp,
			input.rating_rd, input.total_score, $2
		from jsonb_to_recordset($3::jsonb) as input(
			user_id uuid,
			display_name text,
			mmr integer,
			hp integer,
			rating_rd double precision,
			total_score integer
		)
		on conflict (match_id, user_id) do update set
			display_name = excluded.display_name,
			mmr = excluded.mmr,
			hp = excluded.hp,
			rating_rd = excluded.rating_rd,
			total_score = excluded.total_score,
			ended_at = excluded.ended_at
	`, matchID, endedAt, string(encodedPlayers)); err != nil {
		return err
	}
	if snap.Mode == contracts.ModeSingleplayer && len(snap.RoundResults) == 5 && strings.TrimSpace(mapID) != "" {
		rulesetCode := 0
		if contracts.NormalizeRuleset(snap.Config.Ruleset) == contracts.RulesetNMPZ {
			rulesetCode = 1
		}
		if _, err := tx.Exec(ctx, `
			insert into player_map_bests(user_id,map_id,ruleset,best_score,match_id,achieved_at)
			select input.user_id,$2,$3,input.total_score,$1,$4
			from jsonb_to_recordset($5::jsonb)
				as input(user_id uuid, total_score integer)
			on conflict(user_id,map_id,ruleset) do update
			set best_score=excluded.best_score,
			    match_id=excluded.match_id,
			    achieved_at=excluded.achieved_at
			where excluded.best_score>player_map_bests.best_score
		`, matchID, mapID, rulesetCode, endedAt, string(encodedPlayers)); err != nil {
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
		if _, err := tx.Exec(ctx, `
			insert into ranked_guess_events(
				user_id, match_id, round_number, score, guess_ms, evidence, occurred_at
			)
			select
				input.user_id, $1, input.round_number, input.score,
				input.guess_ms, input.evidence, input.occurred_at
			from jsonb_to_recordset($2::jsonb) as input(
				user_id uuid,
				round_number smallint,
				score smallint,
				guess_ms integer,
				evidence real,
				occurred_at timestamptz
			)
			on conflict (match_id, round_number, user_id) do update set
				score = excluded.score,
				guess_ms = excluded.guess_ms,
				evidence = excluded.evidence,
				occurred_at = excluded.occurred_at
		`, matchID, string(encodedGuesses)); err != nil {
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
