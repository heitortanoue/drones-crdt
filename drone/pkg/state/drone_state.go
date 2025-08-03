package state

import (
	"log"
	"sync"

	"github.com/heitortanoue/tcc/pkg/crdt"
)

// DroneState mantém o estado atual do drone incluindo detecções de fogo
type DroneState struct {
	droneID string

	// CRDT para células com fogo detectado
	fires *crdt.AWORSet[crdt.Cell]

	// Metadados das células (mapeamento Dot -> FireMeta)
	metadata map[crdt.Dot]crdt.FireMeta

	// Controle de concorrência
	mutex sync.RWMutex
}

// NewDroneState cria uma nova instância do estado do drone
func NewDroneState(droneID string) *DroneState {
	return &DroneState{
		droneID:  droneID,
		fires:    crdt.NewAWORSet[crdt.Cell](),
		metadata: make(map[crdt.Dot]crdt.FireMeta),
	}
}

func (ds *DroneState) ProcessFireReading(cell crdt.Cell, meta crdt.FireMeta) {
	const temperatureThreshold = 45.0
	const confidenceThreshold = 50.0

	if meta.Confidence < confidenceThreshold {
		log.Printf("[PROCESS] Leitura com baixa confiança: %.1f%% - ignorada", meta.Confidence)
		return
	}

	if meta.Temperature >= temperatureThreshold {
		ds.AddFire(cell, meta)

		log.Printf("[PROCESS] Fogo detectado em: (%d,%d) T=%.1f°C, confiança=%.1f%% → ADICIONADO",
			cell.X, cell.Y, meta.Temperature, meta.Confidence)
	} else {
		ds.RemoveFire(cell)

		log.Printf("[PROCESS] Atividade fraca em: (%d,%d) T=%.1f°C, confiança=%.1f%% → REMOVIDO",
			cell.X, cell.Y, meta.Temperature, meta.Confidence)
	}
}

// AddFire adiciona uma nova detecção de fogo ao estado local
func (ds *DroneState) AddFire(cell crdt.Cell, meta crdt.FireMeta) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	// Gera um novo dot e adiciona ao CRDT
	if ds.fires.Delta == nil {
		ds.fires.Delta = crdt.NewDotKernel[crdt.Cell]()
	}

	dot := ds.fires.Core.Context.NextDot(ds.droneID)
	ds.fires.Core.Entries[dot] = cell
	ds.fires.Delta.Entries[dot] = cell

	// Armazena metadados
	ds.metadata[dot] = meta

	log.Printf("[STATE] Adicionada detecção de fogo em (%d, %d) com dot %s",
		cell.X, cell.Y, dot.String()[:8])
}

// RemoveFire remove uma célula do estado (quando fogo é extinto)
func (ds *DroneState) RemoveFire(cell crdt.Cell) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	ds.fires.Remove(cell)

	// Remove metadados das células removidas
	for dot, storedCell := range ds.fires.Core.Entries {
		if storedCell == cell {
			delete(ds.metadata, dot)
		}
	}

	log.Printf("[STATE] Removida detecção de fogo em (%d, %d)", cell.X, cell.Y)
}

// MergeDelta aplica um delta recebido de outro drone
func (ds *DroneState) MergeDelta(delta crdt.FireDelta) {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	// log.Printf("[STATE] Aplicando delta recebido: %s", delta.String())

	// 1) Reconstrói kernel temporário do delta
	kernel := &crdt.DotKernel[crdt.Cell]{
		Context: &delta.Context,
		Entries: make(map[crdt.Dot]crdt.Cell, len(delta.Entries)),
	}

	// 2) Preenche o mapa Dot→Cell e armazena metadados
	for _, entry := range delta.Entries {
		kernel.Entries[entry.Dot] = entry.Cell
		ds.metadata[entry.Dot] = entry.Meta
	}

	// 3) Aplica merge apenas do estado CRDT
	ds.fires.MergeDelta(kernel)

	log.Printf("[STATE] Aplicado delta com %d entradas", len(delta.Entries))
	log.Printf("[STATE] Estado atualizado: %d células ativas", len(ds.fires.Core.Entries))
}

// GenerateDelta gera um delta das mudanças locais para disseminação
func (ds *DroneState) GenerateDelta() *crdt.FireDelta {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	if ds.fires.Delta == nil || len(ds.fires.Delta.Entries) == 0 {
		log.Printf("[STATE] Nenhum delta para enviar (nenhuma mudança local)")
		return nil // Nenhuma mudança local
	}

	// Constrói o delta para envio
	delta := &crdt.FireDelta{
		Context: *ds.fires.Delta.Context,
		Entries: make([]crdt.FireDeltaEntry, 0, len(ds.fires.Delta.Entries)),
	}

	for dot, cell := range ds.fires.Delta.Entries {
		meta, exists := ds.metadata[dot]
		if !exists {
			// Metadados padrão se não encontrados
			meta = crdt.FireMeta{
				Timestamp:  0,
				Confidence: 1.0,
			}
		}

		delta.Entries = append(delta.Entries, crdt.FireDeltaEntry{
			Dot:  dot,
			Cell: cell,
			Meta: meta,
		})
	}

	return delta
}

// ClearDelta limpa o delta após disseminação
func (ds *DroneState) ClearDelta() {
	ds.mutex.Lock()
	defer ds.mutex.Unlock()

	ds.fires.Delta = nil
}

// GetActiveFires retorna todas as células ativas com fogo
func (ds *DroneState) GetActiveFires() []crdt.Cell {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	return ds.fires.Elements()
}

func (ds *DroneState) GetLatestReadings() map[string]crdt.FireMeta {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	// Cria um mapa para armazenar as leituras mais recentes
	latestReadings := make(map[string]crdt.FireMeta)

	// Itera sobre os metadados e seleciona o mais recente por célula
	for dot, meta := range ds.metadata {
		if existingMeta, exists := latestReadings[dot.NodeID]; !exists || meta.Timestamp > existingMeta.Timestamp {
			latestReadings[dot.NodeID] = meta
		}
	}

	return latestReadings
}

// GetFireMeta retorna os metadados de uma célula específica
func (ds *DroneState) GetFireMeta(cell crdt.Cell) (crdt.FireMeta, bool) {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	// Procura a célula nos entries ativos
	for dot, storedCell := range ds.fires.Core.Entries {
		if storedCell == cell {
			if meta, exists := ds.metadata[dot]; exists {
				return meta, true
			}
		}
	}

	return crdt.FireMeta{}, false
}

// GetStats retorna estatísticas do estado
func (ds *DroneState) GetStats() map[string]interface{} {
	ds.mutex.RLock()
	defer ds.mutex.RUnlock()

	return map[string]interface{}{
		"drone_id":          ds.droneID,
		"active_fires":      len(ds.fires.Core.Entries),
		"metadata_count":    len(ds.metadata),
		"has_pending_delta": ds.fires.Delta != nil,
	}
}

// GetDroneID retorna o ID do drone
func (ds *DroneState) GetDroneID() string {
	return ds.droneID
}
