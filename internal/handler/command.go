package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"fertirriga-backend/internal/domain"
	mqttclient "fertirriga-backend/internal/mqtt"
	"fertirriga-backend/internal/repository"
	"fertirriga-backend/internal/safety"
)

type CommandHandler struct {
	repo         *repository.CommandRepo
	auditRepo    *repository.AuditRepo
	safetyEngine *safety.Engine
	mqttClient   *mqttclient.Client
	wsHub        *WSHub
}

func NewCommandHandler(repo *repository.CommandRepo, auditRepo *repository.AuditRepo, safetyEngine *safety.Engine, mqttClient *mqttclient.Client, wsHub *WSHub) *CommandHandler {
	return &CommandHandler{repo: repo, auditRepo: auditRepo, safetyEngine: safetyEngine, mqttClient: mqttClient, wsHub: wsHub}
}

func (h *CommandHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateCommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if req.DeviceID == "" || req.Action == "" {
		respondError(w, http.StatusBadRequest, "device_id e action são obrigatórios")
		return
	}

	cmd := domain.ManualCommand{
		DeviceID:      req.DeviceID,
		Action:        req.Action,
		TargetChannel: req.TargetChannel,
		Parameters:    req.Parameters,
		Origin:        req.Origin,
		Actor:         req.Actor,
		Status:        "pending",
		Priority:      4,
	}

	// Safety check — avaliar regras antes de despachar
	if h.safetyEngine != nil {
		violations := h.safetyEngine.Check(r.Context(), &cmd)
		if safety.IsBlocking(violations) {
			h.auditRepo.AuditLog(r.Context(), "command_rejected", cmd.Actor, "command", cmd.DeviceID, map[string]interface{}{
				"action":     cmd.Action,
				"violations": violations,
			})
			respondJSON(w, http.StatusUnprocessableEntity, map[string]interface{}{
				"error":      "Comando bloqueado por regras de segurança",
				"violations": violations,
			})
			return
		}
	}

	if err := h.repo.Create(r.Context(), &cmd); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Despachar via MQTT
	if h.mqttClient != nil {
		mqttPayload, _ := json.Marshal(map[string]interface{}{
			"command_id":     cmd.CommandID,
			"action":         cmd.Action,
			"target_channel": cmd.TargetChannel,
			"parameters":     cmd.Parameters,
			"priority":       cmd.Priority,
		})

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := h.mqttClient.PublishCommand(ctx, cmd.DeviceID, "comando", mqttPayload); err != nil {
				slog.Error("[Command] Falha ao despachar MQTT", "error", err, "command", cmd.CommandID)
				return
			}

			// Marcar como dispatched
			now := time.Now()
			cmd.Status = "dispatched"
			cmd.DispatchedAt = &now
			h.repo.UpdateStatus(context.Background(), cmd.CommandID, "dispatched")

			// Broadcast status update via WebSocket
			if h.wsHub != nil {
				h.wsHub.BroadcastEvent("command_status", map[string]interface{}{
					"command_id": cmd.CommandID,
					"status":     "dispatched",
					"device_id":  cmd.DeviceID,
				})
			}
		}()
	}

	h.auditRepo.AuditLog(r.Context(), "command_created", cmd.Actor, "command", cmd.CommandID, cmd)
	respondJSON(w, http.StatusCreated, cmd)
}

func (h *CommandHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	cmd, err := h.repo.Get(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, cmd)
}

func (h *CommandHandler) List(w http.ResponseWriter, r *http.Request) {
	deviceID := r.URL.Query().Get("device_id")
	cmds, err := h.repo.List(r.Context(), deviceID, 50)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, cmds)
}
