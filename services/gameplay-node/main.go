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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"geoduels/pkg/contracts"
	"geoduels/pkg/coordinator"
	"geoduels/pkg/duel"
	"geoduels/pkg/gameticket"
	"geoduels/pkg/locationsampler"
	"geoduels/pkg/matchstore"
	"geoduels/pkg/observability"
	"geoduels/pkg/persistence"
	"geoduels/pkg/singleplayer"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 70 * time.Second
	wsPingPeriod = 25 * time.Second
)

var upgrader = websocket.Upgrader{CheckOrigin: wsOriginAllowed}

type gameplayNode struct {
	mu sync.RWMutex

	nodeID      string
	nodeEpoch   int64
	publicRoute string
	internalURL string

	persist    persistence.Store
	coord      *coordinator.Store
	redis      *redis.Client
	ticketAuth []byte
	coordAuth  string

	samplerCleanup func()
	redisCleanup   func()

	runtimes   map[contracts.MatchMode]gameplayRuntime
	conns      map[string]*websocket.Conn
	connWrite  map[string]*sync.Mutex
	connID     map[string]string
	userMatch  map[string]string
	matchUsers map[string][]string
	matchModes map[string]contracts.MatchMode

	metrics *observability.RuntimeMetrics

	draining atomic.Bool
	drainTTL time.Duration
}

