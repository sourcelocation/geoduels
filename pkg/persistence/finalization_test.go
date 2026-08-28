package persistence

import (
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"geoduels/pkg/contracts"
)

const (
	testPlayer1ID = "019f76f9-08fd-7804-9a7d-53a14d1fe900"
	testPlayer2ID = "019f6aa6-4cd5-73af-9200-384a6cb9c454"
)

func TestNewDuelResultRepresentsDrawWithoutWinnerSentinel(t *testing.T) {
	snap := contracts.MatchSnapshot{
		MatchID: "019f9176-9c92-7e4b-8ed6-40718c5a6bf3",
		Mode:    contracts.ModeDuel,
		Players: map[string]contracts.PlayerState{
			testPlayer1ID: {UserID: testPlayer1ID, HP: 100},
			testPlayer2ID: {UserID: testPlayer2ID, HP: 100},
		},
	}

	result, err := newDuelResult(&snap)
	if err != nil {
		t.Fatal(err)
	}
	if result.outcome != duelOutcomeDraw {
		t.Fatalf("outcome = %v, want draw", result.outcome)
	}
	if result.participants[0].won || result.participants[1].won {
		t.Fatal("draw must not mark either player as winner")
	}
	if result.ratingWinner() != "" {
		t.Fatalf("rating winner = %q, want rating library draw representation", result.ratingWinner())
	}
}

func TestNewDuelResultUsesStablePlayerOrdering(t *testing.T) {
	snap := contracts.MatchSnapshot{
		MatchID: "019f9176-9c92-7e4b-8ed6-40718c5a6bf3",
		Mode:    contracts.ModeDuel,
		Players: map[string]contracts.PlayerState{
			testPlayer1ID: {UserID: testPlayer1ID, HP: 10},
			testPlayer2ID: {UserID: testPlayer2ID, HP: 20},
		},
	}

	result, err := newDuelResult(&snap)
	if err != nil {
		t.Fatal(err)
	}
	if result.participants[0].userIDText != testPlayer2ID {
		t.Fatalf("first player = %q, want lexical UUID order", result.participants[0].userIDText)
	}
	if result.outcome != duelOutcomePlayer1Win || !result.participants[0].won {
		t.Fatal("outcome must follow the stable participant slots")
	}
}

func TestNewDuelResultRejectsInvalidPlayerID(t *testing.T) {
	snap := contracts.MatchSnapshot{
		MatchID: "019f9176-9c92-7e4b-8ed6-40718c5a6bf3",
		Mode:    contracts.ModeDuel,
		Players: map[string]contracts.PlayerState{
			"not-a-uuid":  {UserID: "not-a-uuid", HP: 100},
			testPlayer1ID: {UserID: testPlayer1ID, HP: 100},
		},
	}

	_, err := newDuelResult(&snap)
	if err == nil {
		t.Fatal("expected invalid player id error")
	}
}

func TestFinalizeDuelDrawIntegration(t *testing.T) {
	dsn := os.Getenv("FINALIZATION_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FINALIZATION_TEST_POSTGRES_URL is required")
	}
	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := &DB{pool: pool}

	const matchID = "019fb0f7-c348-753a-8c6d-cf3dd720f5a1"
	if _, err := pool.Exec(t.Context(), `
		insert into match_sessions(
			match_id, preset_id, mode, state, ranked, source_kind,
			config_json, node_id, node_epoch, public_route, lease_expires_at
		)
		values($1, 'ranked_duel', 'duel', 'live', false, 'queue',
		       '{}'::jsonb, 'test-node', 42, 'test-node', now()+interval '1 minute')
	`, matchID); err != nil {
		t.Fatal(err)
	}

	snap := contracts.MatchSnapshot{
		MatchID:  matchID,
		Mode:     contracts.ModeDuel,
		SeasonID: "s1",
		Unranked: true,
		State:    contracts.MatchEnded,
		Players: map[string]contracts.PlayerState{
			testPlayer1ID: {UserID: testPlayer1ID, DisplayName: "One", HP: 100, MMR: 500},
			testPlayer2ID: {UserID: testPlayer2ID, DisplayName: "Two", HP: 100, MMR: 500},
		},
	}
	if _, err := store.FinalizeMatch(snap, 42); err != nil {
		t.Fatal(err)
	}
	if _, err := store.FinalizeMatch(snap, 42); err != nil {
		t.Fatalf("idempotent finalize: %v", err)
	}

	rows, err := pool.Query(t.Context(), `
		select user_id::text, games_played, wins
		from user_stats
		where user_id in ($1, $2)
		order by user_id
	`, testPlayer1ID, testPlayer2ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var userID string
		var games, wins int
		if err := rows.Scan(&userID, &games, &wins); err != nil {
			t.Fatal(err)
		}
		if games != 1 || wins != 0 {
			t.Fatalf("%s stats = games:%d wins:%d, want 1/0", userID, games, wins)
		}
		count++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("stat rows = %d, want 2", count)
	}

	var state string
	var lease *time.Time
	if err := pool.QueryRow(t.Context(), `
		select state, lease_expires_at
		from match_sessions where match_id=$1
	`, matchID).Scan(&state, &lease); err != nil {
		t.Fatal(err)
	}
	if state != "ended" || lease != nil {
		t.Fatalf("session = state:%q lease:%v", state, lease)
	}
}

func TestStaleMatchLeaseReconciliationIntegration(t *testing.T) {
	dsn := os.Getenv("FINALIZATION_TEST_POSTGRES_URL")
	if dsn == "" {
		t.Skip("FINALIZATION_TEST_POSTGRES_URL is required")
	}
	pool, err := pgxpool.New(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := &DB{pool: pool}

	const matchID = "019fb0f7-c348-753a-8c6d-cf3dd720f5a2"
	if _, err := pool.Exec(t.Context(), `
		insert into match_sessions(
			match_id, preset_id, mode, state, ranked, source_kind,
			config_json, node_id, node_epoch, public_route, lease_expires_at
		)
		values($1, 'ranked_duel', 'duel', 'live', false, 'queue',
		       '{}'::jsonb, 'expired-node', 99, 'expired-node', now()-interval '10 minutes')
	`, matchID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `
		insert into runtime_matches(id, state, owner_epoch)
		values($1, 'live', 99)
	`, matchID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `
		insert into users(id, display_name, account_type)
		values
			($1, 'One', 'guest'),
			($2, 'Two', 'guest')
		on conflict(id) do nothing
	`, testPlayer1ID, testPlayer2ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `
		insert into match_participants(match_id, user_id)
		values($1, $2), ($1, $3)
	`, matchID, testPlayer1ID, testPlayer2ID); err != nil {
		t.Fatal(err)
	}

	resolved, err := store.ReconcileStaleMatchSessions(time.Minute, 10)
	if err != nil {
		t.Fatal(err)
	}
	if resolved != 1 {
		t.Fatalf("resolved = %d, want 1", resolved)
	}
	var sessionState, runtimeState string
	if err := pool.QueryRow(t.Context(), `
		select ms.state, rm.state
		from match_sessions ms
		join runtime_matches rm on rm.id=ms.match_id
		where ms.match_id=$1
	`, matchID).Scan(&sessionState, &runtimeState); err != nil {
		t.Fatal(err)
	}
	if sessionState != "ended" || runtimeState != "ended" {
		t.Fatalf("states = session:%q runtime:%q", sessionState, runtimeState)
	}
}
