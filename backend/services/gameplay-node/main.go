package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"geoduels/internal/envcfg"
	"geoduels/internal/httpx"
	"geoduels/internal/jobs"
	"geoduels/internal/maps"
	"geoduels/internal/matches"
	"geoduels/pkg/contracts"
	"geoduels/pkg/coordinator"
	"geoduels/pkg/duel"
	"geoduels/pkg/gameticket"
	"geoduels/pkg/matchstore"
	"geoduels/pkg/observability"
	"geoduels/pkg/persistence"
	"geoduels/pkg/singleplayer"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 70 * time.Second
	wsPingPeriod = 25 * time.Second
	// A match is live while its lease holds (gd_match_status); see matches.defaultMatchLeaseTTL.
	matchLeaseRenewEvery = 10 * time.Second
	matchLeaseTTL        = 2 * time.Minute
)

var upgrader = websocket.Upgrader{CheckOrigin: httpx.WSOriginAllowed}

// gameplayPersistence is the narrow persistence surface the gameplay node
// needs; satisfied by the sqlc-backed persistence store.
type gameplayNode struct {
	mu sync.RWMutex

	nodeID      string
	nodeEpoch   int64
	publicRoute string
	internalURL string

	db         *persistence.DB
	persist    matches.Store
	coord      *coordinator.Store
	redis      *redis.Client
	ticketAuth []byte
	coordAuth  string

	redisCleanup func()
	plans        *roundPlanRegistry

	runtimes     map[contracts.MatchMode]gameplayRuntime
	conns        map[string]*websocket.Conn
	connWrite    map[string]*sync.Mutex
	connID       map[string]string
	userMatch    map[string]string
	matchUsers   map[string][]string
	matchModes   map[string]contracts.MatchMode
	finalizing   map[string]bool
	lastTeamPing map[string]time.Time
	// lastPlayed is when a player last did something in each match: created it, connected, or sent a
	// command other than a heartbeat. A match is in use until idleAfter passes without that.
	lastPlayed map[string]time.Time
	idleAfter  time.Duration

	metrics *observability.RuntimeMetrics

	drain    gameplayDrain
	drainTTL time.Duration
	// matchEnded wakes a drain when a match leaves the node.
	matchEnded chan struct{}
}

