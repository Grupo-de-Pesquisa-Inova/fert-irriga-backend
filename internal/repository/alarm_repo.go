package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"fertirriga-backend/internal/domain"
)

// AlarmRepo gerencia operações de banco para alarmes.
type AlarmRepo struct {
	pool *pgxpool.Pool
}

func NewAlarmRepo(pool *pgxpool.Pool) *AlarmRepo {
	return &AlarmRepo{pool: pool}
}

// ═══════════════════════════════════════════════════
// ALARM RULES
// ═══════════════════════════════════════════════════

func (r *AlarmRepo) CreateRule(ctx context.Context, rule *domain.AlarmRule) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO alarm_rules (zone_id, name, condition_type, channel_key, threshold_value, duration_sec, severity, is_enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`, rule.ZoneID, rule.Name, rule.ConditionType, rule.ChannelKey, rule.ThresholdValue, rule.DurationSec, rule.Severity, rule.IsEnabled,
	).Scan(&rule.ID, &rule.CreatedAt, &rule.UpdatedAt)
}

func (r *AlarmRepo) GetRule(ctx context.Context, id string) (*domain.AlarmRule, error) {
	var rule domain.AlarmRule
	err := r.pool.QueryRow(ctx, `
		SELECT id, zone_id, name, condition_type, channel_key, threshold_value, duration_sec, severity, is_enabled, created_at, updated_at
		FROM alarm_rules WHERE id = $1
	`, id).Scan(&rule.ID, &rule.ZoneID, &rule.Name, &rule.ConditionType, &rule.ChannelKey, &rule.ThresholdValue, &rule.DurationSec, &rule.Severity, &rule.IsEnabled, &rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("alarm rule %s não encontrada: %w", id, err)
	}
	return &rule, nil
}

func (r *AlarmRepo) ListRules(ctx context.Context) ([]domain.AlarmRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, zone_id, name, condition_type, channel_key, threshold_value, duration_sec, severity, is_enabled, created_at, updated_at
		FROM alarm_rules ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := []domain.AlarmRule{}
	for rows.Next() {
		var rule domain.AlarmRule
		if err := rows.Scan(&rule.ID, &rule.ZoneID, &rule.Name, &rule.ConditionType, &rule.ChannelKey, &rule.ThresholdValue, &rule.DurationSec, &rule.Severity, &rule.IsEnabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

func (r *AlarmRepo) UpdateRule(ctx context.Context, rule *domain.AlarmRule) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE alarm_rules SET name = $1, condition_type = $2, channel_key = $3, threshold_value = $4, duration_sec = $5, severity = $6, is_enabled = $7, updated_at = NOW()
		WHERE id = $8
	`, rule.Name, rule.ConditionType, rule.ChannelKey, rule.ThresholdValue, rule.DurationSec, rule.Severity, rule.IsEnabled, rule.ID)
	return err
}

func (r *AlarmRepo) DeleteRule(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM alarm_rules WHERE id = $1`, id)
	return err
}

// GetEnabledRules retorna as regras habilitadas para o alarm worker.
func (r *AlarmRepo) GetEnabledRules(ctx context.Context) ([]domain.AlarmRule, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, zone_id, name, condition_type, channel_key, threshold_value, duration_sec, severity, is_enabled, created_at, updated_at
		FROM alarm_rules WHERE is_enabled = true
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	rules := []domain.AlarmRule{}
	for rows.Next() {
		var rule domain.AlarmRule
		if err := rows.Scan(&rule.ID, &rule.ZoneID, &rule.Name, &rule.ConditionType, &rule.ChannelKey, &rule.ThresholdValue, &rule.DurationSec, &rule.Severity, &rule.IsEnabled, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, rule)
	}
	return rules, nil
}

// ═══════════════════════════════════════════════════
// ALARM EVENTS
// ═══════════════════════════════════════════════════

func (r *AlarmRepo) CreateEvent(ctx context.Context, event *domain.AlarmEvent) error {
	if event.Context == nil {
		event.Context = json.RawMessage(`{}`)
	}
	return r.pool.QueryRow(ctx, `
		INSERT INTO alarm_events (alarm_rule_id, device_id, status, context)
		VALUES ($1, $2, $3, $4)
		RETURNING id, triggered_at, created_at
	`, event.AlarmRuleID, event.DeviceID, event.Status, event.Context,
	).Scan(&event.ID, &event.TriggeredAt, &event.CreatedAt)
}

func (r *AlarmRepo) ListActiveEvents(ctx context.Context) ([]domain.AlarmEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT ae.id, ae.alarm_rule_id, ae.device_id, ae.status, ae.triggered_at, ae.acknowledged_at, ae.acknowledged_by, ae.resolved_at, ae.context, ae.created_at,
			   ar.name, ar.severity
		FROM alarm_events ae
		JOIN alarm_rules ar ON ar.id = ae.alarm_rule_id
		WHERE ae.status = 'active'
		ORDER BY ae.triggered_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []domain.AlarmEvent{}
	for rows.Next() {
		var e domain.AlarmEvent
		if err := rows.Scan(&e.ID, &e.AlarmRuleID, &e.DeviceID, &e.Status, &e.TriggeredAt, &e.AcknowledgedAt, &e.AcknowledgedBy, &e.ResolvedAt, &e.Context, &e.CreatedAt,
			&e.RuleName, &e.RuleSeverity); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *AlarmRepo) ListEventHistory(ctx context.Context, limit int) ([]domain.AlarmEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT ae.id, ae.alarm_rule_id, ae.device_id, ae.status, ae.triggered_at, ae.acknowledged_at, ae.acknowledged_by, ae.resolved_at, ae.context, ae.created_at,
			   ar.name, ar.severity
		FROM alarm_events ae
		JOIN alarm_rules ar ON ar.id = ae.alarm_rule_id
		ORDER BY ae.triggered_at DESC LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []domain.AlarmEvent{}
	for rows.Next() {
		var e domain.AlarmEvent
		if err := rows.Scan(&e.ID, &e.AlarmRuleID, &e.DeviceID, &e.Status, &e.TriggeredAt, &e.AcknowledgedAt, &e.AcknowledgedBy, &e.ResolvedAt, &e.Context, &e.CreatedAt,
			&e.RuleName, &e.RuleSeverity); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
}

func (r *AlarmRepo) AcknowledgeEvent(ctx context.Context, id string, userID string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE alarm_events SET status = 'acknowledged', acknowledged_at = NOW(), acknowledged_by = $1
		WHERE id = $2 AND status = 'active'
	`, userID, id)
	return err
}

func (r *AlarmRepo) ResolveEvent(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `UPDATE alarm_events SET status = 'resolved', resolved_at = NOW() WHERE id = $1`, id)
	return err
}

// HasActiveEventForRule verifica se já existe um evento ativo para uma regra.
func (r *AlarmRepo) HasActiveEventForRule(ctx context.Context, ruleID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM alarm_events WHERE alarm_rule_id = $1 AND status = 'active')`, ruleID).Scan(&exists)
	return exists, err
}
