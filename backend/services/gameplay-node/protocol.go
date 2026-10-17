package main

// The match socket's messages: clients send commands, the node acks each one and sends events.

type commandEnvelope struct {
	CommandID string         `json:"commandId"`
	Type      string         `json:"type"`
	Payload   map[string]any `json:"payload"`
	SentAt    int64          `json:"sentAt"`
}

type commandAck struct {
	Kind      string `json:"kind"`
	CommandID string `json:"commandId"`
	Status    string `json:"status"`
	ErrorCode string `json:"errorCode,omitempty"`
	Message   string `json:"message,omitempty"`
	ServerTS  int64  `json:"serverTs"`
}

type eventEnvelope struct {
	Kind     string `json:"kind"`
	EventID  string `json:"eventId"`
	Type     string `json:"type"`
	MatchID  string `json:"matchId,omitempty"`
	Seq      int64  `json:"seq,omitempty"`
	ServerTS int64  `json:"serverTs"`
	Payload  any    `json:"payload,omitempty"`
}

const (
	eventMatchSnapshot = "match.snapshot"
	eventMatchState    = "match.state"
	eventTeamPing      = "team.ping"
)

const errMatchNotFound = "ERR_MATCH_NOT_FOUND"
