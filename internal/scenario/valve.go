package scenario

import (
	"errors"
	"fertirriga-backend/internal/domain"
	"slices"
	"time"
)

var ValveTargets = []string{"VA", "VAD1", "VAD2", "VG", "VL13", "VL46", "VRA1", "VRA2"}

func IsValve(target string) bool { return slices.Contains(ValveTargets, target) }

// Both the server and controller must validate against current outputs. A manual
// command changes one valve only; it never starts a pump or selects a scenario.
func CheckValveOutputs(outputs []string, target string, state, expected bool) error {
	if !IsValve(target) {
		return errors.New("válvula inválida")
	}
	if slices.Contains(outputs, "M") {
		return errors.New("pare a bomba antes de alterar uma válvula individualmente")
	}
	if slices.Contains(outputs, target) != expected {
		return errors.New("o estado da válvula mudou; confira a telemetria e confirme novamente")
	}
	if !state {
		return nil
	}
	next := append(append([]string{}, outputs...), target)
	sources := 0
	for _, source := range []string{"VA", "VAD1", "VAD2"} {
		if slices.Contains(next, source) {
			sources++
		}
	}
	if sources > 1 {
		return errors.New("feche a origem atual antes de abrir outra; mistura de origens bloqueada")
	}
	ret1, ret2 := slices.Contains(next, "VRA1"), slices.Contains(next, "VRA2")
	if ret1 && ret2 || (ret1 || ret2) && slices.Contains(next, "VG") || ret1 && slices.Contains(next, "VAD2") || ret2 && slices.Contains(next, "VAD1") {
		return errors.New("o retorno selecionado conflita com o percurso aberto")
	}
	return nil
}

func CheckValveDevice(d *domain.Device, target string, state, expected bool, now time.Time) error {
	if d == nil || !d.IsOnline || d.LastSeenAt == nil || now.Sub(*d.LastSeenAt) > 15*time.Second || !d.Payload.IsCurrent(now) {
		return errors.New("controlador offline ou sem telemetria recente")
	}
	if !slices.Contains(d.Payload.Capabilities, "valves_v1") {
		return errors.New("o firmware ainda não suporta acionamento individual das válvulas do modelo")
	}
	if d.Payload.Seguranca.ParadaEmergencia {
		return errors.New("parada de emergência ativa")
	}
	if d.Payload.StatusSistema.Operacao.ModoAtual == "manutencao" {
		return errors.New("controlador em manutenção")
	}
	if d.Payload.StatusSistema.Operacao.ModoAtual == "cenario" {
		return errors.New("encerre o cenário antes de alterar uma válvula")
	}
	if state && (d.Payload.Seguranca.AlertaFalhaFluxo || d.Payload.Seguranca.RelogioDessincronizado) {
		return errors.New("resolva o alerta de fluxo ou de relógio antes de abrir a válvula")
	}
	return CheckValveOutputs(d.Payload.StatusSistema.Operacao.SaidasAtivas, target, state, expected)
}
