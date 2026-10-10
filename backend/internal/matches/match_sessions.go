package matches

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/internal/storekit"
	"geoduels/pkg/contracts"
	"geoduels/pkg/matchkind"
	db "geoduels/pkg/persistence/sqlc/db"
)

// A match's life, all in its match_sessions row: Start writes it starting; a gameplay node picks it
// up and stamps it with its id and lease token, which makes it live while that lease holds; it ends
// when the node finalizes it, when a player replaces it, or when the sweep finds its node gone.

// Seat is one player's place in a new match, with the profile they play under.
type Seat struct {
	Profile contracts.PlayerProfile `json:"profile"`
	Team    string                  `json:"team,omitempty"`
}

// LaunchSpec is what a node needs to run a match, beyond its kind, config and round plan.
type LaunchSpec struct {
	SeasonID string `json:"seasonId,omitempty"`
	Seats    []Seat `json:"seats"`
}

// Plan is the map a match plays, chosen inside its start transaction.
type Plan struct {
	Config contracts.MatchConfig
	MapID  string
}

type StartParams struct {
	MatchID      string
	Kind         contracts.MatchKind
	Seats        []Seat
	SeasonID     string
	Config       contracts.MatchConfig
	PartyID      string
	ReturnTarget *contracts.MatchReturnTarget
	// Plan picks the map and writes the round plan in the start transaction.
	Plan func(ctx context.Context, tx pgx.Tx) (Plan, error)
	// After runs in the start transaction once the match is written.
	After func(ctx context.Context, tx pgx.Tx) error
}

// ConflictError says a player is already in a match that a new one cannot replace.
type ConflictError struct {
	UserID  string
	MatchID string
	Kind    contracts.MatchKind
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("player %s is already in %s match %s", e.UserID, e.Kind, e.MatchID)
}

// Outcomes a match ends with, besides finished.
const (
	OutcomeReplaced    = "replaced"
	OutcomeAborted     = "aborted"
	OutcomeInterrupted = "interrupted"
)

// Start writes a new match and seats its players, all in one transaction. A player's other match
// that is over is closed; one whose kind is replaceable ends as replaced; any other is a conflict.
func (s *PGStore) Start(ctx context.Context, p StartParams) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return s.withTx(ctx, func(tx pgx.Tx) error { return s.StartTx(ctx, tx, p) })
}

// StartTx is Start in the caller's transaction.
func (s *PGStore) StartTx(ctx context.Context, tx pgx.Tx, p StartParams) error {
	spec, ok := matchkind.Lookup(p.Kind)
	if !ok {
		return fmt.Errorf("unknown match kind %q", p.Kind)
	}
	seats := make([]matchkind.Seat, len(p.Seats))
	userIDs := make([]string, len(p.Seats))
	for i, seat := range p.Seats {
		seats[i] = matchkind.Seat{UserID: seat.Profile.UserID, Team: seat.Team}
		userIDs[i] = seat.Profile.UserID
	}
	if err := spec.CheckSeats(seats); err != nil {
		return err
	}
	matchUUID, err := profileUUID(p.MatchID)
	if err != nil {
		return errors.New("match id required")
	}
	{
		q := s.db.WithTx(tx)
		if err := clearSeatsTx(ctx, q, userIDs); err != nil {
			return err
		}
		plan := Plan{Config: contracts.NormalizeMatchConfig(p.Config)}
		if p.Plan != nil {
			if plan, err = p.Plan(ctx, tx); err != nil {
				return err
			}
		}
		config, _ := json.Marshal(contracts.NormalizeMatchConfig(plan.Config))
		launch, _ := json.Marshal(LaunchSpec{SeasonID: p.SeasonID, Seats: p.Seats})
		target := contracts.NormalizeMatchReturnTarget(p.ReturnTarget)
		partyID, err := storekit.OptionalUUID(p.PartyID)
		if err != nil {
			return err
		}
		mapID, err := storekit.OptionalUUID(plan.MapID)
		if err != nil {
			return err
		}
		returnMap, err := storekit.OptionalUUID(target.MapID)
		if err != nil {
			return err
		}
		returnParty, err := storekit.OptionalUUID(target.PartyID)
		if err != nil {
			return err
		}
		if err := q.InsertMatchSession(ctx, db.InsertMatchSessionParams{
			MatchID: matchUUID, Kind: db.GdMatchKind(p.Kind), SourcePartyID: partyID,
			ConfigJson: config, MapID: mapID, SpecJson: launch,
			ReturnTargetKind: string(target.Kind), ReturnTargetMapID: returnMap, ReturnTargetPartyID: returnParty,
		}); err != nil {
			return err
		}
		for _, seat := range p.Seats {
			userUUID, err := profileUUID(seat.Profile.UserID)
			if err != nil {
				return err
			}
			joined := pgtype.Timestamptz{}
			if partyID.Valid {
				t, err := q.ParticipantJoinedAt(ctx, db.ParticipantJoinedAtParams{PartyID: partyID, UserID: userUUID})
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return err
				}
				joined = t
			}
			seated, err := q.InsertMatchSeat(ctx, db.InsertMatchSeatParams{
				MatchID: matchUUID, UserID: userUUID,
				TeamID:      pgtype.Text{String: seat.Team, Valid: seat.Team != ""},
				DisplayName: seat.Profile.DisplayName, AvatarUrl: seat.Profile.AvatarURL, JoinedPartyAt: joined,
			})
			if err != nil {
				return err
			}
			if seated == 0 {
				// Another start seated this player after the seats were cleared.
				return &ConflictError{UserID: seat.Profile.UserID}
			}
		}
		if p.After != nil {
			return p.After(ctx, tx)
		}
		return nil
	}
}

