package matchlaunch

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"geoduels/pkg/contracts"
	"geoduels/pkg/coordinator"
	"geoduels/pkg/gameticket"
	"geoduels/pkg/persistence"
	"geoduels/pkg/sessionpolicy"
)

type AssignmentStatus string

const (
	AssignmentInvalid   AssignmentStatus = "invalid"
	AssignmentValid     AssignmentStatus = "valid"
	AssignmentPending   AssignmentStatus = "pending"
	AssignmentAbandoned AssignmentStatus = "abandoned"
)

type Launcher struct {
	Coord          *coordinator.Store
	Persist        persistence.Store
	HTTPClient     *http.Client
	TicketSecret   []byte
	InternalSecret string
}

func (l Launcher) ValidateAssignment(ctx context.Context, assigned coordinator.Assignment) AssignmentStatus {
	if assigned.MatchID == "" {
		return AssignmentInvalid
	}
	if assigned.RecoverableUntil > 0 && time.Now().UnixMilli() > assigned.RecoverableUntil {
		_ = l.clearEnded(context.Background(), assigned)
		return AssignmentInvalid
	}
	if l.matchEnded(assigned.MatchID) {
		_ = l.Coord.ClearAssignment(context.Background(), assigned)
		return AssignmentInvalid
	}
	node, ok, err := l.Coord.GetNodeByRoute(ctx, assigned.PublicRoute)
	if err != nil {
		return AssignmentPending
	}
	if !ok {
		if assigned.NodeEpoch > 0 {
			_ = l.clearEnded(context.Background(), assigned)
			return AssignmentAbandoned
		}
		return AssignmentPending
	}
	if assigned.NodeEpoch > 0 && node.OwnerEpoch != assigned.NodeEpoch {
		_ = l.clearEnded(context.Background(), assigned)
		return AssignmentAbandoned
	}
	client := l.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	probeCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodHead, strings.TrimRight(node.InternalURL, "/")+"/internal/matches/"+url.PathEscape(assigned.MatchID), nil)
	if err != nil {
		return AssignmentPending
	}
	req.Header.Set("X-Coordinator-Secret", l.InternalSecret)
	resp, err := client.Do(req)
	if err != nil {
		return AssignmentPending
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return AssignmentValid
	case http.StatusNotFound:
		_ = l.clearEnded(context.Background(), assigned)
		return AssignmentAbandoned
	default:
		return AssignmentPending
	}
}

