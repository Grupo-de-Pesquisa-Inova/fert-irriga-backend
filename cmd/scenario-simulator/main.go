// Local protocol simulator. Never drives physical GPIOs.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"sync"
	"time"

	"fertirriga-backend/internal/domain"
	"fertirriga-backend/internal/scenario"
	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

type outcome struct {
	RunID      string    `json:"run_id"`
	Status     string    `json:"status"`
	ErrorCode  string    `json:"error_code,omitempty"`
	Message    string    `json:"message,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
}
type simulator struct {
	sync.Mutex
	active        string
	outputs       []string
	ends          time.Time
	started       time.Time
	outcomes      map[string]outcome
	fault         string
	valveOutcomes map[string]valveOutcome
}

type valveOutcome struct {
	Protocol  string `json:"protocol"`
	CommandID string `json:"command_id"`
	Target    string `json:"target"`
	State     bool   `json:"state"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
}

func main() {
	broker := flag.String("broker", "mqtt://127.0.0.1:51883", "Local MQTT broker")
	device := flag.String("device", "esp32-001", "Device identifier")
	fault := flag.String("fault", "", "noack or flow_fault")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	u, err := url.Parse(*broker)
	if err != nil {
		log.Fatal(err)
	}
	sim := &simulator{outcomes: map[string]outcome{}, fault: *fault}
	var cm *autopaho.ConnectionManager
	cm, err = autopaho.NewConnection(ctx, autopaho.ClientConfig{
		ServerUrls: []*url.URL{u}, KeepAlive: 10, CleanStartOnInitialConnection: true,
		OnConnectionUp: func(c *autopaho.ConnectionManager, _ *paho.Connack) {
			_, err := c.Subscribe(ctx, &paho.Subscribe{Subscriptions: []paho.SubscribeOptions{{Topic: "fertirriga/" + *device + "/cenario", QoS: 1}, {Topic: "fertirriga/" + *device + "/comando", QoS: 1}, {Topic: "fertirriga/" + *device + "/emergencia", QoS: 2}}})
			if err != nil {
				log.Print(err)
			}
		},
		ClientConfig: paho.ClientConfig{ClientID: "scenario-simulator-" + *device, Router: paho.NewStandardRouterWithDefault(func(p *paho.Publish) { sim.command(p.Payload, p.Topic) })},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cm.Disconnect(context.Background())
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	log.Printf("Scenario simulator %s at %s (no GPIO)", *device, *broker)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sim.Lock()
			payload := domain.PayloadESP32{Capabilities: []string{"scenarios_v1", "valves_v1"}, Ts: time.Now().Unix()}
			payload.StatusSistema.Conexao.Estado = "online"
			payload.StatusSistema.Conexao.SinalWifiDBM = -45
			payload.StatusSistema.Operacao.ModoAtual = "stand-by"
			payload.StatusSistema.Operacao.SaidasAtivas = append([]string{}, sim.outputs...)
			payload.StatusSistema.Sensores.Clima.TemperaturaC = 24
			payload.StatusSistema.Sensores.Clima.UmidadePct = 65
			payload.StatusSistema.Sensores.Clima.PressaoHPA = 1013
			if sim.active != "" {
				payload.StatusSistema.Operacao.ModoAtual = "cenario"
				payload.StatusSistema.Sensores.Hidraulica.FluxoDetectado = true
				payload.StatusSistema.Sensores.Hidraulica.VazaoLPM = 4.2
			}
			pending := []outcome{}
			valveAcks := []valveOutcome{}
			for _, o := range sim.valveOutcomes {
				valveAcks = append(valveAcks, o)
			}
			for _, o := range sim.outcomes {
				pending = append(pending, o)
			}
			sim.Unlock()
			publish := func(topic string, v any) error {
				raw, _ := json.Marshal(v)
				c, done := context.WithTimeout(ctx, time.Second)
				defer done()
				_, err := cm.Publish(c, &paho.Publish{Topic: "fertirriga/" + *device + "/" + topic, QoS: 1, Payload: raw})
				return err
			}
			if err := publish("telemetria", payload); err != nil {
				log.Print(err)
			}
			if *fault != "noack" {
				for _, o := range valveAcks {
					if err := publish("ack", o); err != nil {
						log.Print(err)
					}
				}
				for _, o := range pending {
					if err := publish("ack", o); err != nil {
						log.Print(err)
					}
				}
			}
		}
	}
}

