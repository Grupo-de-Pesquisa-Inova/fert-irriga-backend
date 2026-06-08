FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /fertirriga-backend ./cmd/server

# ---

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata
ENV TZ=America/Sao_Paulo

COPY --from=builder /fertirriga-backend /fertirriga-backend

EXPOSE 8080

ENTRYPOINT ["/fertirriga-backend"]
