package main

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"geoduels/internal/accounts"
	"geoduels/internal/moderation"
)

// moderationWarning is a warning as staff review it.
type moderationWarning struct {
	ID                int64      `json:"id"`
	Category          string     `json:"category"`
	Message           string     `json:"message"`
	PreviousNickname  string     `json:"previousNickname,omitempty"`
	ResetNickname     string     `json:"resetNickname,omitempty"`
	EvidenceMessageID string     `json:"evidenceMessageId,omitempty"`
	ActorUserID       string     `json:"actorUserId,omitempty"`
	ActorName         string     `json:"actorName,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	AcknowledgedAt    *time.Time `json:"acknowledgedAt,omitempty"`
	WithdrawnAt       *time.Time `json:"withdrawnAt,omitempty"`
}

// playerWarning is a warning as the warned player sees it.
type playerWarning struct {
	ID             int64      `json:"id"`
	Category       string     `json:"category"`
	Message        string     `json:"message"`
	ResetNickname  string     `json:"resetNickname,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
}

// Warning routes are v1 contracts; existing moderation routes remain compatible.
func (a *api) staffWarnings(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	warnings, err := a.moderation.Warnings(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id")))
	if err != nil {
		return staffError(c, err)
	}
	out := make([]moderationWarning, len(warnings))
	for i, w := range warnings {
		out[i] = moderationWarning{
			ID: w.ID, Category: w.Category, Message: w.Message, PreviousNickname: w.PreviousNickname,
			ResetNickname: w.ResetNickname, EvidenceMessageID: w.EvidenceMessageID, ActorUserID: w.ActorUserID,
			ActorName: w.ActorName, CreatedAt: w.CreatedAt, AcknowledgedAt: w.AcknowledgedAt, WithdrawnAt: w.WithdrawnAt,
		}
	}
	return writeJSON(c, map[string]any{"warnings": out})
}

func (a *api) staffWarn(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	var input moderation.WarningInput
	if err := decodeJSONBody(c.Request(), &input); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	err = a.moderation.Warn(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id")), input)
	if errors.Is(err, moderation.ErrInvalidWarning) {
		return plainTextError(c, http.StatusBadRequest, "choose a category and enter a message of 1–1000 characters; evidence must belong to this player")
	}
	if errors.Is(err, accounts.ErrGuestNickname) {
		return plainTextError(c, http.StatusBadRequest, "guests always play as Guest; their nickname cannot be reset")
	}
	if err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusCreated)
}

func warningID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("warningId"), 10, 64)
	if err != nil || id <= 0 {
		return 0, moderation.ErrInvalidWarning
	}
	return id, nil
}

func (a *api) staffWithdrawWarning(c echo.Context) error {
	actor, err := a.staffActor(c.Request())
	if err != nil {
		return staffError(c, err)
	}
	id, err := warningID(c)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid warning id")
	}
	if err := a.moderation.WithdrawWarning(c.Request().Context(), actor, a.resolveEntityID("user", c.Param("id")), id); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) userWarnings(c echo.Context) error {
	claims, _, err := a.authenticatedAccount(c.Request())
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	warnings, err := a.moderation.PlayerWarnings(c.Request().Context(), claims.Sub)
	if err != nil {
		return staffError(c, err)
	}
	out := make([]playerWarning, len(warnings))
	for i, w := range warnings {
		out[i] = playerWarning{
			ID: w.ID, Category: w.Category, Message: w.Message, ResetNickname: w.ResetNickname,
			CreatedAt: w.CreatedAt, AcknowledgedAt: w.AcknowledgedAt,
		}
	}
	return writeJSON(c, map[string]any{"warnings": out})
}

func (a *api) acknowledgeWarning(c echo.Context) error {
	claims, _, err := a.authenticatedAccount(c.Request())
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	id, err := warningID(c)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid warning id")
	}
	if err := a.moderation.AcknowledgeWarning(c.Request().Context(), claims.Sub, id); err != nil {
		return staffError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}
