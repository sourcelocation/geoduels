package moderation

import (
	"context"
	"errors"
	"fmt"
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
		PreviousNickname: w.PreviousNickname, ResetNickname: w.ResetNickname, EvidenceMessageID: w.EvidenceMessageID})
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

func (a *PGStore) AcknowledgeWarning(ctx context.Context, userID string, id int64) error {
	uid, err := storekit.ProfileUUID(userID)
	if err != nil {
		return ErrNotFound
	}
	count, err := a.q().AcknowledgeWarning(ctx, db.AcknowledgeWarningParams{ID: id, UserID: uid})
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (a *PGStore) WithdrawWarning(ctx context.Context, userID string, id int64, actorID string) error {
	uid, err := storekit.ProfileUUID(userID)
	if err != nil {
		return ErrNotFound
	}
	count, err := a.q().WithdrawWarning(ctx, db.WithdrawWarningParams{ID: id, UserID: uid, WithdrawnBy: storekit.MustUUID(actorID)})
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

// NotifyWarning creates or updates the player's inbox entry for a warning.
func (a *PGStore) NotifyWarning(ctx context.Context, userID string, id int64, payload map[string]any) error {
	tx, err := a.requireTx()
	if err != nil {
		return err
	}
	var notificationID int64
	return storekit.UpsertUserNotificationTx(ctx, tx, userID, "moderation_warning", fmt.Sprintf("warning:%d", id), payload, "", &notificationID)
}
