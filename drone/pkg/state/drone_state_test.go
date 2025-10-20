package state

import (
	"testing"
	"time"

	"github.com/heitortanoue/tcc/pkg/crdt"
)

func TestDroneStateBasic(t *testing.T) {
	// Create drone state
	ds := NewDroneState("drone1")

	// Add a fire
	cell := crdt.Cell{X: 10, Y: 20}
	meta := crdt.FireMeta{
		Timestamp:  time.Now().UnixMilli(),
		Confidence: 0.9,
	}

	ds.AddFire(cell, meta)

	// Verify it was added
	fires := ds.GetActiveFires()
	if len(fires) != 1 {
		t.Errorf("Expected 1 fire, found %d", len(fires))
	}

	if fires[0].Cell != cell {
		t.Errorf("Cell mismatch: expected %+v, found %+v", cell, fires[0].Cell)
	}

	if fires[0].Meta != meta {
		t.Errorf("Meta mismatch: expected %+v, found %+v", meta, fires[0].Meta)
	}

	// Verify metadata
	storedMeta, exists := ds.GetFireMeta(cell)
	if !exists {
		t.Error("Metadata not found")
	}

	if storedMeta.Confidence != meta.Confidence {
		t.Errorf("Confidence mismatch: expected %f, found %f",
			meta.Confidence, storedMeta.Confidence)
	}
}

func TestMergeDelta(t *testing.T) {
	// Create drone state
	ds := NewDroneState("drone1")

	// Create a simulated delta
	delta := crdt.FireDelta{
		Context: crdt.DotContext{
			Clock:    make(crdt.VectorClock),
			DotCloud: make(crdt.DotCloud),
		},
		Entries: []crdt.FireDeltaEntry{
			{
				Dot:  crdt.Dot{NodeID: "drone2", Counter: 1},
				Cell: crdt.Cell{X: 15, Y: 25},
				Meta: crdt.FireMeta{
					Timestamp:  time.Now().UnixMilli(),
					Confidence: 0.8,
				},
			},
		},
	}

	// Apply the delta
	ds.MergeDelta(delta)

	// Verify it was applied
	fires := ds.GetActiveFires()
	if len(fires) != 1 {
		t.Errorf("Expected 1 fire after applying delta, found %d", len(fires))
	}

	expectedCell := crdt.Cell{X: 15, Y: 25}
	if fires[0].Cell != expectedCell {
		t.Errorf("Delta cell mismatch: expected %+v, found %+v",
			expectedCell, fires[0].Cell)
	}
}

func TestGlobalState(t *testing.T) {
	// Initialize global state
	InitGlobalState("test-drone")

	// Add fire via global function
	cell := crdt.Cell{X: 5, Y: 15}
	meta := crdt.FireMeta{
		Timestamp:  time.Now().UnixMilli(),
		Confidence: 0.7,
	}

	AddFire(cell, meta)

	// Verify via global function
	fires := GetActiveFires()
	if len(fires) != 1 {
		t.Errorf("Expected 1 fire in global state, found %d", len(fires))
	}

	// Verify the fire has correct cell and metadata
	if fires[0].Cell != cell {
		t.Errorf("Expected cell %+v, got %+v", cell, fires[0].Cell)
	}
	if fires[0].Meta.Confidence != 0.7 {
		t.Errorf("Expected confidence 0.7, got %f", fires[0].Meta.Confidence)
	}

	// Verify statistics
	stats := GetStats()
	if stats["active_fires"] != 1 {
		t.Errorf("Incorrect statistics: %+v", stats)
	}
}

func TestRemoveFire(t *testing.T) {
	ds := NewDroneState("drone1")

	// Add fire
	cell := crdt.Cell{X: 30, Y: 40}
	meta := crdt.FireMeta{
		Timestamp:  time.Now().UnixMilli(),
		Confidence: 0.95,
	}

	ds.AddFire(cell, meta)

	// Verify it was added
	fires := ds.GetActiveFires()
	if len(fires) != 1 {
		t.Errorf("Expected 1 fire before removal, found %d", len(fires))
	}

	// Remove the fire
	ds.RemoveFire(cell)

	// Verify it was removed
	fires = ds.GetActiveFires()
	if len(fires) != 0 {
		t.Errorf("Expected 0 fires after removal, found %d", len(fires))
	}
}

