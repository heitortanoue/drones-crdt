package sensor

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDeltaSet_BasicOperations(t *testing.T) {
	ds := NewDeltaSet()

	// Testa adição
	delta1 := NewSensorDelta("drone-1", "sensor-A", 25.5)
	ds.Add(delta1)

	if ds.Size() != 1 {
		t.Errorf("Esperado size 1, obtido %d", ds.Size())
	}

	// Testa Contains
	if !ds.Contains(delta1.ID) {
		t.Error("Delta deveria estar presente")
	}

	// Testa GetAll
	all := ds.GetAll()
	if len(all) != 1 {
		t.Errorf("Esperado 1 delta, obtido %d", len(all))
	}

	if all[0].ID != delta1.ID {
		t.Error("Delta retornado não corresponde ao adicionado")
	}
}

func TestDeltaSet_Apply(t *testing.T) {
	ds := NewDeltaSet()

	delta := NewSensorDelta("drone-1", "sensor-A", 30.0)

	// Primeira aplicação deve retornar true
	if !ds.Apply(delta) {
		t.Error("Primeira aplicação deveria retornar true")
	}

	// Segunda aplicação do mesmo delta deve retornar false
	if ds.Apply(delta) {
		t.Error("Segunda aplicação do mesmo delta deveria retornar false")
	}

	if ds.Size() != 1 {
		t.Errorf("Size deveria ser 1, obtido %d", ds.Size())
	}
}

func TestDeltaSet_Merge(t *testing.T) {
	ds1 := NewDeltaSet()
	ds2 := NewDeltaSet()

	// Adiciona deltas no primeiro conjunto
	delta1 := NewSensorDelta("drone-1", "sensor-A", 25.0)
	delta2 := NewSensorDelta("drone-1", "sensor-B", 35.0)
	ds1.Add(delta1)
	ds1.Add(delta2)

	// Adiciona deltas no segundo conjunto (um sobreposto, um novo)
	delta3 := NewSensorDelta("drone-2", "sensor-C", 45.0)
	ds2.Add(delta2) // Mesmo delta que já existe em ds1
	ds2.Add(delta3) // Novo delta

	// Merge ds2 em ds1
	mergedCount := ds1.Merge(ds2)

	if mergedCount != 1 {
		t.Errorf("Esperado merge count 1, obtido %d", mergedCount)
	}

	if ds1.Size() != 3 {
		t.Errorf("Tamanho esperado 3, obtido %d", ds1.Size())
	}

	// Verifica se todos os deltas estão presentes
	if !ds1.Contains(delta1.ID) || !ds1.Contains(delta2.ID) || !ds1.Contains(delta3.ID) {
		t.Error("Nem todos os deltas estão presentes após merge")
	}
}

func TestDeltaSet_GetLatestBySensor(t *testing.T) {
	ds := NewDeltaSet()

	// Adiciona deltas para mesmo sensor em tempos diferentes
	delta1 := SensorDelta{
		ID:        uuid.New(),
		SensorID:  "sensor-A",
		Timestamp: 1000,
		Value:     10.0,
		DroneID:   "drone-1",
	}

	delta2 := SensorDelta{
		ID:        uuid.New(),
		SensorID:  "sensor-A",
		Timestamp: 2000, // Mais recente
		Value:     20.0,
		DroneID:   "drone-1",
	}

	delta3 := SensorDelta{
		ID:        uuid.New(),
		SensorID:  "sensor-B",
		Timestamp: 1500,
		Value:     15.0,
		DroneID:   "drone-1",
	}

	ds.Add(delta1)
	ds.Add(delta2)
	ds.Add(delta3)

	latest := ds.GetLatestBySensor()

	if len(latest) != 2 {
		t.Errorf("Esperado 2 sensores únicos, obtido %d", len(latest))
	}

	// Para sensor-A, deve retornar delta2 (mais recente)
	if latest["sensor-A"].ID != delta2.ID {
		t.Error("Delta mais recente para sensor-A não foi retornado")
	}

	if latest["sensor-A"].Value != 20.0 {
		t.Errorf("Valor esperado 20.0, obtido %f", latest["sensor-A"].Value)
	}

	// Para sensor-B, deve retornar delta3
	if latest["sensor-B"].ID != delta3.ID {
		t.Error("Delta para sensor-B não foi retornado")
	}
}

func TestDeltaSet_CleanupOldDeltas(t *testing.T) {
	ds := NewDeltaSet()

	now := time.Now().UnixMilli()
	oldTime := now - 10000 // 10 segundos atrás

	// Adiciona delta antigo
	oldDelta := SensorDelta{
		ID:        uuid.New(),
		SensorID:  "sensor-old",
		Timestamp: oldTime,
		Value:     10.0,
		DroneID:   "drone-1",
	}

	// Adiciona delta recente
	newDelta := SensorDelta{
		ID:        uuid.New(),
		SensorID:  "sensor-new",
		Timestamp: now,
		Value:     20.0,
		DroneID:   "drone-1",
	}

	ds.Add(oldDelta)
	ds.Add(newDelta)

	// Limpa deltas mais antigos que 5 segundos
	limitTime := now - 5000
	removedCount := ds.CleanupOldDeltas(limitTime)

	if removedCount != 1 {
		t.Errorf("Esperado 1 delta removido, obtido %d", removedCount)
	}

	if ds.Size() != 1 {
		t.Errorf("Tamanho esperado 1, obtido %d", ds.Size())
	}

	// Deve ter mantido apenas o delta recente
	if !ds.Contains(newDelta.ID) {
		t.Error("Delta recente deveria ter sido mantido")
	}

	if ds.Contains(oldDelta.ID) {
		t.Error("Delta antigo deveria ter sido removido")
	}
}

func TestDeltaSet_ConcurrentAccess(t *testing.T) {
	ds := NewDeltaSet()
	done := make(chan bool, 2)

	// Goroutine 1: adiciona deltas
	go func() {
		for i := 0; i < 100; i++ {
			delta := NewSensorDelta("drone-1", "sensor-A", float64(i))
			ds.Add(delta)
		}
		done <- true
	}()

	// Goroutine 2: lê dados
	go func() {
		for i := 0; i < 100; i++ {
			_ = ds.GetAll()
			_ = ds.Size()
		}
		done <- true
	}()

	// Aguarda ambas terminarem
	<-done
	<-done

	// Verifica se não há race conditions
	if ds.Size() != 100 {
		t.Errorf("Esperado 100 deltas, obtido %d", ds.Size())
	}
}