func main() {
	nodeID := envcfg.Get("GAMEPLAY_NODE_ID", "")
	if nodeID == "" {
		h, _ := os.Hostname()
		if h == "" {
			h = "gameplay"
		}
		nodeID = h + "-" + shortID()
	}
	publicRoute := envcfg.Get("GAMEPLAY_PUBLIC_ROUTE", nodeID)
	internalURL := envcfg.Get("GAMEPLAY_INTERNAL_URL", "http://localhost:8091")

	rdb, redisCleanup, err := redisFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	db, err := persistence.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	jobsClient, err := jobs.NewClient(db.Pool(), "", nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	store := matches.NewPGStore(db.Pool(), jobsClient.EnqueueMatchAnalyze)
	mapStore := maps.NewPGStore(db.Pool())
	// Dev-only: import the bundled sample dataset when a required playable
	// map is missing; production nodes leave DEV_MAP_DATASET unset.
	if datasetPath := envcfg.Get("DEV_MAP_DATASET", ""); datasetPath != "" {
		if err := mapStore.EnsurePlayableMaps(datasetPath); err != nil {
			log.Fatal(err)
		}
	}
	singleplayerTTL := envcfg.Duration("SINGLEPLAYER_SESSION_TTL", 24*time.Hour)
	ticketSecret, err := envcfg.RequiredSecret("GAMEPLAY_TICKET_SECRET", 32)
	if err != nil {
		log.Fatal(err)
	}
	internalSecret := strings.TrimSpace(os.Getenv("COORDINATOR_INTERNAL_SECRET"))
	if internalSecret == "" {
		log.Fatal("COORDINATOR_INTERNAL_SECRET is required")
	}

	duelConfigs := newMatchConfigRegistry()
	plans := newRoundPlanRegistry()
	roundForPlan := func(matchID string, roundIndex int) (contracts.LocationPoint, error) {
		return plans.Get(matchID, roundIndex)
	}

	g := &gameplayNode{
		nodeID:       nodeID,
		nodeEpoch:    time.Now().UnixNano(),
		publicRoute:  publicRoute,
		internalURL:  internalURL,
		db:           db,
		persist:      store,
		coord:        coordinator.NewStore(rdb, envcfg.Duration("GAMEPLAY_NODE_TTL", 10*time.Second), 2*time.Hour, singleplayerTTL, 5*time.Second),
		redis:        rdb,
		ticketAuth:   ticketSecret,
		coordAuth:    internalSecret,
		redisCleanup: redisCleanup,
		plans:        plans,
		runtimes: map[contracts.MatchMode]gameplayRuntime{
			contracts.ModeDuel:         duelRuntime{mode: contracts.ModeDuel, engine: duel.New(roundForPlan), configs: duelConfigs},
			contracts.ModeTeamDuel:     duelRuntime{mode: contracts.ModeTeamDuel, engine: duel.New(roundForPlan), configs: duelConfigs},
			contracts.ModeFreeForAll:   duelRuntime{mode: contracts.ModeFreeForAll, engine: duel.New(roundForPlan), configs: duelConfigs},
			contracts.ModeSingleplayer: singleplayerRuntime{engine: singleplayer.New(roundForPlan)},
		},
		conns:        map[string]*websocket.Conn{},
		connWrite:    map[string]*sync.Mutex{},
		connID:       map[string]string{},
		userMatch:    map[string]string{},
		matchUsers:   map[string][]string{},
		matchModes:   map[string]contracts.MatchMode{},
		finalizing:   map[string]bool{},
		lastTeamPing: map[string]time.Time{},
		lastPlayed:   map[string]time.Time{},
		idleAfter:    envcfg.Duration("GAMEPLAY_IDLE_AFTER", 15*time.Minute),
		metrics:      observability.NewRuntimeMetrics(),
		// Only a backstop: a drain ends once no match is in use.
		drainTTL:   envcfg.Duration("GAMEPLAY_DRAIN_TIMEOUT", 2*time.Hour),
		matchEnded: make(chan struct{}, 1),
	}
	defer g.db.Close()
	defer g.redisCleanup()
	defer g.coord.RemoveNode(context.Background(), g.nodeID)

	g.refreshMaintenanceDrain()
	go g.registerLoop()
	go g.matchLeaseLoop()
	go g.tick()

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
		// gorilla/mux wrote empty bodies for unrouted paths and methods.
		if code == http.StatusNotFound || code == http.StatusMethodNotAllowed {
			_ = c.NoContent(code)
			return
		}
		e.DefaultHTTPErrorHandler(err, c)
	}
	e.Use(httpx.CORS)
	e.GET("/health", g.healthLive)
	e.GET("/health/live", g.healthLive)
	e.GET("/health/ready", g.healthReady)
	e.GET("/ws/:node", g.ws)
	e.POST("/internal/matches", g.createMatch)
	e.GET("/internal/matches/:id", g.matchStatus)
	e.HEAD("/internal/matches/:id", g.matchStatus)
	e.POST("/internal/matches/:id/terminate", g.terminateMatch)
	e.GET("/metrics", echo.WrapHandler(observability.Handler(g.metrics.Registry)))

	addr := envcfg.Get("GAMEPLAY_NODE_ADDR", ":8091")
	srv := &http.Server{
		Addr:              addr,
		Handler:           e,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	observability.Log("info", "gameplay-node startup", map[string]any{"addr": addr, "nodeId": g.nodeID, "route": g.publicRoute})
	drained := make(chan struct{})
	go g.handleShutdown(srv, drained)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	<-drained
}

func (g *gameplayNode) createMatch(c echo.Context) error {
	req := c.Request()
	if subtleHeader(req.Header.Get("X-Coordinator-Secret")) != g.coordAuth {
		return httpx.PlainTextError(c, http.StatusForbidden, "forbidden")
	}
	if g.drain.isDraining() {
		return httpx.PlainTextError(c, http.StatusServiceUnavailable, "draining")
	}
	var found contracts.MatchFound
	if err := json.NewDecoder(req.Body).Decode(&found); err != nil {
		return httpx.PlainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	mode := found.Mode
	if mode == "" {
		mode = contracts.ModeDuel
	}
	runtime, ok := g.runtimes[mode]
	if !ok {
		return httpx.PlainTextError(c, http.StatusBadRequest, "unsupported mode")
	}
	if found.MatchID == "" || len(found.Players) == 0 {
		return httpx.PlainTextError(c, http.StatusBadRequest, "invalid match")
	}
	for _, playerID := range found.Players {
		if strings.TrimSpace(playerID) == "" {
			return httpx.PlainTextError(c, http.StatusBadRequest, "invalid match: blank player id")
		}
	}
	if len(found.PlannedRounds) == 0 || found.ResolvedMap.MapID == "" {
		return httpx.PlainTextError(c, http.StatusBadRequest, "match has no resolved round plan")
	}
	if _, err := runtime.GetSnapshot(found.MatchID); err == nil {
		return httpx.JSON(c, http.StatusOK, map[string]string{"status": "exists"})
	}
	found.Config = contracts.NormalizeMatchConfig(found.Config)
	g.plans.Set(found.MatchID, found.PlannedRounds)
	if err := runtime.CreateMatch(found.MatchID, found.Players, found.Profiles, found.Unranked, found.SeasonID, found.Config, found.Teams); err != nil && !strings.Contains(err.Error(), "already exists") {
		return httpx.PlainTextError(c, http.StatusBadGateway, err.Error())
	}
	g.mu.Lock()
	g.matchUsers[found.MatchID] = append([]string(nil), found.Players...)
	g.matchModes[found.MatchID] = mode
	g.lastPlayed[found.MatchID] = time.Now()
	g.mu.Unlock()
	return httpx.JSON(c, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *gameplayNode) matchStatus(c echo.Context) error {
	if subtleHeader(c.Request().Header.Get("X-Coordinator-Secret")) != g.coordAuth {
		return httpx.PlainTextError(c, http.StatusForbidden, "forbidden")
	}
	matchID := strings.TrimSpace(c.Param("id"))
	if matchID == "" {
		return httpx.PlainTextError(c, http.StatusBadRequest, "invalid match")
	}
	if _, ok := g.getSnapshot(matchID); !ok {
		return httpx.PlainTextError(c, http.StatusNotFound, "match not found")
	}
	c.Response().WriteHeader(http.StatusOK)
	return nil
}

func (g *gameplayNode) terminateMatch(c echo.Context) error {
	if subtleHeader(c.Request().Header.Get("X-Coordinator-Secret")) != g.coordAuth {
		return httpx.PlainTextError(c, http.StatusForbidden, "forbidden")
	}
	matchID := strings.TrimSpace(c.Param("id"))
	if matchID == "" {
		return httpx.PlainTextError(c, http.StatusBadRequest, "invalid match")
	}
	snap, ok := g.getSnapshot(matchID)
	if !ok {
		return httpx.PlainTextError(c, http.StatusNotFound, "match not found")
	}
	var payload struct {
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&payload); err != nil {
		return httpx.PlainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	if strings.TrimSpace(payload.UserID) == "" {
		return httpx.PlainTextError(c, http.StatusBadRequest, "invalid user")
	}
	if snap.Mode != contracts.ModeSingleplayer {
		return httpx.PlainTextError(c, http.StatusConflict, "replacement unsupported")
	}
	runtime, ok := g.runtimeForMatch(matchID)
	if !ok {
		return httpx.PlainTextError(c, http.StatusNotFound, "match not found")
	}
	nextSnap, err := runtime.Forfeit(matchID, payload.UserID)
	if err != nil {
		return httpx.PlainTextError(c, http.StatusBadGateway, err.Error())
	}
	if nextSnap != nil && nextSnap.State == contracts.MatchEnded {
		g.terminalize(matchID, nextSnap)
	}
	return httpx.JSON(c, http.StatusOK, map[string]string{"status": "ok"})
}

func (g *gameplayNode) ws(c echo.Context) error {
	req := c.Request()
	nodePath := c.Param("node")
	if nodePath == "" || nodePath != g.publicRoute {
		return httpx.PlainTextError(c, http.StatusNotFound, "not found")
	}
	claims, err := gameticket.Validate(g.ticketAuth, strings.TrimSpace(c.QueryParam("ticket")))
	if err != nil {
		return httpx.PlainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	if claims.Node != nodePath {
		return httpx.PlainTextError(c, http.StatusUnauthorized, "wrong node")
	}
	matchID := claims.MatchID
	userID := claims.Subject
	runtime, ok := g.runtimeForMatch(matchID)
	if !ok {
		return httpx.PlainTextError(c, http.StatusNotFound, "match not found")
	}
	snap, err := runtime.MarkResumed(matchID, userID)
	if err != nil {
		snap, err = runtime.GetSnapshot(matchID)
		if err != nil {
			return httpx.PlainTextError(c, http.StatusNotFound, "match not found")
		}
		if _, ok := snap.Players[userID]; !ok {
			return httpx.PlainTextError(c, http.StatusForbidden, "forbidden")
		}
	}

	conn, err := upgrader.Upgrade(c.Response().Writer, req, nil)
	if err != nil {
		return nil
	}
	defer conn.Close()

	writeMu := &sync.Mutex{}
	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		g.touchPresence(userID)
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	connID := shortID()
	g.bindConnection(userID, matchID, connID, conn, writeMu)
	g.played(matchID)
	g.touchPresence(userID)
	defer g.onDisconnect(userID, matchID, connID)

	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(wsPingPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if !g.safeWriteControl(conn, writeMu, websocket.PingMessage, nil) {
					_ = conn.Close()
					return
				}
			}
		}
	}()

	// Always send the initial snapshot, including for a match that has already
	// ended in memory. Persistence can fail independently of the authoritative
	// runtime; without this snapshot a reconnect receives ping acknowledgements
	// but never leaves the client's awaiting-first-snapshot state.
	g.writeSnapshotToUser(userID, matchID, snap)
	g.publishRuntimeState(matchID, snap, userID)

	for {
		var cmd contracts.CommandEnvelope
		if err := conn.ReadJSON(&cmd); err != nil {
			return nil
		}
		if cmd.CommandID == "" {
			cmd.CommandID = shortID()
		}
		ack, nextSnap := g.executeCommand(userID, matchID, cmd)
		if !g.safeWriteConn(conn, writeMu, ack) {
			return nil
		}
		if nextSnap != nil {
			g.publishRuntimeState(matchID, nextSnap, "")
		}
	}
}

func (g *gameplayNode) executeCommand(userID, matchID string, cmd contracts.CommandEnvelope) (contracts.CommandAck, *contracts.MatchSnapshot) {
	start := time.Now()
	ack := contracts.CommandAck{Kind: "ack", CommandID: cmd.CommandID, Status: "ok", ServerTS: time.Now().UnixMilli()}
	status := "ok"
	errorCode := "none"
	defer func() {
		g.metrics.CommandLatencySeconds.WithLabelValues(cmd.Type).Observe(time.Since(start).Seconds())
		g.metrics.CommandTotal.WithLabelValues(cmd.Type, status, errorCode).Inc()
	}()
	runtime, ok := g.runtimeForMatch(matchID)
	if !ok {
		status = "error"
		errorCode = contracts.ErrMatchNotFound
		ack.Status = "error"
		ack.ErrorCode = contracts.ErrMatchNotFound
		ack.Message = "match not found"
		return ack, nil
	}

	if cmd.Type != "ping" && cmd.Type != "session.leave_match" {
		g.played(matchID)
	}
	switch cmd.Type {
	case "ping":
		return ack, nil
	case "guess.place", "guess.finalize":
		snap, err := runtime.SubmitGuess(contracts.GuessPayload{
			UserID:         userID,
			MatchID:        matchID,
			RoundID:        strPayload(cmd.Payload, "roundId"),
			Lat:            floatPayload(cmd.Payload, "lat"),
			Lng:            floatPayload(cmd.Payload, "lng"),
			IdempotencyKey: cmd.CommandID,
			Finalize:       cmd.Type == "guess.finalize",
		})
		if err != nil {
			status = "error"
			errorCode = contracts.ErrMatchNotFound
			ack.Status = "error"
			ack.ErrorCode = contracts.ErrMatchNotFound
			ack.Message = err.Error()
			return ack, nil
		}
		return ack, snap
	case "team.ping":
		if err := g.sendTeamPing(userID, matchID, strPayload(cmd.Payload, "roundId"), floatPayload(cmd.Payload, "lat"), floatPayload(cmd.Payload, "lng")); err != nil {
			status, errorCode = "error", "invalid_team_ping"
			ack.Status, ack.ErrorCode, ack.Message = "error", errorCode, err.Error()
		}
		return ack, nil
	case "round.advance":
		snap, err := runtime.AdvanceRound(matchID, userID)
		if err != nil {
			status = "error"
			errorCode = contracts.ErrMatchNotFound
			ack.Status = "error"
			ack.ErrorCode = contracts.ErrMatchNotFound
			ack.Message = err.Error()
			return ack, nil
		}
		return ack, snap
	case "match.forfeit":
		snap, err := runtime.Forfeit(matchID, userID)
		if err != nil {
			status = "error"
			errorCode = contracts.ErrMatchNotFound
			ack.Status = "error"
			ack.ErrorCode = contracts.ErrMatchNotFound
			ack.Message = err.Error()
			return ack, nil
		}
		return ack, snap
	case "session.leave_match":
		return ack, nil
	default:
		status = "error"
		errorCode = contracts.ErrMatchNotFound
		ack.Status = "error"
		ack.ErrorCode = contracts.ErrMatchNotFound
		ack.Message = "unsupported command"
		return ack, nil
	}
}

func (g *gameplayNode) sendTeamPing(userID, matchID, roundID string, lat, lng float64) error {
	runtime, ok := g.runtimeForMatch(matchID)
	if !ok {
		return errors.New("match not found")
	}
	snap, err := runtime.GetSnapshot(matchID)
	if err != nil {
		return err
	}
	self, ok := snap.Players[userID]
	if snap.Mode != contracts.ModeTeamDuel || !ok || self.TeamID == "" {
		return errors.New("team ping is unavailable")
	}
	if snap.Phase != contracts.PhaseLive || snap.RoundPhase != contracts.RoundPhaseLive || snap.CurrentRound == nil || snap.CurrentRound.RoundID != roundID {
		return errors.New("round is not live")
	}
	if !self.Finalized {
		return errors.New("finalize your guess before pinging")
	}
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return errors.New("invalid coordinates")
	}
	now := time.Now()
	g.mu.Lock()
	if last := g.lastTeamPing[userID]; !last.IsZero() && now.Sub(last) < 500*time.Millisecond {
		g.mu.Unlock()
		return errors.New("pinging too quickly")
	}
	g.lastTeamPing[userID] = now
	users := append([]string(nil), g.matchUsers[matchID]...)
	g.mu.Unlock()
	payload := map[string]any{"id": "ping-" + shortID(), "roundId": roundID, "senderUserId": userID, "lat": lat, "lng": lng, "expiresAt": now.Add(5 * time.Second).UnixMilli()}
	for _, targetID := range users {
		if target, exists := snap.Players[targetID]; exists && target.TeamID == self.TeamID {
			_ = g.safeWriteMatch(targetID, matchID, contracts.EventEnvelope{Kind: "event", EventID: "evt-" + shortID(), Type: contracts.EventTeamPing, MatchID: matchID, Seq: snap.EventSequence, ServerTS: now.UnixMilli(), Payload: payload})
		}
	}
	return nil
}

func (g *gameplayNode) tick() {
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	lastRetry := time.Now()
	for now := range t.C {
		changed := map[string]bool{}
		for _, runtime := range g.runtimes {
			for _, matchID := range runtime.Tick() {
				changed[matchID] = true
			}
		}
		for matchID := range changed {
			runtime, ok := g.runtimeForMatch(matchID)
			if !ok {
				continue
			}
			snap, err := runtime.GetSnapshot(matchID)
			if err != nil {
				continue
			}
			g.publishRuntimeState(matchID, snap, "")
		}
		if now.Sub(lastRetry) >= finalizationRetryEvery {
			lastRetry = now
			g.retryFinalization(changed)
			g.endIdleSolo(now)
		}
	}
}

const finalizationRetryEvery = 10 * time.Second

// endIdleSolo ends singleplayer games nobody played for idleAfter. A solo game has no clock of its
// own, so without this an abandoned one would stay on the node until it restarts.
func (g *gameplayNode) endIdleSolo(now time.Time) {
	type idle struct{ matchID, userID string }
	var ended []idle
	g.mu.RLock()
	for matchID, players := range g.matchUsers {
		if g.matchModes[matchID] != contracts.ModeSingleplayer || g.finalizing[matchID] || len(players) == 0 {
			continue
		}
		if now.Sub(g.lastPlayed[matchID]) >= g.idleAfter {
			ended = append(ended, idle{matchID, players[0]})
		}
	}
	g.mu.RUnlock()
	for _, m := range ended {
		runtime, ok := g.runtimeForMatch(m.matchID)
		if !ok {
			continue
		}
		snap, err := runtime.Forfeit(m.matchID, m.userID)
		if err != nil {
			continue
		}
		g.publishRuntimeState(m.matchID, snap, "")
	}
}

// retryFinalization finalizes ended matches again whose last attempt failed: an ended match stops
// changing, so nothing else would, and it would stay on the node (and hold a drain) forever.
func (g *gameplayNode) retryFinalization(justPublished map[string]bool) {
	g.mu.RLock()
	pending := make([]string, 0)
	for matchID := range g.matchUsers {
		if !justPublished[matchID] && !g.finalizing[matchID] {
			pending = append(pending, matchID)
		}
	}
	g.mu.RUnlock()
	for _, matchID := range pending {
		if snap, ok := g.getSnapshot(matchID); ok && snap.State == contracts.MatchEnded {
			g.terminalize(matchID, snap)
		}
	}
}

func (g *gameplayNode) publishRuntimeState(matchID string, snap *contracts.MatchSnapshot, excludeUserID string) {
	if snap == nil {
		return
	}
	if snap.State == contracts.MatchEnded {
		g.terminalize(matchID, snap)
		return
	}
	g.broadcastState(matchID, snap, excludeUserID)
}

func (g *gameplayNode) terminalize(matchID string, snap *contracts.MatchSnapshot) {
	if snap == nil {
		return
	}

	g.mu.Lock()
	players, ok := g.matchUsers[matchID]
	if !ok || g.finalizing[matchID] {
		g.mu.Unlock()
		return
	}
	players = append([]string(nil), players...)
	g.finalizing[matchID] = true
	g.mu.Unlock()

	finalized, err := g.persist.FinalizeMatch(*snap)
	if err != nil {
		g.mu.Lock()
		delete(g.finalizing, matchID)
		g.mu.Unlock()
		g.metrics.DBWriteFailures.Inc()
		observability.Log("error", "match finalization failed", map[string]any{
			"matchId": matchID,
			"error":   err.Error(),
		})
		return
	}
	g.broadcastState(matchID, &finalized, "")

	g.mu.Lock()
	delete(g.finalizing, matchID)
	delete(g.matchUsers, matchID)
	delete(g.matchModes, matchID)
	delete(g.lastPlayed, matchID)
	for _, userID := range players {
		if g.userMatch[userID] == matchID {
			delete(g.userMatch, userID)
		}
	}
	g.mu.Unlock()
	select {
	case g.matchEnded <- struct{}{}:
	default:
	}

	g.clearQueuedMatchArtifacts(players)
	if err := g.coord.ClearAssignment(context.Background(), coordinator.Assignment{
		MatchID: matchID,
		Players: players,
	}); err != nil {
		log.Printf("clear assignment failed: %v", err)
	}
}

func (g *gameplayNode) matchLeaseLoop() {
	ticker := time.NewTicker(matchLeaseRenewEvery)
	defer ticker.Stop()
	for {
		g.mu.RLock()
		matchIDs := make([]string, 0, len(g.matchUsers))
		for matchID := range g.matchUsers {
			matchIDs = append(matchIDs, matchID)
		}
		g.mu.RUnlock()
		if len(matchIDs) > 0 {
			if err := g.persist.RenewMatchSessionLeases(g.nodeID, g.nodeEpoch, matchIDs, matchLeaseTTL); err != nil {
				g.metrics.DBWriteFailures.Inc()
			}
		}
		<-ticker.C
	}
}

func (g *gameplayNode) clearQueuedMatchArtifacts(players []string) {
	if len(players) == 0 {
		return
	}
	keys := make([]string, 0, len(players)*2)
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
	if err := g.redis.Del(context.Background(), keys...).Err(); err != nil {
		log.Printf("clear queued match artifacts failed: %v", err)
	}
}

func (g *gameplayNode) registerLoop() {
	t := time.NewTicker(3 * time.Second)
	defer t.Stop()
	for {
		g.refreshMaintenanceDrain()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := g.coord.RegisterNode(ctx, coordinator.NodeRecord{
			NodeID:        g.nodeID,
			OwnerEpoch:    g.nodeEpoch,
			PublicRoute:   g.publicRoute,
			InternalURL:   g.internalURL,
			ActiveMatches: g.liveMatchCount(),
			Draining:      g.drain.isDraining(),
		})
		cancel()
		if err != nil {
			log.Printf("node registration failed: %v", err)
		}
		<-t.C
	}
}

func (g *gameplayNode) bindConnection(userID, matchID, connID string, conn *websocket.Conn, writeMu *sync.Mutex) {
	g.mu.Lock()
	previous := g.conns[userID]
	g.conns[userID] = conn
	g.connWrite[userID] = writeMu
	g.connID[userID] = connID
	g.userMatch[userID] = matchID
	g.metrics.ConnectedUsers.Set(float64(len(g.conns)))
	g.mu.Unlock()

	if previous != nil && previous != conn {
		_ = previous.Close()
	}
}

func (g *gameplayNode) runtimeForMatch(matchID string) (gameplayRuntime, bool) {
	g.mu.RLock()
	mode := g.matchModes[matchID]
	g.mu.RUnlock()
	if mode == "" {
		mode = contracts.ModeDuel
	}
	runtime, ok := g.runtimes[mode]
	return runtime, ok
}

func (g *gameplayNode) getSnapshot(matchID string) (*contracts.MatchSnapshot, bool) {
	g.mu.RLock()
	mode := g.matchModes[matchID]
	g.mu.RUnlock()
	if mode != "" {
		if runtime, ok := g.runtimes[mode]; ok {
			snap, err := runtime.GetSnapshot(matchID)
			return snap, err == nil
		}
	}
	for _, runtime := range g.runtimes {
		snap, err := runtime.GetSnapshot(matchID)
		if err == nil {
			return snap, true
		}
	}
	return nil, false
}

// liveMatchCount is the node's load: every match it holds, whatever its mode.
func (g *gameplayNode) liveMatchCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return len(g.matchUsers)
}

func (g *gameplayNode) played(matchID string) {
	g.mu.Lock()
	if _, ok := g.matchUsers[matchID]; ok {
		g.lastPlayed[matchID] = time.Now()
	}
	g.mu.Unlock()
}

// matchesInUse counts the matches someone played within idleAfter, and says when the first of them
// turns idle.
func (g *gameplayNode) matchesInUse(now time.Time) (count int, nextIdle time.Time) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for matchID := range g.matchUsers {
		idleAt := g.lastPlayed[matchID].Add(g.idleAfter)
		if !now.Before(idleAt) {
			continue
		}
		count++
		if nextIdle.IsZero() || idleAt.Before(nextIdle) {
			nextIdle = idleAt
		}
	}
	return count, nextIdle
}

