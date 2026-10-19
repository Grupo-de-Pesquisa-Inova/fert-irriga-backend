package handler

import (
	"encoding/json"
	"fertirriga-backend/internal/domain"
	"testing"
	"time"
)

func TestHydraulicCommandDoesNotUseLegacyRelayMapping(t *testing.T) {
	cmd := &domain.ManualCommand{CommandID: "example", TargetChannel: "VL13", Action: "open_valve", Parameters: json.RawMessage(`{"expected_state":false}`)}
	raw, err := buildValveCommandPayload(cmd)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Protocol string    `json:"protocol"`
		Target   string    `json:"target"`
		State    bool      `json:"state"`
		Expires  time.Time `json:"expires_at"`
		Legacy   any       `json:"telecomando"`
	}
	if err = json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Protocol != "valves_v1" || payload.Target != "VL13" || !payload.State || payload.Legacy != nil || time.Until(payload.Expires) < 10*time.Second {
		t.Fatal(string(raw))
	}
	cmd.Parameters = json.RawMessage(`{}`)
	if _, err = buildValveCommandPayload(cmd); err == nil {
		t.Fatal("missing expected state accepted")
	}
	cmd.TargetChannel = "irrigacao_conj1"
	if _, err = buildValveCommandPayload(cmd); err != nil {
		t.Fatal("legacy command regressed", err)
	}
}
