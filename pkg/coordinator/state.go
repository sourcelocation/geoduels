package coordinator

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"geoduels/pkg/contracts"
)

var unlockMatchScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)

type NodeRecord struct {
	NodeID        string `json:"nodeId"`
	OwnerEpoch    int64  `json:"ownerEpoch,omitempty"`
	PublicRoute   string `json:"publicRoute"`
	InternalURL   string `json:"internalUrl"`
	ActiveMatches int    `json:"activeMatches"`
	Draining      bool   `json:"draining,omitempty"`
	UpdatedAt     int64  `json:"updatedAt"`
}

type Assignment struct {
	MatchID               string                       `json:"matchId"`
	Mode                  contracts.MatchMode          `json:"mode,omitempty"`
	Config                contracts.MatchConfig        `json:"config,omitempty"`
	NodeID                string                       `json:"nodeId"`
	NodeEpoch             int64                        `json:"nodeEpoch,omitempty"`
	PublicRoute           string                       `json:"publicRoute"`
	Players               []string                     `json:"players"`
	UpdatedAt             int64                        `json:"updatedAt"`
	RecoverableUntil      int64                        `json:"recoverableUntil,omitempty"`
	SourcePartyID         string                       `json:"sourcePartyId,omitempty"`
	SourcePartyInviteCode string                       `json:"sourcePartyInviteCode,omitempty"`
	ReturnTarget          *contracts.MatchReturnTarget `json:"returnTarget,omitempty"`
}

type Store struct {
	rdb           *redis.Client
	nodeTTL       time.Duration
	assignmentTTL time.Duration
	singleTTL     time.Duration
	lockTTL       time.Duration
}

const presenceTTL = 90 * time.Second

func NewStore(rdb *redis.Client, nodeTTL, assignmentTTL, singleTTL, lockTTL time.Duration) *Store {
	if nodeTTL <= 0 {
		nodeTTL = 10 * time.Second
	}
	if assignmentTTL <= 0 {
		assignmentTTL = 2 * time.Hour
	}
	if singleTTL <= 0 {
		singleTTL = 24 * time.Hour
	}
	if lockTTL <= 0 {
		lockTTL = 5 * time.Second
	}
	return &Store{rdb: rdb, nodeTTL: nodeTTL, assignmentTTL: assignmentTTL, singleTTL: singleTTL, lockTTL: lockTTL}
}

func (s *Store) RegisterNode(ctx context.Context, rec NodeRecord) error {
	if rec.NodeID == "" || rec.InternalURL == "" {
		return errors.New("nodeID and internalURL are required")
	}
	if rec.PublicRoute == "" {
		rec.PublicRoute = rec.NodeID
	}
	rec.UpdatedAt = time.Now().UnixMilli()
	existing, ok, err := s.GetNodeByID(ctx, rec.NodeID)
	if err != nil {
		return err
	}
	_, err = s.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		if ok && existing.PublicRoute != "" && existing.PublicRoute != rec.PublicRoute {
			pipe.Del(ctx, nodeRouteKey(existing.PublicRoute))
		}
		pipe.SAdd(ctx, nodeIndexKey(), rec.NodeID)
		pipe.Set(ctx, nodeKey(rec.NodeID), mustJSON(rec), s.nodeTTL)
		pipe.Set(ctx, nodeRouteKey(rec.PublicRoute), mustJSON(rec), s.nodeTTL)
		return nil
	})
	return err
}

func (s *Store) RemoveNode(ctx context.Context, nodeID string) error {
	if nodeID == "" {
		return nil
	}
	rec, _, _ := s.GetNodeByID(ctx, nodeID)
	_, err := s.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, nodeKey(nodeID))
		pipe.SRem(ctx, nodeIndexKey(), nodeID)
		if rec.PublicRoute != "" {
			pipe.Del(ctx, nodeRouteKey(rec.PublicRoute))
		}
		return nil
	})
	return err
}

func (s *Store) ListNodes(ctx context.Context) ([]NodeRecord, error) {
	nodeIDs, err := s.rdb.SMembers(ctx, nodeIndexKey()).Result()
	if err != nil {
		return nil, err
	}
	out := make([]NodeRecord, 0, len(nodeIDs))
	stale := make([]any, 0)
	for _, nodeID := range nodeIDs {
		rec, ok, err := s.GetNodeByID(ctx, nodeID)
		if err != nil {
			return nil, err
		}
		if !ok {
			stale = append(stale, nodeID)
			continue
		}
		out = append(out, rec)
	}
	if len(stale) > 0 {
		_ = s.rdb.SRem(context.Background(), nodeIndexKey(), stale...).Err()
	}
	return out, nil
}

func (s *Store) GetNodeByRoute(ctx context.Context, route string) (NodeRecord, bool, error) {
	if strings.TrimSpace(route) == "" {
		return NodeRecord{}, false, nil
	}
	if rec, ok, err := s.getNode(ctx, nodeRouteKey(route)); ok || err != nil {
		return rec, ok, err
	}
	return s.getNode(ctx, nodeKey(route))
}

