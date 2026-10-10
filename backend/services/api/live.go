package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
	"geoduels/pkg/pgnotify"
)

const (
	liveMaxConnsPerUser = 2
	liveWriteWait       = 8 * time.Second
	livePongWait        = 70 * time.Second
	livePingPeriod      = 20 * time.Second
	// Notifications are written by every service, so the hub looks for new ones
	// rather than waiting to be told. Each look reaches back past the last one,
	// because a transaction commits after the created_at it stamped.
	liveAnnouncePeriod   = 2 * time.Second
	liveAnnounceLookback = 30 * time.Second
)

type liveConn struct {
	userID string
	send   chan contracts.LiveEvent
	conn   *websocket.Conn
}

type liveHub struct {
	api       *api
	mu        sync.Mutex
	conns     map[string][]*liveConn
	subs      map[string]context.CancelFunc
	upgrader  websocket.Upgrader
	startOnce sync.Once
	// announced holds the notifications already sent within the lookback, so a
	// look that overlaps the previous one sends nothing twice. Only the announce
	// loop touches it.
	announced map[int64]time.Time
}

func newLiveHub(a *api) *liveHub {
	return &liveHub{
		api:       a,
		conns:     map[string][]*liveConn{},
		subs:      map[string]context.CancelFunc{},
		announced: map[int64]time.Time{},
		upgrader: websocket.Upgrader{
			CheckOrigin: apiWSOriginAllowed,
		},
	}
}

func (h *liveHub) start() {
	if h == nil {
		return
	}
	h.startOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(liveAnnouncePeriod)
			defer ticker.Stop()
			last := time.Now()
			for now := range ticker.C {
				if h.announce(last.Add(-liveAnnounceLookback)) {
					last = now
				}
			}
		}()
	})
}

func (a *api) userLive(c echo.Context) error {
	if a.live == nil {
		a.live = newLiveHub(a)
		a.live.start()
	}
	r := c.Request()
	claims, err := a.liveClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	if a.social != nil {
		isGuest, _, _, err := a.social.GetSocialAccount(r.Context(), claims.Sub)
		if err != nil || isGuest {
			return plainTextError(c, http.StatusForbidden, "registration_required")
		}
	}
	conn, err := a.live.upgrader.Upgrade(c.Response().Writer, r, nil)
	if err != nil {
		return nil
	}
	a.live.serve(r.Context(), claims.Sub, conn)
	return nil
}

func (a *api) liveClaims(r *http.Request) (auth.AppClaims, error) {
	token := strings.TrimSpace(r.URL.Query().Get("accessToken"))
	if token == "" {
		token = strings.TrimSpace(r.URL.Query().Get("access_token"))
	}
	if token == "" {
		authz := r.Header.Get("Authorization")
		if strings.HasPrefix(authz, "Bearer ") {
			token = strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
		}
	}
	if token == "" {
		return auth.AppClaims{}, errMissingRefreshToken
	}
	return auth.ValidateAppAccessToken(a.appAuthSecret, token)
}

func (h *liveHub) serve(ctx context.Context, userID string, conn *websocket.Conn) {
	session := &liveConn{userID: userID, send: make(chan contracts.LiveEvent, 16), conn: conn}
	h.add(session)
	defer h.remove(session)

	_ = conn.SetReadDeadline(time.Now().Add(livePongWait))
	conn.SetReadLimit(1024)
	conn.SetPongHandler(func(string) error {
		h.api.touchViewerPresence(context.Background(), userID)
		return conn.SetReadDeadline(time.Now().Add(livePongWait))
	})
	h.api.touchViewerPresence(context.Background(), userID)
	h.enqueue(session, contracts.LiveEvent{Type: contracts.LiveHello})

	global, stopGlobal := h.api.statusHub().subscribe()
	defer stopGlobal()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
			h.api.touchViewerPresence(context.Background(), userID)
			_ = conn.SetReadDeadline(time.Now().Add(livePongWait))
		}
	}()

	ping := time.NewTicker(livePingPeriod)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case snapshot := <-global:
			h.enqueue(session, contracts.LiveEvent{
				Type: contracts.LiveGlobalStatus,
				Global: &contracts.BootstrapGlobal{
					OnlinePlayers: snapshot.OnlinePlayers,
					Maintenance:   snapshot.Maintenance,
				},
			})
		case event, ok := <-session.send:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(liveWriteWait))
			if err := conn.WriteJSON(event); err != nil {
				return
			}
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(liveWriteWait))
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(liveWriteWait)); err != nil {
				return
			}
		}
	}
}

