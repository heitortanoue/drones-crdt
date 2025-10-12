package state

import (
	"log"
	"sync"

	"github.com/heitortanoue/tcc/pkg/crdt"
)

// DroneState maintains the current state of the drone including fire detections
type DroneState struct {
	droneID string

	// CRDT for cells where fire has been detected
	fires *crdt.AWORSet[crdt.Cell]

	// Metadata for cells (mapping Dot -> FireMeta)
	metadata map[crdt.Dot]crdt.FireMeta

	// Concurrency control
	mutex sync.RWMutex
}

// NewDroneState creates a new instance of the drone state
func NewDroneState(droneID string) *DroneState {
	return &DroneState{
		droneID:  droneID,
		fires:    crdt.NewAWORSet[crdt.Cell](),
		metadata: make(map[crdt.Dot]crdt.FireMeta),
	}
}

// AddFire adds a new fire detection to the local state
func (ds *DroneState) AddFire(cell crdt.Cell, meta crdt.FireMeta) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	// Use AWORSet.Add which follows the reference implementation:
	// - First removes old occurrences of the cell
	// - Then adds with a new dot
	// - Updates both Core and Delta contexts
	ds.fires.Add(ds.droneID, cell)

	// Find the newly added dot to store metadata
	var newDot crdt.Dot
	for dot, c := range ds.fires.Core.Entries {
		if c == cell && dot.NodeID == ds.droneID {
			// This is the most recent dot for this cell from this drone
			if dot.Counter > newDot.Counter {
				newDot = dot
			}
		}
	}

	// Store metadata for the new dot
	ds.metadata[newDot] = meta

	log.Printf("[STATE] Fire detection added at (%d, %d) with dot %s",
		cell.X, cell.Y, newDot.String()[:8])
}

// RemoveFire removes a cell from the state (when fire is extinguished)
func (ds *DroneState) RemoveFire(cell crdt.Cell) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	// Collect dots to remove metadata before removing from CRDT
	var dotsToRemove []crdt.Dot
	for dot, storedCell := range ds.fires.Core.Entries {
		if storedCell == cell {
			dotsToRemove = append(dotsToRemove, dot)
		}
	}

	// Remove from CRDT (marks in both Core and Delta contexts)
	ds.fires.Remove(cell)

	// Remove metadata for the removed dots
	for _, dot := range dotsToRemove {
		delete(ds.metadata, dot)
	}

	log.Printf("[STATE] Fire detection removed at (%d, %d)", cell.X, cell.Y)
}

// MergeDelta applies a delta received from another drone
func (ds *DroneState) MergeDelta(delta crdt.FireDelta) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	// 1) Create a deep copy of the context to avoid pointer aliasing issues
	contextCopy := crdt.DotContext{
		Clock:    make(crdt.VectorClock, len(delta.Context.Clock)),
		DotCloud: make(crdt.DotCloud, len(delta.Context.DotCloud)),
	}
	for k, v := range delta.Context.Clock {
		contextCopy.Clock[k] = v
	}
	for k, v := range delta.Context.DotCloud {
		contextCopy.DotCloud[k] = v
	}

	// 2) Rebuild a temporary kernel from the delta
	kernel := &crdt.DotKernel[crdt.Cell]{
		Context: &contextCopy,
		Entries: make(map[crdt.Dot]crdt.Cell, len(delta.Entries)),
	}

	// 3) Fill the Dot→Cell map and store metadata
	for _, entry := range delta.Entries {
		kernel.Entries[entry.Dot] = entry.Cell
		ds.metadata[entry.Dot] = entry.Meta
	}

	// 4) Apply merge of the CRDT state
	ds.fires.MergeDelta(kernel)

	log.Printf("[STATE] Delta applied with %d entries from context clock=%v",
		len(delta.Entries), delta.Context.Clock)
}

// GenerateDelta generates a delta of local changes for dissemination
func (ds *DroneState) GenerateDelta() *crdt.FireDelta {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	if ds.fires.Delta == nil || len(ds.fires.Delta.Entries) == 0 {
		log.Printf("[STATE] No delta to send (no local changes)")
		return nil // No local changes
	}

	// Build the delta for dissemination
	delta := &crdt.FireDelta{
		Context: *ds.fires.Delta.Context,
		Entries: make([]crdt.FireDeltaEntry, 0, len(ds.fires.Delta.Entries)),
	}

	for dot, cell := range ds.fires.Delta.Entries {
		meta, exists := ds.metadata[dot]
		if !exists {
			// Default metadata if not found
			meta = crdt.FireMeta{
				Timestamp:  0,
				Confidence: 1.0,
			}
		}

		delta.Entries = append(delta.Entries, crdt.FireDeltaEntry{
			Dot:  dot,
			Cell: cell,
			Meta: meta,
		})
	}

	return delta
}

// ClearDelta clears the delta after dissemination
func (ds *DroneState) ClearDelta() {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	ds.fires.Delta = nil
}

// GetActiveFires returns all active cells with fire
func (ds *DroneState) GetActiveFires() []crdt.Cell {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	return ds.fires.Elements()
}

// GetLatestReadings returns the most recent fire metadata grouped by NodeID
func (ds *DroneState) GetLatestReadings() map[string]crdt.FireMeta {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	latestReadings := make(map[string]crdt.FireMeta)

	// Iterate over metadata and select the most recent one per node
	for dot, meta := range ds.metadata {
		if existingMeta, exists := latestReadings[dot.NodeID]; !exists || meta.Timestamp > existingMeta.Timestamp {
			latestReadings[dot.NodeID] = meta
		}
	}

	return latestReadings
}

// GetFireMeta returns metadata for a specific cell
func (ds *DroneState) GetFireMeta(cell crdt.Cell) (crdt.FireMeta, bool) {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	// Search the cell in the active entries
	for dot, storedCell := range ds.fires.Core.Entries {
		if storedCell == cell {
			if meta, exists := ds.metadata[dot]; exists {
				return meta, true
			}
		}
	}

	return crdt.FireMeta{}, false
}

// GetStats returns statistics about the state
func (ds *DroneState) GetStats() map[string]interface{} {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	return map[string]interface{}{
		"drone_id":          ds.droneID,
		"active_fires":      len(ds.fires.Core.Entries),
		"metadata_count":    len(ds.metadata),
		"has_pending_delta": ds.fires.Delta != nil,
	}
}

// GetDroneID returns the drone ID
func (ds *DroneState) GetDroneID() string {
	return ds.droneID
}

// GetFullState returns the complete state as a FireDelta (for anti-entropy)
func (ds *DroneState) GetFullState() *crdt.FireDelta {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	if len(ds.fires.Core.Entries) == 0 {
		return nil
	}

	delta := &crdt.FireDelta{
		Context: *ds.fires.Core.Context,
		Entries: make([]crdt.FireDeltaEntry, 0, len(ds.fires.Core.Entries)),
	}

	for dot, cell := range ds.fires.Core.Entries {
		meta, exists := ds.metadata[dot]
		if !exists {
			meta = crdt.FireMeta{
				Timestamp:  0,
				Confidence: 1.0,
			}
		}

		delta.Entries = append(delta.Entries, crdt.FireDeltaEntry{
			Dot:  dot,
			Cell: cell,
			Meta: meta,
		})
	}

	return delta
}
