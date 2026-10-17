package chat

import "time"

// Message is a chat message in a conversation.
type Message struct {
	ID                string
	ConversationID    string
	MatchID           string
	SenderUserID      string
	SenderDisplayName string
	SenderTeamID      string
	Kind              Kind
	Audience          Audience
	TeamID            string
	Body              string
	Emote             Emote
	CreatedAt         time.Time
}

type Kind string

const (
	KindText  Kind = "text"
	KindEmote Kind = "emote"
)

// Audience is who sees a message: everyone in the conversation, or only the sender's team.
type Audience string

const (
	AudienceAll  Audience = "all"
	AudienceTeam Audience = "team"
)

type Emote string

const (
	EmoteSkull      Emote = "skull"
	EmoteSob        Emote = "sob"
	EmoteThinking   Emote = "thinking"
	EmoteSunglasses Emote = "sunglasses"
	EmoteWave       Emote = "wave"
)

type ChatRestriction struct {
	ActionType string
	ReasonCode string
	ReasonNote string
	EndsAt     time.Time
}
