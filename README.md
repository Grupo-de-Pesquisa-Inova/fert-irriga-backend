# FertIrriga Backend

Backend Go para o sistema de fertirrigação automatizada **FertIrriga Edge**.

## Stack

| Camada | Tecnologia |
|---|---|
| Linguagem | Go 1.25 |
| HTTP Router | Chi v5 |
| WebSocket | gorilla/websocket |
| MQTT5 Client | Eclipse Paho (autopaho) |
| Banco de Dados | PostgreSQL 16 |
| MQTT Broker | Mosquitto 2 |
| Scheduler | robfig/cron v3 |
| Containers | Docker Compose |

## Requisitos

- Go 1.25+
- Docker + Docker Compose

## Setup Rápido

```bash
# 1. Copiar variáveis de ambiente
cp .env.example .env

# 2. Subir PostgreSQL + Mosquitto
make docker-up

# 3. Rodar o backend (migrações executam automaticamente)
make dev
```

## Endpoints

| Método | Rota | Descrição |
|---|---|---|
| GET | `/api/v1/health` | Health check |
| GET | `/api/v1/devices` | Listar dispositivos |
| GET | `/api/v1/devices/{id}` | Detalhes do dispositivo |
| POST | `/api/v1/devices/{id}/control` | Enviar comando de controle |
| POST | `/api/v1/devices/{id}/emergency-stop` | Parada de emergência |
| GET | `/api/v1/devices/{id}/telemetry` | Histórico de telemetria |
| GET | `/api/v1/devices/{id}/telemetry/latest` | Últimas leituras |
| WS | `/ws/{id}` | Stream de telemetria em tempo real |

## Tópicos MQTT

| Tópico | QoS | Direção |
|---|---|---|
| `fertirriga/{id}/telemetria` | 0 | ESP32 → Backend |
| `fertirriga/{id}/status` | 1 | ESP32 → Backend |
| `fertirriga/{id}/comando` | 1 | Backend → ESP32 |
| `fertirriga/{id}/emergencia` | 2 | Bidirecional |

## Backup

```bash
make backup
```

Backups são salvos em `backups/` (últimos 30 mantidos automaticamente).

## Segurança

> ⚠️ **As credenciais padrão deste repositório são apenas para desenvolvimento local.**

- **Validação de produção:** definindo `APP_ENV=production`, o backend **recusa iniciar** com configurações inseguras — credenciais padrão do banco (`fertirriga:fertirriga`), `sslmode=disable`, MQTT sem usuário/senha ou CORS com `localhost`/`*`. Veja `internal/config/config.go`.
- **Banco de dados:** o `.env.example` e o `docker-compose.yml` usam credenciais triviais destinadas **exclusivamente a ambiente local**. Em produção, defina segredos fortes via variáveis de ambiente e **nunca** versione o arquivo `.env`.
- **Usuário admin:** o `scripts/seed.sql` **não** cria mais um admin com senha padrão. Forneça um hash bcrypt de uma senha forte via `psql -v admin_password_hash=...` (instruções no próprio arquivo).
- **MQTT:** o broker de desenvolvimento aceita conexões anônimas. Para produção, use `docker/mosquitto/mosquitto.prod.conf.example` como base — ele habilita autenticação (`password_file`) e TLS (porta 8883). Aponte o backend para `mqtts://` e defina `MQTT_USERNAME`/`MQTT_PASSWORD`.

## Estrutura

```
Backend/
├── cmd/server/main.go         # Entry point
├── internal/
│   ├── config/                # Configuração por env vars
│   ├── domain/                # Tipos de domínio (espelhando TS)
│   ├── handler/               # HTTP handlers + WebSocket
│   ├── mqtt/                  # Client MQTT5 (Eclipse Paho)
│   ├── repository/            # PostgreSQL (pgx)
│   ├── scheduler/             # Cron jobs
│   └── service/               # Lógica de negócio
├── docker-compose.yml         # PostgreSQL + Mosquitto
├── Dockerfile                 # Build multi-stage
├── Makefile                   # Comandos de desenvolvimento
└── scripts/backup.ps1         # Backup automatizado
```

## Licença

[MIT](./LICENSE) © Grupo de Pesquisa Inova
