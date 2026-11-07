package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	natsserver "github.com/nats-io/nats-server/v2/server"

	"geoduels/pkg/live"
)

type testEvent struct {
	Body string `json:"body"`
}

func (testEvent) EventType() string { return "test.event" }

// stubAPI answers as the API does: the cookie names the user, and every connection gets a session.
func stubAPI(t *testing.T) *apiClient {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/identify"):
			_ = json.NewEncoder(w).Encode(map[string]string{"userId": strings.TrimPrefix(r.Header.Get("Cookie"), "user=")})
		case strings.HasSuffix(r.URL.Path, "/connect"):
			_, _ = w.Write([]byte(`{"type":"session","data":{}}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(server.Close)
	return &apiClient{url: server.URL, secret: "test", http: server.Client()}
}

type harness struct {
	nats    *natsserver.Server
	bus     *live.Bus
	gateway *gateway
	url     string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	nats, err := natsserver.NewServer(&natsserver.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true})
	if err != nil {
		t.Fatal(err)
	}
	go nats.Start()
	if !nats.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS did not start")
	}
	t.Cleanup(nats.Shutdown)
	bus, err := live.Connect(nats.ClientURL(), func() {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(bus.Close)
	g := &gateway{id: "test", api: stubAPI(t), bus: bus, secret: "test", clients: newClientSet[*client](), matches: newClientSet[*matchSocket]()}
	server := httptest.NewServer(g.routes())
	t.Cleanup(server.Close)
	return &harness{nats: nats, bus: bus, gateway: g, url: "ws" + strings.TrimPrefix(server.URL, "http") + "/api/live"}
}

// dial connects as the user and returns once the session has arrived, when the subscription exists.
func (h *harness) dial(t *testing.T, userID string) *websocket.Conn {
	t.Helper()
	header := http.Header{}
	if userID != "" {
		header.Set("Cookie", "user="+userID)
	}
	conn, resp, err := websocket.DefaultDialer.Dial(h.url, header)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	t.Cleanup(func() { _ = conn.Close() })
	if got := readType(t, conn); got != "session" {
		t.Fatalf("first message is %q, want the session", got)
	}
	return conn
}

func readType(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Type
}

func eventually(t *testing.T, what string, within time.Duration, holds func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !holds() {
		if time.Now().After(deadline) {
			t.Fatalf("%s: still not so after %s", what, within)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestEventReachesOnlyTheirPerson(t *testing.T) {
	h := newHarness(t)
	alice, bob := h.dial(t, "alice"), h.dial(t, "bob")
	if err := h.bus.Tell([]string{"alice"}, testEvent{Body: "hi"}); err != nil {
		t.Fatal(err)
	}
	if got := readType(t, alice); got != "test.event" {
		t.Fatalf("alice got %q", got)
	}
	_ = bob.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if _, _, err := bob.ReadMessage(); err == nil {
		t.Fatal("bob heard alice's event")
	}
}

func TestVisitorGetsASessionAndNoSubscription(t *testing.T) {
	h := newHarness(t)
	before := h.nats.NumSubscriptions()
	h.dial(t, "")
	if after := h.nats.NumSubscriptions(); after != before {
		t.Fatalf("a visitor subscribed: %d subscriptions, was %d", after, before)
	}
}

func TestDisconnectClosesTheirConnections(t *testing.T) {
	h := newHarness(t)
	first, second := h.dial(t, "alice"), h.dial(t, "alice")
	if err := h.bus.Disconnect("alice"); err != nil {
		t.Fatal(err)
	}
	for _, conn := range []*websocket.Conn{first, second} {
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		if _, _, err := conn.ReadMessage(); !websocket.IsCloseError(err, websocket.CloseServiceRestart) {
			t.Fatalf("got %v, want to be asked to reconnect", err)
		}
	}
}

// A client that stops reading is dropped, so what's waiting for it can't grow.
func TestSlowClientIsDisconnected(t *testing.T) {
	h := newHarness(t)
	h.dial(t, "slow")
	body := testEvent{Body: strings.Repeat("x", 64<<10)}
	eventually(t, "client held", 5*time.Second, func() bool { return len(h.gateway.clients.snapshot()) == 1 })
	for i := 0; i < 4*outboundBuffer; i++ {
		if err := h.bus.Tell([]string{"slow"}, body); err != nil {
			t.Fatal(err)
		}
	}
	eventually(t, "slow client dropped", writeWait+5*time.Second, func() bool { return len(h.gateway.clients.snapshot()) == 0 })
}

// The leak this service exists to rule out: whatever connects, once it's gone nothing of it is left.
func TestNothingOutlivesAConnection(t *testing.T) {
	h := newHarness(t)
	baselineSubs := h.nats.NumSubscriptions()
	settle := func() int {
		runtime.GC()
		time.Sleep(200 * time.Millisecond)
		return runtime.NumGoroutine()
	}
	warm := h.dial(t, "warm")
	_ = warm.Close()
	eventually(t, "warm-up connection gone", 5*time.Second, func() bool { return len(h.gateway.clients.snapshot()) == 0 })
	baseline := settle()

	const connections = 200
	conns := make([]*websocket.Conn, connections)
	for i := range conns {
		conns[i] = h.dial(t, "user"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	if held := len(h.gateway.clients.snapshot()); held != connections {
		t.Fatalf("holding %d connections, want %d", held, connections)
	}
	for _, conn := range conns {
		_ = conn.Close()
	}
	eventually(t, "connections released", 5*time.Second, func() bool { return len(h.gateway.clients.snapshot()) == 0 })
	eventually(t, "subscriptions released", 5*time.Second, func() bool { return h.nats.NumSubscriptions() == baselineSubs })
	// The offline grace passes and its reports finish; the test's own clients close idle connections.
	time.Sleep(offlineGrace)
	h.gateway.api.http.CloseIdleConnections()
	eventually(t, "goroutines back to where they were", 15*time.Second, func() bool {
		h.gateway.api.http.CloseIdleConnections()
		return settle() <= baseline+5
	})
}
