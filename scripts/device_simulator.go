// device_simulator.go — simula um ESP32 publicando telemetria via MQTT
// Uso: go run scripts/device_simulator.go -broker tcp://localhost:1883 -device esp32-001
//
// Flags de cenário:
//   -scenario normal    → telemetria normal com variações
//   -scenario offline   → para de publicar após 30s (simula desconexão)
//   -scenario emergency → ativa parada de emergência após 20s
//   -scenario flowfail  → simula falha de fluxo (irrigação ativa sem fluxo)
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"
)

type Payload struct {
	StatusSistema StatusSistema `json:"status_sistema"`
	Controle      Controle      `json:"controle"`
	Seguranca     Seguranca     `json:"seguranca"`
}

type StatusSistema struct {
	Conexao  Conexao  `json:"conexao"`
	Sensores Sensores `json:"sensores"`
	Operacao Operacao `json:"operacao"`
}

type Conexao struct {
	Estado         string `json:"estado"`
	SinalWifiDbm   int    `json:"sinal_wifi_dbm"`
	TempoLigadoSeg int    `json:"tempo_ligado_seg"`
}

type Sensores struct {
	Clima      Clima      `json:"clima"`
	Hidraulica Hidraulica `json:"hidraulica"`
}

type Clima struct {
	TemperaturaC float64 `json:"temperatura_c"`
	UmidadePct   float64 `json:"umidade_pct"`
	PressaoHpa   float64 `json:"pressao_hpa"`
}

type Hidraulica struct {
	FluxoDetectado bool    `json:"fluxo_detectado"`
	VazaoLpm       float64 `json:"vazao_lpm"`
}

type Operacao struct {
	ModoAtual    string   `json:"modo_atual"`
	SaidasAtivas []string `json:"saidas_ativas"`
}

type Controle struct {
	Telecomando struct {
		Irrigacao struct {
			Conjunto1 bool `json:"conjunto_1"`
			Conjunto2 bool `json:"conjunto_2"`
		} `json:"irrigacao"`
		Adubacao struct {
			Solucao1 struct{ Bag1, Bag2 bool } `json:"solucao_1"`
			Solucao2 struct{ Bag1, Bag2 bool } `json:"solucao_2"`
		} `json:"adubacao"`
	} `json:"telecomando"`
	Agendamento struct {
		Irrigacao struct {
			Conjunto1 string `json:"conjunto_1"`
			Conjunto2 string `json:"conjunto_2"`
		} `json:"irrigacao"`
		Adubacao struct {
			Sol1Bag1 string `json:"sol_1_bag_1"`
			Sol1Bag2 string `json:"sol_1_bag_2"`
			Sol2Bag1 string `json:"sol_2_bag_1"`
			Sol2Bag2 string `json:"sol_2_bag_2"`
		} `json:"adubacao"`
	} `json:"agendamento"`
}

type Seguranca struct {
	ParadaEmergencia bool `json:"parada_emergencia"`
	AlertaFalhaFluxo bool `json:"alerta_falha_fluxo"`
}

