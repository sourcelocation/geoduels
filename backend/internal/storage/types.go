package storage

type StorageCleanupResult struct {
	ReplaysCompressed  int64
	ExpiredReplays     int64
	MatchSessions      int64
	MatchPlans         int64
	ChatMessages       int64
	ChatConversations  int64
	AuthSessions       int64
	Parties            int64
	MapUploadEvents    int64
	MapDailyUsers      int64
	UserNotifications  int64
	Leases             int64
	RateLimitWindows   int64
	Presence           int64
	InterruptedMatches int64
	ExpiredParties     int64
}
