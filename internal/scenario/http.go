package scenario

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func bad(w http.ResponseWriter, status int, message string) {
	reply(w, status, map[string]string{"error": message})
}

func (s *Service) Routes(r chi.Router) {
	r.Get("/scenarios", func(w http.ResponseWriter, r *http.Request) { reply(w, 200, Catalog) })
	r.Get("/scenario-runs", func(w http.ResponseWriter, r *http.Request) {
		runs, err := s.Store.Runs(r.Context(), r.URL.Query().Get("device_id"))
		if err != nil {
			bad(w, 503, "Não foi possível consultar as execuções.")
			return
		}
		reply(w, 200, runs)
	})
	r.Post("/scenario-runs", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			DeviceID    string `json:"device_id"`
			ScenarioID  string `json:"scenario_id"`
			DurationSec int    `json:"duration_sec"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&req) != nil {
			bad(w, 400, "JSON inválido")
			return
		}
		if err := Validate(req.DeviceID, req.ScenarioID, req.DurationSec); err != nil {
			bad(w, 400, err.Error())
			return
		}
		run, err := s.Start(r.Context(), req.DeviceID, req.ScenarioID, req.DurationSec)
		if err != nil {
			bad(w, 409, err.Error())
			return
		}
		reply(w, 202, run)
	})
	r.Post("/scenario-runs/{id}/stop", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Stop(r.Context(), r.URL.Query().Get("device_id"), chi.URLParam(r, "id")); err != nil {
			bad(w, 409, err.Error())
			return
		}
		reply(w, 202, map[string]string{"status": "stopping"})
	})
	r.Get("/scenario-plans", func(w http.ResponseWriter, r *http.Request) {
		plans, err := s.Store.Plans(r.Context(), r.URL.Query().Get("device_id"))
		if err != nil {
			bad(w, 503, "Não foi possível consultar os agendamentos.")
			return
		}
		reply(w, 200, plans)
	})
	save := func(w http.ResponseWriter, r *http.Request) {
		var p Plan
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192)).Decode(&p) != nil {
			bad(w, 400, "JSON inválido")
			return
		}
		p.ID = chi.URLParam(r, "id")
		p.Name = strings.TrimSpace(p.Name)
		if err := Validate(p.DeviceID, p.ScenarioID, p.DurationSec); err != nil {
			bad(w, 400, err.Error())
			return
		}
		if len(p.Name) == 0 || len(p.Name) > 120 || p.StartAt.IsZero() || (p.Repeat != "once" && p.Repeat != "daily") {
			bad(w, 400, "Informe nome, data, horário e repetição válidos.")
			return
		}
		now := time.Now()
		next := p.StartAt
		if p.Enabled && !next.After(now) {
			if p.Repeat == "once" {
				bad(w, 400, "Escolha um horário futuro para a execução única.")
				return
			}
			next = NextDaily(next, now)
		}
		p.NextAt = nil
		if p.Enabled {
			p.NextAt = &next
		}
		if err := s.Store.SavePlan(r.Context(), &p); err != nil {
			bad(w, 503, "Não foi possível salvar o agendamento.")
			return
		}
		reply(w, 200, p)
	}
	r.Post("/scenario-plans", save)
	r.Put("/scenario-plans/{id}", save)
	r.Delete("/scenario-plans/{id}", func(w http.ResponseWriter, r *http.Request) {
		tag, err := s.Store.Pool.Exec(r.Context(), `UPDATE scenario_plans SET enabled=false,deleted_at=now() WHERE id=$1 AND device_id=$2 AND deleted_at IS NULL`, chi.URLParam(r, "id"), r.URL.Query().Get("device_id"))
		if err != nil {
			bad(w, 503, "Não foi possível excluir o agendamento.")
			return
		}
		if tag.RowsAffected() == 0 {
			bad(w, 404, "Agendamento não encontrado.")
			return
		}
		w.WriteHeader(204)
	})
}
