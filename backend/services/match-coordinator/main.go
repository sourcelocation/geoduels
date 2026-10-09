package main

import (
	"context"
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

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"geoduels/internal/accounts"
	"geoduels/internal/chat"
	"geoduels/internal/envcfg"
	"geoduels/internal/httpx"
	"geoduels/internal/maps"
	"geoduels/internal/matches"
	"geoduels/internal/parties"
	"geoduels/internal/profiles"
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

type chatRestriction = chat.ChatRestriction

var errPartyMapUnavailable = parties.ErrPartyMapUnavailable

type matchCoordinator struct {
	store           matchstore.Store
	state           *coordinator.Store
	db              *persistence.DB
	accounts        *accounts.Service
	profiles        profiles.Store
	matches         matches.Store
	parties         *parties.Service
	chat            chat.Store
	mapsStore       *maps.PGStore
	redis           *redis.Client
	httpClient      *http.Client
	appSecret       []byte
	ticketAuth      []byte
	internal        string
	metrics         *observability.APIMetrics
	draining        atomic.Bool
	matchmakerOwner atomic.Bool
	// Held through a matchmaking tick and while the lease is renewed, taken or handed over, so this
	// replica never runs a tick after giving the lease up.
	matchmakerMu sync.Mutex
	leaseStore   controlplane.LeaseStore
	lease        controlplane.Lease
	leaseClose   func()
	chatMu       sync.Mutex
	chatRecent   map[string][]time.Time
}

var queueUpgrader = websocket.Upgrader{CheckOrigin: httpx.WSOriginAllowed}

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
	pool := persist.Pool()
	mapsStore := maps.NewPGStore(pool)
	matchStore := matches.NewPGStore(pool, nil)
	partyStore := parties.NewPGStore(pool, mapsStore)
	partyService := parties.NewService(partyStore)
	accountsStore := accounts.NewPGStore(pool, nil)
	accountsService := accounts.NewService(accountsStore)
	singleplayerTTL := envcfg.Duration("SINGLEPLAYER_SESSION_TTL", 24*time.Hour)
	if err := partyService.ExpireOpenParties(); err != nil {
		log.Fatal(err)
	}
	if _, err := partyService.ReopenEndedParties(); err != nil {
		log.Fatal(err)
	}
	appSecret, err := envcfg.RequiredSecret("APP_AUTH_SECRET", 32)
	if err != nil {
		log.Fatal(err)
	}
	ticketSecret, err := envcfg.RequiredSecret("GAMEPLAY_TICKET_SECRET", 32)
	if err != nil {
		log.Fatal(err)
	}
	internalSecret := strings.TrimSpace(os.Getenv("COORDINATOR_INTERNAL_SECRET"))
	if internalSecret == "" {
		log.Fatal("COORDINATOR_INTERNAL_SECRET is required")
	}

	q := &matchCoordinator{
		store:      store,
		state:      coordinator.NewStore(rdb, envcfg.Duration("GAMEPLAY_NODE_TTL", 10*time.Second), 2*time.Hour, singleplayerTTL, 5*time.Second),
		db:         persist,
		accounts:   accountsService,
		profiles:   profiles.NewPGStore(pool),
		matches:    matchStore,
		parties:    partyService,
		chat:       chat.NewPGStore(pool),
		mapsStore:  mapsStore,
		redis:      rdb,
		httpClient: &http.Client{Timeout: 3 * time.Second},
		appSecret:  appSecret,
		ticketAuth: ticketSecret,
		internal:   internalSecret,
		metrics:    observability.NewAPIMetrics(),
		chatRecent: map[string][]time.Time{},
	}
	defer q.db.Close()
	defer redisCleanup()
	if err := q.acquireMatchmakerLease(); err != nil {
		log.Fatal(err)
	}
	defer q.releaseMatchmakerLease()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		code := http.StatusInternalServerError
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
		}
		if code == http.StatusNotFound || code == http.StatusMethodNotAllowed {
			_ = c.NoContent(code)
			return
		}
		e.DefaultHTTPErrorHandler(err, c)
	}
	e.Use(httpx.CORS)
	if q.metrics != nil {
		e.Use(q.metrics.EchoMiddleware)
	}
	e.GET("/health", q.healthLive)
	e.GET("/health/live", q.healthLive)
	e.GET("/health/ready", q.healthReady)
	e.GET("/queue", q.queue)
	e.POST("/queue/heartbeat", q.heartbeat)
	e.GET("/queue/online", q.online)
	e.GET("/chat/ws", q.chatWS)
	e.POST("/parties/v2", q.createParty)
	e.POST("/parties/v2/:code/join", q.joinParty)
	e.GET("/parties/v2/:id/ws", q.partyWS)
	if q.metrics != nil {
		e.GET("/metrics", echo.WrapHandler(observability.Handler(q.metrics.Registry)))
	}

	addr := envcfg.Get("MATCH_COORDINATOR_ADDR", envcfg.Get("QUEUE_COORDINATOR_ADDR", ":8090"))
	srv := &http.Server{
		Addr:              addr,
		Handler:           e,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	observability.Log("info", "match-coordinator startup", map[string]any{"addr": addr})
	go q.runPartyCleanupLoop(
		envcfg.Duration("PARTY_CLEANUP_INTERVAL", envcfg.Duration("LOBBY_CLEANUP_INTERVAL", 30*time.Second)),
		envcfg.Duration("PARTY_INACTIVITY_TTL", envcfg.Duration("LOBBY_INACTIVITY_TTL", 5*time.Minute)),
	)
	go q.runMatchmakerLeaseLoop()
	go q.runMatchmakingLoop(
		envcfg.Duration("MATCHMAKING_INTERVAL", 500*time.Millisecond),
		envcfg.Int("MATCHMAKING_BATCH_SIZE", 50),
	)
	drained := make(chan struct{})
	go q.handleShutdown(srv, drained)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	<-drained
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
		q.matchmakerMu.Lock()
		q.runMatchmakingTick(interval, batchSize)
		q.matchmakerMu.Unlock()
	}
}

