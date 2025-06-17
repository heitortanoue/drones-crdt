# Dockerfile para Drone
FROM golang:1.24-alpine AS builder

# Instala dependências para build
RUN apk add --no-cache git curl

WORKDIR /app

# Copia go.mod e go.sum
COPY go.mod go.sum ./

# Download dependências
RUN go mod download

# Copia código fonte
COPY . .

# Build do binário
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o drone .

# Imagem final
FROM alpine:latest

# Instala ca-certificates para HTTPS e curl para healthcheck
RUN apk --no-cache add ca-certificates curl

WORKDIR /app

# Copia binário da fase de build
COPY --from=builder /app/drone /app/drone

# Expõe portas
EXPOSE 7000/udp
EXPOSE 8080/tcp

# Usuário não-root para segurança
RUN adduser -D -s /bin/sh droneuser
USER droneuser

# Comando padrão
CMD ["/app/drone"]