// clearSeatsTx frees the players' seats in matches that are over and ends the replaceable ones still
// running, or reports the first match that blocks a new one.
func clearSeatsTx(ctx context.Context, q *db.Queries, userIDs []string) error {
	rows, err := q.LockActiveSeats(ctx, storekit.UUIDList(userIDs))
	if err != nil {
		return err
	}
	for _, row := range rows {
		kind := contracts.MatchKind(row.Kind)
		outcome, blocks := SeatOutcome(contracts.MatchSessionStatus(row.Status), kind)
		if blocks {
			return &ConflictError{UserID: storekit.UUIDVal(row.UserID), MatchID: storekit.UUIDVal(row.MatchID), Kind: kind}
		}
		if _, err := q.EndMatch(ctx, db.EndMatchParams{MatchID: row.MatchID, Outcome: pgtype.Text{String: outcome, Valid: true}}); err != nil {
			return err
		}
	}
	return nil
}

// SeatOutcome decides what happens to a player's seat in another match when a new one starts: a
// match that is over just closes (interrupted, if nothing ended it); a replaceable one still
// running ends as replaced; any other running match blocks the new one.
func SeatOutcome(status contracts.MatchSessionStatus, kind contracts.MatchKind) (outcome string, blocks bool) {
	if !status.Open() {
		return OutcomeInterrupted, false
	}
	if matchkind.Of(kind).Replaceable {
		return OutcomeReplaced, false
	}
	return "", true
}

// EndMatch ends an open match with an outcome and frees its seats. It reports whether it ended it.
func (s *PGStore) EndMatch(ctx context.Context, matchID, outcome string) (bool, error) {
	id, err := profileUUID(matchID)
	if err != nil {
		return false, err
	}
	ended, err := s.db.EndMatch(ctx, db.EndMatchParams{MatchID: id, Outcome: pgtype.Text{String: outcome, Valid: true}})
	return ended > 0, err
}

// EndInterruptedMatchesTx records the end of the matches no node picked up in time and those whose
// node is gone, in the caller's transaction. Nothing waits on it: a new match already frees its
// players' seats in such matches, and a party reads them as over from their status; it only lets
// the rows be cleaned up.
func EndInterruptedMatchesTx(ctx context.Context, tx pgx.Tx) (int64, error) {
	return db.New(tx).EndInterruptedMatches(ctx)
}

// Node is a gameplay node process: its id, the fencing token of its lease, and where it serves.
type Node struct {
	ID    string
	Epoch int64
	URL   string
}

// PickedUp is a match a node took.
type PickedUp struct {
	MatchID string
	Kind    contracts.MatchKind
	Config  contracts.MatchConfig
	Spec    LaunchSpec
}

