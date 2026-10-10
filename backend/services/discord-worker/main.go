package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/riverqueue/river"

	"geoduels/internal/badges"
	"geoduels/internal/content"
	"geoduels/internal/jobs"
	"geoduels/pkg/observability"
	"geoduels/pkg/persistence"
	db "geoduels/pkg/persistence/sqlc/db"
)

const (
	workerDrainTimeout    = 5 * time.Second
	workerShutdownTimeout = 10 * time.Second
)

// discordPersistence is the narrow persistence surface the discord worker
// needs; satisfied by the sqlc-backed persistence store.
type worker struct {
	db       *persistence.DB
	queries  *db.Queries
	badges   badges.Store
	content  content.Store
	jobs     *jobs.Client
	session  *discordgo.Session
	configMu sync.RWMutex
	config   content.DiscordIntegrationSettings
	draining atomic.Bool
	ready    atomic.Bool
}

func main() {
	w, err := newWorker()
	if err != nil {
		log.Fatal(err)
	}
	defer w.close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := w.openDiscord(); err != nil {
		log.Fatal(err)
	}
	w.startConfigRefresh(ctx)
	w.startReconciliation(ctx)
	if err := w.jobs.Start(ctx); err != nil {
		log.Fatal(err)
	}

	r := http.NewServeMux()
	r.HandleFunc("/health/live", w.healthLive)
	r.HandleFunc("/health/ready", w.healthReady)
	r.HandleFunc("/health", w.healthReady)

	addr := getenv("DISCORD_WORKER_ADDR", ":8094")
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	observability.Log("info", "discord worker startup", map[string]any{"addr": addr})
	drained := make(chan struct{})
	go handleWorkerShutdown(w, srv, cancel, drained)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	<-drained
}

func newWorker() (*worker, error) {
	token := strings.TrimSpace(os.Getenv("DISCORD_BOT_TOKEN"))
	if token == "" {
		return nil, errors.New("DISCORD_BOT_TOKEN is required")
	}
	store, err := persistence.NewFromEnv()
	if err != nil {
		return nil, err
	}
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		store.Close()
		return nil, err
	}
	producer, err := jobs.NewClient(store.Pool(), "", nil, nil)
	if err != nil {
		store.Close()
		return nil, err
	}
	contentStore := content.NewPGStore(store.Pool(), producer)
	settings, err := contentStore.GetDiscordIntegrationSettings()
	if err != nil {
		store.Close()
		return nil, err
	}
	w := &worker{
		db:      store,
		queries: db.New(store.Pool()),
		badges:  badges.NewPGStore(store.Pool(), producer),
		content: contentStore,
		config:  settings,
		session: session,
	}
	session.Identify.Intents = discordgo.IntentsGuildMembers | discordgo.IntentsGuildMessages
	session.AddHandler(w.onGuildMemberAdd)
	session.AddHandler(w.onMessageCreate)

	workers := river.NewWorkers()
	river.AddWorker(workers, &discordSyncWorker{w: w})
	river.AddWorker(workers, &discordSyncAllWorker{w: w})
	jobsClient, err := jobs.NewClient(store.Pool(), jobs.QueueDiscord, workers, nil)
	if err != nil {
		store.Close()
		return nil, err
	}
	w.jobs = jobsClient
	return w, nil
}

func (w *worker) openDiscord() error {
	if err := w.session.Open(); err != nil {
		return err
	}
	w.ready.Store(true)
	return nil
}

func (w *worker) close() {
	if w.session != nil {
		_ = w.session.Close()
	}
	if w.db != nil {
		w.db.Close()
	}
}

func (w *worker) onGuildMemberAdd(_ *discordgo.Session, event *discordgo.GuildMemberAdd) {
	config := w.currentConfig()
	if event == nil || event.User == nil || config.GuildID == "" || event.GuildID != config.GuildID {
		return
	}
	w.awardDiscordMemberBadge(event.User.ID, "member_add")
	if err := w.syncRankRoles(event.User.ID); err != nil {
		log.Printf("discord rank role sync failed for %s: %v", event.User.ID, err)
	}
}

func (w *worker) onMessageCreate(_ *discordgo.Session, event *discordgo.MessageCreate) {
	config := w.currentConfig()
	if event == nil || event.Message == nil || event.Author == nil {
		return
	}
	if config.JoinsChannelID == "" || event.GuildID != config.GuildID || event.ChannelID != config.JoinsChannelID {
		return
	}
	if event.Type != discordgo.MessageTypeGuildMemberJoin {
		return
	}
	w.awardDiscordMemberBadge(event.Author.ID, "joins_channel")
	if err := w.syncRankRoles(event.Author.ID); err != nil {
		log.Printf("discord rank role sync failed for %s: %v", event.Author.ID, err)
	}
}

