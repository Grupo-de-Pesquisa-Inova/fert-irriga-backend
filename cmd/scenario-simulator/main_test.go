package main

import (
	"encoding/json"
	"slices"
	"testing"
	"time"
)

func TestLocalTimerAndDeduplication(t *testing.T) {
	s := &simulator{outcomes: map[string]outcome{}}
	command, _ := json.Marshal(map[string]any{"protocol": "scenarios_v1", "action": "start", "run_id": "timer-test", "scenario_id": "cenario-09", "duration_sec": 1, "expires_at": time.Now().Add(10 * time.Second)})
	s.command(command, "")
	s.Lock()
	ends := s.ends
	s.Unlock()
	s.command(command, "")
	s.Lock()
	if !s.ends.Equal(ends) {
		t.Fatal("duplicate reset timer")
	}
	s.Unlock()
	// No publisher or telemetry loop exists in this test.
	time.Sleep(1100 * time.Millisecond)
	s.Lock()
	defer s.Unlock()
	if s.active != "" || len(s.outputs) != 0 || s.outcomes["timer-test"].Status != "completed" {
		t.Fatal("local timer depended on MQTT")
	}
}

func TestIndividualValveCommands(t *testing.T) {
	s := &simulator{outcomes: map[string]outcome{}}
	send := func(id, target string, state, expected bool, expiry time.Time) []byte {
		raw, _ := json.Marshal(map[string]any{"protocol": "valves_v1", "action": "set_valve", "command_id": id, "target": target, "state": state, "expected_state": expected, "expires_at": expiry})
		s.command(raw, "")
		return raw
	}
	expiry := time.Now().Add(15 * time.Second)
	open := send("open", "VL13", true, false, expiry)
	if !slices.Equal(s.outputs, []string{"VL13"}) || s.valveOutcomes["open"].Status != "executed" {
		t.Fatal("wrong output activated", s.outputs)
	}
	send("expired", "VG", true, false, time.Now().Add(-time.Second))
	if s.valveOutcomes["expired"].Status != "rejected" {
		t.Fatal("expired command executed")
	}
	send("close", "VL13", false, true, expiry)
	s.command(open, "")
	if len(s.outputs) != 0 {
		t.Fatal("duplicate old command reopened the valve")
	}
	s.active = "scenario"
	send("busy", "VL13", true, false, expiry)
	if s.valveOutcomes["busy"].Status != "rejected" || len(s.outputs) != 0 {
		t.Fatal("scenario interlock bypassed")
	}
}
func TestStopBeforeStartAndExpiredCommand(t *testing.T) {
	s := &simulator{outcomes: map[string]outcome{}}
	s.command([]byte(`{"action":"stop","run_id":"cancelled"}`), "")
	for _, id := range []string{"cancelled", "expired"} {
		raw, _ := json.Marshal(map[string]any{"protocol": "scenarios_v1", "action": "start", "run_id": id, "scenario_id": "cenario-09", "duration_sec": 5, "expires_at": time.Now().Add(-time.Second)})
		s.command(raw, "")
	}
	if s.active != "" || s.outcomes["cancelled"].Status != "cancelled" || s.outcomes["expired"].ErrorCode != "expired" {
		t.Fatal(s.outcomes)
	}
}