// runMatchmakingTick matches queued players, on the replica that holds the matchmaker lease only.
func (q *matchCoordinator) runMatchmakingTick(interval time.Duration, batchSize int) {
	if q.draining.Load() || !q.isMatchmakerOwner() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), interval)
	status, err := q.maintenanceStatus(ctx)
	cancel()
	if err == nil && status.QueueBlocked() {
		return
	}
	for _, queue := range matchstore.AllQueueVariants {
		if _, err := q.store.RunMatchmaking(matchstore.QueuePoolRegistered, queue, batchSize); err != nil {
			observability.Log("warn", "matchmaking tick failed", map[string]any{"pool": string(matchstore.QueuePoolRegistered), "queue": string(queue), "error": err.Error()})
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
	ttl := envcfg.Duration("MATCHMAKER_LEASE_TTL", 15*time.Second)
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
	ttl := envcfg.Duration("MATCHMAKER_LEASE_TTL", 15*time.Second)
	interval := ttl / 3
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		if !q.renewOrTakeLease(interval, ttl) {
			return
		}
	}
}

// renewOrTakeLease keeps the matchmaker lease, or takes it once it's free. It reports false once this
// replica is draining: it has handed the lease over and won't take it back.
func (q *matchCoordinator) renewOrTakeLease(interval, ttl time.Duration) bool {
	q.matchmakerMu.Lock()
	defer q.matchmakerMu.Unlock()
	if q.draining.Load() || q.leaseStore == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), interval)
	defer cancel()
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
	return true
}

