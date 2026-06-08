package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"fertirriga-backend/internal/domain"
	"fertirriga-backend/internal/repository"
)

type AlarmHandler struct {
	repo      *repository.AlarmRepo
	auditRepo *repository.AuditRepo
}

func NewAlarmHandler(repo *repository.AlarmRepo, auditRepo *repository.AuditRepo) *AlarmHandler {
	return &AlarmHandler{repo: repo, auditRepo: auditRepo}
}

// ═══════════════════════════════════════════════════
// ALARM RULES
// ═══════════════════════════════════════════════════

func (h *AlarmHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var rule domain.AlarmRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		respondError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if rule.Name == "" || rule.ConditionType == "" || rule.Severity == "" {
		respondError(w, http.StatusBadRequest, "name, condition_type e severity são obrigatórios")
		return
	}
	rule.IsEnabled = true

	if err := h.repo.CreateRule(r.Context(), &rule); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditRepo.AuditLog(r.Context(), "alarm_rule_created", "system", "alarm_rule", rule.ID, rule)
	respondJSON(w, http.StatusCreated, rule)
}

func (h *AlarmHandler) GetRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rule, err := h.repo.GetRule(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, rule)
}

func (h *AlarmHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.repo.ListRules(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, rules)
}

func (h *AlarmHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var rule domain.AlarmRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		respondError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	rule.ID = id
	if err := h.repo.UpdateRule(r.Context(), &rule); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditRepo.AuditLog(r.Context(), "alarm_rule_updated", "system", "alarm_rule", id, rule)
	respondJSON(w, http.StatusOK, rule)
}

func (h *AlarmHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.DeleteRule(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditRepo.AuditLog(r.Context(), "alarm_rule_deleted", "system", "alarm_rule", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ═══════════════════════════════════════════════════
// ALARM EVENTS
// ═══════════════════════════════════════════════════

func (h *AlarmHandler) ListActiveAlarms(w http.ResponseWriter, r *http.Request) {
	events, err := h.repo.ListActiveEvents(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, events)
}

func (h *AlarmHandler) ListAlarmHistory(w http.ResponseWriter, r *http.Request) {
	events, err := h.repo.ListEventHistory(r.Context(), 50)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, events)
}

func (h *AlarmHandler) AcknowledgeAlarm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// TODO: extrair userID do auth context quando implementado
	userID := "system"
	if err := h.repo.AcknowledgeEvent(r.Context(), id, userID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditRepo.AuditLog(r.Context(), "alarm_acknowledged", userID, "alarm_event", id, nil)
	respondJSON(w, http.StatusOK, map[string]string{"status": "acknowledged"})
}
