package sensor

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

// SensorGenerator gera leituras automáticas de sensor (Requisito F1)
type SensorGenerator struct {
	droneID     string
	deltaSet    *DeltaSet
	interval    time.Duration
	running     bool
	stopCh      chan struct{}
	sensorAreas []string // Áreas de sensores simuladas
}

// NewSensorGenerator cria um novo gerador de leituras automáticas
func NewSensorGenerator(droneID string, deltaSet *DeltaSet, interval time.Duration) *SensorGenerator {
	// Define áreas de sensores simuladas para este drone
	areas := []string{
		fmt.Sprintf("area-%s-A", droneID),
		fmt.Sprintf("area-%s-B", droneID),
		fmt.Sprintf("area-%s-C", droneID),
	}

	return &SensorGenerator{
		droneID:     droneID,
		deltaSet:    deltaSet,
		interval:    interval,
		running:     false,
		stopCh:      make(chan struct{}),
		sensorAreas: areas,
	}
}

// Start inicia a geração automática de leituras (Requisito F1)
func (sg *SensorGenerator) Start() {
	if sg.running {
		return
	}

	sg.running = true
	log.Printf("[GENERATOR] Iniciando coleta automática para %s (intervalo: %v)", sg.droneID, sg.interval)

	go sg.generateLoop()
}

// Stop para a geração automática
func (sg *SensorGenerator) Stop() {
	if !sg.running {
		return
	}

	sg.running = false
	close(sg.stopCh)
	log.Printf("[GENERATOR] Parando coleta automática para %s", sg.droneID)
}

// generateLoop executa o loop principal de geração
func (sg *SensorGenerator) generateLoop() {
	ticker := time.NewTicker(sg.interval)
	defer ticker.Stop()

	// Gera uma leitura inicial imediatamente
	sg.generateReading()

	for {
		select {
		case <-ticker.C:
			sg.generateReading()
		case <-sg.stopCh:
			log.Printf("[GENERATOR] Loop de geração finalizado para %s", sg.droneID)
			return
		}
	}
}

// generateReading gera uma leitura simulada de sensor
func (sg *SensorGenerator) generateReading() {
	// Seleciona aleatoriamente uma área de sensor
	sensorID := sg.sensorAreas[rand.Intn(len(sg.sensorAreas))]

	// Gera valor simulado de umidade (0-100%)
	// Adiciona variação baseada no tempo para simular mudanças realistas
	baseValue := 40.0 + 30.0*rand.Float64()        // 40-70% base
	timeVariation := 10.0 * (0.5 - rand.Float64()) // ±5% variação
	value := baseValue + timeVariation

	// Garante que está no range válido
	if value < 0 {
		value = 0
	} else if value > 100 {
		value = 100
	}

	// Cria e adiciona o delta
	delta := NewSensorDelta(sg.droneID, sensorID, value)
	sg.deltaSet.Add(delta)

	log.Printf("[GENERATOR] %s gerou leitura: %s=%.2f%% (ID: %s)",
		sg.droneID, sensorID, value, delta.ID.String()[:8])
}

// GetStats retorna estatísticas do gerador
func (sg *SensorGenerator) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"drone_id":     sg.droneID,
		"running":      sg.running,
		"interval_sec": sg.interval.Seconds(),
		"sensor_areas": sg.sensorAreas,
		"total_areas":  len(sg.sensorAreas),
	}
}

// SetInterval atualiza o intervalo de geração
func (sg *SensorGenerator) SetInterval(interval time.Duration) {
	sg.interval = interval
	log.Printf("[GENERATOR] Intervalo atualizado para %s: %v", sg.droneID, interval)
}

// AddSensorArea adiciona uma nova área de sensor
func (sg *SensorGenerator) AddSensorArea(area string) {
	sg.sensorAreas = append(sg.sensorAreas, area)
	log.Printf("[GENERATOR] Nova área adicionada para %s: %s", sg.droneID, area)
}

// GetSensorAreas retorna as áreas de sensores configuradas
func (sg *SensorGenerator) GetSensorAreas() []string {
	areas := make([]string, len(sg.sensorAreas))
	copy(areas, sg.sensorAreas)
	return areas
}
