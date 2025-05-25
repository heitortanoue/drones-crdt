# Sistema de Drones com SWIM Membership

Sistema distribuído para coleta de dados de sensores IoT usando drones com:
- **SWIM Protocol** (HashiCorp Memberlist) para membership e failure detection
- **Delta CRDTs** para replicação eventual consistente de dados
- **Anti-entropy Gossip** para disseminação de dados entre peers

## 🚀 Migração para SWIM

O sistema foi migrado do gerenciamento manual de peers para usar o protocolo SWIM (Scalable Weakly-consistent Infection-style Process Group Membership) da HashiCorp. Isso oferece:

### ✅ Vantagens do SWIM
- **Failure Detection Automática**: Detecta falhas em ~5 segundos
- **Zero-Configuration Clustering**: Basta especificar seeds iniciais
- **Tolerância a Partições**: Reconexão automática após partições de rede
- **Escalabilidade**: Overhead constante O(1) independente do tamanho do cluster
- **Battle-tested**: Usado em produção por Consul, Nomad, Vault

### 📊 Comparação: Antes vs Depois

| Aspecto            | Sistema Manual    | SWIM               |
| ------------------ | ----------------- | ------------------ |
| Failure Detection  | ❌ Manual/Timeouts | ✅ Automático (~5s) |
| Peer Discovery     | ❌ Lista estática  | ✅ Dinâmico         |
| Network Partitions | ❌ Sem recuperação | ✅ Auto-healing     |
| Configuration      | ❌ URLs hardcoded  | ✅ Apenas seeds     |
| Maintenance        | ❌ Alta            | ✅ Mínima           |

## 🏗️ Arquitetura

```
┌─────────────────┐    SWIM Membership     ┌─────────────────┐
│   Drone A       │◄────────────────────────┤   Drone B       │
│                 │    (UDP/TCP 7946)       │                 │
│ ┌─────────────┐ │                         │ ┌─────────────┐ │
│ │ CRDT Core   │ │    δ-CRDT Gossip       │ │ CRDT Core   │ │
│ │ (Sensors)   │ │◄────────────────────────┤ │ (Sensors)   │ │
│ └─────────────┘ │   (HTTP/JSON 8080)      │ └─────────────┘ │
└─────────────────┘                         └─────────────────┘
        ▲                                            ▲
        │ HTTP API                                   │ HTTP API
        ▼                                            ▼
┌─────────────────┐                         ┌─────────────────┐
│   Sensor IoT    │                         │   Sensor IoT    │
│   (UDP Beacon)  │                         │   (UDP Beacon)  │
└─────────────────┘                         └─────────────────┘
```

### Componentes

1. **SWIM Layer** (porta 7946): Membership, failure detection, cluster formation
2. **API Layer** (porta 8080): REST API para sensores e δ-CRDT gossip
3. **CRDT Core**: Estrutura de dados replicada com convergência eventual
4. **Sensor Integration**: Auto-discovery via UDP broadcast

## 🚀 Quick Start

### Execução Local

```bash
# Construir o projeto
go build -o drone

# Primeiro drone (seed)
./drone -drone=drone-01 -port=8080

# Segundo drone (conecta ao primeiro)
./drone -drone=drone-02 -port=8081 -seeds=drone-01

# Terceiro drone
./drone -drone=drone-03 -port=8082 -seeds=drone-01
```

### Demo Completo

```bash
# Executa demo com 3 drones + failure simulation
./demo-swim.sh
```

### Docker Compose

```bash
# Cluster com 3 drones + 2 sensores
docker-compose -f docker-compose-swim.yml up
```

## 📡 API Endpoints

### CRDT Data
- `POST /sensor` - Adiciona leitura de sensor
- `GET /state` - Estado completo do CRDT
- `GET /deltas` - Deltas pendentes para gossip
- `POST /delta` - Recebe deltas de outros drones

### SWIM Membership
- `GET /members` - Lista membros do cluster
- `POST /join` - Conecta a um nó específico
- `GET /stats` - Estatísticas do drone e cluster

### Maintenance
- `POST /cleanup` - Remove deltas antigos
- `GET /stats` - Métricas detalhadas

## 🔧 Configuração

### Flags da CLI

