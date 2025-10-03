package gossip

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/heitortanoue/tcc/pkg/crdt"
	"github.com/heitortanoue/tcc/pkg/state"
)

// DeltaMsg represents a delta message with TTL for dissemination
type DeltaMsg struct {
	ID        uuid.UUID      `json:"id"`
	TTL       int            `json:"ttl"`
	Data      crdt.FireDelta `json:"data"`
	SenderID  string         `json:"sender_id"`
	Timestamp int64          `json:"timestamp"`
}

// DisseminationSystem manages TTL-based dissemination (Requirement F4)
type DisseminationSystem struct {
	droneID    string
	fanout     int // Fan-out (number of neighbors)
	defaultTTL int // Default initial TTL

	// Communication interfaces
	neighborGetter NeighborGetter
	tcpSender      TCPSender
	cache          *DeduplicationCache

	// Neighbor tracking for prioritization
	neighborTracker *NeighborTracker

	// Execution control
	running bool
	stopCh  chan struct{}
	mutex   sync.RWMutex

	// Metrics
	sentCount        int64
	receivedCount    int64
	droppedCount     int64 // Due to TTL=0 or duplicates
	antiEntropyCount int64
}

// NeighborTracker tracks last sent time to each neighbor
type NeighborTracker struct {
	lastSent map[string]time.Time
	mutex    sync.RWMutex
}

// NeighborGetter interface to obtain neighbors
type NeighborGetter interface {
	GetNeighborURLs() []string
	Count() int
}

// TCPSender interface for TCP sending
type TCPSender interface {
	SendDelta(url string, delta DeltaMsg) error
}

// NewNeighborTracker creates a new neighbor tracker
func NewNeighborTracker() *NeighborTracker {
	return &NeighborTracker{
		lastSent: make(map[string]time.Time),
	}
}

// RecordSent records that a message was sent to a neighbor
func (nt *NeighborTracker) RecordSent(neighbor string) {
	nt.mutex.Lock()
	defer nt.mutex.Unlock()
	nt.lastSent[neighbor] = time.Now()
}

// PrioritizeNeighbors returns neighbors sorted by least recently sent
func (nt *NeighborTracker) PrioritizeNeighbors(neighbors []string, count int) []string {
	nt.mutex.RLock()
	defer nt.mutex.RUnlock()

	type neighborInfo struct {
		url      string
		lastSent time.Time
	}

	// Build list with last sent times
	nlist := make([]neighborInfo, 0, len(neighbors))
	for _, n := range neighbors {
		nlist = append(nlist, neighborInfo{
			url:      n,
			lastSent: nt.lastSent[n],
		})
	}

	// Sort by oldest first (never sent = zero time = highest priority)
	for i := 0; i < len(nlist)-1; i++ {
		for j := i + 1; j < len(nlist); j++ {
			if nlist[i].lastSent.After(nlist[j].lastSent) {
				nlist[i], nlist[j] = nlist[j], nlist[i]
			}
		}
	}

	// Return up to 'count' neighbors
	result := make([]string, 0, count)
	for i := 0; i < count && i < len(nlist); i++ {
		result = append(result, nlist[i].url)
	}
	return result
}

// NewDisseminationSystem creates a new dissemination system
func NewDisseminationSystem(droneID string, fanout, defaultTTL int, neighborGetter NeighborGetter, tcpSender TCPSender) *DisseminationSystem {
	return &DisseminationSystem{
		droneID:         droneID,
		fanout:          fanout,
		defaultTTL:      defaultTTL,
		neighborGetter:  neighborGetter,
		tcpSender:       tcpSender,
		cache:           NewDeduplicationCache(10000), // Cache of 10k IDs
		neighborTracker: NewNeighborTracker(),
		running:         false,
		stopCh:          make(chan struct{}),
	}
}

// Start begins the dissemination system
func (ds *DisseminationSystem) Start() {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	if ds.running {
		return
	}

	ds.running = true

	// Starts periodic heartbeat for delta push
	log.Printf("[DISSEMINATION] Starting heartbeat for delta dissemination")
	go ds.startHeartbeat()
	go ds.startAntiEntropyLoop()

	log.Printf("[DISSEMINATION] Dissemination system started for %s (fanout: %d, TTL: %d)",
		ds.droneID, ds.fanout, ds.defaultTTL)
}

// Stop halts the dissemination system
func (ds *DisseminationSystem) Stop() {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	if !ds.running {
		return
	}

	ds.running = false
	close(ds.stopCh)
	log.Printf("[DISSEMINATION] Stopping dissemination system for %s", ds.droneID)
}

// DisseminateDelta disseminates a delta to neighbors with TTL
func (ds *DisseminationSystem) DisseminateDelta(delta crdt.FireDelta) error {
	ds.mutex.RLock()
	if !ds.running {
		ds.mutex.RUnlock()
		return nil
	}
	ds.mutex.RUnlock()

	// Create message with initial TTL
	msg := DeltaMsg{
		ID:        uuid.New(),
		TTL:       ds.defaultTTL,
		Data:      delta,
		SenderID:  ds.droneID,
		Timestamp: time.Now().UnixMilli(),
	}

	return ds.forwardDelta(msg)
}

