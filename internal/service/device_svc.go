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

	// Horário da leitura: usa o timestamp do ESP (RTC) quando presente, para
	// preservar o momento real de coleta em telemetria atrasada (offline).
	now := time.Now()
	recordedAt := now
	if payload.Ts > 0 {
		recordedAt = time.Unix(payload.Ts, 0).UTC()
	}

	// Telemetria é considerada "histórica" (parte de um flush da fila offline)
	// quando o timestamp é bem mais antigo que agora. Nesse caso só gravamos o
	// ponto no histórico — NÃO sobrescrevemos o estado atual do device com um
	// payload velho.
	isHistorical := payload.Ts > 0 && now.Sub(recordedAt) > 60*time.Second

	// Persistir leitura de sensor
	rec := &domain.TelemetryRecord{
		DeviceID:       deviceID,
		TemperaturaC:   payload.StatusSistema.Sensores.Clima.TemperaturaC,
		UmidadePct:     payload.StatusSistema.Sensores.Clima.UmidadePct,
		PressaoHPA:     payload.StatusSistema.Sensores.Clima.PressaoHPA,
		FluxoDetectado: payload.StatusSistema.Sensores.Hidraulica.FluxoDetectado,
		VazaoLPM:       payload.StatusSistema.Sensores.Hidraulica.VazaoLPM,
		SinalWifiDBM:   payload.StatusSistema.Conexao.SinalWifiDBM,
		RecordedAt:     recordedAt,
	}
	if err := s.telemetryRepo.Insert(ctx, rec); err != nil {
		slog.Error("erro ao persistir telemetria", "device", deviceID, "error", err)
	}

	if isHistorical {
		slog.Debug("telemetria histórica ingerida (sem sobrescrever estado)", "device", deviceID, "recorded_at", recordedAt)
		return
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

// ProcessStatus trata a mensagem de estado de conexão publicada pelo ESP32
// (`{"estado":"online"|"offline"}`). Atualiza apenas o status do device, sem
// gravar telemetria nem sobrescrever o payload de sensores.
func (s *DeviceService) ProcessStatus(ctx context.Context, deviceID string, raw []byte) {
	var msg struct {
		Estado string `json:"estado"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		slog.Warn("status MQTT inválido", "device", deviceID, "error", err)
		return
	}

	online := msg.Estado == "online"
	if err := s.deviceRepo.SetOnlineStatus(ctx, deviceID, online); err != nil {
		slog.Error("erro ao atualizar status do device", "device", deviceID, "error", err)
		return
	}
	slog.Info("status do device atualizado via MQTT", "device", deviceID, "estado", msg.Estado)
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

// SyncSchedules envia a lista de agendamentos para o ESP32 executar LOCALMENTE
// (via RTC), tornando a programação resiliente a quedas de internet. O payload
// é publicado de forma retida, então o ESP recebe a versão atual ao reconectar.
func (s *DeviceService) SyncSchedules(ctx context.Context, deviceID string, schedules []domain.Schedule) error {
	// O ESP trabalha em horário local (-3). Convertemos o StartAt para extrair HH:MM.
	loc := time.FixedZone("BRT", -3*3600)

	type espSchedule struct {
		ID  string `json:"id"`
		Out int    `json:"out"` // índice da saída no ESP (0..5)
		H   int    `json:"h"`
		M   int    `json:"m"`
		Dur int    `json:"dur"` // minutos
		En  bool   `json:"en"`
		Rec bool   `json:"rec"`
	}

	list := make([]espSchedule, 0, len(schedules))
	for _, sc := range schedules {
		if sc.StartAt == nil {
			continue // sem horário não dá para agendar localmente
		}
		t := sc.StartAt.In(loc)
		// valve_number (1..4 na UI) -> índice de saída do ESP (0..5).
		out := sc.ValveNumber - 1
		if out < 0 {
			out = 0
		}
		if out > 5 {
			out = 5
		}
		list = append(list, espSchedule{
			ID:  sc.ID,
			Out: out,
			H:   t.Hour(),
			M:   t.Minute(),
			Dur: sc.DurationSec / 60,
			En:  sc.IsEnabled,
			Rec: sc.ScheduleType == "recurring",
		})
	}

	payload, err := json.Marshal(map[string]interface{}{"schedules": list})
	if err != nil {
		return fmt.Errorf("erro ao serializar agendamentos: %w", err)
	}
	slog.Info("sincronizando agendamentos com o ESP32", "device", deviceID, "qtd", len(list))
	return s.mqtt.PublishCommand(ctx, deviceID, "agendamento", payload)
}

// SetEmergency altera a parada de emergência via MQTT com QoS 2.
func (s *DeviceService) SetEmergency(ctx context.Context, deviceID string, active bool) error {
	payload, err := json.Marshal(map[string]bool{"parada_emergencia": active})
	if err != nil {
		return fmt.Errorf("erro ao serializar emergência: %w", err)
	}

	cmdLog := &domain.CommandLog{
		DeviceID:    deviceID,
		CommandType: "emergencia",
		Payload:     string(payload),
		Status:      "sent",
	}
	if err := s.deviceRepo.LogCommand(ctx, cmdLog); err != nil {
		slog.Warn("erro ao logar emergência", "error", err)
	}

	if active {
		slog.Warn("PARADA DE EMERGÊNCIA acionada", "device", deviceID)
	} else {
		slog.Warn("PARADA DE EMERGÊNCIA resetada", "device", deviceID)
	}
	return s.mqtt.PublishCommand(ctx, deviceID, "emergencia", payload)
}

// EmergencyStop aciona parada de emergência via MQTT com QoS 2.
func (s *DeviceService) EmergencyStop(ctx context.Context, deviceID string) error {
	return s.SetEmergency(ctx, deviceID, true)
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
