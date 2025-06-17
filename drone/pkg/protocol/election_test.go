package protocol

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// MockControlSystemForElection simula ControlSystem para testes de eleição
type MockControlSystemForElection struct {
	reqCounters map[uuid.UUID]int
	udpSender   UDPSender
	mutex       sync.RWMutex
}

func NewMockControlSystemForElection() *MockControlSystemForElection {
	return &MockControlSystemForElection{
		reqCounters: make(map[uuid.UUID]int),
		udpSender:   NewMockUDPSender(),
	}
}

func (m *MockControlSystemForElection) GetRequestCounters() map[uuid.UUID]int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	// Retorna cópia
	result := make(map[uuid.UUID]int)
	for k, v := range m.reqCounters {
		result[k] = v
	}
	return result
}

func (m *MockControlSystemForElection) ResetRequestCounter(deltaID uuid.UUID) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.reqCounters, deltaID)
}

func (m *MockControlSystemForElection) SetRequestCounter(deltaID uuid.UUID, count int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.reqCounters[deltaID] = count
}

func (m *MockControlSystemForElection) GetUDPSender() UDPSender {
	return m.udpSender
}

// Testes

func TestTransmitterElection_NewTransmitterElection(t *testing.T) {
	droneID := "test-election-drone"
	controlSystem := NewMockControlSystemForElection()

	election := NewTransmitterElectionWithInterface(droneID, controlSystem)

	if election == nil {
		t.Fatal("NewTransmitterElection não deveria retornar nil")
	}

	if election.droneID != droneID {
		t.Errorf("Esperado droneID %s, obtido %s", droneID, election.droneID)
	}

	if election.GetState() != IdleState {
		t.Errorf("Estado inicial deveria ser IDLE, obtido %s", election.GetState())
	}

	if election.transmitTimeout != 5*time.Second {
		t.Errorf("Timeout deveria ser 5s, obtido %v", election.transmitTimeout)
	}

	if !election.enabled {
		t.Error("Eleição deveria estar habilitada por padrão")
	}
}

func TestTransmitterElection_InitialState(t *testing.T) {
	droneID := "state-test-drone"
	controlSystem := NewMockControlSystemForElection()
	election := NewTransmitterElectionWithInterface(droneID, controlSystem)

	state := election.GetState()
	if state != IdleState {
		t.Errorf("Estado inicial deveria ser IDLE, obtido %s", state)
	}

	if election.IsTransmitting() {
		t.Error("Não deveria estar transmitindo inicialmente")
	}

	stateInfo := election.GetStateInfo()
	if stateInfo["current_state"] != "IDLE" {
		t.Errorf("Estado em GetStateInfo deveria ser IDLE, obtido %v", stateInfo["current_state"])
	}

	if stateInfo["enabled"] != true {
		t.Errorf("Eleição deveria estar habilitada, obtido %v", stateInfo["enabled"])
	}
}

func TestTransmitterElection_CheckElection_NoCounters(t *testing.T) {
	droneID := "no-counters-drone"
	controlSystem := NewMockControlSystemForElection()
	election := NewTransmitterElectionWithInterface(droneID, controlSystem)

	// Sem contadores configurados
	election.CheckElection()

	// Deve permanecer em IDLE
	if election.GetState() != IdleState {
		t.Errorf("Estado deveria permanecer IDLE, obtido %s", election.GetState())
	}
}

func TestTransmitterElection_CheckElection_WithCounters(t *testing.T) {
	droneID := "with-counters-drone"
	controlSystem := NewMockControlSystemForElection()
	election := NewTransmitterElectionWithInterface(droneID, controlSystem)

	// Configura contador > 0
	deltaID := uuid.New()
	controlSystem.SetRequestCounter(deltaID, 3)

	// Trigger eleição
	election.CheckElection()

	// Deve transicionar para TRANSMITTER
	if election.GetState() != TransmitterState {
		t.Errorf("Estado deveria ser TRANSMITTER, obtido %s", election.GetState())
	}

	if !election.IsTransmitting() {
		t.Error("Deveria estar transmitindo")
	}

	// Contador deve ter sido resetado
	counters := controlSystem.GetRequestCounters()
	if _, exists := counters[deltaID]; exists {
		t.Error("Contador deveria ter sido resetado")
	}
}

