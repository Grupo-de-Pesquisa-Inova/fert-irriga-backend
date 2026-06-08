package scheduler

import (
	"log/slog"

	"github.com/robfig/cron/v3"

	"fertirriga-backend/internal/service"
)

// Scheduler gerencia tarefas agendadas de irrigação e fertirrigação.
type Scheduler struct {
	cron      *cron.Cron
	deviceSvc *service.DeviceService
}

// New cria um scheduler com o serviço de devices.
func New(deviceSvc *service.DeviceService) *Scheduler {
	c := cron.New(cron.WithSeconds())
	return &Scheduler{
		cron:      c,
		deviceSvc: deviceSvc,
	}
}

// Start inicia o scheduler com tarefas padrão.
func (s *Scheduler) Start() {
	// Health check a cada 30 segundos — verificar quais devices estão offline
	s.cron.AddFunc("*/30 * * * * *", func() {
		slog.Debug("scheduler: verificando health dos devices")
		// TODO: implementar lógica de timeout de devices
		// Se um device não enviou telemetria nos últimos 60s, marcar como offline
	})

	// Tarefa de limpeza de telemetria antiga a cada 6 horas
	s.cron.AddFunc("0 0 */6 * * *", func() {
		slog.Info("scheduler: limpeza de telemetria antiga")
		// TODO: deletar telemetria com mais de 30 dias
	})

	s.cron.Start()
	slog.Info("scheduler iniciado")
}

// Stop para o scheduler graciosamente.
func (s *Scheduler) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	slog.Info("scheduler encerrado")
}
