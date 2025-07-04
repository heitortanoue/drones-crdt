package state

import (
	"testing"
	"time"

	"github.com/heitortanoue/tcc/pkg/crdt"
)

func TestDroneStateBasic(t *testing.T) {
	// Cria estado do drone
	ds := NewDroneState("drone1")

	// Adiciona um fogo
	cell := crdt.Cell{X: 10, Y: 20}
	meta := crdt.FireMeta{
		Timestamp:  time.Now().UnixMilli(),
		Confidence: 0.9,
	}

	ds.AddFire(cell, meta)

	// Verifica se foi adicionado
	fires := ds.GetActiveFires()
	if len(fires) != 1 {
		t.Errorf("Esperado 1 fogo, encontrado %d", len(fires))
	}

	if fires[0] != cell {
		t.Errorf("Célula não corresponde: esperado %+v, encontrado %+v", cell, fires[0])
	}

	// Verifica metadados
	storedMeta, exists := ds.GetFireMeta(cell)
	if !exists {
		t.Error("Metadados não encontrados")
	}

	if storedMeta.Confidence != meta.Confidence {
		t.Errorf("Confiança não corresponde: esperado %f, encontrado %f",
			meta.Confidence, storedMeta.Confidence)
	}
}

func TestMergeDelta(t *testing.T) {
	// Cria estado do drone
	ds := NewDroneState("drone1")

	// Cria um delta simulado
	delta := crdt.FireDelta{
		Context: crdt.DotContext{
			Clock:    make(crdt.VectorClock),
			DotCloud: make(crdt.DotCloud),
		},
		Entries: []crdt.FireDeltaEntry{
			{
				Dot:  crdt.Dot{NodeID: "drone2", Counter: 1},
				Cell: crdt.Cell{X: 15, Y: 25},
				Meta: crdt.FireMeta{
					Timestamp:  time.Now().UnixMilli(),
					Confidence: 0.8,
				},
			},
		},
	}

	// Aplica o delta
	ds.MergeDelta(delta)

	// Verifica se foi aplicado
	fires := ds.GetActiveFires()
	if len(fires) != 1 {
		t.Errorf("Esperado 1 fogo após aplicar delta, encontrado %d", len(fires))
	}

	expectedCell := crdt.Cell{X: 15, Y: 25}
	if fires[0] != expectedCell {
		t.Errorf("Célula do delta não corresponde: esperado %+v, encontrado %+v",
			expectedCell, fires[0])
	}
}

func TestGlobalState(t *testing.T) {
	// Inicializa estado global
	InitGlobalState("test-drone")

	// Adiciona fogo via função global
	cell := crdt.Cell{X: 5, Y: 15}
	meta := crdt.FireMeta{
		Timestamp:  time.Now().UnixMilli(),
		Confidence: 0.7,
	}

	AddFire(cell, meta)

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

func TestRemoveFire(t *testing.T) {
	ds := NewDroneState("drone1")

	// Adiciona fogo
	cell := crdt.Cell{X: 30, Y: 40}
	meta := crdt.FireMeta{
		Timestamp:  time.Now().UnixMilli(),
		Confidence: 0.95,
	}

	ds.AddFire(cell, meta)

	// Verifica que foi adicionado
	fires := ds.GetActiveFires()
	if len(fires) != 1 {
		t.Errorf("Esperado 1 fogo antes da remoção, encontrado %d", len(fires))
	}

	// Remove o fogo
	ds.RemoveFire(cell)

	// Verifica que foi removido
	fires = ds.GetActiveFires()
	if len(fires) != 0 {
		t.Errorf("Esperado 0 fogos após remoção, encontrado %d", len(fires))
	}
}
