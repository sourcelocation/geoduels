// Package inbox holds what people are told about their account: messages with no actions, in the
// words they read. The server writes every word, so a new kind needs no app update, and nothing that
// reads the inbox cares which kind a message is.
package inbox

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/internal/jobs"
	"geoduels/internal/storekit"
	db "geoduels/pkg/persistence/sqlc/db"
)

// Stored is a message as it was sent.
type Stored struct {
	ID        int64
	Kind      string
	Title     string
	Body      string
	ImageURL  string
	Modal     bool
	ReadAt    *time.Time
	CreatedAt time.Time
}

func stored(id int64, kind, title, body, image string, modal bool, readAt, createdAt pgtype.Timestamptz) Stored {
	out := Stored{ID: id, Kind: kind, Title: title, Body: body, ImageURL: image, Modal: modal, CreatedAt: createdAt.Time}
	if readAt.Valid {
		value := readAt.Time
		out.ReadAt = &value
	}
	return out
}

// SendTx records the message inside the caller's transaction, with a job that tells the person once
// it commits. It returns the message's id, or 0 if this message was already sent.
func SendTx(ctx context.Context, tx pgx.Tx, teller jobs.Teller, userID string, m Message) (int64, error) {
	user, err := storekit.ProfileUUID(userID)
	if err != nil {
		return 0, err
	}
	id, err := db.New(tx).InsertInboxMessage(ctx, db.InsertInboxMessageParams{
		UserID: user, Kind: m.kind, DedupeKey: m.dedupeKey, Title: m.title, Body: m.body, ImageUrl: m.imageURL, Modal: m.modal,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return id, teller.EnqueueInboxChanged(ctx, tx, userID, id)
}

// DeleteTx removes a message that no longer applies, such as a withdrawn warning.
func DeleteTx(ctx context.Context, tx pgx.Tx, id int64) error {
	return db.New(tx).DeleteInboxMessage(ctx, id)
}

// Page is one page of a person's messages, newest first; before is the id to continue from, or 0.
func Page(ctx context.Context, q db.DBTX, userID string, limit int, before int64) ([]Stored, error) {
	user, err := storekit.ProfileUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := db.New(q).ListInboxMessages(ctx, db.ListInboxMessagesParams{UserID: user, BeforeID: before, RowLimit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]Stored, len(rows))
	for i, r := range rows {
		out[i] = stored(r.ID, r.Kind, r.Title, r.Body, r.ImageUrl, r.Modal, r.ReadAt, r.CreatedAt)
	}
	return out, nil
}

// Get is one of the person's messages.
func Get(ctx context.Context, q db.DBTX, userID string, id int64) (Stored, error) {
	user, err := storekit.ProfileUUID(userID)
	if err != nil {
		return Stored{}, err
	}
	r, err := db.New(q).GetInboxMessage(ctx, db.GetInboxMessageParams{ID: id, UserID: user})
	if err != nil {
		return Stored{}, err
	}
	return stored(r.ID, r.Kind, r.Title, r.Body, r.ImageUrl, r.Modal, r.ReadAt, r.CreatedAt), nil
}

// Unread counts the messages the person hasn't read.
func Unread(ctx context.Context, q db.DBTX, userID string) (int, error) {
	user, err := storekit.ProfileUUID(userID)
	if err != nil {
		return 0, err
	}
	count, err := db.New(q).CountUnreadInboxMessages(ctx, user)
	return int(count), err
}

// MarkRead marks one message read. For a warning, this is acknowledging it.
func MarkRead(ctx context.Context, q db.DBTX, userID string, id int64) error {
	user, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	return db.New(q).MarkInboxMessageRead(ctx, db.MarkInboxMessageReadParams{ID: id, UserID: user})
}

// MarkAllRead marks everything read, except what needs reading on its own.
func MarkAllRead(ctx context.Context, q db.DBTX, userID string) error {
	user, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	return db.New(q).MarkAllInboxMessagesRead(ctx, user)
}
