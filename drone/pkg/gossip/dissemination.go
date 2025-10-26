package gossip

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/heitortanoue/tcc/pkg/crdt"
	"github.com/heitortanoue/tcc/pkg/network"
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

	// Configurable intervals
	deltaPushInterval   time.Duration
	antiEntropyInterval time.Duration

	// Communication interfaces
	neighborGetter NeighborGetter
	tcpSender      TCPSender
	cache          *DeduplicationCache

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

// NeighborGetter interface to obtain neighbors
type NeighborGetter interface {
	GetNeighborURLs() []string
	GetPrioritizedNeighborURLs(count int) []*network.Neighbor
	RecordSent(neighborID string)
	Count() int
}

// TCPSender interface for TCP sending
type TCPSender interface {
	SendDelta(msgType string, url string, delta DeltaMsg) error
}

// NewDisseminationSystem creates a new dissemination system
func NewDisseminationSystem(droneID string, fanout, defaultTTL int, deltaPushInterval, antiEntropyInterval time.Duration, neighborGetter NeighborGetter, tcpSender TCPSender) *DisseminationSystem {
	return &DisseminationSystem{
		droneID:             droneID,
		fanout:              fanout,
		defaultTTL:          defaultTTL,
		deltaPushInterval:   deltaPushInterval,
		antiEntropyInterval: antiEntropyInterval,
		neighborGetter:      neighborGetter,
		tcpSender:           tcpSender,
		cache:               NewDeduplicationCache(10000), // Cache of 10k IDs
		running:             false,
		stopCh:              make(chan struct{}),
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

	// Starts periodic heartbeat for delta push (only if enabled)
	if ds.deltaPushInterval > 0 {
		log.Printf("[DISSEMINATION] Starting heartbeat for delta dissemination (interval: %v)", ds.deltaPushInterval)
		go ds.startHeartbeat()
	} else {
		log.Printf("[DISSEMINATION] Delta push is disabled")
	}

	// Start anti-entropy loop (only if enabled)
	if ds.antiEntropyInterval > 0 {
		log.Printf("[DISSEMINATION] Starting anti-entropy loop (interval: %v)", ds.antiEntropyInterval)
		go ds.startAntiEntropyLoop()
	} else {
		log.Printf("[DISSEMINATION] Anti-entropy is disabled")
	}

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

	return ds.forwardDelta(msg, "DELTA")
}

// ProcessReceivedDelta processes a delta received from another node
func (ds *DisseminationSystem) ProcessReceivedDelta(msg DeltaMsg, msgType string) error {
	ds.mutex.Lock()
	ds.receivedCount++
	ds.mutex.Unlock()

	// Deduplication check
	if ds.cache.Contains(msg.ID) {
		ds.mutex.Lock()
		ds.droppedCount++
		ds.mutex.Unlock()
		log.Printf("[DISSEMINATION] %s %s discarded (duplicate)", msgType, msg.ID.String()[:8])
		return nil
	}

	// Add to cache
	ds.cache.Add(msg.ID)

	// TTL check
	if msg.TTL <= 0 {
		ds.mutex.Lock()
		ds.droppedCount++
		ds.mutex.Unlock()
		log.Printf("[DISSEMINATION] %s %s discarded (TTL=0)", msgType, msg.ID.String()[:8])
		return nil
	}

	log.Printf("[DISSEMINATION] Processing %s %s (TTL: %d)", msgType, msg.ID.String()[:8], msg.TTL)

	// Apply received delta to local state
	state.MergeDelta(msg.Data)

	// Decrement TTL and continue dissemination
	msg.TTL--
	msg.SenderID = ds.droneID // Update sender to this node

	return ds.forwardDelta(msg, msgType)
}

// forwardDelta sends delta to up to 'fanout' neighbors (with prioritization)
func (ds *DisseminationSystem) forwardDelta(msg DeltaMsg, msgType string) error {
	neighbors := ds.neighborGetter.GetNeighborURLs()
	if len(neighbors) == 0 {
		log.Printf("[DISSEMINATION] No neighbors available for %s %s", msgType, msg.ID.String()[:8])
		return nil
	}

	// Limit to configured fanout
	targetCount := ds.fanout
	if len(neighbors) < targetCount {
		targetCount = len(neighbors)
	}

	// Prioritize neighbors that haven't received messages recently
	targets := ds.neighborGetter.GetPrioritizedNeighborURLs(targetCount)

	var errors []error
	successCount := 0

	for _, neighbor := range targets {
		url := neighbor.GetURL()
		if err := ds.tcpSender.SendDelta(msgType, url, msg); err != nil {
			log.Printf("[DISSEMINATION] Error sending %s %s to %s: %v",
				msgType, msg.ID.String()[:8], url, err)
			errors = append(errors, err)
		} else {
			successCount++
			// Record successful send using neighbor ID
			ds.neighborGetter.RecordSent(neighbor.ID)
		}
	}

	ds.mutex.Lock()
	ds.sentCount += int64(successCount)
	ds.mutex.Unlock()

	log.Printf("[DISSEMINATION] %s %s sent to %d/%d neighbors (prioritized)",
		msgType, msg.ID.String()[:8], successCount, len(targets))

	if len(errors) > 0 {
		return errors[0] // Return the first error
	}

	return nil
}

// startHeartbeat periodically triggers sending of local delta
func (ds *DisseminationSystem) startHeartbeat() {
	ticker := time.NewTicker(ds.deltaPushInterval)
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
	ticker := time.NewTicker(ds.antiEntropyInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Get one random neighbor using prioritized method
			targets := ds.neighborGetter.GetPrioritizedNeighborURLs(1)
			if len(targets) == 0 {
				continue
			}

			// Get full state
			fullState := state.GetFullState()
			if fullState == nil || len(fullState.Entries) == 0 {
				log.Printf("[ANTI-ENTROPY] No state to sync")
				continue
			}

			// Use the first (and only) neighbor from prioritized list
			neighbor := targets[0]
			targetURL := neighbor.GetURL()

			// Create anti-entropy message (max TTL to ensure delivery)
			msg := DeltaMsg{
				ID:        uuid.New(),
				TTL:       ds.defaultTTL * 2, // Higher TTL for anti-entropy
				Data:      *fullState,
				SenderID:  ds.droneID,
				Timestamp: time.Now().UnixMilli(),
			}

			log.Printf("[ANTI-ENTROPY] Sending full state (%d entries) to %s",
				len(fullState.Entries), targetURL)

			if err := ds.tcpSender.SendDelta("ANTI-ENTROPY", targetURL, msg); err != nil {
				log.Printf("[ANTI-ENTROPY] Error sending to %s: %v", targetURL, err)
			} else {
				ds.mutex.Lock()
				ds.antiEntropyCount++
				ds.mutex.Unlock()
				ds.neighborGetter.RecordSent(neighbor.ID)
				log.Printf("[ANTI-ENTROPY] Full state synced to %s", targetURL)
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

	// Calculate delta messages (total sent - anti-entropy)
	deltaMessages := ds.sentCount - ds.antiEntropyCount
	if deltaMessages < 0 {
		deltaMessages = 0
	}

	return map[string]interface{}{
		"running":                   ds.running,
		"fanout":                    ds.fanout,
		"default_ttl":               ds.defaultTTL,
		"delta_push_interval_sec":   ds.deltaPushInterval.Seconds(),
		"anti_entropy_interval_sec": ds.antiEntropyInterval.Seconds(),
		"sent_count":                ds.sentCount,
		"received_count":            ds.receivedCount,
		"dropped_count":             ds.droppedCount,
		"anti_entropy_count":        ds.antiEntropyCount,
		"delta_messages_sent":       deltaMessages,
		"cache_size":                ds.cache.Size(),
		"neighbor_count":            ds.neighborGetter.Count(),
	}
}

// IsRunning returns whether the system is running
func (ds *DisseminationSystem) IsRunning() bool {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()
	return ds.running
}
