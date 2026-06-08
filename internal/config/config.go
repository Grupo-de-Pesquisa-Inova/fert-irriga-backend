package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config contém toda a configuração da aplicação, carregada de variáveis de ambiente.
type Config struct {
	Port        int
	DatabaseURL string
	MQTT        MQTTConfig
	CORSOrigins []string
}

// MQTTConfig define a configuração do client MQTT5.
type MQTTConfig struct {
	BrokerURL string
	ClientID  string
	Username  string
	Password  string
}

// Load carrega a configuração de variáveis de ambiente com defaults sensatos para dev.
func Load() (*Config, error) {
	port, err := strconv.Atoi(getEnv("PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("PORT inválida: %w", err)
	}

	dbURL := getEnv("DATABASE_URL", "postgres://fertirriga:fertirriga@localhost:5432/fertirriga?sslmode=disable")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL é obrigatória")
	}

	corsOrigins := strings.Split(getEnv("CORS_ORIGIN", "http://localhost:5173"), ",")

	return &Config{
		Port:        port,
		DatabaseURL: dbURL,
		MQTT: MQTTConfig{
			BrokerURL: getEnv("MQTT_BROKER_URL", "mqtt://localhost:1883"),
			ClientID:  getEnv("MQTT_CLIENT_ID", "fertirriga-backend"),
			Username:  getEnv("MQTT_USERNAME", ""),
			Password:  getEnv("MQTT_PASSWORD", ""),
		},
		CORSOrigins: corsOrigins,
	}, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
