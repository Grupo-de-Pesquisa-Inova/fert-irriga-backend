package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"fertirriga-backend/internal/domain"
	"fertirriga-backend/internal/repository"
)

// AlarmCallback é chamado quando um alarme dispara ou resolve.
type AlarmCallback func(eventType string, rule domain.AlarmRule, deviceID string)

// AlarmEvaluator avalia regras de alarme contra telemetria real.
type AlarmEvaluator struct {
	alarmRepo     *repository.AlarmRepo
	deviceRepo    *repository.DeviceRepo
	telemetryRepo *repository.TelemetryRepo
	auditRepo     *repository.AuditRepo
	interval      time.Duration
	stopCh        chan struct{}
	onAlarm       AlarmCallback
}

func NewAlarmEvaluator(alarmRepo *repository.AlarmRepo, deviceRepo *repository.DeviceRepo, telemetryRepo *repository.TelemetryRepo, auditRepo *repository.AuditRepo) *AlarmEvaluator {
	return &AlarmEvaluator{
		alarmRepo:     alarmRepo,
		deviceRepo:    deviceRepo,
		telemetryRepo: telemetryRepo,
		auditRepo:     auditRepo,
		interval:      5 * time.Second,
		stopCh:        make(chan struct{}),
	}
}

// OnAlarm registra callback para alarmes disparados/resolvidos.
func (e *AlarmEvaluator) OnAlarm(cb AlarmCallback) {
	e.onAlarm = cb
}

func (e *AlarmEvaluator) Start() {
	slog.Info("[AlarmEvaluator] Iniciado", "interval", e.interval)
	go e.loop()
}

func (e *AlarmEvaluator) Stop() {
	close(e.stopCh)
	slog.Info("[AlarmEvaluator] Parado")
}

func (e *AlarmEvaluator) loop() {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.evaluate()
		}
	}
}

func (e *AlarmEvaluator) evaluate() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	rules, err := e.alarmRepo.GetEnabledRules(ctx)
	if err != nil {
		slog.Error("[AlarmEvaluator] Erro ao buscar regras", "error", err)
		return
	}

	devices, err := e.deviceRepo.ListAll(ctx)
	if err != nil {
		slog.Error("[AlarmEvaluator] Erro ao listar devices", "error", err)
		return
	}

	for _, rule := range rules {
		for _, device := range devices {
			triggered := e.evaluateRule(rule, device)

			hasActive, _ := e.alarmRepo.HasActiveEventForRule(ctx, rule.ID)

			if triggered && !hasActive {
				// Disparar novo evento
				contextData, _ := json.Marshal(map[string]interface{}{
					"device_id":   device.DeviceID,
					"channel_key": rule.ChannelKey,
					"threshold":   rule.ThresholdValue,
					"condition":   rule.ConditionType,
				})

				event := &domain.AlarmEvent{
					AlarmRuleID: rule.ID,
					DeviceID:    &device.DeviceID,
					Status:      "active",
					Context:     contextData,
				}

				if err := e.alarmRepo.CreateEvent(ctx, event); err != nil {
					slog.Error("[AlarmEvaluator] Erro ao criar evento", "rule", rule.Name, "error", err)
					continue
				}

				slog.Warn("[AlarmEvaluator] ALARME DISPARADO",
					"rule", rule.Name,
					"severity", rule.Severity,
					"device", device.DeviceID,
				)

				e.auditRepo.AuditLog(ctx, "alarm_triggered", "alarm_evaluator", "alarm_rule", rule.ID, map[string]interface{}{
					"device": device.DeviceID,
					"rule":   rule.Name,
				})

				if e.onAlarm != nil {
					e.onAlarm("alarm_triggered", rule, device.DeviceID)
				}

			} else if !triggered && hasActive {
				// Resolver evento ativo
				activeEvents, _ := e.alarmRepo.ListActiveEvents(ctx)
				for _, ev := range activeEvents {
					if ev.AlarmRuleID == rule.ID && ev.DeviceID != nil && *ev.DeviceID == device.DeviceID {
						e.alarmRepo.ResolveEvent(ctx, ev.ID)
						slog.Info("[AlarmEvaluator] Alarme resolvido",
							"rule", rule.Name,
							"device", device.DeviceID,
						)
						e.auditRepo.AuditLog(ctx, "alarm_resolved", "alarm_evaluator", "alarm_rule", rule.ID, map[string]interface{}{
							"device": device.DeviceID,
						})

						if e.onAlarm != nil {
							e.onAlarm("alarm_resolved", rule, device.DeviceID)
						}
					}
				}
			}
		}
	}
}

// evaluateRule avalia uma regra contra o payload de um device.
func (e *AlarmEvaluator) evaluateRule(rule domain.AlarmRule, device domain.Device) bool {
	p := device.Payload

	switch rule.ConditionType {
	case "threshold_high":
		val := e.getChannelValue(rule.ChannelKey, p)
		if val != nil && rule.ThresholdValue != nil {
			return *val > *rule.ThresholdValue
		}

	case "threshold_low":
		val := e.getChannelValue(rule.ChannelKey, p)
		if val != nil && rule.ThresholdValue != nil {
			return *val < *rule.ThresholdValue
		}

	case "comm_timeout":
		if !device.IsOnline {
			return true
		}
		if device.LastSeenAt != nil {
			return time.Since(*device.LastSeenAt) > 60*time.Second
		}
		return true

	case "flow_fail":
		// Disparar se há irrigação ativa mas sem fluxo detectado
		operando := p.StatusSistema.Operacao.ModoAtual != "stand-by"
		semFluxo := !p.StatusSistema.Sensores.Hidraulica.FluxoDetectado
		return operando && semFluxo

	case "emergency":
		return p.Seguranca.ParadaEmergencia
	}

	return false
}

// getChannelValue extrai o valor numérico de um channel_key do payload.
func (e *AlarmEvaluator) getChannelValue(key *string, p domain.PayloadESP32) *float64 {
	if key == nil {
		return nil
	}
	var val float64
	switch *key {
	case "temperatura_c":
		val = p.StatusSistema.Sensores.Clima.TemperaturaC
	case "umidade_pct":
		val = p.StatusSistema.Sensores.Clima.UmidadePct
	case "pressao_hpa":
		val = p.StatusSistema.Sensores.Clima.PressaoHPA
	case "vazao_lpm":
		val = p.StatusSistema.Sensores.Hidraulica.VazaoLPM
	case "sinal_wifi_dbm":
		val = float64(p.StatusSistema.Conexao.SinalWifiDBM)
	default:
		return nil
	}
	return &val
}
