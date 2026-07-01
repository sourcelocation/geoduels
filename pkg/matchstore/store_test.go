package matchstore

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"geoduels/pkg/contracts"
)

func TestMemoryStoreDoesNotExpandImmediately(t *testing.T) {
	store := newMemory()
	highMMR := 2200
	lowMMR := 1600

	_, match, err := store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "high", DisplayName: "high", MMR: highMMR})
	if err != nil {
		t.Fatalf("first join failed: %v", err)
	}
	if match != nil {
		t.Fatalf("expected first join to queue")
	}

	_, match, err = store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "low", DisplayName: "low", MMR: lowMMR})
	if err != nil {
		t.Fatalf("second join failed: %v", err)
	}
	if match != nil {
		t.Fatalf("expected 600 MMR gap to remain queued immediately")
	}
}

func TestMemoryStoreMatchesPlayersInsideBaseWindowAfterMutualWait(t *testing.T) {
	store := newMemory()

	_, match, err := store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "p1", DisplayName: "p1", MMR: 1600})
	if err != nil {
		t.Fatalf("first join failed: %v", err)
	}
	if match != nil {
		t.Fatalf("expected first join to queue")
	}

	joined, match, err := store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "p2", DisplayName: "p2", MMR: 1600 + baseMatchWindowMMR})
	if err != nil {
		t.Fatalf("second join failed: %v", err)
	}
	if joined.Status != "queued" || match != nil {
		t.Fatalf("expected second player to queue before matchmaking tick, status=%q match=%v", joined.Status, match)
	}
	if matched, err := store.RunMatchmaking(QueuePoolRegistered, contracts.RulesetMoving, 50); err != nil {
		t.Fatalf("matchmaking failed: %v", err)
	} else if matched != 0 {
		t.Fatalf("expected no match before mutual wait, got %d", matched)
	}

	ageMemoryQueue(store, QueuePoolRegistered, mutualMatchWaitMS+int64(10*time.Millisecond))

	if matched, err := store.RunMatchmaking(QueuePoolRegistered, contracts.RulesetMoving, 50); err != nil {
		t.Fatalf("matchmaking after mutual wait failed: %v", err)
	} else if matched != 1 {
		t.Fatalf("expected one match after mutual wait, got %d", matched)
	}
	match, err = store.Poll(QueuePoolRegistered, []contracts.GameRuleset{contracts.RulesetMoving}, "p1")
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if match == nil {
		t.Fatalf("expected players inside base window to match after mutual wait")
	}
}

func TestMemoryStoreCarriesSeasonIDIntoMatch(t *testing.T) {
	store := newMemory()

	if _, _, err := store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "p1", DisplayName: "p1", MMR: 1600, SeasonID: "s-test"}); err != nil {
		t.Fatalf("first join failed: %v", err)
	}
	if _, _, err := store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "p2", DisplayName: "p2", MMR: 1600, SeasonID: "s-test"}); err != nil {
		t.Fatalf("second join failed: %v", err)
	}
	ageMemoryQueue(store, QueuePoolRegistered, mutualMatchWaitMS+int64(10*time.Millisecond))

	if _, err := store.RunMatchmaking(QueuePoolRegistered, contracts.RulesetMoving, 50); err != nil {
		t.Fatalf("matchmaking failed: %v", err)
	}
	match, err := store.Poll(QueuePoolRegistered, []contracts.GameRuleset{contracts.RulesetMoving}, "p1")
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if match == nil || match.SeasonID != "s-test" {
		t.Fatalf("match season = %#v, want s-test", match)
	}
}

