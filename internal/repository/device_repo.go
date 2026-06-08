package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"fertirriga-backend/internal/domain"
)

// DeviceRepo gerencia operações de banco de dados para dispositivos ESP32.
type DeviceRepo struct {
	pool *pgxpool.Pool
}

func NewDeviceRepo(pool *pgxpool.Pool) *DeviceRepo {
	return &DeviceRepo{pool: pool}
}

// Upsert cria ou atualiza um dispositivo. Usado quando o ESP32 envia telemetria pela primeira vez.
func (r *DeviceRepo) Upsert(ctx context.Context, device *domain.Device) error {
	payloadJSON, err := json.Marshal(device.Payload)
	if err != nil {
		return fmt.Errorf("erro ao serializar payload: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO devices (device_id, name, payload, is_online, last_seen_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW())
		ON CONFLICT (device_id) DO UPDATE SET
			payload = $3,
			is_online = $4,
			last_seen_at = $5,
			updated_at = NOW()
	`, device.DeviceID, device.Name, payloadJSON, device.IsOnline, device.LastSeenAt)

	if err != nil {
		return fmt.Errorf("erro ao upsert device %s: %w", device.DeviceID, err)
	}

	return nil
}

// GetByDeviceID busca um dispositivo pelo seu identificador MQTT.
func (r *DeviceRepo) GetByDeviceID(ctx context.Context, deviceID string) (*domain.Device, error) {
	var d domain.Device
	var payloadJSON []byte

	err := r.pool.QueryRow(ctx, `
		SELECT id, device_id, name, payload, is_online, last_seen_at, created_at, updated_at
		FROM devices WHERE device_id = $1
	`, deviceID).Scan(
		&d.ID, &d.DeviceID, &d.Name, &payloadJSON,
		&d.IsOnline, &d.LastSeenAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("device %s não encontrado: %w", deviceID, err)
	}

	if err := json.Unmarshal(payloadJSON, &d.Payload); err != nil {
		return nil, fmt.Errorf("erro ao desserializar payload: %w", err)
	}

	return &d, nil
}

// ListAll retorna todos os dispositivos registrados.
func (r *DeviceRepo) ListAll(ctx context.Context) ([]domain.Device, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, device_id, name, payload, is_online, last_seen_at, created_at, updated_at
		FROM devices ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar devices: %w", err)
	}
	defer rows.Close()

	devices := []domain.Device{}
	for rows.Next() {
		var d domain.Device
		var payloadJSON []byte
		if err := rows.Scan(
			&d.ID, &d.DeviceID, &d.Name, &payloadJSON,
			&d.IsOnline, &d.LastSeenAt, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("erro ao escanear device: %w", err)
		}
		if err := json.Unmarshal(payloadJSON, &d.Payload); err != nil {
			return nil, fmt.Errorf("erro ao desserializar payload: %w", err)
		}
		devices = append(devices, d)
	}

	return devices, nil
}

// UpdatePayload atualiza apenas o payload JSON e marca como online.
func (r *DeviceRepo) UpdatePayload(ctx context.Context, deviceID string, payload domain.PayloadESP32) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("erro ao serializar payload: %w", err)
	}

	now := time.Now()
	_, err = r.pool.Exec(ctx, `
		UPDATE devices
		SET payload = $1, is_online = true, last_seen_at = $2, updated_at = $2
		WHERE device_id = $3
	`, payloadJSON, now, deviceID)

	return err
}

// LogCommand registra um comando enviado ao ESP32.
func (r *DeviceRepo) LogCommand(ctx context.Context, log *domain.CommandLog) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO command_log (device_id, command_type, payload, status)
		VALUES ($1, $2, $3, $4)
	`, log.DeviceID, log.CommandType, log.Payload, log.Status)

	return err
}

// Create registra um novo device manualmente via API.
func (r *DeviceRepo) Create(ctx context.Context, deviceID, name, deviceType, zoneID string) (*domain.Device, error) {
	var d domain.Device
	var payloadJSON []byte

	query := `
		INSERT INTO devices (device_id, name, device_type, zone_id, payload, is_online)
		VALUES ($1, $2, $3, $4, '{}', false)
		RETURNING id, device_id, name, payload, is_online, last_seen_at, created_at, updated_at
	`
	var zonePtr *string
	if zoneID != "" {
		zonePtr = &zoneID
	}

	err := r.pool.QueryRow(ctx, query, deviceID, name, deviceType, zonePtr).Scan(
		&d.ID, &d.DeviceID, &d.Name, &payloadJSON,
		&d.IsOnline, &d.LastSeenAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar device %s: %w", deviceID, err)
	}

	if payloadJSON != nil {
		json.Unmarshal(payloadJSON, &d.Payload)
	}
	return &d, nil
}

// SetOnlineStatus atualiza o status online/offline de um device.
// Mantém o campo autodeclarado payload.status_sistema.conexao.estado consistente
// com is_online, evitando que o frontend leia um "online" congelado.
func (r *DeviceRepo) SetOnlineStatus(ctx context.Context, deviceID string, isOnline bool) error {
	estado := "offline"
	if isOnline {
		estado = "online"
	}
	_, err := r.pool.Exec(ctx, `
		UPDATE devices
		SET is_online = $1,
		    payload = jsonb_set(payload, '{status_sistema,conexao,estado}', to_jsonb($2::text), true),
		    updated_at = NOW()
		WHERE device_id = $3
	`, isOnline, estado, deviceID)
	return err
}

// Delete soft-deleta um device.
func (r *DeviceRepo) Delete(ctx context.Context, deviceID string) error {
	_, err := r.pool.Exec(ctx, `UPDATE devices SET deleted_at = NOW() WHERE device_id = $1 AND deleted_at IS NULL`, deviceID)
	return err
}
