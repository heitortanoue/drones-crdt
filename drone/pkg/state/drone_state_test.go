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

	if fires[0] != cell {
		t.Errorf("Cell mismatch: expected %+v, found %+v", cell, fires[0])
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
	if fires[0] != expectedCell {
		t.Errorf("Delta cell mismatch: expected %+v, found %+v",
			expectedCell, fires[0])
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