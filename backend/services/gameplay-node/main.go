package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"

	"geoduels/internal/envcfg"
	"geoduels/internal/httpx"
	"geoduels/internal/jobs"
	"geoduels/internal/maps"
	"geoduels/internal/matches"
	"geoduels/pkg/contracts"
	"geoduels/pkg/controlplane"
	"geoduels/pkg/duel"
	"geoduels/pkg/maintenance"
	"geoduels/pkg/matchkind"
	"geoduels/pkg/observability"
	"geoduels/pkg/persistence"
	"geoduels/pkg/singleplayer"
)

// A gameplay node runs matches in memory. It holds a lease named after it in control_plane_leases;
// a match it picks up is stamped with the lease's fencing token and stays live while that lease
// holds (gd_match_status). Nothing else tells it what to run: it takes waiting matches itself.
const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 70 * time.Second
	wsPingPeriod = 25 * time.Second

	leaseTTL         = 10 * time.Second
	leaseRenewEvery  = 3 * time.Second
	pickupEvery      = 250 * time.Millisecond
	reconcileGrace   = 5 * time.Second
	maxMatchesOnNode = 2000

	// userHeader names the player the API authenticated before proxying their socket here, and
	// secretHeader proves the request came from the API.
	userHeader   = "X-Geoduels-User"
	secretHeader = "X-Internal-Secret"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

type gameplayNode struct {
	mu sync.RWMutex

	nodeID      string
	internalURL string
	internal    string

	db      *persistence.DB
	persist matches.Store
	maps    *maps.PGStore
	leases  controlplane.LeaseStore
	lease   controlplane.Lease

	plans *roundPlanRegistry

	runtimes     map[matchkind.Engine]gameplayRuntime
	conns        map[string]*websocket.Conn
	connWrite    map[string]*sync.Mutex
	connID       map[string]string
	userMatch    map[string]string
	matchUsers   map[string][]string
	matchKinds   map[string]contracts.MatchKind
	heldSince    map[string]time.Time
	finalizing   map[string]bool
	lastTeamPing map[string]time.Time
	// lastPlayed is when a player last did something in each match: created it, connected, or sent a
	// command other than a heartbeat. A match is in use until its kind's idle end passes without that.
	lastPlayed map[string]time.Time

	metrics *observability.RuntimeMetrics

	readMaintenance func(context.Context) (maintenance.Status, error)
	drain           gameplayDrain
	drainTTL        time.Duration
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
	internalURL := envcfg.Get("GAMEPLAY_INTERNAL_URL", "http://localhost:8091")

	db, err := persistence.NewFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	jobsClient, err := jobs.NewClient(db.Pool(), "", nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	mapStore := maps.NewPGStore(db.Pool())
	// Dev-only: import the bundled sample dataset when a required playable
	// map is missing; production nodes leave DEV_MAP_DATASET unset.
	if datasetPath := envcfg.Get("DEV_MAP_DATASET", ""); datasetPath != "" {
		if err := mapStore.EnsurePlayableMaps(datasetPath); err != nil {
			log.Fatal(err)
		}
	}
	internalSecret := strings.TrimSpace(os.Getenv("COORDINATOR_INTERNAL_SECRET"))
	if internalSecret == "" {
		log.Fatal("COORDINATOR_INTERNAL_SECRET is required")
	}
	leases, err := controlplane.NewPostgresLeaseStore(db.Pool())
	if err != nil {
		log.Fatal(err)
	}

	plans := newRoundPlanRegistry()
	roundForPlan := func(matchID string, roundIndex int) (contracts.LocationPoint, error) {
		return plans.Get(matchID, roundIndex)
	}
	g := &gameplayNode{
		nodeID:      nodeID,
		internalURL: internalURL,
		internal:    internalSecret,
		db:          db,
		persist:     matches.NewPGStore(db.Pool(), jobsClient.EnqueueMatchAnalyze),
		maps:        mapStore,
		leases:      leases,
		plans:       plans,
		runtimes: map[matchkind.Engine]gameplayRuntime{
			matchkind.EngineVersus: versusRuntime{engine: duel.New(roundForPlan)},
			matchkind.EngineSolo:   soloRuntime{engine: singleplayer.New(roundForPlan)},
		},
		conns:        map[string]*websocket.Conn{},
		connWrite:    map[string]*sync.Mutex{},
		connID:       map[string]string{},
		userMatch:    map[string]string{},
		matchUsers:   map[string][]string{},
		matchKinds:   map[string]contracts.MatchKind{},
		heldSince:    map[string]time.Time{},
		finalizing:   map[string]bool{},
		lastTeamPing: map[string]time.Time{},
		lastPlayed:   map[string]time.Time{},
		metrics:      observability.NewRuntimeMetrics(),
		readMaintenance: func(ctx context.Context) (maintenance.Status, error) {
			return maintenance.Read(ctx, db.Pool())
		},
		// Only a backstop: a drain ends once no match is in use.
		drainTTL:   envcfg.Duration("GAMEPLAY_DRAIN_TIMEOUT", 2*time.Hour),
		matchEnded: make(chan struct{}, 1),
	}
	defer g.db.Close()

	g.acquireLease()
	g.refreshMaintenanceDrain()
	go g.leaseLoop()
	go g.pickupLoop()
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
		if code == http.StatusNotFound || code == http.StatusMethodNotAllowed {
			_ = c.NoContent(code)
			return
		}
		e.DefaultHTTPErrorHandler(err, c)
	}
	e.GET("/health", g.healthLive)
	e.GET("/health/live", g.healthLive)
	e.GET("/health/ready", g.healthReady)
	e.GET("/internal/matches/:id/ws", g.ws)
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
	observability.Log("info", "gameplay-node startup", map[string]any{"addr": addr, "nodeId": g.nodeID, "epoch": g.epoch()})
	drained := make(chan struct{})
	go g.handleShutdown(srv, drained)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	<-drained
}

