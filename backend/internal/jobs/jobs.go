// Package jobs owns background work with River. It holds job args, the client
// wrapper, the transactional enqueuer used by other packages, and the periodic
// schedule. Worker implementations live in the services that own their domain.
package jobs

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// Job kinds.
const (
	KindMatchAnalyze     = "match_analyze"
	KindModerationNotify = "moderation_notify"
	KindDiscordSync      = "discord_sync"
	KindDiscordSyncAll   = "discord_sync_all"
	KindGuestCleanup     = "guest_cleanup"
	KindStorageCleanup   = "storage_cleanup"
	KindSeasonReset      = "season_reset"
	KindCurationSweep    = "curation_sweep"
)

// Queues. A worker client works only its own queue, so it never takes a job
// kind it hasn't registered: the moderation worker works the default queue and
// the Discord worker works QueueDiscord.
const QueueDiscord = "discord"

// MatchAnalyzeArgs asks the integrity detector to evaluate a finished match.
type MatchAnalyzeArgs struct {
	MatchID string `json:"matchId"`
}

func (MatchAnalyzeArgs) Kind() string { return KindMatchAnalyze }

// ModerationNotifyArgs delivers a queued moderation-signal notification.
type ModerationNotifyArgs struct {
	SignalID int64 `json:"signalId"`
}

func (ModerationNotifyArgs) Kind() string { return KindModerationNotify }

// DiscordSyncArgs syncs one Discord identity's managed roles.
type DiscordSyncArgs struct {
	Action        string `json:"action"`
	DiscordUserID string `json:"discordUserId"`
}

func (DiscordSyncArgs) Kind() string { return KindDiscordSync }

func (DiscordSyncArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{Queue: QueueDiscord} }

// DiscordSyncAllArgs fans out a Discord role sync across all linked identities.
type DiscordSyncAllArgs struct{}

func (DiscordSyncAllArgs) Kind() string { return KindDiscordSyncAll }

func (DiscordSyncAllArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{Queue: QueueDiscord} }

// GuestCleanupArgs deletes stale guest accounts.
type GuestCleanupArgs struct{}

func (GuestCleanupArgs) Kind() string { return KindGuestCleanup }

// StorageCleanupArgs prunes stale runtime/storage rows.
type StorageCleanupArgs struct{}

func (StorageCleanupArgs) Kind() string { return KindStorageCleanup }

// SeasonResetArgs applies a due ranked season reset.
type SeasonResetArgs struct{}

func (SeasonResetArgs) Kind() string { return KindSeasonReset }

// CurationSweepArgs closes any due Map-of-the-Week cycle.
type CurationSweepArgs struct{}

func (CurationSweepArgs) Kind() string { return KindCurationSweep }

// Enqueuer is the transactional insert surface other packages depend on.
type Enqueuer interface {
	EnqueueModerationSignal(ctx context.Context, tx pgx.Tx, signalID int64) error
	EnqueueDiscordSync(ctx context.Context, tx pgx.Tx, action, discordUserID string) error
	EnqueueDiscordSyncForUser(ctx context.Context, tx pgx.Tx, userID string) error
	EnqueueDiscordSyncAll(ctx context.Context, tx pgx.Tx) error
}

// Client wraps a River client. It may be insert-only (workers == nil) or a full
// worker client.
type Client struct {
	river *river.Client[pgx.Tx]
	pool  *pgxpool.Pool
}

// NewClient builds a River client. Pass nil workers for an insert-only client;
// a worker client works queue (river.QueueDefault or QueueDiscord) and may
// schedule periodic maintenance jobs.
func NewClient(pool *pgxpool.Pool, queue string, workers *river.Workers, periodic []*river.PeriodicJob) (*Client, error) {
	cfg := &river.Config{
		// Behind PgBouncer in transaction mode LISTEN never hears anything, and River would hold a
		// connection for it forever; poll instead.
		PollOnly: strings.EqualFold(os.Getenv("POSTGRES_PGBOUNCER"), "true"),
	}
	if workers != nil {
		cfg.Workers = workers
		cfg.Queues = map[string]river.QueueConfig{queue: {MaxWorkers: 8}}
	}
	if len(periodic) > 0 {
		cfg.PeriodicJobs = periodic
	}
	rc, err := river.NewClient(riverpgxv5.New(pool), cfg)
	if err != nil {
		return nil, err
	}
	return &Client{river: rc, pool: pool}, nil
}

// Start begins processing jobs (worker clients only).
func (c *Client) Start(ctx context.Context) error { return c.river.Start(ctx) }

// Stop stops processing and waits for active jobs.
func (c *Client) Stop(ctx context.Context) error { return c.river.Stop(ctx) }

func (c *Client) EnqueueMatchAnalyze(ctx context.Context, tx pgx.Tx, matchID string) error {
	_, err := c.river.InsertTx(ctx, tx, MatchAnalyzeArgs{MatchID: matchID}, &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}})
	return err
}

func (c *Client) EnqueueModerationSignal(ctx context.Context, tx pgx.Tx, signalID int64) error {
	_, err := c.river.InsertTx(ctx, tx, ModerationNotifyArgs{SignalID: signalID}, &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}})
	return err
}

func (c *Client) EnqueueDiscordSync(ctx context.Context, tx pgx.Tx, action, discordUserID string) error {
	if discordUserID == "" {
		return nil
	}
	_, err := c.river.InsertTx(ctx, tx, DiscordSyncArgs{Action: action, DiscordUserID: discordUserID}, &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}})
	return err
}

func (c *Client) EnqueueDiscordSyncForUser(ctx context.Context, tx pgx.Tx, userID string) error {
	rows, err := listDiscordIdentities(ctx, tx, userID)
	if err != nil {
		return err
	}
	for _, discordUserID := range rows {
		if err := c.EnqueueDiscordSync(ctx, tx, "sync", discordUserID); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) EnqueueDiscordSyncAll(ctx context.Context, tx pgx.Tx) error {
	_, err := c.river.InsertTx(ctx, tx, DiscordSyncAllArgs{}, &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}})
	return err
}

// PeriodicConfig carries the tunable maintenance intervals.
type PeriodicConfig struct {
	GuestCleanupInterval   time.Duration
	StorageCleanupInterval time.Duration
	SeasonResetInterval    time.Duration
	CurationInterval       time.Duration
}
