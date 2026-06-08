package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"fertirriga-backend/internal/repository"
)

// SchedulerWorker verifica agendamentos vencidos e os marca como executados.
//
// O despacho baseado em receitas (recipe steps) foi removido durante a limpeza
// do backend — receitas deixaram de existir no escopo de 1 ESP32. O worker
// permanece responsável por avançar a janela de execução dos schedules e
// registrar a auditoria de cada disparo.
type SchedulerWorker struct {
	scheduleRepo *repository.ScheduleRepo
	auditRepo    *repository.AuditRepo
	interval     time.Duration
	stopCh       chan struct{}
}

func NewSchedulerWorker(scheduleRepo *repository.ScheduleRepo, auditRepo *repository.AuditRepo) *SchedulerWorker {
	return &SchedulerWorker{
		scheduleRepo: scheduleRepo,
		auditRepo:    auditRepo,
		interval:     10 * time.Second,
		stopCh:       make(chan struct{}),
	}
}

func (w *SchedulerWorker) Start() {
	slog.Info("[SchedulerWorker] Iniciado", "interval", w.interval)
	go w.loop()
}

func (w *SchedulerWorker) Stop() {
	close(w.stopCh)
	slog.Info("[SchedulerWorker] Parado")
}

func (w *SchedulerWorker) loop() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.tick()
		}
	}
}

func (w *SchedulerWorker) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dueSchedules, err := w.scheduleRepo.GetDueSchedules(ctx)
	if err != nil {
		slog.Error("[SchedulerWorker] Erro ao buscar schedules", "error", err)
		return
	}

	for _, schedule := range dueSchedules {
		slog.Info("[SchedulerWorker] Executando schedule", "id", schedule.ID, "name", schedule.Name)

		result, _ := json.Marshal(map[string]interface{}{
			"note": "executed by scheduler worker",
		})

		// Atualizar next_execution para schedules recorrentes
		if schedule.ScheduleType == "recurring" && schedule.CronExpression != "" {
			nextExec := time.Now().Add(24 * time.Hour) // placeholder — idealmente parsear cron
			schedule.NextExecutionAt = &nextExec
			w.scheduleRepo.Update(ctx, &schedule)
		} else {
			// One-time: desabilitar após execução
			w.scheduleRepo.SetEnabled(ctx, schedule.ID, false)
		}

		w.auditRepo.AuditLog(ctx, "schedule_executed", "scheduler_worker", "schedule", schedule.ID, map[string]interface{}{
			"result": string(result),
		})

		slog.Info("[SchedulerWorker] Schedule executado", "id", schedule.ID)
	}
}