func TestTransmitterElection_DisabledElection(t *testing.T) {
	droneID := "disabled-drone"
	controlSystem := NewMockControlSystemForElection()
	election := NewTransmitterElectionWithInterface(droneID, controlSystem)

	// Desabilita eleição
	election.SetEnabled(false)

	// Configura contador
	deltaID := uuid.New()
	controlSystem.SetRequestCounter(deltaID, 5)

	// Tenta trigger eleição
	election.CheckElection()

	// Deve permanecer em IDLE
	if election.GetState() != IdleState {
		t.Errorf("Estado deveria permanecer IDLE quando desabilitado, obtido %s", election.GetState())
	}

	// Contador não deve ter sido resetado
	counters := controlSystem.GetRequestCounters()
	if count, exists := counters[deltaID]; !exists || count != 5 {
		t.Error("Contador não deveria ter sido alterado quando eleição está desabilitada")
	}
}

func TestTransmitterElection_SetEnabled_TransmitterToIdle(t *testing.T) {
	droneID := "enable-disable-drone"
	controlSystem := NewMockControlSystemForElection()
	election := NewTransmitterElectionWithInterface(droneID, controlSystem)

	// Força estado transmitter
	deltaID := uuid.New()
	controlSystem.SetRequestCounter(deltaID, 2)
	election.CheckElection()

	// Confirma que está transmitindo
	if election.GetState() != TransmitterState {
		t.Fatal("Deveria estar em estado TRANSMITTER")
	}

	// Desabilita eleição
	election.SetEnabled(false)

	// Deve forçar volta para IDLE
	if election.GetState() != IdleState {
		t.Errorf("Deveria voltar para IDLE quando desabilitado, obtido %s", election.GetState())
	}
}

func TestTransmitterElection_ForceIdle(t *testing.T) {
	droneID := "force-idle-drone"
	controlSystem := NewMockControlSystemForElection()
	election := NewTransmitterElectionWithInterface(droneID, controlSystem)

	// Força estado transmitter
	deltaID := uuid.New()
	controlSystem.SetRequestCounter(deltaID, 1)
	election.CheckElection()

	if election.GetState() != TransmitterState {
		t.Fatal("Deveria estar em estado TRANSMITTER")
	}

	// Força volta para idle
	election.ForceIdle()

	if election.GetState() != IdleState {
		t.Errorf("Deveria ter retornado para IDLE, obtido %s", election.GetState())
	}
}

func TestTransmitterElection_StateTimeout(t *testing.T) {
	droneID := "timeout-drone"
	controlSystem := NewMockControlSystemForElection()
	election := NewTransmitterElectionWithInterface(droneID, controlSystem)

	// Reduz timeout para teste rápido
	election.transmitTimeout = 100 * time.Millisecond

	// Força transição para transmitter
	deltaID := uuid.New()
	controlSystem.SetRequestCounter(deltaID, 1)
	election.CheckElection()

	if election.GetState() != TransmitterState {
		t.Fatal("Deveria estar em estado TRANSMITTER")
	}

	// Aguarda timeout
	time.Sleep(200 * time.Millisecond)

	// Deve ter voltado para IDLE
	if election.GetState() != IdleState {
		t.Errorf("Deveria ter voltado para IDLE após timeout, obtido %s", election.GetState())
	}
}

func TestTransmitterElection_GetStats(t *testing.T) {
	droneID := "stats-drone"
	controlSystem := NewMockControlSystemForElection()
	election := NewTransmitterElectionWithInterface(droneID, controlSystem)

	stats := election.GetStats()

	expectedFields := []string{"drone_id", "current_state", "state_changed", "transmit_timeout", "enabled"}
	for _, field := range expectedFields {
		if _, exists := stats[field]; !exists {
			t.Errorf("Campo %s deveria existir em GetStats", field)
		}
	}

	if stats["drone_id"] != droneID {
		t.Errorf("drone_id deveria ser %s, obtido %v", droneID, stats["drone_id"])
	}

	if stats["current_state"] != "IDLE" {
		t.Errorf("current_state deveria ser IDLE, obtido %v", stats["current_state"])
	}
}

