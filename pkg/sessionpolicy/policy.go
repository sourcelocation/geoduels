package sessionpolicy

import "geoduels/pkg/contracts"

type Decision string

const (
	DecisionReady   Decision = "ready"
	DecisionResume  Decision = "resume"
	DecisionReplace Decision = "replace"
	DecisionReject  Decision = "reject"
)

func NormalizeMode(mode contracts.MatchMode, matchID string) contracts.MatchMode {
	if mode != "" {
		return mode
	}
	return contracts.ModeDuel
}

func Resolve(existingMode, requestedMode contracts.MatchMode) Decision {
	existingMode = NormalizeMode(existingMode, "")
	requestedMode = NormalizeMode(requestedMode, "")
	if requestedMode == "" {
		return DecisionReject
	}
	if existingMode == "" {
		return DecisionReady
	}
	switch requestedMode {
	case contracts.ModeSingleplayer:
		switch existingMode {
		case contracts.ModeSingleplayer:
			return DecisionReplace
		case contracts.ModeDuel:
			return DecisionReject
		default:
			return DecisionReject
		}
	case contracts.ModeDuel:
		switch existingMode {
		case contracts.ModeDuel:
			return DecisionResume
		case contracts.ModeSingleplayer:
			return DecisionReplace
		default:
			return DecisionReject
		}
	default:
		if existingMode == requestedMode {
			return DecisionResume
		}
		return DecisionReject
	}
}
