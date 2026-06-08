package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"fertirriga-backend/internal/domain"
	"fertirriga-backend/internal/repository"
)

type ScheduleHandler struct {
	repo      *repository.ScheduleRepo
	auditRepo *repository.AuditRepo
}

func NewScheduleHandler(repo *repository.ScheduleRepo, auditRepo *repository.AuditRepo) *ScheduleHandler {
	return &ScheduleHandler{repo: repo, auditRepo: auditRepo}
}

func (h *ScheduleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var s domain.Schedule
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		respondError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	if s.ZoneID == "" || s.Name == "" {
		respondError(w, http.StatusBadRequest, "zone_id e name são obrigatórios")
		return
	}
	if s.ScheduleType == "" {
		s.ScheduleType = "one_time"
	}
	if s.Origin == "" {
		s.Origin = "web_manual"
	}
	s.IsEnabled = true
	s.Version = 1

	if err := h.repo.Create(r.Context(), &s); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditRepo.AuditLog(r.Context(), "schedule_created", "system", "schedule", s.ID, s)
	respondJSON(w, http.StatusCreated, s)
}

func (h *ScheduleHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s, err := h.repo.Get(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusNotFound, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, s)
}

func (h *ScheduleHandler) List(w http.ResponseWriter, r *http.Request) {
	zoneID := r.URL.Query().Get("zone_id")
	if zoneID == "" {
		respondError(w, http.StatusBadRequest, "query param zone_id é obrigatório")
		return
	}
	schedules, err := h.repo.ListByZone(r.Context(), zoneID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, schedules)
}

func (h *ScheduleHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var s domain.Schedule
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		respondError(w, http.StatusBadRequest, "JSON inválido")
		return
	}
	s.ID = id
	if err := h.repo.Update(r.Context(), &s); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditRepo.AuditLog(r.Context(), "schedule_updated", "system", "schedule", id, s)
	respondJSON(w, http.StatusOK, s)
}

func (h *ScheduleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Delete(r.Context(), id); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditRepo.AuditLog(r.Context(), "schedule_deleted", "system", "schedule", id, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (h *ScheduleHandler) Enable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.SetEnabled(r.Context(), id, true); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditRepo.AuditLog(r.Context(), "schedule_enabled", "system", "schedule", id, nil)
	respondJSON(w, http.StatusOK, map[string]string{"status": "enabled"})
}

func (h *ScheduleHandler) Disable(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.SetEnabled(r.Context(), id, false); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.auditRepo.AuditLog(r.Context(), "schedule_disabled", "system", "schedule", id, nil)
	respondJSON(w, http.StatusOK, map[string]string{"status": "disabled"})
}

func (h *ScheduleHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	runs, err := h.repo.ListRuns(r.Context(), id)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, runs)
}