```bash
./drone [options]

-drone string     ID único do drone (default "drone-01")
-port int         Porta da API REST (default 8080)
-swim-port int    Porta do protocolo SWIM (default 7946)
-bind string      Endereço para bind (default "0.0.0.0")
-seeds string     Nós seeds separados por vírgula (ex: "drone-01,drone-02")
-help             Mostra ajuda completa
```

### Variáveis de Ambiente (Docker)

```bash
NODE_ID=drone1          # ID do nó
API_PORT=8080          # Porta REST API
SWIM_PORT=7946         # Porta SWIM
BIND_ADDR=0.0.0.0      # Endereço bind
SEEDS=drone1,drone2    # Seeds para cluster
```

## 🧪 Testes

### Teste de Failure Detection

```bash
# Terminal 1: Start cluster
./drone -drone=drone-01 -port=8080
./drone -drone=drone-02 -port=8081 -seeds=drone-01

# Terminal 2: Monitor members
watch -n 2 'curl -s localhost:8080/members | jq'

# Terminal 3: Kill drone-02
pkill -f "drone-02"
# Observe drone-02 desaparecer da lista em ~5s
```

### Teste de Convergência CRDT

```bash
# Envia dados para drone diferentes
curl -X POST localhost:8080/sensor -d '{"sensor_id":"area1","value":65.5}'
curl -X POST localhost:8081/sensor -d '{"sensor_id":"area2","value":71.2}'

# Verifica convergência (após ~30s)
curl localhost:8080/state | jq '.state | length'  # Deve ser 2
curl localhost:8081/state | jq '.state | length'  # Deve ser 2
```

## 📊 Métricas e Monitoramento

### Endpoint /stats

```json
{
  "drone_id": "drone-01",
  "memory_stats": {
    "total_deltas": 150,
    "pending_deltas": 3,
    "latest_readings": 45
  },
  "active_peers": 2,
  "membership": {
    "node_id": "drone-01",
    "total_members": 3,
    "live_members": 2,
    "local_addr": "10.5.0.11:7946"
  }
}
```

### Logs Estruturados

```
[SWIM] Nó drone-02 (10.5.0.12:7946) se juntou ao cluster
[GOSSIP] Enviados 5 deltas para 2/2 peers (via SWIM)
[SWIM] Nó drone-02 deixou o cluster
[PULL] Merged 3 deltas de http://10.5.0.13:8080
```

## 🔧 Configurações Avançadas

### Timeouts SWIM Personalizados

```go
cfg := memberlist.DefaultLANConfig()
cfg.ProbeTimeout = 1 * time.Second     // Ping timeout
cfg.ProbeInterval = 5 * time.Second    // Ping interval
cfg.PushPullInterval = 30 * time.Second // Full sync interval
```

### Rede Custom (Docker)

```yaml
networks:
  farm-network:
    driver: bridge
    ipam:
      config:
        - subnet: 192.168.100.0/24
```

## 🐛 Troubleshooting

### Problema: Nós não descobrem uns aos outros
```bash
# Verificar connectividade SWIM
nc -zv drone-01 7946

# Verificar logs
docker logs drone1 | grep SWIM
```

### Problema: Dados não convergem
```bash
# Verificar deltas pendentes
curl localhost:8080/deltas

# Forçar pull manual
curl -X POST localhost:8080/delta -d '{"sender_id":"manual","deltas":[]}'
```

### Problema: Falha na detecção
```bash
# Verificar configuração de rede
iptables -L | grep 7946

# Verificar membership
curl localhost:8080/members | jq '.members[].status'
```

## 📚 Referências

- [HashiCorp Memberlist](https://github.com/hashicorp/memberlist)
- [SWIM Protocol Paper](https://www.cs.cornell.edu/projects/Quicksilver/public_pdfs/SWIM.pdf)
- [Delta CRDTs](https://arxiv.org/abs/1603.01529)
- [Anti-Entropy Protocols](https://zoo.cs.yale.edu/classes/cs426/2012/lab/bib/demers87epidemic.pdf)

## 🎯 Próximos Passos

- [ ] **Criptografia**: TLS para comunicação entre nós
- [ ] **Autenticação**: JWT/mTLS para segurança
- [ ] **Métricas**: Integração com Prometheus
- [ ] **Compressão**: Gzip para payloads JSON grandes
- [ ] **Vector Clocks**: Para ordenação causal robusta
