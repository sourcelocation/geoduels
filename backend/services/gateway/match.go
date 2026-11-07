package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const matchReadTTL = 70 * time.Second

// serveMatch forwards a seated player's socket to the node running their match. The match lives on the
// node: if this connection goes away the player reconnects, through any gateway, and is sent a snapshot.
func (g *gateway) serveMatch(w http.ResponseWriter, r *http.Request) {
	if g.draining.Load() {
		http.Error(w, "draining", http.StatusServiceUnavailable)
		return
	}
	route, status, err := g.api.route(r.Context(), url.PathEscape(r.PathValue("id")), r.Header.Get("Cookie"))
	if err != nil || status != http.StatusOK {
		http.Error(w, "match unavailable", status)
		return
	}
	header := http.Header{}
	header.Set("X-Internal-Secret", g.secret)
	header.Set("X-Geoduels-User", route.UserID)
	target := websocketTarget(route.NodeURL) + "/internal/matches/" + url.PathEscape(route.MatchID) + "/ws"
	backend, resp, err := websocket.DefaultDialer.DialContext(r.Context(), target, header)
	if err != nil {
		if resp != nil && resp.StatusCode > 0 {
			http.Error(w, "gameplay unavailable", resp.StatusCode)
			return
		}
		http.Error(w, "gameplay unavailable", http.StatusBadGateway)
		return
	}
	defer backend.Close()
	player, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer player.Close()

	socket := &matchSocket{player: player, backend: backend}
	g.matches.add(socket)
	defer g.matches.remove(socket)

	_ = player.SetReadDeadline(time.Now().Add(matchReadTTL))
	errs := make(chan error, 2)
	go proxy(errs, socket.closeBoth, backend, player, func() {
		_ = player.SetReadDeadline(time.Now().Add(matchReadTTL))
	})
	go proxy(errs, socket.closeBoth, player, backend, func() {})
	if err := <-errs; err != nil && !expectedClose(err) {
		log.Printf("match socket failed for %s: %v", route.MatchID, err)
	}
	socket.closeBoth()
	<-errs
}

// matchSocket is a forwarded game connection.
type matchSocket struct {
	player, backend *websocket.Conn
	once            sync.Once
}

func (s *matchSocket) closeBoth() {
	s.once.Do(func() {
		_ = s.player.Close()
		_ = s.backend.Close()
	})
}

// requestClose asks the player to reconnect, which lands them on another gateway and the same match.
func (s *matchSocket) requestClose() {
	message := websocket.FormatCloseMessage(websocket.CloseServiceRestart, "restarting")
	_ = s.player.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))
}

// proxy copies src's messages to dst until either side fails, calling onRead after each one.
func proxy(errs chan<- error, closeBoth func(), dst, src *websocket.Conn, onRead func()) {
	for {
		messageType, payload, err := src.ReadMessage()
		if err != nil {
			closeBoth()
			errs <- err
			return
		}
		onRead()
		if err := dst.WriteMessage(messageType, payload); err != nil {
			closeBoth()
			errs <- err
			return
		}
	}
}

func websocketTarget(base string) string {
	switch {
	case strings.HasPrefix(base, "https://"):
		return "wss://" + strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "http://"):
		return "ws://" + strings.TrimPrefix(base, "http://")
	case strings.HasPrefix(base, "wss://"), strings.HasPrefix(base, "ws://"):
		return base
	default:
		return "ws://" + base
	}
}

func expectedClose(err error) bool {
	if err == nil || errors.Is(err, io.EOF) {
		return true
	}
	return websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived, websocket.CloseServiceRestart)
}
