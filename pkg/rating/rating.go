package rating

import (
	"math"
	"time"
)

const (
	InitialMMR       = 500
	InitialRatingRD  = 220.0
	MinimumRatingRD  = 110.0
	MaximumRatingRD  = 220.0
	MinimumRankedMMR = 500
	MaxDuelMMRDelta  = 40

	LowMMRForgivenessEndMMR = 1000

	ProvisionalLossMinFactor    = 0.20
	ProvisionalProtectionMMRGap = 250
)

var glickoC = math.Sqrt((MaximumRatingRD*MaximumRatingRD - MinimumRatingRD*MinimumRatingRD) / 365.0)

type State struct {
	MMR       int
	RD        float64
	UpdatedAt time.Time
}

type Update struct {
	MMR   int
	RD    float64
	Delta int
}

func CalculateDuelUpdates(p1, p2 State, winner string, now time.Time) (Update, Update) {
	p1InflatedRD := InflateRD(p1.RD, p1.UpdatedAt, now)
	p2InflatedRD := InflateRD(p2.RD, p2.UpdatedAt, now)

	var p1Score, p2Score float64 = 0.5, 0.5
	switch winner {
	case "p1":
		p1Score, p2Score = 1, 0
	case "p2":
		p1Score, p2Score = 0, 1
	}

	p1MMR, p1NextRD := calculateGlickoRating(p1.MMR, p1InflatedRD, p2.MMR, p2InflatedRD, p1Score)
	p2MMR, p2NextRD := calculateGlickoRating(p2.MMR, p2InflatedRD, p1.MMR, p1InflatedRD, p2Score)

	p1MMR = CapDelta(p1.MMR, p1MMR)
	p2MMR = CapDelta(p2.MMR, p2MMR)
	p1MMR = ApplyProvisionalOpponentLossProtection(p1.MMR, p2.MMR, p2InflatedRD, p1MMR)
	p2MMR = ApplyProvisionalOpponentLossProtection(p2.MMR, p1.MMR, p1InflatedRD, p2MMR)
	p1MMR = ApplyLowMMRLossForgiveness(p1.MMR, p1MMR)
	p2MMR = ApplyLowMMRLossForgiveness(p2.MMR, p2MMR)
	p1MMR = ClampRankedMMR(p1MMR)
	p2MMR = ClampRankedMMR(p2MMR)

	return Update{MMR: p1MMR, RD: p1NextRD, Delta: p1MMR - p1.MMR},
		Update{MMR: p2MMR, RD: p2NextRD, Delta: p2MMR - p2.MMR}
}

func CalculateDuelDeltas(p1MMR, p2MMR int, p1RD, p2RD float64, winner string) (int, int) {
	now := time.Now()
	p1, p2 := CalculateDuelUpdates(
		State{MMR: p1MMR, RD: p1RD, UpdatedAt: now},
		State{MMR: p2MMR, RD: p2RD, UpdatedAt: now},
		winner,
		now,
	)
	return p1.Delta, p2.Delta
}

func calculateGlickoRating(rating int, rd float64, opponentRating int, opponentRD float64, score float64) (int, float64) {
	rd = ClampRD(rd)
	opponentRD = ClampRD(opponentRD)

	q := math.Ln10 / 400.0
	g := glickoG(opponentRD)
	expected := glickoExpected(rating, opponentRating, opponentRD)
	dSquared := 1.0 / (q * q * g * g * expected * (1.0 - expected))
	nextVariance := 1.0 / ((1.0 / (rd * rd)) + (1.0 / dSquared))
	nextRating := float64(rating) + q*nextVariance*g*(score-expected)

	return int(math.Round(nextRating)), ClampRD(math.Sqrt(nextVariance))
}

func glickoG(rd float64) float64 {
	q := math.Ln10 / 400.0
	return 1.0 / math.Sqrt(1.0+(3.0*q*q*rd*rd)/(math.Pi*math.Pi))
}

func glickoExpected(selfMMR, opponentMMR int, opponentRD float64) float64 {
	return 1.0 / (1.0 + math.Pow(10.0, -glickoG(opponentRD)*float64(selfMMR-opponentMMR)/400.0))
}

func InflateRD(rd float64, lastUpdated, now time.Time) float64 {
	rd = ClampRD(rd)
	if lastUpdated.IsZero() || now.IsZero() || !now.After(lastUpdated) {
		return rd
	}
	inactiveDays := now.Sub(lastUpdated).Hours() / 24.0
	if inactiveDays <= 0 {
		return rd
	}
	return ClampRD(math.Sqrt(rd*rd + glickoC*glickoC*inactiveDays))
}

func ClampRD(rd float64) float64 {
	if rd <= 0 || math.IsNaN(rd) || math.IsInf(rd, 0) {
		return InitialRatingRD
	}
	if rd < MinimumRatingRD {
		return MinimumRatingRD
	}
	if rd > MaximumRatingRD {
		return MaximumRatingRD
	}
	return rd
}

func ClampRankedMMR(mmr int) int {
	if mmr < MinimumRankedMMR {
		return MinimumRankedMMR
	}
	return mmr
}

func CapDelta(current, next int) int {
	delta := next - current
	if delta > MaxDuelMMRDelta {
		return current + MaxDuelMMRDelta
	}
	if delta < -MaxDuelMMRDelta {
		return current - MaxDuelMMRDelta
	}
	return next
}

func ApplyLowMMRLossForgiveness(current, next int) int {
	delta := next - current
	if delta >= 0 || current >= LowMMRForgivenessEndMMR {
		return next
	}
	if current <= MinimumRankedMMR {
		return current
	}
	rangeSize := LowMMRForgivenessEndMMR - MinimumRankedMMR
	if rangeSize <= 0 {
		return next
	}
	factor := float64(current-MinimumRankedMMR) / float64(rangeSize)
	return current + int(math.Round(float64(delta)*factor))
}

func ApplyProvisionalOpponentLossProtection(selfMMR, opponentMMR int, opponentRD float64, next int) int {
	delta := next - selfMMR
	if delta >= 0 || opponentMMR > selfMMR-ProvisionalProtectionMMRGap {
		return next
	}
	factor := provisionalRDFactor(opponentRD)
	if factor >= 1 {
		return next
	}
	return selfMMR + int(math.Round(float64(delta)*factor))
}

func provisionalRDFactor(rd float64) float64 {
	rd = ClampRD(rd)
	rangeSize := MaximumRatingRD - MinimumRatingRD
	if rangeSize <= 0 {
		return 1
	}
	certainty := 1 - ((rd - MinimumRatingRD) / rangeSize)
	if certainty < ProvisionalLossMinFactor {
		return ProvisionalLossMinFactor
	}
	if certainty > 1 {
		return 1
	}
	return certainty
}
