package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"geoduels/internal/social"
	"geoduels/pkg/live"
	"geoduels/pkg/maintenance"
)

const liveEventsFile = "live-events.json"

// One example of every event a client can be told. The web client's tests parse the same file, so a
// change on either side that the other doesn't follow fails. UPDATE_FIXTURES=1 rewrites it.
func exampleEvents() []live.Event {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	player := social.CompactPlayer{UserID: "u2", DisplayName: "Friend", Relationship: social.RelationshipFriends, PresenceStatus: "online"}
	item := conversationItem{
		ID: "request:r1", ConversationID: "direct:u1:u2", Kind: "request", SenderUserID: "u2", SenderDisplayName: "Friend", Audience: "all",
		Body: "Friend invited you to a party", CreatedAt: at,
		Actions: []itemAction{{Label: "Join", Post: "/api/party-invitations/i1/accept", Open: "/party/ABC"}, {Label: "Decline", Post: "/api/party-invitations/i1/decline"}},
	}
	message := inboxMessage{ID: 7, Kind: "badge", Title: "New badge", Body: "You earned it", ImageURL: "/badge.png", Modal: true, CreatedAt: at}
	state := socialState{
		Friends:  []friendView{{CompactPlayer: player, ConversationID: "direct:u1:u2", Unread: 2, LastMessageAt: &at}},
		Outgoing: []social.FriendRequest{{ID: "r2", Direction: "outgoing", Player: player, CreatedAt: at, ExpiresAt: at}},
		Pending:  []pendingItem{{Item: item, Player: player}},
	}
	party := partySnapshot{
		ID: "p1", Revision: 3, InviteCode: "ABC", OwnerUserID: "u1", State: "open", Mode: "duel", MapScope: "world", CreatedAt: at, ExpiresAt: at,
		Members: []partyMember{{UserID: "u1", DisplayName: "Me", Role: "owner", JoinedAt: at}},
	}
	match := activeMatchSummary{MatchID: "m1", Kind: "ranked_duel", Mode: "duel", Status: "live"}
	queue := queueUpdated{Queued: true, QueuedAt: at.UnixMilli()}
	status := statusEvent{OnlinePlayers: 12, Maintenance: maintenance.DefaultStatus()}
	return []live.Event{
		sessionEvent{
			UserID: "u1", Social: state, Inbox: inboxState{Unread: 1, Messages: []inboxMessage{message}},
			Party: &party, Queue: queue, ActiveMatch: &match, Status: status,
		},
		status,
		state,
		friendPresence{UserID: "u2", Status: "online", Activity: "in_match"},
		inboxAdded{Message: message},
		inboxRead{ID: 7},
		itemAdded{Item: item, Notify: true},
		itemRemoved{ConversationID: "direct:u1:u2", ItemID: "request:r1"},
		partyUpdated{Party: party},
		partyLeft{PartyID: "p1", Reason: "You left this party"},
		queue,
		matchStarted{Match: match},
	}
}

func TestLiveEventsMatchTheSharedExamples(t *testing.T) {
	encoded := make([]json.RawMessage, 0)
	for _, event := range exampleEvents() {
		payload, err := live.Encode(event)
		if err != nil {
			t.Fatal(err)
		}
		encoded = append(encoded, payload)
	}
	path := filepath.Join("..", "..", "..", "tests", "shared", liveEventsFile)
	if os.Getenv("UPDATE_FIXTURES") != "" {
		data, err := json.MarshalIndent(encoded, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	current, err := json.Marshal(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(current, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("events changed: update %s and the web client's parsers\nwant %s\ngot  %s", liveEventsFile, data, current)
	}
}
