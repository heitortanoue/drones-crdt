package sensor

import (
	"fmt"
	"testing"
	"time"
)

func TestSensorDelta_Key(t *testing.T) {
	delta := SensorDelta{
		DroneID:   "drone-01",
		SensorID:  "talhao-3",
		Timestamp: 1651672800123,
		Value:     23.7,
	}

	expected := "drone-01#talhao-3#1651672800123"
	if delta.Key() != expected {
		t.Errorf("Esperado %s, obtido %s", expected, delta.Key())
	}
}

func TestSensorDelta_IsNewerThan(t *testing.T) {
	older := SensorDelta{
		SensorID:  "talhao-3",
		Timestamp: 1651672800000,
	}

	newer := SensorDelta{
		SensorID:  "talhao-3",
		Timestamp: 1651672800123,
	}

	if !newer.IsNewerThan(&older) {
		t.Error("Newer delta should be newer than older")
	}

	if older.IsNewerThan(&newer) {
		t.Error("Older delta should not be newer than newer")
	}

	// Diferentes sensores não são comparáveis
	different := SensorDelta{
		SensorID:  "talhao-4",
		Timestamp: 1651672800123,
	}

	if newer.IsNewerThan(&different) {
		t.Error("Different sensors should not be comparable")
	}
}

func TestNewSensorCRDT(t *testing.T) {
	crdt := NewSensorCRDT("drone-01")

	if crdt.droneID != "drone-01" {
		t.Errorf("Esperado drone ID 'drone-01', obtido %s", crdt.droneID)
	}

	if len(crdt.deltas) != 0 {
		t.Error("CRDT inicial deve ter zero deltas")
	}

	if len(crdt.pendingBuf) != 0 {
		t.Error("Buffer inicial deve estar vazio")
	}
}

func TestSensorCRDT_AddDelta(t *testing.T) {
	crdt := NewSensorCRDT("drone-01")
	timestamp := time.Now().UnixMilli()

	reading := SensorReading{
		SensorID:  "talhao-3",
		Timestamp: timestamp,
		Value:     23.7,
	}

	delta := crdt.AddDelta(reading)

	// Verifica o delta retornado
	if delta.DroneID != "drone-01" {
		t.Errorf("Esperado drone ID 'drone-01', obtido %s", delta.DroneID)
	}

	if delta.SensorID != "talhao-3" {
		t.Errorf("Esperado sensor ID 'talhao-3', obtido %s", delta.SensorID)
	}

	if delta.Value != 23.7 {
		t.Errorf("Esperado valor 23.7, obtido %f", delta.Value)
	}

	// Verifica se foi adicionado ao CRDT
	if len(crdt.deltas) != 1 {
		t.Errorf("Esperado 1 delta no CRDT, obtido %d", len(crdt.deltas))
	}

	// Verifica se foi adicionado aos pendentes
	pending := crdt.GetPendingDeltas()
	if len(pending) != 1 {
		t.Errorf("Esperado 1 delta pendente, obtido %d", len(pending))
	}
}

func TestSensorCRDT_Merge(t *testing.T) {
	crdt1 := NewSensorCRDT("drone-01")
	crdt2 := NewSensorCRDT("drone-02")

	// Adiciona dados ao drone-02
	reading := SensorReading{
		SensorID:  "talhao-3",
		Timestamp: time.Now().UnixMilli(),
		Value:     25.5,
	}
	delta := crdt2.AddDelta(reading)

	// Cria batch para merge
	batch := DeltaBatch{
		SenderID: "drone-02",
		Deltas:   []SensorDelta{*delta},
	}

	// Faz merge no drone-01
	mergedCount := crdt1.Merge(batch)

	if mergedCount != 1 {
		t.Errorf("Esperado 1 delta merged, obtido %d", mergedCount)
	}

	if crdt1.GetTotalDeltasCount() != 1 {
		t.Errorf("Esperado 1 delta total no CRDT1, obtido %d", crdt1.GetTotalDeltasCount())
	}
}

func TestSensorCRDT_MergeDuplicates(t *testing.T) {
	crdt := NewSensorCRDT("drone-01")
	timestamp := time.Now().UnixMilli()

	// Cria delta
	delta := SensorDelta{
		DroneID:   "drone-02",
		SensorID:  "talhao-3",
		Timestamp: timestamp,
		Value:     23.7,
	}

	batch := DeltaBatch{
		SenderID: "drone-02",
		Deltas:   []SensorDelta{delta, delta}, // duplicata
	}

	// Primeira merge
	mergedCount1 := crdt.Merge(batch)
	if mergedCount1 != 1 {
		t.Errorf("Primeira merge: esperado 1, obtido %d", mergedCount1)
	}

	// Segunda merge (mesmos deltas)
	mergedCount2 := crdt.Merge(batch)
	if mergedCount2 != 0 {
		t.Errorf("Segunda merge: esperado 0 (duplicatas), obtido %d", mergedCount2)
	}

	if crdt.GetTotalDeltasCount() != 1 {
		t.Errorf("Esperado 1 delta total, obtido %d", crdt.GetTotalDeltasCount())
	}
}

func TestSensorCRDT_LastWriterWins(t *testing.T) {
	crdt := NewSensorCRDT("drone-01")

	// Primeira leitura
	reading1 := SensorReading{
		SensorID:  "talhao-3",
		Timestamp: 1651672800000,
		Value:     20.0,
	}
	crdt.AddDelta(reading1)

	// Segunda leitura mais recente
	reading2 := SensorReading{
		SensorID:  "talhao-3",
		Timestamp: 1651672800123,
		Value:     25.0,
	}
	crdt.AddDelta(reading2)

	// Verifica LWW
	latest := crdt.GetLatestReadings()
	if latestDelta, ok := latest["talhao-3"]; !ok {
		t.Error("Não encontrou leitura para talhao-3")
	} else if latestDelta.Value != 25.0 {
		t.Errorf("Esperado valor mais recente 25.0, obtido %f", latestDelta.Value)
	}
}

func TestSensorCRDT_ConcurrentOperations(t *testing.T) {
	crdt := NewSensorCRDT("drone-01")

	// Simula operações concorrentes
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			reading := SensorReading{
				SensorID:  fmt.Sprintf("talhao-%d", id),       // sensores diferentes
				Timestamp: time.Now().UnixMilli() + int64(id), // timestamps únicos
				Value:     float64(20 + id),
			}
			crdt.AddDelta(reading)
			done <- true
		}(i)
	}

	// Espera todas as goroutines terminarem
	for i := 0; i < 10; i++ {
		<-done
	}

	if crdt.GetTotalDeltasCount() != 10 {
		t.Errorf("Esperado 10 deltas, obtido %d", crdt.GetTotalDeltasCount())
	}
}
