package gossip

import (
	"sync"

	"github.com/google/uuid"
)

// DeduplicationCache implementa cache LRU para deduplicação (Requisito F7)
type DeduplicationCache struct {
	capacity int
	cache    map[uuid.UUID]*cacheNode
	head     *cacheNode
	tail     *cacheNode
	mutex    sync.RWMutex
}

// cacheNode representa um nó na lista duplamente ligada
type cacheNode struct {
	key  uuid.UUID
	prev *cacheNode
	next *cacheNode
}

// NewDeduplicationCache cria um novo cache LRU
func NewDeduplicationCache(capacity int) *DeduplicationCache {
	if capacity <= 0 {
		capacity = 1000 // Valor padrão
	}

	// Cria sentinelas para head e tail
	head := &cacheNode{}
	tail := &cacheNode{}
	head.next = tail
	tail.prev = head

	return &DeduplicationCache{
		capacity: capacity,
		cache:    make(map[uuid.UUID]*cacheNode),
		head:     head,
		tail:     tail,
	}
}

// Contains verifica se o ID está no cache
func (dc *DeduplicationCache) Contains(id uuid.UUID) bool {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	_, exists := dc.cache[id]
	return exists
}

// Add adiciona um ID ao cache (move para o início se já existe)
func (dc *DeduplicationCache) Add(id uuid.UUID) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	if node, exists := dc.cache[id]; exists {
		// Move para o início (mais recente)
		dc.moveToHead(node)
		return
	}

	// Cria novo nó
	newNode := &cacheNode{key: id}
	dc.cache[id] = newNode
	dc.addToHead(newNode)

	// Remove o menos recente se exceder capacidade
	if len(dc.cache) > dc.capacity {
		tail := dc.removeTail()
		delete(dc.cache, tail.key)
	}
}

// Size retorna o tamanho atual do cache
func (dc *DeduplicationCache) Size() int {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()
	return len(dc.cache)
}

// Clear limpa todo o cache
func (dc *DeduplicationCache) Clear() {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	dc.cache = make(map[uuid.UUID]*cacheNode)
	dc.head.next = dc.tail
	dc.tail.prev = dc.head
}

// GetStats retorna estatísticas do cache
func (dc *DeduplicationCache) GetStats() map[string]interface{} {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	return map[string]interface{}{
		"capacity":    dc.capacity,
		"size":        len(dc.cache),
		"utilization": float64(len(dc.cache)) / float64(dc.capacity),
	}
}

// addToHead adiciona nó após a cabeça
func (dc *DeduplicationCache) addToHead(node *cacheNode) {
	node.prev = dc.head
	node.next = dc.head.next
	dc.head.next.prev = node
	dc.head.next = node
}

// removeNode remove um nó específico
func (dc *DeduplicationCache) removeNode(node *cacheNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

// moveToHead move nó para após a cabeça
func (dc *DeduplicationCache) moveToHead(node *cacheNode) {
	dc.removeNode(node)
	dc.addToHead(node)
}

// removeTail remove e retorna o nó antes da cauda
func (dc *DeduplicationCache) removeTail() *cacheNode {
	lastNode := dc.tail.prev
	dc.removeNode(lastNode)
	return lastNode
}
