package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"fertirriga-backend/internal/domain"
)

// AuditRepo gerencia operações de banco para eventos de auditoria.
type AuditRepo struct {
	pool *pgxpool.Pool
}

func NewAuditRepo(pool *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{pool: pool}
}

func (r *AuditRepo) InsertAuditEvent(ctx context.Context, e *domain.AuditEvent) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audit_events (event_type, actor, target_type, target_id, payload)
		VALUES ($1, $2, $3, $4, $5)
	`, e.EventType, e.Actor, e.TargetType, e.TargetID, e.Payload)
	return err
}

func (r *AuditRepo) ListAuditEvents(ctx context.Context, limit int, offset int) ([]domain.AuditEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_type, actor, target_type, target_id, payload, timestamp
		FROM audit_events ORDER BY timestamp DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []domain.AuditEvent{}
	for rows.Next() {
		var e domain.AuditEvent
		if err := rows.Scan(&e.ID, &e.EventType, &e.Actor, &e.TargetType, &e.TargetID, &e.Payload, &e.Timestamp); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

// AuditLog é helper para registrar um evento de auditoria com facilidade.
func (r *AuditRepo) AuditLog(ctx context.Context, eventType, actor, targetType, targetID string, payload interface{}) {
	data, _ := json.Marshal(payload)
	_ = r.InsertAuditEvent(ctx, &domain.AuditEvent{
		EventType:  eventType,
		Actor:      actor,
		TargetType: targetType,
		TargetID:   targetID,
		Payload:    data,
		Timestamp:  time.Now(),
	})
}
