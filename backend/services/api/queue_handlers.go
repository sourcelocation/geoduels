package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"geoduels/internal/matches"
	"geoduels/internal/queue"
	"geoduels/pkg/maintenance"
	"geoduels/pkg/matchkind"
	"geoduels/pkg/observability"
)

// A queued player stays queued for as long as they're connected: gateway heartbeats keep their tickets
// fresh, and their last connection closing takes them out. The matchmaker runs in whichever process
// gets its lock first, and tells players what it found.

const matchmakeEvery = 500 * time.Millisecond

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

type queueJoinRequest struct {
	Queues string `json:"queues"`
}

func (a *api) joinQueue(c echo.Context) error {
	r := c.Request()
	ctx := r.Context()
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	userID := claims.Sub
	status, err := a.maintenanceStatus(ctx)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "queue unavailable")
	}
	if status.QueueBlocked() {
		return plainTextError(c, http.StatusServiceUnavailable, maintenanceQueueMessage(status))
	}
	if err := a.admit(ctx, matchkind.Of(matchkind.RankedDuel), []string{userID}); err != nil {
		return plainTextError(c, http.StatusForbidden, a.startErrorMessage(err))
	}
	// A player already in a match that queueing would not replace goes back to it.
	if current, ok, err := a.matchStore.ActiveMatchForUser(ctx, userID); err == nil && ok && !matchkind.Of(current.Kind).Replaceable {
		a.tell([]string{userID}, matchStarted{Match: *a.activeMatch(ctx, userID)})
		return c.NoContent(http.StatusNoContent)
	}
	var req queueJoinRequest
	if err := decodeJSONBody(r, &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	profile, err := a.profiles.GetProfile(userID)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "profile unavailable")
	}
	if err := a.queue.Join(ctx, userID, queue.ParseVariants(req.Queues), profile.MMR); err != nil {
		return plainTextError(c, http.StatusBadGateway, "queue unavailable")
	}
	a.tell([]string{userID}, queueUpdated{Queued: true, QueuedAt: time.Now().UnixMilli()})
	return c.NoContent(http.StatusNoContent)
}

func (a *api) leaveQueue(c echo.Context) error {
	r := c.Request()
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	if err := a.queue.Leave(r.Context(), claims.Sub); err != nil {
		return plainTextError(c, http.StatusBadGateway, "queue unavailable")
	}
	a.tell([]string{claims.Sub}, queueUpdated{})
	return c.NoContent(http.StatusNoContent)
}

// runMatchmaker pairs queued players and starts their matches, in whichever process holds the
// matchmaker's lock for the pass.
func (a *api) runMatchmaker() {
	ticker := time.NewTicker(matchmakeEvery)
	defer ticker.Stop()
	for range ticker.C {
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
	// Told once everything has committed.
	started := []startedMatch{}
	removed := []string{}
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
		req := startRequest{
			Kind:   matchkind.RankedDuel,
			Seats:  []matchkind.Seat{{UserID: pair.Second.UserID}, {UserID: pair.First.UserID}},
			Config: queue.Config(pair.Variant),
		}
		match, err := a.startMatchIn(ctx, pairTx, req)
		if err == nil {
			if err := pairTx.Commit(ctx); err != nil {
				return err
			}
			started = append(started, match)
			continue
		}
		_ = pairTx.Rollback(ctx)
		// A player who cannot play any more leaves the queue. The other stays queued in their place.
		var conflict *matches.ConflictError
		var admission *admissionError
		switch {
		case errors.As(err, &conflict) && conflict.UserID != "":
			removed = append(removed, conflict.UserID)
			err = queue.LeaveTx(ctx, tx, conflict.UserID)
		case errors.As(err, &admission) && admission.UserID != "":
			removed = append(removed, admission.UserID)
			err = queue.LeaveTx(ctx, tx, admission.UserID)
		default:
			observability.Log("warn", "ranked match start failed", map[string]any{"error": err.Error()})
			err = nil
		}
		if err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	for _, match := range started {
		a.tellMatchStarted(ctx, match)
	}
	a.tell(removed, queueUpdated{Error: "You can't play right now, so you left the queue."})
	return nil
}
