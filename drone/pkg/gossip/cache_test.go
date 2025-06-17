package gossip

import (
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestDeduplicationCache_NewCache(t *testing.T) {
	// Teste com capacidade válida
	cache := NewDeduplicationCache(100)
	if cache.capacity != 100 {
		t.Errorf("Esperado capacidade 100, obtido %d", cache.capacity)
	}
	if cache.Size() != 0 {
		t.Errorf("Cache novo deveria estar vazio, size: %d", cache.Size())
	}

	// Teste com capacidade inválida (deve usar padrão)
	cache2 := NewDeduplicationCache(0)
	if cache2.capacity != 1000 {
		t.Errorf("Esperado capacidade padrão 1000, obtido %d", cache2.capacity)
	}

	cache3 := NewDeduplicationCache(-5)
	if cache3.capacity != 1000 {
		t.Errorf("Esperado capacidade padrão 1000, obtido %d", cache3.capacity)
	}
}

func TestDeduplicationCache_AddAndContains(t *testing.T) {
	cache := NewDeduplicationCache(3)
	id1 := uuid.New()
	id2 := uuid.New()

	// Verifica que cache está vazio
	if cache.Contains(id1) {
		t.Error("Cache não deveria conter id1")
	}

	// Adiciona primeiro ID
	cache.Add(id1)
	if !cache.Contains(id1) {
		t.Error("Cache deveria conter id1 após Add")
	}
	if cache.Size() != 1 {
		t.Errorf("Esperado size 1, obtido %d", cache.Size())
	}

	// Adiciona segundo ID
	cache.Add(id2)
	if !cache.Contains(id2) {
		t.Error("Cache deveria conter id2")
	}
	if cache.Size() != 2 {
		t.Errorf("Esperado size 2, obtido %d", cache.Size())
	}

	// Adiciona ID duplicado (não deve aumentar size)
	cache.Add(id1)
	if cache.Size() != 2 {
		t.Errorf("Size não deveria mudar com ID duplicado, obtido %d", cache.Size())
	}
}

func TestDeduplicationCache_LRUEviction(t *testing.T) {
	cache := NewDeduplicationCache(2) // Capacidade pequena
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()

	// Adiciona até capacidade máxima
	cache.Add(id1)
	cache.Add(id2)
	if cache.Size() != 2 {
		t.Errorf("Esperado size 2, obtido %d", cache.Size())
	}

	// Adiciona terceiro ID (deve causar eviction)
	cache.Add(id3)
	if cache.Size() != 2 {
		t.Errorf("Size deveria permanecer 2 após eviction, obtido %d", cache.Size())
	}

	// id1 deveria ter sido removido (least recently used)
	if cache.Contains(id1) {
		t.Error("id1 deveria ter sido removido por LRU")
	}
	if !cache.Contains(id2) {
		t.Error("id2 deveria estar no cache")
	}
	if !cache.Contains(id3) {
		t.Error("id3 deveria estar no cache")
	}
}

func TestDeduplicationCache_LRUOrdering(t *testing.T) {
	cache := NewDeduplicationCache(3)
	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	id4 := uuid.New()

	// Adiciona três IDs
	cache.Add(id1)
	cache.Add(id2)
	cache.Add(id3)

	// Acessa id1 para torná-lo mais recente
	cache.Add(id1) // Move para head

	// Adiciona id4 (deve remover id2, que é o menos recente)
	cache.Add(id4)

	if !cache.Contains(id1) {
		t.Error("id1 deveria estar no cache (foi acessado recentemente)")
	}
	if cache.Contains(id2) {
		t.Error("id2 deveria ter sido removido (least recently used)")
	}
	if !cache.Contains(id3) {
		t.Error("id3 deveria estar no cache")
	}
	if !cache.Contains(id4) {
		t.Error("id4 deveria estar no cache")
	}
}

func TestDeduplicationCache_Clear(t *testing.T) {
	cache := NewDeduplicationCache(10)
	id1 := uuid.New()
	id2 := uuid.New()

	cache.Add(id1)
	cache.Add(id2)
	if cache.Size() != 2 {
		t.Errorf("Esperado size 2 antes do clear, obtido %d", cache.Size())
	}

	cache.Clear()
	if cache.Size() != 0 {
		t.Errorf("Cache deveria estar vazio após Clear, size: %d", cache.Size())
	}
	if cache.Contains(id1) || cache.Contains(id2) {
		t.Error("Cache não deveria conter nenhum ID após Clear")
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
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				id := uuid.New()
				cache.Add(id)
				cache.Contains(id) // Test concurrent reads too
			}
		}(i)
	}

	wg.Wait()

	// Verifica que o cache não está corrompido
	if cache.Size() < 0 || cache.Size() > 1000 {
		t.Errorf("Size inválido após acesso concorrente: %d", cache.Size())
	}

	// Test concurrent stats access
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			stats := cache.GetStats()
			if stats == nil {
				t.Error("GetStats não deveria retornar nil")
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
		t.Fatal("GetStats não deveria retornar nil")
	}

	if capacity, ok := stats["capacity"].(int); !ok || capacity != 50 {
		t.Errorf("Esperado capacity 50, obtido %v", stats["capacity"])
	}

	if currentSize, ok := stats["size"].(int); !ok || currentSize != 2 {
		t.Errorf("Esperado size 2, obtido %v", stats["size"])
	}

	if utilizationFloat, ok := stats["utilization"].(float64); !ok || utilizationFloat != 0.04 {
		t.Errorf("Esperado utilization 0.04, obtido %v", stats["utilization"])
	}
}

func TestDeduplicationCache_StressTest(t *testing.T) {
	cache := NewDeduplicationCache(100)

	// Adiciona mais IDs que a capacidade para testar eviction
	var ids []uuid.UUID
	for i := 0; i < 150; i++ {
		id := uuid.New()
		ids = append(ids, id)
		cache.Add(id)
	}

	// Cache deve ter exatamente a capacidade máxima
	if cache.Size() != 100 {
		t.Errorf("Esperado size 100 após stress test, obtido %d", cache.Size())
	}

	// Primeiros 50 IDs deveriam ter sido removidos
	for i := 0; i < 50; i++ {
		if cache.Contains(ids[i]) {
			t.Errorf("ID %d deveria ter sido removido por eviction", i)
		}
	}

	// Últimos 100 IDs deveriam estar presentes
	for i := 50; i < 150; i++ {
		if !cache.Contains(ids[i]) {
			t.Errorf("ID %d deveria estar no cache", i)
		}
	}
}
