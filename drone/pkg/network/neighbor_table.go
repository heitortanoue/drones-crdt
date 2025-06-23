package network

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/heitortanoue/tcc/pkg/protocol"
)

// Neighbor representa um vizinho descoberto via UDP
type Neighbor struct {
	IP       net.IP    `json:"ip"`
	Port     int       `json:"port"`    // porta TCP para dados
	ID       string    `json:"id"`      // ID do drone (UUID)
	Version  int       `json:"version"` // último delta do drone
	LastSeen time.Time `json:"last_seen"`
}

// NeighborTable gerencia a tabela de vizinhos descobertos
type NeighborTable struct {
	neighbors map[string]*Neighbor // chave: IP como string
	mutex     sync.RWMutex
	timeout   time.Duration
}

// NewNeighborTable cria uma nova tabela de vizinhos
func NewNeighborTable(timeout time.Duration) *NeighborTable {
	nt := &NeighborTable{
		neighbors: make(map[string]*Neighbor),
		timeout:   timeout,
	}

	// Inicia goroutine para limpeza de vizinhos expirados
	go nt.cleanupExpired()

	return nt
}

// AddOrUpdate adiciona ou atualiza um vizinho
func (nt *NeighborTable) AddOrUpdate(hello protocol.HelloMessage, ip net.IP, port int) {
	nt.mutex.Lock()
	defer nt.mutex.Unlock()

	key := hello.ID // Usando ID como chave para unicidade

	nt.neighbors[key] = &Neighbor{
		IP:       ip,
		Port:     port,
		ID:       hello.ID,
		LastSeen: time.Now(),
		Version:  hello.Version,
	}

	log.Println("Neighbor added/updated")
	log.Println(nt.String())
}

// GetActiveNeighbors retorna vizinhos ativos (não expirados)
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

// GetNeighborURLs retorna URLs HTTP dos vizinhos ativos
func (nt *NeighborTable) GetNeighborURLs() []string {
	neighbors := nt.GetActiveNeighbors()
	urls := make([]string, 0, len(neighbors))

	for _, neighbor := range neighbors {
		url := fmt.Sprintf("http://%s:%d", neighbor.IP.String(), neighbor.Port)
		urls = append(urls, url)
	}

	return urls
}

// Count retorna o número de vizinhos ativos
func (nt *NeighborTable) Count() int {
	return len(nt.GetActiveNeighbors())
}

// cleanupExpired remove vizinhos expirados periodicamente
func (nt *NeighborTable) cleanupExpired() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		nt.mutex.Lock()
		now := time.Now()

		// Remove vizinhos que não foram vistos dentro do timeout
		for key, neighbor := range nt.neighbors {
			if now.Sub(neighbor.LastSeen) >= nt.timeout {
				delete(nt.neighbors, key)
			}
		}

		nt.mutex.Unlock()
	}
}

// GetStats retorna estatísticas da tabela de vizinhos
func (nt *NeighborTable) GetStats() map[string]interface{} {
	active := nt.GetActiveNeighbors()
	urls := nt.GetNeighborURLs()

	return map[string]interface{}{
		"neighbors_active": len(active),
		"neighbor_urls":    urls,
		"timeout_seconds":  nt.timeout.Seconds(),
	}
}

// String retorna uma representação legível da tabela de vizinhos
func (nt *NeighborTable) String() string {
	result := "Neighbor Table:\n"
	for _, neighbor := range nt.neighbors {
		result += fmt.Sprintf("IP: %s, Port: %d, ID: %s, Version: %d, LastSeen: %s\n",
			neighbor.IP.String(), neighbor.Port, neighbor.ID, neighbor.Version, neighbor.LastSeen.Format(time.RFC3339))
	}
	return result
}
