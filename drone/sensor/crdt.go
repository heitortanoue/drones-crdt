package sensor

import (
	"sync"
	"time"
)

// SensorCRDT implementa um CRDT baseado em deltas para leituras de sensores
type SensorCRDT struct {
	droneID      string                  // ID deste drone
	deltas       map[string]*SensorDelta // todos os deltas conhecidos (key -> delta)
	pendingBuf   []SensorDelta           // buffer de deltas pendentes para envio
	latestByArea map[string]*SensorDelta // última leitura por área de sensor
	mutex        sync.RWMutex            // proteção para concorrência
}

// NewSensorCRDT cria uma nova instância do CRDT de sensores
func NewSensorCRDT(droneID string) *SensorCRDT {
	return &SensorCRDT{
		droneID:      droneID,
		deltas:       make(map[string]*SensorDelta),
		pendingBuf:   make([]SensorDelta, 0),
		latestByArea: make(map[string]*SensorDelta),
	}
}

// AddDelta adiciona uma nova leitura de sensor e retorna o delta gerado
func (s *SensorCRDT) AddDelta(reading SensorReading) *SensorDelta {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Cria o delta
	delta := &SensorDelta{
		DroneID:   s.droneID,
		SensorID:  reading.SensorID,
		Timestamp: reading.Timestamp,
		Value:     reading.Value,
	}

	// Adiciona ao conjunto de deltas conhecidos
	key := delta.Key()
	s.deltas[key] = delta

	// Adiciona ao buffer de pendentes
	s.pendingBuf = append(s.pendingBuf, *delta)

	// Atualiza o último valor por área (LWW - Last Writer Wins)
	if current, exists := s.latestByArea[reading.SensorID]; !exists || delta.IsNewerThan(current) {
		s.latestByArea[reading.SensorID] = delta
	}

	return delta
}

// GetPendingDeltas retorna todos os deltas pendentes para envio
func (s *SensorCRDT) GetPendingDeltas() []SensorDelta {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Cria uma cópia para evitar modificações concorrentes
	pending := make([]SensorDelta, len(s.pendingBuf))
	copy(pending, s.pendingBuf)
	return pending
}

// ClearPendingDeltas limpa o buffer de deltas pendentes
func (s *SensorCRDT) ClearPendingDeltas() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.pendingBuf = s.pendingBuf[:0] // limpa mas mantém capacidade
}

// Merge aplica um lote de deltas remotos com pruning de duplicatas
func (s *SensorCRDT) Merge(batch DeltaBatch) int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	mergedCount := 0

	for _, remoteDelta := range batch.Deltas {
		key := remoteDelta.Key()

		// Pruning: ignora se já conhecemos este delta
		if _, exists := s.deltas[key]; exists {
			continue
		}

		// Adiciona ao conjunto de deltas conhecidos
		s.deltas[key] = &remoteDelta
		mergedCount++

		// Atualiza último valor por área (LWW)
		if current, exists := s.latestByArea[remoteDelta.SensorID]; !exists || remoteDelta.IsNewerThan(current) {
			s.latestByArea[remoteDelta.SensorID] = &remoteDelta
		}
	}

	return mergedCount
}

// GetState retorna o estado completo atual (todas as leituras aplicadas)
func (s *SensorCRDT) GetState() []SensorDelta {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	state := make([]SensorDelta, 0, len(s.deltas))
	for _, delta := range s.deltas {
		state = append(state, *delta)
	}
	return state
}

// GetLatestReadings retorna as leituras mais recentes por área de sensor
func (s *SensorCRDT) GetLatestReadings() map[string]*SensorDelta {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	latest := make(map[string]*SensorDelta)
	for area, delta := range s.latestByArea {
		latest[area] = delta
	}
	return latest
}

// GetTotalDeltasCount retorna o número total de deltas conhecidos
func (s *SensorCRDT) GetTotalDeltasCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.deltas)
}

// CleanupOldDeltas remove deltas antigos com base em um limite de tempo (em milissegundos)
func (s *SensorCRDT) CleanupOldDeltas(limitTimestamp int64) int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	removedCount := 0
	for key, delta := range s.deltas {
		if delta.Timestamp < limitTimestamp {
			delete(s.deltas, key)
			removedCount++
		}
	}
	return removedCount
}

// CleanupOldDeltasByAge remove deltas mais antigos que a duração especificada
func (s *SensorCRDT) CleanupOldDeltasByAge(maxAge time.Duration) int {
	limitTimestamp := time.Now().Add(-maxAge).UnixMilli()
	return s.CleanupOldDeltas(limitTimestamp)
}

// TrimToMaxSize mantém apenas os N deltas mais recentes
func (s *SensorCRDT) TrimToMaxSize(maxSize int) int {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.deltas) <= maxSize {
		return 0
	}

	// Coleta todos os deltas com timestamps
	type deltaWithKey struct {
		key   string
		delta *SensorDelta
	}

	deltas := make([]deltaWithKey, 0, len(s.deltas))
	for key, delta := range s.deltas {
		deltas = append(deltas, deltaWithKey{key: key, delta: delta})
	}

	// Ordena por timestamp (mais recentes primeiro)
	for i := 0; i < len(deltas)-1; i++ {
		for j := i + 1; j < len(deltas); j++ {
			if deltas[i].delta.Timestamp < deltas[j].delta.Timestamp {
				deltas[i], deltas[j] = deltas[j], deltas[i]
			}
		}
	}

	// Remove os mais antigos
	removedCount := 0
	for i := maxSize; i < len(deltas); i++ {
		delete(s.deltas, deltas[i].key)
		removedCount++
	}

	return removedCount
}

// TrimPendingBuffer reduz o buffer de pendentes para um tamanho máximo
func (s *SensorCRDT) TrimPendingBuffer(maxSize int) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.pendingBuf) > maxSize {
		s.pendingBuf = s.pendingBuf[len(s.pendingBuf)-maxSize:]
	}
}

// GetMemoryStats retorna estatísticas de uso de memória
func (s *SensorCRDT) GetMemoryStats() map[string]uint64 {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return map[string]uint64{
		"total_deltas":    uint64(len(s.deltas)),
		"pending_deltas":  uint64(len(s.pendingBuf)),
		"latest_readings": uint64(len(s.latestByArea)),
	}
}