func TestTransmitterElection_MultipleCounters(t *testing.T) {
	droneID := "multi-counter-drone"
	controlSystem := NewMockControlSystemForElection()
	election := NewTransmitterElectionWithInterface(droneID, controlSystem)

	// Configura múltiplos contadores
	delta1 := uuid.New()
	delta2 := uuid.New()
	delta3 := uuid.New()

	controlSystem.SetRequestCounter(delta1, 2)
	controlSystem.SetRequestCounter(delta2, 3)
	controlSystem.SetRequestCounter(delta3, 1)

	// Trigger eleição
	election.CheckElection()

	// Deve ter processado apenas um (greedy)
	if election.GetState() != TransmitterState {
		t.Fatal("Deveria ter transicionado para TRANSMITTER")
	}

	// Apenas um contador deve ter sido resetado
	counters := controlSystem.GetRequestCounters()
	resetCount := 0
	remainingCount := 0

	originalCounters := map[uuid.UUID]int{delta1: 2, delta2: 3, delta3: 1}
	for deltaID, originalCount := range originalCounters {
		if currentCount, exists := counters[deltaID]; exists {
			remainingCount++
			if currentCount != originalCount {
				t.Errorf("Contador para delta %s foi modificado unexpectedly", deltaID)
			}
		} else {
			resetCount++
		}
	}

	if resetCount != 1 {
		t.Errorf("Exatamente 1 contador deveria ter sido resetado, foram %d", resetCount)
	}

	if remainingCount != 2 {
		t.Errorf("2 contadores deveriam permanecer, permaneceram %d", remainingCount)
	}
}

func TestTransmitterElection_ConcurrentAccess(t *testing.T) {
	droneID := "concurrent-drone"
	controlSystem := NewMockControlSystemForElection()
	election := NewTransmitterElectionWithInterface(droneID, controlSystem)

	const numGoroutines = 10
	const numOperations = 50

	var wg sync.WaitGroup

	// Múltiplas goroutines fazendo operações concorrentes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < numOperations; j++ {
				// Operações variadas
				switch j % 4 {
				case 0:
					election.CheckElection()
				case 1:
					election.GetState()
				case 2:
					election.GetStateInfo()
				case 3:
					election.ForceIdle()
				}

				// Pequeno delay para evitar contenção excessiva
				time.Sleep(time.Microsecond)
			}
		}()
	}

	// Aguarda conclusão
	wg.Wait()

	// Verifica que o sistema está funcional
	finalState := election.GetState()
	if finalState != IdleState && finalState != TransmitterState {
		t.Errorf("Estado final deveria ser válido, obtido %s", finalState)
	}
}

func TestTransmitterElection_CheckElection_AlreadyTransmitting(t *testing.T) {
	droneID := "already-transmitting-drone"
	controlSystem := NewMockControlSystemForElection()
	election := NewTransmitterElectionWithInterface(droneID, controlSystem)

	// Primeiro trigger
	delta1 := uuid.New()
	controlSystem.SetRequestCounter(delta1, 1)
	election.CheckElection()

	if election.GetState() != TransmitterState {
		t.Fatal("Deveria estar em TRANSMITTER após primeiro trigger")
	}

	// Segundo trigger com outro delta
	delta2 := uuid.New()
	controlSystem.SetRequestCounter(delta2, 5)
	election.CheckElection()

	// Deve permanecer em TRANSMITTER e não processar o segundo
	if election.GetState() != TransmitterState {
		t.Error("Deveria permanecer em TRANSMITTER")
	}

	// Segundo contador não deve ter sido resetado
	counters := controlSystem.GetRequestCounters()
	if count, exists := counters[delta2]; !exists || count != 5 {
		t.Error("Segundo contador não deveria ter sido processado")
	}
}

func TestTransmitterElection_StateChangedTimestamp(t *testing.T) {
	droneID := "timestamp-drone"
	controlSystem := NewMockControlSystemForElection()
	election := NewTransmitterElectionWithInterface(droneID, controlSystem)

	// Captura timestamp inicial
	initialInfo := election.GetStateInfo()
	initialTimestamp := initialInfo["state_changed"].(int64)

	// Aguarda um pouco
	time.Sleep(10 * time.Millisecond)

	// Trigger transição
	deltaID := uuid.New()
	controlSystem.SetRequestCounter(deltaID, 1)
	election.CheckElection()

	// Verifica que timestamp mudou
	newInfo := election.GetStateInfo()
	newTimestamp := newInfo["state_changed"].(int64)

	if newTimestamp <= initialTimestamp {
		t.Error("Timestamp deveria ter mudado após transição de estado")
	}
}
