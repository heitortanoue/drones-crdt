package protocol

import (
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ElectionState representa estados de eleição/transmissão
type ElectionState string

const (
	IdleState        ElectionState = "IDLE"        // Estado inativo
	TransmitterState ElectionState = "TRANSMITTER" // Estado transmissor ativo
)

// ControlSystemInterface define os métodos necessários do ControlSystem para eleição
type ControlSystemInterface interface {
	GetRequestCounters() map[uuid.UUID]int
	ResetRequestCounter(deltaID uuid.UUID)
	GetUDPSender() UDPSender
}

// Adapter para ControlSystem implementar a interface
type ControlSystemAdapter struct {
	*ControlSystem
}

func (csa *ControlSystemAdapter) GetUDPSender() UDPSender {
	return csa.udpSender
}

// TransmitterElection gerencia eleição de transmissor greedy (Base para F6)
type TransmitterElection struct {
	droneID       string
	controlSystem ControlSystemInterface

	// Estado atual
	currentState    ElectionState
	stateChanged    time.Time
	transmitTimeout time.Duration // 5s conforme requisito

	// Sincronização
	mutex sync.RWMutex

	// Configuração
	enabled bool
}

// NewTransmitterElection cria uma nova instância de eleição
func NewTransmitterElection(droneID string, controlSystem *ControlSystem) *TransmitterElection {
	return &TransmitterElection{
		droneID:         droneID,
		controlSystem:   &ControlSystemAdapter{controlSystem},
		currentState:    IdleState,
		stateChanged:    time.Now(),
		transmitTimeout: 5 * time.Second, // Requisito F6
		enabled:         true,
	}
}

// NewTransmitterElectionWithInterface cria uma nova instância de eleição com interface (para testes)
func NewTransmitterElectionWithInterface(droneID string, controlSystem ControlSystemInterface) *TransmitterElection {
	return &TransmitterElection{
		droneID:         droneID,
		controlSystem:   controlSystem,
		currentState:    IdleState,
		stateChanged:    time.Now(),
		transmitTimeout: 5 * time.Second, // Requisito F6
		enabled:         true,
	}
}

// CheckElection verifica se deve iniciar eleição baseado nos contadores ReqCtr
func (te *TransmitterElection) CheckElection() {
	if !te.enabled {
		return
	}

	te.mutex.Lock()
	defer te.mutex.Unlock()

	// Só processa se estiver em estado idle
	if te.currentState != IdleState {
		return
	}

	// Obtém contadores de request
	reqCounters := te.controlSystem.GetRequestCounters()

	// Verifica se algum contador > 0 (Requisito F6)
	for deltaID, count := range reqCounters {
		if count > 0 {
			log.Printf("[ELECTION] %s detectou demanda para delta %s (ReqCtr=%d)",
				te.droneID, deltaID.String()[:8], count)

			// Inicia processo de transmissão (Requisito F6 completo)
			te.becomeTransmitter(deltaID, count)
			break // Processa um delta por vez (greedy)
		}
	}
}

// becomeTransmitter faz transição para estado transmissor
func (te *TransmitterElection) becomeTransmitter(deltaID uuid.UUID, reqCount int) {
	log.Printf("[ELECTION] %s tornando-se transmissor para delta %s (ReqCtr=%d)",
		te.droneID, deltaID.String()[:8], reqCount)

	// Atualiza estado
	te.currentState = TransmitterState
	te.stateChanged = time.Now()

	// Envia 3x SwitchChannel com ReqCount (Requisito F6)
	te.sendSwitchChannelMessages(deltaID, reqCount)

	// Reseta contador para este delta
	te.controlSystem.ResetRequestCounter(deltaID)

	// Agenda retorno ao estado idle após timeout ou outbox vazio
	go te.scheduleStateTimeout()
}

// sendSwitchChannelMessages envia múltiplas mensagens SwitchChannel
func (te *TransmitterElection) sendSwitchChannelMessages(deltaID uuid.UUID, reqCount int) {
	// Envia 3 mensagens SwitchChannel conforme F6
	for i := 0; i < 3; i++ {
		switchMsg := SwitchChannelMsg{
			SenderID: te.droneID,
			DeltaID:  deltaID,
			ReqCount: reqCount,
		}

		// Serializa e envia via UDP
		if data, err := EncodeMessage("SWITCH_CHANNEL", switchMsg); err == nil {
			te.controlSystem.GetUDPSender().Broadcast(data)
			log.Printf("[ELECTION] %s enviou SwitchChannel #%d para delta %s (ReqCtr=%d)",
				te.droneID, i+1, deltaID.String()[:8], reqCount)
		} else {
			log.Printf("[ELECTION] Erro ao enviar SwitchChannel: %v", err)
		}

		// Pequeno delay entre envios
		if i < 2 {
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// scheduleStateTimeout agenda retorno ao estado idle
func (te *TransmitterElection) scheduleStateTimeout() {
	time.Sleep(te.transmitTimeout)

	te.mutex.Lock()
	defer te.mutex.Unlock()

	// Verifica se ainda está em estado transmissor
	if te.currentState == TransmitterState {
		log.Printf("[ELECTION] %s timeout de transmissão, voltando para idle", te.droneID)
		te.currentState = IdleState
		te.stateChanged = time.Now()
	}
}

// GetState retorna estado atual
func (te *TransmitterElection) GetState() ElectionState {
	te.mutex.RLock()
	defer te.mutex.RUnlock()
	return te.currentState
}

// GetStateInfo retorna informações detalhadas do estado
func (te *TransmitterElection) GetStateInfo() map[string]interface{} {
	te.mutex.RLock()
	defer te.mutex.RUnlock()

	return map[string]interface{}{
		"drone_id":         te.droneID,
		"current_state":    string(te.currentState),
		"state_changed":    te.stateChanged.UnixMilli(),
		"transmit_timeout": te.transmitTimeout.Seconds(),
		"enabled":          te.enabled,
	}
}

// SetEnabled habilita/desabilita eleição
func (te *TransmitterElection) SetEnabled(enabled bool) {
	te.mutex.Lock()
	defer te.mutex.Unlock()

	te.enabled = enabled
	if !enabled && te.currentState == TransmitterState {
		// Force volta ao idle se desabilitado
		te.currentState = IdleState
		te.stateChanged = time.Now()
	}
}

// ForceIdle força retorno ao estado idle (para testes)
func (te *TransmitterElection) ForceIdle() {
	te.mutex.Lock()
	defer te.mutex.Unlock()

	te.currentState = IdleState
	te.stateChanged = time.Now()
}

// GetStats retorna estatísticas da eleição (para compatibilidade)
func (te *TransmitterElection) GetStats() map[string]interface{} {
	return te.GetStateInfo()
}

// IsTransmitting verifica se está atualmente transmitindo
func (te *TransmitterElection) IsTransmitting() bool {
	te.mutex.RLock()
	defer te.mutex.RUnlock()
	return te.currentState == TransmitterState
}
