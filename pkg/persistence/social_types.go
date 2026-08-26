package persistence

import (
	"time"

	"geoduels/pkg/contracts"
)

type RelationshipState string

const (
	RelationshipNone     RelationshipState = "none"
	RelationshipIncoming RelationshipState = "incoming_request"
	RelationshipOutgoing RelationshipState = "outgoing_request"
	RelationshipFriends  RelationshipState = "friends"
	RelationshipBlocked  RelationshipState = "blocked_by_viewer"
)

type CompactPlayer struct {
	UserID         string                 `json:"userId"`
	DisplayName    string                 `json:"displayName"`
	AvatarURL      string                 `json:"avatarUrl,omitempty"`
	MMR            int                    `json:"mmr,omitempty"`
	SelectedBadge  *contracts.PlayerBadge `json:"selectedBadge,omitempty"`
	Relationship   RelationshipState      `json:"relationship"`
	RequestID      string                 `json:"requestId,omitempty"`
	PresenceStatus string                 `json:"presenceStatus,omitempty"`
	Activity       string                 `json:"activity,omitempty"`
	LastSeenAt     *time.Time             `json:"lastSeenAt,omitempty"`
	SharedMatchAt  *time.Time             `json:"sharedMatchAt,omitempty"`
}

type FriendRequest struct {
	ID        string        `json:"id"`
	Direction string        `json:"direction"`
	Player    CompactPlayer `json:"player"`
	CreatedAt time.Time     `json:"createdAt"`
	ExpiresAt time.Time     `json:"expiresAt"`
}

type PartyInvitation struct {
	ID          string        `json:"id"`
	PartyID     string        `json:"partyId"`
	InviteCode  string        `json:"inviteCode,omitempty"`
	Mode        string        `json:"mode"`
	MemberCount int           `json:"memberCount"`
	Inviter     CompactPlayer `json:"inviter"`
	CreatedAt   time.Time     `json:"createdAt"`
	ExpiresAt   time.Time     `json:"expiresAt"`
}

type FriendCode struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type SocialSettings struct {
	Discoverable        bool `json:"discoverable"`
	PresenceVisible     bool `json:"presenceVisible"`
	RequestsEnabled     bool `json:"requestsEnabled"`
	PartyInvitesEnabled bool `json:"partyInvitesEnabled"`
}

type UserEvent struct {
	Sequence   int64     `json:"sequence"`
	Type       string    `json:"type"`
	Payload    []byte    `json:"-"`
	OccurredAt time.Time `json:"occurredAt"`
}
