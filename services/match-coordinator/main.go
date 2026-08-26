package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
	"geoduels/pkg/controlplane"
	"geoduels/pkg/coordinator"
	"geoduels/pkg/maintenance"
	"geoduels/pkg/matchlaunch"
	"geoduels/pkg/matchstore"
	"geoduels/pkg/observability"
	"geoduels/pkg/persistence"
	"geoduels/pkg/sessionpolicy"
)

type matchCoordinator struct {
	store           matchstore.Store
	state           *coordinator.Store
	persist         persistence.Store
	redis           *redis.Client
	httpClient      *http.Client
	appSecret       []byte
	ticketAuth      []byte
	internal        string
	metrics         *observability.APIMetrics
	draining        atomic.Bool
	matchmakerOwner atomic.Bool
	leaseStore      controlplane.LeaseStore
	lease           controlplane.Lease
	leaseClose      func()
	chatMu          sync.Mutex
	chatRecent      map[string][]time.Time
}

var queueUpgrader = websocket.Upgrader{CheckOrigin: wsOriginAllowed}

func main() {
	rdb, redisCleanup, err := redisFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	store, err := matchstore.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	persist, err := persistence.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	singleplayerTTL := getenvDuration("SINGLEPLAYER_SESSION_TTL", 24*time.Hour)
	if err := persist.ExpireStaleRuntimeMatches(context.Background(), string(contracts.ModeSingleplayer), singleplayerTTL); err != nil {
		log.Fatal(err)
	}
	if err := persist.ExpireOpenParties(); err != nil {
		log.Fatal(err)
	}
	if _, err := persist.ReopenEndedParties(); err != nil {
		log.Fatal(err)
	}
	appSecret, err := requiredSecret("APP_AUTH_SECRET", 32)
	if err != nil {
		log.Fatal(err)
	}
	ticketSecret, err := requiredSecret("GAMEPLAY_TICKET_SECRET", 32)
	if err != nil {
		log.Fatal(err)
	}
	internalSecret := strings.TrimSpace(os.Getenv("COORDINATOR_INTERNAL_SECRET"))
	if internalSecret == "" {
		log.Fatal("COORDINATOR_INTERNAL_SECRET is required")
	}

	q := &matchCoordinator{
		store:      store,
		state:      coordinator.NewStore(rdb, getenvDuration("GAMEPLAY_NODE_TTL", 10*time.Second), 2*time.Hour, singleplayerTTL, 5*time.Second),
		persist:    persist,
		redis:      rdb,
		httpClient: &http.Client{Timeout: 3 * time.Second},
		appSecret:  appSecret,
		ticketAuth: ticketSecret,
		internal:   internalSecret,
		metrics:    observability.NewAPIMetrics(),
		chatRecent: map[string][]time.Time{},
	}
	defer q.persist.Close()
	defer redisCleanup()
	if err := q.acquireMatchmakerLease(); err != nil {
		log.Fatal(err)
	}
	defer q.releaseMatchmakerLease()

	r := mux.NewRouter()
	r.HandleFunc("/health", q.healthLive).Methods(http.MethodGet)
	r.HandleFunc("/health/live", q.healthLive).Methods(http.MethodGet)
	r.HandleFunc("/health/ready", q.healthReady).Methods(http.MethodGet)
	r.HandleFunc("/queue", q.queue).Methods(http.MethodGet)
	r.HandleFunc("/queue/heartbeat", q.heartbeat).Methods(http.MethodPost)
	r.HandleFunc("/queue/online", q.online).Methods(http.MethodGet)
	r.HandleFunc("/chat/ws", q.chatWS).Methods(http.MethodGet)
	r.HandleFunc("/parties", q.createParty).Methods(http.MethodPost)
	r.HandleFunc("/parties/{id}/ws", q.partyWS).Methods(http.MethodGet)
	r.HandleFunc("/parties/{id}/presence", q.partyPresence).Methods(http.MethodPost)
	r.HandleFunc("/parties/{id}/start", q.startParty).Methods(http.MethodPost)
	r.HandleFunc("/parties/{id}/leave", q.leaveParty).Methods(http.MethodPost)
	r.HandleFunc("/parties/{id}/kick", q.kickPartyMember).Methods(http.MethodPost)
	r.HandleFunc("/parties/{id}/transfer-owner", q.transferPartyOwner).Methods(http.MethodPost)
	r.HandleFunc("/parties/{id}/team", q.updatePartyTeam).Methods(http.MethodPatch)
	r.HandleFunc("/parties/{id}/settings", q.updatePartySettings).Methods(http.MethodPatch)
	r.HandleFunc("/parties/{code}/join", q.joinParty).Methods(http.MethodPost)
	r.HandleFunc("/parties/{code}", q.getParty).Methods(http.MethodGet)
	r.Handle("/metrics", observability.Handler(q.metrics.Registry)).Methods(http.MethodGet)

	addr := getenv("MATCH_COORDINATOR_ADDR", getenv("QUEUE_COORDINATOR_ADDR", ":8090"))
	srv := &http.Server{
		Addr:              addr,
		Handler:           cors(q.metrics.Middleware(r)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	observability.Log("info", "match-coordinator startup", map[string]any{"addr": addr})
	go q.runPartyCleanupLoop(
		getenvDuration("PARTY_CLEANUP_INTERVAL", getenvDuration("LOBBY_CLEANUP_INTERVAL", 30*time.Second)),
		getenvDuration("PARTY_INACTIVITY_TTL", getenvDuration("LOBBY_INACTIVITY_TTL", 5*time.Minute)),
	)
	go q.runMatchmakerLeaseLoop()
	go q.runMatchmakingLoop(
		getenvDuration("MATCHMAKING_INTERVAL", 500*time.Millisecond),
		getenvInt("MATCHMAKING_BATCH_SIZE", 50),
	)
	go q.handleShutdown(srv)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (q *matchCoordinator) runMatchmakingLoop(interval time.Duration, batchSize int) {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if q.draining.Load() || !q.isMatchmakerOwner() {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), interval)
		status, err := q.maintenanceStatus(ctx)
		cancel()
		if err == nil && status.QueueBlocked() {
			continue
		}
		for _, queue := range matchstore.AllQueueVariants {
			if _, err := q.store.RunMatchmaking(matchstore.QueuePoolRegistered, queue, batchSize); err != nil {
				observability.Log("warn", "matchmaking tick failed", map[string]any{"pool": string(matchstore.QueuePoolRegistered), "queue": string(queue), "error": err.Error()})
			}
		}
	}
}

const matchmakerLeaseName = "matchmaker"

func (q *matchCoordinator) acquireMatchmakerLease() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	owner, err := os.Hostname()
	if err != nil || strings.TrimSpace(owner) == "" {
		owner = "match-coordinator"
	}
	owner += ":" + strconv.Itoa(os.Getpid())
	store, closeStore, err := controlplane.OpenPostgresLeaseStore(ctx, os.Getenv("POSTGRES_URL"))
	if err != nil {
		return fmt.Errorf("open matchmaker durable lease: %w", err)
	}
	ttl := getenvDuration("MATCHMAKER_LEASE_TTL", 15*time.Second)
	lease, acquired, err := store.Acquire(ctx, matchmakerLeaseName, owner, ttl)
	if err != nil {
		closeStore()
		return fmt.Errorf("acquire matchmaker durable lease: %w", err)
	}
	q.leaseStore, q.lease, q.leaseClose = store, lease, closeStore
	q.matchmakerOwner.Store(acquired)
	if !acquired {
		observability.Log("warn", "matchmaker standby", map[string]any{"owner": owner})
		return nil
	}
	observability.Log("info", "matchmaker lease acquired", map[string]any{"owner": owner, "fencingToken": lease.Token})
	return nil
}

