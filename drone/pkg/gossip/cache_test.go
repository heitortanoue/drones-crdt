package gossip

import (
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestDeduplicationCache_NewCache(t *testing.T) {
	// Test with valid capacity
	cache := NewDeduplicationCache(100)
	if cache.capacity != 100 {
		t.Errorf("Expected capacity 100, got %d", cache.capacity)
	}
	if cache.Size() != 0 {
		t.Errorf("New cache should be empty, size: %d", cache.Size())
	}

	// Test with invalid capacity (should use default)
	cache2 := NewDeduplicationCache(0)
	if cache2.capacity != 1000 {
		t.Errorf("Expected default capacity 1000, got %d", cache2.capacity)
	}

	cache3 := NewDeduplicationCache(-5)
	if cache3.capacity != 1000 {
		t.Errorf("Expected default capacity 1000, got %d", cache3.capacity)
	}
}

func TestDeduplicationCache_AddAndContains(t *testing.T) {
	cache := NewDeduplicationCache(3)
	id1 := uuid.New()
	id2 := uuid.New()

	// Cache should start empty
	if cache.Contains(id1) {
		t.Error("Cache should not contain id1 initially")
	}

	// Add first ID
	cache.Add(id1)
	if !cache.Contains(id1) {
		t.Error("Cache should contain id1 after Add")
	}
	if cache.Size() != 1 {
		t.Errorf("Expected size 1, got %d", cache.Size())
	}

	// Add second ID
	cache.Add(id2)
	if !cache.Contains(id2) {
		t.Error("Cache should contain id2")
	}
	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}

	// Add duplicate ID (size should not increase)
	cache.Add(id1)
	if cache.Size() != 2 {
		t.Errorf("Size should not change with duplicate ID, got %d", cache.Size())
	}
}

func TestDeduplicationCache_LRUEviction(t *testing.T) {
	cache := NewDeduplicationCache(2) // Small capacity
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()

	// Fill to capacity
	cache.Add(id1)
	cache.Add(id2)
	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}

	// Add third ID (should trigger eviction)
	cache.Add(id3)
	if cache.Size() != 2 {
		t.Errorf("Size should remain 2 after eviction, got %d", cache.Size())
	}

	// id1 should have been evicted (least recently used)
	if cache.Contains(id1) {
		t.Error("id1 should have been evicted by LRU")
	}
	if !cache.Contains(id2) {
		t.Error("id2 should still be in the cache")
	}
	if !cache.Contains(id3) {
		t.Error("id3 should be in the cache")
	}
}

func TestDeduplicationCache_LRUOrdering(t *testing.T) {
	cache := NewDeduplicationCache(3)
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	id4 := uuid.New()

	// Add three IDs
	cache.Add(id1)
	cache.Add(id2)
	cache.Add(id3)

	// Access id1 to make it most recent
	cache.Add(id1) // Move to head

	// Add id4 (should evict id2 as least recently used)
	cache.Add(id4)

	if !cache.Contains(id1) {
		t.Error("id1 should still be in the cache (was recently accessed)")
	}
	if cache.Contains(id2) {
		t.Error("id2 should have been evicted (least recently used)")
	}
	if !cache.Contains(id3) {
		t.Error("id3 should be in the cache")
	}
	if !cache.Contains(id4) {
		t.Error("id4 should be in the cache")
	}
}

func TestDeduplicationCache_Clear(t *testing.T) {
	cache := NewDeduplicationCache(10)
	id1 := uuid.New()
	id2 := uuid.New()

	cache.Add(id1)
	cache.Add(id2)
	if cache.Size() != 2 {
		t.Errorf("Expected size 2 before Clear, got %d", cache.Size())
	}

	cache.Clear()
	if cache.Size() != 0 {
		t.Errorf("Cache should be empty after Clear, size: %d", cache.Size())
	}
	if cache.Contains(id1) || cache.Contains(id2) {
		t.Error("Cache should not contain any IDs after Clear")
	}
}

func TestDeduplicationCache_ConcurrentAccess(t *testing.T) {
	cache := NewDeduplicationCache(1000)
	var wg sync.WaitGroup
	numGoroutines := 100
	idsPerGoroutine := 10

	// Test concurrent writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id := uuid.New()
				cache.Add(id)
				cache.Contains(id) // Test concurrent reads as well
			}
		}()
	}
	wg.Wait()

	// Cache should not be corrupted
	if cache.Size() < 0 || cache.Size() > 1000 {
		t.Errorf("Invalid size after concurrent access: %d", cache.Size())
	}

	// Test concurrent stats access
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			stats := cache.GetStats()
			if stats == nil {
				t.Error("GetStats should not return nil")
			}
		}()
	}
	wg.Wait()
}

func TestDeduplicationCache_GetStats(t *testing.T) {
	cache := NewDeduplicationCache(50)
	id1 := uuid.New()
	id2 := uuid.New()

	cache.Add(id1)
	cache.Add(id2)

	stats := cache.GetStats()
	if stats == nil {
		t.Fatal("GetStats should not return nil")
	}

	if capacity, ok := stats["capacity"].(int); !ok || capacity != 50 {
		t.Errorf("Expected capacity 50, got %v", stats["capacity"])
	}

	if currentSize, ok := stats["size"].(int); !ok || currentSize != 2 {
		t.Errorf("Expected size 2, got %v", stats["size"])
	}

	if utilizationFloat, ok := stats["utilization"].(float64); !ok || utilizationFloat != 0.04 {
		t.Errorf("Expected utilization 0.04, got %v", stats["utilization"])
	}
}

func TestDeduplicationCache_StressTest(t *testing.T) {
	cache := NewDeduplicationCache(100)

	// Add more IDs than capacity to test eviction
	var ids []uuid.UUID
	for i := 0; i < 150; i++ {
		id := uuid.New()
		ids = append(ids, id)
		cache.Add(id)
	}

	// Cache should have exactly max capacity
	if cache.Size() != 100 {
		t.Errorf("Expected size 100 after stress test, got %d", cache.Size())
	}

	// First 50 IDs should have been evicted
	for i := 0; i < 50; i++ {
		if cache.Contains(ids[i]) {
			t.Errorf("ID %d should have been evicted", i)
		}
	}

	// Last 100 IDs should be present
	for i := 50; i < 150; i++ {
		if !cache.Contains(ids[i]) {
			t.Errorf("ID %d should be in the cache", i)
		}
	}
}