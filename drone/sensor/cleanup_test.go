package sensor

import (
	"fmt"
	"testing"
	"time"
)

// TestCleanupOldDeltas testa a remoção de deltas antigos
func TestCleanupOldDeltas(t *testing.T) {
	crdt := NewSensorCRDT("test-drone")

	// Adiciona alguns deltas com timestamps diferentes
	oldTimestamp := time.Now().Add(-2 * time.Hour).UnixMilli()
	recentTimestamp := time.Now().UnixMilli()

	// Delta antigo
	oldReading := SensorReading{
		SensorID:  "sensor-old",
		Value:     25.0,
		Timestamp: oldTimestamp,
	}
	crdt.AddDelta(oldReading)

	// Delta recente
	recentReading := SensorReading{
		SensorID:  "sensor-recent",
		Value:     30.0,
		Timestamp: recentTimestamp,
	}
	crdt.AddDelta(recentReading)

	// Verifica que temos 2 deltas
	if count := crdt.GetTotalDeltasCount(); count != 2 {
		t.Errorf("Esperado 2 deltas, obtido %d", count)
	}

	// Remove deltas mais antigos que 1 hora
	limitTimestamp := time.Now().Add(-1 * time.Hour).UnixMilli()
	removedCount := crdt.CleanupOldDeltas(limitTimestamp)

	// Deve ter removido apenas o delta antigo
	if removedCount != 1 {
		t.Errorf("Esperado 1 delta removido, obtido %d", removedCount)
	}

	if count := crdt.GetTotalDeltasCount(); count != 1 {
		t.Errorf("Esperado 1 delta restante, obtido %d", count)
	}
}

// TestCleanupOldDeltasByAge testa limpeza por duração
func TestCleanupOldDeltasByAge(t *testing.T) {
	crdt := NewSensorCRDT("test-drone")

	// Adiciona deltas simulando idades diferentes
	timestamps := []int64{
		time.Now().Add(-3 * time.Hour).UnixMilli(),    // Muito antigo
		time.Now().Add(-30 * time.Minute).UnixMilli(), // Moderadamente antigo
		time.Now().UnixMilli(),                        // Recente
	}

	for i, ts := range timestamps {
		reading := SensorReading{
			SensorID:  fmt.Sprintf("sensor-%d", i),
			Value:     float64(20 + i*5),
			Timestamp: ts,
		}
		crdt.AddDelta(reading)
	}

	// Verifica que temos 3 deltas
	if count := crdt.GetTotalDeltasCount(); count != 3 {
		t.Errorf("Esperado 3 deltas, obtido %d", count)
	}

	// Remove deltas mais antigos que 1 hora
	removedCount := crdt.CleanupOldDeltasByAge(1 * time.Hour)

	// Deve ter removido apenas o primeiro (mais antigo que 1 hora)
	if removedCount != 1 {
		t.Errorf("Esperado 1 delta removido, obtido %d", removedCount)
	}

	if count := crdt.GetTotalDeltasCount(); count != 2 {
		t.Errorf("Esperado 2 deltas restantes, obtido %d", count)
	}
}

// TestTrimToMaxSize testa limitação por tamanho máximo
func TestTrimToMaxSize(t *testing.T) {
	crdt := NewSensorCRDT("test-drone")

	// Adiciona 10 deltas com timestamps crescentes
	for i := 0; i < 10; i++ {
		reading := SensorReading{
			SensorID:  fmt.Sprintf("sensor-%d", i),
			Value:     float64(20 + i),
			Timestamp: time.Now().Add(time.Duration(i) * time.Minute).UnixMilli(),
		}
		crdt.AddDelta(reading)
	}

	// Verifica que temos 10 deltas
	if count := crdt.GetTotalDeltasCount(); count != 10 {
		t.Errorf("Esperado 10 deltas, obtido %d", count)
	}

	// Limita a 5 deltas (deve manter os 5 mais recentes)
	removedCount := crdt.TrimToMaxSize(5)

	if removedCount != 5 {
		t.Errorf("Esperado 5 deltas removidos, obtido %d", removedCount)
	}

	if count := crdt.GetTotalDeltasCount(); count != 5 {
		t.Errorf("Esperado 5 deltas restantes, obtido %d", count)
	}

	// Verifica que os deltas restantes são os mais recentes
	state := crdt.GetState()
	if len(state) != 5 {
		t.Errorf("Esperado 5 deltas no estado, obtido %d", len(state))
	}

	// Verifica que manteve os sensores 5-9 (mais recentes)
	foundSensors := make(map[string]bool)
	for _, delta := range state {
		foundSensors[delta.SensorID] = true
	}

	expectedSensors := []string{"sensor-5", "sensor-6", "sensor-7", "sensor-8", "sensor-9"}
	for _, sensor := range expectedSensors {
		if !foundSensors[sensor] {
			t.Errorf("Esperado encontrar sensor %s nos deltas restantes", sensor)
		}
	}
}