func (q *matchCoordinator) runMatchmakerLeaseLoop() {
	ttl := getenvDuration("MATCHMAKER_LEASE_TTL", 15*time.Second)
	interval := ttl / 3
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if q.draining.Load() || q.leaseStore == nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), interval)
		if q.matchmakerOwner.Load() {
			renewed, err := q.leaseStore.Renew(ctx, q.lease, ttl)
			if err != nil || !renewed {
				q.matchmakerOwner.Store(false)
				observability.Log("error", "matchmaker lease lost", map[string]any{"error": err})
			}
		} else {
			lease, acquired, err := q.leaseStore.Acquire(ctx, matchmakerLeaseName, q.lease.Owner, ttl)
			if err == nil && acquired {
				q.lease = lease
				q.matchmakerOwner.Store(true)
				observability.Log("info", "matchmaker lease acquired", map[string]any{"fencingToken": lease.Token})
			}
		}
		cancel()
	}
}

func (q *matchCoordinator) releaseMatchmakerLease() {
	if q.leaseStore != nil && q.matchmakerOwner.Load() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = q.leaseStore.Release(ctx, q.lease)
		cancel()
	}
	if q.leaseClose != nil {
		q.leaseClose()
	}
}

func (q *matchCoordinator) queue(w http.ResponseWriter, r *http.Request) {
	if q.draining.Load() {
		http.Error(w, "draining", http.StatusServiceUnavailable)
		return
	}
	if !q.isMatchmakerOwner() {
		http.Error(w, "matchmaker unavailable", http.StatusServiceUnavailable)
		return
	}
	status, err := q.maintenanceStatus(r.Context())
	if err != nil {
		http.Error(w, "queue unavailable", http.StatusBadGateway)
		return
	}
	if status.QueueBlocked() {
		http.Error(w, maintenanceQueueMessage(status), http.StatusServiceUnavailable)
		return
	}
	claims, identity, ok := q.requireActiveAccount(w, r)
	if !ok {
		return
	}
	if identity.NicknameRequired {
		http.Error(w, "nickname required", http.StatusForbidden)
		return
	}
	if identity.AccountType == "guest" {
		http.Error(w, "account required", http.StatusForbidden)
		return
	}
	userID := claims.Sub

	conn, err := queueUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	q.touchPresence(userID)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	conn.SetPongHandler(func(string) error {
		q.touchPresence(userID)
		return conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	})

	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	var writeMu sync.Mutex

	if assigned, ok, err := q.state.GetAssignmentByUser(r.Context(), userID); err == nil && ok {
		mode := sessionpolicy.NormalizeMode(assigned.Mode, assigned.MatchID)
		switch q.launcher().ValidateAssignment(r.Context(), assigned) {
		case matchlaunch.AssignmentValid:
			if contracts.IsPrivatePartyMode(mode) {
				payload, ok, err := q.launcher().AssignedPayload(userID, assigned)
				if err == nil && ok {
					q.writeQueueMessage(conn, &writeMu, "match_assigned", payload)
					return
				}
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "ACTIVE_MATCH_CONFLICT", "message": "Finish or resume your current duel before queueing again."})
				return
			}
			q.clearSupersededAssignment(context.Background(), assigned)
		case matchlaunch.AssignmentPending:
			if contracts.IsPrivatePartyMode(mode) {
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "ACTIVE_MATCH_CONFLICT", "message": "Finish or resume your current duel before queueing again."})
				return
			}
			q.clearSupersededAssignment(context.Background(), assigned)
		case matchlaunch.AssignmentAbandoned, matchlaunch.AssignmentInvalid:
			_ = q.state.ClearAssignment(context.Background(), assigned)
		}
	}

	profile, err := q.persist.GetProfile(userID)
	if err != nil {
		http.Error(w, "profile unavailable", http.StatusInternalServerError)
		return
	}
	if profile.DisplayName == "" {
		profile.DisplayName = userID
	}
	queuePool := matchstore.QueuePoolRegistered
	selectedQueues := parseQueueVariants(
		r.URL.Query().Get("queues"),
		r.URL.Query().Get("rulesets"),
	)

	if err := q.store.LeaveAllRulesets(queuePool, userID); err != nil {
		http.Error(w, "queue unavailable", http.StatusBadGateway)
		return
	}

	var found *contracts.MatchFound
	for _, queue := range selectedQueues {
		_, nextFound, err := q.store.Join(queuePool, queue, contracts.QueueJoinRequest{
			UserID:            userID,
			DisplayName:       profile.DisplayName,
			AvatarURL:         profile.AvatarURL,
			MMR:               profile.MMR,
			RatingRD:          profile.RatingRD,
			SeasonID:          profile.SeasonID,
			RankedGamesPlayed: profile.RankedGamesPlayed,
			IsGuest:           profile.IsGuest,
			IsAdmin:           profile.IsAdmin,
			SelectedBadge:     profile.SelectedBadge,
		})
		if err != nil {
			http.Error(w, "queue unavailable", http.StatusBadGateway)
			return
		}
		if found == nil {
			found = nextFound
		}
	}

	if !q.writeQueueMessage(conn, &writeMu, "queue_status", contracts.QueueStatusEvent{
		Status:   "queued",
		QueuedAt: time.Now().UnixMilli(),
	}) {
		return
	}

	pollTicker := time.NewTicker(500 * time.Millisecond)
	defer pollTicker.Stop()
	heartbeatTicker := time.NewTicker(10 * time.Second)
	defer heartbeatTicker.Stop()
	pingTicker := time.NewTicker(20 * time.Second)
	defer pingTicker.Stop()
	assigned := false
	defer func() {
		if !assigned {
			_ = q.store.Leave(queuePool, selectedQueues, userID)
		}
	}()

	for {
		if q.draining.Load() {
			q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "DRAINING", "message": "Queue server is restarting. Please re-queue."})
			return
		}
		if found == nil {
			found, err = q.store.Poll(queuePool, selectedQueues, userID)
			if err != nil {
				observability.Log("warn", "queue poll failed", map[string]any{"userId": userID, "pool": string(queuePool), "queues": selectedQueues, "error": err.Error()})
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "QUEUE_POLL_FAILED", "message": "queue poll failed"})
				return
			}
		}
		if found != nil {
			if q.matchEnded(found.MatchID) {
				q.clearQueuedMatch(context.Background(), found.Players)
				found = nil
				continue
			}
			rec, err := q.launcher().EnsureAssignment(r.Context(), *found)
			if err != nil {
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "MATCH_ASSIGN_FAILED", "message": err.Error()})
				return
			}
			payload, ok, err := q.launcher().AssignedPayload(userID, rec)
			if err != nil || !ok {
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "MATCH_ASSIGN_FAILED", "message": "unable to issue gameplay ticket"})
				return
			}
			assigned = true
			q.writeQueueMessage(conn, &writeMu, "match_assigned", payload)
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-pollTicker.C:
		case <-heartbeatTicker.C:
			q.touchPresence(userID)
			status, err := q.store.Heartbeat(queuePool, selectedQueues, userID)
			if err != nil {
				observability.Log("warn", "queue heartbeat failed", map[string]any{"userId": userID, "pool": string(queuePool), "queues": selectedQueues, "error": err.Error()})
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "QUEUE_HEARTBEAT_FAILED", "message": "queue heartbeat failed"})
				return
			}
			if status == matchstore.QueuePresenceMissing {
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "QUEUE_EXPIRED", "message": "Queue expired. Please re-queue."})
				return
			}
		case <-pingTicker.C:
			if !q.writeQueuePing(conn, &writeMu) {
				return
			}
		}
	}
}

