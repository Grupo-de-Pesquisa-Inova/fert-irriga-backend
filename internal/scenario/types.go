package scenario

import (
	"embed"
	"encoding/json"
	"errors"
	"regexp"
	"time"

	"fertirriga-backend/internal/domain"
)

//go:embed catalog.json
var catalogFS embed.FS

type Definition struct {
	ID      string    `json:"id"`
	Name    string    `json:"name"`
	Source  string    `json:"source"`
	Outputs []string  `json:"outputs"`
	Groups  []string  `json:"groups"`
	Color   []float64 `json:"color"`
}

var Catalog = func() []Definition {
	data, _ := catalogFS.ReadFile("catalog.json")
	var result []Definition
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return result
}()

func Find(id string) (Definition, bool) {
	for _, d := range Catalog {
		if d.ID == id {
			return d, true
		}
	}
	return Definition{}, false
}

type Plan struct {
	ID          string     `json:"id"`
	DeviceID    string     `json:"device_id"`
	ScenarioID  string     `json:"scenario_id"`
	Name        string     `json:"name"`
	DurationSec int        `json:"duration_sec"`
	StartAt     time.Time  `json:"start_at"`
	Repeat      string     `json:"repeat"`
	Enabled     bool       `json:"enabled"`
	NextAt      *time.Time `json:"next_at"`
}

type Run struct {
	ID          string     `json:"id"`
	PlanID      *string    `json:"plan_id"`
	DeviceID    string     `json:"device_id"`
	ScenarioID  string     `json:"scenario_id"`
	DurationSec int        `json:"duration_sec"`
	DueAt       time.Time  `json:"due_at"`
	Status      string     `json:"status"`
	ErrorCode   string     `json:"error_code"`
	Message     string     `json:"message"`
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at"`
	EndsAt      *time.Time `json:"ends_at"`
	FinishedAt  *time.Time `json:"finished_at"`
}

var devicePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,100}$`)

func Validate(device, id string, duration int) error {
	if !devicePattern.MatchString(device) {
		return errors.New("dispositivo inválido")
	}
	if _, ok := Find(id); !ok {
		return errors.New("cenário inválido")
	}
	if duration < 1 || duration > 7200 {
		return errors.New("duração deve estar entre 1 e 7200 segundos")
	}
	return nil
}

// Recorrência diária ancorada no instante escolhido, sem deriva após atrasos.
func NextDaily(due, now time.Time) time.Time {
	return due.Add(time.Duration(int64(now.Sub(due)/(24*time.Hour))+1) * 24 * time.Hour)
}

func CheckDevice(d *domain.Device, now time.Time) (string, string) {
	if d == nil || !d.IsOnline || d.LastSeenAt == nil || now.Sub(*d.LastSeenAt) > 60*time.Second {
		return "device_offline", "Irrigação não iniciada: controlador offline ou sem telemetria recente. A causa da desconexão não foi confirmada."
	}
	if d.Payload.Seguranca.ParadaEmergencia {
		return "emergency", "Parada de emergência ativa."
	}
	if d.Payload.Seguranca.AlertaFalhaFluxo {
		return "flow_fault", "Alerta de falha de fluxo ativo."
	}
	if d.Payload.Seguranca.RelogioDessincronizado {
		return "clock_desync", "Relógio do controlador dessincronizado."
	}
	if d.Payload.StatusSistema.Operacao.ModoAtual == "manutencao" {
		return "maintenance", "Controlador em manutenção."
	}
	supported := false
	for _, c := range d.Payload.Capabilities {
		if c == "scenarios_v1" {
			supported = true
		}
	}
	if !supported {
		return "unsupported_firmware", "Controlador ainda não suporta os cenários hidráulicos (scenarios_v1)."
	}
	if len(d.Payload.StatusSistema.Operacao.SaidasAtivas) > 0 {
		return "device_busy", "Há saídas ativas. Aguarde o término da operação atual."
	}
	return "", ""
}
