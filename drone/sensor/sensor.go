package sensor

import (
	"time"
)

// SensorDelta representa uma leitura de sensor como delta CRDT
type SensorDelta struct {
	DroneID   string  `json:"drone_id"`  // ID único do drone
	SensorID  string  `json:"sensor_id"` // ID do sensor/área
	Timestamp int64   `json:"timestamp"` // Unix epoch em ms
	Value     float64 `json:"value"`     // Valor da umidade em %
}

// DeltaBatch representa um lote de deltas para envio
type DeltaBatch struct {
	SenderID string        `json:"sender_id"` // quem está enviando
	Deltas   []SensorDelta `json:"deltas"`    // array de deltas
}

// Key gera uma chave única para o delta (usado para deduplicação)
func (d *SensorDelta) Key() string {
	return d.DroneID + "#" + d.SensorID + "#" + itoa(d.Timestamp)
}

// IsNewerThan verifica se este delta é mais recente que outro para o mesmo sensor
func (d *SensorDelta) IsNewerThan(other *SensorDelta) bool {
	return d.SensorID == other.SensorID && d.Timestamp > other.Timestamp
}

// SensorReading representa uma leitura de sensor para entrada de API
type SensorReading struct {
	SensorID  string  `json:"sensor_id"`
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

// GenerateTimestamp cria um timestamp atual em milissegundos
func GenerateTimestamp() int64 {
	return time.Now().UnixMilli()
}

// ---------- Utilitário -------------------------------------------
func itoa(i int64) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var digits [20]byte
	pos := len(digits)
	for i > 0 {
		pos--
		digits[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		digits[pos] = '-'
	}
	return string(digits[pos:])
}
