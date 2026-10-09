package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/riverqueue/river"

	"geoduels/internal/accounts"
	"geoduels/internal/content"
	"geoduels/internal/curation"
	"geoduels/internal/jobs"
	"geoduels/internal/moderation"
	"geoduels/internal/seasons"
	"geoduels/internal/storage"
	"geoduels/pkg/observability"
	"geoduels/pkg/persistence"
	db "geoduels/pkg/persistence/sqlc/db"
)

const (
	workerDrainTimeout    = 5 * time.Second
	workerShutdownTimeout = 10 * time.Second
)

// worker owns the long-lived dependencies of the job worker.
type worker struct {
	db         *persistence.DB
	jobs       *jobs.Client
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
	if err := w.jobs.Start(ctx); err != nil {
		log.Fatal(err)
	}

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
	drained := make(chan struct{})
	go handleWorkerShutdown(w, srv, cancel, drained)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	<-drained
}

func newWorker() (*worker, error) {
	store, err := persistence.NewFromEnv()
	if err != nil {
		return nil, err
	}
	pool := store.Pool()

	// Insert-only client for producers inside this process.
	producer, err := jobs.NewClient(pool, "", nil, nil)
	if err != nil {
		store.Close()
		return nil, err
	}
	contentStore := content.NewPGStore(pool, producer)
	moderationService := moderation.NewService(moderation.NewPGStore(pool, producer), moderation.NewRiskEngineFromEnv())
	curationService := curation.NewService(curation.NewPGStore(pool))
	accountsStore := accounts.NewPGStore(pool, producer)
	accountsService := accounts.NewService(accountsStore)
	storageStore := storage.NewPGStore(pool)
	seasonsStore := seasons.NewPGStore(pool)

	workers := river.NewWorkers()
	river.AddWorker(workers, &matchAnalyzeWorker{moderation: moderationService})
	river.AddWorker(workers, &moderationNotifyWorker{queries: db.New(pool), content: contentStore, httpClient: &http.Client{Timeout: 3 * time.Second}})
	river.AddWorker(workers, &guestCleanupWorker{
		accounts: accountsService,
		ttl:      getenvDuration("GUEST_ACCOUNT_TTL", 24*time.Hour),
		batch:    getenvInt("GUEST_ACCOUNT_CLEANUP_BATCH_SIZE", 1000),
	})
	river.AddWorker(workers, &storageCleanupWorker{
		storage: storageStore,
		batch:   getenvInt("STORAGE_CLEANUP_BATCH_SIZE", 1000),
	})
	river.AddWorker(workers, &seasonResetWorker{seasons: seasonsStore})
	river.AddWorker(workers, &curationSweepWorker{curation: curationService})

	periodic := jobs.PeriodicJobs(jobs.PeriodicConfig{
		GuestCleanupInterval:   getenvDuration("GUEST_ACCOUNT_CLEANUP_INTERVAL", time.Hour),
		StorageCleanupInterval: getenvDuration("STORAGE_CLEANUP_INTERVAL", time.Hour),
		SeasonResetInterval:    time.Minute,
		CurationInterval:       time.Minute,
	})
	jobsClient, err := jobs.NewClient(pool, river.QueueDefault, workers, periodic)
	if err != nil {
		store.Close()
		return nil, err
	}
	return &worker{
		db:         store,
		jobs:       jobsClient,
		httpClient: &http.Client{Timeout: 3 * time.Second},
	}, nil
}

func (w *worker) close() {
	if w.db != nil {
		w.db.Close()
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

func handleWorkerShutdown(w *worker, srv *http.Server, cancel context.CancelFunc, drained chan<- struct{}) {
	defer close(drained)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	defer signal.Stop(sigCh)

	<-sigCh
	w.draining.Store(true)
	cancel()
	time.Sleep(workerDrainTimeout)

	ctx, shutdownCancel := context.WithTimeout(context.Background(), workerShutdownTimeout)
	defer shutdownCancel()
	if w.jobs != nil {
		if err := w.jobs.Stop(ctx); err != nil {
			log.Printf("moderation worker job stop failed: %v", err)
		}
	}
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

func getenvInt(k string, fallback int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getenvDuration(k string, fallback time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil && parsed > 0 {
			return parsed
		}
	}
	return fallback
}
