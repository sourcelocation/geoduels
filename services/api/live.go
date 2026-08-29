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

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
)

const (
	liveMaxConnsPerUser = 2
	liveWriteWait       = 8 * time.Second
	livePongWait        = 70 * time.Second
	livePingPeriod      = 20 * time.Second
	liveSweepPeriod     = 10 * time.Second
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
}

func newLiveHub(a *api) *liveHub {
	return &liveHub{
		api:   a,
		conns: map[string][]*liveConn{},
		subs:  map[string]context.CancelFunc{},
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
			ticker := time.NewTicker(liveSweepPeriod)
			defer ticker.Stop()
			for range ticker.C {
				h.sweepPresence()
			}
		}()
	})
}

func (a *api) userLive(w http.ResponseWriter, r *http.Request) {
	if a.live == nil {
		a.live = newLiveHub(a)
		a.live.start()
	}
	claims, err := a.liveClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if a.social != nil {
		isGuest, _, _, err := a.social.GetSocialAccount(claims.Sub)
		if err != nil || isGuest {
			http.Error(w, "registration_required", http.StatusForbidden)
			return
		}
	}
	conn, err := a.live.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	a.live.serve(r.Context(), claims.Sub, conn)
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

func (h *liveHub) subscribeUser(userID string) context.CancelFunc {
	if h.api.redis == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(context.Background())
	pubsub := h.api.redis.Subscribe(ctx, liveUserChannel(userID))
	go func() {
		defer pubsub.Close()
		ch := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var event contracts.LiveEvent
				if json.Unmarshal([]byte(msg.Payload), &event) != nil || event.Type == "" {
					continue
				}
				h.dispatchLocal(userID, event)
			}
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

func (h *liveHub) publish(userID string, event contracts.LiveEvent) {
	if strings.TrimSpace(userID) == "" {
		return
	}
	h.dispatchLocal(userID, event)
	if h.api.redis == nil {
		return
	}
	body, err := json.Marshal(event)
	if err != nil {
		return
	}
	_ = h.api.redis.Publish(context.Background(), liveUserChannel(userID), body).Err()
}

func (h *liveHub) publishLatestNotification(userID, notificationType string) {
	if h == nil || h.api.notificationService == nil || userID == "" {
		return
	}
	items, err := h.api.notificationService.List(context.Background(), userID, 10)
	if err != nil {
		return
	}
	for i := range items {
		if items[i].Type == notificationType {
			n := items[i]
			h.publish(userID, contracts.LiveEvent{Type: contracts.LiveNotificationUpsert, Notification: &n})
			return
		}
	}
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

func (h *liveHub) notePresence(ctx context.Context, userID string) {
	if h == nil || userID == "" || h.api.coord == nil {
		return
	}
	status := "online"
	activity := ""
	if assigned, ok, err := h.api.coord.GetAssignmentByUser(ctx, userID); err == nil && ok && assigned.MatchID != "" {
		activity = "in_match"
	}
	if h.api.social != nil {
		if settings, err := h.api.social.GetSocialSettings(userID); err == nil && !settings.PresenceVisible {
			return
		}
	}
	previous, _ := h.presenceState(ctx, userID)
	now := time.Now().UTC()
	next := presenceState{Status: status, Activity: activity, LastSeenAt: now}
	if err := h.writePresenceState(ctx, userID, next); err != nil {
		return
	}
	_ = h.setAnnounced(ctx, userID)
	if previous.Status == next.Status && previous.Activity == next.Activity {
		return
	}
	h.fanoutPresence(userID, contracts.LivePresencePatch{
		UserID:         userID,
		PresenceStatus: status,
		Activity:       activity,
		LastSeenAt:     &now,
	})
}

func (h *liveHub) sweepPresence() {
	if h == nil || h.api.coord == nil || h.api.redis == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if _, err := h.api.coord.CountPresentUsers(ctx); err != nil {
		return
	}
	ids, err := h.api.redis.SMembers(ctx, announcedPresenceKey()).Result()
	if err != nil || len(ids) == 0 {
		return
	}
	present, err := h.api.coord.PresentUsers(ctx, ids)
	if err != nil {
		return
	}
	for _, userID := range ids {
		if present[userID] {
			h.notePresence(ctx, userID)
			continue
		}
		previous, _ := h.presenceState(ctx, userID)
		_ = h.api.redis.SRem(ctx, announcedPresenceKey(), userID).Err()
		_ = h.api.redis.Del(ctx, presenceStateKey(userID)).Err()
		if previous.Status == "offline" || previous.Status == "" {
			continue
		}
		lastSeenAt := previous.LastSeenAt
		if lastSeenAt.IsZero() {
			lastSeenAt = time.Now().UTC()
		}
		h.fanoutPresence(userID, contracts.LivePresencePatch{
			UserID:         userID,
			PresenceStatus: "offline",
			LastSeenAt:     &lastSeenAt,
		})
	}
}

func (h *liveHub) fanoutPresence(userID string, patch contracts.LivePresencePatch) {
	if h.api.social == nil {
		return
	}
	friends, err := h.api.social.ListFriends(userID, 100)
	if err != nil {
		return
	}
	event := contracts.LiveEvent{Type: contracts.LivePresenceEvent, Presence: &patch}
	for _, friend := range friends {
		h.publish(friend.UserID, event)
	}
}

type presenceState struct {
	Status     string    `json:"status"`
	Activity   string    `json:"activity,omitempty"`
	LastSeenAt time.Time `json:"lastSeenAt,omitempty"`
}

func (h *liveHub) presenceState(ctx context.Context, userID string) (presenceState, error) {
	var state presenceState
	if h.api.redis == nil {
		return state, nil
	}
	raw, err := h.api.redis.Get(ctx, presenceStateKey(userID)).Bytes()
	if err != nil {
		return state, err
	}
	_ = json.Unmarshal(raw, &state)
	return state, nil
}

func (h *liveHub) writePresenceState(ctx context.Context, userID string, state presenceState) error {
	if h.api.redis == nil {
		return nil
	}
	body, _ := json.Marshal(state)
	return h.api.redis.Set(ctx, presenceStateKey(userID), body, 2*time.Minute).Err()
}

func (h *liveHub) setAnnounced(ctx context.Context, userID string) error {
	if h.api.redis == nil {
		return nil
	}
	return h.api.redis.SAdd(ctx, announcedPresenceKey(), userID).Err()
}

func liveUserChannel(userID string) string {
	return "live:user:" + userID
}

func announcedPresenceKey() string {
	return "rt:presence:announced"
}

func presenceStateKey(userID string) string {
	return "rt:presence:state:" + userID
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
