# ── Estágio 1: build ──────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Migrations são embutidas no binário via //go:embed — não precisam ser copiadas.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /fertirriga-backend ./cmd/server

# ── Estágio 2: runtime ────────────────────────────────────────
FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata wget \
    && addgroup -S app && adduser -S -G app app
ENV TZ=America/Sao_Paulo

COPY --from=builder /fertirriga-backend /usr/local/bin/fertirriga-backend

USER app
EXPOSE 8080

# Healthcheck para Dokploy/Traefik
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- http://localhost:8080/api/v1/health || exit 1

ENTRYPOINT ["fertirriga-backend"]
