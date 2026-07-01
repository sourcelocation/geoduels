package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"geoduels/pkg/observability"
)

const (
	apiDrainTimeout    = 5 * time.Second
	apiShutdownTimeout = 10 * time.Second
)

func main() {
	a, err := newAPI()
	if err != nil {
		log.Fatal(err)
	}
	defer a.close()
	r := routes(a)
	addr := getenv("API_ADDR", ":8080")
	srv := &http.Server{
		Addr:              addr,
		Handler:           cors(a.metrics.Middleware(r)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	observability.Log("info", "api startup", map[string]any{"addr": addr})
	go a.runGuestAccountCleanupLoop()
	go a.runStorageCleanupLoop()
	go a.runRankedSeasonResetLoop()
	go handleAPIShutdown(a, srv)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func handleAPIShutdown(a *api, srv *http.Server) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	<-sigCh
	a.draining.Store(true)
	time.Sleep(apiDrainTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), apiShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("api shutdown failed: %v", err)
	}
}