// handOverLease gives the matchmaker lease up between ticks, so another replica takes it on its next
// check rather than after this one has exited.
func (q *matchCoordinator) handOverLease() {
	q.matchmakerMu.Lock()
	defer q.matchmakerMu.Unlock()
	if q.leaseStore != nil && q.matchmakerOwner.Load() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := q.leaseStore.Release(ctx, q.lease); err != nil {
			observability.Log("warn", "matchmaker lease hand-over failed", map[string]any{"error": err.Error()})
		}
		cancel()
		q.matchmakerOwner.Store(false)
		observability.Log("info", "matchmaker lease handed over", nil)
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

func (q *matchCoordinator) queue(c echo.Context) error {
	r := c.Request()
	if q.draining.Load() {
		return httpx.PlainTextError(c, http.StatusServiceUnavailable, "draining")
	}
	status, err := q.maintenanceStatus(r.Context())
	if err != nil {
		return httpx.PlainTextError(c, http.StatusBadGateway, "queue unavailable")
	}
	if status.QueueBlocked() {
		return httpx.PlainTextError(c, http.StatusServiceUnavailable, maintenanceQueueMessage(status))
	}
	claims, identity, err := q.requireActiveAccount(c)
	if err != nil {
		return err
	}
	if identity.NicknameRequired {
		return httpx.PlainTextError(c, http.StatusForbidden, "nickname required")
	}
	if identity.IsGuest {
		return httpx.PlainTextError(c, http.StatusForbidden, "account required")
	}
	userID := claims.Sub

	conn, err := queueUpgrader.Upgrade(c.Response().Writer, r, nil)
	if err != nil {
		return nil
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
					return nil
				}
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "ACTIVE_MATCH_CONFLICT", "message": "Finish or resume your current duel before queueing again."})
				return nil
			}
			q.clearSupersededAssignment(context.Background(), assigned)
		case matchlaunch.AssignmentPending:
			if contracts.IsPrivatePartyMode(mode) {
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "ACTIVE_MATCH_CONFLICT", "message": "Finish or resume your current duel before queueing again."})
				return nil
			}
			q.clearSupersededAssignment(context.Background(), assigned)
		case matchlaunch.AssignmentAbandoned, matchlaunch.AssignmentInvalid:
			_ = q.state.ClearAssignment(context.Background(), assigned)
		}
	}

	profile, err := q.profiles.GetProfile(userID)
	if err != nil {
		return httpx.PlainTextError(c, http.StatusInternalServerError, "profile unavailable")
	}
	if profile.DisplayName == "" {
		profile.DisplayName = userID
	}
	queuePool := matchstore.QueuePoolRegistered
	selectedQueues := parseQueueVariants(
		c.QueryParam("queues"),
		c.QueryParam("rulesets"),
	)

	if err := q.store.LeaveAllRulesets(queuePool, userID); err != nil {
		return httpx.PlainTextError(c, http.StatusBadGateway, "queue unavailable")
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
			return httpx.PlainTextError(c, http.StatusBadGateway, "queue unavailable")
		}
		if found == nil {
			found = nextFound
		}
	}

	if !q.writeQueueMessage(conn, &writeMu, "queue_status", contracts.QueueStatusEvent{
		Status:   "queued",
		QueuedAt: time.Now().UnixMilli(),
	}) {
		return nil
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
			// Another replica serves the queue; the client reconnects there and queues again.
			q.writeQueueMessage(conn, &writeMu, "queue_reconnect", nil)
			return nil
		}
		if found == nil {
			found, err = q.store.Poll(queuePool, selectedQueues, userID)
			if err != nil {
				observability.Log("warn", "queue poll failed", map[string]any{"userId": userID, "pool": string(queuePool), "queues": selectedQueues, "error": err.Error()})
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "QUEUE_POLL_FAILED", "message": "queue poll failed"})
				return nil
			}
		}
		if found != nil {
			if q.matchOver(found.MatchID) {
				q.clearQueuedMatch(context.Background(), found.Players)
				found = nil
				continue
			}
			rec, err := q.launcher().EnsureAssignment(r.Context(), *found)
			if err != nil {
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "MATCH_ASSIGN_FAILED", "message": err.Error()})
				return nil
			}
			payload, ok, err := q.launcher().AssignedPayload(userID, rec)
			if err != nil || !ok {
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "MATCH_ASSIGN_FAILED", "message": "unable to issue gameplay ticket"})
				return nil
			}
			assigned = true
			q.writeQueueMessage(conn, &writeMu, "match_assigned", payload)
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-pollTicker.C:
		case <-heartbeatTicker.C:
			q.touchPresence(userID)
			status, err := q.store.Heartbeat(queuePool, selectedQueues, userID)
			if err != nil {
				observability.Log("warn", "queue heartbeat failed", map[string]any{"userId": userID, "pool": string(queuePool), "queues": selectedQueues, "error": err.Error()})
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "QUEUE_HEARTBEAT_FAILED", "message": "queue heartbeat failed"})
				return nil
			}
			if status == matchstore.QueuePresenceMissing {
				q.writeQueueMessage(conn, &writeMu, "queue_error", map[string]string{"code": "QUEUE_EXPIRED", "message": "Queue expired. Please re-queue."})
				return nil
			}
		case <-pingTicker.C:
			if !q.writeQueuePing(conn, &writeMu) {
				return nil
			}
		}
	}
}

// A nil lease store is used only by focused handler tests and older embedding
// programs. The production composition root always installs a durable lease.
func (q *matchCoordinator) isMatchmakerOwner() bool {
	return q.leaseStore == nil || q.matchmakerOwner.Load()
}

