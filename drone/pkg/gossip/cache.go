package gossip

import (
	"sync"

	"github.com/google/uuid"
)

// DeduplicationCache implements an LRU cache for deduplication
type DeduplicationCache struct {
	capacity int
	cache    map[uuid.UUID]*cacheNode
	head     *cacheNode
	tail     *cacheNode
	mutex    sync.RWMutex
}

// cacheNode represents a node in the doubly linked list
type cacheNode struct {
	key  uuid.UUID
	prev *cacheNode
	next *cacheNode
}

// NewDeduplicationCache creates a new LRU cache
func NewDeduplicationCache(capacity int) *DeduplicationCache {
	if capacity <= 0 {
		capacity = 1000 // Default value
	}

	// Create sentinels for head and tail
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

// Contains checks if the ID is in the cache
func (dc *DeduplicationCache) Contains(id uuid.UUID) bool {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	_, exists := dc.cache[id]
	return exists
}

// Add inserts an ID into the cache (moves it to the head if it already exists)
func (dc *DeduplicationCache) Add(id uuid.UUID) {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	if node, exists := dc.cache[id]; exists {
		// Move to the head (most recently used)
		dc.moveToHead(node)
		return
	}

	// Create new node
	newNode := &cacheNode{key: id}
	dc.cache[id] = newNode
	dc.addToHead(newNode)

	// Remove least recently used if over capacity
	if len(dc.cache) > dc.capacity {
		tail := dc.removeTail()
		delete(dc.cache, tail.key)
	}
}

// Size returns the current cache size
func (dc *DeduplicationCache) Size() int {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()
	return len(dc.cache)
}

// Clear removes all entries from the cache
func (dc *DeduplicationCache) Clear() {
	dc.mutex.Lock()
	defer dc.mutex.Unlock()

	dc.cache = make(map[uuid.UUID]*cacheNode)
	dc.head.next = dc.tail
	dc.tail.prev = dc.head
}

// GetStats returns cache statistics
func (dc *DeduplicationCache) GetStats() map[string]interface{} {
	dc.mutex.RLock()
	defer dc.mutex.RUnlock()

	return map[string]interface{}{
		"capacity":    dc.capacity,
		"size":        len(dc.cache),
		"utilization": float64(len(dc.cache)) / float64(dc.capacity),
	}
}

// addToHead inserts a node right after the head
func (dc *DeduplicationCache) addToHead(node *cacheNode) {
	node.prev = dc.head
	node.next = dc.head.next
	dc.head.next.prev = node
	dc.head.next = node
}

// removeNode removes a specific node
func (dc *DeduplicationCache) removeNode(node *cacheNode) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

// moveToHead moves a node to right after the head
func (dc *DeduplicationCache) moveToHead(node *cacheNode) {
	dc.removeNode(node)
	dc.addToHead(node)
}

// removeTail removes and returns the node before the tail
func (dc *DeduplicationCache) removeTail() *cacheNode {
	lastNode := dc.tail.prev
	dc.removeNode(lastNode)
	return lastNode
}