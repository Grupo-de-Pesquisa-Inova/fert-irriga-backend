package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"fertirriga-backend/internal/domain"
)

// TelemetryRepo gerencia persistência de dados time-series de sensores.
type TelemetryRepo struct {
	pool *pgxpool.Pool
}

func NewTelemetryRepo(pool *pgxpool.Pool) *TelemetryRepo {
	return &TelemetryRepo{pool: pool}
}

// Insert persiste um ponto de telemetria.
//
// ON CONFLICT DO NOTHING torna a ingestão idempotente: ao reenviar a fila
// offline do ESP32 (store-and-forward), registros duplicados — mesma
// (device_id, recorded_at) — são ignorados em vez de duplicar dados.
func (r *TelemetryRepo) Insert(ctx context.Context, rec *domain.TelemetryRecord) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO telemetry (device_id, temperatura_c, umidade_pct, pressao_hpa,
			fluxo_detectado, vazao_lpm, sinal_wifi_dbm, recorded_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (device_id, recorded_at) DO NOTHING
	`, rec.DeviceID, rec.TemperaturaC, rec.UmidadePct, rec.PressaoHPA,
		rec.FluxoDetectado, rec.VazaoLPM, rec.SinalWifiDBM, rec.RecordedAt)

	if err != nil {
		return fmt.Errorf("erro ao inserir telemetria: %w", err)
	}
	return nil
}

// GetByDeviceID retorna telemetria de um device em um intervalo de tempo.
func (r *TelemetryRepo) GetByDeviceID(ctx context.Context, deviceID string, from, to time.Time, limit int) ([]domain.TelemetryRecord, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, device_id, temperatura_c, umidade_pct, pressao_hpa,
			fluxo_detectado, vazao_lpm, sinal_wifi_dbm, recorded_at
		FROM telemetry
		WHERE device_id = $1 AND recorded_at BETWEEN $2 AND $3
		ORDER BY recorded_at DESC
		LIMIT $4
	`, deviceID, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar telemetria: %w", err)
	}
	defer rows.Close()

	records := []domain.TelemetryRecord{}
	for rows.Next() {
		var rec domain.TelemetryRecord
		if err := rows.Scan(
			&rec.ID, &rec.DeviceID, &rec.TemperaturaC, &rec.UmidadePct,
			&rec.PressaoHPA, &rec.FluxoDetectado, &rec.VazaoLPM,
			&rec.SinalWifiDBM, &rec.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("erro ao escanear telemetria: %w", err)
		}
		records = append(records, rec)
	}

	return records, nil
}

// CountByDeviceID retorna o total de registros de telemetria de um device.
func (r *TelemetryRepo) CountByDeviceID(ctx context.Context, deviceID string) (int, error) {
	var total int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM telemetry WHERE device_id = $1`, deviceID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("erro ao contar telemetria: %w", err)
	}
	return total, nil
}

// GetPage retorna uma página de telemetria (mais recentes primeiro) com limit/offset.
func (r *TelemetryRepo) GetPage(ctx context.Context, deviceID string, limit, offset int) ([]domain.TelemetryRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, device_id, temperatura_c, umidade_pct, pressao_hpa,
			fluxo_detectado, vazao_lpm, sinal_wifi_dbm, recorded_at
		FROM telemetry
		WHERE device_id = $1
		ORDER BY recorded_at DESC
		LIMIT $2 OFFSET $3
	`, deviceID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar página de telemetria: %w", err)
	}
	defer rows.Close()

	records := []domain.TelemetryRecord{}
	for rows.Next() {
		var rec domain.TelemetryRecord
		if err := rows.Scan(
			&rec.ID, &rec.DeviceID, &rec.TemperaturaC, &rec.UmidadePct,
			&rec.PressaoHPA, &rec.FluxoDetectado, &rec.VazaoLPM,
			&rec.SinalWifiDBM, &rec.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("erro ao escanear telemetria: %w", err)
		}
		records = append(records, rec)
	}

	return records, nil
}

// GetLatest retorna os últimos N registros de um device.
func (r *TelemetryRepo) GetLatest(ctx context.Context, deviceID string, limit int) ([]domain.TelemetryRecord, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	rows, err := r.pool.Query(ctx, `
		SELECT id, device_id, temperatura_c, umidade_pct, pressao_hpa,
			fluxo_detectado, vazao_lpm, sinal_wifi_dbm, recorded_at
		FROM telemetry
		WHERE device_id = $1
		ORDER BY recorded_at DESC
		LIMIT $2
	`, deviceID, limit)
	if err != nil {
		return nil, fmt.Errorf("erro ao consultar telemetria recente: %w", err)
	}
	defer rows.Close()

	records := []domain.TelemetryRecord{}
	for rows.Next() {
		var rec domain.TelemetryRecord
		if err := rows.Scan(
			&rec.ID, &rec.DeviceID, &rec.TemperaturaC, &rec.UmidadePct,
			&rec.PressaoHPA, &rec.FluxoDetectado, &rec.VazaoLPM,
			&rec.SinalWifiDBM, &rec.RecordedAt,
		); err != nil {
			return nil, fmt.Errorf("erro ao escanear telemetria: %w", err)
		}
		records = append(records, rec)
	}

	return records, nil
}
