package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"fertirriga-backend/internal/domain"
)

// ScheduleRepo gerencia operações de banco para agendamentos.
type ScheduleRepo struct {
	pool *pgxpool.Pool
}

func NewScheduleRepo(pool *pgxpool.Pool) *ScheduleRepo {
	return &ScheduleRepo{pool: pool}
}

func (r *ScheduleRepo) Create(ctx context.Context, s *domain.Schedule) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO schedules (zone_id, recipe_id, name, schedule_type, cron_expression, start_at, start_window_min, duration_sec, origin, is_enabled, version, next_execution_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at
	`, s.ZoneID, s.RecipeID, s.Name, s.ScheduleType, s.CronExpression, s.StartAt, s.StartWindowMin, s.DurationSec, s.Origin, s.IsEnabled, s.Version, s.NextExecutionAt,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
}

func (r *ScheduleRepo) Get(ctx context.Context, id string) (*domain.Schedule, error) {
	var s domain.Schedule
	err := r.pool.QueryRow(ctx, `
		SELECT id, zone_id, recipe_id, name, schedule_type, cron_expression, start_at, start_window_min, duration_sec,
			   origin, is_enabled, version, next_execution_at, created_at, updated_at, deleted_at
		FROM schedules WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&s.ID, &s.ZoneID, &s.RecipeID, &s.Name, &s.ScheduleType, &s.CronExpression, &s.StartAt, &s.StartWindowMin, &s.DurationSec,
		&s.Origin, &s.IsEnabled, &s.Version, &s.NextExecutionAt, &s.CreatedAt, &s.UpdatedAt, &s.DeletedAt)
	if err != nil {
		return nil, fmt.Errorf("schedule %s não encontrado: %w", id, err)
	}
	return &s, nil
}

func (r *ScheduleRepo) ListByZone(ctx context.Context, zoneID string) ([]domain.Schedule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, zone_id, recipe_id, name, schedule_type, cron_expression, start_at, start_window_min, duration_sec,
			   origin, is_enabled, version, next_execution_at, created_at, updated_at, deleted_at
		FROM schedules WHERE zone_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC
	`, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schedules := []domain.Schedule{}
	for rows.Next() {
		var s domain.Schedule
		if err := rows.Scan(&s.ID, &s.ZoneID, &s.RecipeID, &s.Name, &s.ScheduleType, &s.CronExpression, &s.StartAt, &s.StartWindowMin, &s.DurationSec,
			&s.Origin, &s.IsEnabled, &s.Version, &s.NextExecutionAt, &s.CreatedAt, &s.UpdatedAt, &s.DeletedAt); err != nil {
			return nil, err
		}
		schedules = append(schedules, s)
	}
	return schedules, nil
}

func (r *ScheduleRepo) Update(ctx context.Context, s *domain.Schedule) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE schedules SET name = $1, schedule_type = $2, cron_expression = $3, start_at = $4, start_window_min = $5,
			   duration_sec = $6, is_enabled = $7, version = version + 1, next_execution_at = $8, updated_at = NOW()
		WHERE id = $9 AND deleted_at IS NULL
	`, s.Name, s.ScheduleType, s.CronExpression, s.StartAt, s.StartWindowMin, s.DurationSec, s.IsEnabled, s.NextExecutionAt, s.ID)
	return err
}

func (r *ScheduleRepo) SetEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE schedules SET is_enabled = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`, enabled, id)
	return err
}

func (r *ScheduleRepo) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE schedules SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}

// GetDueSchedules retorna schedules que devem ser executados agora.
func (r *ScheduleRepo) GetDueSchedules(ctx context.Context) ([]domain.Schedule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, zone_id, recipe_id, name, schedule_type, cron_expression, start_at, start_window_min, duration_sec,
			   origin, is_enabled, version, next_execution_at, created_at, updated_at, deleted_at
		FROM schedules
		WHERE is_enabled = true AND deleted_at IS NULL AND next_execution_at <= NOW()
		ORDER BY next_execution_at
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schedules := []domain.Schedule{}
	for rows.Next() {
		var s domain.Schedule
		if err := rows.Scan(&s.ID, &s.ZoneID, &s.RecipeID, &s.Name, &s.ScheduleType, &s.CronExpression, &s.StartAt, &s.StartWindowMin, &s.DurationSec,
			&s.Origin, &s.IsEnabled, &s.Version, &s.NextExecutionAt, &s.CreatedAt, &s.UpdatedAt, &s.DeletedAt); err != nil {
			return nil, err
		}
		schedules = append(schedules, s)
	}
	return schedules, nil
}

// CreateRun registra uma execução de agendamento.
func (r *ScheduleRepo) CreateRun(ctx context.Context, run *domain.ScheduleRun) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO schedule_runs (schedule_id, status, started_at) VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, run.ScheduleID, run.Status, run.StartedAt).Scan(&run.ID, &run.CreatedAt)
}

// ListRuns retorna o histórico de execuções de um schedule.
func (r *ScheduleRepo) ListRuns(ctx context.Context, scheduleID string) ([]domain.ScheduleRun, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, schedule_id, status, started_at, finished_at, result, created_at
		FROM schedule_runs WHERE schedule_id = $1 ORDER BY created_at DESC LIMIT 50
	`, scheduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := []domain.ScheduleRun{}
	for rows.Next() {
		var run domain.ScheduleRun
		if err := rows.Scan(&run.ID, &run.ScheduleID, &run.Status, &run.StartedAt, &run.FinishedAt, &run.Result, &run.CreatedAt); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func (r *ScheduleRepo) UpdateRunStatus(ctx context.Context, runID, status string, result []byte) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE schedule_runs SET status = $1, finished_at = NOW(), result = $2 WHERE id = $3
	`, status, result, runID)
	return err
}
