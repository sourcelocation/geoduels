package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"

	"geoduels/internal/matches"
	"geoduels/internal/queue"
	"geoduels/pkg/contracts"
	"geoduels/pkg/maintenance"
	"geoduels/pkg/matchkind"
	"geoduels/pkg/observability"
	"geoduels/pkg/pgnotify"
)

// A queued player holds the queue socket open. It keeps their tickets fresh and tells them which
// match the matchmaker started for them; the matchmaker itself runs in whichever process gets its
// lock first.

const (
	queueTouchEvery = 10 * time.Second
	queuePingEvery  = 20 * time.Second
	queueReadTTL    = 70 * time.Second
	matchmakeEvery  = 500 * time.Millisecond
)

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

type socketWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (w *socketWriter) send(event string, payload any) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(8 * time.Second))
	return w.conn.WriteJSON(map[string]any{"type": event, "payload": payload}) == nil
}

func (w *socketWriter) ping() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)) == nil
}

func (a *api) queueSocket(c echo.Context) error {
	r := c.Request()
	if a.draining.Load() {
		return plainTextError(c, http.StatusServiceUnavailable, "draining")
	}
	status, err := a.maintenanceStatus(r.Context())
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "queue unavailable")
	}
	if status.QueueBlocked() {
		return plainTextError(c, http.StatusServiceUnavailable, maintenanceQueueMessage(status))
	}
	claims, err := a.liveClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	userID := claims.Sub
	if err := a.admit(r.Context(), matchkind.Of(matchkind.RankedDuel), []string{userID}); err != nil {
		return plainTextError(c, http.StatusForbidden, a.startErrorMessage(err))
	}
	profile, err := a.profiles.GetProfile(userID)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "profile unavailable")
	}
	variants := queue.ParseVariants(c.QueryParam("queues"))

	conn, err := a.live.upgrader.Upgrade(c.Response().Writer, r, nil)
	if err != nil {
		return nil
	}
	defer conn.Close()
	w := &socketWriter{conn: conn}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(queueReadTTL))
	conn.SetPongHandler(func(string) error {
		a.touchViewerPresence(context.Background(), userID)
		return conn.SetReadDeadline(time.Now().Add(queueReadTTL))
	})
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	a.touchViewerPresence(context.Background(), userID)

	// A player already in a match that queueing would not replace goes back to it.
	if current, ok, err := a.matchStore.ActiveMatchForUser(r.Context(), userID); err == nil && ok && !matchkind.Of(current.Kind).Replaceable {
		w.send("match_found", a.activeMatch(r.Context(), userID))
		return nil
	}

	// Listen before joining, so the match the matchmaker starts is never missed.
	events, unsubscribe := a.events.Subscribe(pgnotify.LiveTopic(userID))
	defer unsubscribe()
	if err := a.queue.Join(r.Context(), userID, variants, profile.MMR); err != nil {
		w.send("queue_error", map[string]string{"code": "QUEUE_UNAVAILABLE", "message": "queue unavailable"})
		return nil
	}
	matched := false
	defer func() {
		if !matched {
			_ = a.queue.Leave(context.Background(), userID)
		}
	}()
	if !w.send("queue_status", contracts.QueueStatusEvent{Status: "queued", QueuedAt: time.Now().UnixMilli()}) {
		return nil
	}

	// foundMatch reports the player's new match, if the matchmaker started one.
	foundMatch := func() bool {
		current := a.activeMatch(context.Background(), userID)
		if current == nil || current.Kind != matchkind.RankedDuel {
			return false
		}
		matched = true
		w.send("match_found", current)
		return true
	}
	touch := time.NewTicker(queueTouchEvery)
	defer touch.Stop()
	ping := time.NewTicker(queuePingEvery)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case message := <-events:
			if message.Resync {
				if foundMatch() {
					return nil
				}
				continue
			}
			var event contracts.LiveEvent
			if json.Unmarshal(message.Data, &event) == nil && event.Type == contracts.LiveMatchStarted && event.Match != nil && event.Match.Kind == matchkind.RankedDuel {
				matched = true
				w.send("match_found", event.Match)
				return nil
			}
		case <-touch.C:
			if a.draining.Load() {
				// Another process serves the queue; the client reconnects there and queues again.
				w.send("queue_reconnect", nil)
				return nil
			}
			queued, err := a.queue.Touch(context.Background(), userID)
			if err != nil {
				observability.Log("warn", "queue touch failed", map[string]any{"userId": userID, "error": err.Error()})
				continue
			}
			if !queued {
				if foundMatch() {
					return nil
				}
				w.send("queue_error", map[string]string{"code": "QUEUE_EXPIRED", "message": "Queue expired. Please re-queue."})
				return nil
			}
		case <-ping.C:
			if !w.ping() {
				return nil
			}
		}
	}
}

// runMatchmaker pairs queued players and starts their matches, in whichever process holds the
// matchmaker's lock for the pass.
func (a *api) runMatchmaker() {
	ticker := time.NewTicker(matchmakeEvery)
	defer ticker.Stop()
	for range ticker.C {
		if a.draining.Load() {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := a.matchmakingPass(ctx); err != nil {
			observability.Log("warn", "matchmaking pass failed", map[string]any{"error": err.Error()})
		}
		cancel()
	}
}

func (a *api) matchmakingPass(ctx context.Context) error {
	status, err := a.maintenanceStatus(ctx)
	if err != nil || status.QueueBlocked() {
		return err
	}
	tx, err := a.db.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	locked, err := queue.LockTx(ctx, tx)
	if err != nil || !locked {
		return err
	}
	tickets, err := queue.TicketsTx(ctx, tx)
	if err != nil {
		return err
	}
	for _, pair := range queue.Pairings(tickets, time.Now()) {
		pairTx, err := tx.Begin(ctx)
		if err != nil {
			return err
		}
		taken, err := queue.TakeTx(ctx, pairTx, pair.First.UserID, pair.Second.UserID)
		if err != nil || !taken {
			_ = pairTx.Rollback(ctx)
			if err != nil {
				return err
			}
			continue
		}
		_, err = a.startMatchIn(ctx, pairTx, startRequest{
			Kind:   matchkind.RankedDuel,
			Seats:  []matchkind.Seat{{UserID: pair.Second.UserID}, {UserID: pair.First.UserID}},
			Config: queue.Config(pair.Variant),
		})
		if err == nil {
			if err := pairTx.Commit(ctx); err != nil {
				return err
			}
			continue
		}
		_ = pairTx.Rollback(ctx)
		// A player who cannot play any more leaves the queue; their socket finds out from its
		// next touch. The other stays queued in their place.
		var conflict *matches.ConflictError
		var admission *admissionError
		switch {
		case errors.As(err, &conflict) && conflict.UserID != "":
			err = queue.LeaveTx(ctx, tx, conflict.UserID)
		case errors.As(err, &admission) && admission.UserID != "":
			err = queue.LeaveTx(ctx, tx, admission.UserID)
		default:
			observability.Log("warn", "ranked match start failed", map[string]any{"error": err.Error()})
			err = nil
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (a *api) publishPartyChanged(ctx context.Context, partyID string) {
	if partyID == "" {
		return
	}
	if err := pgnotify.Publish(ctx, a.db.Pool(), pgnotify.PartyTopic(partyID), pgnotify.PartyChanged); err != nil {
		observability.Log("warn", "party event publish failed", map[string]any{"partyId": partyID, "error": err.Error()})
	}
}
