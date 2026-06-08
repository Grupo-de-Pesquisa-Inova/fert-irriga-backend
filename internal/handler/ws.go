package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // CORS já é controlado pelo middleware Chi
	},
}

// WSClient representa uma conexão WebSocket ativa.
type WSClient struct {
	DeviceID string
	Conn     *websocket.Conn
	Send     chan []byte
}

// WSHub gerencia todas as conexões WebSocket e broadcast de telemetria.
type WSHub struct {
	mu         sync.RWMutex
	clients    map[string]map[*WSClient]bool // deviceID -> set de clients
	register   chan *WSClient
	unregister chan *WSClient
	broadcast  chan wsMessage
}

type wsMessage struct {
	DeviceID string
	Payload  []byte
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[string]map[*WSClient]bool),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		broadcast:  make(chan wsMessage, 256),
	}
}

// Run processa registros, desregistros e broadcasts. Deve rodar em goroutine.
func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.DeviceID] == nil {
				h.clients[client.DeviceID] = make(map[*WSClient]bool)
			}
			h.clients[client.DeviceID][client] = true
			h.mu.Unlock()
			slog.Info("WebSocket client conectado", "device", client.DeviceID)

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.DeviceID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.Send)
					if len(clients) == 0 {
						delete(h.clients, client.DeviceID)
					}
				}
			}
			h.mu.Unlock()
			slog.Info("WebSocket client desconectado", "device", client.DeviceID)

		case msg := <-h.broadcast:
			h.mu.RLock()
			clients := h.clients[msg.DeviceID]
			for client := range clients {
				select {
				case client.Send <- msg.Payload:
				default:
					// Client lento — desconectar
					close(client.Send)
					delete(clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast envia telemetria para todos os clients conectados a um device.
func (h *WSHub) Broadcast(deviceID string, payload []byte) {
	h.broadcast <- wsMessage{DeviceID: deviceID, Payload: payload}
}

// BroadcastGlobal envia um evento para TODOS os clients conectados (alarmes, status).
func (h *WSHub) BroadcastGlobal(payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, clients := range h.clients {
		for client := range clients {
			select {
			case client.Send <- payload:
			default:
			}
		}
	}
}

// BroadcastEvent envia um evento tipado como JSON para todos os clients.
func (h *WSHub) BroadcastEvent(eventType string, data interface{}) {
	eventJSON, _ := json.Marshal(map[string]interface{}{
		"type": eventType,
		"data": data,
	})
	h.BroadcastGlobal(eventJSON)
}

// HandleConnection faz upgrade HTTP → WebSocket e gerencia a conexão.
func (h *WSHub) HandleConnection(w http.ResponseWriter, r *http.Request, deviceID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("erro ao fazer upgrade WebSocket", "error", err)
		return
	}

	client := &WSClient{
		DeviceID: deviceID,
		Conn:     conn,
		Send:     make(chan []byte, 64),
	}

	h.register <- client

	// Writer goroutine — envia mensagens para o client
	go func() {
		defer func() {
			conn.Close()
			h.unregister <- client
		}()
		for msg := range client.Send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	// Reader goroutine — mantém conexão viva e detecta desconexão
	go func() {
		defer func() {
			h.unregister <- client
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}
