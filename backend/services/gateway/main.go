// The gateway holds browsers' sockets and nothing else. It knows no game rules, reads no database, and
// doesn't look inside what it forwards, so any number of them can run, and one can go away at any
// moment: its clients reconnect to another and start from a fresh session.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"

	"geoduels/pkg/live"
)

const shutdownSpread = 5 * time.Second

func main() {
	secret := strings.TrimSpace(os.Getenv("COORDINATOR_INTERNAL_SECRET"))
	if secret == "" {
		log.Fatal("COORDINATOR_INTERNAL_SECRET is required")
	}
	apiURL := strings.TrimRight(strings.TrimSpace(os.Getenv("API_URL")), "/")
	if apiURL == "" {
		log.Fatal("API_URL is required")
	}
	gateway := &gateway{
		id:      uuid.NewString(),
		api:     &apiClient{url: apiURL, secret: secret, http: &http.Client{Timeout: 15 * time.Second}},
		secret:  secret,
		clients: newClientSet[*client](),
		matches: newClientSet[*matchSocket](),
	}
	bus, err := live.Connect(os.Getenv("NATS_URL"), gateway.busReconnected)
	if err != nil {
		log.Fatal(err)
	}
	defer bus.Close()
	gateway.bus = bus

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go gateway.runHeartbeat(ctx)
	go gateway.runStatus(ctx)

	addr := os.Getenv("GATEWAY_ADDR")
	if addr == "" {
		addr = ":8095"
	}
	server := &http.Server{Addr: addr, Handler: gateway.routes(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		gateway.draining.Store(true)
		// Clients move to another gateway a few at a time, not all at once.
		gateway.clients.closeAll(shutdownSpread)
		gateway.matches.closeAll(shutdownSpread)
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("gateway %s listening on %s", gateway.id, addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