// A nil lease store is used only by focused handler tests and older embedding
// programs. The production composition root always installs a durable lease.
func (q *matchCoordinator) isMatchmakerOwner() bool {
	return q.leaseStore == nil || q.matchmakerOwner.Load()
}

func (q *matchCoordinator) heartbeat(w http.ResponseWriter, r *http.Request) {
	claims, identity, ok := q.requireActiveAccount(w, r)
	if !ok {
		return
	}
	if identity.NicknameRequired {
		http.Error(w, "nickname required", http.StatusForbidden)
		return
	}
	if identity.AccountType == "guest" {
		http.Error(w, "account required", http.StatusForbidden)
		return
	}
	q.touchPresence(claims.Sub)

	status, err := q.store.Heartbeat(matchstore.QueuePoolRegistered, matchstore.AllQueueVariants, claims.Sub)
	if err != nil {
		http.Error(w, "queue unavailable", http.StatusBadGateway)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"status": status})
}

func parseQueueVariants(rawQueues string, legacyRulesets string) []matchstore.QueueVariant {
	if strings.TrimSpace(rawQueues) == "" {
		return parseLegacyQueueRulesets(legacyRulesets)
	}
	out := []matchstore.QueueVariant{}
	seen := map[matchstore.QueueVariant]bool{}
	for _, part := range strings.Split(rawQueues, ",") {
		queue := matchstore.NormalizeQueueVariant(matchstore.QueueVariant(strings.TrimSpace(strings.ToLower(part))))
		if !matchstore.IsRankedQueueVariant(queue) {
			continue
		}
		if seen[queue] {
			continue
		}
		seen[queue] = true
		out = append(out, queue)
	}
	if len(out) == 0 {
		return []matchstore.QueueVariant{matchstore.QueueMoving}
	}
	return out
}

