package forward

type Protocol string

const (
	ProtocolTCP    Protocol = "tcp"
	ProtocolUDP    Protocol = "udp"
	ProtocolTCPUDP Protocol = "tcp+udp"
)

type Rule struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Protocol Protocol `json:"protocol"`
	Listen   string   `json:"listen"`
	Target   string   `json:"target"`
	Enable   bool     `json:"enable"`
}

type RuleStats struct {
	RuleID       string `json:"rule_id"`
	BytesIn      uint64 `json:"bytes_in"`
	BytesOut     uint64 `json:"bytes_out"`
	ActiveConns  int    `json:"active_conns"`
	TotalConns   uint64 `json:"total_conns"`
}

type Status struct {
	TotalRules     int         `json:"total_rules"`
	ActiveRules    int         `json:"active_rules"`
	TotalConns     uint64      `json:"total_conns"`
	TotalBytesIn   uint64      `json:"total_bytes_in"`
	TotalBytesOut  uint64      `json:"total_bytes_out"`
	RuleStats      []RuleStats `json:"rule_stats"`
}

type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}
