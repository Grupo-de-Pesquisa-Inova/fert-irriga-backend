package handler

import (
	"net/http"

	"fertirriga-backend/internal/repository"
)

// AuditHandler expõe o histórico de eventos de auditoria.
type AuditHandler struct {
	repo *repository.AuditRepo
}

func NewAuditHandler(repo *repository.AuditRepo) *AuditHandler {
	return &AuditHandler{repo: repo}
}

func (h *AuditHandler) ListAuditEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.repo.ListAuditEvents(r.Context(), 50, 0)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, events)
}