func TestMemoryStoreExpandsWindowOverTime(t *testing.T) {
	store := newMemory()
	highMMR := 1000 + baseMatchWindowMMR + 1
	lowMMR := 1000

	_, match, err := store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "high", DisplayName: "high", MMR: highMMR})
	if err != nil {
		t.Fatalf("first join failed: %v", err)
	}
	if match != nil {
		t.Fatalf("expected first join to queue")
	}

	_, match, err = store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "low", DisplayName: "low", MMR: lowMMR})
	if err != nil {
		t.Fatalf("second join failed: %v", err)
	}
	if match != nil {
		t.Fatalf("expected second player to remain queued before the rating window expands")
	}

	mem := store.(*memoryStore)
	for i := range mem.queues[QueuePoolRegistered][QueueMoving] {
		mem.queues[QueuePoolRegistered][QueueMoving][i].JoinedAtUnixMS -= mutualMatchWaitMS + matchExpandEveryMS
	}

	if matched, err := store.RunMatchmaking(QueuePoolRegistered, contracts.RulesetMoving, 50); err != nil {
		t.Fatalf("matchmaking failed: %v", err)
	} else if matched != 1 {
		t.Fatalf("expected one match after the queue window expanded, got %d", matched)
	}
	match, err = store.Poll(QueuePoolRegistered, []contracts.GameRuleset{contracts.RulesetMoving}, "high")
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if match == nil {
		t.Fatalf("expected a match after the queue window expanded")
	}
}

func TestMemoryStoreCapsExpandedWindowAtFiveHundredMMR(t *testing.T) {
	store := newMemory()

	_, match, err := store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "high", DisplayName: "high", MMR: 2001})
	if err != nil {
		t.Fatalf("first join failed: %v", err)
	}
	if match != nil {
		t.Fatalf("expected first join to queue")
	}

	_, match, err = store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "low", DisplayName: "low", MMR: 1500})
	if err != nil {
		t.Fatalf("second join failed: %v", err)
	}
	if match != nil {
		t.Fatalf("expected second player outside the capped rating window to remain queued")
	}

	mem := store.(*memoryStore)
	for i := range mem.queues[QueuePoolRegistered][QueueMoving] {
		mem.queues[QueuePoolRegistered][QueueMoving][i].JoinedAtUnixMS -= mutualMatchWaitMS + matchExpandEveryMS*100
	}

	if matched, err := store.RunMatchmaking(QueuePoolRegistered, contracts.RulesetMoving, 50); err != nil {
		t.Fatalf("matchmaking outside cap failed: %v", err)
	} else if matched != 0 {
		t.Fatalf("expected no matches outside cap, got %d", matched)
	}
	match, err = store.Poll(QueuePoolRegistered, []contracts.GameRuleset{contracts.RulesetMoving}, "high")
	if err != nil {
		t.Fatalf("poll outside cap failed: %v", err)
	}
	if match != nil {
		t.Fatalf("expected 501 MMR gap to remain queued, match=%v", match)
	}

	_, match, err = store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "within", DisplayName: "within", MMR: 1501})
	if err != nil {
		t.Fatalf("third join failed: %v", err)
	}
	if match != nil {
		t.Fatalf("expected fresh third player to remain queued before matchmaking tick")
	}

	for i := range mem.queues[QueuePoolRegistered][QueueMoving] {
		if mem.queues[QueuePoolRegistered][QueueMoving][i].UserID == "within" {
			mem.queues[QueuePoolRegistered][QueueMoving][i].JoinedAtUnixMS -= mutualMatchWaitMS + matchExpandEveryMS*100
		}
	}
	if matched, err := store.RunMatchmaking(QueuePoolRegistered, contracts.RulesetMoving, 50); err != nil {
		t.Fatalf("matchmaking inside cap failed: %v", err)
	} else if matched != 1 {
		t.Fatalf("expected one match inside cap, got %d", matched)
	}
	match, err = store.Poll(QueuePoolRegistered, []contracts.GameRuleset{contracts.RulesetMoving}, "high")
	if err != nil {
		t.Fatalf("poll inside cap failed: %v", err)
	}
	if match == nil {
		t.Fatalf("expected 500 MMR gap to match after full expansion")
	}
}