func (h *liveHub) add(session *liveConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	existing := append([]*liveConn(nil), h.conns[session.userID]...)
	for len(existing) >= liveMaxConnsPerUser {
		oldest := existing[0]
		existing = existing[1:]
		_ = oldest.conn.Close()
	}
	h.conns[session.userID] = append(existing, session)
	if _, ok := h.subs[session.userID]; !ok {
		h.subs[session.userID] = h.subscribeUser(session.userID)
	}
}

func (h *liveHub) remove(session *liveConn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	remaining := h.conns[session.userID][:0]
	for _, item := range h.conns[session.userID] {
		if item != session {
			remaining = append(remaining, item)
		}
	}
	if len(remaining) == 0 {
		delete(h.conns, session.userID)
		if cancel := h.subs[session.userID]; cancel != nil {
			cancel()
			delete(h.subs, session.userID)
		}
	} else {
		h.conns[session.userID] = remaining
	}
	h.dropLocked(session)
}

func (h *liveHub) dropLocked(session *liveConn) {
	select {
	case <-session.send:
	default:
	}
	closeQuiet(session.send)
	_ = session.conn.Close()
}

func closeQuiet(ch chan contracts.LiveEvent) {
	defer func() { _ = recover() }()
	close(ch)
}

func (h *liveHub) enqueue(session *liveConn, event contracts.LiveEvent) {
	select {
	case session.send <- event:
	default:
	}
}

// subscribeUser forwards the user's events, whichever process published them, to their sockets here.
func (h *liveHub) subscribeUser(userID string) context.CancelFunc {
	events, cancel := h.api.events.Subscribe(pgnotify.LiveTopic(userID))
	go func() {
		for message := range events {
			var event contracts.LiveEvent
			if message.Resync || json.Unmarshal(message.Data, &event) != nil || event.Type == "" {
				continue
			}
			h.dispatchLocal(userID, event)
		}
	}()
	return cancel
}

func (h *liveHub) dispatchLocal(userID string, event contracts.LiveEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, session := range h.conns[userID] {
		h.enqueue(session, event)
	}
}

// publish sends the event to the user's sockets in every process.
func (h *liveHub) publish(userID string, event contracts.LiveEvent) {
	if strings.TrimSpace(userID) == "" {
		return
	}
	if err := pgnotify.Publish(context.Background(), h.api.db.Pool(), pgnotify.LiveTopic(userID), event); err != nil {
		h.dispatchLocal(userID, event)
	}
}

// announce sends the people connected here the notifications written for them
// since a moment, whichever service wrote them. It reports whether it looked.
func (h *liveHub) announce(since time.Time) bool {
	if h.api.notificationService == nil {
		return false
	}
	h.mu.Lock()
	users := make([]string, 0, len(h.conns))
	for userID := range h.conns {
		users = append(users, userID)
	}
	h.mu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), liveAnnouncePeriod)
	defer cancel()
	items, err := h.api.notificationService.UnseenSince(ctx, users, since)
	if err != nil {
		return false
	}
	for _, item := range items {
		// Writing a notification again keeps its id and moves its created_at.
		if at, sent := h.announced[item.Notification.ID]; sent && at.Equal(item.Notification.CreatedAt) {
			continue
		}
		h.announced[item.Notification.ID] = item.Notification.CreatedAt
		notification := item.Notification
		h.dispatchLocal(item.UserID, contracts.LiveEvent{Type: contracts.LiveNotificationUpsert, Notification: &notification})
	}
	for id, createdAt := range h.announced {
		if createdAt.Before(since) {
			delete(h.announced, id)
		}
	}
	return true
}

func (h *liveHub) publishInvalidate(userIDs ...string) {
	event := contracts.LiveEvent{Type: contracts.LiveInvalidate, Resources: []string{"friends-page"}}
	seen := map[string]struct{}{}
	for _, userID := range userIDs {
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		h.publish(userID, event)
	}
}

func apiWSOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if raw == "" {
		raw = "http://localhost:3000,http://127.0.0.1:3000"
	}
	for _, item := range strings.Split(raw, ",") {
		allowed := strings.TrimSpace(item)
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}
