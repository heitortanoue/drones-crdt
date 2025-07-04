package sensor

import (
	"sync"
	"time"
)

// FireReading representa uma leitura de detecção de incêndio
type FireReading struct {
	X          int     `json:"x"`          // Coordenada X da célula
	Y          int     `json:"y"`          // Coordenada Y da célula
	Confidence float64 `json:"confidence"` // Nível de confiança (0-100%)
	Timestamp  int64   `json:"timestamp"`  // Timestamp em milissegundos
	SensorID   string  `json:"sensor_id"`  // ID do sensor que fez a leitura
}

// FireSensor representa um sensor simples que coleta leituras
type FireSensor struct {
	readings  []FireReading        // Lista de leituras acumuladas
	generator *FireSensorGenerator // Gerador automático
	sensorID  string               // ID único do sensor
	mutex     sync.RWMutex         // Proteção para concorrência
}

// NewFireSensor cria uma nova instância do sensor de incêndio
func NewFireSensor(sensorID string, sampleInterval time.Duration) *FireSensor {
	generator := NewFireSensorGenerator(sensorID, sampleInterval)

	sensor := &FireSensor{
		readings:  make([]FireReading, 0),
		generator: generator,
		sensorID:  sensorID,
	}

	// Configura a referência circular para o gerador
	generator.SetSensor(sensor)

	return sensor
}

// Start inicia a coleta automática de dados
func (fs *FireSensor) Start() {
	fs.generator.Start()
}

// Stop para a coleta automática
func (fs *FireSensor) Stop() {
	fs.generator.Stop()
}

// AddReading adiciona uma leitura à lista (usado pelo gerador e manualmente)
func (fs *FireSensor) AddReading(reading FireReading) {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	// Adiciona o ID do sensor se não estiver definido
	if reading.SensorID == "" {
		reading.SensorID = fs.sensorID
	}

	fs.readings = append(fs.readings, reading)
}

// AddManualReading adiciona uma leitura manual (mais para testes)
func (fs *FireSensor) AddManualReading(x, y int, confidence float64) {
	reading := FireReading{
		X:          x,
		Y:          y,
		Confidence: confidence,
		Timestamp:  GenerateTimestamp(),
		SensorID:   fs.sensorID,
	}
	fs.AddReading(reading)
}

// GetReadings retorna todas as leituras acumuladas
func (fs *FireSensor) GetReadings() []FireReading {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	// Retorna uma cópia para evitar modificações concorrentes
	readings := make([]FireReading, len(fs.readings))
	copy(readings, fs.readings)
	return readings
}

// GetAndClearReadings retorna todas as leituras e limpa a lista (para envio ao drone)
func (fs *FireSensor) GetAndClearReadings() []FireReading {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	// Copia as leituras
	readings := make([]FireReading, len(fs.readings))
	copy(readings, fs.readings)

	// Limpa a lista
	fs.readings = fs.readings[:0]

	return readings
}

// GetStats retorna estatísticas do sensor
func (fs *FireSensor) GetStats() map[string]interface{} {
	fs.mutex.RLock()
	readingCount := len(fs.readings)
	fs.mutex.RUnlock()

	return map[string]interface{}{
		"sensor_id":     fs.sensorID,
		"reading_count": readingCount,
		"generator":     fs.generator.GetStats(),
	}
}

// GenerateTimestamp cria um timestamp atual em milissegundos
func GenerateTimestamp() int64 {
	return time.Now().UnixMilli()
}
