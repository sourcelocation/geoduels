package main

import (
	"context"
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/riverqueue/river"

	"geoduels/internal/inbox"
	"geoduels/internal/jobs"
	"geoduels/internal/presence"
	"geoduels/internal/profiles"
	"geoduels/pkg/live"
	"geoduels/pkg/observability"
)

// The gateway holds the sockets and knows nothing else; everything about who someone is and what they
// should see is answered here, over routes only the gateway can reach. /internal isn't routed publicly.

const inboxStartSize = 20

// internalOnly refuses anything that doesn't hold the shared secret.
func (a *api) internalOnly(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		secret := c.Request().Header.Get("X-Internal-Secret")
		if subtle.ConstantTimeCompare([]byte(secret), []byte(a.internalSecret)) != 1 {
			return plainTextError(c, http.StatusForbidden, "forbidden")
		}
		return next(c)
	}
}

// identifyConnection says whose session the cookie holds; none is a visitor, not an error.
func (a *api) identifyConnection(c echo.Context) error {
	session, err := a.authSessionFromCookies(c.Request())
	if err != nil {
		return writeJSON(c, map[string]string{"userId": ""})
	}
	return writeJSON(c, map[string]string{"userId": session.UserID})
}

type connectRequest struct {
	GatewayID  string              `json:"gatewayId"`
	Connection presence.Connection `json:"connection"`
}

// connectLive records the connection and returns its session, encoded as it goes to the client.
func (a *api) connectLive(c echo.Context) error {
	var req connectRequest
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	ctx := c.Request().Context()
	first, err := presence.Connect(ctx, a.db.Pool(), req.GatewayID, req.Connection)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "presence unavailable")
	}
	if first {
		if err := profiles.TouchLastSeen(ctx, a.db.Pool(), req.Connection.UserID, time.Now().UTC()); err != nil {
			observability.Log("warn", "last seen update failed", map[string]any{"userId": req.Connection.UserID, "error": err.Error()})
		}
		a.tellPresenceChanged(ctx, req.Connection.UserID)
	}
	session, err := a.sessionFor(ctx, req.Connection.UserID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "session unavailable")
	}
	payload, err := live.Encode(session)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "session unavailable")
	}
	return c.Blob(http.StatusOK, echo.MIMEApplicationJSON, payload)
}

// disconnectLive removes the connection; the gateway has already waited out a page reload.
func (a *api) disconnectLive(c echo.Context) error {
	var req presence.Connection
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	ctx := c.Request().Context()
	last, err := presence.Disconnect(ctx, a.db.Pool(), req)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "presence unavailable")
	}
	if last {
		a.wentOffline(ctx, []string{req.UserID})
	}
	return c.NoContent(http.StatusNoContent)
}

type heartbeatRequest struct {
	GatewayID   string                `json:"gatewayId"`
	Connections []presence.Connection `json:"connections"`
}

// heartbeatLive makes the rows match what the gateway holds, repairing anything that went missing.
func (a *api) heartbeatLive(c echo.Context) error {
	var req heartbeatRequest
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	ctx := c.Request().Context()
	gone, err := presence.Reconcile(ctx, a.db.Pool(), req.GatewayID, req.Connections)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "presence unavailable")
	}
	a.wentOffline(ctx, gone)
	users := make([]string, 0, len(req.Connections))
	for _, connection := range req.Connections {
		if connection.UserID != "" {
			users = append(users, connection.UserID)
		}
	}
	if err := a.queue.TouchAll(ctx, users); err != nil {
		observability.Log("warn", "queue touch failed", map[string]any{"error": err.Error()})
	}
	return c.NoContent(http.StatusNoContent)
}

// statusLive is the status each gateway sends its clients when it changes.
func (a *api) statusLive(c echo.Context) error {
	payload, err := live.Encode(a.status.current())
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "status unavailable")
	}
	return c.Blob(http.StatusOK, echo.MIMEApplicationJSON, payload)
}

// sessionFor reads everything a connection starts with.
func (a *api) sessionFor(ctx context.Context, userID string) (sessionEvent, error) {
	session := sessionEvent{
		UserID: userID, Status: a.status.current(),
		Social: emptySocialState(), Inbox: inboxState{Messages: []inboxMessage{}},
	}
	if userID == "" {
		return session, nil
	}
	var err error
	if session.Social, err = a.socialStateFor(ctx, userID); err != nil {
		return sessionEvent{}, err
	}
	pool := a.db.Pool()
	if session.Inbox.Unread, err = inbox.Unread(ctx, pool, userID); err != nil {
		return sessionEvent{}, err
	}
	messages, err := inbox.Page(ctx, pool, userID, inboxStartSize, 0)
	if err != nil {
		return sessionEvent{}, err
	}
	session.Inbox.Messages = newInboxMessages(messages)
	current, err := a.parties.GetCurrentParty(userID)
	if err != nil {
		return sessionEvent{}, err
	}
	if current != nil {
		party, found, err := a.loadParty(ctx, current.ID)
		if err != nil {
			return sessionEvent{}, err
		}
		if found {
			view := newPartySnapshot(party)
			session.Party = &view
		}
	}
	session.ActiveMatch = a.activeMatch(ctx, userID)
	queuedAt, queued, err := a.queue.QueuedSince(ctx, userID)
	if err != nil {
		return sessionEvent{}, err
	}
	if queued {
		session.Queue = queueUpdated{Queued: true, QueuedAt: queuedAt.UnixMilli()}
	}
	return session, nil
}

