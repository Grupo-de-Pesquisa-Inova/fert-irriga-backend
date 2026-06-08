package safety

import (
	"context"
	"fmt"
	"time"

	"fertirriga-backend/internal/domain"
	"fertirriga-backend/internal/repository"
)

// ViolationType identifica o tipo de violação de segurança.
type ViolationType string

const (
	ViolationEmergencyActive   ViolationType = "emergency_active"
	ViolationMaintenanceLock   ViolationType = "maintenance_lock"
	ViolationMutexConflict     ViolationType = "mutex_conflict"
	ViolationMaxRuntimeExceeded ViolationType = "max_runtime_exceeded"
	ViolationCommTimeout       ViolationType = "comm_timeout"
	ViolationPriorityBlocked   ViolationType = "priority_blocked"
)

// Violation representa uma violação de regra de segurança.
type Violation struct {
	Type    ViolationType `json:"type"`
	Message string        `json:"message"`
}

// Engine avalia regras de segurança antes de despachar comandos.
type Engine struct {
	deviceRepo  *repository.DeviceRepo
	commandRepo *repository.CommandRepo
}

func NewEngine(deviceRepo *repository.DeviceRepo, commandRepo *repository.CommandRepo) *Engine {
	return &Engine{
		deviceRepo:  deviceRepo,
		commandRepo: commandRepo,
	}
}

// Check avalia todas as regras de segurança para um comando.
// Retorna nil se o comando é seguro, ou uma lista de violações.
func (e *Engine) Check(ctx context.Context, cmd *domain.ManualCommand) []Violation {
	var violations []Violation

	// RS-01: Verificar se parada de emergência está ativa
	device, err := e.deviceRepo.GetByDeviceID(ctx, cmd.DeviceID)
	if err != nil {
		violations = append(violations, Violation{
			Type:    ViolationCommTimeout,
			Message: fmt.Sprintf("Dispositivo %s não encontrado ou indisponível", cmd.DeviceID),
		})
		return violations
	}

	// Verificar emergência ativa via payload
	if device.Payload.Seguranca.ParadaEmergencia {
		violations = append(violations, Violation{
			Type:    ViolationEmergencyActive,
			Message: "Parada de emergência ativa — nenhum comando pode ser enviado",
		})
	}

	// RS-06: Verificar maintenance lock
	if device.Payload.StatusSistema.Operacao.ModoAtual == "manutencao" {
		violations = append(violations, Violation{
			Type:    ViolationMaintenanceLock,
			Message: fmt.Sprintf("Dispositivo %s está em modo manutenção — comandos remotos bloqueados", cmd.DeviceID),
		})
	}

	// RS-02: Verificar comm timeout (device offline > 60s)
	if !device.IsOnline || (device.LastSeenAt != nil && time.Since(*device.LastSeenAt) > 60*time.Second) {
		violations = append(violations, Violation{
			Type:    ViolationCommTimeout,
			Message: fmt.Sprintf("Dispositivo %s offline ou sem comunicação há mais de 60s", cmd.DeviceID),
		})
	}

	// RS-05: Verificar prioridade (comandos de menor prioridade não sobrepõem maiores)
	if cmd.Priority > 1 {
		cmds, err := e.commandRepo.List(ctx, cmd.DeviceID, 10)
		if err == nil {
			for _, c := range cmds {
				if c.Status == "dispatched" && c.Priority < cmd.Priority {
					violations = append(violations, Violation{
						Type:    ViolationPriorityBlocked,
						Message: fmt.Sprintf("Existe comando ativo com prioridade %d (superior à solicitada %d)", c.Priority, cmd.Priority),
					})
					break
				}
			}
		}
	}

	if len(violations) == 0 {
		return nil
	}
	return violations
}

// IsBlocking retorna true se alguma violação é bloqueante.
func IsBlocking(violations []Violation) bool {
	if violations == nil {
		return false
	}
	for _, v := range violations {
		switch v.Type {
		case ViolationEmergencyActive, ViolationMaintenanceLock, ViolationCommTimeout, ViolationMutexConflict, ViolationMaxRuntimeExceeded:
			return true
		}
	}
	return false
}