func (g *gameplayNode) touchPresence(userID string) {
	if err := g.coord.TouchPresence(context.Background(), userID); err != nil {
		log.Printf("presence touch failed for %s: %v", userID, err)
	}
}

func (g *gameplayNode) onDisconnect(userID, matchID, connID string) {
	g.mu.Lock()
	if currentConnID := g.connID[userID]; currentConnID != "" && currentConnID != connID {
		g.mu.Unlock()
		return
	}
	delete(g.conns, userID)
	delete(g.connWrite, userID)
	delete(g.connID, userID)
	g.metrics.ConnectedUsers.Set(float64(len(g.conns)))
	g.mu.Unlock()
	if matchID != "" {
		if runtime, ok := g.runtimeForMatch(matchID); ok {
			if snap, err := runtime.MarkDisconnected(matchID, userID); err == nil {
				g.broadcastState(matchID, snap, "")
			}
		}
	}
}

func (g *gameplayNode) broadcastState(matchID string, snap *contracts.MatchSnapshot, excludeUserID string) {
	if snap == nil {
		return
	}
	g.mu.RLock()
	users := append([]string(nil), g.matchUsers[matchID]...)
	g.mu.RUnlock()
	for _, userID := range users {
		if userID == excludeUserID {
			continue
		}
		evt := contracts.EventEnvelope{
			Kind:     "event",
			EventID:  "evt-" + shortID(),
			Type:     contracts.EventMatchState,
			MatchID:  matchID,
			Seq:      snap.EventSequence,
			ServerTS: time.Now().UnixMilli(),
			Payload:  contracts.ClientSnapshotForPlayer(snap, userID),
		}
		_ = g.safeWriteMatch(userID, matchID, evt)
	}
}

