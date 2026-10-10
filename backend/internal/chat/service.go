package chat

import "context"

type Store interface {
	RecordChatMessage(conversationID, scopeKind, scopeID string, message ChatMessage) error
	ListChatMessages(conversationID string, limit int) ([]ChatMessage, error)
	ListChatMessagesForUser(conversationID, userID string, limit int, revealTeam bool) ([]ChatMessage, error)
	MatchOver(ctx context.Context, matchID string) (bool, error)
	ActivePartyChatTeam(partyID, userID string) (matchID, teamID string, ok bool, err error)
	ChatTeamForMatch(matchID, userID string) (teamID string, ok bool, err error)
	GetActiveChatRestriction(userID string) (ChatRestriction, bool, error)
}

type Service struct{ Store }

func NewService(store Store) *Service { return &Service{Store: store} }
