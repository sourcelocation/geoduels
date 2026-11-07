package inbox

import "fmt"

// Message is one inbox message, in the words people see. Only the constructors below make one, so a
// message always has its words and its key.
type Message struct {
	kind, dedupeKey, title, body, imageURL string
	// modal asks for the reader's attention: it opens by itself and stays until read.
	modal bool
}

// FriendshipAccepted tells someone their request was accepted.
func FriendshipAccepted(requestID, friendName string) Message {
	return Message{
		kind: "friendship_accepted", dedupeKey: "friendship_accepted:" + requestID,
		title: friendName + " accepted your friend request",
	}
}

// BadgeUnlocked tells someone about a new badge. A badge unlocks once per level.
func BadgeUnlocked(userID, badgeID string, level int, label, description, imageURL string) Message {
	return Message{
		kind: "badge_unlocked", dedupeKey: fmt.Sprintf("badge_unlocked:%s:%s:%d", userID, badgeID, level),
		title: "New badge unlocked: " + label, body: description, imageURL: imageURL,
	}
}

// RatingRefunded tells someone they got rating back because their opponent cheated.
func RatingRefunded(dedupeKey string, delta int) Message {
	return Message{
		kind: "mmr_refund", dedupeKey: dedupeKey, title: fmt.Sprintf("Rating refunded: +%d MMR", delta),
		body: "A player you lost to was banned for cheating. Your rating has been recalculated.",
	}
}

// AccountBanned tells someone their account is suspended.
func AccountBanned(logID int64, reason string) Message {
	body := "Your account access has been restricted."
	if reason != "" {
		body = "Reason: " + reason
	}
	return Message{
		kind: "account_banned", dedupeKey: fmt.Sprintf("account_banned:%d", logID),
		title: "Account suspended", body: body, modal: true,
	}
}

// AccountUnbanned tells someone their restriction was lifted.
func AccountUnbanned(logID int64) Message {
	return Message{kind: "account_unbanned", dedupeKey: fmt.Sprintf("account_unbanned:%d", logID), title: "Account restriction removed"}
}

// ReportedPlayerBanned thanks someone whose report led to a ban.
func ReportedPlayerBanned(logID int64, reporterID string) Message {
	return Message{
		kind: "reported_player_banned", dedupeKey: fmt.Sprintf("reported_player_banned:%d:%s", logID, reporterID),
		title: "Report action taken", body: "A player you reported was suspended after review. Thanks for keeping GeoDuels fair.",
	}
}

// Warned is a warning from the moderators. Reading it is acknowledging it.
func Warned(warningID int64, reason, nicknameReset string) Message {
	body := reason
	if nicknameReset != "" {
		body += " Your nickname was reset to " + nicknameReset + "."
	}
	return Message{
		kind: "moderation_warning", dedupeKey: fmt.Sprintf("warning:%d", warningID),
		title: "Moderation warning", body: body, modal: true,
	}
}

// WarningWithdrawn tells someone a warning no longer stands.
func WarningWithdrawn(warningID int64) Message {
	return Message{
		kind: "moderation_warning_withdrawn", dedupeKey: fmt.Sprintf("warning_withdrawn:%d", warningID),
		title: "A warning was withdrawn",
	}
}