// wentOffline is everything that follows from people having no connection left.
func (a *api) wentOffline(ctx context.Context, userIDs []string) {
	now := time.Now().UTC()
	for _, userID := range userIDs {
		if userID == "" {
			continue
		}
		if err := profiles.TouchLastSeen(ctx, a.db.Pool(), userID, now); err != nil {
			observability.Log("warn", "last seen update failed", map[string]any{"userId": userID, "error": err.Error()})
		}
		if err := a.queue.Leave(ctx, userID); err != nil {
			observability.Log("warn", "queue leave failed", map[string]any{"userId": userID, "error": err.Error()})
		}
		a.tellPresenceChanged(ctx, userID)
	}
}

// tellPresenceChanged tells the person's friends, and any party they're in, what they're doing now.
func (a *api) tellPresenceChanged(ctx context.Context, userID string) {
	if userID == "" {
		return
	}
	if current, err := a.parties.GetCurrentParty(userID); err == nil && current != nil {
		a.tellParty(ctx, current.ID)
	}
	settings, err := a.social.GetSocialSettings(ctx, userID)
	if err != nil || !settings.PresenceVisible {
		return
	}
	friends, err := a.social.ListFriends(ctx, userID, 500)
	if err != nil || len(friends) == 0 {
		return
	}
	event := friendPresence{UserID: userID, Status: "offline"}
	if online, err := presence.Online(ctx, a.db.Pool(), []string{userID}); err == nil && online[userID] {
		event.Status = "online"
		if playing, err := a.matchStore.UsersInLiveMatches(ctx, []string{userID}); err == nil && playing[userID] {
			event.Activity = "in_match"
		}
	}
	ids := make([]string, len(friends))
	for i, friend := range friends {
		ids[i] = friend.UserID
	}
	a.tell(ids, event)
}

// tell publishes an event. Losing one only delays what someone sees, so it never fails a request.
func (a *api) tell(userIDs []string, event live.Event) {
	if err := a.bus.Tell(userIDs, event); err != nil {
		observability.Log("warn", "live publish failed", map[string]any{"type": event.EventType(), "error": err.Error()})
	}
}

// runPresenceSweep removes gateways that stopped reporting. Every replica runs it, and each row is
// removed by only one of them.
func (a *api) runPresenceSweep() {
	ticker := time.NewTicker(presence.HeartbeatEvery)
	defer ticker.Stop()
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		gone, err := presence.Sweep(ctx, a.db.Pool())
		if err != nil {
			observability.Log("warn", "presence sweep failed", map[string]any{"error": err.Error()})
		}
		a.wentOffline(ctx, gone)
		cancel()
	}
}

// matchRoute says where a player's match socket goes. The gateway forwards to it and holds no state.
func (a *api) matchRoute(c echo.Context) error {
	r := c.Request()
	session, err := a.authSessionFromCookies(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	userID := session.UserID
	if banned, err := a.accountBanned(userID); err != nil || banned {
		return plainTextError(c, http.StatusForbidden, "forbidden")
	}
	matchID := a.resolveEntityID("match", c.Param("id"))
	found, nodeURL, status := a.matchNode(r.Context(), matchID, userID)
	if !found {
		return plainTextError(c, status, "match unavailable")
	}
	return writeJSON(c, map[string]string{"userId": userID, "matchId": matchID, "nodeUrl": strings.TrimRight(nodeURL, "/")})
}

// River workers: a transaction anywhere asks for these, and they run once it has committed.

type inboxChangedWorker struct {
	river.WorkerDefaults[jobs.InboxChangedArgs]
	api *api
}

func (w *inboxChangedWorker) Work(ctx context.Context, job *river.Job[jobs.InboxChangedArgs]) error {
	message, err := inbox.Get(ctx, w.api.db.Pool(), job.Args.UserID, job.Args.MessageID)
	if err != nil {
		return err
	}
	return w.api.bus.Tell([]string{job.Args.UserID}, inboxAdded{Message: newInboxMessage(message)})
}

type partyChangedWorker struct {
	river.WorkerDefaults[jobs.PartyChangedArgs]
	api *api
}

func (w *partyChangedWorker) Work(ctx context.Context, job *river.Job[jobs.PartyChangedArgs]) error {
	w.api.tellParty(ctx, job.Args.PartyID)
	return nil
}

type presenceChangedWorker struct {
	river.WorkerDefaults[jobs.PresenceChangedArgs]
	api *api
}

func (w *presenceChangedWorker) Work(ctx context.Context, job *river.Job[jobs.PresenceChangedArgs]) error {
	for _, userID := range job.Args.UserIDs {
		w.api.tellPresenceChanged(ctx, userID)
	}
	return nil
}
