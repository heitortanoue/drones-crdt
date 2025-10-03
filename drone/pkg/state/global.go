package state

import (
	"sync"

	"github.com/heitortanoue/tcc/pkg/crdt"
)

var (
	// Global instance of the drone state
	globalState *DroneState
	once        sync.Once
)

// InitGlobalState initializes the global state of the drone.
// Must be called once during application startup.
func InitGlobalState(droneID string) {
	once.Do(func() {
		globalState = NewDroneState(droneID)
	})
}

// GetState returns the global state instance.
// Returns nil if it has not been initialized.
func GetState() *DroneState {
	return globalState
}

// MergeDelta applies a delta to the global state
func MergeDelta(delta crdt.FireDelta) {
	if globalState != nil {
		globalState.MergeDelta(delta)
	}
}

// Convenience functions for accessing the global state

// AddFire adds a fire detection to the global state
func AddFire(cell crdt.Cell, meta crdt.FireMeta) {
	if globalState != nil {
		globalState.AddFire(cell, meta)
	}
}

// RemoveFire removes a fire detection from the global state
func RemoveFire(cell crdt.Cell) {
	if globalState != nil {
		globalState.RemoveFire(cell)
	}
}

// GetActiveFires returns the currently active fire cells from the global state
func GetActiveFires() []crdt.Cell {
	if globalState != nil {
		return globalState.GetActiveFires()
	}
	return nil
}

// GetLatestReadings returns the latest fire readings from the global state
func GetLatestReadings() map[string]crdt.FireMeta {
	if globalState != nil {
		return globalState.GetLatestReadings()
	}
	return nil
}

// GenerateDelta generates a delta of the global state
func GenerateDelta() *crdt.FireDelta {
	if globalState != nil {
		return globalState.GenerateDelta()
	}
	return nil
}

// ClearDelta clears the delta of the global state
func ClearDelta() {
	if globalState != nil {
		globalState.ClearDelta()
	}
}

// GetStats returns statistics of the global state
func GetStats() map[string]interface{} {
	if globalState != nil {
		return globalState.GetStats()
	}
	return map[string]interface{}{
		"error": "global state not initialized",
	}
}

// GetFullState returns the complete state (for anti-entropy)
func GetFullState() *crdt.FireDelta {
	if globalState != nil {
		return globalState.GetFullState()
	}
	return nil
}
