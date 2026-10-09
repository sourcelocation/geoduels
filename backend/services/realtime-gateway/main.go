package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"geoduels/internal/envcfg"
	"geoduels/internal/httpx"
	"geoduels/pkg/coordinator"
	"geoduels/pkg/observability"
)

type realtimeGateway struct {
	state    *coordinator.Store
	redis    *redis.Client
	metrics  *observability.APIMetrics
	draining atomic.Bool
	// drainSpread is how long a drain takes to close every socket, so clients don't all reconnect at once.
	drainSpread time.Duration

	socketsMu sync.Mutex
	sockets   map[*websocket.Conn]func()
}

var wsProxyUpgrader = websocket.Upgrader{CheckOrigin: httpx.WSOriginAllowed}

func main() {
	rdb, redisCleanup, err := redisFromEnv()
	if err != nil {
		log.Fatal(err)
	}
	g := &realtimeGateway{
		state:       coordinator.NewStore(rdb, envcfg.Duration("GAMEPLAY_NODE_TTL", 10*time.Second), 2*time.Hour, 24*time.Hour, 5*time.Second),
		redis:       rdb,
		metrics:     observability.NewAPIMetrics(),
		drainSpread: envcfg.Duration("REALTIME_GATEWAY_DRAIN_SPREAD", 10*time.Second),
		sockets:     map[*websocket.Conn]func(){},
	}
	defer redisCleanup()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		code := http.StatusInternalServerError
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
		}
		// gorilla/mux wrote empty bodies for unrouted paths and methods.
		if code == http.StatusNotFound || code == http.StatusMethodNotAllowed {
			_ = c.NoContent(code)
			return
		}
		e.DefaultHTTPErrorHandler(err, c)
	}
	e.Use(httpx.CORS)
	e.Use(g.metrics.EchoMiddleware)
	e.GET("/health", g.healthLive)
	e.GET("/health/live", g.healthLive)
	e.GET("/health/ready", g.healthReady)
	e.GET("/ws/:node", g.wsProxy)
	e.GET("/metrics", echo.WrapHandler(observability.Handler(g.metrics.Registry)))

	addr := envcfg.Get("REALTIME_GATEWAY_ADDR", ":8092")
	srv := &http.Server{
		Addr:              addr,
		Handler:           e,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	observability.Log("info", "realtime-gateway startup", map[string]any{"addr": addr})
	drained := make(chan struct{})
	go g.handleShutdown(srv, drained)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	<-drained
}

func (g *realtimeGateway) wsProxy(c echo.Context) error {
	r := c.Request()
	if g.draining.Load() {
		return httpx.PlainTextError(c, http.StatusServiceUnavailable, "draining")
	}
	nodeRoute := strings.TrimSpace(c.Param("node"))
	if nodeRoute == "" {
		return httpx.PlainTextError(c, http.StatusBadRequest, "missing node")
	}
	node, ok, err := g.state.GetNodeByRoute(r.Context(), nodeRoute)
	if err != nil {
		return httpx.PlainTextError(c, http.StatusBadGateway, "routing unavailable")
	}
	if !ok || strings.TrimSpace(node.InternalURL) == "" {
		return httpx.PlainTextError(c, http.StatusNotFound, "node unavailable")
	}

	target := websocketTarget(node.InternalURL, r.URL.RequestURI())
	backendConn, resp, err := websocket.DefaultDialer.DialContext(r.Context(), target, nil)
	if err != nil {
		if resp != nil && resp.StatusCode > 0 {
			return httpx.PlainTextError(c, resp.StatusCode, "gameplay unavailable")
		}
		return httpx.PlainTextError(c, http.StatusBadGateway, "gameplay unavailable")
	}
	defer backendConn.Close()

	clientConn, err := wsProxyUpgrader.Upgrade(c.Response().Writer, r, nil)
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
	if !g.track(clientConn, closeBoth) {
		sendRestart(clientConn)
		return nil
	}
	defer g.untrack(clientConn)

	errc := make(chan error, 2)
	go proxyWS(errc, closeBoth, clientConn, backendConn)
	go proxyWS(errc, closeBoth, backendConn, clientConn)

	if err := <-errc; err != nil && !isExpectedWSClose(err) {
		log.Printf("websocket proxy failed for route %s: %v", nodeRoute, err)
	}
	closeBoth()
	<-errc
	return nil
}

func (g *realtimeGateway) healthLive(c echo.Context) error {
	c.Response().WriteHeader(http.StatusOK)
	_, _ = c.Response().Write([]byte("ok"))
	return nil
}

func (g *realtimeGateway) healthReady(c echo.Context) error {
	if g.draining.Load() {
		return httpx.PlainTextError(c, http.StatusServiceUnavailable, "draining")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := g.redis.Ping(ctx).Err(); err != nil {
		return httpx.PlainTextError(c, http.StatusServiceUnavailable, "redis not ready")
	}
	c.Response().WriteHeader(http.StatusOK)
	_, _ = c.Response().Write([]byte("ready"))
	return nil
}

// track registers a proxied socket for the drain, or refuses it once the drain has begun.
func (g *realtimeGateway) track(clientConn *websocket.Conn, closeBoth func()) bool {
	g.socketsMu.Lock()
	defer g.socketsMu.Unlock()
	if g.draining.Load() {
		return false
	}
	g.sockets[clientConn] = closeBoth
	return true
}

func (g *realtimeGateway) untrack(clientConn *websocket.Conn) {
	g.socketsMu.Lock()
	delete(g.sockets, clientConn)
	g.socketsMu.Unlock()
}

// handleShutdown drains the gateway. It holds no match state, so it doesn't wait for matches: it
// asks every client to reconnect (1012, spread over drainSpread), and clients come back through
// another gateway to the same gameplay node, which resumes them.
func (g *realtimeGateway) handleShutdown(srv *http.Server, drained chan<- struct{}) {
	defer close(drained)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	<-sigCh
	g.socketsMu.Lock()
	g.draining.Store(true)
	sockets := make(map[*websocket.Conn]func(), len(g.sockets))
	for conn, closeBoth := range g.sockets {
		sockets[conn] = closeBoth
	}
	g.socketsMu.Unlock()

	if len(sockets) > 0 {
		pause := g.drainSpread / time.Duration(len(sockets))
		first := true
		for conn := range sockets {
			if !first {
				time.Sleep(pause)
			}
			first = false
			sendRestart(conn)
		}
		// Clients that haven't answered the close frame within a second are cut.
		time.Sleep(time.Second)
		for _, closeBoth := range sockets {
			closeBoth()
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("realtime-gateway shutdown failed: %v", err)
	}
}

// sendRestart asks a client to reconnect. gorilla/websocket allows WriteControl alongside the proxy's writes.
func sendRestart(conn *websocket.Conn) {
	message := websocket.FormatCloseMessage(websocket.CloseServiceRestart, "restarting")
	_ = conn.WriteControl(websocket.CloseMessage, message, time.Now().Add(time.Second))
}

func websocketTarget(baseURL, requestURI string) string {
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
	return base + requestURI
}

func proxyWS(errc chan<- error, closeBoth func(), dst, src *websocket.Conn) {
	for {
		messageType, payload, err := src.ReadMessage()
		if err != nil {
			closeBoth()
			errc <- err
			return
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

func redisFromEnv() (*redis.Client, func(), error) {
	url := envcfg.Get("REDIS_URL", "")
	if url == "" {
		return nil, nil, errors.New("REDIS_URL is required")
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, nil, err
	}
	rdb := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, nil, err
	}
	return rdb, func() { _ = rdb.Close() }, nil
}
