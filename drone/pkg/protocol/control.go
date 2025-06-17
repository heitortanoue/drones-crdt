package protocol

import (
	"encoding/json"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/heitortanoue/tcc/pkg/sensor"
)

// ControlSystem gerencia protocolos de controle (Requisito F3)
type ControlSystem struct {
	droneID   string
	sensorAPI *sensor.SensorAPI
	udpSender UDPSender // Interface para envio UDP

	// Contadores para eleição (base para F6)
	reqCounters map[uuid.UUID]int // ReqCtr[id] para cada delta

	// Controle de execução
	running bool
	stopCh  chan struct{}
	mutex   sync.RWMutex

	// Configuração
	advertiseInterval time.Duration // 3-6s aleatório
}

// UDPSender interface para envio de mensagens UDP
type UDPSender interface {
	Broadcast(data []byte)
	SendTo(data []byte, targetIP string, targetPort int) error
}

// NewControlSystem cria um novo sistema de controle
func NewControlSystem(droneID string, sensorAPI *sensor.SensorAPI, udpSender UDPSender) *ControlSystem {
	return &ControlSystem{
		droneID:           droneID,
		sensorAPI:         sensorAPI,
		udpSender:         udpSender,
		reqCounters:       make(map[uuid.UUID]int),
		running:           false,
		stopCh:            make(chan struct{}),
		advertiseInterval: 5 * time.Second, // Média entre 3-6s
	}
}

// Start inicia o sistema de controle
func (cs *ControlSystem) Start() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if cs.running {
		return
	}

	cs.running = true
	log.Printf("[CONTROL] Iniciando sistema de controle para %s", cs.droneID)

	// Inicia loop de advertise
	go cs.advertiseLoop()
}

// Stop para o sistema de controle
func (cs *ControlSystem) Stop() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if !cs.running {
		return
	}

	cs.running = false
	close(cs.stopCh)
	log.Printf("[CONTROL] Parando sistema de controle para %s", cs.droneID)
}

// advertiseLoop envia mensagens Advertise periodicamente
func (cs *ControlSystem) advertiseLoop() {
	for {
		// Intervalo aleatório entre 3-6 segundos (Requisito F3)
		randomInterval := time.Duration(3000+rand.Intn(3000)) * time.Millisecond

		select {
		case <-time.After(randomInterval):
			cs.sendAdvertise()
		case <-cs.stopCh:
			return
		}
	}
}

// sendAdvertise envia mensagem Advertise com deltas disponíveis
func (cs *ControlSystem) sendAdvertise() {
	// Obtém todos os IDs de deltas disponíveis
	deltaIDs := cs.sensorAPI.GetAllDeltaIDs()

	if len(deltaIDs) == 0 {
		return // Nada para anunciar
	}

	// Cria mensagem Advertise
	msg := CreateAdvertiseMessage(cs.droneID, deltaIDs)

	// Serializa para JSON
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[CONTROL] Erro ao serializar Advertise: %v", err)
		return
	}

	// Broadcast via UDP
	cs.udpSender.Broadcast(data)

	log.Printf("[CONTROL] %s enviou Advertise com %d deltas", cs.droneID, len(deltaIDs))
}

// ProcessMessage processa mensagem de controle recebida
func (cs *ControlSystem) ProcessMessage(data []byte, senderIP string) {
	var msg ControlMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Printf("[CONTROL] Erro ao decodificar mensagem: %v", err)
		return
	}

	// Ignora mensagens próprias
	if msg.SenderID == cs.droneID {
		return
	}

	log.Printf("[CONTROL] %s recebeu %s de %s", cs.droneID, msg.Type, msg.SenderID)

	switch msg.Type {
	case AdvertiseType:
		cs.handleAdvertise(msg, senderIP)
	case RequestType:
		cs.handleRequest(msg, senderIP)
	case SwitchChannelType:
		cs.handleSwitchChannel(msg, senderIP)
	default:
		log.Printf("[CONTROL] Tipo de mensagem desconhecido: %s", msg.Type)
	}
}

// handleAdvertise processa mensagem Advertise
func (cs *ControlSystem) handleAdvertise(msg ControlMessage, senderIP string) {
	advertise, ok := ParseAdvertiseMessage(msg)
	if !ok {
		log.Printf("[CONTROL] Erro ao parsear Advertise de %s", msg.SenderID)
		return
	}

	// Identifica deltas que não possuímos
	missingIDs := cs.sensorAPI.GetMissingDeltas(advertise.HaveIDs)

	if len(missingIDs) > 0 {
		// Envia Request para deltas ausentes
		cs.sendRequest(missingIDs, senderIP)

		// Incrementa contadores para eleição (base F6)
		cs.mutex.Lock()
		for _, id := range missingIDs {
			cs.reqCounters[id]++
		}
		cs.mutex.Unlock()

		log.Printf("[CONTROL] %s solicitou %d deltas de %s", cs.droneID, len(missingIDs), msg.SenderID)
	}
}

// handleRequest processa mensagem Request
func (cs *ControlSystem) handleRequest(msg ControlMessage, senderIP string) {
	request, ok := ParseRequestMessage(msg)
	if !ok {
		log.Printf("[CONTROL] Erro ao parsear Request de %s", msg.SenderID)
		return
	}

	// Obtém deltas solicitados que possuímos
	availableDeltas := cs.sensorAPI.GetDeltasByIDs(request.WantedIDs)

	if len(availableDeltas) > 0 {
		log.Printf("[CONTROL] %s atendendo Request de %s com %d deltas",
			cs.droneID, msg.SenderID, len(availableDeltas))

		// Por enquanto, loga que enviaria os deltas
		// Na Fase 4 implementaremos a transmissão via TCP
		for _, delta := range availableDeltas {
			log.Printf("[CONTROL] Enviaria delta %s para %s",
				delta.ID.String()[:8], msg.SenderID)
		}
	}
}

// handleSwitchChannel processa mensagem SwitchChannel (base para F6)
func (cs *ControlSystem) handleSwitchChannel(msg ControlMessage, senderIP string) {
	switchMsg, ok := ParseSwitchChannelMessage(msg)
	if !ok {
		log.Printf("[CONTROL] Erro ao parsear SwitchChannel de %s", msg.SenderID)
		return
	}

	log.Printf("[CONTROL] %s recebeu SwitchChannel para delta %s de %s",
		cs.droneID, switchMsg.DeltaID.String()[:8], msg.SenderID)

	// Base para coordenação de transmissão (será expandido na Fase 4)
}

// sendRequest envia mensagem Request para deltas específicos
func (cs *ControlSystem) sendRequest(wantedIDs []uuid.UUID, targetIP string) {
	msg := CreateRequestMessage(cs.droneID, wantedIDs)

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[CONTROL] Erro ao serializar Request: %v", err)
		return
	}

	// Envia diretamente para o IP de origem (unicast)
	if err := cs.udpSender.SendTo(data, targetIP, 7000); err != nil {
		log.Printf("[CONTROL] Erro ao enviar Request: %v", err)
	}
}

// GetStats retorna estatísticas do sistema de controle
func (cs *ControlSystem) GetStats() map[string]interface{} {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	return map[string]interface{}{
		"drone_id":           cs.droneID,
		"running":            cs.running,
		"advertise_interval": cs.advertiseInterval.Seconds(),
		"req_counters":       len(cs.reqCounters),
	}
}

// GetRequestCounters retorna contadores de request (base para F6)
func (cs *ControlSystem) GetRequestCounters() map[uuid.UUID]int {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	counters := make(map[uuid.UUID]int)
	for id, count := range cs.reqCounters {
		counters[id] = count
	}

	return counters
}
