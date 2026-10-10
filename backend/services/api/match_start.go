package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/internal/maps"
	"geoduels/internal/matches"
	"geoduels/internal/parties"
	"geoduels/pkg/contracts"
	"geoduels/pkg/entityid"
	"geoduels/pkg/maintenance"
	"geoduels/pkg/matchkind"
	"geoduels/pkg/pgnotify"
)

// Every match starts here, whether the queue paired its players, a party's owner started it, or a
// player started one alone: the players are admitted, seated with their profiles, the map is
// planned, and the match is written for a gameplay node to pick up, all in one transaction.

type startRequest struct {
	Kind   contracts.MatchKind
	Seats  []matchkind.Seat
	Config contracts.MatchConfig
	// PartyID is the party the match was started from.
	PartyID string
	// MapAccessUserID is whose private maps the match may play.
	MapAccessUserID string
	ReturnTarget    *contracts.MatchReturnTarget
}

// admissionError says why a player may not play now.
type admissionError struct {
	UserID string
	Name   string
	Reason string
}

func (e *admissionError) Error() string {
	if e.Name != "" {
		return e.Name + ": " + e.Reason
	}
	return e.Reason
}

func (a *api) maintenanceStatus(ctx context.Context) (maintenance.Status, error) {
	readCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return maintenance.Read(readCtx, a.db.Pool())
}

// admit checks that every player may play a match of this kind now.
func (a *api) admit(ctx context.Context, spec matchkind.Spec, userIDs []string) error {
	status, err := a.maintenanceStatus(ctx)
	if err != nil {
		return err
	}
	if status.PlayBlocked() {
		return &admissionError{Reason: maintenancePlayMessage(status)}
	}
	for _, userID := range userIDs {
		identity, err := a.accounts.GetIdentity(userID)
		if err != nil {
			return &admissionError{UserID: userID, Reason: "account unavailable"}
		}
		refuse := func(reason string) error {
			return &admissionError{UserID: userID, Name: identity.DisplayName, Reason: reason}
		}
		switch {
		case identity.IsBanned:
			return refuse("account is banned")
		case identity.NicknameRequired:
			return refuse("nickname required")
		case identity.AuthMigrationRequired:
			return refuse("connect discord to continue")
		case identity.IsGuest && !spec.GuestsAllow:
			return refuse("account required")
		}
	}
	return nil
}

// startMatch starts a match in its own transaction.
func (a *api) startMatch(ctx context.Context, req startRequest) (string, error) {
	return a.startMatchIn(ctx, nil, req)
}

// startMatchIn starts a match in tx, or in a transaction of its own when tx is nil.
func (a *api) startMatchIn(ctx context.Context, tx pgx.Tx, req startRequest) (string, error) {
	spec, ok := matchkind.Lookup(req.Kind)
	if !ok {
		return "", fmt.Errorf("unknown match kind %q", req.Kind)
	}
	if err := spec.CheckSeats(req.Seats); err != nil {
		return "", err
	}
	userIDs := make([]string, len(req.Seats))
	for i, seat := range req.Seats {
		userIDs[i] = seat.UserID
	}
	if err := a.admit(ctx, spec, userIDs); err != nil {
		return "", err
	}
	matchID := entityid.New()
	params := matches.StartParams{
		MatchID: matchID, Kind: req.Kind, Config: req.Config,
		PartyID: req.PartyID, ReturnTarget: req.ReturnTarget,
	}
	for _, seat := range req.Seats {
		profile, err := a.profiles.GetProfile(seat.UserID)
		if err != nil {
			return "", fmt.Errorf("profile unavailable: %w", err)
		}
		if profile.DisplayName == "" {
			profile.DisplayName = seat.UserID
		}
		if spec.Rated && params.SeasonID == "" {
			params.SeasonID = profile.SeasonID
		}
		params.Seats = append(params.Seats, matches.Seat{Team: seat.Team, Profile: contracts.PlayerProfile{
			UserID: seat.UserID, DisplayName: profile.DisplayName, AvatarURL: profile.AvatarURL,
			MMR: profile.MMR, RatingRD: profile.RatingRD, RankedGamesPlayed: profile.RankedGamesPlayed,
			IsGuest: profile.IsGuest, IsAdmin: profile.IsAdmin, SelectedBadge: profile.SelectedBadge,
		}})
	}
	params.Plan = func(ctx context.Context, tx pgx.Tx) (matches.Plan, error) {
		plan, err := a.mapsStore.PlanMatchTx(ctx, tx, maps.MatchPlanRequest{
			MatchID: matchID, Spec: spec, Config: req.Config, MapAccessUserID: req.MapAccessUserID, Players: userIDs,
		})
		return matches.Plan{Config: plan.Config, MapID: plan.MapID}, err
	}
	params.After = func(ctx context.Context, tx pgx.Tx) error {
		if req.PartyID != "" {
			if err := parties.MarkMatchStartedTx(ctx, tx, req.PartyID, matchID); err != nil {
				return err
			}
			if err := pgnotify.Publish(ctx, tx, pgnotify.PartyTopic(req.PartyID), pgnotify.PartyChanged); err != nil {
				return err
			}
		}
		started := contracts.LiveEvent{Type: contracts.LiveMatchStarted, Match: &contracts.ActiveMatchSummary{
			MatchID: matchID, Kind: req.Kind, Mode: spec.Mode, Status: contracts.MatchViewStarting,
		}}
		for _, userID := range userIDs {
			if err := pgnotify.Publish(ctx, tx, pgnotify.LiveTopic(userID), started); err != nil {
				return err
			}
		}
		return nil
	}
	var err error
	if tx == nil {
		err = a.matchStore.Start(ctx, params)
	} else {
		err = a.matchStore.StartTx(ctx, tx, params)
	}
	if err != nil {
		return "", err
	}
	return matchID, nil
}

// startErrorMessage is what a player is told when a match could not start.
func (a *api) startErrorMessage(err error) string {
	var admission *admissionError
	if errors.As(err, &admission) {
		return admission.Error()
	}
	var conflict *matches.ConflictError
	if errors.As(err, &conflict) {
		name := conflict.UserID
		if identity, err := a.accounts.GetIdentity(conflict.UserID); err == nil && strings.TrimSpace(identity.DisplayName) != "" {
			name = identity.DisplayName
		}
		if conflict.Kind == "" {
			return name + " is already in a match"
		}
		return name + " is already in a " + strings.ReplaceAll(string(conflict.Kind), "_", " ") + " match"
	}
	return "match could not start"
}