// -------------------------------------------------------------------------
// Delta Operations Tests - CRITICAL
// -------------------------------------------------------------------------

func TestGenerateDelta(t *testing.T) {
	ds := NewDroneState("drone1")

	// Initially no delta
	delta := ds.GenerateDelta()
	if delta != nil {
		t.Error("Expected nil delta from empty state")
	}

	// Add a fire
	cell := crdt.Cell{X: 10, Y: 20}
	meta := crdt.FireMeta{
		Timestamp:  time.Now().UnixMilli(),
		Confidence: 0.85,
	}
	ds.AddFire(cell, meta)

	// Generate delta
	delta = ds.GenerateDelta()
	if delta == nil {
		t.Fatal("Expected non-nil delta after adding fire")
	}

	// Verify delta structure
	if len(delta.Entries) != 1 {
		t.Errorf("Expected 1 entry in delta, got %d", len(delta.Entries))
	}

	if delta.Entries[0].Cell != cell {
		t.Errorf("Cell mismatch in delta: expected %+v, got %+v",
			cell, delta.Entries[0].Cell)
	}

	if delta.Entries[0].Meta.Confidence != meta.Confidence {
		t.Errorf("Metadata mismatch in delta")
	}

	// Verify Context is present
	if delta.Context.Clock == nil {
		t.Error("Expected Context.Clock in delta")
	}
}

func TestClearDelta(t *testing.T) {
	ds := NewDroneState("drone1")

	// Add fire to create delta
	cell := crdt.Cell{X: 5, Y: 15}
	meta := crdt.FireMeta{
		Timestamp:  time.Now().UnixMilli(),
		Confidence: 0.9,
	}
	ds.AddFire(cell, meta)

	// Verify delta exists
	if ds.GenerateDelta() == nil {
		t.Fatal("Expected delta after adding fire")
	}

	// Clear delta
	ds.ClearDelta()

	// Verify delta is cleared
	delta := ds.GenerateDelta()
	if delta != nil {
		t.Error("Expected nil delta after ClearDelta()")
	}

	// Verify fire is still in state (only delta cleared, not core)
	fires := ds.GetActiveFires()
	if len(fires) != 1 {
		t.Errorf("Expected fire to remain after ClearDelta, got %d fires", len(fires))
	}
}

func TestGetFullState(t *testing.T) {
	ds := NewDroneState("drone1")

	// Empty state
	fullState := ds.GetFullState()
	if fullState != nil {
		t.Error("Expected nil full state from empty drone state")
	}

	// Add multiple fires
	cells := []crdt.Cell{
		{X: 10, Y: 20},
		{X: 30, Y: 40},
		{X: 50, Y: 60},
	}

	for i, cell := range cells {
		meta := crdt.FireMeta{
			Timestamp:  time.Now().UnixMilli() + int64(i),
			Confidence: 0.8 + float64(i)*0.05,
		}
		ds.AddFire(cell, meta)
	}

	// Get full state
	fullState = ds.GetFullState()
	if fullState == nil {
		t.Fatal("Expected non-nil full state")
	}

	// Verify all entries are present
	if len(fullState.Entries) != len(cells) {
		t.Errorf("Expected %d entries in full state, got %d",
			len(cells), len(fullState.Entries))
	}

	// Verify Context is the Core's context
	if fullState.Context.Clock == nil {
		t.Error("Expected Context in full state")
	}
}

// -------------------------------------------------------------------------
// Multiple Fires and Metadata Tests
// -------------------------------------------------------------------------

