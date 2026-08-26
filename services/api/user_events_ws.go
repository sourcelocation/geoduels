package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"geoduels/pkg/auth"
	"geoduels/pkg/persistence"
)

var userEventsUpgrader = websocket.Upgrader{
	CheckOrigin: userEventsOriginAllowed,
}

func userEventsOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" || sameOriginRequest(r, origin) {
		return true
	}
	allowed := allowedOriginsSet()
	return allowed["*"] || allowed[origin]
}

func sameOriginRequest(r *http.Request, origin string) bool {
	parsed, err := url.Parse(origin)
	return err == nil && strings.EqualFold(parsed.Host, r.Host)
}

func (a *api) userEventsWS(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("access_token"))
	claims, err := auth.ValidateAppAccessToken(a.appAuthSecret, token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	store, ok := a.socialStore()
	if !ok {
		http.Error(w, "events unavailable", http.StatusNotImplemented)
		return
	}
	isGuest, _, _, err := store.GetSocialAccount(claims.Sub)
	if err != nil {
		http.Error(w, "events unavailable", http.StatusForbidden)
		return
	}
	if allowed, retry, err := a.allowSocialAction(r, claims.Sub, "socket_connect"); err != nil || !allowed {
		writeSocialRateLimited(w, retry)
		return
	}
	conn, err := userEventsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	connectionID := socialConnectionID()
	a.touchGlobalCountPresence(claims.Sub)
	if !isGuest {
		a.setSocialPresence(claims.Sub, connectionID, "online")
		a.publishFriendPresence(store, claims.Sub, "online")
		defer func() {
			if a.clearSocialPresence(claims.Sub, connectionID) {
				_ = store.TouchLastSeen(claims.Sub, time.Now())
				a.publishFriendPresence(store, claims.Sub, "offline")
			}
		}()
	}
	statusEvents, unsubscribeStatus := a.statusHub().subscribe()
	defer unsubscribeStatus()

	_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))
		a.touchGlobalCountPresence(claims.Sub)
		a.setSocialPresence(claims.Sub, connectionID, "online")
		return nil
	})
	clientMessages := make(chan map[string]any, 4)
	go func() {
		defer close(clientMessages)
		for {
			var message map[string]any
			if conn.ReadJSON(&message) != nil {
				return
			}
			clientMessages <- message
		}
	}()

	poll := time.NewTicker(time.Second)
	ping := time.NewTicker(25 * time.Second)
	defer poll.Stop()
	defer ping.Stop()
	for {
		select {
		case message, open := <-clientMessages:
			if !open {
				return
			}
			if !isGuest && message["type"] == "presence" {
				status, _ := message["status"].(string)
				if status != "away" {
					status = "online"
				}
				a.setSocialPresence(claims.Sub, connectionID, status)
			}
		case <-poll.C:
			if isGuest {
				continue
			}
			events, err := store.ListUserEvents(claims.Sub, after, 100)
			if err != nil {
				_ = conn.WriteJSON(map[string]any{"type": "resync_required"})
				continue
			}
			for _, event := range events {
				envelope := map[string]any{
					"sequence": event.Sequence, "type": event.Type, "occurredAt": event.OccurredAt,
				}
				var data any
				if json.Unmarshal(event.Payload, &data) == nil {
					envelope["data"] = data
				}
				if conn.WriteJSON(envelope) != nil {
					return
				}
				after = event.Sequence
			}
		case status, open := <-statusEvents:
			if !open {
				return
			}
			if conn.WriteJSON(map[string]any{
				"type": "global_status.changed",
				"data": status,
			}) != nil {
				return
			}
		case <-ping.C:
			if conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)) != nil {
				return
			}
		}
	}
}

func (a *api) touchGlobalCountPresence(userID string) {
	if a.coord != nil {
		_ = a.coord.TouchPresence(context.Background(), userID)
	}
}

func (a *api) setSocialPresence(userID, connectionID, status string) {
	if a.redis == nil {
		return
	}
	key := "social:presence:" + userID
	value, _ := json.Marshal(map[string]any{"status": status, "seenAt": time.Now().UnixMilli()})
	pipe := a.redis.Pipeline()
	pipe.HSet(context.Background(), key, connectionID, value)
	pipe.Expire(context.Background(), key, 90*time.Second)
	_, _ = pipe.Exec(context.Background())
}

func (a *api) clearSocialPresence(userID, connectionID string) bool {
	if a.redis == nil {
		return true
	}
	key := "social:presence:" + userID
	_ = a.redis.HDel(context.Background(), key, connectionID).Err()
	count, err := a.redis.HLen(context.Background(), key).Result()
	return err != nil || count == 0
}

func (a *api) publishFriendPresence(store persistence.SocialRepository, userID, status string) {
	friends, err := store.ListFriends(userID, 500)
	if err != nil {
		return
	}
	for _, friend := range friends {
		_, _ = store.AppendUserEvent(friend.UserID, "presence.changed", map[string]any{
			"userId": userID, "status": status,
		})
	}
}

func (a *api) applySocialPresence(players []persistence.CompactPlayer) {
	if a.redis == nil || len(players) == 0 {
		return
	}
	for i := range players {
		values, err := a.redis.HVals(context.Background(), "social:presence:"+players[i].UserID).Result()
		if err != nil || len(values) == 0 {
			players[i].PresenceStatus = "offline"
			continue
		}
		players[i].PresenceStatus = "away"
		for _, raw := range values {
			var item struct {
				Status string `json:"status"`
			}
			if json.Unmarshal([]byte(raw), &item) == nil && item.Status == "online" {
				players[i].PresenceStatus = "online"
				break
			}
		}
	}
}

func socialConnectionID() string {
	value := make([]byte, 8)
	if _, err := rand.Read(value); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(value)
}
