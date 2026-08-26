package main

import (
	"errors"
	"testing"
	"time"

	"geoduels/pkg/contentfilter"
	"geoduels/pkg/contracts"
	"geoduels/pkg/persistence"
)

func TestBuildCoordinatorChatMessageRejectsAbusiveText(t *testing.T) {
	contentfilter.SetDefaultFilter(testChatFilter{blocked: "blocked phrase"})
	defer contentfilter.SetDefaultFilter(nil)

	q := &matchCoordinator{}
	_, err := q.buildCoordinatorChatMessage(chatScope{ConversationID: "party:p1", Kind: "party", ID: "p1"}, "u1", "Player", chatClientCommand{
		Type: "chat.send",
		Payload: map[string]any{
			"body": "blocked phrase",
		},
	})
	if !errors.Is(err, contentfilter.ErrAbusiveText) {
		t.Fatalf("buildCoordinatorChatMessage error = %v, want %v", err, contentfilter.ErrAbusiveText)
	}
}

func TestBuildCoordinatorChatMessageAcceptsWaveEmote(t *testing.T) {
	q := &matchCoordinator{}
	message, err := q.buildCoordinatorChatMessage(chatScope{ConversationID: "party:p1", Kind: "party", ID: "p1"}, "u1", "Player", chatClientCommand{
		Type: "chat.emote",
		Payload: map[string]any{
			"emote": "wave",
		},
	})
	if err != nil {
		t.Fatalf("buildCoordinatorChatMessage error = %v", err)
	}
	if message.Emote != contracts.ChatEmoteWave {
		t.Fatalf("message.Emote = %q, want %q", message.Emote, contracts.ChatEmoteWave)
	}
}

func TestBuildCoordinatorChatMessageAcceptsTeamAudience(t *testing.T) {
	q := &matchCoordinator{}
	message, err := q.buildCoordinatorChatMessage(chatScope{ConversationID: "party:p1", Kind: "party", ID: "p1"}, "u1", "Player", chatClientCommand{
		Type:    "chat.send",
		Payload: map[string]any{"body": "north", "audience": "team"},
	})
	if err != nil {
		t.Fatalf("buildCoordinatorChatMessage error = %v", err)
	}
	if message.Audience != contracts.ChatAudienceTeam {
		t.Fatalf("message.Audience = %q, want %q", message.Audience, contracts.ChatAudienceTeam)
	}
}

func TestBuildCoordinatorChatMessageRejectsUnknownAudience(t *testing.T) {
	q := &matchCoordinator{}
	_, err := q.buildCoordinatorChatMessage(chatScope{ConversationID: "party:p1", Kind: "party", ID: "p1"}, "u1", "Player", chatClientCommand{
		Type:    "chat.send",
		Payload: map[string]any{"body": "north", "audience": "opponents"},
	})
	if err == nil {
		t.Fatal("expected unsupported audience error")
	}
}

func TestChatRestrictionErrorMessageIncludesExpiry(t *testing.T) {
	endsAt := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	got := chatRestrictionErrorMessage(persistence.ChatRestriction{ActionType: "chat_mute", EndsAt: endsAt})
	want := "chat access is restricted until 2026-07-28T12:00:00Z"
	if got != want {
		t.Fatalf("chatRestrictionErrorMessage = %q, want %q", got, want)
	}
}

type testChatFilter struct {
	blocked string
}

func (f testChatFilter) IsAbusive(text string) bool {
	return text == f.blocked
}
