package matchstore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"geoduels/pkg/contracts"
)

var atomicMatchScript = redis.NewScript(`
local zkey = KEYS[1]
local ticket_prefix = KEYS[2]
local joined_key = KEYS[3]
local self = ARGV[1]
local min = ARGV[2]
local max = ARGV[3]
local now_ms = tonumber(ARGV[4])
local base_window = tonumber(ARGV[5])
local expand_every_ms = tonumber(ARGV[6])
local expand_step = tonumber(ARGV[7])
local max_window = tonumber(ARGV[8])
local min_mutual_wait_ms = tonumber(ARGV[10])
local self_ticket_raw = redis.call('GET', ticket_prefix .. self)
local self_joined = now_ms
if self_ticket_raw then
  local decoded = cjson.decode(self_ticket_raw)
  if decoded and decoded.joinedAtUnixMs then
    self_joined = tonumber(decoded.joinedAtUnixMs)
  end
end
local candidates = redis.call('ZRANGEBYSCORE', zkey, min, max)
local best = ''
local best_ticket_raw = ''
local best_diff = max_window + 1
for _, c in ipairs(candidates) do
  if c ~= self then
    local candidate_score = redis.call('ZSCORE', zkey, c)
    local candidate_ticket_raw = redis.call('GET', ticket_prefix .. c)
    if candidate_score and not candidate_ticket_raw then
      redis.call('ZREM', zkey, c)
    elseif candidate_score and candidate_ticket_raw then
      local candidate = cjson.decode(candidate_ticket_raw)
      local candidate_joined = now_ms
      if candidate and candidate.joinedAtUnixMs then
        candidate_joined = tonumber(candidate.joinedAtUnixMs)
      end
      local self_wait = math.max(0, now_ms - self_joined)
      local candidate_wait = math.max(0, now_ms - candidate_joined)
      local wait_ms = math.max(self_wait, candidate_wait)
      local allowed = base_window
      if expand_every_ms > 0 then
        allowed = allowed + math.floor(wait_ms / expand_every_ms) * expand_step
      end
      if allowed > max_window then
        allowed = max_window
      end
      local diff = math.abs(tonumber(candidate_score) - tonumber(ARGV[9]))
      if self_wait >= min_mutual_wait_ms and candidate_wait >= min_mutual_wait_ms and diff <= allowed and diff < best_diff then
        best = c
        best_ticket_raw = candidate_ticket_raw
        best_diff = diff
      end
    end
  end
end
if best ~= '' then
  local removed = redis.call('ZREM', zkey, self, best)
  if removed == 2 then
    redis.call('DEL', ticket_prefix .. self, ticket_prefix .. best)
    redis.call('ZREM', joined_key, self, best)
    return best_ticket_raw
  end
end
return ''
`)

var queueHeartbeatScript = redis.NewScript(`
local zkey = KEYS[1]
local ticket_key = KEYS[2]
local match_key = KEYS[3]
local joined_key = KEYS[4]
local user = ARGV[1]
local ttl_ms = tonumber(ARGV[2])

if redis.call('EXISTS', match_key) == 1 then
  return 'matched'
end

local queued = redis.call('ZSCORE', zkey, user)
if not queued then
  if redis.call('EXISTS', ticket_key) == 1 then
    redis.call('DEL', ticket_key)
  end
  redis.call('ZREM', joined_key, user)
  return 'missing'
end

if redis.call('EXISTS', ticket_key) == 0 then
  redis.call('ZREM', zkey, user)
  redis.call('ZREM', joined_key, user)
  return 'missing'
end

redis.call('PEXPIRE', ticket_key, ttl_ms)
return 'queueing'
`)

