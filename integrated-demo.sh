#!/bin/bash

# Demonstração Integrada: Sistema de Coleta de Dados IoT com Drones
# Este script demonstra o sistema completo de sensores IoT + descoberta automática + drones

set -e

echo "🚁 === DEMO INTEGRADO: SISTEMA DE COLETA DE DADOS IoT ===  🚁"
echo ""

# Configurações
SENSOR_DEVICE_DIR="sensor-device"
DRONE_DIR="drone"
BASE_PORT=8080
DISCOVERY_PORT=9999
SENSOR_BASE_PORT=8000

# Função para limpar processos em background
cleanup() {
    echo ""
    echo "🧹 Limpando processos em background..."

    # Mata processos por PID se existirem
    if [ ! -z "$SENSOR1_PID" ]; then kill $SENSOR1_PID 2>/dev/null || true; fi
    if [ ! -z "$SENSOR2_PID" ]; then kill $SENSOR2_PID 2>/dev/null || true; fi
    if [ ! -z "$SENSOR3_PID" ]; then kill $SENSOR3_PID 2>/dev/null || true; fi
    if [ ! -z "$DRONE1_PID" ]; then kill $DRONE1_PID 2>/dev/null || true; fi
    if [ ! -z "$DRONE2_PID" ]; then kill $DRONE2_PID 2>/dev/null || true; fi

    # Mata qualquer processo restante nas portas usadas
    for port in $BASE_PORT $((BASE_PORT+1)) $DISCOVERY_PORT $((DISCOVERY_PORT+1)) $SENSOR_BASE_PORT $((SENSOR_BASE_PORT+1)) $((SENSOR_BASE_PORT+2)); do
        lsof -ti tcp:$port 2>/dev/null | xargs kill -9 2>/dev/null || true
    done

    echo "✅ Limpeza concluída!"
}

# Configura limpeza ao sair
trap cleanup EXIT INT TERM

echo "📦 Etapa 1: Compilando projetos..."
echo ""

# Compila sensor device
echo "🔧 Compilando sensor device..."
cd $SENSOR_DEVICE_DIR
go build -o sensor-device . || {
    echo "❌ Erro ao compilar sensor device"
    exit 1
}
echo "✅ Sensor device compilado com sucesso"

# Compila drone server
echo "🔧 Compilando drone server..."
cd ../$DRONE_DIR
go build -o drone-server . || {
    echo "❌ Erro ao compilar drone server"
    exit 1
}
echo "✅ Drone server compilado com sucesso"

cd ..

echo ""
echo "🌱 Etapa 2: Iniciando sensores IoT da fazenda..."
echo ""

# Inicia sensores simulando diferentes áreas da fazenda
echo "🌾 Iniciando Sensor 1 - Área A (Norte da fazenda)..."
cd $SENSOR_DEVICE_DIR
./sensor-device -port=$SENSOR_BASE_PORT -sensor="sensor-north-field" -location="Norte da Fazenda - Área A" &
SENSOR1_PID=$!
sleep 2

echo "🌽 Iniciando Sensor 2 - Área B (Sul da fazenda)..."
./sensor-device -port=$((SENSOR_BASE_PORT+1)) -sensor="sensor-south-field" -location="Sul da Fazenda - Área B" &
SENSOR2_PID=$!
sleep 2

echo "🍃 Iniciando Sensor 3 - Estufas..."
./sensor-device -port=$((SENSOR_BASE_PORT+2)) -sensor="sensor-greenhouse" -location="Estufas - Área C" &
SENSOR3_PID=$!
sleep 2

cd ..

echo "✅ Todos os sensores IoT estão ativos e transmitindo dados!"
echo ""

echo "🚁 Etapa 3: Iniciando drones com descoberta automática..."
echo ""

# Inicia primeiro drone com descoberta
echo "🚁 Iniciando Drone 1 com descoberta automática..."
cd $DRONE_DIR
./drone-server -drone="drone-alpha" -port=$BASE_PORT -discovery=$DISCOVERY_PORT &
DRONE1_PID=$!
sleep 3

# Inicia segundo drone com gossip protocol
echo "🚁 Iniciando Drone 2 com gossip protocol..."
./drone-server -drone="drone-beta" -port=$((BASE_PORT+1)) -discovery=$((DISCOVERY_PORT+1)) -peers="http://localhost:$BASE_PORT" -gossip=10 &
DRONE2_PID=$!
sleep 3

cd ..

echo "✅ Drones estão operacionais e descobrindo sensores!"
echo ""

echo "⏱️  Etapa 4: Aguardando descoberta e coleta de dados..."
echo ""
echo "Os drones estão agora:"
echo "📡 Descobrindo sensores automaticamente via UDP beacons"
echo "📊 Coletando dados dos sensores via HTTP polling"
echo "🔄 Sincronizando dados entre si via gossip protocol"
echo "💾 Armazenando tudo em estruturas CRDT para consistência"
echo ""

sleep 10

echo "🔍 Etapa 5: Verificando sensores descobertos..."
echo ""