func TestMultipleFires(t *testing.T) {
	ds := NewDroneState("drone1")

	// Add multiple fires
	fires := []crdt.Cell{
		{X: 10, Y: 10},
		{X: 20, Y: 20},
		{X: 30, Y: 30},
	}

	for _, cell := range fires {
		meta := crdt.FireMeta{
			Timestamp:  time.Now().UnixMilli(),
			Confidence: 0.9,
		}
		ds.AddFire(cell, meta)
	}

	// Verify all are present
	activeFires := ds.GetActiveFires()
	if len(activeFires) != len(fires) {
		t.Errorf("Expected %d active fires, got %d", len(fires), len(activeFires))
	}

	// Remove one
	ds.RemoveFire(fires[1])

	// Verify only 2 remain
	activeFires = ds.GetActiveFires()
	if len(activeFires) != 2 {
		t.Errorf("Expected 2 active fires after removal, got %d", len(activeFires))
	}
}

func TestSameCellMultipleTimes(t *testing.T) {
	ds := NewDroneState("drone1")

	cell := crdt.Cell{X: 10, Y: 20}

	// Add same cell multiple times
	for i := 0; i < 3; i++ {
		meta := crdt.FireMeta{
			Timestamp:  time.Now().UnixMilli() + int64(i*1000),
			Confidence: 0.7 + float64(i)*0.1,
		}
		ds.AddFire(cell, meta)
	}

	// Should only have 1 fire (optimization removes old occurrences)
	fires := ds.GetActiveFires()
	if len(fires) != 1 {
		t.Errorf("Expected 1 fire after multiple adds of same cell, got %d", len(fires))
	}

	// Verify it has the latest metadata
	meta, exists := ds.GetFireMeta(cell)
	if !exists {
		t.Fatal("Expected metadata for cell")
	}

	// Should have the last confidence value (third iteration: 0.7 + 2*0.1 = 0.9)
	expectedConfidence := 0.9
	// Use small epsilon for float comparison
	epsilon := 0.0001
	if meta.Confidence < expectedConfidence-epsilon || meta.Confidence > expectedConfidence+epsilon {
		t.Errorf("Expected latest metadata (confidence ≈ %.2f), got %.6f",
			expectedConfidence, meta.Confidence)
	}
}

func TestGetLatestReadings(t *testing.T) {
	ds := NewDroneState("drone1")

	// Add fire from this drone
	cell1 := crdt.Cell{X: 10, Y: 20}
	meta1 := crdt.FireMeta{
		Timestamp:  1000,
		Confidence: 0.8,
	}
	ds.AddFire(cell1, meta1)

	// Simulate receiving delta from another drone
	cell2 := crdt.Cell{X: 30, Y: 40}
	delta := crdt.FireDelta{
		Context: crdt.DotContext{
			Clock:    crdt.VectorClock{"drone2": 1},
			DotCloud: make(crdt.DotCloud),
		},
		Entries: []crdt.FireDeltaEntry{
			{
				Dot:  crdt.Dot{NodeID: "drone2", Counter: 1},
				Cell: cell2,
				Meta: crdt.FireMeta{
					Timestamp:  2000,
					Confidence: 0.9,
				},
			},
		},
	}
	ds.MergeDelta(delta)

	// Get latest readings
	readings := ds.GetLatestReadings()

	// Should have readings from both drones
	if len(readings) != 2 {
		t.Errorf("Expected readings from 2 drones, got %d", len(readings))
	}

	// Verify drone1's reading
	if reading, ok := readings["drone1"]; !ok {
		t.Error("Expected reading from drone1")
	} else if reading.Timestamp != 1000 {
		t.Errorf("Expected timestamp 1000 from drone1, got %d", reading.Timestamp)
	}

	// Verify drone2's reading
	if reading, ok := readings["drone2"]; !ok {
		t.Error("Expected reading from drone2")
	} else if reading.Timestamp != 2000 {
		t.Errorf("Expected timestamp 2000 from drone2, got %d", reading.Timestamp)
	}
}

// -------------------------------------------------------------------------
// Convergence and Integration Tests
// -------------------------------------------------------------------------