func main() {
	deviceID := flag.String("device", "esp32-001", "Device ID")
	interval := flag.Duration("interval", 5*time.Second, "Intervalo de publicação")
	brokerURL := flag.String("broker", "mqtt://localhost:1883", "URL do broker MQTT")
	scenario := flag.String("scenario", "normal", "Cenário: normal, offline, emergency, flowfail")
	flag.Parse()

	log.Printf("[SIM] Simulador ESP32 '%s' iniciado", *deviceID)
	log.Printf("[SIM] Broker: %s | Intervalo: %s | Cenário: %s", *brokerURL, *interval, *scenario)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Conectar ao broker MQTT
	serverURL, err := url.Parse(*brokerURL)
	if err != nil {
		log.Fatalf("[SIM] URL inválida: %v", err)
	}

	var cm *autopaho.ConnectionManager

	cliCfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{serverURL},
		KeepAlive:                     30,
		CleanStartOnInitialConnection: true,
		SessionExpiryInterval:         60,
		ConnectRetryDelay:             3 * time.Second,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, _ *paho.Connack) {
			log.Println("[SIM] Conectado ao broker MQTT")

			// Subscrever ao tópico de comandos para responder ACK
			cmdTopic := fmt.Sprintf("fertirriga/%s/comando", *deviceID)
			if _, err := cm.Subscribe(ctx, &paho.Subscribe{
				Subscriptions: []paho.SubscribeOptions{
					{Topic: cmdTopic, QoS: 1},
				},
			}); err != nil {
				log.Printf("[SIM] Erro ao subscrever %s: %v", cmdTopic, err)
			} else {
				log.Printf("[SIM] Subscrito em %s (auto-ACK habilitado)", cmdTopic)
			}
		},
		OnConnectError: func(err error) {
			log.Printf("[SIM] Erro de conexão MQTT: %v", err)
		},
		ClientConfig: paho.ClientConfig{
			ClientID: fmt.Sprintf("sim-%s-%d", *deviceID, time.Now().UnixMilli()%10000),
			Router: paho.NewStandardRouterWithDefault(func(p *paho.Publish) {
				// Recebeu comando → responder com ACK automático
				handleCommand(ctx, cm, *deviceID, p)
			}),
		},
	}

	cm, err = autopaho.NewConnection(ctx, cliCfg)
	if err != nil {
		log.Fatalf("[SIM] Falha ao criar conexão: %v", err)
	}

	// Aguardar conexão
	waitCtx, waitCancel := context.WithTimeout(ctx, 10*time.Second)
	defer waitCancel()
	if err := cm.AwaitConnection(waitCtx); err != nil {
		log.Printf("[SIM] Timeout de conexão MQTT — continuando sem MQTT: %v", err)
	}

	startTime := time.Now()
	tick := time.NewTicker(*interval)
	defer tick.Stop()

	baseTemp := 22.0 + rand.Float64()*5
	baseHumidity := 55.0 + rand.Float64()*20
	basePressure := 1013.25

	for {
		select {
		case <-sigCh:
			log.Println("[SIM] Desligando...")
			cm.Disconnect(ctx)
			return
		case <-tick.C:
			uptime := int(time.Since(startTime).Seconds())

			// Cenário: offline → para após 30s
			if *scenario == "offline" && uptime > 30 {
				log.Println("[SIM] 🔴 Cenário OFFLINE — parando publicação")
				<-sigCh
				return
			}

			// Simular variações senoidais nos sensores
			t := float64(uptime) / 300.0
			temp := baseTemp + 3*math.Sin(t) + rand.Float64()*0.5
			humidity := baseHumidity + 5*math.Cos(t*0.7) + rand.Float64()*1
			pressure := basePressure + 2*math.Sin(t*0.3) + rand.Float64()*0.2

			// Simular fluxo intermitente
			flowActive := uptime%120 < 60
			flowRate := 0.0
			if flowActive {
				flowRate = 2.5 + rand.Float64()*0.5
			}

			// Cenário: flowfail → irrigação ativa mas sem fluxo
			if *scenario == "flowfail" && uptime > 15 {
				flowActive = false
				flowRate = 0
			}

			payload := Payload{
				StatusSistema: StatusSistema{
					Conexao: Conexao{
						Estado:         "online",
						SinalWifiDbm:   -40 - rand.Intn(30),
						TempoLigadoSeg: uptime,
					},
					Sensores: Sensores{
						Clima: Clima{
							TemperaturaC: math.Round(temp*10) / 10,
							UmidadePct:   math.Round(humidity*10) / 10,
							PressaoHpa:   math.Round(pressure*10) / 10,
						},
						Hidraulica: Hidraulica{
							FluxoDetectado: flowActive,
							VazaoLpm:       math.Round(flowRate*10) / 10,
						},
					},
					Operacao: Operacao{
						ModoAtual:    "stand-by",
						SaidasAtivas: []string{},
					},
				},
				Seguranca: Seguranca{
					ParadaEmergencia: false,
					AlertaFalhaFluxo: false,
				},
			}

			payload.Controle.Agendamento.Irrigacao.Conjunto1 = "08:00"
			payload.Controle.Agendamento.Irrigacao.Conjunto2 = "08:30"
			payload.Controle.Agendamento.Adubacao.Sol1Bag1 = "09:00"
			payload.Controle.Agendamento.Adubacao.Sol1Bag2 = "09:30"
			payload.Controle.Agendamento.Adubacao.Sol2Bag1 = "10:00"
			payload.Controle.Agendamento.Adubacao.Sol2Bag2 = "10:30"

			if flowActive || *scenario == "flowfail" {
				payload.StatusSistema.Operacao.ModoAtual = "irrigacao agendada"
				payload.StatusSistema.Operacao.SaidasAtivas = []string{"irrigacao_conj1"}
			}

			// Cenário: emergency → ativa emergência após 20s
			if *scenario == "emergency" && uptime > 20 {
				payload.Seguranca.ParadaEmergencia = true
				payload.Seguranca.AlertaFalhaFluxo = true
				log.Println("[SIM] 🚨 Cenário EMERGÊNCIA — parada ativa")
			}

			// Cenário: flowfail log
			if *scenario == "flowfail" && uptime > 15 {
				payload.Seguranca.AlertaFalhaFluxo = true
			}

			data, _ := json.Marshal(payload)

			// Publicar via MQTT
			topic := fmt.Sprintf("fertirriga/%s/telemetria", *deviceID)
			if _, err := cm.Publish(ctx, &paho.Publish{
				Topic:   topic,
				QoS:     0,
				Payload: data,
			}); err != nil {
				log.Printf("[SIM] ⚠️  Erro ao publicar: %v", err)
			} else {
				log.Printf("[SIM] ✅ %s | T=%.1f°C H=%.1f%% | Fluxo=%v (%.1fL/m) | WiFi=%ddBm",
					*deviceID,
					payload.StatusSistema.Sensores.Clima.TemperaturaC,
					payload.StatusSistema.Sensores.Clima.UmidadePct,
					flowActive, flowRate,
					payload.StatusSistema.Conexao.SinalWifiDbm,
				)
			}
		}
	}
}

