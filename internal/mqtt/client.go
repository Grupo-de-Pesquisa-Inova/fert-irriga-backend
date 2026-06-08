package mqttclient

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/autopaho"
	"github.com/eclipse/paho.golang/paho"

	"fertirriga-backend/internal/config"
)

// TelemetryHandler é chamado quando uma mensagem de telemetria chega via MQTT.
type TelemetryHandler func(deviceID string, payload []byte)

// EmergencyHandler é chamado quando uma parada de emergência é recebida.
type EmergencyHandler func(deviceID string, payload []byte)

// AckHandler é chamado quando um ACK de comando é recebido do ESP32.
type AckHandler func(deviceID string, payload []byte)

// Client encapsula o Eclipse Paho MQTT5 com autopaho para reconexão automática.
type Client struct {
	cm                *autopaho.ConnectionManager
	mu                sync.RWMutex
	telemetryHandlers []TelemetryHandler
	emergencyHandlers []EmergencyHandler
	ackHandlers       []AckHandler
}

// NewClient cria e conecta um client MQTT5 ao broker Mosquitto.
func NewClient(ctx context.Context, cfg config.MQTTConfig) (*Client, error) {
	brokerURL, err := url.Parse(cfg.BrokerURL)
	if err != nil {
		return nil, fmt.Errorf("URL MQTT inválida: %w", err)
	}

	c := &Client{}

	cliCfg := autopaho.ClientConfig{
		ServerUrls:                    []*url.URL{brokerURL},
		KeepAlive:                     30,
		CleanStartOnInitialConnection: true,
		SessionExpiryInterval:         60,
		ConnectRetryDelay:             5 * time.Second,
		OnConnectionUp: func(cm *autopaho.ConnectionManager, connAck *paho.Connack) {
			slog.Info("MQTT5 conectado ao broker", "url", cfg.BrokerURL)

			// Subscrever tópicos FertIrriga
			if _, err := cm.Subscribe(ctx, &paho.Subscribe{
				Subscriptions: []paho.SubscribeOptions{
					{Topic: "fertirriga/+/telemetria", QoS: 0},
					{Topic: "fertirriga/+/status", QoS: 1},
					{Topic: "fertirriga/+/emergencia", QoS: 2},
					{Topic: "fertirriga/+/ack", QoS: 1},
				},
			}); err != nil {
				slog.Error("falha ao subscrever tópicos MQTT", "error", err)
			} else {
				slog.Info("tópicos MQTT subscritos com sucesso")
			}
		},
		OnConnectError: func(err error) {
			slog.Warn("erro de conexão MQTT (reconectando...)", "error", err)
		},
		ClientConfig: paho.ClientConfig{
			ClientID: cfg.ClientID,
			Router: paho.NewStandardRouterWithDefault(func(p *paho.Publish) {
				c.handleMessage(p)
			}),
		},
	}

	if cfg.Username != "" {
		cliCfg.ConnectUsername = cfg.Username
		cliCfg.ConnectPassword = []byte(cfg.Password)
	}

	cm, err := autopaho.NewConnection(ctx, cliCfg)
	if err != nil {
		return nil, fmt.Errorf("falha ao criar conexão MQTT: %w", err)
	}
	c.cm = cm

	// Aguardar conexão inicial (não-bloqueante se broker indisponível)
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := cm.AwaitConnection(waitCtx); err != nil {
		slog.Warn("timeout aguardando conexão MQTT inicial — continuando sem MQTT", "error", err)
	}

	return c, nil
}

// OnTelemetry registra um handler para mensagens de telemetria.
func (c *Client) OnTelemetry(handler TelemetryHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.telemetryHandlers = append(c.telemetryHandlers, handler)
}

// OnEmergency registra um handler para parada de emergência.
func (c *Client) OnEmergency(handler EmergencyHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.emergencyHandlers = append(c.emergencyHandlers, handler)
}

// OnAck registra um handler para ACK de comandos.
func (c *Client) OnAck(handler AckHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ackHandlers = append(c.ackHandlers, handler)
}

// PublishCommand envia um comando para um device via MQTT5.
func (c *Client) PublishCommand(ctx context.Context, deviceID string, commandType string, payload []byte) error {
	topic := fmt.Sprintf("fertirriga/%s/%s", deviceID, commandType)

	qos := byte(1)
	if commandType == "emergencia" {
		qos = 2
	}

	_, err := c.cm.Publish(ctx, &paho.Publish{
		Topic:   topic,
		QoS:     qos,
		Payload: payload,
	})

	if err != nil {
		return fmt.Errorf("erro ao publicar MQTT [%s]: %w", topic, err)
	}

	slog.Info("comando MQTT publicado", "topic", topic, "qos", qos)
	return nil
}

// Disconnect encerra a conexão MQTT graciosamente.
func (c *Client) Disconnect(ctx context.Context) error {
	return c.cm.Disconnect(ctx)
}

// handleMessage roteia mensagens recebidas para os handlers registrados.
func (c *Client) handleMessage(p *paho.Publish) {
	parts := strings.Split(p.Topic, "/")
	if len(parts) < 3 {
		slog.Warn("tópico MQTT inesperado", "topic", p.Topic)
		return
	}

	deviceID := parts[1]
	msgType := parts[2]

	slog.Debug("mensagem MQTT recebida", "topic", p.Topic, "device", deviceID, "type", msgType)

	c.mu.RLock()
	defer c.mu.RUnlock()

	switch msgType {
	case "telemetria", "status":
		for _, h := range c.telemetryHandlers {
			h(deviceID, p.Payload)
		}
	case "emergencia":
		for _, h := range c.emergencyHandlers {
			h(deviceID, p.Payload)
		}
	case "ack":
		for _, h := range c.ackHandlers {
			h(deviceID, p.Payload)
		}
	}
}
