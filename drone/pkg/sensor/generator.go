package sensor

import (
	"log"
	"math/rand"
	"time"

	"github.com/heitortanoue/tcc/pkg/crdt"
	"github.com/heitortanoue/tcc/pkg/state"
)

// FireSensorGenerator automatically generates fire detection readings
type FireSensorGenerator struct {
	sensorID string
	sensor   *FireSensor // Reference to the sensor that will receive the readings
	interval time.Duration
	running  bool
	stopCh   chan struct{}
	gridSize int // Size of the coverage grid (e.g., 10x10)
	baseX    int // Base X coordinate for this sensor
	baseY    int // Base Y coordinate for this sensor
}

// NewFireSensorGenerator creates a new fire detection generator
func NewFireSensorGenerator(sensorID string, interval time.Duration) *FireSensorGenerator {
	// Each sensor covers a specific grid area
	hash := hashString(sensorID)
	gridSize := 10
	baseX := (hash % 5) * gridSize       // 5 horizontal regions
	baseY := ((hash / 5) % 5) * gridSize // 5 vertical regions

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

// SetSensor sets the reference to the FireSensor that will store the readings
func (fsg *FireSensorGenerator) SetSensor(sensor *FireSensor) {
	fsg.sensor = sensor
}

// Start begins automatic fire detection generation
func (fsg *FireSensorGenerator) Start() {
	if fsg.running {
		return
	}

	fsg.running = true
	log.Printf("[FIRE-GENERATOR] Starting automatic detection for %s (interval: %v)", fsg.sensorID, fsg.interval)

	go fsg.generateLoop()
}

// Stop halts automatic generation
func (fsg *FireSensorGenerator) Stop() {
	if !fsg.running {
		return
	}

	fsg.running = false
	close(fsg.stopCh)
	log.Printf("[FIRE-GENERATOR] Stopping automatic detection for %s", fsg.sensorID)
}

// generateLoop runs the main generation loop
func (fsg *FireSensorGenerator) generateLoop() {
	ticker := time.NewTicker(fsg.interval)
	defer ticker.Stop()

	// Generate an initial detection immediately
	fsg.generateDetection()

	for {
		select {
		case <-ticker.C:
			fsg.generateDetection()
		case <-fsg.stopCh:
			log.Printf("[FIRE-GENERATOR] Detection loop terminated for %s", fsg.sensorID)
			return
		}
	}
}

// generateDetection creates a simulated fire detection
func (fsg *FireSensorGenerator) generateDetection() {
	if fsg.sensor == nil {
		return // No sensor configured
	}

	// Generate random coordinates within the sensor’s coverage area
	x := fsg.baseX + rand.Intn(fsg.gridSize)
	y := fsg.baseY + rand.Intn(fsg.gridSize)

	// Generate confidence level (mostly low, occasionally high)
	var confidence float64
	if rand.Float64() < 0.1 { // 10% chance of high confidence detection
		confidence = 70.0 + rand.Float64()*30.0 // 70–100%
	} else { // 90% chance of low confidence detection
		confidence = 10.0 + rand.Float64()*40.0 // 10–50%
	}

	// Create the reading
	reading := FireReading{
		X:          x,
		Y:          y,
		Confidence: confidence,
		Timestamp:  time.Now().UnixMilli(),
		SensorID:   fsg.sensorID,
	}

	// Add to sensor’s internal list
	fsg.sensor.AddReading(reading)

	// Add to global state for dissemination
	var cell crdt.Cell
	cell.X = reading.X
	cell.Y = reading.Y
	var meta crdt.FireMeta
	meta.Timestamp = reading.Timestamp
	meta.Confidence = reading.Confidence

	state.AddFire(cell, meta)

	log.Printf("[FIRE-GENERATOR] %s detected: (%d,%d) confidence=%.1f%% - added to global state",
		fsg.sensorID, x, y, confidence)
}

// SetInterval updates the generation interval
func (fsg *FireSensorGenerator) SetInterval(interval time.Duration) {
	fsg.interval = interval
	log.Printf("[FIRE-GENERATOR] Interval updated for %s: %v", fsg.sensorID, interval)
}

// GetStats returns statistics for the generator
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

// hashString creates a simple hash of a string for area distribution
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