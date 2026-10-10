// Package pgnotify carries events between processes through Postgres. Publish sends an event with
// pg_notify, which delivers it only if the surrounding transaction commits; a Hub keeps one
// connection listening and hands each event to the local subscribers of its topic.
//
// Events are hints: a subscriber that misses one (the hub reconnecting, a slow consumer) recovers by
// reading state again. A Resync event tells it that it may have missed some.
package pgnotify

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/observability"
	db "geoduels/pkg/persistence/sqlc/db"
)

// channel is the one Postgres channel every event goes through; ListenEvents listens on it.
const channel = "gd_events"

// maxPayload stays under Postgres's 8000-byte notification limit.
const maxPayload = 7900

var ErrTooLarge = errors.New("event too large to publish")

// Event is one published event. Data is the publisher's JSON.
type Event struct {
	Topic  string          `json:"t"`
	Data   json.RawMessage `json:"d,omitempty"`
	Resync bool            `json:"-"`
}

// Topics events are published on.
func LiveTopic(userID string) string         { return "live:" + userID }
func PartyTopic(partyID string) string       { return "party:" + partyID }
func ChatTopic(conversationID string) string { return "chat:" + conversationID }

// PartyChanged is published on a party's topic whenever what its members see may have changed.
const PartyChanged = "changed"

// Publish sends data to the topic's subscribers in every process, once q's transaction commits (at
// once when q is the pool).
func Publish(ctx context.Context, q db.DBTX, topic string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(Event{Topic: topic, Data: raw})
	if err != nil {
		return err
	}
	if len(payload) > maxPayload {
		return ErrTooLarge
	}
	return db.New(q).Notify(ctx, db.NotifyParams{Channel: channel, Payload: string(payload)})
}

// Hub listens on one dedicated connection and fans events out to local subscribers.
type Hub struct {
	dsn  string
	mu   sync.Mutex
	subs map[string]map[chan Event]struct{}
}

func NewHub(dsn string) *Hub {
	return &Hub{dsn: dsn, subs: map[string]map[chan Event]struct{}{}}
}

// Subscribe receives the topic's events until cancel is called. A subscriber that falls behind
// drops events rather than stalling the hub.
func (h *Hub) Subscribe(topic string) (<-chan Event, func()) {
	ch := make(chan Event, 32)
	h.mu.Lock()
	if h.subs[topic] == nil {
		h.subs[topic] = map[chan Event]struct{}{}
	}
	h.subs[topic][ch] = struct{}{}
	h.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			h.mu.Lock()
			delete(h.subs[topic], ch)
			if len(h.subs[topic]) == 0 {
				delete(h.subs, topic)
			}
			h.mu.Unlock()
		})
	}
}

// Deliver hands an event to this process's subscribers only, as a hub hearing it would.
func (h *Hub) Deliver(event Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.subs[event.Topic] {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *Hub) resyncAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for topic, subs := range h.subs {
		for ch := range subs {
			select {
			case ch <- Event{Topic: topic, Resync: true}:
			default:
			}
		}
	}
}

// Run listens until ctx ends, reconnecting after a lost connection; every subscriber gets a Resync
// after a reconnect.
func (h *Hub) Run(ctx context.Context) {
	backoff := 250 * time.Millisecond
	for connected := false; ctx.Err() == nil; {
		err := h.listen(ctx, func() {
			if connected {
				h.resyncAll()
			}
			connected = true
			backoff = 250 * time.Millisecond
		})
		if ctx.Err() != nil {
			return
		}
		observability.Log("warn", "event listener disconnected", map[string]any{"error": errString(err)})
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 5*time.Second)
	}
}

func (h *Hub) listen(ctx context.Context, onListening func()) error {
	conn, err := pgx.Connect(ctx, h.dsn)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	if err := db.New(conn).ListenEvents(ctx); err != nil {
		return err
	}
	onListening()
	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var event Event
		if json.Unmarshal([]byte(notification.Payload), &event) != nil || event.Topic == "" {
			continue
		}
		h.Deliver(event)
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
