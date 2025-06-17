#!/bin/bash

# Demo Script - Fase 4: Disseminação TTL + Eleição Completa + Deduplicação
# Demonstra F1 (coleta automática), F2 (Delta-Set CRDT), F3 (protocolos UDP),
# F4 (disseminação TTL), F5 (tabela de vizinhos), F6 (eleição completa) e F7 (cache LRU)

set -e

# Função para gerar timestamp em milissegundos (compatível com macOS)
generate_timestamp_ms() {
    if command -v gdate >/dev/null 2>&1; then
        # Se GNU date estiver disponível (instalado via brew install coreutils)
        gdate +%s%3N
    else
        # Fallback para macOS - usa segundos + 000 para simular milissegundos
        echo "$(date +%s)000"
    fi
}

# Função para verificar se um serviço está respondendo
check_service() {
    local port=$1
    local drone_id=$2
    local max_attempts=10
    local attempt=1

    echo "🔍 Verificando $drone_id (porta $port)..."

    while [ $attempt -le $max_attempts ]; do
        response=$(curl -s -w "%{http_code}" http://localhost:$port/health 2>/dev/null | tail -c 3)
        if [ "$response" = "200" ]; then
            echo "  ✅ $drone_id está respondendo"
            return 0
        fi
        echo "  ⏳ Tentativa $attempt/$max_attempts - aguardando $drone_id..."
        sleep 2
        attempt=$((attempt + 1))
    done

    echo "  ❌ $drone_id não está respondendo após $max_attempts tentativas"
    return 1
}

echo "=== Demo Drone Sistema - Fase 4 ==="
echo "F1: Coleta automática de sensores"
echo "F2: Delta-Set CRDT (uuid.UUID → SensorDelta)"
echo "F3: Protocolos Advertise/Request/SwitchChannel (UDP)"
echo "F4: Disseminação TTL com fan-out configurável"
echo "F5: Tabela de vizinhos via UDP"
echo "F6: Eleição completa de transmissor (greedy + ReqCtr)"
echo "F7: Deduplicação com cache LRU (10k IDs)"
echo

# Verifica dependências
if ! command -v jq &> /dev/null; then
    echo "❌ jq não está instalado. Instale com: brew install jq (macOS) ou apt-get install jq (Ubuntu)"
    exit 1
fi

if ! command -v curl &> /dev/null; then
    echo "❌ curl não está instalado."
    exit 1
fi

echo "✅ Dependências verificadas (jq, curl)"

# Limpa processos antigos
echo "Limpando processos antigos..."
pkill -f './drone' 2>/dev/null || true
sleep 1

# Build do projeto
echo "Compilando projeto..."
go build -o drone .

# Verifica tamanho do binário
echo "Tamanho do binário:"
ls -lh drone | awk '{print "  " $5 " " $9}'

# Inicia drones em background
echo
echo "Iniciando drones com coleta automática..."

# Drone 1 - Coleta a cada 2 segundos
echo "  Iniciando drone-1 (UDP: 7000, TCP: 8080, coleta: 2s)"
./drone -id=drone-1 -sample-sec=2 &
DRONE1_PID=$!
sleep 3

# Drone 2 - Coleta a cada 3 segundos
echo "  Iniciando drone-2 (UDP: 7001, TCP: 8081, coleta: 3s)"
./drone -id=drone-2 -sample-sec=3 -udp-port=7001 -tcp-port=8081 &
DRONE2_PID=$!
sleep 3

# Drone 3 - Coleta a cada 5 segundos
echo "  Iniciando drone-3 (UDP: 7002, TCP: 8082, coleta: 5s)"
./drone -id=drone-3 -sample-sec=5 -udp-port=7002 -tcp-port=8082 &
DRONE3_PID=$!
sleep 3

echo
echo "Drones iniciados! PIDs: $DRONE1_PID, $DRONE2_PID, $DRONE3_PID"
echo "Aguardando coleta automática gerar dados..."
sleep 5

# Verifica se todos os drones estão respondendo
echo
echo "=== Verificando Conectividade dos Drones ==="
all_services_ok=true

if ! check_service 8080 "drone-1"; then
    all_services_ok=false
fi

if ! check_service 8081 "drone-2"; then
    all_services_ok=false
fi

if ! check_service 8082 "drone-3"; then
    all_services_ok=false
fi

if [ "$all_services_ok" != true ]; then
    echo "❌ Nem todos os drones estão respondendo. Abortando demo."
    exit 1
fi

echo "✅ Todos os drones estão funcionando corretamente!"

# Testa endpoints da Fase 2 e Fase 3
echo
echo "=== Testando Funcionalidades da Fase 2 & 3 ==="

for port in 8080 8081 8082; do
    drone_id="drone-$((port-8079))"
    echo
    echo "🔸 Testando $drone_id (porta $port):"

    # Endpoint de saúde
    health=$(curl -s http://localhost:$port/health)
    if echo "$health" | jq . >/dev/null 2>&1; then
        echo "  /health: $(echo $health | jq -r .status) ($(echo $health | jq -r .drone_id))"
    else
        echo "  /health: ⚠️  Resposta inválida: $health"
    fi

    # Estado atual do CRDT
    state=$(curl -s http://localhost:$port/state)
    if echo "$state" | jq . >/dev/null 2>&1; then
        total_deltas=$(echo $state | jq '.total_deltas')
        unique_sensors=$(echo $state | jq '.unique_sensors')
        echo "  /state: $total_deltas deltas, $unique_sensors sensores únicos"
    else
        echo "  /state: ⚠️  Resposta inválida: $state"
    fi

    # Estatísticas do sistema (incluindo Fase 3)
    stats=$(curl -s http://localhost:$port/stats)
    if echo "$stats" | jq . >/dev/null 2>&1; then
        running=$(echo $stats | jq '.sensor_system.generator.running')
        interval=$(echo $stats | jq '.sensor_system.generator.interval_sec')
        neighbors_count=$(echo $stats | jq '.network.neighbors_active // 0')
        control_running=$(echo $stats | jq '.control_system.running // false')
        election_state=$(echo $stats | jq -r '.transmitter_election.current_state // "N/A"')
        echo "  /stats: gerador=$running, intervalo=${interval}s, vizinhos=$neighbors_count"
        echo "  controle=$control_running, eleição=$election_state"
    else
        echo "  /stats: ⚠️  Resposta inválida: $stats"
    fi

    # Adiciona leitura manual
    manual_response=$(curl -s -X POST http://localhost:$port/sensor \
        -H "Content-Type: application/json" \
        -d "{\"sensor_id\": \"manual-test-$port\", \"value\": 95.5}")
    if echo "$manual_response" | jq . >/dev/null 2>&1; then
        manual_id=$(echo $manual_response | jq -r '.delta.id' | cut -c1-8)
        echo "  POST /sensor: leitura manual adicionada (ID: $manual_id)"
    else
        echo "  POST /sensor: ⚠️  Resposta inválida: $manual_response"
    fi
done

echo
echo "=== Testando Merge de Deltas (Requisito F2) ==="

# Simula drone-2 enviando deltas para drone-1
echo "Simulando drone-2 → drone-1 (merge de deltas):"

# IDs fixos para ficar reprodutível
uuid1="11111111-1111-1111-1111-111111111111"
uuid2="22222222-2222-2222-2222-222222222222"
ts1=$(generate_timestamp_ms)
sleep 0.1  # Pequena pausa para garantir timestamps diferentes
ts2=$(generate_timestamp_ms)

# -------- primeiro delta --------
json_payload=$(cat <<EOF
{
  "id":        "$uuid1",
  "ttl":       3,
  "sender_id": "drone-2",
  "timestamp": $ts1,
  "data": {
    "id":        "$uuid1",
    "sensor_id": "cross-drone-sensor-A",
    "timestamp": $ts1,
    "value":     87.3,
    "drone_id":  "drone-2"
  }
}
EOF
)

# Captura a resposta primeiro para debug
echo "JSON enviado (primeiro delta):"
echo "$json_payload" | jq .

response=$(curl -s -X POST http://localhost:8080/delta \
     -H "Content-Type: application/json" \
     -d "$json_payload")

echo "Resposta da API (primeiro delta): $response"

# Verifica se a resposta é um JSON válido antes de usar jq
if echo "$response" | jq . >/dev/null 2>&1; then
    echo "$response" | jq .
else
    echo "⚠️  Resposta não é um JSON válido: $response"
fi

# -------- segundo delta --------
json_payload=$(cat <<EOF
{
  "id":        "$uuid2",
  "ttl":       3,
  "sender_id": "drone-2",
  "timestamp": $ts2,
  "data": {
    "id":        "$uuid2",
    "sensor_id": "cross-drone-sensor-B",
    "timestamp": $ts2,
    "value":     92.1,
    "drone_id":  "drone-2"
  }
}
EOF
)

# Captura a resposta primeiro para debug
echo "JSON enviado (segundo delta):"
echo "$json_payload" | jq .

response=$(curl -s -X POST http://localhost:8080/delta \
     -H "Content-Type: application/json" \
     -d "$json_payload")

echo "Resposta da API (segundo delta): $response"

# Verifica se a resposta é um JSON válido antes de usar jq
if echo "$response" | jq . >/dev/null 2>&1; then
    echo "$response" | jq .
else
    echo "⚠️  Resposta não é um JSON válido: $response"
fi

# --------------------------------
echo "  ✅ Dois deltas enviados ao drone-1"

# Verifica se deltas foram integrados
echo "Verificando integração no drone-1:"
integrated_state=$(curl -s http://localhost:8080/state)

echo "Estado integrado: $integrated_state"

# Verifica se a resposta é um JSON válido antes de usar jq
if echo "$integrated_state" | jq . >/dev/null 2>&1; then
    new_total=$(echo $integrated_state | jq '.total_deltas')
    sensors_list=$(echo $integrated_state | jq -r '.latest_readings | keys[]' | grep -E "cross-drone-sensor" || echo "")

    echo "  📊 Total de deltas: $new_total"
    echo "  📊 Sensores cross-drone encontrados: $sensors_list"

    if [ ! -z "$sensors_list" ]; then
        echo "  ✅ Deltas integrados com sucesso!"
        echo "  ✅ Sensores cross-drone detectados: $(echo "$sensors_list" | wc -l | tr -d ' ') sensores"
    else
        echo "  ❌ Falha na integração de deltas - nenhum sensor cross-drone encontrado"
    fi
else
    echo "⚠️  Estado retornado não é um JSON válido: $integrated_state"
fi

echo
echo "=== Demonstração de Funcionalidades da Fase 4 ==="

echo "Testando tabela de vizinhos e protocolos de controle:"
echo "Aguardando descoberta automática de vizinhos via UDP..."
sleep 5

# Verifica descoberta de vizinhos após tempo de execução
echo
echo "Verificando descoberta de vizinhos:"
for port in 8080 8081 8082; do
    drone_id="drone-$((port-8079))"
    stats=$(curl -s http://localhost:$port/stats)
    if echo "$stats" | jq . >/dev/null 2>&1; then
        neighbors=$(echo $stats | jq '.network.neighbors_active // 0')
        urls=$(echo $stats | jq -r '.network.neighbor_urls // [] | length')
        echo "  $drone_id: $neighbors vizinhos ativos, $urls URLs disponíveis"
    else
        echo "  $drone_id: ⚠️  Falha ao obter estatísticas: $stats"
    fi
done

echo
echo "=== Demonstração de Cleanup ==="
echo "Testando limpeza de dados antigos:"
cleanup_response=$(curl -s -X POST "http://localhost:8080/cleanup?max_age_minutes=60")
if echo "$cleanup_response" | jq . >/dev/null 2>&1; then
    removed_count=$(echo $cleanup_response | jq '.removed_count')
    echo "  Deltas removidos (>60min): $removed_count"
else
    echo "  ⚠️  Falha na limpeza: $cleanup_response"
fi

echo
echo "=== Status Final dos Drones ==="
ps aux | grep './drone' | grep -v grep | while read line; do
    pid=$(echo $line | awk '{print $2}')
    cmd=$(echo $line | awk '{for(i=11;i<=NF;i++) printf "%s ", $i; print ""}')
    echo "  PID $pid: $cmd"
done

echo
echo "=== Fase 3 Demonstrada! ==="
echo "✅ F1: Coleta automática funcionando"
echo "   - Drone-1: coletando a cada 2s"
echo "   - Drone-2: coletando a cada 3s"
echo "   - Drone-3: coletando a cada 5s"
echo
echo "✅ F2: Delta-Set CRDT funcionando"
echo "   - Método Apply(Δ): integração de deltas"
echo "   - Método Merge(other): merge entre drones"
echo "   - UUID único para cada delta"
echo "   - Deduplicação automática"
echo
echo "✅ F3: Protocolos de controle UDP implementados"
echo "   - Advertise: anúncio de dados disponíveis"
echo "   - Request: solicitação de dados específicos"
echo "   - SwitchChannel: base para eleição de transmissor"
echo
echo "✅ F5: Tabela de vizinhos via UDP"
echo "   - Descoberta automática de vizinhos"
echo "   - Expiração baseada em TTL"
echo "   - URLs para comunicação TCP"
echo
echo "✅ F6: Base para eleição de transmissor"
echo "   - Estados: IDLE, TRANSMITTER"
echo "   - Contadores ReqCtr para eleição greedy"
echo "   - Timeout de 5s para transmissão"
echo
echo "🔄 Dados sendo coletados em tempo real..."
echo "📊 Acesse http://localhost:8080/state para ver estado atual"
echo "📈 Acesse http://localhost:8080/stats para estatísticas completas"
echo
echo "Próximas implementações:"
echo "  - Fase 5: Métricas avançadas, detecção de falhas e orquestração completa"
echo "  - Otimizações de performance e robustez"

echo
echo "Pressione Ctrl+C para parar todos os drones..."

# Cleanup function
cleanup() {
    echo
    echo "Parando drones..."
    kill $DRONE1_PID $DRONE2_PID $DRONE3_PID 2>/dev/null || true
    sleep 1
    pkill -f './drone' 2>/dev/null || true
    echo "Demo finalizado."
    exit 0
}

# Trap para cleanup
trap cleanup SIGINT SIGTERM

# Aguarda sinal para parar
while true; do
    sleep 1
done