func TestRedisStoreMatchesPlayersInsideBaseWindowAfterMutualWait(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	store := &redisStore{rdb: rdb}

	if _, match, err := store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "p1", DisplayName: "p1", MMR: 1600}); err != nil {
		t.Fatalf("first join failed: %v", err)
	} else if match != nil {
		t.Fatalf("expected first join to queue")
	}

	joined, match, err := store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "p2", DisplayName: "p2", MMR: 1600})
	if err != nil {
		t.Fatalf("second join failed: %v", err)
	}
	if joined.Status != "queued" || match != nil {
		t.Fatalf("expected redis store to queue until matchmaking tick, status=%q match=%v", joined.Status, match)
	}

	if matched, err := store.RunMatchmaking(QueuePoolRegistered, contracts.RulesetMoving, 50); err != nil {
		t.Fatalf("redis matchmaking failed: %v", err)
	} else if matched != 0 {
		t.Fatalf("expected redis store to wait before matching, got %d", matched)
	}

	ageRedisQueue(t, rdb, QueuePoolRegistered, []string{"p1", "p2"}, mutualMatchWaitMS+int64(10*time.Millisecond))

	if matched, err := store.RunMatchmaking(QueuePoolRegistered, contracts.RulesetMoving, 50); err != nil {
		t.Fatalf("redis matchmaking after mutual wait failed: %v", err)
	} else if matched != 1 {
		t.Fatalf("expected redis store to create one match after mutual wait, got %d", matched)
	}
	match, err = store.Poll(QueuePoolRegistered, []contracts.GameRuleset{contracts.RulesetMoving}, "p1")
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if match == nil {
		t.Fatalf("expected redis store to match after mutual wait")
	}
}

func TestRedisStoreCarriesSeasonIDIntoMatch(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	store := &redisStore{rdb: rdb}
	if _, _, err := store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "p1", DisplayName: "p1", MMR: 1600, SeasonID: "s-test"}); err != nil {
		t.Fatalf("first join failed: %v", err)
	}
	if _, _, err := store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "p2", DisplayName: "p2", MMR: 1600, SeasonID: "s-test"}); err != nil {
		t.Fatalf("second join failed: %v", err)
	}
	ageRedisQueue(t, rdb, QueuePoolRegistered, []string{"p1", "p2"}, mutualMatchWaitMS+int64(10*time.Millisecond))

	if _, err := store.RunMatchmaking(QueuePoolRegistered, contracts.RulesetMoving, 50); err != nil {
		t.Fatalf("matchmaking failed: %v", err)
	}
	match, err := store.Poll(QueuePoolRegistered, []contracts.GameRuleset{contracts.RulesetMoving}, "p1")
	if err != nil {
		t.Fatalf("poll failed: %v", err)
	}
	if match == nil || match.SeasonID != "s-test" {
		t.Fatalf("match season = %#v, want s-test", match)
	}
}

func TestRedisStoreKeepsRulesetQueuesSeparateAndClearsOtherSelections(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	store := &redisStore{rdb: rdb}
	req := func(userID string) contracts.QueueJoinRequest {
		return contracts.QueueJoinRequest{UserID: userID, DisplayName: userID, MMR: 1600}
	}
	if _, _, err := store.Join(QueuePoolRegistered, contracts.RulesetMoving, req("both")); err != nil {
		t.Fatalf("join moving: %v", err)
	}
	if _, _, err := store.Join(QueuePoolRegistered, contracts.RulesetNMPZ, req("both")); err != nil {
		t.Fatalf("join nmpz: %v", err)
	}
	if _, _, err := store.Join(QueuePoolRegistered, contracts.RulesetNMPZ, req("nmpz-only")); err != nil {
		t.Fatalf("join nmpz-only: %v", err)
	}
	ageRedisQueueRuleset(t, rdb, QueuePoolRegistered, contracts.RulesetNMPZ, []string{"both", "nmpz-only"}, mutualMatchWaitMS+int64(10*time.Millisecond))
	if matched, err := store.RunMatchmaking(QueuePoolRegistered, contracts.RulesetMoving, 50); err != nil {
		t.Fatalf("moving matchmaking failed: %v", err)
	} else if matched != 0 {
		t.Fatalf("expected no moving match, got %d", matched)
	}
	if matched, err := store.RunMatchmaking(QueuePoolRegistered, contracts.RulesetNMPZ, 50); err != nil {
		t.Fatalf("nmpz matchmaking failed: %v", err)
	} else if matched != 1 {
		t.Fatalf("expected one nmpz match, got %d", matched)
	}
	match, err := store.Poll(QueuePoolRegistered, []contracts.GameRuleset{contracts.RulesetNMPZ}, "both")
	if err != nil {
		t.Fatalf("poll nmpz match: %v", err)
	}
	if match == nil || match.Config.Ruleset != contracts.RulesetNMPZ {
		t.Fatalf("expected nmpz match, got %#v", match)
	}
	if queued, err := store.IsQueued(QueuePoolRegistered, []contracts.GameRuleset{contracts.RulesetMoving}, "both"); err != nil {
		t.Fatalf("moving queue check: %v", err)
	} else if queued {
		t.Fatal("expected matched user to be removed from moving queue")
	}
}

