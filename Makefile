.PHONY: dev build test docker-up docker-down backup tidy clean

# Desenvolvimento local
dev:
	go run ./cmd/server

# Build binário de produção
build:
	CGO_ENABLED=0 go build -o fertirriga-backend ./cmd/server

# Testes
test:
	go test ./... -v -race

# Docker
docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

# Backup PostgreSQL via Docker
backup:
	@mkdir -p backups
	docker exec fertirriga-db pg_dump -U fertirriga fertirriga > backups/backup_$(shell powershell -Command "Get-Date -Format 'yyyy-MM-dd_HH-mm-ss'").sql
	@echo "Backup criado em backups/"

# Dependências
tidy:
	go mod tidy

# Limpar binários
clean:
	@if exist fertirriga-backend del fertirriga-backend
	@if exist fertirriga-backend.exe del fertirriga-backend.exe
