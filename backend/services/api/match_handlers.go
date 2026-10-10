package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"

	"geoduels/internal/matches"
	"geoduels/pkg/contracts"
	"geoduels/pkg/matchkind"
)

// A match has one address, /api/v2/matches/{id}: its view says what it is and whether it is being
// played, and a seated player connects to it at /ws, which this service proxies to the gameplay node
// running it.

const (
	// placementWait is how long a start or a connect waits for a node to pick the match up.
	placementWait  = 10 * time.Second
	placementPoll  = 150 * time.Millisecond
	matchSocketTTL = 70 * time.Second
)

// matchView is what the viewer (empty when anonymous) sees of a match.
func (a *api) matchView(ctx context.Context, viewerID, matchID string) (contracts.MatchView, error) {
	view := contracts.MatchView{MatchID: matchID, Status: contracts.MatchViewMissing}
	session, live, err := a.matchStore.GetSession(ctx, matchID)
	if err != nil {
		return view, err
	}
	result, recorded, err := a.getPublicFinalMatchSnapshot(matchID)
	if err != nil {
		return view, err
	}
	switch {
	case live:
		view.Kind, view.Status = session.Kind, contracts.MatchViewStatus(session.Status)
	case recorded:
		view.Kind, view.Status = result.Kind, contracts.MatchViewEnded
	default:
		return view, nil
	}
	view.Mode = matchkind.Of(view.Kind).Mode
	open := view.Status == contracts.MatchViewStarting || view.Status == contracts.MatchViewLive
	if viewerID == "" && open {
		view.SignInRequired = true
		return view, nil
	}
	if view.Status == contracts.MatchViewEnded && recorded {
		view.Result = result
	}
	if live {
		config := session.Config
		view.Config = &config
		for _, seat := range session.Seats {
			view.Players = append(view.Players, contracts.MatchViewSeat{UserID: seat.UserID, DisplayName: seat.DisplayName, AvatarURL: seat.AvatarURL, TeamID: seat.Team})
		}
		view.Playing = open && session.Seated(viewerID)
		view.ReturnTarget, view.Party = a.returnTarget(viewerID, session)
	} else {
		config := result.Config
		view.Config = &config
		view.ReturnTarget = &contracts.MatchReturnTarget{Kind: contracts.MatchReturnHome}
	}
	if viewerID != "" {
		if current, ok, err := a.matchStore.ActiveMatchForUser(ctx, viewerID); err == nil && ok && current.MatchID != matchID {
			view.CurrentMatchID = current.MatchID
		}
	}
	return view, nil
}

// returnTarget is where the viewer goes after the match: back to its party while they are still in
// it, otherwise where the match was started from.
func (a *api) returnTarget(viewerID string, session matches.Session) (*contracts.MatchReturnTarget, *contracts.MatchViewParty) {
	target := contracts.NormalizeMatchReturnTarget(session.ReturnTarget)
	if target.Kind != contracts.MatchReturnParty {
		return target, nil
	}
	home := &contracts.MatchReturnTarget{Kind: contracts.MatchReturnHome}
	if viewerID == "" || target.PartyID == "" {
		return home, nil
	}
	party, found, err := a.parties.GetPartyByID(target.PartyID)
	if err != nil || !found || party.State == contracts.PartyClosed || party.State == contracts.PartyExpired || !partyHasMember(party, viewerID) {
		return home, nil
	}
	target.PartyInviteCode = party.InviteCode
	return target, &contracts.MatchViewParty{ID: party.ID, InviteCode: party.InviteCode}
}

func (a *api) getMatchView(c echo.Context) error {
	matchID := a.resolveEntityID("match", c.Param("id"))
	if matchID == "" {
		return plainTextError(c, http.StatusBadRequest, "invalid match")
	}
	viewerID := ""
	if claims, ok := a.optionalAuthenticatedClaims(c.Request()); ok {
		if banned, err := a.accountBanned(claims.Sub); err == nil && banned {
			return writeJSON(c, contracts.MatchView{MatchID: matchID, Status: contracts.MatchViewForbidden})
		}
		viewerID = claims.Sub
	}
	view, err := a.matchView(c.Request().Context(), viewerID, matchID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "match unavailable")
	}
	return writeJSON(c, view)
}

// matchBootstrap is a match page's first request: it renews the session from the refresh cookie
// and returns the view the signed-in player sees.
func (a *api) matchBootstrap(c echo.Context) error {
	matchID := a.resolveEntityID("match", c.Param("id"))
	if matchID == "" {
		return plainTextError(c, http.StatusBadRequest, "invalid match")
	}
	authPayload, nextRefreshToken, err := a.rotateSessionFromCookie(c.Request())
	if err != nil {
		a.clearRefreshCookie(c, c.Request())
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	a.setRefreshCookie(c, c.Request(), nextRefreshToken)
	banned, err := a.accountBanned(authPayload.User.ID)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "identity not found")
	}
	if banned {
		return writeJSON(c, contracts.MatchBootstrapResponse{Auth: authPayload, Match: contracts.MatchView{MatchID: matchID, Status: contracts.MatchViewForbidden}})
	}
	view, err := a.matchView(c.Request().Context(), authPayload.User.ID, matchID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "match unavailable")
	}
	return writeJSON(c, contracts.MatchBootstrapResponse{Auth: authPayload, Match: view})
}