// PickUp gives the oldest match waiting for a node to run, and stamps it with the node once run
// accepts it. It returns the match's id, empty when none was waiting; on an error after run
// accepted it, the caller must let that match go, since another node may pick it up.
func (s *PGStore) PickUp(ctx context.Context, node Node, run func(PickedUp) error) (string, error) {
	accepted := ""
	err := s.withTx(ctx, func(tx pgx.Tx) error {
		q := s.db.WithTx(tx)
		row, err := q.PickUpWaitingMatch(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		match := PickedUp{MatchID: storekit.UUIDVal(row.MatchID), Kind: contracts.MatchKind(row.Kind)}
		if err := json.Unmarshal(row.ConfigJson, &match.Config); err != nil {
			return err
		}
		match.Config = contracts.NormalizeMatchConfig(match.Config)
		if err := json.Unmarshal(row.SpecJson, &match.Spec); err != nil {
			return err
		}
		if err := run(match); err != nil {
			return err
		}
		accepted = match.MatchID
		return q.PlaceMatch(ctx, db.PlaceMatchParams{
			MatchID: row.MatchID, NodeID: pgtype.Text{String: node.ID, Valid: true},
			NodeEpoch: pgtype.Int8{Int64: node.Epoch, Valid: true}, NodeUrl: pgtype.Text{String: node.URL, Valid: true},
		})
	})
	return accepted, err
}

// NodeOpenMatches lists the matches the node process runs that have not ended.
func (s *PGStore) NodeOpenMatches(ctx context.Context, node Node) (map[string]bool, error) {
	rows, err := s.db.ListNodeOpenMatches(ctx, db.ListNodeOpenMatchesParams{
		NodeID: pgtype.Text{String: node.ID, Valid: true}, NodeEpoch: pgtype.Int8{Int64: node.Epoch, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rows))
	for _, row := range rows {
		out[storekit.UUIDVal(row)] = true
	}
	return out, nil
}

// MatchSessionStatus says whether a match is starting, live, ended or interrupted;
// MatchSessionMissing when it has no session (never started, or cleaned up).
func (s *PGStore) MatchSessionStatus(ctx context.Context, matchID string) (contracts.MatchSessionStatus, error) {
	id, err := profileUUID(matchID)
	if err != nil {
		return contracts.MatchSessionMissing, errors.New("matchID required")
	}
	status, err := s.db.GetMatchSessionStatus(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.MatchSessionMissing, nil
	}
	if err != nil {
		return contracts.MatchSessionMissing, err
	}
	return contracts.MatchSessionStatus(status), nil
}

// Session is a match's live record.
type Session struct {
	MatchID      string
	Kind         contracts.MatchKind
	Status       contracts.MatchSessionStatus
	NodeURL      string
	Config       contracts.MatchConfig
	PartyID      string
	ReturnTarget *contracts.MatchReturnTarget
	CreatedAt    time.Time
	Outcome      string
	Seats        []SessionSeat
}

type SessionSeat struct {
	UserID      string
	Team        string
	DisplayName string
	AvatarURL   string
	Active      bool
}

// Seated reports whether the user plays in the match.
func (s Session) Seated(userID string) bool {
	for _, seat := range s.Seats {
		if seat.UserID == userID {
			return true
		}
	}
	return false
}

// GetSession reads a match's live record; false once it is gone.
func (s *PGStore) GetSession(ctx context.Context, matchID string) (Session, bool, error) {
	id, err := profileUUID(matchID)
	if err != nil {
		return Session{}, false, nil
	}
	row, err := s.db.GetMatchSession(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, false, nil
	}
	if err != nil {
		return Session{}, false, err
	}
	session := Session{
		MatchID: storekit.UUIDVal(row.MatchID), Kind: contracts.MatchKind(row.Kind),
		Status: contracts.MatchSessionStatus(row.Status), NodeURL: row.NodeUrl,
		PartyID: storekit.UUIDVal(row.SourcePartyID), CreatedAt: row.CreatedAt.Time, Outcome: row.Outcome,
		ReturnTarget: contracts.NormalizeMatchReturnTarget(&contracts.MatchReturnTarget{
			Kind:    contracts.MatchReturnTargetKind(row.ReturnTargetKind),
			MapID:   storekit.UUIDVal(row.ReturnTargetMapID),
			PartyID: storekit.UUIDVal(row.ReturnTargetPartyID),
		}),
	}
	if err := json.Unmarshal(row.ConfigJson, &session.Config); err != nil {
		return Session{}, false, err
	}
	session.Config = contracts.NormalizeMatchConfig(session.Config)
	seats, err := s.db.ListMatchSeats(ctx, id)
	if err != nil {
		return Session{}, false, err
	}
	for _, seat := range seats {
		session.Seats = append(session.Seats, SessionSeat{
			UserID: storekit.UUIDVal(seat.UserID), Team: seat.TeamID, DisplayName: seat.DisplayName,
			AvatarURL: seat.AvatarUrl, Active: seat.Active,
		})
	}
	return session, true, nil
}

// ActiveMatch is the match a player is in now.
type ActiveMatch struct {
	MatchID string
	Kind    contracts.MatchKind
	Status  contracts.MatchSessionStatus
}

// ActiveMatchForUser returns the starting or live match the player is in, if any.
func (s *PGStore) ActiveMatchForUser(ctx context.Context, userID string) (ActiveMatch, bool, error) {
	id, err := profileUUID(userID)
	if err != nil {
		return ActiveMatch{}, false, nil
	}
	row, err := s.db.GetActiveMatchForUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ActiveMatch{}, false, nil
	}
	if err != nil {
		return ActiveMatch{}, false, err
	}
	return ActiveMatch{MatchID: storekit.UUIDVal(row.MatchID), Kind: contracts.MatchKind(row.Kind), Status: contracts.MatchSessionStatus(row.Status)}, true, nil
}

// UsersInLiveMatches says which of the players are in a starting or live match.
func (s *PGStore) UsersInLiveMatches(ctx context.Context, userIDs []string) (map[string]bool, error) {
	out := map[string]bool{}
	ids := make([]string, 0, len(userIDs))
	for _, id := range userIDs {
		if _, err := profileUUID(id); err == nil {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := s.db.ListUsersInLiveMatches(ctx, storekit.UUIDList(ids))
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[storekit.UUIDVal(row)] = true
	}
	return out, nil
}

// SeatTeams maps each seated player to their team, for engines that need it.
func (spec LaunchSpec) SeatTeams() map[string]string {
	teams := map[string]string{}
	for _, seat := range spec.Seats {
		if team := strings.TrimSpace(seat.Team); team != "" {
			teams[seat.Profile.UserID] = team
		}
	}
	return teams
}
