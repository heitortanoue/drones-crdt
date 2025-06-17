package sensor

import (
	"time"

	"github.com/google/uuid"
)

// SensorReading representa uma leitura de sensor para entrada de API
type SensorReading struct {
	SensorID  string  `json:"sensor_id"`
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// DeltaBatch representa um lote de deltas para transmissão
type DeltaBatch struct {
	SenderID string        `json:"sender_id"` // ID do drone que envia
	Deltas   []SensorDelta `json:"deltas"`    // Array de deltas
}

// SensorAPI representa a interface principal para o sistema de sensores
type SensorAPI struct {
	deltaSet  *DeltaSet
	generator *SensorGenerator
	droneID   string
}

// NewSensorAPI cria uma nova instância da API de sensores
func NewSensorAPI(droneID string, sampleInterval time.Duration) *SensorAPI {
	deltaSet := NewDeltaSet()
	generator := NewSensorGenerator(droneID, deltaSet, sampleInterval)

	return &SensorAPI{
		deltaSet:  deltaSet,
		generator: generator,
		droneID:   droneID,
	}
}

// Start inicia a coleta automática de dados
func (sa *SensorAPI) Start() {
	sa.generator.Start()
}

// Stop para a coleta automática
func (sa *SensorAPI) Stop() {
	sa.generator.Stop()
}

// AddManualReading adiciona uma leitura manual via API
func (sa *SensorAPI) AddManualReading(reading SensorReading) SensorDelta {
	delta := SensorDelta{
		ID:        uuid.New(),
		SensorID:  reading.SensorID,
		Timestamp: reading.Timestamp,
		Value:     reading.Value,
		DroneID:   sa.droneID,
	}

	sa.deltaSet.Add(delta)
	return delta
}

// GetDeltaSet retorna referência para o DeltaSet
func (sa *SensorAPI) GetDeltaSet() *DeltaSet {
	return sa.deltaSet
}

// GetGenerator retorna referência para o Generator
func (sa *SensorAPI) GetGenerator() *SensorGenerator {
	return sa.generator
}

// GetState retorna todos os deltas atuais
func (sa *SensorAPI) GetState() []SensorDelta {
	return sa.deltaSet.GetAll()
}

// GetLatestReadings retorna as leituras mais recentes por sensor
func (sa *SensorAPI) GetLatestReadings() map[string]SensorDelta {
	return sa.deltaSet.GetLatestBySensor()
}

// MergeBatch aplica um lote de deltas de outro drone
func (sa *SensorAPI) MergeBatch(batch DeltaBatch) int {
	mergedCount := 0
	for _, delta := range batch.Deltas {
		if sa.deltaSet.Apply(delta) {
			mergedCount++
		}
	}
	return mergedCount
}

// GetMissingDeltas retorna IDs de deltas que não possuímos
func (sa *SensorAPI) GetMissingDeltas(haveIDs []uuid.UUID) []uuid.UUID {
	return sa.deltaSet.GetMissingIDs(haveIDs)
}

// GetDeltasByIDs retorna deltas específicos por seus IDs
func (sa *SensorAPI) GetDeltasByIDs(ids []uuid.UUID) []SensorDelta {
	return sa.deltaSet.GetByIDs(ids)
}

// GetAllDeltaIDs retorna todos os IDs de deltas que possuímos
func (sa *SensorAPI) GetAllDeltaIDs() []uuid.UUID {
	return sa.deltaSet.GetAllIDs()
}

// GetStats retorna estatísticas completas do sistema de sensores
func (sa *SensorAPI) GetStats() map[string]interface{} {
	deltaStats := sa.deltaSet.GetStats()
	generatorStats := sa.generator.GetStats()

	return map[string]interface{}{
		"drone_id":       sa.droneID,
		"delta_set":      deltaStats,
		"generator":      generatorStats,
		"latest_sensors": len(sa.GetLatestReadings()),
	}
}

// GenerateTimestamp cria um timestamp atual em milissegundos
func GenerateTimestamp() int64 {
	return time.Now().UnixMilli()
}

// CleanupOldData remove dados antigos baseado na idade
func (sa *SensorAPI) CleanupOldData(maxAge time.Duration) int {
	limitTimestamp := time.Now().Add(-maxAge).UnixMilli()
	return sa.deltaSet.CleanupOldDeltas(limitTimestamp)
}

// ApplyDelta aplica um delta individual no CRDT
func (sa *SensorAPI) ApplyDelta(delta SensorDelta) {
	sa.deltaSet.Apply(delta)
}
