package parties

import (
	"errors"
	"time"

	"geoduels/internal/badges"
	"geoduels/pkg/gameplay"
	"geoduels/pkg/matchkind"
)

// ErrNotFound identifies a missing party or membership.
var ErrNotFound = errors.New("no rows in result set")

// Party is a party with its members, as one read sees it.
type Party struct {
	ID               string
	InviteCode       string
	OwnerUserID      string
	State            State
	Mode             matchkind.Mode
	MapScope         string
	MapName          string
	MapLocationCount int
	Config           gameplay.MatchConfig
	ActiveMatchID    string
	LastMatchID      string
	CreatedAt        time.Time
	ExpiresAt        time.Time
	Members          []Member
}

// State is where a party is in its life. A party in a match is stored open; reads derive in_match
// from its last match.
type State string

const (
	StateOpen    State = "open"
	StateInMatch State = "in_match"
	StateClosed  State = "closed"
	StateExpired State = "expired"
)

type Member struct {
	UserID         string
	DisplayName    string
	AvatarURL      string
	IsGuest        bool
	IsAdmin        bool
	SelectedBadge  *badges.Badge
	TeamID         string
	Role           string
	Ready          bool
	Connected      bool
	PresenceStatus Presence
	InActiveMatch  bool
	JoinedAt       time.Time
}

// Presence is how recently a member's party socket was seen.
type Presence string

const (
	PresenceOnline  Presence = "online"
	PresenceAway    Presence = "away"
	PresenceOffline Presence = "offline"
)

// CurrentParty is the open party a player is in.
type CurrentParty struct {
	ID         string
	InviteCode string
}
