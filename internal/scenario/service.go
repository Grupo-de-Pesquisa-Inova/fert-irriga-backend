package scenario

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"fertirriga-backend/internal/domain"
)

type DeviceReader interface {
	GetByDeviceID(context.Context, string) (*domain.Device, error)
}
type Publisher interface {
	PublishCommand(context.Context, string, string, []byte) error
}
type Service struct {
	Store   *Store
	Devices DeviceReader
	MQTT    Publisher
}

func (s *Service) dispatch(ctx context.Context, r *Run) error {
	d, err := s.Devices.GetByDeviceID(ctx, r.DeviceID)
	if err != nil {
		d = nil
	}
	code, message := CheckDevice(d, time.Now())
	if code != "" {
		return s.Store.Fail(ctx, r.ID, code, message)
	}
	def, _ := Find(r.ScenarioID)
	raw, err := json.Marshal(map[string]any{"protocol": "scenarios_v1", "action": "start", "run_id": r.ID, "scenario_id": r.ScenarioID, "outputs": def.Outputs, "duration_sec": r.DurationSec, "expires_at": time.Now().Add(15 * time.Second).UTC()})
	if err != nil {
		return err
	}
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = s.MQTT.PublishCommand(sendCtx, r.DeviceID, "cenario", raw); err != nil {
		// A transport error may occur after delivery: never claim the valves stayed closed.
		_, dbErr := s.Store.Pool.Exec(ctx, `UPDATE scenario_runs SET status='uncertain',error_code='mqtt_unconfirmed',message='Envio MQTT sem confirmação. O controlador pode ter recebido o comando; confirme a parada antes de tentar novamente.' WHERE id=$1 AND status='pending'`, r.ID)
		if dbErr != nil {
			return dbErr
		}
	}
	return nil
}

func (s *Service) Start(ctx context.Context, device, id string, duration int) (Run, error) {
	if err := Validate(device, id, duration); err != nil {
		return Run{}, err
	}
	r := Run{DeviceID: device, ScenarioID: id, DurationSec: duration, DueAt: time.Now(), Status: "pending"}
	if err := s.Store.CreateRun(ctx, &r); err != nil {
		return r, errors.New("não foi possível reservar a execução; verifique se já existe um cenário ativo ou sem confirmação")
	}
	if err := s.dispatch(ctx, &r); err != nil {
		return r, err
	}
	return scanRun(s.Store.Pool.QueryRow(ctx, `SELECT `+runColumns+` FROM scenario_runs WHERE id=$1`, r.ID))
}

func (s *Service) Stop(ctx context.Context, device, id string) error {
	r, err := scanRun(s.Store.Pool.QueryRow(ctx, `SELECT `+runColumns+` FROM scenario_runs WHERE id=$1 AND device_id=$2`, id, device))
	if err != nil {
		return errors.New("execução não encontrada")
	}
	if r.Status == "completed" || r.Status == "cancelled" || r.Status == "failed" {
		return errors.New("execução já encerrada")
	}
	// Persist before publish so an immediate ACK cannot be overwritten.
	if _, err = s.Store.Pool.Exec(ctx, `UPDATE scenario_runs SET status='stopping',ends_at=now(),message='Parada solicitada; aguardando confirmação.' WHERE id=$1 AND status IN ('pending','running','uncertain','stopping')`, id); err != nil {
		return err
	}
	raw, _ := json.Marshal(map[string]any{"protocol": "scenarios_v1", "action": "stop", "run_id": id})
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err = s.MQTT.PublishCommand(sendCtx, device, "cenario", raw); err != nil {
		_, dbErr := s.Store.Pool.Exec(ctx, `UPDATE scenario_runs SET status='uncertain',error_code='stop_unconfirmed',message='Não foi possível confirmar a parada pelo MQTT. Verifique o controlador.' WHERE id=$1 AND status='stopping'`, id)
		if dbErr != nil {
			return dbErr
		}
		return errors.New("parada sem confirmação; verifique a conexão e o controlador")
	}
	return nil
}

func (s *Service) Tick(ctx context.Context) error {
	if err := s.Store.Expire(ctx); err != nil {
		return err
	}
	for i := 0; i < 100; i++ {
		r, err := s.Store.Claim(ctx, time.Now())
		if err != nil {
			return err
		}
		if r == nil {
			return nil
		}
		if r.Status == "pending" {
			if err = s.dispatch(ctx, r); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Service) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		tickCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if err := s.Tick(tickCtx); err != nil {
			slog.Error("cenários: falha no agendador", "error", err)
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
