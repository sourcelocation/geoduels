package main

import (
	"flag"
	"fmt"
	"log"
	"time"

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
	maintenance, ok := store.(persistence.StorageMaintenance)
	if !ok {
		log.Fatal("configured persistence store does not support storage maintenance")
	}

	var total persistence.StorageCleanupResult
	for batch := 1; *maxBatches == 0 || batch <= *maxBatches; batch++ {
		result, err := maintenance.CleanupStorage(*batchSize)
		if err != nil {
			log.Fatal(err)
		}
		total = add(total, result)
		log.Printf("storage maintenance batch=%d result=%+v", batch, result)
		if result == (persistence.StorageCleanupResult{}) {
			break
		}
		if *pause > 0 {
			time.Sleep(*pause)
		}
	}
	fmt.Printf("storage maintenance total=%+v\n", total)
}

func add(a, b persistence.StorageCleanupResult) persistence.StorageCleanupResult {
	return persistence.StorageCleanupResult{
		ReplaysCompressed:  a.ReplaysCompressed + b.ReplaysCompressed,
		ExpiredReplays:     a.ExpiredReplays + b.ExpiredReplays,
		RuntimeMatches:     a.RuntimeMatches + b.RuntimeMatches,
		MatchSessions:      a.MatchSessions + b.MatchSessions,
		MatchPlans:         a.MatchPlans + b.MatchPlans,
		ChatMessages:       a.ChatMessages + b.ChatMessages,
		ChatConversations:  a.ChatConversations + b.ChatConversations,
		AuthSessions:       a.AuthSessions + b.AuthSessions,
		Parties:            a.Parties + b.Parties,
		MapUploadEvents:    a.MapUploadEvents + b.MapUploadEvents,
		MapDailyUsers:      a.MapDailyUsers + b.MapDailyUsers,
		UserNotifications:  a.UserNotifications + b.UserNotifications,
		NotificationOutbox: a.NotificationOutbox + b.NotificationOutbox,
		DiscordSyncOutbox:  a.DiscordSyncOutbox + b.DiscordSyncOutbox,
	}
}