func TestQueueVariantConfigDefinesAllSixQueues(t *testing.T) {
	tests := map[QueueVariant]contracts.MatchConfig{
		QueueMoving:       {Ruleset: contracts.RulesetMoving, StreetNames: contracts.StreetNamesShown},
		QueueNoMove:       {Ruleset: contracts.RulesetNoMove, StreetNames: contracts.StreetNamesShown},
		QueueNMPZ:         {Ruleset: contracts.RulesetNMPZ, StreetNames: contracts.StreetNamesShown},
		QueueMovingHidden: {Ruleset: contracts.RulesetMoving, StreetNames: contracts.StreetNamesHidden},
		QueueNoMoveHidden: {Ruleset: contracts.RulesetNoMove, StreetNames: contracts.StreetNamesHidden},
		QueueNMPZHidden:   {Ruleset: contracts.RulesetNMPZ, StreetNames: contracts.StreetNamesHidden},
	}
	for queue, want := range tests {
		got := QueueVariantConfig(queue)
		if got.Ruleset != want.Ruleset || got.StreetNames != want.StreetNames {
			t.Fatalf("%s config = %#v, want %#v", queue, got, want)
		}
	}
}

func TestMemoryStoreMatchesHiddenNoMoveQueueWithExpectedConfig(t *testing.T) {
	store := newMemory()
	for _, userID := range []string{"p1", "p2"} {
		if _, _, err := store.Join(
			QueuePoolRegistered,
			QueueNoMoveHidden,
			contracts.QueueJoinRequest{UserID: userID, DisplayName: userID, MMR: 1500},
		); err != nil {
			t.Fatalf("join %s: %v", userID, err)
		}
	}
	ageMemoryQueue(store, QueuePoolRegistered, mutualMatchWaitMS+1)
	if matched, err := store.RunMatchmaking(QueuePoolRegistered, QueueNoMoveHidden, 50); err != nil || matched != 1 {
		t.Fatalf("RunMatchmaking = %d, %v; want 1, nil", matched, err)
	}
	match, err := store.Poll(
		QueuePoolRegistered,
		[]QueueVariant{QueueNoMoveHidden},
		"p1",
	)
	if err != nil || match == nil {
		t.Fatalf("Poll = %#v, %v; want match", match, err)
	}
	if match.Config.Ruleset != contracts.RulesetNoMove ||
		match.Config.StreetNames != contracts.StreetNamesHidden {
		t.Fatalf("match config = %#v", match.Config)
	}
}

func ageMemoryQueue(store Store, pool QueuePool, ageMS int64) {
	mem := store.(*memoryStore)
	for _, queue := range AllQueueVariants {
		for i := range mem.queues[pool][queue] {
			mem.queues[pool][queue][i].JoinedAtUnixMS -= ageMS
		}
	}
}

func ageRedisQueue(t *testing.T, rdb *redis.Client, pool QueuePool, userIDs []string, ageMS int64) {
	ageRedisQueueRuleset(t, rdb, pool, contracts.RulesetMoving, userIDs, ageMS)
}

