package domain

import "time"

// PayloadESP32 espelha a interface IPayloadESP32 do frontend TypeScript.
type PayloadESP32 struct {
	StatusSistema SystemStatus   `json:"status_sistema"`
	Controle      ControlStatus  `json:"controle"`
	Seguranca     SecurityStatus `json:"seguranca"`
}

type SystemStatus struct {
	Conexao  ConnectionStatus `json:"conexao"`
	Sensores SensorData       `json:"sensores"`
	Operacao OperationStatus  `json:"operacao"`
}

type ConnectionStatus struct {
	Estado         string `json:"estado"`
	SinalWifiDBM   int    `json:"sinal_wifi_dbm"`
	TempoLigadoSeg int64  `json:"tempo_ligado_seg"`
}

type SensorData struct {
	Clima      ClimateData   `json:"clima"`
	Hidraulica HydraulicData `json:"hidraulica"`
}

type ClimateData struct {
	TemperaturaC float64 `json:"temperatura_c"`
	UmidadePct   float64 `json:"umidade_pct"`
	PressaoHPA   float64 `json:"pressao_hpa"`
}

type HydraulicData struct {
	FluxoDetectado bool    `json:"fluxo_detectado"`
	VazaoLPM       float64 `json:"vazao_lpm"`
}

type OperationStatus struct {
	ModoAtual    string   `json:"modo_atual"`
	SaidasAtivas []string `json:"saidas_ativas"`
}

type ControlStatus struct {
	Telecomando RemoteControl `json:"telecomando"`
	Agendamento ESP32Schedule `json:"agendamento"`
}

type RemoteControl struct {
	Irrigacao IrrigationControl    `json:"irrigacao"`
	Adubacao  FertilizationControl `json:"adubacao"`
}

type IrrigationControl struct {
	Conjunto1 bool `json:"conjunto_1"`
	Conjunto2 bool `json:"conjunto_2"`
}

type FertilizationControl struct {
	Solucao1 BagControl `json:"solucao_1"`
	Solucao2 BagControl `json:"solucao_2"`
}

type BagControl struct {
	Bag1 bool `json:"bag_1"`
	Bag2 bool `json:"bag_2"`
}

type ESP32Schedule struct {
	Irrigacao IrrigationSchedule    `json:"irrigacao"`
	Adubacao  FertilizationSchedule `json:"adubacao"`
}

type IrrigationSchedule struct {
	Conjunto1 string `json:"conjunto_1"`
	Conjunto2 string `json:"conjunto_2"`
}

type FertilizationSchedule struct {
	Sol1Bag1 string `json:"sol_1_bag_1"`
	Sol1Bag2 string `json:"sol_1_bag_2"`
	Sol2Bag1 string `json:"sol_2_bag_1"`
	Sol2Bag2 string `json:"sol_2_bag_2"`
}

type SecurityStatus struct {
	ParadaEmergencia bool `json:"parada_emergencia"`
	AlertaFalhaFluxo bool `json:"alerta_falha_fluxo"`
}

// Device representa um ESP32 registrado no sistema.
type Device struct {
	ID         string       `json:"id"`
	DeviceID   string       `json:"device_id"`
	Name       string       `json:"name"`
	Payload    PayloadESP32 `json:"payload"`
	IsOnline   bool         `json:"is_online"`
	LastSeenAt *time.Time   `json:"last_seen_at,omitempty"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

// TelemetryRecord é um ponto de dados de sensor para persistência.
type TelemetryRecord struct {
	ID             int64     `json:"id"`
	DeviceID       string    `json:"device_id"`
	TemperaturaC   float64   `json:"temperatura_c"`
	UmidadePct     float64   `json:"umidade_pct"`
	PressaoHPA     float64   `json:"pressao_hpa"`
	FluxoDetectado bool      `json:"fluxo_detectado"`
	VazaoLPM       float64   `json:"vazao_lpm"`
	SinalWifiDBM   int       `json:"sinal_wifi_dbm"`
	RecordedAt     time.Time `json:"recorded_at"`
}

// CommandLog registra comandos enviados para auditoria.
type CommandLog struct {
	ID          int64      `json:"id"`
	DeviceID    string     `json:"device_id"`
	CommandType string     `json:"command_type"`
	Payload     string     `json:"payload"`
	Status      string     `json:"status"`
	SentAt      time.Time  `json:"sent_at"`
	AckedAt     *time.Time `json:"acked_at,omitempty"`
}
