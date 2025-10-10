package sensor

import (
	"testing"
	"time"
)

func TestFireSensorGenerator_BasicFunctionality(t *testing.T) {
	gridSize := 1000
	sensor := NewFireSensor("test-sensor", 100*time.Millisecond, gridSize, gridSize)

	if sensor.generator.running {
		t.Error("Generator should not be running initially")
	}

	stats := sensor.GetStats()
	if stats["sensor_id"] != "test-sensor" {
		t.Error("Incorrect SensorID in stats")
	}

	// Start the generator
	sensor.Start()

	if !sensor.generator.running {
		t.Error("Generator should be running after Start()")
	}

	// Wait for a few generations
	time.Sleep(350 * time.Millisecond)

	// Stop the generator
	sensor.Stop()

	if sensor.generator.running {
		t.Error("Generator should not be running after Stop()")
	}

	readings := sensor.GetReadings()
	if len(readings) < 2 {
		t.Errorf("Expected at least 2 readings, got %d", len(readings))
	}

	for _, reading := range readings {
		if reading.X < 0 || reading.X >= gridSize {
			t.Errorf("X coordinate out of grid bounds: %d (expected 0-%d)", reading.X, gridSize-1)
		}
		if reading.Y < 0 || reading.Y >= gridSize {
			t.Errorf("Y coordinate out of grid bounds: %d (expected 0-%d)", reading.Y, gridSize-1)
		}
		if reading.Confidence < 0 || reading.Confidence > 100 {
			t.Errorf("Confidence out of valid range: %f", reading.Confidence)
		}
		if reading.SensorID != "test-sensor" {
			t.Errorf("Incorrect SensorID in reading: %s", reading.SensorID)
		}
	}
}

func TestFireSensor_ManualReadings(t *testing.T) {
	gridSize := 1000
	sensor := NewFireSensor("manual-test-sensor", time.Hour, gridSize, gridSize)

	sensor.AddManualReading(15, 25, 85.5)

	readings := sensor.GetReadings()
	if len(readings) != 1 {
		t.Errorf("Expected 1 manual reading, got %d", len(readings))
	}

	reading := readings[0]
	if reading.X != 15 || reading.Y != 25 {
		t.Error("Incorrect coordinates in manual reading")
	}
	if reading.Confidence != 85.5 {
		t.Error("Incorrect confidence value in manual reading")
	}
	if reading.SensorID != "manual-test-sensor" {
		t.Error("Incorrect SensorID in manual reading")
	}
}

func TestFireSensor_GetAndClearReadings(t *testing.T) {
	gridSize := 1000
	sensor := NewFireSensor("clear-test-sensor", time.Hour, gridSize, gridSize)

	sensor.AddManualReading(10, 20, 75.0)
	sensor.AddManualReading(15, 25, 80.0)
	sensor.AddManualReading(20, 30, 90.0)

	// Verify we have 3 readings
	readings := sensor.GetReadings()
	if len(readings) != 3 {
		t.Errorf("Expected 3 readings, got %d", len(readings))
	}

	// Get and clear the readings (simulate sending to drone)
	clearedReadings := sensor.GetAndClearReadings()
	if len(clearedReadings) != 3 {
		t.Errorf("Expected 3 readings returned, got %d", len(clearedReadings))
	}

	// Verify the list was cleared
	remainingReadings := sensor.GetReadings()
	if len(remainingReadings) != 0 {
		t.Errorf("Expected 0 readings after clearing, got %d", len(remainingReadings))
	}

	// Add a new reading after clearing
	sensor.AddManualReading(5, 10, 60.0)
	newReadings := sensor.GetReadings()
	if len(newReadings) != 1 {
		t.Errorf("Expected 1 new reading, got %d", len(newReadings))
	}
}