echo "Sensores descobertos pelo Drone Alpha:"
curl -s http://localhost:$BASE_PORT/sensors | jq . || echo "Dados em formato raw"

echo ""
echo "Sensores descobertos pelo Drone Beta:"
curl -s http://localhost:$((BASE_PORT+1))/sensors | jq . || echo "Dados em formato raw"

echo ""
sleep 5

echo "📈 Etapa 6: Verificando dados coletados..."
echo ""

echo "Estado do CRDT no Drone Alpha:"
curl -s http://localhost:$BASE_PORT/state | jq '. | length' && echo " deltas armazenados" || echo "Dados em formato raw"

echo ""
echo "Estado do CRDT no Drone Beta:"
curl -s http://localhost:$((BASE_PORT+1))/state | jq '. | length' && echo " deltas armazenados" || echo "Dados em formato raw"

echo ""
sleep 3

echo "📊 Etapa 7: Estatísticas do sistema..."
echo ""

echo "Estatísticas do Drone Alpha:"
curl -s http://localhost:$BASE_PORT/stats | jq . || echo "Dados em formato raw"

echo ""
echo "Estatísticas do Drone Beta:"
curl -s http://localhost:$((BASE_PORT+1))/stats | jq . || echo "Dados em formato raw"

echo ""
sleep 5

echo "🧪 Etapa 8: Testando alguns endpoints dos sensores..."
echo ""

echo "Status do Sensor Norte:"
curl -s http://localhost:$SENSOR_BASE_PORT/status | jq . || echo "Dados em formato raw"

echo ""
echo "Leitura atual do Sensor Sul:"
curl -s http://localhost:$((SENSOR_BASE_PORT+1))/reading | jq . || echo "Dados em formato raw"

echo ""
echo "Saúde do Sensor Estufa:"
curl -s http://localhost:$((SENSOR_BASE_PORT+2))/health | jq . || echo "Dados em formato raw"

echo ""
sleep 5

echo "⏰ Aguardando mais coletas de dados (30 segundos)..."
echo "Durante este tempo, observe os logs dos processos para ver:"
echo "• Beacons UDP sendo enviados pelos sensores"
echo "• Descoberta automática pelos drones"
echo "• Polling de dados a cada 30 segundos"
echo "• Sincronização via gossip protocol"
echo ""

sleep 30

echo ""
echo "📋 Etapa 9: Relatório final do sistema..."
echo ""

echo "=== RELATÓRIO FINAL ==="
echo ""

echo "1. Sensores ativos:"
for port in $SENSOR_BASE_PORT $((SENSOR_BASE_PORT+1)) $((SENSOR_BASE_PORT+2)); do
    status=$(curl -s http://localhost:$port/health 2>/dev/null | jq -r '.status' 2>/dev/null || echo "offline")
    echo "   Sensor porta $port: $status"
done

echo ""
echo "2. Total de dados coletados:"
drone1_count=$(curl -s http://localhost:$BASE_PORT/state 2>/dev/null | jq '. | length' 2>/dev/null || echo "0")
drone2_count=$(curl -s http://localhost:$((BASE_PORT+1))/state 2>/dev/null | jq '. | length' 2>/dev/null || echo "0")
echo "   Drone Alpha: $drone1_count deltas"
echo "   Drone Beta: $drone2_count deltas"

echo ""
echo "3. Peers conectados:"
peers1=$(curl -s http://localhost:$BASE_PORT/peers 2>/dev/null | jq '.peers | length' 2>/dev/null || echo "0")
peers2=$(curl -s http://localhost:$((BASE_PORT+1))/peers 2>/dev/null | jq '.peers | length' 2>/dev/null || echo "0")
echo "   Drone Alpha: $peers1 peers"
echo "   Drone Beta: $peers2 peers"

echo ""
echo "🎉 === DEMONSTRAÇÃO CONCLUÍDA COM SUCESSO! ==="
echo ""
echo "Sistema completo testado:"
echo "✅ Sensores IoT simulando fazenda real"
echo "✅ Descoberta automática via UDP beacons"
echo "✅ Coleta de dados via HTTP polling"
echo "✅ Armazenamento em estruturas CRDT"
echo "✅ Sincronização entre drones via gossip"
echo "✅ APIs RESTful para monitoramento"
echo ""
echo "Para explorar mais:"
echo "• Acesse http://localhost:$BASE_PORT/stats para estatísticas do Drone Alpha"
echo "• Acesse http://localhost:$((BASE_PORT+1))/stats para estatísticas do Drone Beta"
echo "• Acesse http://localhost:$SENSOR_BASE_PORT/status para status do Sensor Norte"
echo ""
echo "Pressione Ctrl+C para finalizar todos os processos."
echo ""

# Mantém os processos rodando até o usuário pressionar Ctrl+C
while true; do
    sleep 5
    # Verifica se algum processo crítico morreu
    if ! kill -0 $SENSOR1_PID 2>/dev/null || ! kill -0 $DRONE1_PID 2>/dev/null; then
        echo "⚠️  Processo crítico finalizado. Encerrando demo..."
        break
    fi
done