// ProcessReceivedDelta processes a delta received from another node
func (ds *DisseminationSystem) ProcessReceivedDelta(msg DeltaMsg) error {
	ds.mutex.Lock()
	ds.receivedCount++
	ds.mutex.Unlock()

	// Deduplication check
	if ds.cache.Contains(msg.ID) {
		ds.mutex.Lock()
		ds.droppedCount++
		ds.mutex.Unlock()
		log.Printf("[DISSEMINATION] Delta %s discarded (duplicate)", msg.ID.String()[:8])
		return nil
	}

	// Add to cache
	ds.cache.Add(msg.ID)

	// TTL check
	if msg.TTL <= 0 {
		ds.mutex.Lock()
		ds.droppedCount++
		ds.mutex.Unlock()
		log.Printf("[DISSEMINATION] Delta %s discarded (TTL=0)", msg.ID.String()[:8])
		return nil
	}

	log.Printf("[DISSEMINATION] Processing delta %s (TTL: %d)", msg.ID.String()[:8], msg.TTL)

	// Apply received delta to local state
	state.MergeDelta(msg.Data)

	// Decrement TTL and continue dissemination
	msg.TTL--
	msg.SenderID = ds.droneID // Update sender to this node

	return ds.forwardDelta(msg)
}

// forwardDelta sends delta to up to 'fanout' neighbors (with prioritization)
func (ds *DisseminationSystem) forwardDelta(msg DeltaMsg) error {
	neighbors := ds.neighborGetter.GetNeighborURLs()
	if len(neighbors) == 0 {
		log.Printf("[DISSEMINATION] No neighbors available for delta %s", msg.ID.String()[:8])
		return nil
	}

	// Limit to configured fanout
	targetCount := ds.fanout
	if len(neighbors) < targetCount {
		targetCount = len(neighbors)
	}

	// Prioritize neighbors that haven't received messages recently
	targets := ds.neighborTracker.PrioritizeNeighbors(neighbors, targetCount)

	var errors []error
	successCount := 0

	for _, url := range targets {
		if err := ds.tcpSender.SendDelta(url, msg); err != nil {
			log.Printf("[DISSEMINATION] Error sending delta %s to %s: %v",
				msg.ID.String()[:8], url, err)
			errors = append(errors, err)
		} else {
			successCount++
			// Record successful send
			ds.neighborTracker.RecordSent(url)
		}
	}

	ds.mutex.Lock()
	ds.sentCount += int64(successCount)
	ds.mutex.Unlock()

	log.Printf("[DISSEMINATION] Delta %s sent to %d/%d neighbors (prioritized)",
		msg.ID.String()[:8], successCount, len(targets))

	if len(errors) > 0 {
		return errors[0] // Return the first error
	}

	return nil
}

// selectRandomNeighbors selects up to 'count' neighbors randomly
func selectRandomNeighbors(neighbors []string, count int) []string {
	if len(neighbors) <= count {
		return neighbors
	}

	// Copy to avoid modifying original
	shuffled := make([]string, len(neighbors))
	copy(shuffled, neighbors)

	// Fisher-Yates shuffle
	for i := len(shuffled) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled[:count]
}

// startHeartbeat periodically triggers sending of local delta
func (ds *DisseminationSystem) startHeartbeat() {
	ticker := time.NewTicker(5 * time.Second) // heartbeat interval
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Extract local delta from drone state
			delta := state.GenerateDelta()
			// Only send if there are pending changes
			if delta != nil && len(delta.Entries) > 0 {
				log.Printf("[DISSEMINATION] Generating delta with %d entries", len(delta.Entries))
				err := ds.DisseminateDelta(*delta)
				if err != nil {
					log.Printf("[DISSEMINATION] Error disseminating delta: %v", err)
				} else {
					// Clear delta after successful dissemination
					state.ClearDelta()
					log.Printf("[DISSEMINATION] Delta disseminated with %d entries", len(delta.Entries))
				}
			}
		case <-ds.stopCh:
			return
		}
	}
}

// startAntiEntropyLoop periodically sends full state to a random neighbor
func (ds *DisseminationSystem) startAntiEntropyLoop() {
	ticker := time.NewTicker(60 * time.Second) // Anti-entropy every 60 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			neighbors := ds.neighborGetter.GetNeighborURLs()
			if len(neighbors) == 0 {
				continue
			}

			// Get full state
			fullState := state.GetFullState()
			if fullState == nil || len(fullState.Entries) == 0 {
				log.Printf("[ANTI-ENTROPY] No state to sync")
				continue
			}

			// Select one random neighbor for full state sync
			target := neighbors[rand.Intn(len(neighbors))]

			// Create anti-entropy message (max TTL to ensure delivery)
			msg := DeltaMsg{
				ID:        uuid.New(),
				TTL:       ds.defaultTTL * 2, // Higher TTL for anti-entropy
				Data:      *fullState,
				SenderID:  ds.droneID,
				Timestamp: time.Now().UnixMilli(),
			}

			log.Printf("[ANTI-ENTROPY] Sending full state (%d entries) to %s",
				len(fullState.Entries), target)

			if err := ds.tcpSender.SendDelta(target, msg); err != nil {
				log.Printf("[ANTI-ENTROPY] Error sending to %s: %v", target, err)
			} else {
				ds.mutex.Lock()
				ds.antiEntropyCount++
				ds.mutex.Unlock()
				ds.neighborTracker.RecordSent(target)
				log.Printf("[ANTI-ENTROPY] Full state synced to %s", target)
			}

		case <-ds.stopCh:
			return
		}
	}
}

// GetStats returns dissemination system statistics
func (ds *DisseminationSystem) GetStats() map[string]interface{} {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	return map[string]interface{}{
		"running":            ds.running,
		"fanout":             ds.fanout,
		"default_ttl":        ds.defaultTTL,
		"sent_count":         ds.sentCount,
		"received_count":     ds.receivedCount,
		"dropped_count":      ds.droppedCount,
		"anti_entropy_count": ds.antiEntropyCount,
		"cache_size":         ds.cache.Size(),
		"neighbor_count":     ds.neighborGetter.Count(),
	}
}

// IsRunning returns whether the system is running
func (ds *DisseminationSystem) IsRunning() bool {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	return ds.running
}
