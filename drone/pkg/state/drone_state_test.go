package state

import (
	"testing"
	"time"

	"github.com/heitortanoue/tcc/pkg/crdt"
)

func TestProcessFireReading_HighConfidenceHighTemperature_InsertsFire(t *testing.T) {
	ds := NewDroneState("drone1")
	cell := crdt.Cell{X: 1, Y: 2}
	meta := crdt.FireMeta{
		Timestamp:   time.Now().UnixMilli(),
		Confidence:  60.0,  // acima do limiar de 50.0
		Temperature: 50.0,  // acima do limiar de 45.0
	}

	ds.ProcessFireReading(cell, meta)

	fires := ds.GetActiveFires()
	if len(fires) != 1 {
		t.Errorf("Esperado 1 fogo inserido, encontrado %d", len(fires))
	}
	if fires[0] != cell {
		t.Errorf("Célula inserida não corresponde: esperado %+v, encontrado %+v", cell, fires[0])
	}
}

func TestProcessFireReading_HighConfidenceLowTemperature_RemovesFire(t *testing.T) {
	ds := NewDroneState("drone1")
	cell := crdt.Cell{X: 3, Y: 4}
	// pré-popula o estado com um fogo para testar remoção
	ds.AddFire(cell, crdt.FireMeta{
		Timestamp:   time.Now().UnixMilli(),
		Confidence:  60.0,
		Temperature: 100.0,
	})

	meta := crdt.FireMeta{
		Timestamp:   time.Now().UnixMilli(),
		Confidence:  60.0,  // acima do limiar de 50.0
		Temperature: 40.0,  // abaixo do limiar de 45.0
	}

	ds.ProcessFireReading(cell, meta)

	fires := ds.GetActiveFires()
	if len(fires) != 0 {
		t.Errorf("Esperado 0 fogos após remoção, encontrado %d", len(fires))
	}
}

func TestProcessFireReading_LowConfidence_IgnoredWhenNoPriorFire(t *testing.T) {
	ds := NewDroneState("drone1")
	cell := crdt.Cell{X: 5, Y: 6}
	meta := crdt.FireMeta{
		Timestamp:   time.Now().UnixMilli(),
		Confidence:  10.0,  // abaixo do limiar de 50.0
		Temperature: 100.0, // mesmo assim alta temperatura, mas deve ser ignorado
	}

	ds.ProcessFireReading(cell, meta)

	fires := ds.GetActiveFires()
	if len(fires) != 0 {
		t.Errorf("Esperado 0 fogos (baixa confiança), encontrado %d", len(fires))
	}
}

func TestProcessFireReading_LowConfidence_DoesNotRemoveExistingFire(t *testing.T) {
	ds := NewDroneState("drone1")
	cell := crdt.Cell{X: 7, Y: 8}
	// pré-popula com fogo
	ds.AddFire(cell, crdt.FireMeta{
		Timestamp:   time.Now().UnixMilli(),
		Confidence:  60.0,
		Temperature: 100.0,
	})

	meta := crdt.FireMeta{
		Timestamp:   time.Now().UnixMilli(),
		Confidence:  10.0, // baixa confiança
		Temperature: 20.0, // temperatura baixa, mas leitura deve ser ignorada
	}

	ds.ProcessFireReading(cell, meta)

	fires := ds.GetActiveFires()
	if len(fires) != 1 {
		t.Errorf("Esperado 1 fogo (baixa confiança não remove), encontrado %d", len(fires))
	}
	if fires[0] != cell {
		t.Errorf("Célula existente não corresponde: esperado %+v, encontrado %+v", cell, fires[0])
	}
}

func TestGlobalState(t *testing.T) {
	// Inicializa estado global
	InitGlobalState("test-drone")

	// Adiciona fogo via função global
	cell := crdt.Cell{X: 5, Y: 15}
	meta := crdt.FireMeta{
		Timestamp:   time.Now().UnixMilli(),
		Confidence:  95,
		Temperature: 100.0,
	}

	ProcessFireReading(cell, meta)

	// Verifica via função global
	fires := GetActiveFires()
	if len(fires) != 1 {
		t.Errorf("Esperado 1 fogo no estado global, encontrado %d", len(fires))
	}

	// Verifica estatísticas
	stats := GetStats()
	if stats["active_fires"] != 1 {
		t.Errorf("Estatísticas incorretas: %+v", stats)
	}
}