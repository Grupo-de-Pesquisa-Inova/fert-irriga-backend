package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"fertirriga-backend/internal/domain"
	mqttclient "fertirriga-backend/internal/mqtt"
	"fertirriga-backend/internal/repository"
)

// DeviceService contém a lógica de negócio para operações com dispositivos.
type DeviceService struct {
	deviceRepo    *repository.DeviceRepo
	telemetryRepo *repository.TelemetryRepo
	mqtt          *mqttclient.Client
}

func NewDeviceService(dr *repository.DeviceRepo, tr *repository.TelemetryRepo, mqtt *mqttclient.Client) *DeviceService {
	return &DeviceService{
		deviceRepo:    dr,
		telemetryRepo: tr,
		mqtt:          mqtt,
	}
}

// ProcessTelemetry recebe payload bruto do MQTT, persiste telemetria e atualiza device.
func (s *DeviceService) ProcessTelemetry(ctx context.Context, deviceID string, raw []byte) {
	var payload domain.PayloadESP32
	if err := json.Unmarshal(raw, &payload); err != nil {
		slog.Error("erro ao desserializar telemetria", "device", deviceID, "error", err)
		return
	}

	// Persistir leitura de sensor
	now := time.Now()
	rec := &domain.TelemetryRecord{
		DeviceID:       deviceID,
		TemperaturaC:   payload.StatusSistema.Sensores.Clima.TemperaturaC,
		UmidadePct:     payload.StatusSistema.Sensores.Clima.UmidadePct,
		PressaoHPA:     payload.StatusSistema.Sensores.Clima.PressaoHPA,
		FluxoDetectado: payload.StatusSistema.Sensores.Hidraulica.FluxoDetectado,
		VazaoLPM:       payload.StatusSistema.Sensores.Hidraulica.VazaoLPM,
		SinalWifiDBM:   payload.StatusSistema.Conexao.SinalWifiDBM,
		RecordedAt:     now,
	}
	if err := s.telemetryRepo.Insert(ctx, rec); err != nil {
		slog.Error("erro ao persistir telemetria", "device", deviceID, "error", err)
	}

	// Atualizar payload do device (upsert garante criação automática)
	device := &domain.Device{
		DeviceID:   deviceID,
		Name:       deviceID,
		Payload:    payload,
		IsOnline:   true,
		LastSeenAt: &now,
	}
	if err := s.deviceRepo.Upsert(ctx, device); err != nil {
		slog.Error("erro ao atualizar device", "device", deviceID, "error", err)
	}

	slog.Debug("telemetria processada", "device", deviceID,
		"temp", rec.TemperaturaC, "umid", rec.UmidadePct)
}

// GetDevice retorna o estado atual de um device.
func (s *DeviceService) GetDevice(ctx context.Context, deviceID string) (*domain.Device, error) {
	return s.deviceRepo.GetByDeviceID(ctx, deviceID)
}

// ListDevices retorna todos os devices registrados.
func (s *DeviceService) ListDevices(ctx context.Context) ([]domain.Device, error) {
	return s.deviceRepo.ListAll(ctx)
}

// SendControl envia um comando de controle ao ESP32 via MQTT.
func (s *DeviceService) SendControl(ctx context.Context, deviceID string, control domain.ControlStatus) error {
	payload, err := json.Marshal(control)
	if err != nil {
		return fmt.Errorf("erro ao serializar comando: %w", err)
	}

	// Logar comando para auditoria
	cmdLog := &domain.CommandLog{
		DeviceID:    deviceID,
		CommandType: "controle",
		Payload:     string(payload),
		Status:      "sent",
	}
	if err := s.deviceRepo.LogCommand(ctx, cmdLog); err != nil {
		slog.Warn("erro ao logar comando", "error", err)
	}

	return s.mqtt.PublishCommand(ctx, deviceID, "comando", payload)
}

// EmergencyStop aciona parada de emergência via MQTT com QoS 2.
func (s *DeviceService) EmergencyStop(ctx context.Context, deviceID string) error {
	payload := []byte(`{"parada_emergencia":true}`)

	cmdLog := &domain.CommandLog{
		DeviceID:    deviceID,
		CommandType: "emergencia",
		Payload:     string(payload),
		Status:      "sent",
	}
	if err := s.deviceRepo.LogCommand(ctx, cmdLog); err != nil {
		slog.Warn("erro ao logar emergência", "error", err)
	}

	slog.Warn("PARADA DE EMERGÊNCIA acionada", "device", deviceID)
	return s.mqtt.PublishCommand(ctx, deviceID, "emergencia", payload)
}

// GetTelemetry retorna histórico de telemetria de um device.
func (s *DeviceService) GetTelemetry(ctx context.Context, deviceID string, from, to time.Time, limit int) ([]domain.TelemetryRecord, error) {
	return s.telemetryRepo.GetByDeviceID(ctx, deviceID, from, to, limit)
}

// GetLatestTelemetry retorna as últimas leituras de um device.
func (s *DeviceService) GetLatestTelemetry(ctx context.Context, deviceID string, limit int) ([]domain.TelemetryRecord, error) {
	return s.telemetryRepo.GetLatest(ctx, deviceID, limit)
}

// GetTelemetryPage retorna uma página de telemetria e o total de registros do device.
func (s *DeviceService) GetTelemetryPage(ctx context.Context, deviceID string, limit, offset int) ([]domain.TelemetryRecord, int, error) {
	total, err := s.telemetryRepo.CountByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, 0, err
	}
	records, err := s.telemetryRepo.GetPage(ctx, deviceID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return records, total, nil
}
