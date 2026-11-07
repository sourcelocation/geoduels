package moderation

import (
	"context"
	"errors"
	"geoduels/internal/inbox"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/internal/accounts"
	"geoduels/internal/storekit"
	db "geoduels/pkg/persistence/sqlc/db"
)

func (a *PGStore) UserExists(ctx context.Context, userID string) (bool, error) {
	uid, err := storekit.ProfileUUID(userID)
	if err != nil {
		return false, nil
	}
	return a.q().WarnedUserExists(ctx, uid)
}

func (a *PGStore) EvidenceSentBy(ctx context.Context, messageID, userID string) (bool, error) {
	mid, err := storekit.ProfileUUID(messageID)
	if err != nil {
		return false, nil
	}
	return a.q().WarningEvidenceBelongsToUser(ctx, db.WarningEvidenceBelongsToUserParams{ID: mid, SenderUserID: storekit.MustUUID(userID)})
}

// ResetNickname asks accounts to replace the nickname in this transaction.
func (a *PGStore) ResetNickname(ctx context.Context, userID string) (string, string, error) {
	tx, err := a.requireTx()
	if err != nil {
		return "", "", err
	}
	previous, reset, err := accounts.ResetNicknameTx(ctx, tx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return previous, reset, err
}

func (a *PGStore) InsertWarning(ctx context.Context, w Warning) error {
	return a.q().InsertWarning(ctx, db.InsertWarningParams{ID: w.ID, UserID: storekit.MustUUID(w.UserID), Category: w.Category,
		PreviousNickname: w.PreviousNickname, ResetNickname: w.ResetNickname, EvidenceMessageID: w.EvidenceMessageID, InboxMessageID: pgtype.Int8{Int64: w.InboxMessageID, Valid: w.InboxMessageID != 0}})
}

func (a *PGStore) ListWarnings(ctx context.Context, userID string) ([]Warning, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	uid, err := storekit.ProfileUUID(userID)
	if err != nil {
		return nil, ErrNotFound
	}
	rows, err := a.q().ListWarnings(ctx, uid)
	if err != nil {
		return nil, err
	}
	out := make([]Warning, 0, len(rows))
	for _, r := range rows {
		out = append(out, Warning{ID: r.ID, UserID: userID, Category: r.Category, Message: r.Message,
			PreviousNickname: r.PreviousNickname, ResetNickname: r.ResetNickname, EvidenceMessageID: r.EvidenceMessageID,
			ActorUserID: storekit.UUIDVal(r.ActorUserID), ActorName: r.ActorName, CreatedAt: r.CreatedAt.Time,
			AcknowledgedAt: optionalTime(r.AcknowledgedAt), WithdrawnAt: optionalTime(r.WithdrawnAt)})
	}
	return out, nil
}

func optionalTime(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

// WithdrawWarning retracts a warning: it leaves the player's inbox, and they're told.
func (a *PGStore) WithdrawWarning(ctx context.Context, userID string, id int64, actorID string) error {
	tx, err := a.requireTx()
	if err != nil {
		return err
	}
	uid, err := storekit.ProfileUUID(userID)
	if err != nil {
		return ErrNotFound
	}
	message, err := a.q().WithdrawWarning(ctx, db.WithdrawWarningParams{ID: id, UserID: uid, WithdrawnBy: storekit.MustUUID(actorID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if message.Valid {
		if err := inbox.DeleteTx(ctx, tx, message.Int64); err != nil {
			return err
		}
	}
	_, err = inbox.SendTx(ctx, tx, a.jobs, userID, inbox.WarningWithdrawn(id))
	return err
}

// SendWarning puts a warning in the player's inbox, returning the message that acknowledges it.
func (a *PGStore) SendWarning(ctx context.Context, userID string, id int64, reason, nicknameReset string) (int64, error) {
	tx, err := a.requireTx()
	if err != nil {
		return 0, err
	}
	return inbox.SendTx(ctx, tx, a.jobs, userID, inbox.Warned(id, reason, nicknameReset))
}
