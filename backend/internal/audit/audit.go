// Package audit owns the staff audit log. Modules record their staff actions
// through Record inside their own transaction instead of writing
// moderation_log directly.
package audit

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/internal/storekit"
	"geoduels/pkg/contracts"
	db "geoduels/pkg/persistence/sqlc/db"
)

// Action identifies an audited staff action. Values match the
// gd_moderation_log_action enum.
type Action string

const (
	ActionRoleGranted  Action = "role_granted"
	ActionRoleRevoked  Action = "role_revoked"
	ActionBadgeGranted Action = "badge_granted"
	ActionPermanentBan Action = "permanent_ban"
	ActionTemporaryBan Action = "temporary_ban"
	ActionChatMute     Action = "chat_mute"
	ActionChatUnmute   Action = "chat_unmute"
	ActionReportMute   Action = "report_mute"
	ActionReportUnmute Action = "report_unmute"
	ActionUnban        Action = "unban"
	ActionRefund       Action = "refund"
	ActionNote         Action = "note"
	ActionWarning      Action = "warning"
)

// Entry is one record on the unified staff audit log.
type Entry struct {
	ActorID   string
	SubjectID string
	Action    Action
	Reason    string
	ExpiresAt *time.Time
	Metadata  any
}

// Record appends entry using conn, normally the caller's transaction, so the
// audit row commits or rolls back with the action it describes.
func Record(ctx context.Context, conn db.DBTX, entry Entry) (int64, error) {
	metadata := []byte("{}")
	if entry.Metadata != nil {
		if encoded, err := json.Marshal(entry.Metadata); err == nil {
			metadata = encoded
		}
	}
	expiresAt := pgtype.Timestamptz{}
	if entry.ExpiresAt != nil {
		expiresAt = storekit.Timestamptz(*entry.ExpiresAt)
	}
	return db.New(conn).InsertModerationLog(ctx, db.InsertModerationLogParams{
		SubjectUserID: storekit.MustUUID(strings.TrimSpace(entry.SubjectID)),
		ActorUserID:   strings.TrimSpace(entry.ActorID),
		Action:        db.GdModerationLogAction(entry.Action),
		Reason:        strings.TrimSpace(entry.Reason),
		ExpiresAt:     expiresAt,
		Metadata:      metadata,
	})
}

// List returns the newest entries, optionally for one subject.
func List(ctx context.Context, conn db.DBTX, subjectID string, limit int) ([]contracts.ModerationAuditLogEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	subject := pgtype.UUID{}
	if strings.TrimSpace(subjectID) != "" {
		u, err := storekit.ProfileUUID(subjectID)
		if err != nil {
			return nil, err
		}
		subject = u
	}
	rows, err := db.New(conn).ListModerationLog(ctx, db.ListModerationLogParams{SubjectUserID: subject, RowLimit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]contracts.ModerationAuditLogEntry, 0, len(rows))
	for _, x := range rows {
		out = append(out, contracts.ModerationAuditLogEntry{
			ID: x.LogID, SubjectUserID: storekit.UUIDVal(x.SubjectUserID), SubjectName: storekit.TextVal(x.SubjectDisplayName),
			ActorUserID: storekit.UUIDVal(x.ActorUserID), ActorName: storekit.TextVal(x.ActorDisplayName), Action: string(x.Action),
			Reason: x.Reason, ExpiresAt: x.ExpiresAt.Time, SignalIDs: x.SignalIds, Metadata: json.RawMessage(x.Metadata), CreatedAt: x.CreatedAt.Time,
		})
	}
	return out, nil
}
