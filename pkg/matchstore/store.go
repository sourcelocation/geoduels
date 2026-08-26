package matchstore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"geoduels/pkg/contracts"
	"geoduels/pkg/entityid"
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
	ID                string                          `json:"id"`
	UserID            string                          `json:"userId"`
	DisplayName       string                          `json:"displayName"`
	AvatarURL         string                          `json:"avatarUrl,omitempty"`
	MMR               int                             `json:"mmr"`
	RatingRD          float64                         `json:"ratingRd,omitempty"`
	SeasonID          string                          `json:"seasonId,omitempty"`
	RankedGamesPlayed int                             `json:"rankedGamesPlayed,omitempty"`
	IsGuest           bool                            `json:"isGuest,omitempty"`
	IsAdmin           bool                            `json:"isAdmin,omitempty"`
	SelectedBadge     *contracts.PlayerBadge          `json:"selectedBadge,omitempty"`
	Ruleset           contracts.GameRuleset           `json:"ruleset,omitempty"`
	StreetNames       contracts.StreetNamesVisibility `json:"streetNames,omitempty"`
	Queue             QueueVariant                    `json:"queue,omitempty"`
	JoinedAtUnixMS    int64                           `json:"joinedAtUnixMs"`
}

type Store interface {
	Join(pool QueuePool, queue QueueVariant, req contracts.QueueJoinRequest) (contracts.QueueJoinResponse, *contracts.MatchFound, error)
	Heartbeat(pool QueuePool, queues []QueueVariant, userID string) (string, error)
	Leave(pool QueuePool, queues []QueueVariant, userID string) error
	LeaveAllRulesets(pool QueuePool, userID string) error
	Poll(pool QueuePool, queues []QueueVariant, userID string) (*contracts.MatchFound, error)
	IsQueued(pool QueuePool, queues []QueueVariant, userID string) (bool, error)
	RunMatchmaking(pool QueuePool, queue QueueVariant, limit int) (int, error)
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
	QueuePoolRegistered QueuePool = "registered"
)

type QueueVariant = string

const (
	QueueMoving       QueueVariant = "moving"
	QueueNoMove       QueueVariant = "no_move"
	QueueNMPZ         QueueVariant = "nmpz"
	QueueMovingHidden QueueVariant = "moving_hidden"
	QueueNoMoveHidden QueueVariant = "no_move_hidden"
	QueueNMPZHidden   QueueVariant = "nmpz_hidden"
)

var allQueuePools = []QueuePool{QueuePoolRegistered}
var AllQueueVariants = []QueueVariant{
	QueueMoving,
	QueueNoMoveHidden,
}

func NormalizeQueueVariant(value QueueVariant) QueueVariant {
	switch value {
	case QueueNoMove, QueueNMPZ, QueueMovingHidden, QueueNoMoveHidden, QueueNMPZHidden:
		return value
	default:
		return QueueMoving
	}
}

func IsRankedQueueVariant(value QueueVariant) bool {
	return value == QueueMoving || value == QueueNoMoveHidden
}

func QueueVariantConfig(queue QueueVariant) contracts.MatchConfig {
	queue = NormalizeQueueVariant(queue)
	cfg := contracts.MatchConfig{
		Ruleset:        contracts.RulesetMoving,
		StreetNames:    contracts.StreetNamesShown,
		MultiplierMode: contracts.MultiplierIndividual,
	}
	switch queue {
	case QueueNoMove, QueueNoMoveHidden:
		cfg.Ruleset = contracts.RulesetNoMove
	case QueueNMPZ, QueueNMPZHidden:
		cfg.Ruleset = contracts.RulesetNMPZ
	}
	switch queue {
	case QueueMovingHidden, QueueNoMoveHidden, QueueNMPZHidden:
		cfg.StreetNames = contracts.StreetNamesHidden
	}
	return contracts.NormalizeMatchConfig(cfg)
}

