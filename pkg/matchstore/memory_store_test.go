package matchstore

import (
	"math"
	"sort"
	"sync"
	"time"

	"geoduels/pkg/contracts"
)

type memoryStore struct {
	mu      sync.Mutex
	queues  map[QueuePool]map[QueueVariant][]ticket
	matches map[QueuePool]map[QueueVariant]map[string]contracts.MatchFound
}

func newMemory() Store {
	queues := map[QueueVariant][]ticket{}
	matches := map[QueueVariant]map[string]contracts.MatchFound{}
	for _, queue := range AllQueueVariants {
		queues[queue] = []ticket{}
		matches[queue] = map[string]contracts.MatchFound{}
	}
	return &memoryStore{
		queues: map[QueuePool]map[QueueVariant][]ticket{
			QueuePoolRegistered: queues,
		},
		matches: map[QueuePool]map[QueueVariant]map[string]contracts.MatchFound{
			QueuePoolRegistered: matches,
		},
	}
}

func (m *memoryStore) Join(pool QueuePool, variant QueueVariant, req contracts.QueueJoinRequest) (contracts.QueueJoinResponse, *contracts.MatchFound, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	variant = NormalizeQueueVariant(variant)
	config := QueueVariantConfig(variant)
	queue := m.queues[pool][variant]
	for _, t := range queue {
		if t.UserID == req.UserID {
			return contracts.QueueJoinResponse{TicketID: t.ID, Status: "queued"}, nil, nil
		}
	}
	for other := range m.queues {
		if other == pool {
			continue
		}
		m.leaveLocked(other, AllQueueVariants, req.UserID)
	}
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
		Ruleset:           config.Ruleset,
		StreetNames:       config.StreetNames,
		Queue:             variant,
		JoinedAtUnixMS:    time.Now().UnixMilli(),
	}
	queue = append(queue, t)
	sort.Slice(queue, func(i, j int) bool { return queue[i].JoinedAtUnixMS < queue[j].JoinedAtUnixMS })
	m.queues[pool][variant] = queue
	return contracts.QueueJoinResponse{TicketID: t.ID, Status: "queued"}, nil, nil
}

func (m *memoryStore) Leave(pool QueuePool, queues []QueueVariant, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.leaveLocked(pool, normalizedQueues(queues), userID)
	return nil
}

func (m *memoryStore) LeaveAllRulesets(pool QueuePool, userID string) error {
	return m.Leave(pool, AllQueueVariants, userID)
}

func (m *memoryStore) leaveLocked(pool QueuePool, variants []QueueVariant, userID string) {
	for _, variant := range variants {
		queue := m.queues[pool][variant]
		out := queue[:0]
		for _, t := range queue {
			if t.UserID != userID {
				out = append(out, t)
			}
		}
		m.queues[pool][variant] = out
		delete(m.matches[pool][variant], userID)
	}
}

func (m *memoryStore) Heartbeat(pool QueuePool, queues []QueueVariant, userID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, queue := range normalizedQueues(queues) {
		if _, ok := m.matches[pool][queue][userID]; ok {
			return QueuePresenceMatched, nil
		}
		for _, t := range m.queues[pool][queue] {
			if t.UserID == userID {
				return QueuePresenceQueueing, nil
			}
		}
	}
	return QueuePresenceMissing, nil
}

func (m *memoryStore) Poll(pool QueuePool, queues []QueueVariant, userID string) (*contracts.MatchFound, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, queue := range normalizedQueues(queues) {
		mf, ok := m.matches[pool][queue][userID]
		if !ok {
			continue
		}
		delete(m.matches[pool][queue], userID)
		return &mf, nil
	}
	return nil, nil
}

func (m *memoryStore) IsQueued(pool QueuePool, queues []QueueVariant, userID string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, queue := range normalizedQueues(queues) {
		for _, t := range m.queues[pool][queue] {
			if t.UserID == userID {
				return true, nil
			}
		}
	}
	return false, nil
}

func (m *memoryStore) RunMatchmaking(pool QueuePool, variant QueueVariant, limit int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	variant = NormalizeQueueVariant(variant)
	if limit <= 0 {
		limit = 50
	}
	matched := 0
	for {
		if matched >= limit {
			break
		}
		matchedThisPass := false
		queue := append([]ticket(nil), m.queues[pool][variant]...)
		nowMS := time.Now().UnixMilli()
		for _, t := range queue {
			if matched >= limit {
				break
			}
			if nowMS-t.JoinedAtUnixMS < mutualMatchWaitMS {
				continue
			}
			if m.tryMatchLocked(pool, variant, t.UserID, nowMS) {
				matched++
				matchedThisPass = true
			}
		}
		if !matchedThisPass {
			break
		}
	}
	return matched, nil
}

func (m *memoryStore) tryMatchLocked(pool QueuePool, variant QueueVariant, userID string, nowMS int64) bool {
	queue := m.queues[pool][variant]
	selfIdx := -1
	for i, t := range queue {
		if t.UserID == userID {
			selfIdx = i
			break
		}
	}
	if selfIdx < 0 {
		return false
	}
	self := queue[selfIdx]
	remaining := append([]ticket{}, queue[:selfIdx]...)
	remaining = append(remaining, queue[selfIdx+1:]...)
	bestIdx := bestMatchIndex(remaining, self, nowMS)
	if bestIdx < 0 {
		return false
	}
	op := remaining[bestIdx]
	remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	m.queues[pool][variant] = remaining
	match := matchFromTickets(op, self)
	m.matches[pool][variant][op.UserID] = match
	m.matches[pool][variant][self.UserID] = match
	return true
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func allowedMatchDiff(nowMS, joinedA, joinedB int64) int {
	waitMS := nowMS - joinedA
	if other := nowMS - joinedB; other > waitMS {
		waitMS = other
	}
	if waitMS < 0 {
		waitMS = 0
	}
	allowed := baseMatchWindowMMR
	if matchExpandEveryMS > 0 {
		allowed += int(waitMS/matchExpandEveryMS) * matchExpandStepMMR
	}
	if allowed > maxMatchWindowMMR {
		return maxMatchWindowMMR
	}
	return allowed
}

func bestMatchIndex(queue []ticket, self ticket, nowMS int64) int {
	bestIdx := -1
	bestDiff := math.MaxInt
	for i, q := range queue {
		if q.UserID == self.UserID {
			continue
		}
		d := abs(q.MMR - self.MMR)
		if waitedLongEnough(nowMS, self.JoinedAtUnixMS, q.JoinedAtUnixMS) && d <= allowedMatchDiff(nowMS, self.JoinedAtUnixMS, q.JoinedAtUnixMS) && d < bestDiff {
			bestDiff = d
			bestIdx = i
		}
	}
	return bestIdx
}

func waitedLongEnough(nowMS, joinedA, joinedB int64) bool {
	return nowMS-joinedA >= mutualMatchWaitMS && nowMS-joinedB >= mutualMatchWaitMS
}
