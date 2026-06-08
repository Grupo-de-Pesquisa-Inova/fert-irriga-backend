package safety

import (
	"testing"
)

func TestViolationType_IsBlocking(t *testing.T) {
	tests := []struct {
		name     string
		viols    []Violation
		blocking bool
	}{
		{
			name:     "nil violations — not blocking",
			viols:    nil,
			blocking: false,
		},
		{
			name:     "empty violations — not blocking",
			viols:    []Violation{},
			blocking: false,
		},
		{
			name: "emergency — blocking",
			viols: []Violation{{
				Type:    ViolationEmergencyActive,
				Message: "Parada de emergência ativa",
			}},
			blocking: true,
		},
		{
			name: "comm timeout — blocking",
			viols: []Violation{{
				Type:    ViolationCommTimeout,
				Message: "Device offline",
			}},
			blocking: true,
		},
		{
			name: "mutex conflict — blocking",
			viols: []Violation{{
				Type:    ViolationMutexConflict,
				Message: "Mutex group conflict",
			}},
			blocking: true,
		},
		{
			name: "max runtime — blocking",
			viols: []Violation{{
				Type:    ViolationMaxRuntimeExceeded,
				Message: "Max runtime exceeded",
			}},
			blocking: true,
		},
		{
			name: "maintenance lock — blocking",
			viols: []Violation{{
				Type:    ViolationMaintenanceLock,
				Message: "Device in maintenance mode",
			}},
			blocking: true,
		},
		{
			name: "priority only — not blocking",
			viols: []Violation{{
				Type:    ViolationPriorityBlocked,
				Message: "Lower priority",
			}},
			blocking: false,
		},
		{
			name: "mixed — one blocking stops all",
			viols: []Violation{
				{Type: ViolationPriorityBlocked, Message: "Lower priority"},
				{Type: ViolationCommTimeout, Message: "Device offline"},
			},
			blocking: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsBlocking(tc.viols)
			if got != tc.blocking {
				t.Errorf("IsBlocking() = %v, want %v", got, tc.blocking)
			}
		})
	}
}

func TestViolationType_String(t *testing.T) {
	v := Violation{
		Type:    ViolationEmergencyActive,
		Message: "test message",
	}

	if v.Type != "emergency_active" {
		t.Errorf("unexpected type: %s", v.Type)
	}
	if v.Message != "test message" {
		t.Errorf("unexpected message: %s", v.Message)
	}
}
