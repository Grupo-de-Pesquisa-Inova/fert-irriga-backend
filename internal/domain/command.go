package domain

import (
	"encoding/json"
	"time"
)

// ManualCommand representa um comando rastreável enviado a um device.
type ManualCommand struct {
	ID            string          `json:"id"`
	CommandID     string          `json:"command_id"`
	DeviceID      string          `json:"device_id"`
	Origin        string          `json:"origin"`
	Actor         string          `json:"actor"`
	Action        string          `json:"action"`
	TargetChannel string          `json:"target_channel,omitempty"`
	Parameters    json.RawMessage `json:"parameters"`
	Status        string          `json:"status"`
	SafetyContext json.RawMessage `json:"safety_context,omitempty"`
	Priority      int             `json:"priority"`
	RequestedAt   time.Time       `json:"requested_at"`
	DispatchedAt  *time.Time      `json:"dispatched_at,omitempty"`
	AckedAt       *time.Time      `json:"acked_at,omitempty"`
	ExpiredAt     *time.Time      `json:"expired_at,omitempty"`
	Result        json.RawMessage `json:"result,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

// CreateCommandRequest é o payload para criar um novo comando via API.
type CreateCommandRequest struct {
	DeviceID      string          `json:"device_id"`
	Action        string          `json:"action"`
	TargetChannel string          `json:"target_channel,omitempty"`
	Parameters    json.RawMessage `json:"parameters,omitempty"`
	Origin        string          `json:"origin,omitempty"`
	Actor         string          `json:"actor,omitempty"`
	DurationSec   int             `json:"duration_sec,omitempty"`
}