func parseLegacyQueueRulesets(raw string) []matchstore.QueueVariant {
	if strings.TrimSpace(raw) == "" {
		return []matchstore.QueueVariant{matchstore.QueueMoving}
	}
	out := []matchstore.QueueVariant{}
	seen := map[matchstore.QueueVariant]bool{}
	for range strings.Split(raw, ",") {
		variant := matchstore.QueueMoving
		if !seen[variant] {
			seen[variant] = true
			out = append(out, variant)
		}
	}
	return out
}

func (q *matchCoordinator) matchEnded(matchID string) bool {
	if matchID == "" {
		return false
	}
	rec, ok, err := q.persist.GetRuntimeMatch(context.Background(), matchID)
	if err != nil {
		log.Printf("runtime match lookup failed for %s: %v", matchID, err)
		return false
	}
	return ok && rec.State == string(contracts.MatchEnded)
}

func (q *matchCoordinator) clearQueuedMatch(ctx context.Context, players []string) {
	if q.redis == nil || len(players) == 0 {
		return
	}
	keys := make([]string, 0, len(players))
	for _, userID := range players {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		keys = append(keys, matchstore.QueueMatchKeysForUsers([]string{userID})...)
	}
	if len(keys) == 0 {
		return
	}
	if err := q.redis.Del(ctx, keys...).Err(); err != nil {
		log.Printf("clear queued match failed for %v: %v", players, err)
	}
}