func ageRedisQueueRuleset(t *testing.T, rdb *redis.Client, pool QueuePool, ruleset contracts.GameRuleset, userIDs []string, ageMS int64) {
	t.Helper()
	ctx := context.Background()
	joinedAt := time.Now().UnixMilli() - ageMS
	for _, userID := range userIDs {
		key := queueTicketKey(pool, ruleset, userID)
		raw, err := rdb.Get(ctx, key).Bytes()
		if err != nil {
			t.Fatalf("get queue ticket for %s: %v", userID, err)
		}
		var tkt ticket
		if err := json.Unmarshal(raw, &tkt); err != nil {
			t.Fatalf("decode queue ticket for %s: %v", userID, err)
		}
		tkt.JoinedAtUnixMS = joinedAt
		nextRaw, err := json.Marshal(tkt)
		if err != nil {
			t.Fatalf("encode queue ticket for %s: %v", userID, err)
		}
		if err := rdb.Set(ctx, key, nextRaw, queueTicketTTL).Err(); err != nil {
			t.Fatalf("set queue ticket for %s: %v", userID, err)
		}
		if err := rdb.ZAdd(ctx, queueJoinedKey(pool, ruleset), redis.Z{Score: float64(joinedAt), Member: userID}).Err(); err != nil {
			t.Fatalf("set queue joined score for %s: %v", userID, err)
		}
	}
}

func TestMemoryStoreHeartbeatReflectsQueueState(t *testing.T) {
	store := newMemory()

	if _, _, err := store.Join(QueuePoolRegistered, contracts.RulesetMoving, contracts.QueueJoinRequest{UserID: "solo", DisplayName: "solo", MMR: 1000}); err != nil {
		t.Fatalf("join failed: %v", err)
	}

	status, err := store.Heartbeat(QueuePoolRegistered, []contracts.GameRuleset{contracts.RulesetMoving}, "solo")
	if err != nil {
		t.Fatalf("heartbeat failed: %v", err)
	}
	if status != QueuePresenceQueueing {
		t.Fatalf("expected queueing heartbeat status, got %q", status)
	}

	if err := store.Leave(QueuePoolRegistered, []contracts.GameRuleset{contracts.RulesetMoving}, "solo"); err != nil {
		t.Fatalf("leave failed: %v", err)
	}

	status, err = store.Heartbeat(QueuePoolRegistered, []contracts.GameRuleset{contracts.RulesetMoving}, "solo")
	if err != nil {
		t.Fatalf("heartbeat after leave failed: %v", err)
	}
	if status != QueuePresenceMissing {
		t.Fatalf("expected missing heartbeat status after leave, got %q", status)
	}
}

func ageRedisTicket(t *testing.T, rdb *redis.Client, pool QueuePool, userID string, ageMS int64) {
	t.Helper()
	ctx := context.Background()
	key := queueTicketKey(pool, contracts.RulesetMoving, userID)
	raw, err := rdb.Get(ctx, key).Bytes()
	if err != nil {
		t.Fatalf("get redis ticket %s: %v", userID, err)
	}
	var queued ticket
	if err := json.Unmarshal(raw, &queued); err != nil {
		t.Fatalf("unmarshal redis ticket %s: %v", userID, err)
	}
	queued.JoinedAtUnixMS -= ageMS
	aged, err := json.Marshal(queued)
	if err != nil {
		t.Fatalf("marshal redis ticket %s: %v", userID, err)
	}
	if err := rdb.Set(ctx, key, aged, queueTicketTTL).Err(); err != nil {
		t.Fatalf("set redis ticket %s: %v", userID, err)
	}
	if err := rdb.ZAdd(ctx, queueJoinedKey(pool, contracts.RulesetMoving), redis.Z{Score: float64(queued.JoinedAtUnixMS), Member: userID}).Err(); err != nil {
		t.Fatalf("age redis joined score %s: %v", userID, err)
	}
}
