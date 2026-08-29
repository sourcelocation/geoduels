package persistence

import "geoduels/pkg/social"

// Social DTOs are owned by the social domain. These aliases preserve the
// persistence package's compatibility surface for existing adapters/tests.
type (
	RelationshipState  = social.RelationshipState
	CompactPlayer      = social.CompactPlayer
	CompactPartyInvite = social.CompactPartyInvite
	FriendRequest      = social.FriendRequest
	PartyInvitation    = social.PartyInvitation
	FriendCode         = social.FriendCode
	SocialSettings     = social.SocialSettings
)

const (
	RelationshipNone     = social.RelationshipNone
	RelationshipIncoming = social.RelationshipIncoming
	RelationshipOutgoing = social.RelationshipOutgoing
	RelationshipFriends  = social.RelationshipFriends
	RelationshipBlocked  = social.RelationshipBlocked
)
