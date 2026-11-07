// Package live carries events to people. Anything that has something to tell a person publishes it to
// that person's subject after committing; whichever gateway holds their connection forwards it. The
// gateway never reads an event, and nothing is kept: Postgres has the truth, and every connection
// starts from a full session, so an event that gets lost only delays what someone sees.
package live

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
)

// Event is something a person can be told. Each kind is its own type, marshalled as the event's data.
type Event interface {
	EventType() string
}

// Encode is an event as clients receive it.
func Encode(event Event) ([]byte, error) {
	return json.Marshal(struct {
		Type string `json:"type"`
		Data Event  `json:"data"`
	}{event.EventType(), event})
}

// Publisher tells people things.
type Publisher interface {
	Tell(userIDs []string, event Event) error
	Disconnect(userID string) error
}

func userSubject(userID string) string    { return "user." + userID }
func controlSubject(userID string) string { return "control.user." + userID }

// Bus is the connection to NATS.
type Bus struct {
	conn *nats.Conn
}

// Connect keeps trying for as long as the process lives. onReconnect runs each time the connection
// comes back, when anything published meanwhile is gone.
func Connect(url string, onReconnect func()) (*Bus, error) {
	if url == "" {
		return nil, errors.New("NATS_URL is required")
	}
	conn, err := nats.Connect(url,
		nats.MaxReconnects(-1),
		nats.RetryOnFailedConnect(true),
		nats.ReconnectWait(time.Second),
		nats.ReconnectHandler(func(*nats.Conn) { onReconnect() }),
	)
	if err != nil {
		return nil, err
	}
	return &Bus{conn: conn}, nil
}

func (b *Bus) Close() { b.conn.Close() }

func (b *Bus) Tell(userIDs []string, event Event) error {
	payload, err := Encode(event)
	if err != nil {
		return err
	}
	var errs []error
	for _, userID := range userIDs {
		if userID != "" {
			errs = append(errs, b.conn.Publish(userSubject(userID), payload))
		}
	}
	return errors.Join(errs...)
}

// Disconnect closes the person's connections, as when they're banned or sign out everywhere.
func (b *Bus) Disconnect(userID string) error {
	return b.conn.Publish(controlSubject(userID), nil)
}

// Subscription is a connection's interest in one person.
type Subscription struct {
	events, control *nats.Subscription
}

// Subscribe delivers the person's events as they were encoded. Both callbacks must not block.
func (b *Bus) Subscribe(userID string, onEvent func([]byte), onDisconnect func()) (*Subscription, error) {
	events, err := b.conn.Subscribe(userSubject(userID), func(message *nats.Msg) { onEvent(message.Data) })
	if err != nil {
		return nil, err
	}
	control, err := b.conn.Subscribe(controlSubject(userID), func(*nats.Msg) { onDisconnect() })
	if err != nil {
		_ = events.Unsubscribe()
		return nil, err
	}
	return &Subscription{events: events, control: control}, nil
}

func (s *Subscription) Close() {
	_ = s.events.Unsubscribe()
	_ = s.control.Unsubscribe()
}