var releaseMatcherLockScript = redis.NewScript(`
if redis.call('GET', KEYS[1]) == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)

const (
	baseMatchWindowMMR = 150
	matchExpandEveryMS = int64(2 * 1000)
	matchExpandStepMMR = 75
	maxMatchWindowMMR  = 500
	mutualMatchWaitMS  = int64(1 * 1000)
	queueTicketTTL     = 30 * time.Second
	matcherLockTTL     = 2 * time.Second

	QueuePresenceMissing  = "missing"
	QueuePresenceQueueing = "queueing"
	QueuePresenceMatched  = "matched"
)

type ticket struct {
	ID                string                 `json:"id"`
	UserID            string                 `json:"userId"`
	DisplayName       string                 `json:"displayName"`
	AvatarURL         string                 `json:"avatarUrl,omitempty"`
	MMR               int                    `json:"mmr"`
	RatingRD          float64                `json:"ratingRd,omitempty"`
	SeasonID          string                 `json:"seasonId,omitempty"`
	RankedGamesPlayed int                    `json:"rankedGamesPlayed,omitempty"`
	IsGuest           bool                   `json:"isGuest,omitempty"`
	IsAdmin           bool                   `json:"isAdmin,omitempty"`
	SelectedBadge     *contracts.PlayerBadge `json:"selectedBadge,omitempty"`
	Ruleset           contracts.GameRuleset  `json:"ruleset,omitempty"`
	JoinedAtUnixMS    int64                  `json:"joinedAtUnixMs"`
}

type Store interface {
	Join(pool QueuePool, ruleset contracts.GameRuleset, req contracts.QueueJoinRequest) (contracts.QueueJoinResponse, *contracts.MatchFound, error)
	Heartbeat(pool QueuePool, rulesets []contracts.GameRuleset, userID string) (string, error)
	Leave(pool QueuePool, rulesets []contracts.GameRuleset, userID string) error
	LeaveAllRulesets(pool QueuePool, userID string) error
	Poll(pool QueuePool, rulesets []contracts.GameRuleset, userID string) (*contracts.MatchFound, error)
	IsQueued(pool QueuePool, rulesets []contracts.GameRuleset, userID string) (bool, error)
	RunMatchmaking(pool QueuePool, ruleset contracts.GameRuleset, limit int) (int, error)
}

func NewFromEnv() (Store, error) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		return nil, errors.New("REDIS_URL is required")
	}
	opt, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	c := redis.NewClient(opt)
	if err := c.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &redisStore{rdb: c}, nil
}

// Redis implementation

type redisStore struct {
	rdb *redis.Client
}

type QueuePool string

const (
	QueuePoolGuest      QueuePool = "guest"
	QueuePoolRegistered QueuePool = "registered"
)

var allQueuePools = []QueuePool{QueuePoolGuest, QueuePoolRegistered}
var allQueueRulesets = []contracts.GameRuleset{contracts.RulesetMoving, contracts.RulesetNoMove, contracts.RulesetNMPZ}

func AllQueuePools() []QueuePool {
	return append([]QueuePool(nil), allQueuePools...)
}

func PoolForGuest(isGuest bool) QueuePool {
	if isGuest {
		return QueuePoolGuest
	}
	return QueuePoolRegistered
}

func QueueMatchKeysForUsers(users []string) []string {
	keys := make([]string, 0, len(users)*len(allQueuePools)*len(allQueueRulesets))
	seen := map[string]struct{}{}
	for _, userID := range users {
		if userID == "" {
			continue
		}
		for _, pool := range allQueuePools {
			legacyKey := "queue:" + string(pool) + ":match:" + userID
			if _, ok := seen[legacyKey]; !ok {
				seen[legacyKey] = struct{}{}
				keys = append(keys, legacyKey)
			}
			for _, ruleset := range allQueueRulesets {
				key := queueMatchKey(pool, ruleset, userID)
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = struct{}{}
				keys = append(keys, key)
			}
		}
	}
	return keys
}

func normalizedRulesets(in []contracts.GameRuleset) []contracts.GameRuleset {
	if len(in) == 0 {
		return []contracts.GameRuleset{contracts.RulesetMoving}
	}
	out := make([]contracts.GameRuleset, 0, len(in))
	seen := map[contracts.GameRuleset]bool{}
	for _, raw := range in {
		ruleset := contracts.NormalizeRuleset(raw)
		if seen[ruleset] {
			continue
		}
		seen[ruleset] = true
		out = append(out, ruleset)
	}
	if len(out) == 0 {
		return []contracts.GameRuleset{contracts.RulesetMoving}
	}
	return out
}

func queuePrefix(pool QueuePool, ruleset contracts.GameRuleset) string {
	return "queue:" + string(pool) + ":" + string(contracts.NormalizeRuleset(ruleset))
}

func queueMembersKey(pool QueuePool, ruleset contracts.GameRuleset) string {
	return queuePrefix(pool, ruleset) + ":pool"
}

func queueJoinedKey(pool QueuePool, ruleset contracts.GameRuleset) string {
	return queuePrefix(pool, ruleset) + ":joined"
}

func queueTicketKey(pool QueuePool, ruleset contracts.GameRuleset, userID string) string {
	return queuePrefix(pool, ruleset) + ":ticket:" + userID
}

func queueTicketPrefix(pool QueuePool, ruleset contracts.GameRuleset) string {
	return queuePrefix(pool, ruleset) + ":ticket:"
}

func queueMatchKey(pool QueuePool, ruleset contracts.GameRuleset, userID string) string {
	return queuePrefix(pool, ruleset) + ":match:" + userID
}

func queueMatcherLockKey(pool QueuePool, ruleset contracts.GameRuleset) string {
	return queuePrefix(pool, ruleset) + ":matcher-lock"
}

func (r *redisStore) Join(pool QueuePool, ruleset contracts.GameRuleset, req contracts.QueueJoinRequest) (contracts.QueueJoinResponse, *contracts.MatchFound, error) {
	ctx := context.Background()
	ruleset = contracts.NormalizeRuleset(ruleset)
	name := req.DisplayName
	if name == "" {
		name = req.UserID
	}
	t := ticket{
		ID:                ticketID(req.UserID),
		UserID:            req.UserID,
		DisplayName:       name,
		AvatarURL:         req.AvatarURL,
		MMR:               req.MMR,
		RatingRD:          req.RatingRD,
		SeasonID:          req.SeasonID,
		RankedGamesPlayed: req.RankedGamesPlayed,
		IsGuest:           req.IsGuest,
		IsAdmin:           req.IsAdmin,
		SelectedBadge:     req.SelectedBadge,
		Ruleset:           ruleset,
		JoinedAtUnixMS:    time.Now().UnixMilli(),
	}
	tb, _ := json.Marshal(t)
	_, err := r.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, other := range allQueuePools {
			if other == pool {
				continue
			}
			for _, otherRuleset := range allQueueRulesets {
				pipe.ZRem(ctx, queueMembersKey(other, otherRuleset), req.UserID)
				pipe.Del(ctx, queueTicketKey(other, otherRuleset, req.UserID), queueMatchKey(other, otherRuleset, req.UserID))
				pipe.ZRem(ctx, queueJoinedKey(other, otherRuleset), req.UserID)
			}
		}
		pipe.Set(ctx, queueTicketKey(pool, ruleset, req.UserID), tb, queueTicketTTL)
		pipe.ZAdd(ctx, queueMembersKey(pool, ruleset), redis.Z{Score: float64(req.MMR), Member: req.UserID})
		pipe.ZAdd(ctx, queueJoinedKey(pool, ruleset), redis.Z{Score: float64(t.JoinedAtUnixMS), Member: req.UserID})
		return nil
	})
	if err != nil {
		return contracts.QueueJoinResponse{}, nil, err
	}
	return contracts.QueueJoinResponse{TicketID: t.ID, Status: "queued"}, nil, nil
}

func (r *redisStore) Heartbeat(pool QueuePool, rulesets []contracts.GameRuleset, userID string) (string, error) {
	ctx := context.Background()
	anyQueueing := false
	for _, ruleset := range normalizedRulesets(rulesets) {
		raw, err := queueHeartbeatScript.Run(
			ctx,
			r.rdb,
			[]string{queueMembersKey(pool, ruleset), queueTicketKey(pool, ruleset, userID), queueMatchKey(pool, ruleset, userID), queueJoinedKey(pool, ruleset)},
			userID,
			intStr(queueTicketTTL.Milliseconds()),
		).Result()
		if err != nil {
			return QueuePresenceMissing, err
		}
		status, _ := raw.(string)
		if status == QueuePresenceMatched {
			return QueuePresenceMatched, nil
		}
		if status == QueuePresenceQueueing {
			anyQueueing = true
		}
	}
	if anyQueueing {
		return QueuePresenceQueueing, nil
	}
	return QueuePresenceMissing, nil
}

func (r *redisStore) Leave(pool QueuePool, rulesets []contracts.GameRuleset, userID string) error {
	ctx := context.Background()
	_, err := r.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, ruleset := range normalizedRulesets(rulesets) {
			pipe.ZRem(ctx, queueMembersKey(pool, ruleset), userID)
			pipe.ZRem(ctx, queueJoinedKey(pool, ruleset), userID)
			pipe.Del(ctx, queueTicketKey(pool, ruleset, userID), queueMatchKey(pool, ruleset, userID))
		}
		return nil
	})
	return err
}

func (r *redisStore) LeaveAllRulesets(pool QueuePool, userID string) error {
	return r.Leave(pool, allQueueRulesets, userID)
}

func (r *redisStore) Poll(pool QueuePool, rulesets []contracts.GameRuleset, userID string) (*contracts.MatchFound, error) {
	ctx := context.Background()
	for _, ruleset := range normalizedRulesets(rulesets) {
		b, err := r.rdb.GetDel(ctx, queueMatchKey(pool, ruleset, userID)).Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			return nil, err
		}
		var m contracts.MatchFound
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		m.Config = contracts.NormalizeMatchConfig(m.Config)
		return &m, nil
	}
	return nil, nil
}

func (r *redisStore) RunMatchmaking(pool QueuePool, ruleset contracts.GameRuleset, limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	ruleset = contracts.NormalizeRuleset(ruleset)
	ctx := context.Background()
	owner := ticketID("matcher")
	locked, err := r.rdb.SetNX(ctx, queueMatcherLockKey(pool, ruleset), owner, matcherLockTTL).Result()
	if err != nil || !locked {
		return 0, err
	}
	defer releaseMatcherLockScript.Run(ctx, r.rdb, []string{queueMatcherLockKey(pool, ruleset)}, owner)

	users, err := r.rdb.ZRangeByScore(ctx, queueJoinedKey(pool, ruleset), &redis.ZRangeBy{
		Min: "-inf",
		Max: intStr(time.Now().UnixMilli() - mutualMatchWaitMS),
	}).Result()
	if err != nil {
		return 0, err
	}
	matched := 0
	for _, userID := range users {
		if matched >= limit {
			break
		}
		ok, err := r.tryMatch(ctx, pool, ruleset, userID)
		if err != nil {
			return matched, err
		}
		if ok {
			matched++
		}
	}
	return matched, nil
}

func (r *redisStore) tryMatch(ctx context.Context, pool QueuePool, ruleset contracts.GameRuleset, userID string) (bool, error) {
	selfRaw, err := r.rdb.Get(ctx, queueTicketKey(pool, ruleset, userID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			if _, remErr := r.rdb.ZRem(ctx, queueMembersKey(pool, ruleset), userID).Result(); remErr != nil {
				return false, remErr
			}
			if _, remErr := r.rdb.ZRem(ctx, queueJoinedKey(pool, ruleset), userID).Result(); remErr != nil {
				return false, remErr
			}
			return false, nil
		}
		return false, err
	}
	var selfTicket ticket
	if err := json.Unmarshal(selfRaw, &selfTicket); err != nil {
		return false, err
	}
	rawOpp, err := atomicMatchScript.Run(
		ctx,
		r.rdb,
		[]string{queueMembersKey(pool, ruleset), queueTicketPrefix(pool, ruleset), queueJoinedKey(pool, ruleset)},
		userID,
		intStr(int64(selfTicket.MMR-maxMatchWindowMMR)),
		intStr(int64(selfTicket.MMR+maxMatchWindowMMR)),
		intStr(time.Now().UnixMilli()),
		intStr(baseMatchWindowMMR),
		intStr(matchExpandEveryMS),
		intStr(matchExpandStepMMR),
		intStr(maxMatchWindowMMR),
		intStr(int64(selfTicket.MMR)),
		intStr(mutualMatchWaitMS),
	).Result()
	if err != nil {
		return false, err
	}
	oppTicket := ticket{}
	if rawOpp != nil {
		oppRaw, _ := rawOpp.(string)
		if oppRaw != "" {
			if err := json.Unmarshal([]byte(oppRaw), &oppTicket); err != nil {
				return false, err
			}
		}
	}
	if oppTicket.UserID == "" {
		return false, nil
	}
	match := matchFromTickets(oppTicket, selfTicket)
	mb, _ := json.Marshal(match)
	_, err = r.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, queueMatchKey(pool, ruleset, userID), mb, 2*time.Minute)
		pipe.Set(ctx, queueMatchKey(pool, ruleset, oppTicket.UserID), mb, 2*time.Minute)
		for _, queuedRuleset := range allQueueRulesets {
			for _, matchedUserID := range []string{userID, oppTicket.UserID} {
				pipe.ZRem(ctx, queueMembersKey(pool, queuedRuleset), matchedUserID)
				pipe.ZRem(ctx, queueJoinedKey(pool, queuedRuleset), matchedUserID)
				pipe.Del(ctx, queueTicketKey(pool, queuedRuleset, matchedUserID))
				if queuedRuleset != ruleset {
					pipe.Del(ctx, queueMatchKey(pool, queuedRuleset, matchedUserID))
				}
			}
		}
		return nil
	})
	return err == nil, err
}

func (r *redisStore) IsQueued(pool QueuePool, rulesets []contracts.GameRuleset, userID string) (bool, error) {
	ctx := context.Background()
	queued := false
	for _, ruleset := range normalizedRulesets(rulesets) {
		ok, err := r.rdb.Exists(ctx, queueTicketKey(pool, ruleset, userID)).Result()
		if err != nil {
			return false, err
		}
		if ok == 0 {
			if _, remErr := r.rdb.ZRem(ctx, queueMembersKey(pool, ruleset), userID).Result(); remErr != nil {
				return false, remErr
			}
			if _, remErr := r.rdb.ZRem(ctx, queueJoinedKey(pool, ruleset), userID).Result(); remErr != nil {
				return false, remErr
			}
			continue
		}
		if _, err := r.rdb.ZScore(ctx, queueMembersKey(pool, ruleset), userID).Result(); err != nil {
			if errors.Is(err, redis.Nil) {
				if delErr := r.rdb.Del(ctx, queueTicketKey(pool, ruleset, userID)).Err(); delErr != nil {
					return false, delErr
				}
				if _, remErr := r.rdb.ZRem(ctx, queueJoinedKey(pool, ruleset), userID).Result(); remErr != nil {
					return false, remErr
				}
				continue
			}
			return false, err
		}
		queued = true
	}
	return queued, nil
}

func ticketID(userID string) string {
	return userID + "-" + intStr(time.Now().UnixMilli())
}

func matchFromTickets(opponent, self ticket) contracts.MatchFound {
	ruleset := contracts.NormalizeRuleset(self.Ruleset)
	seasonID := self.SeasonID
	if seasonID == "" {
		seasonID = opponent.SeasonID
	}
	return contracts.MatchFound{
		MatchID:  "m-" + intStr(time.Now().UnixMilli()),
		Mode:     contracts.ModeDuel,
		SeasonID: seasonID,
		Config: contracts.NormalizeMatchConfig(contracts.MatchConfig{
			Ruleset: ruleset,
			MapKey:  contracts.MapKeyForRuleset(ruleset),
		}),
		Players: []string{opponent.UserID, self.UserID},
		Profiles: map[string]contracts.PlayerProfile{
			opponent.UserID: profileFromTicket(opponent),
			self.UserID:     profileFromTicket(self),
		},
		MapScope: "world",
	}
}

func profileFromTicket(t ticket) contracts.PlayerProfile {
	return contracts.PlayerProfile{
		UserID:            t.UserID,
		DisplayName:       t.DisplayName,
		MMR:               t.MMR,
		RatingRD:          t.RatingRD,
		RankedGamesPlayed: t.RankedGamesPlayed,
		AvatarURL:         t.AvatarURL,
		IsGuest:           t.IsGuest,
		IsAdmin:           t.IsAdmin,
		SelectedBadge:     t.SelectedBadge,
	}
}

func intStr(v int64) string {
	if v == 0 {
		return "0"
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	o := ""
	for v > 0 {
		o = string(rune('0'+(v%10))) + o
		v /= 10
	}
	return sign + o
}
