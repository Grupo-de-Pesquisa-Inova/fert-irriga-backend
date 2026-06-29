package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"fertirriga-backend/internal/domain"
)

// CommandRepo gerencia operações de banco para comandos rastreáveis.
type CommandRepo struct {
	pool *pgxpool.Pool
}

func NewCommandRepo(pool *pgxpool.Pool) *CommandRepo {
	return &CommandRepo{pool: pool}
}

func (r *CommandRepo) Create(ctx context.Context, cmd *domain.ManualCommand) error {
	if cmd.CommandID == "" {
		cmd.CommandID = uuid.NewString()
	}
	if cmd.Parameters == nil {
		cmd.Parameters = json.RawMessage(`{}`)
	}
	if cmd.Origin == "" {
		cmd.Origin = "web_manual"
	}
	if cmd.Actor == "" {
		cmd.Actor = "system"
	}
	if cmd.Status == "" {
		cmd.Status = "pending"
	}
	if cmd.Priority == 0 {
		cmd.Priority = 4
	}

	return r.pool.QueryRow(ctx, `
		INSERT INTO manual_commands (command_id, device_id, origin, actor, action, target_channel, parameters, status, safety_context, priority)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, requested_at, created_at
	`, cmd.CommandID, cmd.DeviceID, cmd.Origin, cmd.Actor, cmd.Action, cmd.TargetChannel, cmd.Parameters, cmd.Status, cmd.SafetyContext, cmd.Priority,
	).Scan(&cmd.ID, &cmd.RequestedAt, &cmd.CreatedAt)
}

func (r *CommandRepo) Get(ctx context.Context, id string) (*domain.ManualCommand, error) {
	var cmd domain.ManualCommand
	err := r.pool.QueryRow(ctx, `
		SELECT id, command_id, device_id, origin, actor, action, target_channel, parameters, status, safety_context, priority,
			   requested_at, dispatched_at, acked_at, expired_at, result, created_at
		FROM manual_commands WHERE id = $1
	`, id).Scan(&cmd.ID, &cmd.CommandID, &cmd.DeviceID, &cmd.Origin, &cmd.Actor, &cmd.Action, &cmd.TargetChannel, &cmd.Parameters, &cmd.Status, &cmd.SafetyContext, &cmd.Priority,
		&cmd.RequestedAt, &cmd.DispatchedAt, &cmd.AckedAt, &cmd.ExpiredAt, &cmd.Result, &cmd.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("command %s não encontrado: %w", id, err)
	}
	return &cmd, nil
}

func (r *CommandRepo) GetByCommandID(ctx context.Context, commandID string) (*domain.ManualCommand, error) {
	var cmd domain.ManualCommand
	err := r.pool.QueryRow(ctx, `
		SELECT id, command_id, device_id, origin, actor, action, target_channel, parameters, status, safety_context, priority,
			   requested_at, dispatched_at, acked_at, expired_at, result, created_at
		FROM manual_commands WHERE command_id = $1
	`, commandID).Scan(&cmd.ID, &cmd.CommandID, &cmd.DeviceID, &cmd.Origin, &cmd.Actor, &cmd.Action, &cmd.TargetChannel, &cmd.Parameters, &cmd.Status, &cmd.SafetyContext, &cmd.Priority,
		&cmd.RequestedAt, &cmd.DispatchedAt, &cmd.AckedAt, &cmd.ExpiredAt, &cmd.Result, &cmd.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("command %s não encontrado: %w", commandID, err)
	}
	return &cmd, nil
}

func (r *CommandRepo) List(ctx context.Context, deviceID string, limit int) ([]domain.ManualCommand, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `
		SELECT id, command_id, device_id, origin, actor, action, target_channel, parameters, status, safety_context, priority,
			   requested_at, dispatched_at, acked_at, expired_at, result, created_at
		FROM manual_commands
	`
	var args []interface{}
	if deviceID != "" {
		query += ` WHERE device_id = $1 ORDER BY requested_at DESC LIMIT $2`
		args = append(args, deviceID, limit)
	} else {
		query += ` ORDER BY requested_at DESC LIMIT $1`
		args = append(args, limit)
	}

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cmds := []domain.ManualCommand{}
	for rows.Next() {
		var cmd domain.ManualCommand
		if err := rows.Scan(&cmd.ID, &cmd.CommandID, &cmd.DeviceID, &cmd.Origin, &cmd.Actor, &cmd.Action, &cmd.TargetChannel, &cmd.Parameters, &cmd.Status, &cmd.SafetyContext, &cmd.Priority,
			&cmd.RequestedAt, &cmd.DispatchedAt, &cmd.AckedAt, &cmd.ExpiredAt, &cmd.Result, &cmd.CreatedAt); err != nil {
			return nil, err
		}
		cmds = append(cmds, cmd)
	}
	return cmds, nil
}

func (r *CommandRepo) UpdateStatus(ctx context.Context, commandID, status string) error {
	_, err := r.pool.Exec(ctx, `UPDATE manual_commands SET status = $1 WHERE command_id = $2`, status, commandID)
	return err
}

func (r *CommandRepo) MarkDispatched(ctx context.Context, commandID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE manual_commands SET status = 'dispatched', dispatched_at = NOW() WHERE command_id = $1`, commandID)
	return err
}

func (r *CommandRepo) MarkAcked(ctx context.Context, commandID, status string, result json.RawMessage) error {
	if status == "" {
		status = "executed"
	}
	_, err := r.pool.Exec(ctx, `UPDATE manual_commands SET status = $1, acked_at = NOW(), result = $2 WHERE command_id = $3`, status, result, commandID)
	return err
}