func (w *worker) awardDiscordMemberBadge(discordUserID, source string) {
	if awarded, err := w.badges.AwardDiscordServerMemberByDiscordID(discordUserID); err != nil {
		log.Printf("discord member badge award failed for %s: %v", discordUserID, err)
	} else if awarded {
		observability.Log("info", "discord member badge awarded", map[string]any{"discordUserId": discordUserID, "source": source})
	}
}

func (w *worker) startConfigRefresh(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				settings, err := w.content.GetDiscordIntegrationSettings()
				if err != nil {
					observability.Log("warn", "discord settings refresh failed", map[string]any{"error": err.Error()})
					continue
				}
				w.configMu.Lock()
				w.config = settings
				w.configMu.Unlock()
			}
		}
	}()
}

func (w *worker) startReconciliation(ctx context.Context) {
	go func() {
		var lastRun time.Time
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			config := w.currentConfig()
			interval := time.Duration(config.ReconcileIntervalMinutes) * time.Minute
			if interval <= 0 {
				interval = 15 * time.Minute
			}
			if config.GuildID != "" && (lastRun.IsZero() || time.Since(lastRun) >= interval) {
				w.reconcileMembers(ctx)
				lastRun = time.Now()
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (w *worker) reconcileMembers(ctx context.Context) {
	config := w.currentConfig()
	if config.GuildID == "" {
		return
	}
	after := ""
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		members, err := w.session.GuildMembers(config.GuildID, after, 1000)
		if err != nil {
			log.Printf("discord member reconcile failed: %v", err)
			return
		}
		if len(members) == 0 {
			return
		}
		for _, member := range members {
			if member == nil || member.User == nil {
				continue
			}
			after = member.User.ID
			w.awardDiscordMemberBadge(member.User.ID, "reconcile")
			if err := w.syncRankRoles(member.User.ID); err != nil {
				log.Printf("discord member role reconcile failed for %s: %v", member.User.ID, err)
			}
		}
		if len(members) < 1000 {
			return
		}
	}
}

// discordSyncWorker syncs one Discord identity's managed roles.
type discordSyncWorker struct {
	river.WorkerDefaults[jobs.DiscordSyncArgs]
	w *worker
}

func (x *discordSyncWorker) Work(ctx context.Context, job *river.Job[jobs.DiscordSyncArgs]) error {
	return x.w.processDiscordSync(job.Args.Action, job.Args.DiscordUserID)
}

// discordSyncAllWorker fans a role sync across every linked Discord identity.
type discordSyncAllWorker struct {
	river.WorkerDefaults[jobs.DiscordSyncAllArgs]
	w *worker
}

func (x *discordSyncAllWorker) Work(ctx context.Context, _ *river.Job[jobs.DiscordSyncAllArgs]) error {
	ids, err := x.w.queries.ListAllDiscordIdentityIDs(ctx)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := x.w.processDiscordSync(badges.DiscordSyncActionSync, id); err != nil {
			observability.Log("warn", "discord sync all entry failed", map[string]any{"discord_user_id": id, "error": err.Error()})
		}
	}
	return nil
}

func (w *worker) processDiscordSync(action, discordUserID string) error {
	switch action {
	case badges.DiscordSyncActionCleanupRoles:
		// Cleanup jobs can outlive an unlink/relink sequence. Re-resolve the
		// identity so a stale cleanup cannot remove a newly valid role.
		_, linked, err := w.badges.GetDiscordLinkedUser(discordUserID)
		if err != nil {
			return err
		}
		if discordSyncActionForLinkState(action, linked) == badges.DiscordSyncActionSync {
			return w.syncRankRoles(discordUserID)
		}
		return w.cleanupRankRoles(discordUserID)
	case badges.DiscordSyncActionSync:
		return w.syncDiscordUser(discordUserID)
	default:
		return errors.New("unknown discord sync action")
	}
}

func discordSyncActionForLinkState(requested string, linked bool) string {
	if requested == badges.DiscordSyncActionCleanupRoles && linked {
		return badges.DiscordSyncActionSync
	}
	return requested
}

func (w *worker) syncDiscordUser(discordUserID string) error {
	config := w.currentConfig()
	if config.GuildID == "" {
		return errors.New("discord guild is not configured")
	}
	member, err := w.session.GuildMember(config.GuildID, discordUserID)
	if err != nil {
		if isDiscordNotFound(err) {
			return nil
		}
		return err
	}
	if member == nil || member.User == nil {
		return nil
	}
	w.awardDiscordMemberBadge(discordUserID, "sync")
	return w.syncRankRoles(discordUserID)
}

func (w *worker) syncRankRoles(discordUserID string) error {
	config := w.currentConfig()
	if config.GuildID == "" {
		return errors.New("discord guild is not configured")
	}
	user, ok, err := w.badges.GetDiscordLinkedUser(discordUserID)
	if err != nil {
		return err
	}
	if !ok {
		return w.cleanupRankRoles(discordUserID)
	}
	member, err := w.session.GuildMember(config.GuildID, discordUserID)
	if err != nil {
		if isDiscordNotFound(err) {
			return nil
		}
		return err
	}
	if member == nil {
		return nil
	}
	targetRole := rankRoleForMMR(config, user.HighestEloBadgeMMR)
	return w.applyExclusiveRankRole(config, discordUserID, member.Roles, targetRole)
}

func (w *worker) cleanupRankRoles(discordUserID string) error {
	config := w.currentConfig()
	if config.GuildID == "" {
		return errors.New("discord guild is not configured")
	}
	member, err := w.session.GuildMember(config.GuildID, discordUserID)
	if err != nil {
		if isDiscordNotFound(err) {
			return nil
		}
		return err
	}
	if member == nil {
		return nil
	}
	return w.applyExclusiveRankRole(config, discordUserID, member.Roles, "")
}

func rankRoleForMMR(config content.DiscordIntegrationSettings, mmr int) string {
	switch {
	case mmr >= 2000:
		return config.Elo2000RoleID
	case mmr >= 1500:
		return config.Elo1500RoleID
	case mmr >= 1000:
		return config.Elo1000RoleID
	default:
		return ""
	}
}

func (w *worker) applyExclusiveRankRole(config content.DiscordIntegrationSettings, discordUserID string, currentRoles []string, targetRole string) error {
	current := map[string]bool{}
	for _, roleID := range currentRoles {
		current[roleID] = true
	}
	managedRoleIDs := append([]string{}, config.ManagedRoleIDs...)
	managedRoleIDs = append(managedRoleIDs, config.Elo1000RoleID, config.Elo1500RoleID, config.Elo2000RoleID)
	seenRoleIDs := map[string]bool{}
	for _, roleID := range managedRoleIDs {
		if roleID == "" {
			continue
		}
		if seenRoleIDs[roleID] {
			continue
		}
		seenRoleIDs[roleID] = true
		if roleID == targetRole {
			if !current[roleID] {
				if err := w.session.GuildMemberRoleAdd(config.GuildID, discordUserID, roleID); err != nil {
					return err
				}
			}
			continue
		}
		if current[roleID] {
			if err := w.session.GuildMemberRoleRemove(config.GuildID, discordUserID, roleID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (w *worker) currentConfig() content.DiscordIntegrationSettings {
	w.configMu.RLock()
	defer w.configMu.RUnlock()
	return w.config
}

func isDiscordNotFound(err error) bool {
	var restErr *discordgo.RESTError
	if !errors.As(err, &restErr) || restErr.Response == nil {
		return false
	}
	return restErr.Response.StatusCode == http.StatusNotFound
}

func (w *worker) healthLive(rw http.ResponseWriter, _ *http.Request) {
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte("ok"))
}

func (w *worker) healthReady(rw http.ResponseWriter, _ *http.Request) {
	if w.draining.Load() || !w.ready.Load() {
		http.Error(rw, "not ready", http.StatusServiceUnavailable)
		return
	}
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte("ready"))
}

func handleWorkerShutdown(w *worker, srv *http.Server, cancel context.CancelFunc, drained chan<- struct{}) {
	defer close(drained)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	<-sigCh
	w.draining.Store(true)
	cancel()
	time.Sleep(workerDrainTimeout)

	ctx, shutdownCancel := context.WithTimeout(context.Background(), workerShutdownTimeout)
	defer shutdownCancel()
	if w.jobs != nil {
		if err := w.jobs.Stop(ctx); err != nil {
			log.Printf("discord worker job stop failed: %v", err)
		}
	}
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("discord worker shutdown failed: %v", err)
	}
}

func getenv(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