func leaseName(nodeID string) string { return "gameplay-node:" + nodeID }

// acquireLease waits until this process holds the node's lease. A process that died holding it
// keeps it until it expires.
func (g *gameplayNode) acquireLease() {
	owner := g.nodeID + ":" + shortID()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		lease, acquired, err := g.leases.Acquire(ctx, leaseName(g.nodeID), owner, leaseTTL)
		cancel()
		if err == nil && acquired {
			g.mu.Lock()
			g.lease = lease
			g.mu.Unlock()
			return
		}
		observability.Log("warn", "waiting for node lease", map[string]any{"nodeId": g.nodeID, "error": errString(err)})
		time.Sleep(time.Second)
	}
}

func (g *gameplayNode) epoch() int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.lease.Token
}

func (g *gameplayNode) node() matches.Node {
	return matches.Node{ID: g.nodeID, Epoch: g.epoch(), URL: g.internalURL}
}

// leaseLoop renews the lease and reconciles the node's matches with the database. A lost lease
// means every match stamped with it is interrupted: the node lets them go and takes a new one.
func (g *gameplayNode) leaseLoop() {
	ticker := time.NewTicker(leaseRenewEvery)
	defer ticker.Stop()
	for range ticker.C {
		g.refreshMaintenanceDrain()
		g.mu.RLock()
		lease := g.lease
		g.mu.RUnlock()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		renewed, err := g.leases.Renew(ctx, lease, leaseTTL)
		cancel()
		if err != nil {
			g.metrics.OwnershipRenewFailures.Inc()
			continue
		}
		if !renewed {
			observability.Log("error", "node lease lost; its matches are interrupted", map[string]any{"nodeId": g.nodeID})
			g.dropAll()
			g.acquireLease()
			continue
		}
		g.reconcile()
	}
}

