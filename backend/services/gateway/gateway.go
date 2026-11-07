package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"geoduels/internal/presence"
	"geoduels/pkg/live"
)

const (
	writeWait  = 8 * time.Second
	pongWait   = 70 * time.Second
	pingPeriod = 20 * time.Second
	readLimit  = 1024
	// A client that falls this far behind is disconnected: it reconnects to a fresh session, so
	// memory stays bounded however slow it is and nothing is lost.
	outboundBuffer = 256
	// How long a closed connection still counts, so reloading a page doesn't flicker offline.
	offlineGrace = 5 * time.Second
	statusEvery  = 10 * time.Second
)

type gateway struct {
	id         string
	api        *apiClient
	bus        *live.Bus
	secret     string
	clients    *clientSet[*client]
	matches    *clientSet[*matchSocket]
	draining   atomic.Bool
	lastStatus atomic.Pointer[string]
}

var upgrader = websocket.Upgrader{CheckOrigin: originAllowed}

// originAllowed keeps other sites from opening sockets with a visitor's cookie.
func originAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	allowed := os.Getenv("CORS_ALLOWED_ORIGINS")
	if allowed == "" {
		allowed = "http://localhost:24000,http://127.0.0.1:24000"
	}
	for _, item := range strings.Split(allowed, ",") {
		item = strings.TrimSpace(item)
		if item == "*" || item == origin {
			return true
		}
	}
	return false
}

func (g *gateway) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, _ *http.Request) {
		if g.draining.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /api/live", g.serveLive)
	mux.HandleFunc("GET /api/live/matches/{id}", g.serveMatch)
	return mux
}

// serveLive is one person's social connection. A visitor without a session gets the same socket,
// and only ever hears the status.
func (g *gateway) serveLive(w http.ResponseWriter, r *http.Request) {
	if g.draining.Load() {
		http.Error(w, "draining", http.StatusServiceUnavailable)
		return
	}
	userID, err := g.api.identify(r.Context(), r.Header.Get("Cookie"))
	if err != nil {
		http.Error(w, "unavailable", http.StatusBadGateway)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := newClient(conn, presence.Connection{ID: uuid.NewString(), UserID: userID})
	defer client.closeNow()

	// Subscribe first and read the session second, so nothing that happens in between is missed.
	if userID != "" {
		subscription, err := g.bus.Subscribe(userID, client.send, client.requestClose)
		if err != nil {
			return
		}
		defer subscription.Close()
	}
	connectCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	session, err := g.api.connect(connectCtx, g.id, client.connection)
	cancel()
	if err != nil {
		return
	}
	defer g.disconnectLater(client.connection)
	g.clients.add(client)
	defer g.clients.remove(client)

	if err := client.write(session); err != nil {
		return
	}
	if status := g.lastStatus.Load(); status != nil {
		client.send([]byte(*status))
	}
	client.run()
}

// disconnectLater reports a closed connection once a reload would have reconnected.
func (g *gateway) disconnectLater(connection presence.Connection) {
	time.AfterFunc(offlineGrace, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := g.api.disconnect(ctx, connection); err != nil {
			// The next heartbeat repairs it.
			log.Printf("disconnect report failed: %v", err)
		}
	})
}

// busReconnected closes every connection: what was published while the bus was away is gone, and
// clients get it all back in the session they reconnect to. They come back a few at a time.
func (g *gateway) busReconnected() {
	log.Printf("bus reconnected; asking clients to reconnect")
	go g.clients.closeAll(shutdownSpread)
}

func (g *gateway) heldConnections() []presence.Connection {
	clients := g.clients.snapshot()
	out := make([]presence.Connection, len(clients))
	for i, c := range clients {
		out[i] = c.connection
	}
	return out
}

// runHeartbeat reports what this gateway holds. It's what repairs anything lost along the way, and
// its absence is how others find out this gateway is gone.
func (g *gateway) runHeartbeat(ctx context.Context) {
	ticker := time.NewTicker(presence.HeartbeatEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			callCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := g.api.heartbeat(callCtx, g.id, g.heldConnections()); err != nil {
				log.Printf("heartbeat failed: %v", err)
			}
			cancel()
		}
	}
}

// runStatus sends the online count and maintenance status to this gateway's clients when they change.
func (g *gateway) runStatus(ctx context.Context) {
	ticker := time.NewTicker(statusEvery)
	defer ticker.Stop()
	for {
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		status, err := g.api.status(callCtx)
		cancel()
		if err == nil {
			text := string(status)
			if last := g.lastStatus.Swap(&text); last == nil || *last != text {
				for _, c := range g.clients.snapshot() {
					c.send(status)
				}
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// client is one socket and what's waiting to be written to it.
type client struct {
	conn       *websocket.Conn
	connection presence.Connection
	outbound   chan []byte
	closing    chan struct{}
	closeOnce  sync.Once
}

func newClient(conn *websocket.Conn, connection presence.Connection) *client {
	return &client{conn: conn, connection: connection, outbound: make(chan []byte, outboundBuffer), closing: make(chan struct{})}
}

// send never blocks: a client too far behind is disconnected.
func (c *client) send(payload []byte) {
	select {
	case c.outbound <- payload:
	default:
		c.requestClose()
	}
}

func (c *client) requestClose() {
	c.closeOnce.Do(func() { close(c.closing) })
}

func (c *client) closeNow() {
	c.requestClose()
	_ = c.conn.Close()
}

func (c *client) write(payload []byte) error {
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(websocket.TextMessage, payload)
}

// run writes until the connection ends. Clients send nothing, so reading only notices that it did.
func (c *client) run() {
	c.conn.SetReadLimit(readLimit)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { return c.conn.SetReadDeadline(time.Now().Add(pongWait)) })
	go func() {
		defer c.requestClose()
		for {
			if _, _, err := c.conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	ping := time.NewTicker(pingPeriod)
	defer ping.Stop()
	for {
		select {
		case <-c.closing:
			message := websocket.FormatCloseMessage(websocket.CloseServiceRestart, "reconnect")
			_ = c.conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))
			return
		case payload := <-c.outbound:
			if err := c.write(payload); err != nil {
				return
			}
		case <-ping.C:
			if err := c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeWait)); err != nil {
				return
			}
		}
	}
}

// closer is anything that can be asked to go away.
type closer interface {
	requestClose()
	comparable
}

// clientSet is the sockets this gateway holds of one kind.
type clientSet[T closer] struct {
	mu      sync.Mutex
	members map[T]struct{}
}

func newClientSet[T closer]() *clientSet[T] { return &clientSet[T]{members: map[T]struct{}{}} }

func (s *clientSet[T]) add(member T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members[member] = struct{}{}
}

func (s *clientSet[T]) remove(member T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.members, member)
}

func (s *clientSet[T]) snapshot() []T {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]T, 0, len(s.members))
	for member := range s.members {
		out = append(out, member)
	}
	return out
}

// closeAll asks everyone to reconnect, spread over the duration so they don't all arrive at once.
func (s *clientSet[T]) closeAll(spread time.Duration) {
	members := s.snapshot()
	if len(members) == 0 {
		return
	}
	pause := spread / time.Duration(len(members))
	for _, member := range members {
		member.requestClose()
		time.Sleep(pause)
	}
}
