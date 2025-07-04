package state

import (
	"sync"

	"github.com/heitortanoue/tcc/pkg/crdt"
)

var (
	// Instância global do estado do drone
	globalState *DroneState
	once        sync.Once
)

// InitGlobalState inicializa o estado global do drone
// Deve ser chamado uma vez durante a inicialização da aplicação
func InitGlobalState(droneID string) {
	once.Do(func() {
		globalState = NewDroneState(droneID)
	})
}

// GetGlobalState retorna a instância global do estado
// Retorna nil se não foi inicializado
func GetState() *DroneState {
	return globalState
}

// MergeDelta aplica delta ao estado global
func MergeDelta(delta crdt.FireDelta) {
	if globalState != nil {
		globalState.MergeDelta(delta)
	}
}

// Funções de conveniência para acesso ao estado global

// AddFire adiciona fogo ao estado global
func AddFire(cell crdt.Cell, meta crdt.FireMeta) {
	if globalState != nil {
		globalState.AddFire(cell, meta)
	}
}

// RemoveFire remove fogo do estado global
func RemoveFire(cell crdt.Cell) {
	if globalState != nil {
		globalState.RemoveFire(cell)
	}
}

// GetGlobalActiveFires retorna focos ativos do estado global
func GetActiveFires() []crdt.Cell {
	if globalState != nil {
		return globalState.GetActiveFires()
	}
	return nil
}

func GetLatestReadings() map[string]crdt.FireMeta {
	if globalState != nil {
		return globalState.GetLatestReadings()
	}
	return nil
}

// GenerateGlobalDelta gera delta do estado global
func GenerateDelta() *crdt.FireDelta {
	if globalState != nil {
		return globalState.GenerateDelta()
	}
	return nil
}

// ClearGlobalDelta limpa delta do estado global
func ClearDelta() {
	if globalState != nil {
		globalState.ClearDelta()
	}
}

// GetGlobalStats retorna estatísticas do estado global
func GetStats() map[string]interface{} {
	if globalState != nil {
		return globalState.GetStats()
	}
	return map[string]interface{}{
		"error": "estado global não inicializado",
	}
}