func QueueMatchKeysForUsers(users []string) []string {
	keys := make([]string, 0, len(users)*len(allQueuePools)*len(AllQueueVariants))
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
			for _, queue := range AllQueueVariants {
				key := queueMatchKey(pool, queue, userID)
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

func normalizedQueues(in []QueueVariant) []QueueVariant {
	if len(in) == 0 {
		return []QueueVariant{QueueMoving}
	}
	out := make([]QueueVariant, 0, len(in))
	seen := map[QueueVariant]bool{}
	for _, raw := range in {
		queue := NormalizeQueueVariant(raw)
		if seen[queue] {
			continue
		}
		seen[queue] = true
		out = append(out, queue)
	}
	if len(out) == 0 {
		return []QueueVariant{QueueMoving}
	}
	return out
}

func queuePrefix(pool QueuePool, queue QueueVariant) string {
	return "queue:" + string(pool) + ":" + string(NormalizeQueueVariant(queue))
}

func queueMembersKey(pool QueuePool, queue QueueVariant) string {
	return queuePrefix(pool, queue) + ":pool"
}

func queueJoinedKey(pool QueuePool, queue QueueVariant) string {
	return queuePrefix(pool, queue) + ":joined"
}

func queueTicketKey(pool QueuePool, queue QueueVariant, userID string) string {
	return queuePrefix(pool, queue) + ":ticket:" + userID
}

func queueTicketPrefix(pool QueuePool, queue QueueVariant) string {
	return queuePrefix(pool, queue) + ":ticket:"
}

func queueMatchKey(pool QueuePool, queue QueueVariant, userID string) string {
	return queuePrefix(pool, queue) + ":match:" + userID
}

func queueMatcherLockKey(pool QueuePool, queue QueueVariant) string {
	return queuePrefix(pool, queue) + ":matcher-lock"
}

func (r *redisStore) Join(pool QueuePool, queue QueueVariant, req contracts.QueueJoinRequest) (contracts.QueueJoinResponse, *contracts.MatchFound, error) {
	ctx := context.Background()
	queue = NormalizeQueueVariant(queue)
	config := QueueVariantConfig(queue)
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
		Ruleset:           config.Ruleset,
		StreetNames:       config.StreetNames,
		Queue:             queue,
		JoinedAtUnixMS:    time.Now().UnixMilli(),
	}
	tb, _ := json.Marshal(t)
	_, err := r.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, other := range allQueuePools {
			if other == pool {
				continue
			}
			for _, otherQueue := range AllQueueVariants {
				pipe.ZRem(ctx, queueMembersKey(other, otherQueue), req.UserID)
				pipe.Del(ctx, queueTicketKey(other, otherQueue, req.UserID), queueMatchKey(other, otherQueue, req.UserID))
				pipe.ZRem(ctx, queueJoinedKey(other, otherQueue), req.UserID)
			}
		}
		pipe.Set(ctx, queueTicketKey(pool, queue, req.UserID), tb, queueTicketTTL)
		pipe.ZAdd(ctx, queueMembersKey(pool, queue), redis.Z{Score: float64(req.MMR), Member: req.UserID})
		pipe.ZAdd(ctx, queueJoinedKey(pool, queue), redis.Z{Score: float64(t.JoinedAtUnixMS), Member: req.UserID})
		return nil
	})
	if err != nil {
		return contracts.QueueJoinResponse{}, nil, err
	}
	return contracts.QueueJoinResponse{TicketID: t.ID, Status: "queued"}, nil, nil
}

