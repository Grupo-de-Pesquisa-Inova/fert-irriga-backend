package domain

import (
	"testing"
	"time"
)

func TestRealtimeTelemetryExcludesReplayAndFutureClock(t *testing.T) {
	now := time.Unix(1800000000, 0)
	for _, c := range []struct {
		ts   int64
		want bool
	}{{0, true}, {now.Unix(), true}, {now.Add(-61 * time.Second).Unix(), false}, {now.Add(11 * time.Second).Unix(), false}} {
		if got := (PayloadESP32{Ts: c.ts}).IsCurrent(now); got != c.want {
			t.Fatalf("ts=%d got %v", c.ts, got)
		}
	}
}
