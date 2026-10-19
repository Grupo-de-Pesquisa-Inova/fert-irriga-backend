package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ Pool *pgxpool.Pool }

func (s *Store) Plans(ctx context.Context, device string) ([]Plan, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id,device_id,scenario_id,name,duration_sec,start_at,repeat,enabled,next_at FROM scenario_plans WHERE device_id=$1 AND deleted_at IS NULL ORDER BY start_at`, device)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	plans := []Plan{}
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.DeviceID, &p.ScenarioID, &p.Name, &p.DurationSec, &p.StartAt, &p.Repeat, &p.Enabled, &p.NextAt); err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

func (s *Store) SavePlan(ctx context.Context, p *Plan) error {
	if p.ID == "" {
		return s.Pool.QueryRow(ctx, `INSERT INTO scenario_plans(device_id,scenario_id,name,duration_sec,start_at,repeat,enabled,next_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`, p.DeviceID, p.ScenarioID, p.Name, p.DurationSec, p.StartAt, p.Repeat, p.Enabled, p.NextAt).Scan(&p.ID)
	}
	tag, err := s.Pool.Exec(ctx, `UPDATE scenario_plans SET scenario_id=$2,name=$3,duration_sec=$4,start_at=$5,repeat=$6,enabled=$7,next_at=$8 WHERE id=$1 AND device_id=$9 AND deleted_at IS NULL`, p.ID, p.ScenarioID, p.Name, p.DurationSec, p.StartAt, p.Repeat, p.Enabled, p.NextAt, p.DeviceID)
	if err == nil && tag.RowsAffected() == 0 {
		return errors.New("agendamento não encontrado")
	}
	return err
}

const runColumns = `id,plan_id,device_id,scenario_id,duration_sec,due_at,status,error_code,message,created_at,started_at,ends_at,finished_at`

func scanRun(row pgx.Row) (Run, error) {
	var r Run
	err := row.Scan(&r.ID, &r.PlanID, &r.DeviceID, &r.ScenarioID, &r.DurationSec, &r.DueAt, &r.Status, &r.ErrorCode, &r.Message, &r.CreatedAt, &r.StartedAt, &r.EndsAt, &r.FinishedAt)
	return r, err
}
func (s *Store) Runs(ctx context.Context, device string) ([]Run, error) {
	rows, err := s.Pool.Query(ctx, `SELECT `+runColumns+` FROM scenario_runs WHERE device_id=$1 ORDER BY created_at DESC LIMIT 100`, device)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []Run{}
	for rows.Next() {
		r, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}
func (s *Store) CreateRun(ctx context.Context, r *Run) error {
	return s.Pool.QueryRow(ctx, `INSERT INTO scenario_runs(plan_id,device_id,scenario_id,duration_sec,due_at,status,error_code,message,finished_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,CASE WHEN $6='failed' THEN now() ELSE NULL END) RETURNING id,created_at`, r.PlanID, r.DeviceID, r.ScenarioID, r.DurationSec, r.DueAt, r.Status, r.ErrorCode, r.Message).Scan(&r.ID, &r.CreatedAt)
}

// Claim advances a schedule and journals the attempt atomically, across workers/restarts.
func (s *Store) Claim(ctx context.Context, now time.Time) (*Run, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var p Plan
	var due time.Time
	err = tx.QueryRow(ctx, `SELECT id,device_id,scenario_id,duration_sec,repeat,next_at FROM scenario_plans WHERE enabled AND deleted_at IS NULL AND next_at <= $1 ORDER BY next_at FOR UPDATE SKIP LOCKED LIMIT 1`, now).Scan(&p.ID, &p.DeviceID, &p.ScenarioID, &p.DurationSec, &p.Repeat, &due)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var next *time.Time
	if p.Repeat == "daily" {
		n := NextDaily(due, now)
		next = &n
	}
	if _, err = tx.Exec(ctx, `UPDATE scenario_plans SET next_at=$2,enabled=$3 WHERE id=$1`, p.ID, next, next != nil); err != nil {
		return nil, err
	}
	r := Run{PlanID: &p.ID, DeviceID: p.DeviceID, ScenarioID: p.ScenarioID, DurationSec: p.DurationSec, DueAt: due, Status: "pending"}
	if now.Sub(due) > 60*time.Second {
		r.Status = "failed"
		r.ErrorCode = "schedule_missed"
		r.Message = "Horário perdido: o agendador não processou a ativação dentro de 60 segundos. Verifique indisponibilidade do servidor ou banco. Não houve acionamento tardio."
		if p.Repeat == "daily" {
			r.Message += fmt.Sprintf(" %d horário(s) diário(s) vencido(s) desde esta ocorrência.", int(now.Sub(due)/(24*time.Hour))+1)
		}
	}
	// Savepoint handles simultaneous manual starts without losing the missed-run journal.
	if _, err = tx.Exec(ctx, `SAVEPOINT reserve_device`); err != nil {
		return nil, err
	}
	insert := func() error {
		return tx.QueryRow(ctx, `INSERT INTO scenario_runs(plan_id,device_id,scenario_id,duration_sec,due_at,status,error_code,message,finished_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,CASE WHEN $6='failed' THEN now() ELSE NULL END) RETURNING id,created_at`, r.PlanID, r.DeviceID, r.ScenarioID, r.DurationSec, r.DueAt, r.Status, r.ErrorCode, r.Message).Scan(&r.ID, &r.CreatedAt)
	}
	if err = insert(); err != nil {
		if _, rb := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT reserve_device`); rb != nil {
			return nil, rb
		}
		r.Status = "failed"
		r.ErrorCode = "device_busy"
		r.Message = "Outro cenário está ativo ou aguarda confirmação de parada."
		if err = insert(); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) Fail(ctx context.Context, id, code, message string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE scenario_runs SET status='failed',error_code=$2,message=$3,finished_at=now() WHERE id=$1 AND status='pending'`, id, code, message)
	return err
}

// MQTT outcomes are scoped to the device and run; duplicates/out-of-order ACKs cannot regress terminal states.
func (s *Store) Outcome(ctx context.Context, device string, raw []byte) error {
	var e struct {
		RunID      string    `json:"run_id"`
		Status     string    `json:"status"`
		ErrorCode  string    `json:"error_code"`
		Message    string    `json:"message"`
		StartedAt  time.Time `json:"started_at"`
		FinishedAt time.Time `json:"finished_at"`
	}
	if err := json.Unmarshal(raw, &e); err != nil {
		return err
	}
	if e.RunID == "" {
		return nil
	}
	switch e.Status {
	case "running":
		if e.StartedAt.IsZero() || e.StartedAt.After(time.Now().Add(10*time.Second)) {
			return errors.New("início de execução inválido")
		}
		_, err := s.Pool.Exec(ctx, `UPDATE scenario_runs SET status='running',started_at=$3,ends_at=$3+duration_sec*interval '1 second',error_code='',message='' WHERE id=$1 AND device_id=$2 AND status IN ('pending','uncertain') AND $3 >= created_at - interval '10 seconds'`, e.RunID, device, e.StartedAt)
		return err
	case "completed", "cancelled", "failed":
		if e.FinishedAt.IsZero() || e.FinishedAt.After(time.Now().Add(10*time.Second)) {
			return errors.New("fim de execução inválido")
		}
		if e.Status == "failed" && e.ErrorCode == "" {
			e.ErrorCode = "device_error"
		}
		_, err := s.Pool.Exec(ctx, `UPDATE scenario_runs SET status=$3,error_code=$4,message=$5,finished_at=$6 WHERE id=$1 AND device_id=$2 AND status IN ('pending','running','stopping','uncertain') AND $6 >= created_at - interval '10 seconds'`, e.RunID, device, e.Status, e.ErrorCode, e.Message, e.FinishedAt)
		return err
	}
	return nil
}

func (s *Store) Expire(ctx context.Context) error {
	_, err := s.Pool.Exec(ctx, `UPDATE scenario_runs SET status='uncertain',error_code=CASE WHEN status='pending' THEN 'ack_timeout' ELSE 'completion_timeout' END,message='Sem confirmação do controlador. A conexão ou o sistema pode ter falhado; verifique a estufa. Um novo cenário ficará bloqueado até confirmar a parada.' WHERE (status='pending' AND created_at < now()-interval '20 seconds') OR (status IN ('running','stopping') AND COALESCE(ends_at,created_at) < now()-interval '20 seconds')`)
	return err
}
