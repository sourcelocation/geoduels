package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
	"geoduels/pkg/coordinator"
	"geoduels/pkg/persistence"
)

type liveSocialStore struct {
	testRepositories
	friends []persistence.CompactPlayer
}

func (s *liveSocialStore) GetSocialAccount(string) (bool, bool, bool, error) {
	return false, true, true, nil
}
func (s *liveSocialStore) GetSocialSettings(string) (persistence.SocialSettings, error) {
	return persistence.SocialSettings{PresenceVisible: true}, nil
}
func (s *liveSocialStore) ListFriends(userID string, _ int) ([]persistence.CompactPlayer, error) {
	if userID == "viewer-1" {
		return s.friends, nil
	}
	return nil, nil
}

func TestUserLiveSendsHelloAndPresencePatch(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	secret := []byte("01234567890123456789012345678901")
	token, err := auth.IssueAppAccessToken(secret, "viewer-1", "session-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	store := &liveSocialStore{friends: []persistence.CompactPlayer{{UserID: "friend-1"}}}
	coordStore := coordinator.NewStore(rdb, time.Minute, time.Hour, time.Hour, time.Second)
	a := &api{
		social:        store,
		coord:         coordStore,
		redis:         rdb,
		appAuthSecret: secret,
	}
	a.live = newLiveHub(a)
	server := httptest.NewServer(http.HandlerFunc(a.userLive))
	t.Cleanup(server.Close)

	base := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/me/live?accessToken="
	received := make(chan contracts.LiveEvent, 1)
	friendToken, err := auth.IssueAppAccessToken(secret, "friend-1", "session-2", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	friendConn, _, err := websocket.DefaultDialer.Dial(base+friendToken, nil)
	if err != nil {
		t.Fatalf("friend dial: %v", err)
	}
	t.Cleanup(func() { _ = friendConn.Close() })
	go func() {
		_ = friendConn.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			var event contracts.LiveEvent
			if err := friendConn.ReadJSON(&event); err != nil {
				return
			}
			if event.Type == contracts.LivePresenceEvent {
				received <- event
				return
			}
		}
	}()
	conn, _, err := websocket.DefaultDialer.Dial(base+token, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var hello contracts.LiveEvent
	if err := conn.ReadJSON(&hello); err != nil {
		t.Fatalf("hello: %v", err)
	}
	if hello.Type != contracts.LiveHello {
		t.Fatalf("first event = %q", hello.Type)
	}
	select {
	case event := <-received:
		if event.Presence == nil || event.Presence.UserID != "viewer-1" || event.Presence.PresenceStatus != "online" {
			t.Fatalf("presence = %+v", event.Presence)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for presence patch")
	}
}