func (s *simulator) command(raw []byte, topic string) {
	s.Lock()
	defer s.Unlock()
	var c struct {
		Protocol    string    `json:"protocol"`
		Action      string    `json:"action"`
		RunID       string    `json:"run_id"`
		ScenarioID  string    `json:"scenario_id"`
		DurationSec int       `json:"duration_sec"`
		ExpiresAt   time.Time `json:"expires_at"`
		Emergency   bool      `json:"parada_emergencia"`
		CommandID   string    `json:"command_id"`
		Target      string    `json:"target"`
		State       bool      `json:"state"`
		Expected    *bool     `json:"expected_state"`
	}
	if json.Unmarshal(raw, &c) != nil {
		return
	}
	if c.Protocol == "valves_v1" && c.CommandID != "" {
		if s.valveOutcomes == nil {
			s.valveOutcomes = map[string]valveOutcome{}
		}
		if _, exists := s.valveOutcomes[c.CommandID]; exists {
			return
		}
		o := valveOutcome{Protocol: "valves_v1", CommandID: c.CommandID, Target: c.Target, State: slices.Contains(s.outputs, c.Target), Status: "rejected"}
		switch {
		case c.Action != "set_valve" || c.Expected == nil:
			o.Message = "Comando inválido."
		case c.ExpiresAt.Before(time.Now()) || c.ExpiresAt.After(time.Now().Add(20*time.Second)):
			o.Message = "Comando expirado ou validade inválida."
		case s.active != "":
			o.Message = "Outro cenário ativo."
		default:
			if err := scenario.CheckValveOutputs(s.outputs, c.Target, c.State, *c.Expected); err != nil {
				o.Message = err.Error()
			} else {
				s.outputs = slices.DeleteFunc(s.outputs, func(v string) bool { return v == c.Target })
				if c.State {
					s.outputs = append(s.outputs, c.Target)
				}
				o.State, o.Status = c.State, "executed"
			}
		}
		s.valveOutcomes[c.CommandID] = o
		return
	}
	if c.Emergency {
		s.outputs = nil
	}
	if c.Emergency && s.active != "" {
		c.Action = "stop"
		c.RunID = s.active
	}
	if c.RunID == "" {
		return
	}
	now := time.Now().UTC()
	if c.Action == "stop" {
		if s.active == c.RunID {
			s.outputs = nil
			s.active = ""
		}
		if previous, ok := s.outcomes[c.RunID]; ok && previous.Status != "running" {
			return
		}
		s.outcomes[c.RunID] = outcome{RunID: c.RunID, Status: "cancelled", StartedAt: s.started, FinishedAt: now}
		return
	}
	if _, exists := s.outcomes[c.RunID]; exists {
		return
	} // duplicate starts never reset the local timer
	result := outcome{RunID: c.RunID, Status: "failed", FinishedAt: now}
	if c.Protocol != "scenarios_v1" || c.Action != "start" {
		return
	}
	if now.After(c.ExpiresAt) {
		result.ErrorCode = "expired"
		result.Message = "Comando expirado."
	} else if s.active != "" || len(s.outputs) > 0 {
		result.ErrorCode = "device_busy"
		result.Message = "Outro cenário ativo."
	} else if err := scenario.Validate("sim", c.ScenarioID, c.DurationSec); err != nil {
		result.ErrorCode = "invalid_command"
		result.Message = err.Error()
	} else {
		def, _ := scenario.Find(c.ScenarioID)
		s.active = c.RunID
		s.outputs = def.Outputs
		s.started = now
		s.ends = now.Add(time.Duration(c.DurationSec) * time.Second)
		if s.fault == "flow_fault" {
			s.ends = now.Add(3 * time.Second)
		}
		runID := c.RunID
		time.AfterFunc(time.Until(s.ends), func() { s.finish(runID) })
		result = outcome{RunID: c.RunID, Status: "running", StartedAt: now}
	}
	s.outcomes[c.RunID] = result
	fmt.Printf("scenario %s: %s\n", c.RunID, result.Status)
}

// Independent of telemetry/MQTT: a blocked connection must never extend irrigation.
func (s *simulator) finish(id string) {
	s.Lock()
	defer s.Unlock()
	if s.active != id {
		return
	}
	result := outcome{RunID: id, Status: "completed", StartedAt: s.started, FinishedAt: time.Now().UTC()}
	if s.fault == "flow_fault" {
		result.Status = "failed"
		result.ErrorCode = "flow_fault"
		result.Message = "Fluxo não detectado. Bomba desligada pelo controlador simulado."
	}
	s.outcomes[id] = result
	s.active = ""
	s.outputs = nil
}
