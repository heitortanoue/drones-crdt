#!/bin/bash

# Script de demonstração do sistema CRDT para drones
# Inicia uma rede de 3 drones e demonstra sincronização

echo "=== Sistema CRDT para Drones - Demonstração ==="
echo ""

# Verifica se go está disponível
if ! command -v go &> /dev/null; then
    echo "Erro: Go não encontrado. Instale Go primeiro."
    exit 1
fi

# Compila o projeto
echo "1. Compilando o projeto..."
go build -o drone-server main.go
if [ $? -ne 0 ]; then
    echo "Erro na compilação!"
    exit 1
fi
echo "✅ Compilação bem-sucedida"
echo ""

# Função para matar processos ao final
cleanup() {
    echo ""
    echo "🛑 Parando drones..."
    kill $DRONE1_PID $DRONE2_PID $DRONE3_PID 2>/dev/null
    rm -f drone-server
    exit 0
}
trap cleanup SIGINT SIGTERM

# Inicia os 3 drones
echo "2. Iniciando rede de drones..."

./drone-server -drone=drone-01 -port=8080 \
  -peers="http://localhost:8081,http://localhost:8082" -gossip=3 &
DRONE1_PID=$!

./drone-server -drone=drone-02 -port=8081 \
  -peers="http://localhost:8080,http://localhost:8082" -gossip=3 &
DRONE2_PID=$!

./drone-server -drone=drone-03 -port=8082 \
  -peers="http://localhost:8080,http://localhost:8081" -gossip=3 &
DRONE3_PID=$!

echo "✅ Drones iniciados:"
echo "   - Drone-01: http://localhost:8080"
echo "   - Drone-02: http://localhost:8081"
echo "   - Drone-03: http://localhost:8082"
echo ""

# Aguarda os drones iniciarem
echo "3. Aguardando drones iniciarem..."
sleep 3

# Função para enviar leitura de sensor
send_reading() {
    local port=$1
    local sensor=$2
    local value=$3
    local timestamp=$(date +%s)000

    curl -s -X POST "http://localhost:$port/sensor" \
      -H 'Content-Type: application/json' \
      -d "{\"sensor_id\":\"$sensor\",\"timestamp\":$timestamp,\"value\":$value}" \
      | jq -r '.delta | "\(.drone_id) -> \(.sensor_id) = \(.value)"' 2>/dev/null || echo "Leitura enviada"
}

# Função para verificar estado
check_state() {
    local port=$1
    local drone_name=$2

    local count=$(curl -s "http://localhost:$port/state" | jq '.state | length' 2>/dev/null || echo "0")
    echo "   $drone_name: $count leituras"
}

echo "4. Enviando leituras de sensores..."

# Envia algumas leituras
echo "📡 Drone-01 coletando dados do talhao-1:"
send_reading 8080 "talhao-1" 22.5

echo "📡 Drone-02 coletando dados do talhao-2:"
send_reading 8081 "talhao-2" 18.3

echo "📡 Drone-03 coletando dados do talhao-3:"
send_reading 8082 "talhao-3" 25.1

echo "📡 Drone-01 coletando dados do talhao-4:"
send_reading 8080 "talhao-4" 21.8

sleep 2

echo "📡 Drone-02 atualizando talhao-1:"
send_reading 8081 "talhao-1" 23.0

echo ""

# Aguarda sincronização via gossip
echo "5. Aguardando sincronização via gossip (10s)..."
for i in {10..1}; do
    echo -ne "   Aguardando $i segundos...\r"
    sleep 1
done
echo "                                    "

# Verifica estados finais
echo "6. Verificando convergência:"
check_state 8080 "Drone-01"
check_state 8081 "Drone-02"
check_state 8082 "Drone-03"

echo ""
echo "7. Estados detalhados dos drones:"

for port in 8080 8081 8082; do
    echo ""
    echo "🤖 Drone na porta $port:"
    curl -s "http://localhost:$port/state" | jq -r '.state[] | "  \(.sensor_id): \(.value)% (ts: \(.timestamp), drone: \(.drone_id))"' 2>/dev/null || echo "  Erro ao buscar estado"
done

echo ""
echo "8. Testando tolerância a falhas..."
echo "🔥 Simulando falha temporária do Drone-02..."
kill $DRONE2_PID 2>/dev/null

sleep 2

echo "📡 Drones restantes continuam operando:"
send_reading 8080 "talhao-5" 19.5
send_reading 8082 "talhao-6" 26.8

echo ""
echo "💡 Pressione Ctrl+C para parar a demonstração"
echo "📊 Monitor em tempo real disponível em:"
echo "   - http://localhost:8080/state"
echo "   - http://localhost:8081/state"
echo "   - http://localhost:8082/state"

# Aguarda indefinidamente
while true; do
    sleep 1
done
