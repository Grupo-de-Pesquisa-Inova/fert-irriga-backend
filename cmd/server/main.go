package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fertirriga-backend/internal/config"
	"fertirriga-backend/internal/domain"
	"fertirriga-backend/internal/handler"
	mqttclient "fertirriga-backend/internal/mqtt"
	"fertirriga-backend/internal/repository"
	"fertirriga-backend/internal/safety"
	"fertirriga-backend/internal/scheduler"
	"fertirriga-backend/internal/service"
	"fertirriga-backend/internal/worker"
)

func main() {
	// Structured logging em JSON
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("iniciando FertIrriga Backend")

	// Carregar configuração
	cfg, err := config.Load()
	if err != nil {
		slog.Error("falha ao carregar configuração", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Conectar ao PostgreSQL
	pool, err := repository.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("falha ao conectar ao PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Executar migrações
	if err := repository.RunMigrations(ctx, pool); err != nil {
		slog.Error("falha ao executar migrações", "error", err)
		os.Exit(1)
	}

	// Inicializar repositórios
	deviceRepo := repository.NewDeviceRepo(pool)
	telemetryRepo := repository.NewTelemetryRepo(pool)
	auditRepo := repository.NewAuditRepo(pool)
	scheduleRepo := repository.NewScheduleRepo(pool)
	commandRepo := repository.NewCommandRepo(pool)
	alarmRepo := repository.NewAlarmRepo(pool)

	// Conectar MQTT5
	mqtt, err := mqttclient.NewClient(ctx, cfg.MQTT)
	if err != nil {
		slog.Error("falha ao conectar MQTT", "error", err)
		os.Exit(1)
	}

	// Inicializar serviço
	deviceSvc := service.NewDeviceService(deviceRepo, telemetryRepo, mqtt)

	// Inicializar WebSocket hub
	wsHub := handler.NewWSHub()
	go wsHub.Run()

	// Conectar telemetria MQTT → WebSocket broadcast
	mqtt.OnTelemetry(func(deviceID string, payload []byte) {
		deviceSvc.ProcessTelemetry(ctx, deviceID, payload)
		wsHub.Broadcast(deviceID, payload)
	})

	mqtt.OnEmergency(func(deviceID string, payload []byte) {
		slog.Warn("EMERGÊNCIA recebida via MQTT", "device", deviceID)
		wsHub.Broadcast(deviceID, payload)
		wsHub.BroadcastEvent("emergency", map[string]interface{}{
			"device_id": deviceID,
		})
	})

	// Consumer de ACK — atualiza comando e broadcast para frontend
	mqtt.OnAck(func(deviceID string, payload []byte) {
		var ackData struct {
			CommandID string `json:"command_id"`
			Status    string `json:"status"`
		}
		if err := json.Unmarshal(payload, &ackData); err != nil {
			slog.Warn("[ACK] Payload inválido", "error", err)
			return
		}
		slog.Info("[ACK] Recebido", "device", deviceID, "command", ackData.CommandID, "status", ackData.Status)

		if err := commandRepo.MarkAcked(ctx, ackData.CommandID, payload); err != nil {
			slog.Error("[ACK] Erro ao atualizar comando", "error", err)
			return
		}

		auditRepo.AuditLog(ctx, "command_acked", "device:"+deviceID, "command", ackData.CommandID, ackData)

		wsHub.BroadcastEvent("command_status", map[string]interface{}{
			"command_id": ackData.CommandID,
			"status":     "executed",
			"device_id":  deviceID,
		})
	})

	// Inicializar scheduler
	sched := scheduler.New(deviceSvc)
	sched.Start()
	defer sched.Stop()

	// Inicializar safety engine
	safetyEngine := safety.NewEngine(deviceRepo, commandRepo)

	// Inicializar workers
	schedWorker := worker.NewSchedulerWorker(scheduleRepo, auditRepo)
	schedWorker.Start()
	defer schedWorker.Stop()

	alarmEval := worker.NewAlarmEvaluator(alarmRepo, deviceRepo, telemetryRepo, auditRepo)
	alarmEval.OnAlarm(func(eventType string, rule domain.AlarmRule, deviceID string) {
		wsHub.BroadcastEvent(eventType, map[string]interface{}{
			"rule_name": rule.Name,
			"severity":  rule.Severity,
			"device_id": deviceID,
			"condition": rule.ConditionType,
		})
	})
	alarmEval.Start()
	defer alarmEval.Stop()

	healthWorker := worker.NewDeviceHealthWorker(deviceRepo, auditRepo)
	healthWorker.Start()
	defer healthWorker.Stop()

	// Inicializar handlers
	scheduleHandler := handler.NewScheduleHandler(scheduleRepo, auditRepo)
	commandHandler := handler.NewCommandHandler(commandRepo, auditRepo, safetyEngine, mqtt, wsHub)
	alarmHandler := handler.NewAlarmHandler(alarmRepo, auditRepo)
	auditHandler := handler.NewAuditHandler(auditRepo)

	// Criar router HTTP
	router := handler.NewRouter(cfg, deviceSvc, wsHub, scheduleHandler, commandHandler, alarmHandler, auditHandler)

	// Iniciar servidor HTTP
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("servidor HTTP iniciado", "port", cfg.Port, "cors", cfg.CORSOrigins)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("falha no servidor HTTP", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("encerrando servidor...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	mqtt.Disconnect(shutdownCtx)
	srv.Shutdown(shutdownCtx)

	slog.Info("FertIrriga Backend encerrado com sucesso")
}