// reconcile drops the matches the database says this node no longer runs (replaced, aborted, or
// ended elsewhere) and ends the ones it says the node runs but the node does not have.
func (g *gameplayNode) reconcile() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	open, err := g.persist.NodeOpenMatches(ctx, g.node())
	if err != nil {
		return
	}
	g.mu.RLock()
	gone, missing := reconcilePlan(g.heldSince, g.finalizing, open, time.Now())
	g.mu.RUnlock()
	for _, matchID := range gone {
		observability.Log("info", "match no longer runs here", map[string]any{"matchId": matchID})
		g.release(matchID, "match ended")
	}
	for _, matchID := range missing {
		if _, err := g.persist.EndMatch(ctx, matchID, matches.OutcomeInterrupted); err != nil {
			observability.Log("warn", "ending lost match failed", map[string]any{"matchId": matchID, "error": err.Error()})
		}
	}
}

// reconcilePlan compares the matches a node holds with those the database says it runs. Gone are
// held matches the database no longer gives the node, past a grace that covers a pickup still
// committing and not while being recorded; missing are matches it gives the node that it lacks.
func reconcilePlan(held map[string]time.Time, finalizing map[string]bool, open map[string]bool, now time.Time) (gone, missing []string) {
	for matchID, since := range held {
		if !open[matchID] && !finalizing[matchID] && now.Sub(since) > reconcileGrace {
			gone = append(gone, matchID)
		}
	}
	for matchID := range open {
		if _, ok := held[matchID]; !ok {
			missing = append(missing, matchID)
		}
	}
	sort.Strings(gone)
	sort.Strings(missing)
	return gone, missing
}

// dropAll lets every match go, as when the node's lease was lost.
func (g *gameplayNode) dropAll() {
	g.mu.RLock()
	held := make([]string, 0, len(g.heldSince))
	for matchID := range g.heldSince {
		held = append(held, matchID)
	}
	g.mu.RUnlock()
	for _, matchID := range held {
		g.release(matchID, "match interrupted")
	}
}

// admitting reports whether the node takes new matches.
func (g *gameplayNode) admitting() bool {
	return !g.drain.isDraining() && g.liveMatchCount() < maxMatchesOnNode
}

// pickupLoop takes waiting matches while the node admits them. A busier node waits a little longer
// before each try, so idle nodes take most of them.
func (g *gameplayNode) pickupLoop() {
	for {
		if !g.admitting() {
			time.Sleep(pickupEvery)
			continue
		}
		time.Sleep(min(time.Duration(g.liveMatchCount())*10*time.Millisecond, 500*time.Millisecond))
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		picked, err := g.persist.PickUp(ctx, g.node(), g.startMatch)
		cancel()
		if err != nil {
			observability.Log("warn", "match pickup failed", map[string]any{"error": err.Error()})
			if picked != "" {
				g.forget(picked)
			}
			picked = ""
		}
		if picked == "" {
			time.Sleep(pickupEvery)
		}
	}
}

// startMatch creates a picked-up match in memory. It runs in the pickup's transaction: if it fails,
// the match waits for another node.
func (g *gameplayNode) startMatch(m matches.PickedUp) error {
	spec, ok := matchkind.Lookup(m.Kind)
	if !ok {
		return errors.New("unknown match kind")
	}
	runtime := g.runtimes[spec.Engine]
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rounds, err := g.maps.MatchRounds(ctx, m.MatchID)
	if err != nil {
		return err
	}
	g.plans.Set(m.MatchID, rounds)
	created := newMatch{MatchID: m.MatchID, Kind: m.Kind, Profiles: map[string]contracts.PlayerProfile{}, Teams: m.Spec.SeatTeams(), SeasonID: m.Spec.SeasonID, Config: m.Config}
	for _, seat := range m.Spec.Seats {
		created.Players = append(created.Players, seat.Profile.UserID)
		created.Profiles[seat.Profile.UserID] = seat.Profile
	}
	if err := runtime.CreateMatch(created); err != nil {
		g.plans.Delete(m.MatchID)
		return err
	}
	now := time.Now()
	g.mu.Lock()
	g.matchUsers[m.MatchID] = created.Players
	g.matchKinds[m.MatchID] = m.Kind
	g.heldSince[m.MatchID] = now
	g.lastPlayed[m.MatchID] = now
	g.mu.Unlock()
	return nil
}