func (g *gameplayNode) writeSnapshotToUser(userID, matchID string, snap *contracts.MatchSnapshot) {
	if snap == nil {
		return
	}
	evt := contracts.EventEnvelope{
		Kind:     "event",
		EventID:  "evt-" + shortID(),
		Type:     contracts.EventMatchSnapshot,
		MatchID:  matchID,
		Seq:      snap.EventSequence,
		ServerTS: time.Now().UnixMilli(),
		Payload:  contracts.ClientSnapshotForPlayer(snap, userID),
	}
	_ = g.safeWriteMatch(userID, matchID, evt)
}

func (g *gameplayNode) safeWriteMatch(userID, matchID string, payload any) bool {
	g.mu.RLock()
	if g.userMatch[userID] != matchID {
		g.mu.RUnlock()
		return false
	}
	conn := g.conns[userID]
	wm := g.connWrite[userID]
	g.mu.RUnlock()
	if conn == nil || wm == nil {
		return false
	}
	return g.safeWriteConn(conn, wm, payload)
}

func (g *gameplayNode) safeWriteConn(conn *websocket.Conn, wm *sync.Mutex, payload any) bool {
	wm.Lock()
	defer wm.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	if err := conn.WriteJSON(payload); err != nil {
		_ = conn.Close()
		return false
	}
	return true
}

