package sensor

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/heitortanoue/tcc/pkg/crdt"
	"github.com/heitortanoue/tcc/pkg/state"
)

type FireSensorGenerator struct {
	sensorID  string
	sensor    *FireSensor
	interval  time.Duration
	running   bool
	stopCh    chan struct{}
	gridSizeX int
	gridSizeY int

	activeFires map[crdt.Cell]time.Time
	fireMutex   sync.RWMutex
}

// NewFireSensorGenerator creates a new fire detection generator
func NewFireSensorGenerator(sensorID string, interval time.Duration, gridSizeX, gridSizeY int) *FireSensorGenerator {
	return &FireSensorGenerator{
		sensorID:    sensorID,
		interval:    interval,
		running:     false,
		stopCh:      make(chan struct{}),
		gridSizeX:   gridSizeX,
		gridSizeY:   gridSizeY,
		activeFires: make(map[crdt.Cell]time.Time),
	}
}

// SetSensor sets the reference to the FireSensor
func (fsg *FireSensorGenerator) SetSensor(sensor *FireSensor) {
	fsg.sensor = sensor
}

// Start begins automatic fire detection generation
func (fsg *FireSensorGenerator) Start() {
	if fsg.running {
		return
	}

	fsg.running = true
	log.Printf("[GENERATOR] Starting automatic detection for %s (interval: %v)",
		fsg.sensorID, fsg.interval)

	go fsg.generateLoop()
}

// Stop halts automatic generation
func (fsg *FireSensorGenerator) Stop() {
	if !fsg.running {
		return
	}

	fsg.running = false
	close(fsg.stopCh)
	log.Printf("[GENERATOR] Stopping automatic detection for %s", fsg.sensorID)
}

// generateLoop runs the main generation loop
func (fsg *FireSensorGenerator) generateLoop() {
	ticker := time.NewTicker(fsg.interval)
	defer ticker.Stop()

	// Initial detection
	fsg.generateDetection()

	for {
		select {
		case <-ticker.C:
			// Randomly decide: add new fire or remove existing one
			if rand.Float64() < 0.7 { // 70% chance to add
				fsg.generateDetection()
			} else { // 30% chance to remove
				fsg.removeRandomFire()
			}
		case <-fsg.stopCh:
			log.Printf("[GENERATOR] Detection loop terminated for %s", fsg.sensorID)
			return
		}
	}
}

// generateDetection creates a simulated fire detection (ADD)
func (fsg *FireSensorGenerator) generateDetection() {
	if fsg.sensor == nil {
		return
	}

	posX, posY := fsg.sensor.GetPosition()

	fireRadius := 10
	offsetX := rand.Intn(2*fireRadius+1) - fireRadius
	offsetY := rand.Intn(2*fireRadius+1) - fireRadius

	x := posX + offsetX
	y := posY + offsetY

	if x < 0 {
		x = 0
	}
	if x >= fsg.gridSizeX {
		x = fsg.gridSizeX - 1
	}
	if y < 0 {
		y = 0
	}
	if y >= fsg.gridSizeY {
		y = fsg.gridSizeY - 1
	}

	var confidence float64
	if rand.Float64() < 0.1 {
		confidence = 70.0 + rand.Float64()*30.0
	} else {
		confidence = 10.0 + rand.Float64()*40.0
	}

	reading := FireReading{
		X:          x,
		Y:          y,
		Confidence: confidence,
		Timestamp:  time.Now().UnixMilli(),
		SensorID:   fsg.sensorID,
	}

	fsg.sensor.AddReading(reading)

	cell := crdt.Cell{X: reading.X, Y: reading.Y}
	meta := crdt.FireMeta{
		Timestamp:  reading.Timestamp,
		Confidence: reading.Confidence,
	}

	state.AddFire(cell, meta)

	// Track this fire for potential removal
	fsg.fireMutex.Lock()
	fsg.activeFires[cell] = time.Now()
	fsg.fireMutex.Unlock()

	log.Printf("[GENERATOR] %s ADD: (%d,%d) confidence=%.1f%%",
		fsg.sensorID, x, y, confidence)
}

// removeRandomFire removes a random active fire (REMOVE)
func (fsg *FireSensorGenerator) removeRandomFire() {
	fsg.fireMutex.Lock()
	defer fsg.fireMutex.Unlock()

	if len(fsg.activeFires) == 0 {
		return
	}

	// Select random fire to remove
	var selectedCell crdt.Cell
	i := rand.Intn(len(fsg.activeFires))
	for cell := range fsg.activeFires {
		if i == 0 {
			selectedCell = cell
			break
		}
		i--
	}

	// Remove from global state
	state.RemoveFire(selectedCell)
	delete(fsg.activeFires, selectedCell)

	log.Printf("[GENERATOR] %s REMOVE: (%d,%d) - fire extinguished",
		fsg.sensorID, selectedCell.X, selectedCell.Y)
}

// GetStats returns statistics for the generator
func (fsg *FireSensorGenerator) GetStats() map[string]interface{} {
	fsg.fireMutex.RLock()
	activeCount := len(fsg.activeFires)
	fsg.fireMutex.RUnlock()

	return map[string]interface{}{
		"sensor_id":    fsg.sensorID,
		"running":      fsg.running,
		"interval_sec": fsg.interval.Seconds(),
		"grid_size":    map[string]int{"x": fsg.gridSizeX, "y": fsg.gridSizeY},
		"active_fires": activeCount,
	}
}

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