func (s *Store) GetNodeByID(ctx context.Context, nodeID string) (NodeRecord, bool, error) {
	return s.getNode(ctx, nodeKey(nodeID))
}

func (s *Store) TryLockMatch(ctx context.Context, matchID, holder string) (bool, error) {
	if matchID == "" {
		return false, errors.New("matchID required")
	}
	if strings.TrimSpace(holder) == "" {
		return false, errors.New("holder required")
	}
	return s.rdb.SetNX(ctx, lockKey(matchID), holder, s.lockTTL).Result()
}

func (s *Store) UnlockMatch(ctx context.Context, matchID, holder string) error {
	if matchID == "" || strings.TrimSpace(holder) == "" {
		return nil
	}
	return unlockMatchScript.Run(ctx, s.rdb, []string{lockKey(matchID)}, holder).Err()
}

func (s *Store) SaveAssignment(ctx context.Context, rec Assignment) error {
	if rec.MatchID == "" || rec.NodeID == "" || len(rec.Players) == 0 {
		return errors.New("matchID, nodeID, and players are required")
	}
	if rec.PublicRoute == "" {
		rec.PublicRoute = rec.NodeID
	}
	now := time.Now()
	rec.UpdatedAt = now.UnixMilli()
	ttl := s.assignmentTTL
	if rec.Mode == contracts.ModeSingleplayer {
		ttl = s.singleTTL
		rec.RecoverableUntil = now.Add(s.singleTTL).UnixMilli()
	}
	b, _ := json.Marshal(rec)
	_, err := s.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, matchKey(rec.MatchID), b, ttl)
		for _, userID := range rec.Players {
			if strings.TrimSpace(userID) == "" {
				continue
			}
			pipe.Set(ctx, userKey(userID), b, ttl)
		}
		return nil
	})
	return err
}

func (s *Store) GetAssignmentByUser(ctx context.Context, userID string) (Assignment, bool, error) {
	return s.getAssignment(ctx, userKey(userID))
}

func (s *Store) GetAssignmentByMatch(ctx context.Context, matchID string) (Assignment, bool, error) {
	return s.getAssignment(ctx, matchKey(matchID))
}

func (s *Store) ClearAssignment(ctx context.Context, rec Assignment) error {
	if rec.MatchID == "" {
		return nil
	}
	_, err := s.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, matchKey(rec.MatchID))
		for _, userID := range rec.Players {
			if strings.TrimSpace(userID) == "" {
				continue
			}
			pipe.Del(ctx, userKey(userID))
		}
		return nil
	})
	return err
}

func (s *Store) TouchPresence(ctx context.Context, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return nil
	}
	now := time.Now().UnixMilli()
	_, err := s.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.ZAdd(ctx, presenceKey(), redis.Z{Score: float64(now), Member: userID})
		pipe.ZRemRangeByScore(ctx, presenceKey(), "-inf", strconv.FormatInt(now-presenceTTL.Milliseconds(), 10))
		return nil
	})
	return err
}

func (s *Store) CountPresentUsers(ctx context.Context) (int, error) {
	now := time.Now().UnixMilli()
	if err := s.rdb.ZRemRangeByScore(ctx, presenceKey(), "-inf", strconv.FormatInt(now-presenceTTL.Milliseconds(), 10)).Err(); err != nil {
		return 0, err
	}
	total, err := s.rdb.ZCard(ctx, presenceKey()).Result()
	return int(total), err
}

func (s *Store) getAssignment(ctx context.Context, key string) (Assignment, bool, error) {
	if key == "" {
		return Assignment{}, false, nil
	}
	b, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return Assignment{}, false, nil
		}
		return Assignment{}, false, err
	}
	var rec Assignment
	if err := json.Unmarshal(b, &rec); err != nil {
		return Assignment{}, false, err
	}
	if rec.MatchID == "" {
		return Assignment{}, false, nil
	}
	return rec, true, nil
}

func (s *Store) setJSON(ctx context.Context, key string, v any, ttl time.Duration) error {
	return s.rdb.Set(ctx, key, mustJSON(v), ttl).Err()
}

func (s *Store) getNode(ctx context.Context, key string) (NodeRecord, bool, error) {
	if strings.TrimSpace(key) == "" {
		return NodeRecord{}, false, nil
	}
	b, err := s.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return NodeRecord{}, false, nil
		}
		return NodeRecord{}, false, err
	}
	var rec NodeRecord
	if err := json.Unmarshal(b, &rec); err != nil {
		return NodeRecord{}, false, err
	}
	if rec.NodeID == "" {
		return NodeRecord{}, false, nil
	}
	return rec, true, nil
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func nodeKey(nodeID string) string {
	return "rt:node:" + nodeID
}

func nodeRouteKey(route string) string {
	return "rt:node-route:" + route
}

func nodeIndexKey() string {
	return "rt:nodes"
}

func matchKey(matchID string) string {
	return "rt:active:match:" + matchID
}

func userKey(userID string) string {
	return "rt:active:user:" + userID
}

func lockKey(matchID string) string {
	return "rt:active:lock:" + matchID
}

func presenceKey() string {
	return "rt:presence:online"
}