func main() {
	ctx := context.Background()
	nodeID := getenv("GAMEPLAY_NODE_ID", "")
	if nodeID == "" {
		h, _ := os.Hostname()
		if h == "" {
			h = "gameplay"
		}
		nodeID = h + "-" + shortID()
	}
	publicRoute := getenv("GAMEPLAY_PUBLIC_ROUTE", nodeID)
	internalURL := getenv("GAMEPLAY_INTERNAL_URL", "http://localhost:8091")

	rdb, redisCleanup, err := redisFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	samplers := map[string]*locationsampler.Sampler{}
	samplerCleanups := []func(){}
	for _, mapKey := range []string{contracts.MapKeyMoving, contracts.MapKeyNMPZ} {
		sampler, cleanup, err := locationsampler.NewFromEnvForMapKey(ctx, mapKey, locationsampler.Config{})
		if err != nil {
			log.Fatal(err)
		}
		samplers[mapKey] = sampler
		samplerCleanups = append(samplerCleanups, cleanup)
	}
	samplerCleanup := func() {
		for _, cleanup := range samplerCleanups {
			cleanup()
		}
	}
	store, err := persistence.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	singleplayerTTL := getenvDuration("SINGLEPLAYER_SESSION_TTL", 24*time.Hour)
	if err := store.ExpireStaleRuntimeMatches("solo-", singleplayerTTL); err != nil {
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

	duelConfigs := newMatchConfigRegistry()
	singleplayerConfigs := newMatchConfigRegistry()
	roundForConfig := func(matchID string, roundIndex int) (contracts.LocationPoint, error) {
		cfg := duelConfigs.Get(matchID)
		sampler := samplers[cfg.MapKey]
		if sampler == nil {
			sampler = samplers[contracts.MapKeyMoving]
		}
		return sampler.NextRound(context.Background(), matchID, roundIndex)
	}
	roundForSingleplayerConfig := func(matchID string, roundIndex int) (contracts.LocationPoint, error) {
		cfg := singleplayerConfigs.Get(matchID)
		sampler := samplers[cfg.MapKey]
		if sampler == nil {
			sampler = samplers[contracts.MapKeyMoving]
		}
		return sampler.NextRound(context.Background(), matchID, roundIndex)
	}

	g := &gameplayNode{
		nodeID:         nodeID,
		nodeEpoch:      time.Now().UnixNano(),
		publicRoute:    publicRoute,
		internalURL:    internalURL,
		persist:        store,
		coord:          coordinator.NewStore(rdb, getenvDuration("GAMEPLAY_NODE_TTL", 10*time.Second), 2*time.Hour, singleplayerTTL, 5*time.Second),
		redis:          rdb,
		ticketAuth:     ticketSecret,
		coordAuth:      internalSecret,
		samplerCleanup: samplerCleanup,
		redisCleanup:   redisCleanup,
		runtimes: map[contracts.MatchMode]gameplayRuntime{
			contracts.ModeDuel:         duelRuntime{mode: contracts.ModeDuel, engine: duel.New(roundForConfig), configs: duelConfigs},
			contracts.ModeTeamDuel:     duelRuntime{mode: contracts.ModeTeamDuel, engine: duel.New(roundForConfig), configs: duelConfigs},
			contracts.ModeFreeForAll:   duelRuntime{mode: contracts.ModeFreeForAll, engine: duel.New(roundForConfig), configs: duelConfigs},
			contracts.ModeSingleplayer: singleplayerRuntime{engine: singleplayer.New(roundForSingleplayerConfig), configs: singleplayerConfigs},
		},
		conns:      map[string]*websocket.Conn{},
		connWrite:  map[string]*sync.Mutex{},
		connID:     map[string]string{},
		userMatch:  map[string]string{},
		matchUsers: map[string][]string{},
		matchModes: map[string]contracts.MatchMode{},
		metrics:    observability.NewRuntimeMetrics(),
		drainTTL:   getenvDuration("GAMEPLAY_DRAIN_TIMEOUT", 9*time.Minute+30*time.Second),
	}
	defer g.persist.Close()
	defer g.samplerCleanup()
	defer g.redisCleanup()
	defer g.coord.RemoveNode(context.Background(), g.nodeID)

	go g.registerLoop()
	go g.tick()

	r := mux.NewRouter()
	r.HandleFunc("/health", g.healthLive).Methods(http.MethodGet)
	r.HandleFunc("/health/live", g.healthLive).Methods(http.MethodGet)
	r.HandleFunc("/health/ready", g.healthReady).Methods(http.MethodGet)
	r.HandleFunc("/ws/{node}", g.ws).Methods(http.MethodGet)
	r.HandleFunc("/internal/matches", g.createMatch).Methods(http.MethodPost)
	r.HandleFunc("/internal/matches/{id}", g.matchStatus).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/internal/matches/{id}/terminate", g.terminateMatch).Methods(http.MethodPost)
	r.Handle("/metrics", observability.Handler(g.metrics.Registry)).Methods(http.MethodGet)

	addr := getenv("GAMEPLAY_NODE_ADDR", ":8091")
	srv := &http.Server{
		Addr:              addr,
		Handler:           corsMiddleware(r),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	observability.Log("info", "gameplay-node startup", map[string]any{"addr": addr, "nodeId": g.nodeID, "route": g.publicRoute})
	go g.handleShutdown(srv)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func (g *gameplayNode) createMatch(w http.ResponseWriter, req *http.Request) {
	if subtleHeader(req.Header.Get("X-Coordinator-Secret")) != g.coordAuth {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if g.draining.Load() {
		http.Error(w, "draining", http.StatusServiceUnavailable)
		return
	}
	var found contracts.MatchFound
	if err := json.NewDecoder(req.Body).Decode(&found); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	mode := found.Mode
	if mode == "" {
		mode = contracts.ModeDuel
	}
	runtime, ok := g.runtimes[mode]
	if !ok {
		http.Error(w, "unsupported mode", http.StatusBadRequest)
		return
	}
	if found.MatchID == "" || len(found.Players) == 0 {
		http.Error(w, "invalid match", http.StatusBadRequest)
		return
	}
	if _, err := runtime.GetSnapshot(found.MatchID); err == nil {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "exists"})
		return
	}
	found.Config = contracts.NormalizeMatchConfig(found.Config)
	if err := runtime.CreateMatch(found.MatchID, found.Players, found.Profiles, found.Unranked, found.SeasonID, found.Config, found.Teams); err != nil && !strings.Contains(err.Error(), "already exists") {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if err := g.persist.RecordRuntimeMatch(found.MatchID, string(contracts.MatchLive), g.nodeEpoch, false); err != nil {
		http.Error(w, "match persistence unavailable", http.StatusBadGateway)
		return
	}
	g.mu.Lock()
	g.matchUsers[found.MatchID] = append([]string(nil), found.Players...)
	g.matchModes[found.MatchID] = mode
	g.mu.Unlock()
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (g *gameplayNode) matchStatus(w http.ResponseWriter, req *http.Request) {
	if subtleHeader(req.Header.Get("X-Coordinator-Secret")) != g.coordAuth {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	matchID := strings.TrimSpace(mux.Vars(req)["id"])
	if matchID == "" {
		http.Error(w, "invalid match", http.StatusBadRequest)
		return
	}
	if _, ok := g.getSnapshot(matchID); !ok {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (g *gameplayNode) terminateMatch(w http.ResponseWriter, req *http.Request) {
	if subtleHeader(req.Header.Get("X-Coordinator-Secret")) != g.coordAuth {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	matchID := strings.TrimSpace(mux.Vars(req)["id"])
	if matchID == "" {
		http.Error(w, "invalid match", http.StatusBadRequest)
		return
	}
	snap, ok := g.getSnapshot(matchID)
	if !ok {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}
	var payload struct {
		UserID string `json:"userId"`
	}
	if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.UserID) == "" {
		http.Error(w, "invalid user", http.StatusBadRequest)
		return
	}
	if snap.Mode != contracts.ModeSingleplayer {
		http.Error(w, "replacement unsupported", http.StatusConflict)
		return
	}
	runtime, ok := g.runtimeForMatch(matchID)
	if !ok {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}
	nextSnap, err := runtime.Forfeit(matchID, payload.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if nextSnap != nil && nextSnap.State == contracts.MatchEnded {
		g.terminalize(matchID, nextSnap)
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (g *gameplayNode) ws(w http.ResponseWriter, req *http.Request) {
	nodePath := mux.Vars(req)["node"]
	if nodePath == "" || nodePath != g.publicRoute {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	claims, err := gameticket.Validate(g.ticketAuth, strings.TrimSpace(req.URL.Query().Get("ticket")))
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if claims.Node != nodePath {
		http.Error(w, "wrong node", http.StatusUnauthorized)
		return
	}
	matchID := claims.MatchID
	userID := claims.Subject
	runtime, ok := g.runtimeForMatch(matchID)
	if !ok {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}
	snap, err := runtime.MarkResumed(matchID, userID)
	if err != nil {
		snap, err = runtime.GetSnapshot(matchID)
		if err != nil {
			http.Error(w, "match not found", http.StatusNotFound)
			return
		}
		if _, ok := snap.Players[userID]; !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		return
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

	g.writeSnapshotToUser(userID, matchID, snap)
	g.broadcastState(matchID, snap, userID)

	for {
		var cmd contracts.CommandEnvelope
		if err := conn.ReadJSON(&cmd); err != nil {
			return
		}
		if cmd.CommandID == "" {
			cmd.CommandID = shortID()
		}
		ack, nextSnap := g.executeCommand(userID, matchID, cmd)
		if !g.safeWriteConn(conn, writeMu, ack) {
			return
		}
		if nextSnap != nil {
			g.broadcastState(matchID, nextSnap, "")
			if nextSnap.State == contracts.MatchEnded {
				g.terminalize(matchID, nextSnap)
			}
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

func (g *gameplayNode) tick() {
	t := time.NewTicker(1 * time.Second)
	defer t.Stop()
	for range t.C {
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
			g.broadcastState(matchID, snap, "")
			if snap.State == contracts.MatchEnded {
				g.terminalize(matchID, snap)
			}
		}
	}
}

func (g *gameplayNode) terminalize(matchID string, snap *contracts.MatchSnapshot) {
	if snap == nil {
		return
	}

	g.mu.Lock()
	players, ok := g.matchUsers[matchID]
	if !ok {
		g.mu.Unlock()
		return
	}
	delete(g.matchUsers, matchID)
	delete(g.matchModes, matchID)
	for _, userID := range players {
		if g.userMatch[userID] == matchID {
			delete(g.userMatch, userID)
		}
	}
	g.mu.Unlock()

	if snap.Mode == contracts.ModeDuel {
		if err := g.persist.RecordMatchResult(*snap); err != nil {
			log.Printf("record match result failed: %v", err)
		}
	}
	if err := g.persist.RecordRuntimeMatch(matchID, string(contracts.MatchEnded), g.nodeEpoch, true); err != nil {
		log.Printf("record runtime match failed: %v", err)
	}
	if _, err := g.persist.ReopenEndedLobbies(); err != nil {
		log.Printf("reopen ended lobbies failed: %v", err)
	}
	if b, err := json.Marshal(snap); err == nil {
		if err := g.persist.RecordFinalMatchSnapshot(matchID, b); err != nil {
			log.Printf("record final snapshot failed: %v", err)
		}
	}
	g.clearQueuedMatchArtifacts(players)
	if err := g.coord.ClearAssignment(context.Background(), coordinator.Assignment{
		MatchID: matchID,
		Players: players,
	}); err != nil {
		log.Printf("clear assignment failed: %v", err)
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
		err := g.coord.RegisterNode(context.Background(), coordinator.NodeRecord{
			NodeID:        g.nodeID,
			OwnerEpoch:    g.nodeEpoch,
			PublicRoute:   g.publicRoute,
			InternalURL:   g.internalURL,
			ActiveMatches: g.activeMatchCount(),
			Draining:      g.draining.Load(),
		})
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

func (g *gameplayNode) activeMatchCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	total := 0
	for matchID := range g.matchUsers {
		if contracts.IsPrivatePartyMode(g.matchModes[matchID]) {
			total++
		}
	}
	return total
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

func (g *gameplayNode) healthLive(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (g *gameplayNode) healthReady(w http.ResponseWriter, _ *http.Request) {
	if g.draining.Load() {
		http.Error(w, "draining", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := g.redis.Ping(ctx).Err(); err != nil {
		http.Error(w, "redis not ready", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ready"))
}

func (g *gameplayNode) handleShutdown(srv *http.Server) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	<-sigCh
	g.draining.Store(true)
	if err := g.coord.RegisterNode(context.Background(), coordinator.NodeRecord{
		NodeID:        g.nodeID,
		OwnerEpoch:    g.nodeEpoch,
		PublicRoute:   g.publicRoute,
		InternalURL:   g.internalURL,
		ActiveMatches: g.activeMatchCount(),
		Draining:      true,
	}); err != nil {
		log.Printf("node drain registration failed: %v", err)
	}

	deadline := time.Now().Add(g.drainTTL)
	for time.Now().Before(deadline) {
		if g.activeMatchCount() == 0 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown failed: %v", err)
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

func corsMiddleware(next http.Handler) http.Handler {
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
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func wsOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	allowed := allowedOriginsSet()
	return allowed["*"] || allowed[origin]
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
