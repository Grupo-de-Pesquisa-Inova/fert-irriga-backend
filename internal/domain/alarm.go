package domain

import (
	"encoding/json"
	"time"
)

// AlarmRule define uma regra de alarme configurável.
type AlarmRule struct {
	ID             string   `json:"id"`
	ZoneID         *string  `json:"zone_id,omitempty"`
	Name           string   `json:"name"`
	ConditionType  string   `json:"condition_type"`
	ChannelKey     *string  `json:"channel_key,omitempty"`
	ThresholdValue *float64 `json:"threshold_value,omitempty"`
	DurationSec    *int     `json:"duration_sec,omitempty"`
	Severity       string   `json:"severity"`
	IsEnabled      bool     `json:"is_enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AlarmEvent é um evento de alarme disparado por uma regra.
type AlarmEvent struct {
	ID             string          `json:"id"`
	AlarmRuleID    string          `json:"alarm_rule_id"`
	DeviceID       *string         `json:"device_id,omitempty"`
	Status         string          `json:"status"`
	TriggeredAt    time.Time       `json:"triggered_at"`
	AcknowledgedAt *time.Time      `json:"acknowledged_at,omitempty"`
	AcknowledgedBy *string         `json:"acknowledged_by,omitempty"`
	ResolvedAt     *time.Time      `json:"resolved_at,omitempty"`
	Context        json.RawMessage `json:"context"`
	CreatedAt      time.Time       `json:"created_at"`

	// Join field (populado em queries)
	RuleName     string `json:"rule_name,omitempty"`
	RuleSeverity string `json:"rule_severity,omitempty"`
}