func (l Launcher) EnsureAssignment(ctx context.Context, found contracts.MatchFound) (coordinator.Assignment, error) {
	found.Mode = sessionpolicy.NormalizeMode(found.Mode, found.MatchID)
	found.Config = contracts.NormalizeMatchConfig(found.Config)
	if assigned, ok, err := l.Coord.GetAssignmentByMatch(ctx, found.MatchID); err == nil && ok {
		return assigned, nil
	}
	holder := newLockHolder(found.MatchID)
	locked, err := l.Coord.TryLockMatch(ctx, found.MatchID, holder)
	if err != nil {
		return coordinator.Assignment{}, err
	}
	if !locked {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if assigned, ok, err := l.Coord.GetAssignmentByMatch(ctx, found.MatchID); err == nil && ok {
				return assigned, nil
			}
			if err := ctx.Err(); err != nil {
				return coordinator.Assignment{}, err
			}
			time.Sleep(100 * time.Millisecond)
		}
		return coordinator.Assignment{}, errors.New("assignment timed out")
	}
	defer func() {
		_ = l.Coord.UnlockMatch(context.Background(), found.MatchID, holder)
	}()
	if planner, ok := l.Persist.(interface {
		PrepareMatchPlan(context.Context, *contracts.MatchFound) error
	}); ok {
		if err := planner.PrepareMatchPlan(ctx, &found); err != nil {
			return coordinator.Assignment{}, err
		}
	}

	nodes, err := l.Coord.ListNodes(ctx)
	if err != nil {
		return coordinator.Assignment{}, err
	}
	available := make([]coordinator.NodeRecord, 0, len(nodes))
	for _, node := range nodes {
		if node.Draining {
			continue
		}
		available = append(available, node)
	}
	target, ok := pickLeastLoadedNode(available)
	if !ok {
		return coordinator.Assignment{}, errors.New("no gameplay nodes available")
	}

	body, _ := json.Marshal(found)
	createCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(createCtx, http.MethodPost, strings.TrimRight(target.InternalURL, "/")+"/internal/matches", bytes.NewReader(body))
	if err != nil {
		return coordinator.Assignment{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Coordinator-Secret", l.InternalSecret)
	client := l.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return coordinator.Assignment{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return coordinator.Assignment{}, errors.New("gameplay node rejected match create")
	}

	rec := coordinator.Assignment{
		MatchID:               found.MatchID,
		Mode:                  found.Mode,
		Config:                found.Config,
		NodeID:                target.NodeID,
		NodeEpoch:             target.OwnerEpoch,
		PublicRoute:           target.PublicRoute,
		Players:               append([]string(nil), found.Players...),
		SourcePartyID:         found.SourcePartyID,
		SourcePartyInviteCode: found.SourcePartyInviteCode,
	}
	if l.Persist != nil {
		if err := l.Persist.UpsertMatchSession(persistence.MatchSessionUpsert{
			Found:       found,
			NodeID:      target.NodeID,
			NodeEpoch:   target.OwnerEpoch,
			PublicRoute: target.PublicRoute,
		}); err != nil {
			return coordinator.Assignment{}, err
		}
	}
	if err := l.Coord.SaveAssignment(ctx, rec); err != nil {
		return coordinator.Assignment{}, err
	}
	return rec, nil
}

func (l Launcher) AssignedPayload(userID string, assigned coordinator.Assignment) (contracts.MatchAssignedPayload, bool, error) {
	if assigned.MatchID == "" || assigned.PublicRoute == "" {
		return contracts.MatchAssignedPayload{}, false, nil
	}
	assignedPlayer := false
	for _, playerID := range assigned.Players {
		if playerID == userID {
			assignedPlayer = true
			break
		}
	}
	if !assignedPlayer {
		return contracts.MatchAssignedPayload{}, false, nil
	}
	node, ok, err := l.Coord.GetNodeByRoute(context.Background(), assigned.PublicRoute)
	if err != nil {
		return contracts.MatchAssignedPayload{}, false, err
	}
	if !ok || strings.TrimSpace(node.InternalURL) == "" {
		return contracts.MatchAssignedPayload{}, false, nil
	}
	ticket, err := gameticket.Issue(l.TicketSecret, userID, assigned.MatchID, assigned.PublicRoute, 2*time.Minute)
	if err != nil {
		return contracts.MatchAssignedPayload{}, false, err
	}
	return contracts.MatchAssignedPayload{
		MatchID:               assigned.MatchID,
		Mode:                  string(sessionpolicy.NormalizeMode(assigned.Mode, assigned.MatchID)),
		Config:                contracts.NormalizeMatchConfig(assigned.Config),
		Node:                  assigned.PublicRoute,
		Ticket:                ticket,
		WSPath:                "/ws/" + assigned.PublicRoute,
		SourcePartyID:         assigned.SourcePartyID,
		SourcePartyInviteCode: assigned.SourcePartyInviteCode,
	}, true, nil
}

func (l Launcher) clearEnded(ctx context.Context, assigned coordinator.Assignment) error {
	_ = l.Coord.ClearAssignment(ctx, assigned)
	if l.Persist != nil {
		_ = l.Persist.RecordRuntimeMatch(assigned.MatchID, string(contracts.MatchEnded), assigned.NodeEpoch, true)
	}
	return nil
}

func (l Launcher) matchEnded(matchID string) bool {
	if matchID == "" || l.Persist == nil {
		return false
	}
	rec, ok, err := l.Persist.GetRuntimeMatch(matchID)
	return err == nil && ok && rec.State == string(contracts.MatchEnded)
}

func pickLeastLoadedNode(nodes []coordinator.NodeRecord) (coordinator.NodeRecord, bool) {
	if len(nodes) == 0 {
		return coordinator.NodeRecord{}, false
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].ActiveMatches == nodes[j].ActiveMatches {
			return nodes[i].NodeID < nodes[j].NodeID
		}
		return nodes[i].ActiveMatches < nodes[j].ActiveMatches
	})
	return nodes[0], true
}

func newLockHolder(matchID string) string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return matchID + ":" + hex.EncodeToString(buf[:])
	}
	return matchID + ":" + strconv.FormatInt(time.Now().UnixNano(), 36)
}
