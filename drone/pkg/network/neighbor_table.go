package network

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// Neighbor representa um vizinho descoberto via UDP
type Neighbor struct {
	IP       net.IP    `json:"ip"`
	LastSeen time.Time `json:"last_seen"`
	Port     int       `json:"port"` // porta TCP para dados
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
func (nt *NeighborTable) AddOrUpdate(ip net.IP, port int) {
	nt.mutex.Lock()
	defer nt.mutex.Unlock()

	key := ip.String()
	nt.neighbors[key] = &Neighbor{
		IP:       ip,
		LastSeen: time.Now(),
		Port:     port,
	}
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

	return map[string]interface{}{
		"neighbors_active": len(active),
		"timeout_seconds":  nt.timeout.Seconds(),
	}
}