func (g *gameplayNode) ws(c echo.Context) error {
	req := c.Request()
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(req.Header.Get(secretHeader))), []byte(g.internal)) != 1 {
		return httpx.PlainTextError(c, http.StatusForbidden, "forbidden")
	}
	matchID := strings.TrimSpace(c.Param("id"))
	userID := strings.TrimSpace(req.Header.Get(userHeader))
	if matchID == "" || userID == "" {
		return httpx.PlainTextError(c, http.StatusBadRequest, "invalid match")
	}
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
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})

	connID := shortID()
	g.bindConnection(userID, matchID, connID, conn, writeMu)
	g.played(matchID)
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

	// Always send the initial snapshot, including for a match that has already ended in memory:
	// without it a reconnect never leaves the client's awaiting-first-snapshot state.
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
	fail := func(code, message string) (contracts.CommandAck, *contracts.MatchSnapshot) {
		status, errorCode = "error", code
		ack.Status, ack.ErrorCode, ack.Message = "error", code, message
		return ack, nil
	}
	runtime, ok := g.runtimeForMatch(matchID)
	if !ok {
		return fail(contracts.ErrMatchNotFound, "match not found")
	}
	if cmd.Type != "ping" && cmd.Type != "session.leave_match" {
		g.played(matchID)
	}
	switch cmd.Type {
	case "ping", "session.leave_match":
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
			return fail(contracts.ErrMatchNotFound, err.Error())
		}
		return ack, snap
	case "team.ping":
		if err := g.sendTeamPing(userID, matchID, strPayload(cmd.Payload, "roundId"), floatPayload(cmd.Payload, "lat"), floatPayload(cmd.Payload, "lng")); err != nil {
			return fail("invalid_team_ping", err.Error())
		}
		return ack, nil
	case "round.advance":
		snap, err := runtime.AdvanceRound(matchID, userID)
		if err != nil {
			return fail(contracts.ErrMatchNotFound, err.Error())
		}
		return ack, snap
	case "match.forfeit":
		snap, err := runtime.Forfeit(matchID, userID)
		if err != nil {
			return fail(contracts.ErrMatchNotFound, err.Error())
		}
		return ack, snap
	default:
		return fail(contracts.ErrMatchNotFound, "unsupported command")
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
			if snap, ok := g.getSnapshot(matchID); ok {
				g.publishRuntimeState(matchID, snap, "")
			}
		}
		if now.Sub(lastRetry) >= finalizationRetryEvery {
			lastRetry = now
			g.retryFinalization(changed)
			g.endIdle(now)
		}
	}
}

const finalizationRetryEvery = 10 * time.Second

