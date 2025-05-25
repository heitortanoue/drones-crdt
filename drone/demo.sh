#!/bin/bash

# Demo do sistema de drones com SWIM membership
# Este script demonstra como o sistema funciona com descoberta automática de peers

echo "=== Demo: Sistema de Drones com SWIM Membership ==="
echo

# Limpa processos anteriores
echo "Limpando processos anteriores..."
pkill -f "./drone" || true
sleep 2

# Constrói o projeto
echo "Construindo projeto..."
go build -o drone
if [ $? -ne 0 ]; then
    echo "❌ Erro na compilação"
    exit 1
fi

echo "✅ Projeto compilado com sucesso"
echo

# Função para aguardar o servidor ficar disponível
wait_for_server() {
    local port=$1
    local name=$2
    echo "Aguardando $name ficar online..."

    for i in {1..15}; do
        if curl -s http://localhost:$port/stats > /dev/null 2>&1; then
            echo "✅ $name online"
            return 0
        fi
        sleep 1
        echo -n "."
    done

    echo "❌ Timeout aguardando $name"
    return 1
}

# Inicia o primeiro drone (seed do cluster)
echo "=== Iniciando Drone 1 (Seed) ==="
./drone -drone=drone-01 -port=8080 -swim-port=7946 &
DRONE1_PID=$!
echo "Drone 1 PID: $DRONE1_PID"

wait_for_server 8080 "Drone 1"
sleep 2

# Inicia o segundo drone (conecta ao primeiro)
echo
echo "=== Iniciando Drone 2 ==="
./drone -drone=drone-02 -port=8081 -swim-port=7947 -seeds=127.0.0.1:7946 &
DRONE2_PID=$!
echo "Drone 2 PID: $DRONE2_PID"

wait_for_server 8081 "Drone 2"
sleep 3

# Inicia o terceiro drone (conecta ao cluster)
echo
echo "=== Iniciando Drone 3 ==="
./drone -drone=drone-03 -port=8082 -swim-port=7948 -seeds=127.0.0.1:7946 &
DRONE3_PID=$!
echo "Drone 3 PID: $DRONE3_PID"

wait_for_server 8082 "Drone 3"
sleep 3

echo
echo "=== Cluster SWIM Iniciado ==="
echo "Drone 1: http://localhost:8080 (SWIM: 7946)"
echo "Drone 2: http://localhost:8081 (SWIM: 7947)"
echo "Drone 3: http://localhost:8082 (SWIM: 7948)"
echo

# Função para mostrar membros do cluster
show_cluster_members() {
    echo "=== Membros do Cluster ==="
    for port in 8080 8081 8082; do
        echo "--- Drone na porta $port ---"
        curl -s http://localhost:$port/members | jq -r '.members[] | "  \(.node_id): \(.address) (\(.status))"' 2>/dev/null || echo "  Erro ao consultar"
    done
    echo
}

# Mostra membros iniciais
show_cluster_members

# Função para enviar dados de sensor
send_sensor_data() {
    local port=$1
    local sensor_id=$2
    local value=$3

    curl -s -X POST http://localhost:$port/sensor \
        -H "Content-Type: application/json" \
        -d "{\"sensor_id\":\"$sensor_id\",\"value\":$value}" > /dev/null

    if [ $? -eq 0 ]; then
        echo "✅ Enviado: $sensor_id=$value para porta $port"
    else
        echo "❌ Erro enviando para porta $port"
    fi
}

# Simula coleta de dados de sensores
echo "=== Simulando Coleta de Dados ==="
echo "Enviando dados de sensores para diferentes drones..."

# Drone 1 coleta dados da área norte
send_sensor_data 8080 "area-norte-01" 65.2
send_sensor_data 8080 "area-norte-02" 71.8

# Drone 2 coleta dados da área sul
send_sensor_data 8081 "area-sul-01" 58.4
send_sensor_data 8081 "area-sul-02" 63.1

# Drone 3 coleta dados da área leste
send_sensor_data 8082 "area-leste-01" 69.7
send_sensor_data 8082 "area-leste-02" 74.3

echo
echo "Aguardando propagação via gossip SWIM..."
sleep 10

# Função para mostrar estado do CRDT
show_crdt_state() {
    local port=$1
    local drone_name=$2

    echo "--- Estado do $drone_name (porta $port) ---"
    local count=$(curl -s http://localhost:$port/state | jq '.state | length' 2>/dev/null)
    if [ "$count" != "null" ] && [ "$count" != "" ]; then
        echo "  Total de leituras: $count"
        curl -s http://localhost:$port/state | jq -r '.state[] | "  \(.sensor_id): \(.value)% (\(.drone_id))"' 2>/dev/null | head -10
    else
        echo "  Erro ao consultar estado"
    fi
    echo
}

# Mostra convergência dos dados
echo "=== Convergência dos Dados (CRDT) ==="
show_crdt_state 8080 "Drone 1"
show_crdt_state 8081 "Drone 2"
show_crdt_state 8082 "Drone 3"

# Testa failure detection - mata um drone
echo "=== Testando Failure Detection ==="
echo "Terminando Drone 2 para testar detecção de falhas..."
kill $DRONE2_PID 2>/dev/null
sleep 8

echo "Membros do cluster após falha:"
show_cluster_members

# Testa adicionar novo drone ao cluster existente
echo "=== Adicionando Novo Drone ao Cluster ==="
./drone -drone=drone-04 -port=8084 -swim-port=7949 -seeds=127.0.0.1:7946 &
DRONE4_PID=$!
echo "Drone 4 PID: $DRONE4_PID"

wait_for_server 8084 "Drone 4"
sleep 5

echo "Membros após adição do Drone 4:"
show_cluster_members

# Envia mais dados e verifica convergência
echo "=== Teste Final de Convergência ==="
send_sensor_data 8084 "area-oeste-01" 55.9
send_sensor_data 8080 "area-norte-03" 67.4

sleep 8

echo "Estado final dos CRDTs:"
show_crdt_state 8080 "Drone 1"
show_crdt_state 8082 "Drone 3"
show_crdt_state 8084 "Drone 4"

# Mostra estatísticas finais
echo "=== Estatísticas Finais ==="
for port in 8080 8082 8084; do
    echo "--- Drone na porta $port ---"
    curl -s http://localhost:$port/stats | jq -r '{
        drone_id,
        active_peers,
        latest_by_sensor,
        membership: .membership.total_members
    }' 2>/dev/null || echo "Erro ao consultar stats"
done

echo
echo "=== Demo Concluído ==="
echo "Pressione Ctrl+C para encerrar todos os drones"
echo

# Função de limpeza
cleanup() {
    echo
    echo "Encerrando todos os drones..."
    kill $DRONE1_PID $DRONE3_PID $DRONE4_PID 2>/dev/null
    pkill -f "./drone" || true
    echo "✅ Demo encerrado"
    exit 0
}

# Captura sinais para limpeza
trap cleanup SIGINT SIGTERM

# Mantém o script rodando
while true; do
    read -t 1 input
    if [ "$input" = "q" ]; then
        cleanup
    fi
done
