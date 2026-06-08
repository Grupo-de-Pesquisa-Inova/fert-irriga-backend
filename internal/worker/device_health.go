package worker

import (
	"context"
	"log/slog"
	"time"

	"fertirriga-backend/internal/repository"
)

// DeviceHealthWorker marca devices como offline se last_seen > threshold.
type DeviceHealthWorker struct {
	deviceRepo *repository.DeviceRepo
	auditRepo  *repository.AuditRepo
	interval   time.Duration
	threshold  time.Duration
	stopCh     chan struct{}
}

func NewDeviceHealthWorker(deviceRepo *repository.DeviceRepo, auditRepo *repository.AuditRepo) *DeviceHealthWorker {
	return &DeviceHealthWorker{
		deviceRepo: deviceRepo,
		auditRepo:  auditRepo,
		interval:   15 * time.Second,
		threshold:  60 * time.Second,
		stopCh:     make(chan struct{}),
	}
}

func (w *DeviceHealthWorker) Start() {
	slog.Info("[DeviceHealthWorker] Iniciado", "interval", w.interval, "threshold", w.threshold)
	go w.loop()
}

func (w *DeviceHealthWorker) Stop() {
	close(w.stopCh)
	slog.Info("[DeviceHealthWorker] Parado")
}

func (w *DeviceHealthWorker) loop() {
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

func (w *DeviceHealthWorker) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	devices, err := w.deviceRepo.ListAll(ctx)
	if err != nil {
		slog.Error("[DeviceHealthWorker] Erro ao listar devices", "error", err)
		return
	}

	now := time.Now()
	for _, d := range devices {
		if !d.IsOnline {
			continue
		}

		if d.LastSeenAt == nil || now.Sub(*d.LastSeenAt) > w.threshold {
			slog.Warn("[DeviceHealthWorker] Marcando device como offline",
				"device", d.DeviceID,
				"last_seen", d.LastSeenAt,
			)

			if err := w.deviceRepo.SetOnlineStatus(ctx, d.DeviceID, false); err != nil {
				slog.Error("[DeviceHealthWorker] Erro ao atualizar status", "device", d.DeviceID, "error", err)
				continue
			}

			w.auditRepo.AuditLog(ctx, "device_offline", "health_worker", "device", d.DeviceID, map[string]interface{}{
				"last_seen": d.LastSeenAt,
				"threshold": w.threshold.String(),
			})
		}
	}
}
