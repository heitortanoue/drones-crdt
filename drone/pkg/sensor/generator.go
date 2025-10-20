package sensor

import (
	"log"
	"math/rand"
	"time"

	"github.com/heitortanoue/tcc/pkg/crdt"
	"github.com/heitortanoue/tcc/pkg/state"
)

type FireSensorGenerator struct {
	sensorID            string
	sensor              *FireSensor
	interval            time.Duration
	running             bool
	stopCh              chan struct{}
	gridSizeX           int
	gridSizeY           int
	confidenceThreshold float64
}

// NewFireSensorGenerator creates a new fire detection generator
func NewFireSensorGenerator(sensorID string, interval time.Duration, gridSizeX, gridSizeY int, confidenceThreshold float64) *FireSensorGenerator {
	return &FireSensorGenerator{
		sensorID:            sensorID,
		interval:            interval,
		running:             false,
		stopCh:              make(chan struct{}),
		gridSizeX:           gridSizeX,
		gridSizeY:           gridSizeY,
		confidenceThreshold: confidenceThreshold,
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
			fsg.generateDetection()
		case <-fsg.stopCh:
			log.Printf("[GENERATOR] Detection loop terminated for %s", fsg.sensorID)
			return
		}
	}
}

// generateDetection creates a simulated fire detection
// Randomly decides to ADD or REMOVE fires near the drone's position
func (fsg *FireSensorGenerator) generateDetection() {
	if fsg.sensor == nil {
		return
	}

	posX, posY := fsg.sensor.GetPosition()
	fireRadius := 10

	// Generate random position within fireRadius
	offsetX := rand.Intn(2*fireRadius+1) - fireRadius
	offsetY := rand.Intn(2*fireRadius+1) - fireRadius

	x := posX + offsetX
	y := posY + offsetY

	// Clamp to grid boundaries
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

	cell := crdt.Cell{X: x, Y: y}

	// Simple uniform distribution: 0-100%
	confidence := rand.Float64() * 100.0

	if confidence < fsg.confidenceThreshold {
		// Low confidence - ignore
		log.Printf("[GENERATOR] %s IGNORE: (%d,%d) confidence=%.1f%% (below threshold %.1f%%)",
			fsg.sensorID, x, y, confidence, fsg.confidenceThreshold)
		return
	}

	// Check if fire already exists in state
	activeFires := state.GetActiveFires()
	fireExists := false
	for _, fire := range activeFires {
		if fire.Cell.X == cell.X && fire.Cell.Y == cell.Y {
			fireExists = true
			break
		}
	}

	// Fire already detected at this cell - REMOVE it
	if fireExists {
		state.RemoveFire(cell)
		log.Printf("[GENERATOR] %s REMOVE: (%d,%d) - fire already detected, removing",
			fsg.sensorID, x, y)
		return
	}

	reading := FireReading{
		X:          x,
		Y:          y,
		Confidence: confidence,
		Timestamp:  time.Now().UnixMilli(),
		SensorID:   fsg.sensorID,
	}

	fsg.sensor.AddReading(reading)

	meta := crdt.FireMeta{
		Timestamp:  reading.Timestamp,
		Confidence: reading.Confidence,
		DetectedBy: fsg.sensorID,
	}

	state.AddFire(cell, meta)

	log.Printf("[GENERATOR] %s ADD: (%d,%d) confidence=%.1f%%",
		fsg.sensorID, x, y, confidence)
}

// GetStats returns statistics for the generator
func (fsg *FireSensorGenerator) GetStats() map[string]interface{} {

	return map[string]interface{}{
		"sensor_id":            fsg.sensorID,
		"running":              fsg.running,
		"interval_sec":         fsg.interval.Seconds(),
		"confidence_threshold": fsg.confidenceThreshold,
		"grid_size":            map[string]int{"x": fsg.gridSizeX, "y": fsg.gridSizeY},
	}
}
