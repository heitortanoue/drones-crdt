# Sistema CRDT para Drones de Monitoramento

Este projeto implementa um sistema distribuído baseado em CRDT (Conflict-free Replicated Data Type) para drones que coletam dados de sensores de umidade do solo.

## Funcionalidades Implementadas

✅ **Delta-based CRDT**: Sistema baseado em deltas para replicação eficiente
✅ **API REST completa**: Endpoints para injeção, consulta e sincronização
✅ **Buffer de deltas**: Armazenamento de deltas pendentes para envio
✅ **Merge com pruning**: Eliminação automática de duplicatas
✅ **Gossip anti-entropy**: Sincronização automática entre peers
✅ **Handshake automático**: Entrada dinâmica de novos nós na rede
✅ **Garbage Collection**: Limpeza inteligente de dados antigos
✅ **Sistema de Manutenção**: Endpoints para monitoramento e limpeza
✅ **Last Writer Wins**: Resolução de conflitos por timestamp
✅ **Thread-safe**: Operações seguras para concorrência
✅ **Logs Estruturados**: Sistema de logging com timestamps detalhados
✅ **Testes unitários**: Cobertura completa com casos de conflito

## Arquitetura

```
├── sensor/           # Estruturas de dados e CRDT core
│   ├── sensor.go     # Definições de SensorDelta e SensorReading
│   ├── crdt.go       # Implementação do CRDT principal
│   └── sensor_test.go# Testes unitários
├── api/              # Servidor HTTP REST
│   ├── server.go     # Endpoints da API
│   └── server_test.go# Testes da API
├── gossip/           # Cliente para sincronização P2P
│   └── client.go     # Implementação do gossip
├── examples/         # Exemplos de uso
│   └── client_example.go
└── main.go           # Aplicação principal
```

## API REST

### POST /sensor
Registra uma nova leitura local:
```bash
curl -X POST http://localhost:8080/sensor \
  -H 'Content-Type: application/json' \
  -d '{"sensor_id":"talhao-3","timestamp":1651672800123,"value":23.7}'
```

### GET /deltas
Recupera deltas pendentes:
```bash
curl http://localhost:8080/deltas
```

### POST /delta
Recebe lote de deltas de outro drone:
```bash
curl -X POST http://localhost:8080/delta \
  -H 'Content-Type: application/json' \
  -d '{"sender_id":"drone-02","deltas":[...]}'
```

### GET /state
Retorna estado completo convergido:
```bash
curl http://localhost:8080/state
```

## Execução

### Drone único
```bash
go run main.go -drone=drone-01 -port=8080
```

### Rede de 3 drones com gossip
```bash
# Terminal 1 - Drone 01
go run main.go -drone=drone-01 -port=8080 \
  -peers="http://localhost:8081,http://localhost:8082" -gossip=5

# Terminal 2 - Drone 02
go run main.go -drone=drone-02 -port=8081 \
  -peers="http://localhost:8080,http://localhost:8082" -gossip=5

# Terminal 3 - Drone 03
go run main.go -drone=drone-03 -port=8082 \
  -peers="http://localhost:8080,http://localhost:8081" -gossip=5
```

## Testes

```bash
# Executa todos os testes
go test ./...

# Testes com benchmark
go test -bench=. ./...

# Testes com cobertura
go test -cover ./...
```

## Formato de Dados

### SensorDelta
```json
{
  "drone_id": "drone-01",
  "sensor_id": "talhao-3",
  "timestamp": 1651672800123,
  "value": 23.7
}
```

### DeltaBatch
```json
{
  "sender_id": "drone-02",
  "deltas": [
    {"drone_id": "drone-02", "sensor_id": "talhao-3", ...},
    {"drone_id": "drone-02", "sensor_id": "talhao-5", ...}
  ]
}
```

## Características do CRDT

- **Convergência**: Todos os drones convergem para o mesmo estado
- **Comutatividade**: Ordem de aplicação dos deltas não importa
- **Idempotência**: Reaplicar deltas não altera o resultado
- **Associatividade**: Merge pode ser feito em qualquer ordem
- **Tolerância a partições**: Funciona mesmo com conectividade intermitente

## Resolução de Conflitos

- **Last Writer Wins (LWW)**: Para mesmo sensor, timestamp mais recente vence
- **Add Wins**: Inserções vencem remoções concorrentes
- **Deduplicação**: Deltas duplicados são automaticamente ignorados

## Monitoramento

O sistema inclui logs detalhados:
- Timestamp de recebimento de deltas
- Contadores de merge bem-sucedidos
- Status do gossip entre peers
- Detecção de duplicatas e conflitos

## Próximos Passos (Cronograma)

- [x] ~~Implementar AddDelta e buffer~~
- [x] ~~Merge com pruning de duplicatas~~
- [x] ~~Testes unitários~~
- [x] ~~API REST completa~~
- [x] ~~Gossip anti-entropy~~
- [ ] Benchmark para 1000+ deltas
- [ ] Handshake para novos nós
- [ ] Remoção de tombstones
- [ ] Suporte a re-join após falha
- [ ] Logs com timestamp de recebimento