// myMatch returns the match the player can return to, if any.
func (a *api) myMatch(c echo.Context) error {
	claims, err := a.authenticatedClaims(c.Request())
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	return writeJSON(c, map[string]any{"match": a.activeMatch(c.Request().Context(), claims.Sub)})
}

func (a *api) activeMatch(ctx context.Context, userID string) *contracts.ActiveMatchSummary {
	current, ok, err := a.matchStore.ActiveMatchForUser(ctx, userID)
	if err != nil || !ok {
		return nil
	}
	return &contracts.ActiveMatchSummary{MatchID: current.MatchID, Kind: current.Kind, Mode: matchkind.Of(current.Kind).Mode, Status: contracts.MatchViewStatus(current.Status)}
}

// createMatch starts a match the player plays alone and returns its view once a node runs it.
func (a *api) createMatch(c echo.Context) error {
	r := c.Request()
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	var req contracts.MatchStartRequest
	if err := decodeJSONBody(r, &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	if matchkind.Of(req.Kind).Origin != matchkind.OriginDirect {
		return plainTextError(c, http.StatusBadRequest, "this kind of match cannot be started directly")
	}
	config := contracts.NormalizeMatchConfig(req.Config)
	target := contracts.NormalizeMatchReturnTarget(req.ReturnTarget)
	if target.Kind == contracts.MatchReturnParty {
		target = &contracts.MatchReturnTarget{Kind: contracts.MatchReturnHome}
	}
	userID := claims.Sub
	matchID, err := a.startMatch(r.Context(), startRequest{
		Kind: req.Kind, Seats: []matchkind.Seat{{UserID: userID}}, Config: config,
		MapAccessUserID: userID, ReturnTarget: target,
	})
	if err != nil {
		return a.writeStartError(c, err)
	}
	a.awaitPlacement(r.Context(), matchID)
	view, err := a.matchView(r.Context(), userID, matchID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "match unavailable")
	}
	return writeJSONStatus(c, http.StatusCreated, view)
}

func (a *api) writeStartError(c echo.Context, err error) error {
	var admission *admissionError
	if errors.As(err, &admission) {
		if admission.UserID == "" {
			return writeAPIError(c, http.StatusServiceUnavailable, "maintenance", admission.Reason)
		}
		return writeAPIError(c, http.StatusForbidden, "not_admitted", admission.Reason)
	}
	var conflict *matches.ConflictError
	if errors.As(err, &conflict) {
		return writeJSONStatus(c, http.StatusConflict, map[string]string{"code": "ACTIVE_MATCH", "message": a.startErrorMessage(err), "matchId": conflict.MatchID})
	}
	log.Printf("match start failed: %v", err)
	return plainTextError(c, http.StatusBadGateway, "match could not start")
}

// awaitPlacement waits until a node picks the match up or the wait runs out.
func (a *api) awaitPlacement(ctx context.Context, matchID string) contracts.MatchSessionStatus {
	deadline := time.Now().Add(placementWait)
	for {
		status, err := a.matchStore.MatchSessionStatus(ctx, matchID)
		if err == nil && status != contracts.MatchSessionStarting {
			return status
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			return status
		}
		time.Sleep(placementPoll)
	}
}

