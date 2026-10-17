package gameplay

// MatchConfig is how a match is played. Its JSON is how match sessions and parties store it.
type MatchConfig struct {
	Ruleset             Ruleset        `json:"ruleset,omitempty"`
	StreetNames         StreetNames    `json:"streetNames,omitempty"`
	MapID               string         `json:"mapId,omitempty"`
	MapName             string         `json:"mapName,omitempty"`
	MapKey              string         `json:"mapKey,omitempty"` // Legacy read compatibility.
	RoundTimerMode      RoundTimerMode `json:"roundTimerMode,omitempty"`
	RoundTimeLimitMS    int64          `json:"roundTimeLimitMs,omitempty"`
	PressureTimeLimitMS int64          `json:"pressureTimeLimitMs,omitempty"`
	MultiplierMode      MultiplierMode `json:"multiplierMode,omitempty"`
}

type Ruleset = string

const (
	RulesetMoving Ruleset = "moving"
	RulesetNoMove Ruleset = "no_move"
	RulesetNMPZ   Ruleset = "nmpz"
)

type StreetNames = string

const (
	StreetNamesShown  StreetNames = "shown"
	StreetNamesHidden StreetNames = "hidden"
)

type RoundTimerMode string

const (
	RoundTimerNone     RoundTimerMode = "none"
	RoundTimerPressure RoundTimerMode = "pressure"
	RoundTimerFixed    RoundTimerMode = "fixed"
)

type MultiplierMode string

const (
	MultiplierShared     MultiplierMode = "shared"
	MultiplierIndividual MultiplierMode = "individual"
)

const (
	DefaultFixedRoundTimeMS = int64(45_000)
	MinimumFixedRoundTimeMS = int64(10_000)
	MaximumFixedRoundTimeMS = int64(120_000)
	DefaultPressureTimeMS   = int64(15_000)
)

func NormalizeRuleset(v Ruleset) Ruleset {
	switch v {
	case RulesetNoMove:
		return RulesetNoMove
	case RulesetNMPZ:
		return RulesetNMPZ
	default:
		return RulesetMoving
	}
}

func NormalizeStreetNames(v StreetNames) StreetNames {
	if v == StreetNamesHidden {
		return StreetNamesHidden
	}
	return StreetNamesShown
}

func NormalizeMatchConfig(cfg MatchConfig) MatchConfig {
	cfg.Ruleset = NormalizeRuleset(cfg.Ruleset)
	cfg.StreetNames = NormalizeStreetNames(cfg.StreetNames)
	if cfg.MultiplierMode != MultiplierIndividual {
		cfg.MultiplierMode = MultiplierShared
	}
	if cfg.MapID == "" {
		cfg.MapID = cfg.MapKey
	}
	cfg.MapKey = ""
	switch cfg.RoundTimerMode {
	case "":
		cfg.RoundTimerMode = RoundTimerNone
		cfg.RoundTimeLimitMS = 0
		if cfg.PressureTimeLimitMS <= 0 {
			cfg.PressureTimeLimitMS = DefaultPressureTimeMS
		}
	case RoundTimerFixed:
		cfg.RoundTimerMode = RoundTimerFixed
		if cfg.RoundTimeLimitMS <= 0 {
			cfg.RoundTimeLimitMS = DefaultFixedRoundTimeMS
		}
		if cfg.RoundTimeLimitMS < MinimumFixedRoundTimeMS {
			cfg.RoundTimeLimitMS = MinimumFixedRoundTimeMS
		}
		if cfg.RoundTimeLimitMS > MaximumFixedRoundTimeMS {
			cfg.RoundTimeLimitMS = MaximumFixedRoundTimeMS
		}
	case RoundTimerPressure:
		cfg.RoundTimerMode = RoundTimerNone
		cfg.RoundTimeLimitMS = 0
		if cfg.PressureTimeLimitMS <= 0 {
			cfg.PressureTimeLimitMS = DefaultPressureTimeMS
		}
	case RoundTimerNone:
		cfg.RoundTimerMode = RoundTimerNone
		cfg.RoundTimeLimitMS = 0
	default:
		cfg.RoundTimerMode = RoundTimerNone
		cfg.RoundTimeLimitMS = 0
	}
	switch cfg.PressureTimeLimitMS {
	case 0, DefaultPressureTimeMS, 30_000, 60_000, 90_000:
	default:
		cfg.PressureTimeLimitMS = 0
	}
	return cfg
}