func (q *matchCoordinator) online(w http.ResponseWriter, r *http.Request) {
	total, err := q.state.CountPresentUsers(r.Context())
	if err != nil {
		http.Error(w, "unavailable", http.StatusBadGateway)
		return
	}
	status, err := q.maintenanceStatus(r.Context())
	if err != nil {
		http.Error(w, "unavailable", http.StatusBadGateway)
		return
	}
	resp := map[string]any{"online": total}
	if status.IsVisible() {
		resp["maintenance"] = status
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (q *matchCoordinator) touchPresence(userID string) {
	if err := q.state.TouchPresence(context.Background(), userID); err != nil {
		log.Printf("presence touch failed for %s: %v", userID, err)
	}
}

func (q *matchCoordinator) launcher() matchlaunch.Launcher {
	return matchlaunch.Launcher{
		Coord:          q.state,
		Persist:        q.persist,
		HTTPClient:     q.httpClient,
		TicketSecret:   q.ticketAuth,
		InternalSecret: q.internal,
	}
}

func (q *matchCoordinator) authenticatedClaims(r *http.Request) (auth.AppClaims, error) {
	var claims auth.AppClaims
	var err error
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(authz, "Bearer ") {
		claims, err = auth.ValidateAppAccessToken(q.appSecret, strings.TrimSpace(strings.TrimPrefix(authz, "Bearer ")))
	} else {
		accessToken := strings.TrimSpace(r.URL.Query().Get("accessToken"))
		if accessToken == "" {
			return auth.AppClaims{}, errors.New("missing bearer token")
		}
		claims, err = auth.ValidateAppAccessToken(q.appSecret, accessToken)
	}
	if err != nil {
		return auth.AppClaims{}, err
	}
	return claims, nil
}

func (q *matchCoordinator) requireActiveAccount(w http.ResponseWriter, r *http.Request) (auth.AppClaims, persistence.Identity, bool) {
	claims, err := q.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return auth.AppClaims{}, persistence.Identity{}, false
	}
	identity, err := q.persist.GetIdentity(claims.Sub)
	if err != nil {
		http.Error(w, "identity not found", http.StatusUnauthorized)
		return auth.AppClaims{}, persistence.Identity{}, false
	}
	if identity.IsBanned {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "user is banned", "code": "account_banned"})
		return auth.AppClaims{}, persistence.Identity{}, false
	}
	return claims, identity, true
}

