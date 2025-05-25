package sensor

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func BenchmarkSensorCRDT_AddDelta(b *testing.B) {
	crdt := NewSensorCRDT("benchmark-drone")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reading := SensorReading{
			SensorID:  fmt.Sprintf("sensor-%d", i%100),   // 100 sensores diferentes
			Timestamp: time.Now().UnixMilli() + int64(i), // timestamps únicos
			Value:     rand.Float64() * 100,
		}
		crdt.AddDelta(reading)
	}
}

func BenchmarkSensorCRDT_Merge1000Deltas(b *testing.B) {
	// Prepara 1000 deltas
	deltas := make([]SensorDelta, 1000)
	baseTime := time.Now().UnixMilli()

	for i := 0; i < 1000; i++ {
		deltas[i] = SensorDelta{
			DroneID:   "remote-drone",
			SensorID:  fmt.Sprintf("sensor-%d", i%50),
			Timestamp: baseTime + int64(i),
			Value:     rand.Float64() * 100,
		}
	}

	batch := DeltaBatch{
		SenderID: "benchmark-sender",
		Deltas:   deltas,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Cria novo CRDT para cada iteração
		testCRDT := NewSensorCRDT("test-drone")
		testCRDT.Merge(batch)
	}
}

func BenchmarkSensorCRDT_MergeDuplicates(b *testing.B) {
	crdt := NewSensorCRDT("benchmark-drone")

	// Prepara deltas com muitas duplicatas
	deltas := make([]SensorDelta, 1000)
	baseTime := time.Now().UnixMilli()

	for i := 0; i < 1000; i++ {
		deltas[i] = SensorDelta{
			DroneID:   "remote-drone",
			SensorID:  fmt.Sprintf("sensor-%d", i%10), // apenas 10 sensores (muitas duplicatas)
			Timestamp: baseTime + int64(i%10),         // timestamps repetidos
			Value:     rand.Float64() * 100,
		}
	}

	batch := DeltaBatch{
		SenderID: "benchmark-sender",
		Deltas:   deltas,
	}

	// Primeira merge para popular
	crdt.Merge(batch)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Merge com duplicatas (deve ser rápido devido ao pruning)
		crdt.Merge(batch)
	}
}

func BenchmarkSensorCRDT_GetState(b *testing.B) {
	crdt := NewSensorCRDT("benchmark-drone")

	// Adiciona muitos deltas
	for i := 0; i < 10000; i++ {
		reading := SensorReading{
			SensorID:  fmt.Sprintf("sensor-%d", i),
			Timestamp: time.Now().UnixMilli() + int64(i),
			Value:     rand.Float64() * 100,
		}
		crdt.AddDelta(reading)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = crdt.GetState()
	}
}

func BenchmarkSensorCRDT_ConcurrentOperations(b *testing.B) {
	crdt := NewSensorCRDT("benchmark-drone")

	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			reading := SensorReading{
				SensorID:  fmt.Sprintf("sensor-%d", counter%100),
				Timestamp: time.Now().UnixMilli() + int64(counter),
				Value:     rand.Float64() * 100,
			}
			crdt.AddDelta(reading)
			counter++
		}
	})
}

// Test específico para validar performance com 1000+ deltas
func TestSensorCRDT_Performance1000Plus(t *testing.T) {
	if testing.Short() {
		t.Skip("Pulando teste de performance em modo short")
	}

	crdt := NewSensorCRDT("perf-test-drone")

	// Mede tempo para adicionar 1000 deltas
	start := time.Now()

	for i := 0; i < 1000; i++ {
		reading := SensorReading{
			SensorID:  fmt.Sprintf("sensor-%d", i),
			Timestamp: time.Now().UnixMilli() + int64(i),
			Value:     rand.Float64() * 100,
		}
		crdt.AddDelta(reading)
	}

	addDuration := time.Since(start)

	// Mede tempo para merge de 1000 deltas
	batch := DeltaBatch{
		SenderID: "remote-test",
		Deltas:   make([]SensorDelta, 1000),
	}

	baseTime := time.Now().UnixMilli()
	for i := 0; i < 1000; i++ {
		batch.Deltas[i] = SensorDelta{
			DroneID:   "remote-drone",
			SensorID:  fmt.Sprintf("remote-sensor-%d", i),
			Timestamp: baseTime + int64(i),
			Value:     rand.Float64() * 100,
		}
	}

	start = time.Now()
	mergedCount := crdt.Merge(batch)
	mergeDuration := time.Since(start)

	// Verifica resultados
	if mergedCount != 1000 {
		t.Errorf("Esperado merge de 1000 deltas, obtido %d", mergedCount)
	}

	totalCount := crdt.GetTotalDeltasCount()
	if totalCount != 2000 {
		t.Errorf("Esperado total de 2000 deltas, obtido %d", totalCount)
	}

	// Reporta performance
	t.Logf("Performance para 1000+ deltas:")
	t.Logf("  Add: %v (%.2f ops/ms)", addDuration, 1000.0/float64(addDuration.Milliseconds()))
	t.Logf("  Merge: %v (%.2f ops/ms)", mergeDuration, 1000.0/float64(mergeDuration.Milliseconds()))
	t.Logf("  Total deltas: %d", totalCount)

	// Verifica se performance está aceitável (< 100ms para 1000 operações)
	if addDuration > 100*time.Millisecond {
		t.Errorf("Performance de Add muito lenta: %v > 100ms", addDuration)
	}

	if mergeDuration > 100*time.Millisecond {
		t.Errorf("Performance de Merge muito lenta: %v > 100ms", mergeDuration)
	}
}