func (g *gameplayNode) safeWriteControl(conn *websocket.Conn, wm *sync.Mutex, messageType int, payload []byte) bool {
	wm.Lock()
	defer wm.Unlock()
	_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
	return conn.WriteControl(messageType, payload, time.Now().Add(wsWriteWait)) == nil
}

func (g *gameplayNode) healthLive(c echo.Context) error {
	c.Response().WriteHeader(http.StatusOK)
	_, _ = c.Response().Write([]byte("ok"))
	return nil
}

func (g *gameplayNode) healthReady(c echo.Context) error {
	// Maintenance drains match admission, not pod readiness: the ordered
	// StatefulSet rollout must be able to replace nodes during maintenance.
	if g.drain.isStopping() {
		return httpx.PlainTextError(c, http.StatusServiceUnavailable, "draining")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := g.redis.Ping(ctx).Err(); err != nil {
		return httpx.PlainTextError(c, http.StatusServiceUnavailable, "redis not ready")
	}
	c.Response().WriteHeader(http.StatusOK)
	_, _ = c.Response().Write([]byte("ready"))
	return nil
}

// handleShutdown drains the node: it takes no new matches, waits until none is in use (the
// deadline is only a backstop), then asks the sockets left to reconnect and stops.
func (g *gameplayNode) handleShutdown(srv *http.Server, drained chan<- struct{}) {
	defer close(drained)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	<-sigCh
	deadline := g.drain.startShutdown(time.Now()).Add(g.drainTTL)
	registerCtx, registerCancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := g.coord.RegisterNode(registerCtx, coordinator.NodeRecord{
		NodeID:        g.nodeID,
		OwnerEpoch:    g.nodeEpoch,
		PublicRoute:   g.publicRoute,
		InternalURL:   g.internalURL,
		ActiveMatches: g.liveMatchCount(),
		Draining:      true,
	}); err != nil {
		log.Printf("node drain registration failed: %v", err)
	}
	registerCancel()

	// Matches nobody played for idleAfter don't hold the drain; they end with the node.
	backstop := time.NewTimer(time.Until(deadline))
	defer backstop.Stop()
wait:
	for {
		inUse, nextIdle := g.matchesInUse(time.Now())
		if inUse == 0 {
			break
		}
		turnsIdle := time.NewTimer(time.Until(nextIdle))
		select {
		case <-g.matchEnded:
		case <-turnsIdle.C:
		case <-backstop.C:
			turnsIdle.Stop()
			observability.Log("warn", "drain deadline passed with matches in use", map[string]any{
				"nodeId":  g.nodeID,
				"matches": inUse,
			})
			break wait
		}
		turnsIdle.Stop()
	}
	if left := g.liveMatchCount(); left > 0 {
		observability.Log("info", "drain leaves idle matches to end with the node", map[string]any{"nodeId": g.nodeID, "matches": left})
	}
	g.closeConnections()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown failed: %v", err)
	}
}

// closeConnections asks every socket still open to reconnect: Shutdown doesn't close WebSockets.
func (g *gameplayNode) closeConnections() {
	g.mu.RLock()
	conns := make(map[*websocket.Conn]*sync.Mutex, len(g.conns))
	for userID, conn := range g.conns {
		conns[conn] = g.connWrite[userID]
	}
	g.mu.RUnlock()
	message := websocket.FormatCloseMessage(websocket.CloseServiceRestart, "restarting")
	for conn, wm := range conns {
		g.safeWriteControl(conn, wm, websocket.CloseMessage, message)
		_ = conn.Close()
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

func shortID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func subtleHeader(value string) string {
	return strings.TrimSpace(value)
}

func strPayload(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func floatPayload(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case string:
		f, err := strconv.ParseFloat(t, 64)
		if err != nil {
			return 0
		}
		return f
	default:
		return 0
	}
}
