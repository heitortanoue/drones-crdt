# Sistema de Drones Distribuído - Implementação Completa

Um sistema distribuído de coleta e disseminação de dados de sensores usando drones com arquitetura baseada em CRDTs (Conflict-free Replicated Data Types) e protocolos de gossip.

## 📋 Índice

- [Visão Geral](#visão-geral)
- [Tecnologias e Protocolos](#tecnologias-e-protocolos)
- [Arquitetura do Sistema](#arquitetura-do-sistema)
- [Componentes Principais](#componentes-principais)
- [Fluxo Principal da Aplicação](#fluxo-principal-da-aplicação)
- [Protocolos de Comunicação](#protocolos-de-comunicação)
- [Cenários de Falha e Tolerância](#cenários-de-falha-e-tolerância)
- [APIs e Endpoints](#apis-e-endpoints)
- [Configuração e Execução](#configuração-e-execução)
- [Estrutura do Projeto](#estrutura-do-projeto)

## 🎯 Visão Geral

Este sistema implementa uma rede distribuída de drones para coleta e sincronização de dados de sensores ambientais. Cada drone atua como um nó independente capaz de:

- **Coletar dados** de sensores locais automaticamente
- **Descobrir vizinhos** dinamicamente via protocolo UDP
- **Sincronizar dados** usando CRDTs para consistência eventual
- **Disseminar informações** via protocolo de gossip com TTL
- **Eleger transmissores** usando algoritmo greedy baseado em demanda
- **Tolerar falhas** através de replicação e redundância

### Características Principais

- ✅ **Consistência Eventual**: CRDTs garantem convergência sem coordenação
- ✅ **Descoberta Dinâmica**: Vizinhos descobertos automaticamente via UDP
- ✅ **Tolerância a Falhas**: Sistema continua operando mesmo com nós falhos
- ✅ **Escalabilidade**: Protocolo de gossip escala logaritmicamente
- ✅ **Zero Configuração**: Nós se conectam automaticamente
- ✅ **APIs REST**: Interface HTTP para integração e monitoramento

## 🔧 Tecnologias e Protocolos

### Tecnologias Base

- **Go 1.21+**: Linguagem principal para alta performance e concorrência
- **HTTP/REST**: API de comunicação e endpoints de dados
- **UDP**: Canal de controle para descoberta e coordenação
- **JSON**: Serialização de dados e mensagens
- **Goroutines**: Concorrência nativa para operações paralelas

### Protocolos Implementados

#### 1. **CRDT (Conflict-free Replicated Data Types)**
- **Tipo**: OR-Set (Observed-Remove Set)
- **Operações**: Add-wins semântica para resolução de conflitos
- **Garantias**: Comutatividade, associatividade, idempotência
- **Uso**: Sincronização de dados de sensores entre drones

#### 2. **Protocolo de Gossip com TTL**
- **Modelo**: Push-based epidemic broadcasting
- **TTL**: Time-to-Live para controle de propagação
- **Fan-out**: Número configurável de vizinhos por rodada
- **Deduplicação**: Cache LRU para evitar loops e reenvios

#### 3. **Algoritmo de Eleição Greedy**
- **Tipo**: Transmitter election baseado em demanda
- **Métrica**: Contadores ReqCtr por delta de dados
- **Estados**: IDLE ↔ TRANSMITTER com timeouts automáticos
- **Protocolo**: Mensagens SwitchChannel para coordenação

#### 4. **Descoberta de Vizinhos UDP**
- **Mecanismo**: Broadcast e escuta ativa na porta 7000
- **Timeout**: Expiração automática de vizinhos inativos (9s)
- **Atualização**: Tabela dinâmica com timestamps

## 🏗️ Arquitetura do Sistema

```
┌─────────────────────────────────────────────────────────────┐
│                     APLICAÇÃO PRINCIPAL                     │
│                       (main.go)                            │
└─────────────────┬───────────────────────────────────────────┘
                  │
        ┌─────────┴─────────┐
        │     CONFIG        │
        │  (internal/config) │
        └─────────┬─────────┘
                  │
    ┌─────────────┼─────────────┐
    │             │             │
┌───▼────┐   ┌────▼────┐   ┌────▼────┐
│SENSOR  │   │NETWORK  │   │PROTOCOL │
│ CRDT   │   │UDP/HTTP │   │CONTROL  │
└───┬────┘   └────┬────┘   └────┬────┘
    │             │             │
    └─────────────┼─────────────┘
                  │
            ┌─────▼─────┐
            │  GOSSIP   │
            │DISSEMINATE│
            └───────────┘
```

### Camadas da Arquitetura

1. **Camada de Aplicação** (`main.go`)
   - Orquestração de componentes
   - Configuração e flags CLI
   - Gestão de ciclo de vida

2. **Camada de Configuração** (`internal/config`)
   - Parâmetros centralizados
   - Valores padrão
   - Validação de configuração

3. **Camada de Dados** (`pkg/sensor`)
   - CRDT para dados de sensores
   - Geração automática de leituras
   - API de acesso aos dados

4. **Camada de Rede** (`pkg/network`)
   - Servidores UDP e TCP
   - Descoberta de vizinhos
   - Gestão de conexões

5. **Camada de Protocolo** (`pkg/protocol`)
   - Sistema de controle de mensagens
   - Eleição de transmissores
   - Coordenação distribuída

6. **Camada de Disseminação** (`pkg/gossip`)
   - Protocolo de gossip
   - Cache de deduplicação
   - Controle de TTL

## 🔧 Componentes Principais

### 1. Sistema de Sensores (`pkg/sensor`)

```go
// SensorAPI - Interface principal para dados de sensores
type SensorAPI struct {
    deltaSet  *DeltaSet        // CRDT para armazenamento
    generator *SensorGenerator // Geração automática
    droneID   string          // Identificação do drone
}
```

**Funcionalidades:**
- **Coleta Automática**: Geração periódica de leituras simuladas
- **CRDT Integration**: Armazenamento em OR-Set distribuído
- **API Manual**: Adição de leituras via HTTP
- **Merge Distribuído**: Sincronização com outros drones
- **Cleanup**: Remoção automática de dados antigos

**Tipos de Dados:**
- Leituras de área de cobertura (percentual)
- Timestamps em milissegundos
- IDs únicos UUID v4
- Metadados de drone origem

### 2. Sistema de Rede (`pkg/network`)

#### 2.1 Servidor UDP (`UDPServer`)
```go
type UDPServer struct {
    conn             *net.UDPConn
    neighborTable    *NeighborTable
    messageProcessor MessageProcessor
    droneID          string
    port             int
}
```

**Responsabilidades:**
- Descoberta automática de vizinhos
- Canal de controle (porta 7000)
- Broadcast de mensagens de coordenação
- Atualização da tabela de vizinhos

#### 2.2 Servidor TCP (`TCPServer`)
```go
type TCPServer struct {
    port    int
    mux     *http.ServeMux
    droneID string
    server  *http.Server
}
```

**Responsabilidades:**
- API HTTP REST (porta 8080)
- Canal de dados para sincronização
- Endpoints de monitoramento
- Interface de integração externa

#### 2.3 Tabela de Vizinhos (`NeighborTable`)
```go
type NeighborTable struct {
    neighbors map[string]*Neighbor
    mutex     sync.RWMutex
    timeout   time.Duration
}
```

**Funcionalidades:**
- Descoberta dinâmica de vizinhos
- Expiração automática (timeout 9s)
- Thread-safe concurrent access
- URLs para comunicação HTTP

### 3. Sistema de Protocolo (`pkg/protocol`)

#### 3.1 Sistema de Controle (`ControlSystem`)
```go
type ControlSystem struct {
    droneID       string
    sensorAPI     SensorAPIInterface
    udpSender     UDPSender
    reqCounters   map[uuid.UUID]int
    running       bool
}
```

**Mensagens de Controle:**
- **ADVERTISE**: Anuncia deltas disponíveis
- **REQUEST**: Solicita deltas específicos
- **SWITCH_CHANNEL**: Coordena mudança de canal

**Fluxo de Controle:**
1. Advertise periódico (3-6s) com lista de deltas
2. Request baseado em deltas missing
3. Response com dados solicitados
4. Atualização de contadores ReqCtr

#### 3.2 Eleição de Transmissor (`TransmitterElection`)
```go
type TransmitterElection struct {
    droneID         string
    controlSystem   ControlSystemInterface
    currentState    ElectionState
    transmitTimeout time.Duration
}
```

**Estados:**
- **IDLE**: Estado padrão, monitora demanda
- **TRANSMITTER**: Transmitindo ativamente dados

**Algoritmo:**
1. Monitora contadores ReqCtr > 0
2. Transição para TRANSMITTER quando detecta demanda
3. Envia 3x mensagens SwitchChannel
4. Timeout automático para retornar ao IDLE (5s)

### 4. Sistema de Gossip (`pkg/gossip`)

#### 4.1 Sistema de Disseminação (`DisseminationSystem`)
```go
type DisseminationSystem struct {
    droneID       string
    fanout        int
    defaultTTL    int
    neighborTable *NeighborTable
    tcpSender     TCPSender
    cache         *DeduplicationCache
}
```

**Características:**
- **TTL Control**: Decrementa TTL a cada hop
- **Fan-out**: Seleciona N vizinhos aleatórios
- **Deduplicação**: Cache LRU evita reprocessamento
- **Async Processing**: Goroutines para paralelismo

#### 4.2 Cache de Deduplicação (`DeduplicationCache`)
```go
type DeduplicationCache struct {
    cache    map[string]*list.Element
    lruList  *list.List
    maxSize  int
    mutex    sync.RWMutex
}
```

**Funcionalidades:**
- LRU eviction policy
- Thread-safe concurrent access
- Configuração dinâmica de tamanho
- Estatísticas de hit/miss ratio

#### 4.3 TCP Sender (`HTTPTCPSender`)
```go
type HTTPTCPSender struct {
    client  *http.Client
    timeout time.Duration
}
```

**Características:**
- HTTP client configurável
- Timeout por request
- Retry logic
- Error handling robusto

## 🔄 Fluxo Principal da Aplicação

### 1. Inicialização do Sistema

```
1. Parse de argumentos CLI
2. Carregamento de configuração
3. Criação de componentes:
   ├── SensorAPI com CRDT
   ├── NeighborTable com timeout
   ├── UDPServer (porta 7000)
   ├── TCPServer (porta 8080)
   ├── ControlSystem para coordenação
   ├── TransmitterElection greedy
   └── DisseminationSystem com gossip
4. Start de todos os serviços
5. Registro de signal handlers
```

### 2. Coleta de Dados

```
SensorGenerator (goroutine) ->
├── Gera leitura a cada interval
├── Cria SensorDelta com UUID
├── Adiciona ao CRDT local
└── Trigger disseminação via gossip
```

### 3. Descoberta de Vizinhos

```
UDPServer (goroutine) ->
├── Escuta porta 7000
├── Processa pacotes UDP recebidos
├── Atualiza NeighborTable automaticamente
└── Expira vizinhos inativos (9s timeout)
```

### 4. Coordenação Distribuída

```
ControlSystem (goroutine) ->
├── Advertise periódico (3-6s):
│   ├── Lista deltas disponíveis
│   └── Broadcast via UDP
├── Processa mensagens recebidas:
│   ├── ADVERTISE: verifica deltas missing
│   ├── REQUEST: responde com deltas
│   └── SWITCH_CHANNEL: atualiza contadores
└── Atualiza contadores ReqCtr
```

### 5. Eleição de Transmissor

```
TransmitterElection (loop) ->
├── Monitora contadores ReqCtr
├── Se ReqCtr[delta] > 0:
│   ├── Transição IDLE -> TRANSMITTER
│   ├── Envia 3x SwitchChannel messages
│   ├── Reset contador para delta
│   └── Schedule timeout (5s)
└── Timeout: TRANSMITTER -> IDLE
```

### 6. Disseminação via Gossip

```
DisseminationSystem ->
├── Recebe trigger de novo delta
├── Verifica cache deduplicação
├── Se não duplicado:
│   ├── Adiciona ao cache
│   ├── Seleciona fanout vizinhos
│   ├── Decrementa TTL
│   ├── Se TTL > 0: propaga
│   └── Async send via HTTP
└── Atualiza estatísticas
```

### 7. Sincronização de Dados

```
HTTP Endpoints ->
├── POST /delta: recebe dados externos
├── CRDT.merge(): integra com dados locais
├── GET /state: retorna estado atual
├── GET /stats: métricas do sistema
└── POST /cleanup: remove dados antigos
```

## 📡 Protocolos de Comunicação

### Canal de Controle (UDP - Porta 7000)

**Formato das Mensagens:**
```json
{
  "type": "ADVERTISE|REQUEST|SWITCH_CHANNEL",
  "sender_id": "drone-1",
  "timestamp": 1687123456789,
  "data": { /* payload específico */ }
}
```

**Tipos de Mensagem:**

1. **ADVERTISE**
```json
{
  "type": "ADVERTISE",
  "sender_id": "drone-1",
  "timestamp": 1687123456789,
  "data": {
    "have_ids": ["uuid1", "uuid2", "uuid3"]
  }
}
```

2. **REQUEST**
```json
{
  "type": "REQUEST",
  "sender_id": "drone-2",
  "timestamp": 1687123456789,
  "data": {
    "wanted_ids": ["uuid1", "uuid3"]
  }
}
```

3. **SWITCH_CHANNEL**
```json
{
  "type": "SWITCH_CHANNEL",
  "sender_id": "drone-1",
  "timestamp": 1687123456789,
  "data": {
    "delta_id": "uuid1",
    "req_count": 3
  }
}
```

### Canal de Dados (HTTP - Porta 8080)

**Endpoints Principais:**

1. **Sincronização de Deltas**
```http
POST /delta
Content-Type: application/json

{
  "sender_id": "drone-1",
  "deltas": [
    {
      "id": "uuid1",
      "sensor_id": "area-drone-1-A",
      "timestamp": 1687123456789,
      "value": 85.5,
      "drone_id": "drone-1"
    }
  ]
}
```

2. **Estado do CRDT**
```http
GET /state
```

3. **Estatísticas do Sistema**
```http
GET /stats
```

## ⚠️ Cenários de Falha e Tolerância

### 1. Falha de Nó Individual

**Cenário:** Um drone para de funcionar ou perde conectividade

**Mecanismos de Tolerância:**
- **Timeout de Vizinho**: Vizinhos expiram automaticamente após 9s
- **Redistribuição**: Outros nós assumem responsabilidades
- **Replicação**: Dados existem em múltiplos nós
- **Gossip Resilience**: Protocolo continua com nós restantes

**Comportamento:**
```
Drone-1 falha ->
├── Vizinhos param de receber heartbeat
├── NeighborTable remove Drone-1 após timeout
├── Algoritmo de gossip adapta fan-out
├── Eleição de transmissor continua com nós ativos
└── Dados já replicados permanecem disponíveis
```

### 2. Partição de Rede

**Cenário:** Rede se divide em partições isoladas

**Mecanismos de Tolerância:**
- **CRDT Eventual Consistency**: Partições convergem quando reconectadas
- **Gossip dentro de Partições**: Continua operando localmente
- **State Reconciliation**: Merge automático na reconexão

**Comportamento:**
```
Partição de Rede ->
├── Cada partição opera independentemente
├── CRDTs mantêm consistência local
├── Gossip continua dentro de cada partição
├── Reconexão: merge automático de estados
└── Convergência eventual garantida
```

### 3. Falha de Comunicação UDP

**Cenário:** Canal de controle UDP falha ou congestionado

**Mecanismos de Tolerância:**
- **HTTP Fallback**: Comunicação via canal de dados TCP
- **Timeout Adaptativo**: Ajusta timeouts baseado em condições
- **Retry Logic**: Retentativas automáticas
- **Graceful Degradation**: Sistema continua com funcionalidade reduzida

**Comportamento:**
```
UDP Falha ->
├── Descoberta de vizinhos impactada
├── Controle via HTTP como fallback
├── Gossip continua via TCP
├── Performance reduzida mas funcional
└── Auto-recovery quando UDP volta
```

### 4. Sobrecarga do Sistema

**Cenário:** Alto volume de dados ou muitos nós ativos

**Mecanismos de Tolerância:**
- **Backpressure**: Controle de fluxo automático
- **Cache LRU**: Evita reprocessamento desnecessário
- **Async Processing**: Operações não-bloqueantes
- **Rate Limiting**: Throttling de disseminação

**Comportamento:**
```
Alta Carga ->
├── Cache LRU filtra duplicados
├── Goroutines processam async
├── TTL limita propagação excessiva
├── Fan-out adaptativo reduz overhead
└── Degradação controlada de performance
```

### 5. Dados Corrompidos ou Conflitantes

**Cenário:** Dados inconsistentes ou corrompidos chegam ao sistema

**Mecanismos de Tolerância:**
- **CRDT Semantics**: Add-wins resolve conflitos automaticamente
- **Validation**: Verificação de formato e timestamps
- **Idempotência**: Operações seguras para reprocessamento
- **Cleanup**: Remoção automática de dados antigos

**Comportamento:**
```
Dados Conflitantes ->
├── CRDT aplica semântica add-wins
├── Timestamps determinam ordem
├── Merge sempre converge
├── Validation rejeita dados malformados
└── Estado eventual consistente
```

## 🌐 APIs e Endpoints

### Endpoints de Dados

#### `GET /health`
**Descrição:** Status de saúde do drone
```json
{
  "drone_id": "drone-1",
  "status": "healthy",
  "port": 8080
}
```

#### `POST /sensor`
**Descrição:** Adiciona leitura manual de sensor
```json
// Request
{
  "sensor_id": "manual-sensor-1",
  "value": 92.3
}

// Response
{
  "id": "uuid-generated",
  "sensor_id": "manual-sensor-1",
  "timestamp": 1687123456789,
  "value": 92.3,
  "drone_id": "drone-1"
}
```

#### `POST /delta`
**Descrição:** Recebe e integra deltas de outros drones
```json
// Request
{
  "sender_id": "drone-2",
  "deltas": [...]
}

// Response
{
  "merged_count": 5,
  "total_deltas": 27
}
```

#### `GET /state`
**Descrição:** Estado atual do CRDT
```json
{
  "deltas": [...],
  "total_count": 27,
  "latest_by_sensor": {...}
}
```

#### `GET /stats`
**Descrição:** Estatísticas completas do sistema
```json
{
  "drone_id": "drone-1",
  "uptime_seconds": 3600,
  "sensor": {
    "total_deltas": 27,
    "sensors_active": 3,
    "generator_running": true
  },
  "network": {
    "udp_port": 7000,
    "tcp_port": 8080,
    "neighbors_active": 5
  },
  "protocol": {
    "control_running": true,
    "election_state": "IDLE",
    "req_counters": {...}
  },
  "gossip": {
    "dissemination_running": true,
    "cache_size": 100,
    "sent_count": 15,
    "received_count": 32
  }
}
```

#### `POST /cleanup`
**Descrição:** Remove dados antigos
```json
// Request
{
  "max_age_hours": 24
}

// Response
{
  "removed_count": 15,
  "remaining_count": 12
}
```

### Endpoints de Controle

#### `GET /neighbors`
**Descrição:** Lista de vizinhos ativos
```json
{
  "neighbors": [
    {
      "ip": "192.168.1.100",
      "port": 8080,
      "last_seen": "2023-06-19T10:30:45Z"
    }
  ],
  "count": 1
}
```

#### `POST /control/election`
**Descrição:** Controle manual da eleição
```json
// Request
{
  "action": "force_idle|enable|disable"
}

// Response
{
  "previous_state": "TRANSMITTER",
  "current_state": "IDLE",
  "enabled": true
}
```

## ⚙️ Configuração e Execução

### Parâmetros de Linha de Comando

```bash
./drone [opções]

Opções:
  -id string          ID único deste drone (default "drone-1")
  -sample-sec int     Intervalo de coleta em segundos (default 10)
  -fanout int         Número de vizinhos para gossip (default 3)
  -ttl int           TTL inicial para mensagens (default 4)
  -udp-port int      Porta UDP para controle (default 7000)
  -tcp-port int      Porta TCP para dados (default 8080)
  -bind string       Endereço para bind (default "0.0.0.0")
  -help              Mostra ajuda de uso
```

### Exemplos de Execução

#### Drone Básico
```bash
./drone -id "drone-1"
```

#### Drone com Configuração Customizada
```bash
./drone \
  -id "drone-office-01" \
  -sample-sec 5 \
  -fanout 5 \
  -ttl 6 \
  -udp-port 7001 \
  -tcp-port 8081
```

#### Rede de Drones Local
```bash
# Terminal 1
./drone -id "drone-1" -udp-port 7000 -tcp-port 8080

# Terminal 2
./drone -id "drone-2" -udp-port 7001 -tcp-port 8081

# Terminal 3
./drone -id "drone-3" -udp-port 7002 -tcp-port 8082
```

### Scripts de Demonstração

#### Demo Básico (Funcionalidades)
```bash
./demo.sh
```
Demonstra todas as funcionalidades implementadas em condições normais.

#### Demo de Cenários de Falha
```bash
./demo_failure_scenarios.sh
```
Testa e valida todos os mecanismos de tolerância a falhas:
- **Falha de nó individual**: Kill de processo e detecção automática
- **Partição de rede**: Operação independente e reconexão
- **Recuperação**: Redescoberta e convergência de dados
- **Sobrecarga**: Injeção massiva e estabilidade do sistema
- **Timeout de vizinhos**: Expiração automática após 9s
- **Merge CRDT**: Convergência eventual após reconexão

### Variáveis de Ambiente

```bash
# Configuração de logging
export DRONE_LOG_LEVEL=debug

# Configuração de rede
export DRONE_BIND_ADDR=192.168.1.100
export DRONE_MAX_NEIGHBORS=10

# Configuração de timeouts
export DRONE_NEIGHBOR_TIMEOUT=15s
export DRONE_ELECTION_TIMEOUT=10s
```

### Compilação

```bash
# Build para plataforma atual
go build -o bin/drone .

# Build para Linux
GOOS=linux GOARCH=amd64 go build -o bin/drone-linux .

# Build para Windows
GOOS=windows GOARCH=amd64 go build -o bin/drone.exe .

# Build com otimizações
go build -ldflags="-s -w" -o bin/drone .
```

### Execução via Docker

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o drone .

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/drone .
EXPOSE 7000/udp 8080/tcp
CMD ["./drone"]
```

```bash
# Build da imagem
docker build -t drone-system .

# Execução de rede de drones
docker-compose up -d
```

## 📁 Estrutura do Projeto

```
drone/
├── main.go                    # Aplicação principal e orquestração
├── go.mod                     # Dependências do módulo Go
├── go.sum                     # Checksums das dependências
├── README.md                  # Documentação principal
├── demo.sh                    # Script de demonstração das funcionalidades
├── demo_failure_scenarios.sh  # Script de teste de cenários de falha
├── Dockerfile                 # Container configuration
├── docker-compose.yml         # Multi-node setup
│
├── internal/                  # Código interno (não exportado)
│   └── config/
│       └── config.go         # Configuração centralizada
│
├── pkg/                      # Pacotes exportáveis
│   ├── api/                  # Definições de API (futuro)
│   ├── logging/              # Sistema de logging
│   │   └── logger.go
│   │
│   ├── sensor/               # Sistema de sensores e CRDT
│   │   ├── sensor.go         # API principal de sensores
│   │   ├── crdt.go          # Implementação CRDT OR-Set
│   │   └── generator.go      # Geração automática de dados
│   │
│   ├── network/              # Camada de rede e comunicação
│   │   ├── neighbor_table.go # Descoberta e gestão de vizinhos
│   │   ├── udp_server.go     # Servidor UDP (canal controle)
│   │   └── tcp_server.go     # Servidor HTTP (canal dados)
│   │
│   ├── protocol/             # Protocolos de coordenação
│   │   ├── control.go        # Sistema de controle distribuído
│   │   ├── election.go       # Eleição de transmissor greedy
│   │   └── messages.go       # Formato e codificação de mensagens
│   │
│   └── gossip/               # Sistema de disseminação
│       ├── dissemination.go  # Protocolo de gossip com TTL
│       ├── cache.go          # Cache LRU para deduplicação
│       └── tcp_sender.go     # Cliente HTTP para envio
│
├── test/                     # Testes de integração
│   ├── integration_fase2_test.go
│   ├── integration_fase3_test.go
│   └── integration_fase4_test.go
│
├── scripts/                  # Scripts utilitários
├── bin/                     # Binários compilados
└── docs/                    # Documentação adicional
    ├── FASE4.md             # Especificação da Fase 4
    └── ARCHITECTURE.md      # Detalhes arquiteturais
```

### Descrição dos Pacotes

#### `internal/config`
- Configuração centralizada do sistema
- Valores padrão e validação
- Estruturas de dados para parâmetros

#### `pkg/sensor`
- **`sensor.go`**: API principal para acesso aos dados de sensores
- **`crdt.go`**: Implementação do CRDT OR-Set para consenso distribuído
- **`generator.go`**: Geração automática de leituras de sensores simulados

#### `pkg/network`
- **`neighbor_table.go`**: Descoberta dinâmica e gestão de vizinhos
- **`udp_server.go`**: Servidor UDP para canal de controle (porta 7000)
- **`tcp_server.go`**: Servidor HTTP para canal de dados (porta 8080)

#### `pkg/protocol`
- **`control.go`**: Sistema de controle distribuído com mensagens de coordenação
- **`election.go`**: Algoritmo de eleição greedy para transmissores
- **`messages.go`**: Definição e codificação de mensagens de protocolo

#### `pkg/gossip`
- **`dissemination.go`**: Protocolo de gossip com TTL e fan-out
- **`cache.go`**: Cache LRU para deduplicação e prevenção de loops
- **`tcp_sender.go`**: Cliente HTTP para envio de dados via gossip

---

## 🔬 Testes e Validação

O sistema possui uma suíte completa de testes cobrindo:

- **Testes Unitários**: Todos os componentes principais
- **Testes de Integração**: Cenários end-to-end
- **Testes de Concorrência**: Operações paralelas
- **Testes de Falha**: Cenários de erro e recovery
- **Benchmarks**: Performance e escalabilidade

Execute com:
```bash
# Todos os testes
go test ./...

# Testes com verbose
go test -v ./...

# Testes de integração
go test -v ./test

# Benchmarks
go test -bench=. ./...
```

---

## 📊 Métricas e Monitoramento

O sistema expõe métricas detalhadas através do endpoint `/stats`:

- **Sensor**: Deltas coletados, sensores ativos, status do gerador
- **Network**: Conexões ativas, vizinhos descobertos, status dos servidores
- **Protocol**: Estado da eleição, contadores de requests, mensagens processadas
- **Gossip**: Cache hits/misses, mensagens enviadas/recebidas, TTL statistics

---

## 📝 Licença

Este projeto está licenciado sob a Licença MIT - veja o arquivo [LICENSE](LICENSE) para detalhes.

---