// matchSocket proxies a seated player's socket to the node running the match.
func (a *api) matchSocket(c echo.Context) error {
	r := c.Request()
	if a.draining.Load() {
		return plainTextError(c, http.StatusServiceUnavailable, "draining")
	}
	claims, err := a.liveClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	if banned, err := a.accountBanned(claims.Sub); err != nil || banned {
		return plainTextError(c, http.StatusForbidden, "forbidden")
	}
	matchID := a.resolveEntityID("match", c.Param("id"))
	session, found, err := a.matchStore.GetSession(r.Context(), matchID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "match unavailable")
	}
	if !found {
		return plainTextError(c, http.StatusNotFound, "match not found")
	}
	if !session.Seated(claims.Sub) {
		return plainTextError(c, http.StatusForbidden, "forbidden")
	}
	if session.Status == contracts.MatchSessionStarting {
		a.awaitPlacement(r.Context(), matchID)
		if session, found, err = a.matchStore.GetSession(r.Context(), matchID); err != nil || !found {
			return plainTextError(c, http.StatusBadGateway, "match unavailable")
		}
	}
	if session.Status != contracts.MatchSessionLive || session.NodeURL == "" {
		return plainTextError(c, http.StatusGone, "match is not being played")
	}

	header := http.Header{}
	header.Set("X-Internal-Secret", a.internalSecret)
	header.Set("X-Geoduels-User", claims.Sub)
	target := websocketTarget(session.NodeURL, "/internal/matches/"+url.PathEscape(matchID)+"/ws")
	backendConn, resp, err := websocket.DefaultDialer.DialContext(r.Context(), target, header)
	if err != nil {
		if resp != nil && resp.StatusCode > 0 {
			return plainTextError(c, resp.StatusCode, "gameplay unavailable")
		}
		return plainTextError(c, http.StatusBadGateway, "gameplay unavailable")
	}
	defer backendConn.Close()
	clientConn, err := a.live.upgrader.Upgrade(c.Response().Writer, r, nil)
	if err != nil {
		return nil
	}
	defer clientConn.Close()

	var once sync.Once
	closeBoth := func() {
		once.Do(func() {
			_ = clientConn.Close()
			_ = backendConn.Close()
		})
	}
	if !a.proxied.track(clientConn, closeBoth) {
		sendRestart(clientConn)
		return nil
	}
	defer a.proxied.untrack(clientConn)

	a.touchViewerPresence(context.Background(), claims.Sub)
	errc := make(chan error, 2)
	go proxyWS(errc, closeBoth, backendConn, clientConn, func() {
		_ = clientConn.SetReadDeadline(time.Now().Add(matchSocketTTL))
		a.touchViewerPresence(context.Background(), claims.Sub)
	})
	go proxyWS(errc, closeBoth, clientConn, backendConn, nil)
	if err := <-errc; err != nil && !isExpectedWSClose(err) {
		log.Printf("match socket proxy failed for %s: %v", matchID, err)
	}
	closeBoth()
	<-errc
	return nil
}

// proxiedSockets tracks the game sockets this process proxies, so a drain can ask them to reconnect
// through another one.
type proxiedSockets struct {
	mu       sync.Mutex
	draining bool
	sockets  map[*websocket.Conn]func()
}

func (p *proxiedSockets) track(conn *websocket.Conn, closeBoth func()) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.draining {
		return false
	}
	if p.sockets == nil {
		p.sockets = map[*websocket.Conn]func(){}
	}
	p.sockets[conn] = closeBoth
	return true
}

func (p *proxiedSockets) untrack(conn *websocket.Conn) {
	p.mu.Lock()
	delete(p.sockets, conn)
	p.mu.Unlock()
}

// drain asks every proxied socket to reconnect, spread over spread so clients don't all come back at
// once, and cuts the ones that don't go.
func (p *proxiedSockets) drain(spread time.Duration) {
	p.mu.Lock()
	p.draining = true
	sockets := make(map[*websocket.Conn]func(), len(p.sockets))
	for conn, closeBoth := range p.sockets {
		sockets[conn] = closeBoth
	}
	p.mu.Unlock()
	if len(sockets) == 0 {
		return
	}
	pause := spread / time.Duration(len(sockets))
	first := true
	for conn := range sockets {
		if !first {
			time.Sleep(pause)
		}
		first = false
		sendRestart(conn)
	}
	time.Sleep(time.Second)
	for _, closeBoth := range sockets {
		closeBoth()
	}
}

// sendRestart asks a client to reconnect. gorilla/websocket allows WriteControl alongside the proxy's writes.
func sendRestart(conn *websocket.Conn) {
	message := websocket.FormatCloseMessage(websocket.CloseServiceRestart, "restarting")
	_ = conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))
}

func websocketTarget(baseURL, path string) string {
	base := strings.TrimRight(baseURL, "/")
	switch {
	case strings.HasPrefix(base, "https://"):
		base = "wss://" + strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "http://"):
		base = "ws://" + strings.TrimPrefix(base, "http://")
	case strings.HasPrefix(base, "wss://"), strings.HasPrefix(base, "ws://"):
	default:
		base = "ws://" + base
	}
	return base + path
}

// proxyWS copies src's messages to dst until either side fails, calling onRead after each one.
func proxyWS(errc chan<- error, closeBoth func(), dst, src *websocket.Conn, onRead func()) {
	if onRead != nil {
		onRead()
	}
	for {
		messageType, payload, err := src.ReadMessage()
		if err != nil {
			closeBoth()
			errc <- err
			return
		}
		if onRead != nil {
			onRead()
		}
		if err := dst.WriteMessage(messageType, payload); err != nil {
			closeBoth()
			errc <- err
			return
		}
	}
}

func isExpectedWSClose(err error) bool {
	if err == nil || errors.Is(err, io.EOF) {
		return true
	}
	return websocket.IsCloseError(
		err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
		websocket.CloseServiceRestart,
	)
}
