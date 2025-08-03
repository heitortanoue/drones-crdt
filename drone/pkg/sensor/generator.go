package sensor

import (
	"log"
	"math/rand"
	"time"

	"github.com/heitortanoue/tcc/pkg/crdt"
	"github.com/heitortanoue/tcc/pkg/state"
)

// FireSensorGenerator gera leituras automáticas de detecção de incêndio
type FireSensorGenerator struct {
	sensorID string
	sensor   *FireSensor // Referência para o sensor que receberá as leituras
	interval time.Duration
	running  bool
	stopCh   chan struct{}
	gridSize int // Tamanho da grade de cobertura (ex: 10x10)
	baseX    int // Coordenada X base para este sensor
	baseY    int // Coordenada Y base para este sensor
}

// NewFireSensorGenerator cria um novo gerador de detecções de incêndio
func NewFireSensorGenerator(sensorID string, interval time.Duration) *FireSensorGenerator {
	// Cada sensor cobre uma área específica da grade
	hash := hashString(sensorID)
	gridSize := 10
	baseX := (hash % 5) * gridSize       // 5 regiões horizontais
	baseY := ((hash / 5) % 5) * gridSize // 5 regiões verticais

	return &FireSensorGenerator{
		sensorID: sensorID,
		interval: interval,
		running:  false,
		stopCh:   make(chan struct{}),
		gridSize: gridSize,
		baseX:    baseX,
		baseY:    baseY,
	}
}

// SetSensor define a referência do sensor para receber as leituras
func (fsg *FireSensorGenerator) SetSensor(sensor *FireSensor) {
	fsg.sensor = sensor
}

// Start inicia a geração automática de detecções de incêndio
func (fsg *FireSensorGenerator) Start() {
	if fsg.running {
		return
	}

	fsg.running = true
	log.Printf("[GENERATOR] Iniciando detecção automática para %s (intervalo: %v)", fsg.sensorID, fsg.interval)

	go fsg.generateLoop()
}

// Stop para a geração automática
func (fsg *FireSensorGenerator) Stop() {
	if !fsg.running {
		return
	}

	fsg.running = false
	close(fsg.stopCh)
	log.Printf("[GENERATOR] Parando detecção automática para %s", fsg.sensorID)
}

// generateLoop executa o loop principal de geração
func (fsg *FireSensorGenerator) generateLoop() {
	ticker := time.NewTicker(fsg.interval)
	defer ticker.Stop()

	// Gera uma detecção inicial imediatamente
	fsg.generateDetection()

	for {
		select {
		case <-ticker.C:
			fsg.generateDetection()
		case <-fsg.stopCh:
			log.Printf("[GENERATOR] Loop de detecção finalizado para %s", fsg.sensorID)
			return
		}
	}
}

// generateDetection gera uma detecção simulada de incêndio
func (fsg *FireSensorGenerator) generateDetection() {
	if fsg.sensor == nil {
		return // Sem sensor configurado
	}

	// Gera coordenadas aleatórias dentro da área do sensor
	x := fsg.baseX + rand.Intn(fsg.gridSize)
	y := fsg.baseY + rand.Intn(fsg.gridSize)

	// Gera nível de confiança (mais provável de ser baixo, ocasionalmente alto)
	var confidence float64
	if rand.Float64() < 0.9 {
		confidence = 70.0 + rand.Float64()*30.0 // 70-100%
	} else {
		confidence = 10.0 + rand.Float64()*40.0 // 10-50%
	}

	var temperature float64
	if confidence > 50.0 {
		temperature = 45.0 + rand.Float64()*55.0 // 45-100°C
	} else {
		temperature = 20.0 + rand.Float64()*25.0 // 20-45°C
	}

	// Cria a leitura
	reading := FireReading{
		X:           x,
		Y:           y,
		Confidence:  confidence,
		Temperature: temperature,
		Timestamp:   time.Now().UnixMilli(),
		SensorID:    fsg.sensorID,
	}

	// Adiciona ao sensor (lista interna)
	fsg.sensor.AddReading(reading)

	// Adiciona ao estado global para disseminação
	var cell crdt.Cell
	cell.X = reading.X
	cell.Y = reading.Y
	
	var meta crdt.FireMeta
	meta.Timestamp = reading.Timestamp
	meta.Confidence = reading.Confidence
	meta.Temperature = reading.Temperature

	state.ProcessFireReading(cell, meta)

	log.Printf("[GENERATOR] %s detectou: (%d,%d) confiança=%.1f%% - adicionado ao estado global",
		fsg.sensorID, x, y, confidence)
}

// SetInterval atualiza o intervalo de geração
func (fsg *FireSensorGenerator) SetInterval(interval time.Duration) {
	fsg.interval = interval
	log.Printf("[GENERATOR] Intervalo atualizado para %s: %v", fsg.sensorID, interval)
}

// GetStats retorna estatísticas do gerador
func (fsg *FireSensorGenerator) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"sensor_id":    fsg.sensorID,
		"running":      fsg.running,
		"interval_sec": fsg.interval.Seconds(),
		"grid_size":    fsg.gridSize,
		"base_x":       fsg.baseX,
		"base_y":       fsg.baseY,
		"coverage_area": map[string]interface{}{
			"x_range": []int{fsg.baseX, fsg.baseX + fsg.gridSize - 1},
			"y_range": []int{fsg.baseY, fsg.baseY + fsg.gridSize - 1},
		},
	}
}

// hashString cria um hash simples de uma string para distribuição
func hashString(s string) int {
	hash := 0
	for _, char := range s {
		hash = (hash*31 + int(char)) % 1000
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}