// endIdle ends the matches nobody played for their kind's idle end, so an abandoned one does not
// keep its players seated forever.
func (g *gameplayNode) endIdle(now time.Time) {
	type idle struct{ matchID, userID string }
	var ended []idle
	g.mu.RLock()
	for matchID, players := range g.matchUsers {
		after := matchkind.Of(g.matchKinds[matchID]).IdleEnd
		if g.finalizing[matchID] || len(players) == 0 || after <= 0 {
			continue
		}
		if now.Sub(g.lastPlayed[matchID]) >= after {
			ended = append(ended, idle{matchID, players[0]})
		}
	}
	g.mu.RUnlock()
	for _, m := range ended {
		runtime, ok := g.runtimeForMatch(m.matchID)
		if !ok {
			continue
		}
		snap, err := runtime.Abandon(m.matchID, m.userID)
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

// terminalize records an ended match, which ends it in the database and frees its players' seats,
// then lets it go.
func (g *gameplayNode) terminalize(matchID string, snap *contracts.MatchSnapshot) {
	if snap == nil {
		return
	}
	g.mu.Lock()
	if _, ok := g.matchUsers[matchID]; !ok || g.finalizing[matchID] {
		g.mu.Unlock()
		return
	}
	g.finalizing[matchID] = true
	g.mu.Unlock()

	finalized, err := g.persist.FinalizeMatch(*snap)
	if err != nil {
		g.mu.Lock()
		delete(g.finalizing, matchID)
		g.mu.Unlock()
		g.metrics.DBWriteFailures.Inc()
		observability.Log("error", "match finalization failed", map[string]any{"matchId": matchID, "error": err.Error()})
		return
	}
	g.broadcastState(matchID, &finalized, "")
	g.forget(matchID)
}

// release lets a match go that the database no longer says this node runs, telling its players.
func (g *gameplayNode) release(matchID, reason string) {
	g.mu.RLock()
	users := append([]string(nil), g.matchUsers[matchID]...)
	g.mu.RUnlock()
	message := websocket.FormatCloseMessage(websocket.CloseNormalClosure, reason)
	for _, userID := range users {
		g.mu.RLock()
		conn, wm, mine := g.conns[userID], g.connWrite[userID], g.userMatch[userID] == matchID
		g.mu.RUnlock()
		if mine && conn != nil && wm != nil {
			g.safeWriteControl(conn, wm, websocket.CloseMessage, message)
			_ = conn.Close()
		}
	}
	g.forget(matchID)
}

// forget removes a match from the node and its engine.
func (g *gameplayNode) forget(matchID string) {
	runtime, ok := g.runtimeForMatch(matchID)
	g.mu.Lock()
	players := g.matchUsers[matchID]
	delete(g.finalizing, matchID)
	delete(g.matchUsers, matchID)
	delete(g.matchKinds, matchID)
	delete(g.heldSince, matchID)
	delete(g.lastPlayed, matchID)
	for _, userID := range players {
		if g.userMatch[userID] == matchID {
			delete(g.userMatch, userID)
		}
	}
	g.mu.Unlock()
	if ok {
		runtime.Remove(matchID)
	}
	g.plans.Delete(matchID)
	select {
	case g.matchEnded <- struct{}{}:
	default:
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
	kind, ok := g.matchKinds[matchID]
	g.mu.RUnlock()
	if !ok {
		return nil, false
	}
	runtime, ok := g.runtimes[matchkind.Of(kind).Engine]
	return runtime, ok
}

func (g *gameplayNode) getSnapshot(matchID string) (*contracts.MatchSnapshot, bool) {
	runtime, ok := g.runtimeForMatch(matchID)
	if !ok {
		return nil, false
	}
	snap, err := runtime.GetSnapshot(matchID)
	return snap, err == nil
}

// liveMatchCount is the node's load: every match it holds, whatever its kind.
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

// matchesInUse counts the matches someone played within their kind's idle end, and says when the
// first of them turns idle.
func (g *gameplayNode) matchesInUse(now time.Time) (count int, nextIdle time.Time) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	for matchID := range g.matchUsers {
		idleAt := g.lastPlayed[matchID].Add(matchkind.Of(g.matchKinds[matchID]).IdleEnd)
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
	if runtime, ok := g.runtimeForMatch(matchID); ok {
		if snap, err := runtime.MarkDisconnected(matchID, userID); err == nil {
			g.broadcastState(matchID, snap, "")
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
	c.Response().WriteHeader(http.StatusOK)
	_, _ = c.Response().Write([]byte("ready"))
	return nil
}

// handleShutdown drains the node: it takes no new matches, waits until none is in use (the
// deadline is only a backstop), then asks the sockets left to reconnect, gives up its lease so its
// matches read as interrupted at once, and stops.
func (g *gameplayNode) handleShutdown(srv *http.Server, drained chan<- struct{}) {
	defer close(drained)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	<-sigCh
	deadline := g.drain.startShutdown(time.Now()).Add(g.drainTTL)

	// Matches nobody played within their idle end don't hold the drain; they end with the node.
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
	g.mu.RLock()
	lease := g.lease
	g.mu.RUnlock()
	releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := g.leases.Release(releaseCtx, lease); err != nil {
		log.Printf("node lease release failed: %v", err)
	}
	releaseCancel()

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

func shortID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
