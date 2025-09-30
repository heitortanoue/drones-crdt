package network

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/heitortanoue/tcc/pkg/protocol"
)

// Neighbor represents a neighbor discovered via UDP
type Neighbor struct {
	IP       net.IP    `json:"ip"`
	Port     int       `json:"port"`    // TCP port for data
	ID       string    `json:"id"`      // Drone ID (UUID)
	Version  int       `json:"version"` // Last delta version of the drone
	LastSeen time.Time `json:"last_seen"`
}

// NeighborTable manages the table of discovered neighbors
type NeighborTable struct {
	neighbors map[string]*Neighbor // key: Drone ID
	mutex     sync.RWMutex
	timeout   time.Duration
}

// NewNeighborTable creates a new neighbor table
func NewNeighborTable(timeout time.Duration) *NeighborTable {
	nt := &NeighborTable{
		neighbors: make(map[string]*Neighbor),
		timeout:   timeout,
	}

	// Start goroutine for cleaning up expired neighbors
	go nt.cleanupExpired()

	return nt
}

// AddOrUpdate adds or updates a neighbor entry
func (nt *NeighborTable) AddOrUpdate(hello protocol.HelloMessage, ip net.IP, port int) {
	nt.mutex.Lock()
	defer nt.mutex.Unlock()

	key := hello.ID // Use drone ID as unique key

	nt.neighbors[key] = &Neighbor{
		IP:       ip,
		Port:     port,
		ID:       hello.ID,
		LastSeen: time.Now(),
	}

	log.Println("Neighbor added/updated")
	log.Println(nt.String())
}

// GetActiveNeighbors returns only active (non-expired) neighbors
func (nt *NeighborTable) GetActiveNeighbors() []*Neighbor {
	nt.mutex.RLock()
	defer nt.mutex.RUnlock()

	now := time.Now()
	var active []*Neighbor

	for _, neighbor := range nt.neighbors {
		if now.Sub(neighbor.LastSeen) < nt.timeout {
			active = append(active, neighbor)
		}
	}

	return active
}

// GetNeighborURLs returns HTTP URLs of active neighbors
func (nt *NeighborTable) GetNeighborURLs() []string {
	neighbors := nt.GetActiveNeighbors()
	urls := make([]string, 0, len(neighbors))

	for _, neighbor := range neighbors {
		url := fmt.Sprintf("http://%s:%d", neighbor.IP.String(), neighbor.Port)
		urls = append(urls, url)
	}

	return urls
}

// Count returns the number of active neighbors
func (nt *NeighborTable) Count() int {
	return len(nt.GetActiveNeighbors())
}

// cleanupExpired periodically removes expired neighbors
func (nt *NeighborTable) cleanupExpired() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		nt.mutex.Lock()
		now := time.Now()

		// Remove neighbors not seen within timeout
		for key, neighbor := range nt.neighbors {
			if now.Sub(neighbor.LastSeen) >= nt.timeout {
				delete(nt.neighbors, key)
			}
		}

		nt.mutex.Unlock()
	}
}

// GetStats returns neighbor table statistics
func (nt *NeighborTable) GetStats() map[string]interface{} {
	active := nt.GetActiveNeighbors()
	urls := nt.GetNeighborURLs()

	return map[string]interface{}{
		"neighbors_active": len(active),
		"neighbor_urls":    urls,
		"timeout_seconds":  nt.timeout.Seconds(),
	}
}

// String returns a human-readable representation of the neighbor table
func (nt *NeighborTable) String() string {
	result := "Neighbor Table:\n"
	for _, neighbor := range nt.neighbors {
		result += fmt.Sprintf("IP: %s, Port: %d, ID: %s, Version: %d, LastSeen: %s\n",
			neighbor.IP.String(), neighbor.Port, neighbor.ID, neighbor.Version, neighbor.LastSeen.Format(time.RFC3339))
	}
	return result
}