func TestThreeDroneConvergence(t *testing.T) {
	// Create 3 drone states
	d1 := NewDroneState("drone1")
	d2 := NewDroneState("drone2")
	d3 := NewDroneState("drone3")

	// Each detects different fire
	cell1 := crdt.Cell{X: 10, Y: 10}
	cell2 := crdt.Cell{X: 20, Y: 20}
	cell3 := crdt.Cell{X: 30, Y: 30}

	d1.AddFire(cell1, crdt.FireMeta{Timestamp: 1000, Confidence: 0.8})
	d2.AddFire(cell2, crdt.FireMeta{Timestamp: 2000, Confidence: 0.85})
	d3.AddFire(cell3, crdt.FireMeta{Timestamp: 3000, Confidence: 0.9})

	// Simulate gossip: d1 sends to d2, d2 sends to d3, d3 sends to d1
	delta1 := d1.GenerateDelta()
	delta2 := d2.GenerateDelta()
	delta3 := d3.GenerateDelta()

	if delta1 != nil {
		d2.MergeDelta(*delta1)
	}
	if delta2 != nil {
		d3.MergeDelta(*delta2)
	}
	if delta3 != nil {
		d1.MergeDelta(*delta3)
	}

	// Second round of gossip for full convergence
	delta1 = d1.GenerateDelta()
	delta2 = d2.GenerateDelta()
	delta3 = d3.GenerateDelta()

	if delta1 != nil {
		d2.MergeDelta(*delta1)
		d3.MergeDelta(*delta1)
	}
	if delta2 != nil {
		d1.MergeDelta(*delta2)
		d3.MergeDelta(*delta2)
	}
	if delta3 != nil {
		d1.MergeDelta(*delta3)
		d2.MergeDelta(*delta3)
	}

	// All drones should have all 3 fires
	fires1 := d1.GetActiveFires()
	fires2 := d2.GetActiveFires()
	fires3 := d3.GetActiveFires()

	if len(fires1) != 3 {
		t.Errorf("Drone1 expected 3 fires, got %d", len(fires1))
	}
	if len(fires2) != 3 {
		t.Errorf("Drone2 expected 3 fires, got %d", len(fires2))
	}
	if len(fires3) != 3 {
		t.Errorf("Drone3 expected 3 fires, got %d", len(fires3))
	}
}

func TestAntiEntropyWithFullState(t *testing.T) {
	// Drone1 has complete state
	d1 := NewDroneState("drone1")
	cells := []crdt.Cell{
		{X: 10, Y: 10},
		{X: 20, Y: 20},
		{X: 30, Y: 30},
	}
	for _, cell := range cells {
		d1.AddFire(cell, crdt.FireMeta{Timestamp: time.Now().UnixMilli(), Confidence: 0.9})
	}
	d1.ClearDelta() // Clear delta to simulate it was already disseminated

	// Drone2 is a new drone with no state
	d2 := NewDroneState("drone2")

	// Anti-entropy: d1 sends full state to d2
	fullState := d1.GetFullState()
	if fullState == nil {
		t.Fatal("Expected full state from d1")
	}

	d2.MergeDelta(*fullState)

	// d2 should now have all fires
	fires2 := d2.GetActiveFires()
	if len(fires2) != len(cells) {
		t.Errorf("Expected %d fires in d2 after anti-entropy, got %d",
			len(cells), len(fires2))
	}
}

func TestReAddAfterRemoveWithMetadata(t *testing.T) {
	ds := NewDroneState("drone1")

	cell := crdt.Cell{X: 10, Y: 20}
	meta1 := crdt.FireMeta{Timestamp: 1000, Confidence: 0.7}

	// Add fire
	ds.AddFire(cell, meta1)

	// Remove fire
	ds.RemoveFire(cell)

	// Verify removed
	fires := ds.GetActiveFires()
	if len(fires) != 0 {
		t.Error("Expected no fires after removal")
	}

	// Re-add with different metadata
	meta2 := crdt.FireMeta{Timestamp: 2000, Confidence: 0.95}
	ds.AddFire(cell, meta2)

	// Verify present with new metadata
	fires = ds.GetActiveFires()
	if len(fires) != 1 {
		t.Errorf("Expected 1 fire after re-add, got %d", len(fires))
	}

	storedMeta, exists := ds.GetFireMeta(cell)
	if !exists {
		t.Fatal("Expected metadata after re-add")
	}

	if storedMeta.Timestamp != 2000 {
		t.Errorf("Expected new timestamp 2000, got %d", storedMeta.Timestamp)
	}
}
