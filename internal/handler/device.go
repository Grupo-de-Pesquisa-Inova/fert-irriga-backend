package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"fertirriga-backend/internal/domain"
	"fertirriga-backend/internal/service"
)

// DeviceHandler contém os handlers REST para operações com devices.
type DeviceHandler struct {
	svc *service.DeviceService
}

func NewDeviceHandler(svc *service.DeviceService) *DeviceHandler {
	return &DeviceHandler{svc: svc}
}

// ListDevices retorna todos os dispositivos registrados.
// GET /api/v1/devices
func (h *DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.svc.ListDevices(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao listar devices", err)
		return
	}
	if devices == nil {
		devices = []domain.Device{}
	}
	writeJSON(w, http.StatusOK, devices)
}

// GetDevice retorna o estado atual de um dispositivo.
// GET /api/v1/devices/{deviceID}
func (h *DeviceHandler) GetDevice(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")

	device, err := h.svc.GetDevice(r.Context(), deviceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "device não encontrado", err)
		return
	}
	writeJSON(w, http.StatusOK, device)
}

// SendControl envia um comando de controle ao ESP32.
// POST /api/v1/devices/{deviceID}/control
func (h *DeviceHandler) SendControl(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")

	var control domain.ControlStatus
	if err := json.NewDecoder(r.Body).Decode(&control); err != nil {
		writeError(w, http.StatusBadRequest, "payload inválido", err)
		return
	}

	if err := h.svc.SendControl(r.Context(), deviceID, control); err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao enviar comando", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "sent",
		"device":  deviceID,
		"message": "comando enviado ao ESP32",
	})
}

// EmergencyStop aciona parada de emergência.
// POST /api/v1/devices/{deviceID}/emergency-stop
func (h *DeviceHandler) EmergencyStop(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")

	if err := h.svc.EmergencyStop(r.Context(), deviceID); err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao acionar parada de emergência", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "emergency_stop_sent",
		"device":  deviceID,
		"message": "PARADA DE EMERGÊNCIA enviada via MQTT QoS 2",
	})
}

// EmergencyReset libera a parada de emergência.
// POST /api/v1/devices/{deviceID}/emergency-reset
func (h *DeviceHandler) EmergencyReset(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")

	if err := h.svc.SetEmergency(r.Context(), deviceID, false); err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao resetar parada de emergência", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "emergency_reset_sent",
		"device":  deviceID,
		"message": "RESET DE EMERGÊNCIA enviado via MQTT QoS 2",
	})
}

// GetTelemetry retorna histórico de telemetria em um intervalo.
// GET /api/v1/devices/{deviceID}/telemetry?from=...&to=...&limit=100
func (h *DeviceHandler) GetTelemetry(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")

	from := parseTime(r.URL.Query().Get("from"), time.Now().Add(-24*time.Hour))
	to := parseTime(r.URL.Query().Get("to"), time.Now())
	limit := parseInt(r.URL.Query().Get("limit"), 100)

	records, err := h.svc.GetTelemetry(r.Context(), deviceID, from, to, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao consultar telemetria", err)
		return
	}
	if records == nil {
		records = []domain.TelemetryRecord{}
	}
	writeJSON(w, http.StatusOK, records)
}

// GetLatestTelemetry retorna as últimas leituras.
// GET /api/v1/devices/{deviceID}/telemetry/latest?limit=50
func (h *DeviceHandler) GetLatestTelemetry(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")
	limit := parseInt(r.URL.Query().Get("limit"), 50)

	records, err := h.svc.GetLatestTelemetry(r.Context(), deviceID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao consultar telemetria", err)
		return
	}
	if records == nil {
		records = []domain.TelemetryRecord{}
	}
	writeJSON(w, http.StatusOK, records)
}

// GetTelemetryHistory retorna uma página de telemetria com metadados de paginação.
// GET /api/v1/devices/{deviceID}/telemetry/history?page=1&page_size=20
func (h *DeviceHandler) GetTelemetryHistory(w http.ResponseWriter, r *http.Request) {
	deviceID := chi.URLParam(r, "deviceID")

	page := parseInt(r.URL.Query().Get("page"), 1)
	if page < 1 {
		page = 1
	}
	pageSize := parseInt(r.URL.Query().Get("page_size"), 20)
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	records, total, err := h.svc.GetTelemetryPage(r.Context(), deviceID, pageSize, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "erro ao consultar histórico de telemetria", err)
		return
	}
	if records == nil {
		records = []domain.TelemetryRecord{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":     records,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string, err error) {
	slog.Error(msg, "error", err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func parseTime(s string, fallback time.Time) time.Time {
	if s == "" {
		return fallback
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return fallback
	}
	return t
}

func parseInt(s string, fallback int) int {
	if s == "" {
		return fallback
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return v
}
