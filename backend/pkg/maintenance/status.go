package maintenance

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	db "geoduels/pkg/persistence/sqlc/db"
)

// settingKey is the site setting that holds the status.
const settingKey = "maintenance"

type Phase string

const (
	PhaseNormal  Phase = "normal"
	PhaseWarning Phase = "warning"
	PhaseActive  Phase = "active"
)

type Status struct {
	Phase       Phase      `json:"phase"`
	StartsAt    *time.Time `json:"startsAt,omitempty"`
	EndsAt      *time.Time `json:"endsAt,omitempty"`
	QueuePaused bool       `json:"queuePaused,omitempty"`
	PlayPaused  bool       `json:"playPaused,omitempty"`
	Message     string     `json:"message,omitempty"`
}

func DefaultStatus() Status {
	return Status{Phase: PhaseNormal}
}

func (s Status) Normalized() Status {
	switch s.Phase {
	case PhaseNormal, PhaseWarning, PhaseActive:
	default:
		s.Phase = PhaseNormal
	}
	s.Message = strings.TrimSpace(s.Message)
	return s
}

func (s Status) IsVisible() bool {
	return s.Phase != PhaseNormal || s.QueuePaused || s.PlayPaused || s.Message != "" || s.StartsAt != nil || s.EndsAt != nil
}

func (s Status) QueueBlocked() bool {
	return s.QueuePaused || s.PlayPaused
}

func (s Status) PlayBlocked() bool {
	return s.PlayPaused
}

// Read returns the maintenance status every service follows; normal when none was set.
func Read(ctx context.Context, q db.DBTX) (Status, error) {
	raw, err := db.New(q).GetSetting(ctx, settingKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return DefaultStatus(), nil
	}
	if err != nil {
		return DefaultStatus(), err
	}
	var status Status
	if err := json.Unmarshal(raw, &status); err != nil {
		return DefaultStatus(), err
	}
	return status.Normalized(), nil
}
