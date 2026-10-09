package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"geoduels/internal/storage"
	"geoduels/pkg/persistence"
)

func main() {
	batchSize := flag.Int("batch-size", 1000, "maximum rows processed per cleanup category")
	maxBatches := flag.Int("max-batches", 1, "maximum cleanup batches; use 0 to continue until idle")
	pause := flag.Duration("pause", 100*time.Millisecond, "pause between cleanup batches")
	flag.Parse()

	store, err := persistence.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	maintenance := storage.NewPGStore(store.Pool())

	var total storage.StorageCleanupResult
	for batch := 1; *maxBatches == 0 || batch <= *maxBatches; batch++ {
		result, err := maintenance.CleanupStorage(*batchSize)
		if err != nil {
			log.Fatal(err)
		}
		total = add(total, result)
		log.Printf("storage maintenance batch=%d result=%+v", batch, result)
		if result == (storage.StorageCleanupResult{}) {
			break
		}
		if *pause > 0 {
			time.Sleep(*pause)
		}
	}
	fmt.Printf("storage maintenance total=%+v\n", total)
}

func add(a, b storage.StorageCleanupResult) storage.StorageCleanupResult {
	return storage.StorageCleanupResult{
		ReplaysCompressed: a.ReplaysCompressed + b.ReplaysCompressed,
		ExpiredReplays:    a.ExpiredReplays + b.ExpiredReplays,
		MatchSessions:     a.MatchSessions + b.MatchSessions,
		MatchPlans:        a.MatchPlans + b.MatchPlans,
		ChatMessages:      a.ChatMessages + b.ChatMessages,
		ChatConversations: a.ChatConversations + b.ChatConversations,
		AuthSessions:      a.AuthSessions + b.AuthSessions,
		Parties:           a.Parties + b.Parties,
		MapUploadEvents:   a.MapUploadEvents + b.MapUploadEvents,
		MapDailyUsers:     a.MapDailyUsers + b.MapDailyUsers,
		UserNotifications: a.UserNotifications + b.UserNotifications,
	}
}
