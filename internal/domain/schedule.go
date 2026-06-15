package domain

import (
	"encoding/json"
	"time"
)

// Schedule representa um agendamento de irrigação/fertirrigação.
type Schedule struct {
	ID              string     `json:"id"`
	ZoneID          *string    `json:"zone_id,omitempty"`
	DeviceID        string     `json:"device_id,omitempty"`
	RecipeID        *string    `json:"recipe_id,omitempty"`
	ValveNumber     int        `json:"valve_number"`
	Name            string     `json:"name"`
	ScheduleType    string     `json:"schedule_type"`
	CronExpression  string     `json:"cron_expression,omitempty"`
	StartAt         *time.Time `json:"start_at,omitempty"`
	StartWindowMin  int        `json:"start_window_min"`
	DurationSec     int        `json:"duration_sec"`
	Origin          string     `json:"origin"`
	IsEnabled       bool       `json:"is_enabled"`
	Version         int        `json:"version"`
	NextExecutionAt *time.Time `json:"next_execution_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"deleted_at,omitempty"`
}

// ScheduleRun registra uma execução de agendamento.
type ScheduleRun struct {
	ID         string          `json:"id"`
	ScheduleID string          `json:"schedule_id"`
	Status     string          `json:"status"`
	StartedAt  *time.Time      `json:"started_at,omitempty"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
	Result     json.RawMessage `json:"result,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
