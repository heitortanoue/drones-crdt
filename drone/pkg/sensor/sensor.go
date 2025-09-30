package sensor

import (
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
	readings  []FireReading        // Accumulated list of readings
	generator *FireSensorGenerator // Automatic generator
	sensorID  string               // Unique sensor ID
	mutex     sync.RWMutex         // Concurrency protection
}

// NewFireSensor creates a new fire sensor instance
func NewFireSensor(sensorID string, sampleInterval time.Duration) *FireSensor {
	generator := NewFireSensorGenerator(sensorID, sampleInterval)

	sensor := &FireSensor{
		readings:  make([]FireReading, 0),
		generator: generator,
		sensorID:  sensorID,
	}

	// Set circular reference for the generator
	generator.SetSensor(sensor)

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

	return map[string]interface{}{
		"sensor_id":     fs.sensorID,
		"reading_count": readingCount,
		"generator":     fs.generator.GetStats(),
	}
}

// GenerateTimestamp creates a current timestamp in milliseconds
func GenerateTimestamp() int64 {
	return time.Now().UnixMilli()
}