func (r *redisStore) Heartbeat(pool QueuePool, queues []QueueVariant, userID string) (string, error) {
	ctx := context.Background()
	anyQueueing := false
	for _, queue := range normalizedQueues(queues) {
		raw, err := queueHeartbeatScript.Run(
			ctx,
			r.rdb,
			[]string{queueMembersKey(pool, queue), queueTicketKey(pool, queue, userID), queueMatchKey(pool, queue, userID), queueJoinedKey(pool, queue)},
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

func (r *redisStore) Leave(pool QueuePool, queues []QueueVariant, userID string) error {
	ctx := context.Background()
	_, err := r.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, queue := range normalizedQueues(queues) {
			pipe.ZRem(ctx, queueMembersKey(pool, queue), userID)
			pipe.ZRem(ctx, queueJoinedKey(pool, queue), userID)
			pipe.Del(ctx, queueTicketKey(pool, queue, userID), queueMatchKey(pool, queue, userID))
		}
		return nil
	})
	return err
}

func (r *redisStore) LeaveAllRulesets(pool QueuePool, userID string) error {
	return r.Leave(pool, AllQueueVariants, userID)
}

func (r *redisStore) Poll(pool QueuePool, queues []QueueVariant, userID string) (*contracts.MatchFound, error) {
	ctx := context.Background()
	for _, queue := range normalizedQueues(queues) {
		b, err := r.rdb.GetDel(ctx, queueMatchKey(pool, queue, userID)).Bytes()
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

func (r *redisStore) RunMatchmaking(pool QueuePool, queue QueueVariant, limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	queue = NormalizeQueueVariant(queue)
	ctx := context.Background()
	owner := ticketID("matcher")
	locked, err := r.rdb.SetNX(ctx, queueMatcherLockKey(pool, queue), owner, matcherLockTTL).Result()
	if err != nil || !locked {
		return 0, err
	}
	defer releaseMatcherLockScript.Run(ctx, r.rdb, []string{queueMatcherLockKey(pool, queue)}, owner)

	users, err := r.rdb.ZRangeByScore(ctx, queueJoinedKey(pool, queue), &redis.ZRangeBy{
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
		ok, err := r.tryMatch(ctx, pool, queue, userID)
		if err != nil {
			return matched, err
		}
		if ok {
			matched++
		}
	}
	return matched, nil
}

func (r *redisStore) tryMatch(ctx context.Context, pool QueuePool, queue QueueVariant, userID string) (bool, error) {
	selfRaw, err := r.rdb.Get(ctx, queueTicketKey(pool, queue, userID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			if _, remErr := r.rdb.ZRem(ctx, queueMembersKey(pool, queue), userID).Result(); remErr != nil {
				return false, remErr
			}
			if _, remErr := r.rdb.ZRem(ctx, queueJoinedKey(pool, queue), userID).Result(); remErr != nil {
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
		[]string{queueMembersKey(pool, queue), queueTicketPrefix(pool, queue), queueJoinedKey(pool, queue)},
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
		pipe.Set(ctx, queueMatchKey(pool, queue, userID), mb, 2*time.Minute)
		pipe.Set(ctx, queueMatchKey(pool, queue, oppTicket.UserID), mb, 2*time.Minute)
		for _, queuedQueue := range AllQueueVariants {
			for _, matchedUserID := range []string{userID, oppTicket.UserID} {
				pipe.ZRem(ctx, queueMembersKey(pool, queuedQueue), matchedUserID)
				pipe.ZRem(ctx, queueJoinedKey(pool, queuedQueue), matchedUserID)
				pipe.Del(ctx, queueTicketKey(pool, queuedQueue, matchedUserID))
				if queuedQueue != queue {
					pipe.Del(ctx, queueMatchKey(pool, queuedQueue, matchedUserID))
				}
			}
		}
		return nil
	})
	return err == nil, err
}

func (r *redisStore) IsQueued(pool QueuePool, queues []QueueVariant, userID string) (bool, error) {
	ctx := context.Background()
	queued := false
	for _, queue := range normalizedQueues(queues) {
		ok, err := r.rdb.Exists(ctx, queueTicketKey(pool, queue, userID)).Result()
		if err != nil {
			return false, err
		}
		if ok == 0 {
			if _, remErr := r.rdb.ZRem(ctx, queueMembersKey(pool, queue), userID).Result(); remErr != nil {
				return false, remErr
			}
			if _, remErr := r.rdb.ZRem(ctx, queueJoinedKey(pool, queue), userID).Result(); remErr != nil {
				return false, remErr
			}
			continue
		}
		if _, err := r.rdb.ZScore(ctx, queueMembersKey(pool, queue), userID).Result(); err != nil {
			if errors.Is(err, redis.Nil) {
				if delErr := r.rdb.Del(ctx, queueTicketKey(pool, queue, userID)).Err(); delErr != nil {
					return false, delErr
				}
				if _, remErr := r.rdb.ZRem(ctx, queueJoinedKey(pool, queue), userID).Result(); remErr != nil {
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
		MatchID:  entityid.New(),
		Mode:     contracts.ModeDuel,
		SeasonID: seasonID,
		Config: contracts.NormalizeMatchConfig(contracts.MatchConfig{
			Ruleset:        ruleset,
			StreetNames:    self.StreetNames,
			MultiplierMode: contracts.MultiplierIndividual,
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
