package scenario

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"fertirriga-backend/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type testDevice struct{ d *domain.Device }

func (d testDevice) GetByDeviceID(context.Context, string) (*domain.Device, error) { return d.d, nil }

type testMQTT struct{ calls int }

func (p *testMQTT) PublishCommand(context.Context, string, string, []byte) error {
	p.calls++
	return nil
}

func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("SCENARIO_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set SCENARIO_TEST_DATABASE_URL to an isolated PostgreSQL")
	}
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	if config.ConnConfig.Host != "127.0.0.1" && config.ConnConfig.Host != "localhost" {
		t.Fatal("integration tests require a local database")
	}
	admin, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("scenario_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close(); admin.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	sql, err := os.ReadFile("../repository/migrations/015_scenarios.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(sql)); err != nil {
		t.Fatal(err)
	}
	return &Store{Pool: pool}
}
func TestExecutionLifecycle(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	now := time.Now()
	device := &domain.Device{IsOnline: true, LastSeenAt: &now, Payload: domain.PayloadESP32{Capabilities: []string{"scenarios_v1"}}}
	mqtt := &testMQTT{}
	svc := &Service{Store: store, Devices: testDevice{device}, MQTT: mqtt}
	r, err := svc.Start(ctx, "sim", "cenario-09", 30)
	if err != nil || r.Status != "pending" || mqtt.calls != 1 {
		t.Fatalf("%+v %v calls=%d", r, err, mqtt.calls)
	}
	if _, err = svc.Start(ctx, "sim", "cenario-01", 5); err == nil {
		t.Fatal("overlapping start accepted")
	}
	event := func(device, status string) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"run_id": r.ID, "status": status, "started_at": now, "finished_at": time.Now()})
		if err := store.Outcome(ctx, device, raw); err != nil {
			t.Fatal(err)
		}
	}
	event("wrong-device", "completed")
	runs, _ := store.Runs(ctx, "sim")
	if runs[0].Status != "pending" {
		t.Fatal("cross-device ack accepted")
	}
	event("sim", "running")
	runs, _ = store.Runs(ctx, "sim")
	if runs[0].Status != "running" || runs[0].EndsAt == nil {
		t.Fatal("missing confirmed timer")
	}
	event("sim", "completed")
	event("sim", "running")
	event("sim", "failed")
	runs, _ = store.Runs(ctx, "sim")
	if runs[0].Status != "completed" {
		t.Fatal("terminal status regressed")
	}
	device.IsOnline = false
	failed, err := svc.Start(ctx, "sim", "cenario-09", 5)
	if err != nil || failed.ErrorCode != "device_offline" || mqtt.calls != 1 {
		t.Fatalf("offline dispatch: %+v %v", failed, err)
	}
}
func TestMissedScheduleAndDailyRecovery(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	now := time.Now()
	due := now.Add(-49 * time.Hour)
	p := Plan{DeviceID: "sim", ScenarioID: "cenario-01", Name: "daily", DurationSec: 30, StartAt: due, Repeat: "daily", Enabled: true, NextAt: &due}
	if err := store.SavePlan(ctx, &p); err != nil {
		t.Fatal(err)
	}
	r, err := store.Claim(ctx, now)
	if err != nil || r == nil || r.ErrorCode != "schedule_missed" {
		t.Fatalf("%+v %v", r, err)
	}
	plans, err := store.Plans(ctx, "sim")
	if err != nil || !plans[0].NextAt.After(now) {
		t.Fatal(plans, err)
	}
	if r, err = store.Claim(ctx, now); err != nil || r != nil {
		t.Fatal("catch-up dispatch", r, err)
	}
}
func TestTimeoutAndLateOutcome(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	r := Run{DeviceID: "sim", ScenarioID: "cenario-01", DurationSec: 30, DueAt: time.Now(), Status: "pending"}
	if err := store.CreateRun(ctx, &r); err != nil {
		t.Fatal(err)
	}
	store.Pool.Exec(ctx, `UPDATE scenario_runs SET created_at=now()-interval '30 seconds' WHERE id=$1`, r.ID)
	if err := store.Expire(ctx); err != nil {
		t.Fatal(err)
	}
	runs, _ := store.Runs(ctx, "sim")
	if runs[0].Status != "uncertain" || runs[0].ErrorCode != "ack_timeout" {
		t.Fatal(runs)
	}
	other := r
	other.ID = ""
	if err := store.CreateRun(ctx, &other); err == nil {
		t.Fatal("uncertain run did not lock device")
	}
	raw, _ := json.Marshal(map[string]any{"run_id": r.ID, "status": "cancelled", "finished_at": time.Now()})
	if err := store.Outcome(ctx, "sim", raw); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateRun(ctx, &other); err != nil {
		t.Fatal("confirmed stop did not unlock", err)
	}
}
func TestConcurrentClaims(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	now := time.Now()
	p := Plan{DeviceID: "sim", ScenarioID: "cenario-01", Name: "once", DurationSec: 30, StartAt: now, Repeat: "once", Enabled: true, NextAt: &now}
	if err := store.SavePlan(ctx, &p); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan *Run, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := store.Claim(ctx, now)
			if err != nil {
				t.Error(err)
			}
			results <- r
		}()
	}
	wg.Wait()
	close(results)
	count := 0
	for r := range results {
		if r != nil {
			count++
		}
	}
	if count != 1 {
		t.Fatal("duplicate dispatch", count)
	}
}
func TestConflictingScheduleIsJournaled(t *testing.T) {
	store := testStore(t)
	ctx := context.Background()
	now := time.Now()
	active := Run{DeviceID: "sim", ScenarioID: "cenario-01", DurationSec: 30, DueAt: now, Status: "pending"}
	if err := store.CreateRun(ctx, &active); err != nil {
		t.Fatal(err)
	}
	p := Plan{DeviceID: "sim", ScenarioID: "cenario-02", Name: "conflict", DurationSec: 30, StartAt: now, Repeat: "once", Enabled: true, NextAt: &now}
	if err := store.SavePlan(ctx, &p); err != nil {
		t.Fatal(err)
	}
	r, err := store.Claim(ctx, now)
	if err != nil || r == nil || r.ErrorCode != "device_busy" {
		t.Fatalf("%+v %v", r, err)
	}
}
