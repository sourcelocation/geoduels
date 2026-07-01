package partyevents

import "strings"

const KindChanged = "changed"

func Channel(partyID string) string {
	return "party:events:" + strings.TrimSpace(partyID)
}
