package moderation

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"geoduels/internal/audit"
	"geoduels/pkg/contracts"
	pkgstaff "geoduels/pkg/staff"
)

var ErrInvalidWarning = errors.New("invalid warning")

// WarningInput is a staff request to warn a player, optionally replacing the
// player's nickname with a neutral one.
type WarningInput struct {
	Category          string `json:"category"`
	Message           string `json:"message"`
	ResetNickname     bool   `json:"resetNickname"`
	EvidenceMessageID string `json:"evidenceMessageId,omitempty"`
}

// Warning is one stored warning. The message lives on its audit entry.
type Warning struct {
	ID                int64
	UserID            string
	Category          string
	Message           string
	PreviousNickname  string
	ResetNickname     string
	EvidenceMessageID string
	ActorUserID       string
	ActorName         string
	CreatedAt         time.Time
	AcknowledgedAt    *time.Time
	WithdrawnAt       *time.Time
}

func (in WarningInput) normalized() (WarningInput, error) {
	in.Category = strings.TrimSpace(in.Category)
	in.Message = strings.TrimSpace(in.Message)
	in.EvidenceMessageID = strings.TrimSpace(in.EvidenceMessageID)
	switch in.Category {
	case "chat_abuse", "staff_impersonation", "inappropriate_nickname", "other":
	default:
		return WarningInput{}, ErrInvalidWarning
	}
	if in.Message == "" || utf8.RuneCountInString(in.Message) > 1000 || len(in.EvidenceMessageID) > 128 {
		return WarningInput{}, ErrInvalidWarning
	}
	return in, nil
}

// Warn records a warning and notifies the player. With ResetNickname, the
// player's public nickname is replaced in the same transaction.
func (s *Service) Warn(ctx context.Context, actor pkgstaff.Actor, userID string, in WarningInput) error {
	if err := requireReviewer(actor); err != nil {
		return err
	}
	in, err := in.normalized()
	if err != nil {
		return err
	}
	return s.store.WithinTx(ctx, func(store Store) error {
		exists, err := store.UserExists(ctx, userID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		if in.EvidenceMessageID != "" {
			sent, err := store.EvidenceSentBy(ctx, in.EvidenceMessageID, userID)
			if err != nil {
				return err
			}
			if !sent {
				return ErrInvalidWarning
			}
		}
		id, err := store.RecordAudit(ctx, audit.Entry{SubjectID: userID, ActorID: actor.ID, Action: audit.ActionWarning, Reason: in.Message,
			Metadata: map[string]any{"category": in.Category, "resetNickname": in.ResetNickname, "evidenceMessageId": in.EvidenceMessageID}})
		if err != nil {
			return err
		}
		warning := Warning{ID: id, UserID: userID, Category: in.Category, EvidenceMessageID: in.EvidenceMessageID}
		if in.ResetNickname {
			warning.PreviousNickname, warning.ResetNickname, err = store.ResetNickname(ctx, userID)
			if err != nil {
				return err
			}
		}
		if err := store.InsertWarning(ctx, warning); err != nil {
			return err
		}
		payload := map[string]any{"warningId": id, "category": in.Category, "reason": in.Message}
		if warning.ResetNickname != "" {
			payload["nicknameReset"] = warning.ResetNickname
		}
		return store.NotifyWarning(ctx, userID, id, payload)
	})
}

// Warnings is the staff view of a player's warnings.
func (s *Service) Warnings(ctx context.Context, actor pkgstaff.Actor, userID string) ([]contracts.ModerationWarning, error) {
	if err := requireReviewer(actor); err != nil {
		return nil, err
	}
	warnings, err := s.store.ListWarnings(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]contracts.ModerationWarning, 0, len(warnings))
	for _, w := range warnings {
		out = append(out, contracts.ModerationWarning{ID: w.ID, Category: w.Category, Message: w.Message,
			PreviousNickname: w.PreviousNickname, ResetNickname: w.ResetNickname, EvidenceMessageID: w.EvidenceMessageID,
			ActorUserID: w.ActorUserID, ActorName: w.ActorName, CreatedAt: w.CreatedAt, AcknowledgedAt: w.AcknowledgedAt, WithdrawnAt: w.WithdrawnAt})
	}
	return out, nil
}

// PlayerWarnings is a player's own view: no issuer or evidence details, and no
// withdrawn warnings. Callers pass the authenticated subject.
func (s *Service) PlayerWarnings(ctx context.Context, userID string) ([]contracts.PlayerWarning, error) {
	warnings, err := s.store.ListWarnings(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]contracts.PlayerWarning, 0, len(warnings))
	for _, w := range warnings {
		if w.WithdrawnAt != nil {
			continue
		}
		out = append(out, contracts.PlayerWarning{ID: w.ID, Category: w.Category, Message: w.Message,
			ResetNickname: w.ResetNickname, CreatedAt: w.CreatedAt, AcknowledgedAt: w.AcknowledgedAt})
	}
	return out, nil
}

// AcknowledgeWarning marks the player's own warning as read. Repeating it
// keeps the first timestamp.
func (s *Service) AcknowledgeWarning(ctx context.Context, userID string, id int64) error {
	return s.store.AcknowledgeWarning(ctx, userID, id)
}

// WithdrawWarning retracts a warning. A reset nickname is not restored.
func (s *Service) WithdrawWarning(ctx context.Context, actor pkgstaff.Actor, userID string, id int64) error {
	if err := requireReviewer(actor); err != nil {
		return err
	}
	return s.store.WithinTx(ctx, func(store Store) error {
		if err := store.WithdrawWarning(ctx, userID, id, actor.ID); err != nil {
			return err
		}
		if _, err := store.RecordAudit(ctx, audit.Entry{SubjectID: userID, ActorID: actor.ID, Action: audit.ActionNote, Reason: "Warning withdrawn", Metadata: map[string]any{"warningId": id}}); err != nil {
			return err
		}
		return store.NotifyWarning(ctx, userID, id, map[string]any{"warningId": id, "withdrawn": true, "reason": "This warning has been withdrawn."})
	})
}
