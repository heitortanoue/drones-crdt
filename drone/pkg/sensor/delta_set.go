package sensor

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// SensorDelta representa uma leitura de sensor como delta CRDT (Fase 2)
type SensorDelta struct {
	ID        uuid.UUID `json:"id"`        // UUID único do delta
	SensorID  string    `json:"sensor_id"` // ID do sensor/área
	Timestamp int64     `json:"timestamp"` // Unix epoch em ms
	Value     float64   `json:"value"`     // Valor da umidade em %
	DroneID   string    `json:"drone_id"`  // ID único do drone
}

// DeltaSet implementa Delta-Set CRDT específico (uuid.UUID → SensorDelta)
type DeltaSet struct {
	deltas map[uuid.UUID]SensorDelta // mapa UUID -> SensorDelta
	mutex  sync.RWMutex              // proteção para concorrência
}

// NewDeltaSet cria um novo Delta-Set vazio
func NewDeltaSet() *DeltaSet {
	return &DeltaSet{
		deltas: make(map[uuid.UUID]SensorDelta),
	}
}

// Add adiciona um novo delta ao conjunto
func (ds *DeltaSet) Add(delta SensorDelta) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()
	ds.deltas[delta.ID] = delta
}

// Apply aplica um delta ao conjunto (Método Apply(Δ) do requisito F2)
func (ds *DeltaSet) Apply(delta SensorDelta) bool {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	// Verifica se já existe
	if _, exists := ds.deltas[delta.ID]; exists {
		return false // Já existe, sem mudança
	}

	ds.deltas[delta.ID] = delta
	return true // Novo delta aplicado
}

// Merge combina outro DeltaSet com este (Método Merge(other) do requisito F2)
func (ds *DeltaSet) Merge(other *DeltaSet) int {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	other.mutex.RLock()
	defer other.mutex.RUnlock()

	mergedCount := 0
	for id, delta := range other.deltas {
		if _, exists := ds.deltas[id]; !exists {
			ds.deltas[id] = delta
			mergedCount++
		}
	}

	return mergedCount
}

// GetAll retorna todos os deltas no conjunto
func (ds *DeltaSet) GetAll() []SensorDelta {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	result := make([]SensorDelta, 0, len(ds.deltas))
	for _, delta := range ds.deltas {
		result = append(result, delta)
	}

	return result
}

// Contains verifica se um delta específico existe
func (ds *DeltaSet) Contains(id uuid.UUID) bool {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	_, exists := ds.deltas[id]
	return exists
}

// GetByIDs retorna deltas para uma lista de IDs
func (ds *DeltaSet) GetByIDs(ids []uuid.UUID) []SensorDelta {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	result := make([]SensorDelta, 0, len(ids))
	for _, id := range ids {
		if delta, exists := ds.deltas[id]; exists {
			result = append(result, delta)
		}
	}

	return result
}

// GetMissingIDs retorna IDs que não estão neste conjunto
func (ds *DeltaSet) GetMissingIDs(ids []uuid.UUID) []uuid.UUID {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	missing := make([]uuid.UUID, 0)
	for _, id := range ids {
		if _, exists := ds.deltas[id]; !exists {
			missing = append(missing, id)
		}
	}

	return missing
}

// GetAllIDs retorna todos os IDs dos deltas
func (ds *DeltaSet) GetAllIDs() []uuid.UUID {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	ids := make([]uuid.UUID, 0, len(ds.deltas))
	for id := range ds.deltas {
		ids = append(ids, id)
	}

	return ids
}

// Size retorna o número de deltas no conjunto
func (ds *DeltaSet) Size() int {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	return len(ds.deltas)
}

// GetLatestBySensor retorna o delta mais recente por sensor
func (ds *DeltaSet) GetLatestBySensor() map[string]SensorDelta {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	latest := make(map[string]SensorDelta)

	for _, delta := range ds.deltas {
		if current, exists := latest[delta.SensorID]; !exists || delta.Timestamp > current.Timestamp {
			latest[delta.SensorID] = delta
		}
	}

	return latest
}

// CleanupOldDeltas remove deltas mais antigos que o timestamp limite
func (ds *DeltaSet) CleanupOldDeltas(limitTimestamp int64) int {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	removedCount := 0
	for id, delta := range ds.deltas {
		if delta.Timestamp < limitTimestamp {
			delete(ds.deltas, id)
			removedCount++
		}
	}

	return removedCount
}

// GetStats retorna estatísticas do Delta-Set
func (ds *DeltaSet) GetStats() map[string]interface{} {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	sensorCount := make(map[string]int)
	var oldestTimestamp, newestTimestamp int64

	for _, delta := range ds.deltas {
		sensorCount[delta.SensorID]++

		if oldestTimestamp == 0 || delta.Timestamp < oldestTimestamp {
			oldestTimestamp = delta.Timestamp
		}
		if delta.Timestamp > newestTimestamp {
			newestTimestamp = delta.Timestamp
		}
	}

	return map[string]interface{}{
		"total_deltas":     len(ds.deltas),
		"unique_sensors":   len(sensorCount),
		"oldest_timestamp": oldestTimestamp,
		"newest_timestamp": newestTimestamp,
		"sensor_breakdown": sensorCount,
	}
}

// NewSensorDelta cria um novo SensorDelta com timestamp atual
func NewSensorDelta(droneID, sensorID string, value float64) SensorDelta {
	return SensorDelta{
		ID:        uuid.New(),
		SensorID:  sensorID,
		Timestamp: time.Now().UnixMilli(),
		Value:     value,
		DroneID:   droneID,
	}
}
