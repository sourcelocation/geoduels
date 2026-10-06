package moderation

import "strings"

func riskSignalQueues(severity string) bool {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "high", "critical":
		return true
	default:
		return false
	}
}

func normalizeRiskSignalType(value string) string {
	if strings.TrimSpace(value) == "" {
		return "gameplay_integrity"
	}
	return value
}

func normalizeRiskSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low", "medium", "high", "critical":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "low"
	}
}

func normalizeRiskEvidenceStrength(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "weak", "limited", "substantial", "strong":
		return strings.ToLower(strings.TrimSpace(value))
	case "very_low", "low":
		return "weak"
	case "medium":
		return "limited"
	case "high":
		return "substantial"
	case "very_high":
		return "strong"
	default:
		return "weak"
	}
}

func nonempty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func normalizeRiskSignal(signal RiskSignal) RiskSignal {
	signal.SignalType = normalizeRiskSignalType(signal.SignalType)
	signal.Severity = normalizeRiskSeverity(signal.Severity)
	signal.EvidenceStrength = normalizeRiskEvidenceStrength(signal.EvidenceStrength)
	signal.ReasonCode = nonempty(signal.ReasonCode, "risk_engine_signal")
	return signal
}
