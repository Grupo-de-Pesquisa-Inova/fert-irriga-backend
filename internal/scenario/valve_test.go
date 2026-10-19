package scenario

import (
	"fertirriga-backend/internal/domain"
	"testing"
	"time"
)

func TestIndividualValveInterlocks(t *testing.T) {
	tests := []struct {
		name                     string
		outputs                  []string
		target                   string
		state, expected, blocked bool
	}{
		{"open selected valve", nil, "VL13", true, false, false},
		{"close selected valve", []string{"VL13"}, "VL13", false, true, false},
		{"pump not individually controllable", nil, "M", true, false, true},
		{"stale confirmation", []string{"VL13"}, "VL13", true, false, true},
		{"running pump", []string{"M", "VL13"}, "VL13", false, true, true},
		{"mixed sources", []string{"VA"}, "VAD1", true, false, true},
		{"wrong return tank", []string{"VAD1"}, "VRA2", true, false, true},
		{"return with irrigation", []string{"VG"}, "VRA1", true, false, true},
		{"safe closing remains available", []string{"VA", "VAD1"}, "VAD1", false, true, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if blocked := CheckValveOutputs(test.outputs, test.target, test.state, test.expected) != nil; blocked != test.blocked {
				t.Fatalf("blocked=%v", blocked)
			}
		})
	}
}

func TestIndividualValveRequiresCurrentCompatibleDevice(t *testing.T) {
	now := time.Now()
	device := &domain.Device{IsOnline: true, LastSeenAt: &now, Payload: domain.PayloadESP32{Capabilities: []string{"scenarios_v1"}}}
	if CheckValveDevice(device, "VL13", true, false, now) == nil {
		t.Fatal("legacy firmware accepted")
	}
	device.Payload.Capabilities = append(device.Payload.Capabilities, "valves_v1")
	if err := CheckValveDevice(device, "VL13", true, false, now); err != nil {
		t.Fatal(err)
	}
	if CheckValveDevice(device, "VL13", true, false, now.Add(16*time.Second)) == nil {
		t.Fatal("stale telemetry accepted")
	}
	device.Payload.Seguranca.ParadaEmergencia = true
	if CheckValveDevice(device, "VL13", true, false, now) == nil {
		t.Fatal("emergency ignored")
	}
}
