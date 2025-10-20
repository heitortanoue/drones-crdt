package sensor

import (
	"fmt"
	"sync"
	"time"
)

// FireReading represents a fire detection reading
type FireReading struct {
	X          int     `json:"x"`          // X coordinate of the cell
	Y          int     `json:"y"`          // Y coordinate of the cell
	Confidence float64 `json:"confidence"` // Confidence level (0–100%)
	Timestamp  int64   `json:"timestamp"`  // Timestamp in milliseconds
	SensorID   string  `json:"sensor_id"`  // ID of the sensor that produced the reading
}

// FireSensor represents a simple fire sensor that collects readings
type FireSensor struct {
	readings            []FireReading
	generator           *FireSensorGenerator
	sensorID            string
	mutex               sync.RWMutex
	posX                int
	posY                int
	posMutex            sync.RWMutex
	gridSizeX           int
	gridSizeY           int
	confidenceThreshold float64
}

// NewFireSensor creates a new fire sensor instance
func NewFireSensor(sensorID string, sampleInterval time.Duration, gridSizeX, gridSizeY int, confidenceThreshold float64) *FireSensor {
	sensor := &FireSensor{
		readings:            make([]FireReading, 0),
		sensorID:            sensorID,
		posX:                gridSizeX / 2,
		posY:                gridSizeY / 2,
		gridSizeX:           gridSizeX,
		gridSizeY:           gridSizeY,
		confidenceThreshold: confidenceThreshold,
	}

	generator := NewFireSensorGenerator(sensorID, sampleInterval, gridSizeX, gridSizeY, confidenceThreshold)
	generator.SetSensor(sensor)
	sensor.generator = generator

	return sensor
}

// Start begins automatic data collection
func (fs *FireSensor) Start() {
	fs.generator.Start()
}

// Stop halts automatic data collection
func (fs *FireSensor) Stop() {
	fs.generator.Stop()
}

// AddReading appends a reading to the list (used by the generator or manually)
func (fs *FireSensor) AddReading(reading FireReading) {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	// Ensure the SensorID is set
	if reading.SensorID == "" {
		reading.SensorID = fs.sensorID
	}

	fs.readings = append(fs.readings, reading)
}

// AddManualReading adds a manual reading (mainly for testing purposes)
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

// GetReadings returns all accumulated readings
func (fs *FireSensor) GetReadings() []FireReading {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	// Return a copy to avoid concurrent modifications
	readings := make([]FireReading, len(fs.readings))
	copy(readings, fs.readings)
	return readings
}

// GetAndClearReadings returns all readings and clears the list (for drone transmission)
func (fs *FireSensor) GetAndClearReadings() []FireReading {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()

	// Copy readings
	readings := make([]FireReading, len(fs.readings))
	copy(readings, fs.readings)

	// Clear the list
	fs.readings = fs.readings[:0]

	return readings
}

// GetStats returns sensor statistics
func (fs *FireSensor) GetStats() map[string]interface{} {
	fs.mutex.RLock()
	readingCount := len(fs.readings)
	fs.mutex.RUnlock()

	fs.posMutex.RLock()
	posX, posY := fs.posX, fs.posY
	fs.posMutex.RUnlock()

	return map[string]interface{}{
		"sensor_id":     fs.sensorID,
		"reading_count": readingCount,
		"position":      map[string]int{"x": posX, "y": posY},
		"generator":     fs.generator.GetStats(),
	}
}

func (fs *FireSensor) SetPosition(x, y int) error {
	if x < 0 || x >= fs.gridSizeX || y < 0 || y >= fs.gridSizeY {
		return &PositionError{
			X:         x,
			Y:         y,
			GridSizeX: fs.gridSizeX,
			GridSizeY: fs.gridSizeY,
		}
	}

	fs.posMutex.Lock()
	fs.posX = x
	fs.posY = y
	fs.posMutex.Unlock()

	return nil
}

func (fs *FireSensor) GetPosition() (int, int) {
	fs.posMutex.RLock()
	defer fs.posMutex.RUnlock()
	return fs.posX, fs.posY
}

type PositionError struct {
	X, Y                 int
	GridSizeX, GridSizeY int
}

func (e *PositionError) Error() string {
	return fmt.Sprintf("position (%d, %d) is out of grid bounds (0-%d, 0-%d)",
		e.X, e.Y, e.GridSizeX-1, e.GridSizeY-1)
}

func GenerateTimestamp() int64 {
	return time.Now().UnixMilli()
}
