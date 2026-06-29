package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"fertirriga-backend/internal/config"
	"fertirriga-backend/internal/service"
)

// NewRouter cria o router Chi com todos os endpoints e middlewares.
func NewRouter(
	cfg *config.Config,
	deviceSvc *service.DeviceService,
	wsHub *WSHub,
	scheduleHandler *ScheduleHandler,
	commandHandler *CommandHandler,
	alarmHandler *AlarmHandler,
	auditHandler *AuditHandler,
) http.Handler {
	r := chi.NewRouter()

	// Middlewares globais
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Heartbeat("/ping"))
	r.Use(middleware.Heartbeat("/healthz"))

	// CORS para o frontend
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Handlers
	deviceHandler := NewDeviceHandler(deviceSvc)

	// Rotas da API v1
	r.Route("/api/v1", func(r chi.Router) {
		// Health check
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok","service":"fertirriga-backend"}`))
		})

		// System status
		r.Get("/system/status", func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusOK, map[string]string{
				"status":    "ok",
				"db":        "connected",
				"mqtt":      "connected",
				"scheduler": "running",
			})
		})

		// Devices
		r.Route("/devices", func(r chi.Router) {
			r.Get("/", deviceHandler.ListDevices)
			r.Get("/{deviceID}", deviceHandler.GetDevice)
			r.Post("/{deviceID}/control", deviceHandler.SendControl)
			r.Post("/{deviceID}/emergency-stop", deviceHandler.EmergencyStop)
			r.Post("/{deviceID}/emergency-reset", deviceHandler.EmergencyReset)
			r.Get("/{deviceID}/telemetry", deviceHandler.GetTelemetry)
			r.Get("/{deviceID}/telemetry/latest", deviceHandler.GetLatestTelemetry)
			r.Get("/{deviceID}/telemetry/history", deviceHandler.GetTelemetryHistory)
		})

		// Commands
		r.Route("/commands", func(r chi.Router) {
			r.Post("/", commandHandler.Create)
			r.Get("/", commandHandler.List)
			r.Get("/{id}", commandHandler.Get)
		})

		// Schedules
		r.Route("/schedules", func(r chi.Router) {
			r.Post("/", scheduleHandler.Create)
			r.Get("/", scheduleHandler.List)
			r.Get("/{id}", scheduleHandler.Get)
			r.Put("/{id}", scheduleHandler.Update)
			r.Delete("/{id}", scheduleHandler.Delete)
			r.Post("/{id}/enable", scheduleHandler.Enable)
			r.Post("/{id}/disable", scheduleHandler.Disable)
			r.Get("/{id}/runs", scheduleHandler.ListRuns)
		})

		// Alarms
		r.Route("/alarms", func(r chi.Router) {
			r.Get("/", alarmHandler.ListActiveAlarms)
			r.Get("/history", alarmHandler.ListAlarmHistory)
			r.Post("/{id}/ack", alarmHandler.AcknowledgeAlarm)
		})

		// Alarm Rules
		r.Route("/alarm-rules", func(r chi.Router) {
			r.Post("/", alarmHandler.CreateRule)
			r.Get("/", alarmHandler.ListRules)
			r.Get("/{id}", alarmHandler.GetRule)
			r.Put("/{id}", alarmHandler.UpdateRule)
			r.Delete("/{id}", alarmHandler.DeleteRule)
		})

		// Audit
		r.Get("/audit", auditHandler.ListAuditEvents)
	})

	// WebSocket (fora do prefixo API)
	r.Get("/ws/{deviceID}", func(w http.ResponseWriter, r *http.Request) {
		deviceID := chi.URLParam(r, "deviceID")
		wsHub.HandleConnection(w, r, deviceID)
	})

	return r
}