// handleCommand processa um comando recebido e responde com ACK automático.
func handleCommand(ctx context.Context, cm *autopaho.ConnectionManager, deviceID string, p *paho.Publish) {
	parts := strings.Split(p.Topic, "/")
	if len(parts) < 3 {
		return
	}

	log.Printf("[SIM] 📨 Comando recebido: %s", string(p.Payload))

	// Extrair command_id do payload
	var cmdPayload struct {
		CommandID     string `json:"command_id"`
		Action        string `json:"action"`
		TargetChannel string `json:"target_channel"`
	}
	if err := json.Unmarshal(p.Payload, &cmdPayload); err != nil {
		log.Printf("[SIM] ⚠️  Payload de comando inválido: %v", err)
		return
	}

	// Simular tempo de execução (200-800ms)
	time.Sleep(time.Duration(200+rand.Intn(600)) * time.Millisecond)

	// Publicar ACK
	ackPayload, _ := json.Marshal(map[string]interface{}{
		"command_id": cmdPayload.CommandID,
		"status":     "executed",
		"result": map[string]interface{}{
			"action":         cmdPayload.Action,
			"target_channel": cmdPayload.TargetChannel,
			"executed_at":    time.Now().UTC().Format(time.RFC3339),
			"success":        true,
		},
	})

	ackTopic := fmt.Sprintf("fertirriga/%s/ack", deviceID)
	if _, err := cm.Publish(ctx, &paho.Publish{
		Topic:   ackTopic,
		QoS:     1,
		Payload: ackPayload,
	}); err != nil {
		log.Printf("[SIM] ⚠️  Erro ao publicar ACK: %v", err)
	} else {
		log.Printf("[SIM] ✅ ACK enviado para comando %s (action=%s)", cmdPayload.CommandID, cmdPayload.Action)
	}
}