func (q *matchCoordinator) heartbeat(c echo.Context) error {
	claims, identity, err := q.requireActiveAccount(c)
	if err != nil {
		return err
	}
	if identity.NicknameRequired {
		return httpx.PlainTextError(c, http.StatusForbidden, "nickname required")
	}
	if identity.IsGuest {
		return httpx.PlainTextError(c, http.StatusForbidden, "account required")
	}
	q.touchPresence(claims.Sub)

	status, err := q.store.Heartbeat(matchstore.QueuePoolRegistered, matchstore.AllQueueVariants, claims.Sub)
	if err != nil {
		return httpx.PlainTextError(c, http.StatusBadGateway, "queue unavailable")
	}
	return httpx.JSON(c, http.StatusOK, map[string]string{"status": status})
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

// matchOver reports whether a match already ended or was interrupted.
func (q *matchCoordinator) matchOver(matchID string) bool {
	if matchID == "" {
		return false
	}
	status, err := q.matches.MatchSessionStatus(context.Background(), matchID)
	if err != nil {
		log.Printf("match status lookup failed for %s: %v", matchID, err)
		return false
	}
	return status.Over()
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

func (q *matchCoordinator) online(c echo.Context) error {
	r := c.Request()
	total, err := q.state.CountPresentUsers(r.Context())
	if err != nil {
		return httpx.PlainTextError(c, http.StatusBadGateway, "unavailable")
	}
	status, err := q.maintenanceStatus(r.Context())
	if err != nil {
		return httpx.PlainTextError(c, http.StatusBadGateway, "unavailable")
	}
	resp := map[string]any{"online": total}
	if status.IsVisible() {
		resp["maintenance"] = status
	}
	return httpx.JSON(c, http.StatusOK, resp)
}

func (q *matchCoordinator) touchPresence(userID string) {
	if err := q.state.TouchPresence(context.Background(), userID); err != nil {
		log.Printf("presence touch failed for %s: %v", userID, err)
	}
}

func (q *matchCoordinator) launcher() matchlaunch.Launcher {
	l := matchlaunch.Launcher{
		Coord:          q.state,
		Persist:        q.matches,
		HTTPClient:     q.httpClient,
		TicketSecret:   q.ticketAuth,
		InternalSecret: q.internal,
	}
	if q.mapsStore != nil {
		l.Planner = q.mapsStore
	}
	return l
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

func (q *matchCoordinator) requireActiveAccount(c echo.Context) (auth.AppClaims, accounts.Identity, error) {
	r := c.Request()
	claims, err := q.authenticatedClaims(r)
	if err != nil {
		return auth.AppClaims{}, accounts.Identity{}, httpx.PlainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	identity, err := q.accounts.GetIdentity(claims.Sub)
	if err != nil {
		return auth.AppClaims{}, accounts.Identity{}, httpx.PlainTextError(c, http.StatusUnauthorized, "identity not found")
	}
	if identity.IsBanned {
		return auth.AppClaims{}, accounts.Identity{}, httpx.JSON(c, http.StatusForbidden, map[string]string{"error": "user is banned", "code": "account_banned"})
	}
	return claims, identity, nil
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

func (q *matchCoordinator) healthLive(c echo.Context) error {
	c.Response().WriteHeader(http.StatusOK)
	_, _ = c.Response().Write([]byte("ok"))
	return nil
}

func (q *matchCoordinator) healthReady(c echo.Context) error {
	if q.draining.Load() {
		return httpx.PlainTextError(c, http.StatusServiceUnavailable, "draining")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := q.redis.Ping(ctx).Err(); err != nil {
		return httpx.PlainTextError(c, http.StatusServiceUnavailable, "redis not ready")
	}
	if _, err := q.mapsStore.ResolveGameplayMapID(contracts.ModeDuel, contracts.RulesetMoving, ""); err != nil {
		return httpx.PlainTextError(c, http.StatusServiceUnavailable, "moving map is not configured")
	}
	if _, err := q.mapsStore.ResolveGameplayMapID(contracts.ModeDuel, contracts.RulesetNoMove, ""); err != nil {
		return httpx.PlainTextError(c, http.StatusServiceUnavailable, "no-move map is not configured")
	}
	if _, err := q.mapsStore.ResolveGameplayMapID(contracts.ModeSingleplayer, contracts.RulesetNMPZ, ""); err != nil {
		return httpx.PlainTextError(c, http.StatusServiceUnavailable, "nmpz map is not configured")
	}
	c.Response().WriteHeader(http.StatusOK)
	_, _ = c.Response().Write([]byte("ready"))
	return nil
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

func (q *matchCoordinator) handleShutdown(srv *http.Server, drained chan<- struct{}) {
	defer close(drained)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	<-sigCh
	q.draining.Store(true)
	q.handOverLease()
	time.Sleep(20 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("match-coordinator shutdown failed: %v", err)
	}
}

func redisFromEnv() (*redis.Client, func(), error) {
	url := envcfg.Get("REDIS_URL", "")
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
