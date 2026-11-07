package main

import (
	"time"

	"geoduels/internal/inbox"
	"geoduels/internal/social"
	"geoduels/pkg/maintenance"
)

// What people are told over their live connection. Each event is its own type; tests/shared holds an
// example of every one, which the web client parses too, so the two sides can't drift apart.

// friendView is a friend, and the conversation with them.
type friendView struct {
	social.CompactPlayer
	ConversationID string     `json:"conversationId"`
	Unread         int        `json:"unread,omitempty"`
	LastMessageAt  *time.Time `json:"lastMessageAt,omitempty"`
}

// socialState is everything about a person's friends, as it now is.
type socialState struct {
	Friends  []friendView           `json:"friends"`
	Outgoing []social.FriendRequest `json:"outgoing"`
	// Pending is what waits for this person's answer, with who it's from.
	Pending []pendingItem `json:"pending"`
	// What pending was made from. The conversation between two people reads these.
	incoming    []social.FriendRequest
	invitations []social.PartyInvitation
}

type pendingItem struct {
	Item   conversationItem     `json:"item"`
	Player social.CompactPlayer `json:"player"`
}

func (socialState) EventType() string { return "social.updated" }

// inboxMessage is one inbox message as clients show it.
type inboxMessage struct {
	ID        int64      `json:"id"`
	Kind      string     `json:"kind"`
	Title     string     `json:"title"`
	Body      string     `json:"body,omitempty"`
	ImageURL  string     `json:"imageUrl,omitempty"`
	Modal     bool       `json:"modal,omitempty"`
	ReadAt    *time.Time `json:"readAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

func newInboxMessage(m inbox.Stored) inboxMessage {
	return inboxMessage{
		ID: m.ID, Kind: m.Kind, Title: m.Title, Body: m.Body, ImageURL: m.ImageURL, Modal: m.Modal, ReadAt: m.ReadAt, CreatedAt: m.CreatedAt,
	}
}

func newInboxMessages(messages []inbox.Stored) []inboxMessage {
	out := make([]inboxMessage, len(messages))
	for i, m := range messages {
		out[i] = newInboxMessage(m)
	}
	return out
}

// inboxState is the start of a person's inbox.
type inboxState struct {
	Unread   int            `json:"unread"`
	Messages []inboxMessage `json:"messages"`
}

// inboxAdded is a new message; the account may have changed with it.
type inboxAdded struct {
	Message inboxMessage `json:"message"`
}

func (inboxAdded) EventType() string { return "inbox.added" }

// inboxRead says a message was read, on any device. ID 0 is all of them.
type inboxRead struct {
	ID int64 `json:"id"`
}

func (inboxRead) EventType() string { return "inbox.read" }

// itemAction is a button on a conversation item: post, then open. A client needs no idea what it does.
type itemAction struct {
	Label string `json:"label"`
	Post  string `json:"post,omitempty"`
	Open  string `json:"open,omitempty"`
}

// conversationItem is one thing in a conversation: a message, or a request waiting for an answer.
type conversationItem struct {
	ID                string       `json:"id"`
	ConversationID    string       `json:"conversationId"`
	Kind              string       `json:"kind"`
	SenderUserID      string       `json:"senderUserId"`
	SenderDisplayName string       `json:"senderDisplayName"`
	SenderTeamID      string       `json:"senderTeamId,omitempty"`
	MatchID           string       `json:"matchId,omitempty"`
	Audience          string       `json:"audience"`
	TeamID            string       `json:"teamId,omitempty"`
	Body              string       `json:"body,omitempty"`
	Emote             string       `json:"emote,omitempty"`
	CreatedAt         time.Time    `json:"createdAt"`
	Actions           []itemAction `json:"actions,omitempty"`
}

// itemAdded is a new item, or a new version of one with the same id. Notify asks for a toast.
type itemAdded struct {
	Item   conversationItem `json:"item"`
	Notify bool             `json:"notify,omitempty"`
}

func (itemAdded) EventType() string { return "conversation.item" }

// itemRemoved is an item that no longer applies, like a request that was answered.
type itemRemoved struct {
	ConversationID string `json:"conversationId"`
	ItemID         string `json:"itemId"`
}

func (itemRemoved) EventType() string { return "conversation.item_removed" }

// friendPresence is what a friend is doing now.
type friendPresence struct {
	UserID   string `json:"userId"`
	Status   string `json:"presenceStatus"`
	Activity string `json:"activity,omitempty"`
}

func (friendPresence) EventType() string { return "friend.presence" }

// partyUpdated is the whole party. A client keeps the highest revision it has seen.
type partyUpdated struct {
	Party partySnapshot `json:"party"`
}

func (partyUpdated) EventType() string { return "party.updated" }

// partyLeft tells someone they're no longer in the party.
type partyLeft struct {
	PartyID string `json:"partyId"`
	Reason  string `json:"reason"`
}

func (partyLeft) EventType() string { return "party.left" }

// queueUpdated is where a person is in the queue.
type queueUpdated struct {
	Queued   bool   `json:"queued"`
	QueuedAt int64  `json:"queuedAt,omitempty"`
	Error    string `json:"error,omitempty"`
}

func (queueUpdated) EventType() string { return "queue.updated" }

// matchStarted tells a player they're seated in a new match.
type matchStarted struct {
	Match activeMatchSummary `json:"match"`
}

func (matchStarted) EventType() string { return "match.started" }

// statusEvent is the online count and maintenance status.
type statusEvent struct {
	OnlinePlayers int                `json:"onlinePlayers"`
	Maintenance   maintenance.Status `json:"maintenance"`
}

func (statusEvent) EventType() string { return "status" }

// sessionEvent is everything a client needs. Every connection starts with one, so no client ever has
// to catch up on what it missed.
type sessionEvent struct {
	UserID string         `json:"userId,omitempty"`
	Social socialState    `json:"social"`
	Inbox  inboxState     `json:"inbox"`
	Party  *partySnapshot `json:"party,omitempty"`
	Queue  queueUpdated   `json:"queue"`
	// ActiveMatch is the match to go back to, which may have started while disconnected.
	ActiveMatch *activeMatchSummary `json:"activeMatch,omitempty"`
	Status      statusEvent         `json:"status"`
}

func (sessionEvent) EventType() string { return "session" }
