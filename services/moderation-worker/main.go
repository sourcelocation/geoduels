package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"geoduels/pkg/observability"
	"geoduels/pkg/persistence"
)

const (
	workerDrainTimeout    = 5 * time.Second
	workerShutdownTimeout = 10 * time.Second
)

type worker struct {
	store      persistence.Store
	httpClient *http.Client
	draining   atomic.Bool
}

func main() {
	w, err := newWorker()
	if err != nil {
		log.Fatal(err)
	}
	defer w.close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.startReportNotificationWorker(ctx)

	r := http.NewServeMux()
	r.HandleFunc("/health/live", w.healthLive)
	r.HandleFunc("/health/ready", w.healthReady)
	r.HandleFunc("/health", w.healthReady)

	addr := getenv("MODERATION_WORKER_ADDR", ":8093")
	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	observability.Log("info", "moderation worker startup", map[string]any{"addr": addr})
	go handleWorkerShutdown(w, srv, cancel)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func newWorker() (*worker, error) {
	store, err := persistence.NewFromEnv()
	if err != nil {
		return nil, err
	}
	return &worker{
		store:      store,
		httpClient: &http.Client{Timeout: 3 * time.Second},
	}, nil
}

func (w *worker) close() {
	if w.store != nil {
		w.store.Close()
	}
}

func (w *worker) healthLive(rw http.ResponseWriter, _ *http.Request) {
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte("ok"))
}

func (w *worker) healthReady(rw http.ResponseWriter, _ *http.Request) {
	if w.draining.Load() {
		http.Error(rw, "draining", http.StatusServiceUnavailable)
		return
	}
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte("ready"))
}

func handleWorkerShutdown(w *worker, srv *http.Server, cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	<-sigCh
	w.draining.Store(true)
	cancel()
	time.Sleep(workerDrainTimeout)

	ctx, shutdownCancel := context.WithTimeout(context.Background(), workerShutdownTimeout)
	defer shutdownCancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("moderation worker shutdown failed: %v", err)
	}
}

func getenv(k, fallback string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return fallback
}