func (q *matchCoordinator) writeQueueMessage(conn *websocket.Conn, writeMu *sync.Mutex, event string, payload any) bool {
	writeMu.Lock()
	defer writeMu.Unlock()
	return conn.WriteJSON(map[string]any{
		"type":    event,
		"payload": payload,
	}) == nil
}

func (q *matchCoordinator) writeQueuePing(conn *websocket.Conn, writeMu *sync.Mutex) bool {
	writeMu.Lock()
	defer writeMu.Unlock()
	return conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)) == nil
}

func (q *matchCoordinator) healthLive(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (q *matchCoordinator) healthReady(w http.ResponseWriter, _ *http.Request) {
	if q.draining.Load() {
		http.Error(w, "draining", http.StatusServiceUnavailable)
		return
	}
	if !q.isMatchmakerOwner() {
		http.Error(w, "matchmaker standby", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := q.redis.Ping(ctx).Err(); err != nil {
		http.Error(w, "redis not ready", http.StatusServiceUnavailable)
		return
	}
	if _, err := q.persist.ResolveGameplayMapID(contracts.ModeDuel, contracts.RulesetMoving, ""); err != nil {
		http.Error(w, "moving map is not configured", http.StatusServiceUnavailable)
		return
	}
	if _, err := q.persist.ResolveGameplayMapID(contracts.ModeDuel, contracts.RulesetNoMove, ""); err != nil {
		http.Error(w, "no-move map is not configured", http.StatusServiceUnavailable)
		return
	}
	if _, err := q.persist.ResolveGameplayMapID(contracts.ModeSingleplayer, contracts.RulesetNMPZ, ""); err != nil {
		http.Error(w, "nmpz map is not configured", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

func (q *matchCoordinator) maintenanceStatus(ctx context.Context) (maintenance.Status, error) {
	readCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return maintenance.Read(readCtx, q.redis)
}

func maintenanceQueueMessage(status maintenance.Status) string {
	if status.Message != "" {
		return status.Message
	}
	switch status.Phase {
	case maintenance.PhaseActive:
		return "Maintenance in progress. Queueing is temporarily unavailable."
	case maintenance.PhaseWarning:
		return "Queueing has been paused for scheduled maintenance."
	default:
		return "Queue unavailable"
	}
}

func (q *matchCoordinator) handleShutdown(srv *http.Server) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	<-sigCh
	q.draining.Store(true)
	time.Sleep(20 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("match-coordinator shutdown failed: %v", err)
	}
}

func redisFromEnv() (*redis.Client, func(), error) {
	url := getenv("REDIS_URL", "")
	if url == "" {
		return nil, nil, errors.New("REDIS_URL is required")
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, nil, err
	}
	rdb := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, nil, err
	}
	return rdb, func() { _ = rdb.Close() }, nil
}

func cors(next http.Handler) http.Handler {
	allowed := allowedOriginsSet()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" && (allowed["*"] || allowed[origin]) {
			if allowed["*"] {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func allowedOriginsSet() map[string]bool {
	raw := getenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")
	out := map[string]bool{}
	for _, s := range strings.Split(raw, ",") {
		origin := strings.TrimSpace(s)
		if origin == "" {
			continue
		}
		out[origin] = true
	}
	return out
}

func wsOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	allowed := allowedOriginsSet()
	return allowed["*"] || allowed[origin]
}

func getenv(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}

func getenvDuration(k string, fallback time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func getenvInt(k string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func requiredSecret(k string, minLen int) ([]byte, error) {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return nil, errors.New(k + " is required")
	}
	if len(v) < minLen {
		return nil, errors.New(k + " must be at least " + strconv.Itoa(minLen) + " characters")
	}
	return []byte(v), nil
}
