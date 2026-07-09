package worker

import (
	"context"
	"log/slog"
	"time"

	"fertirriga-backend/internal/repository"
)

// TelemetryRetentionWorker apaga periodicamente telemetria mais antiga que a
// janela de retenção, evitando que a tabela cresça sem limite.
type TelemetryRetentionWorker struct {
	telemetryRepo   *repository.TelemetryRepo
	interval        time.Duration
	retentionMonths int
	stopCh          chan struct{}
}

func NewTelemetryRetentionWorker(telemetryRepo *repository.TelemetryRepo) *TelemetryRetentionWorker {
	return &TelemetryRetentionWorker{
		telemetryRepo:   telemetryRepo,
		interval:        24 * time.Hour,
		retentionMonths: 6,
		stopCh:          make(chan struct{}),
	}
}

// Start dispara uma limpeza imediata (para aliviar o acumulado já existente
// assim que o backend sobe) e depois repete a cada interval.
func (w *TelemetryRetentionWorker) Start() {
	slog.Info("[TelemetryRetentionWorker] Iniciado", "interval", w.interval, "retention_months", w.retentionMonths)
	go w.loop()
}

func (w *TelemetryRetentionWorker) Stop() {
	close(w.stopCh)
	slog.Info("[TelemetryRetentionWorker] Parado")
}

func (w *TelemetryRetentionWorker) loop() {
	w.tick()

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

func (w *TelemetryRetentionWorker) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cutoff := time.Now().AddDate(0, -w.retentionMonths, 0)
	deleted, err := w.telemetryRepo.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		slog.Error("[TelemetryRetentionWorker] Erro ao apagar telemetria antiga", "error", err)
		return
	}

	if deleted > 0 {
		slog.Info("[TelemetryRetentionWorker] Telemetria antiga removida", "linhas", deleted, "cutoff", cutoff)
	}
}
