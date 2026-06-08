package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config contém toda a configuração da aplicação, carregada de variáveis de ambiente.
type Config struct {
	Env         string
	Port        int
	DatabaseURL string
	MQTT        MQTTConfig
	CORSOrigins []string
}

// IsProduction indica se a aplicação roda em ambiente de produção.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.Env, "production") || strings.EqualFold(c.Env, "prod")
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

	cfg := &Config{
		Env:         getEnv("APP_ENV", "development"),
		Port:        port,
		DatabaseURL: dbURL,
		MQTT: MQTTConfig{
			BrokerURL: getEnv("MQTT_BROKER_URL", "mqtt://localhost:1883"),
			ClientID:  getEnv("MQTT_CLIENT_ID", "fertirriga-backend"),
			Username:  getEnv("MQTT_USERNAME", ""),
			Password:  getEnv("MQTT_PASSWORD", ""),
		},
		CORSOrigins: corsOrigins,
	}

	if err := cfg.validateForProduction(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validateForProduction recusa configurações inseguras quando APP_ENV=production,
// impedindo que credenciais e padrões de desenvolvimento cheguem a produção.
func (c *Config) validateForProduction() error {
	if !c.IsProduction() {
		return nil
	}

	// Credenciais triviais de desenvolvimento não podem ser usadas em produção.
	if strings.Contains(c.DatabaseURL, "fertirriga:fertirriga") {
		return fmt.Errorf("DATABASE_URL usa credenciais padrão de desenvolvimento; defina credenciais fortes em produção")
	}

	// O broker MQTT deve exigir autenticação em produção (ele fica exposto à internet
	// para o ESP32, ao contrário do banco, que normalmente vive em rede interna).
	if c.MQTT.Username == "" || c.MQTT.Password == "" {
		return fmt.Errorf("MQTT_USERNAME e MQTT_PASSWORD são obrigatórios em produção")
	}

	// CORS não pode liberar origens de desenvolvimento em produção.
	for _, o := range c.CORSOrigins {
		if strings.Contains(o, "localhost") || strings.TrimSpace(o) == "*" {
			return fmt.Errorf("CORS_ORIGIN não pode conter localhost ou '*' em produção: %q", o)
		}
	}

	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
