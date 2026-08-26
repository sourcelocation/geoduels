package persistence

import (
	"strconv"
	"strings"
)

func mapVisibleToUserSQL(alias string, userArg int, includeUnlisted bool) string {
	qualifier := strings.TrimSpace(alias)
	if qualifier != "" {
		qualifier += "."
	}
	predicate := "(" +
		qualifier + "owner_user_id is null" +
		" or " + qualifier + "official_at is not null" +
		" or " + qualifier + "owner_user_id = nullif($" + strconv.Itoa(userArg) + ",'')::uuid" +
		" or " + qualifier + "visibility = 'public'"
	if includeUnlisted {
		predicate += " or " + qualifier + "visibility = 'unlisted'"
	}
	return predicate + ")"
}

func mapVisibleToUser(ownerUserID, accessUserID, visibility string, official bool) bool {
	if ownerUserID == "" || official || ownerUserID == accessUserID {
		return true
	}
	switch strings.TrimSpace(strings.ToLower(visibility)) {
	case "public", "unlisted":
		return true
	default:
		return false
	}
}
