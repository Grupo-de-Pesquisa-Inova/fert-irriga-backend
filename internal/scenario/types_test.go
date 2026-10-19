package scenario

import (
	"fertirriga-backend/internal/domain"
	"testing"
	"time"
)

func TestHydraulicCatalog(t *testing.T) {
	if len(Catalog) != 11 {
		t.Fatal("expected 11 Blender scenarios")
	}
	seen := map[string]bool{}
	for _, c := range Catalog {
		if seen[c.ID] {
			t.Fatal("duplicate", c.ID)
		}
		seen[c.ID] = true
		outputs := map[string]bool{}
		for _, v := range c.Outputs {
			outputs[v] = true
		}
		sources := 0
		for _, v := range []string{"VA", "VAD1", "VAD2"} {
			if outputs[v] {
				sources++
			}
		}
		if sources != 1 || !outputs["M"] {
			t.Fatal("invalid source/pump", c)
		}
		cleaning := outputs["VRA1"] || outputs["VRA2"]
		if cleaning && (outputs["VG"] || outputs["VL13"] || outputs["VL46"]) {
			t.Fatal("return must isolate crop lines", c)
		}
		if !cleaning && (!outputs["VG"] || (!outputs["VL13"] && !outputs["VL46"])) {
			t.Fatal("missing discharge", c)
		}
	}
}
func TestValidation(t *testing.T) {
	for _, d := range []int{0, -1, 7201} {
		if Validate("esp32-001", "cenario-01", d) == nil {
			t.Fatal("accepted duration", d)
		}
	}
	if Validate("esp32-001", "invalid", 1) == nil || Validate("a/+/b", "cenario-01", 1) == nil {
		t.Fatal("invalid command accepted")
	}
	if err := Validate("esp32-001", "cenario-11", 1); err != nil {
		t.Fatal(err)
	}
}
func TestDailyDoesNotDriftOrCatchUp(t *testing.T) {
	due := time.Date(2026, 10, 9, 11, 0, 0, 0, time.UTC)
	now := due.Add(49 * time.Hour)
	next := NextDaily(due, now)
	if !next.Equal(due.Add(72 * time.Hour)) {
		t.Fatal(next)
	}
}
func TestSafety(t *testing.T) {
	now := time.Now()
	fresh := func() *domain.Device {
		return &domain.Device{IsOnline: true, LastSeenAt: &now, Payload: domain.PayloadESP32{Capabilities: []string{"scenarios_v1"}}}
	}
	cases := []struct {
		name, code string
		change     func(*domain.Device)
	}{
		{"offline", "device_offline", func(d *domain.Device) { d.IsOnline = false }},
		{"stale", "device_offline", func(d *domain.Device) { old := now.Add(-61 * time.Second); d.LastSeenAt = &old }},
		{"no timestamp", "device_offline", func(d *domain.Device) { d.LastSeenAt = nil }},
		{"unsupported", "unsupported_firmware", func(d *domain.Device) { d.Payload.Capabilities = nil }},
		{"emergency", "emergency", func(d *domain.Device) { d.Payload.Seguranca.ParadaEmergencia = true }},
		{"flow", "flow_fault", func(d *domain.Device) { d.Payload.Seguranca.AlertaFalhaFluxo = true }},
		{"clock", "clock_desync", func(d *domain.Device) { d.Payload.Seguranca.RelogioDessincronizado = true }},
		{"busy", "device_busy", func(d *domain.Device) { d.Payload.StatusSistema.Operacao.SaidasAtivas = []string{"VA"} }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := fresh()
			c.change(d)
			code, _ := CheckDevice(d, now)
			if code != c.code {
				t.Fatal(code)
			}
		})
	}
	if code, _ := CheckDevice(fresh(), now); code != "" {
		t.Fatal(code)
	}
}