// TestTrimPendingBuffer testa limitação do buffer de pendentes
func TestTrimPendingBuffer(t *testing.T) {
	crdt := NewSensorCRDT("test-drone")

	// Adiciona 10 deltas para encher o buffer pendente
	for i := 0; i < 10; i++ {
		reading := SensorReading{
			SensorID:  fmt.Sprintf("sensor-%d", i),
			Value:     float64(20 + i),
			Timestamp: GenerateTimestamp(),
		}
		crdt.AddDelta(reading)
	}

	// Verifica que temos 10 pendentes
	pending := crdt.GetPendingDeltas()
	if len(pending) != 10 {
		t.Errorf("Esperado 10 deltas pendentes, obtido %d", len(pending))
	}

	// Limita o buffer a 5
	crdt.TrimPendingBuffer(5)

	// Verifica que agora temos apenas 5 pendentes (os mais recentes)
	pending = crdt.GetPendingDeltas()
	if len(pending) != 5 {
		t.Errorf("Esperado 5 deltas pendentes após trim, obtido %d", len(pending))
	}
}

// TestGetMemoryStats testa as estatísticas de memória
func TestGetMemoryStats(t *testing.T) {
	crdt := NewSensorCRDT("test-drone")

	// Estado inicial
	stats := crdt.GetMemoryStats()
	if stats["total_deltas"] != 0 {
		t.Errorf("Esperado 0 deltas iniciais, obtido %d", stats["total_deltas"])
	}

	// Adiciona alguns deltas
	for i := 0; i < 5; i++ {
		reading := SensorReading{
			SensorID:  fmt.Sprintf("sensor-%d", i),
			Value:     float64(20 + i),
			Timestamp: GenerateTimestamp(),
		}
		crdt.AddDelta(reading)
	}

	// Verifica estatísticas
	stats = crdt.GetMemoryStats()
	if stats["total_deltas"] != 5 {
		t.Errorf("Esperado 5 deltas totais, obtido %d", stats["total_deltas"])
	}

	if stats["pending_deltas"] != 5 {
		t.Errorf("Esperado 5 deltas pendentes, obtido %d", stats["pending_deltas"])
	}

	if stats["latest_readings"] != 5 {
		t.Errorf("Esperado 5 leituras latest, obtido %d", stats["latest_readings"])
	}
}

// TestCleanupIntegration testa integração de várias operações de limpeza
func TestCleanupIntegration(t *testing.T) {
	crdt := NewSensorCRDT("test-drone")

	// Cenário: adiciona deltas com idades variadas
	baseTime := time.Now()
	scenarios := []struct {
		sensorID string
		age      time.Duration
		value    float64
	}{
		{"sensor-very-old", 5 * time.Hour, 10.0},
		{"sensor-old", 2 * time.Hour, 15.0},
		{"sensor-medium", 30 * time.Minute, 20.0},
		{"sensor-recent", 5 * time.Minute, 25.0},
		{"sensor-new", 0, 30.0},
	}

	for _, scenario := range scenarios {
		reading := SensorReading{
			SensorID:  scenario.sensorID,
			Value:     scenario.value,
			Timestamp: baseTime.Add(-scenario.age).UnixMilli(),
		}
		crdt.AddDelta(reading)
	}

	// Verifica estado inicial
	initialCount := crdt.GetTotalDeltasCount()
	if initialCount != 5 {
		t.Errorf("Esperado 5 deltas iniciais, obtido %d", initialCount)
	}

	// Limpeza por idade: remove dados mais antigos que 1 hora
	removedByAge := crdt.CleanupOldDeltasByAge(1 * time.Hour)
	if removedByAge != 2 {
		t.Errorf("Esperado 2 deltas removidos por idade, obtido %d", removedByAge)
	}

	// Deve sobrar 3 deltas
	if count := crdt.GetTotalDeltasCount(); count != 3 {
		t.Errorf("Esperado 3 deltas após limpeza por idade, obtido %d", count)
	}

	// Limpeza por tamanho: mantém apenas 2
	removedBySize := crdt.TrimToMaxSize(2)
	if removedBySize != 1 {
		t.Errorf("Esperado 1 delta removido por tamanho, obtido %d", removedBySize)
	}

	// Deve sobrar 2 deltas (os mais recentes)
	finalCount := crdt.GetTotalDeltasCount()
	if finalCount != 2 {
		t.Errorf("Esperado 2 deltas finais, obtido %d", finalCount)
	}

	// Verifica que sobraram os sensores mais recentes
	state := crdt.GetState()
	sensorIDs := make([]string, len(state))
	for i, delta := range state {
		sensorIDs[i] = delta.SensorID
	}

	expectedFinal := []string{"sensor-recent", "sensor-new"}
	for _, expected := range expectedFinal {
		found := false
		for _, id := range sensorIDs {
			if id == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Esperado encontrar sensor %s nos deltas finais", expected)
		}
	}
}
