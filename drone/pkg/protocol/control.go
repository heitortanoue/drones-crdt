package protocol

import (
	"encoding/json"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/heitortanoue/tcc/pkg/sensor"
)

// ControlSystem gerencia o envio de mensagens HELLO
type ControlSystem struct {
	droneID   string
	sensorAPI *sensor.FireSensor
	udpSender UDPSender

	// Controle de execução
	running bool
	stopCh  chan struct{}
	mutex   sync.RWMutex
}

// UDPSender interface para envio de mensagens UDP
type UDPSender interface {
	Broadcast(data []byte)
	SendTo(data []byte, targetIP string, targetPort int) error
}

// NewControlSystem cria um novo sistema de controle
func NewControlSystem(droneID string, sensorAPI *sensor.FireSensor, udpSender UDPSender) *ControlSystem {
	return &ControlSystem{
		droneID:   droneID,
		sensorAPI: sensorAPI,
		udpSender: udpSender,
		running:   false,
		stopCh:    make(chan struct{}),
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

	// Inicia loop de mensagens HELLO
	go cs.helloLoop()
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

// helloLoop envia mensagens HELLO periodicamente
func (cs *ControlSystem) helloLoop() {
	for {
		// Intervalo aleatório entre 3-6 segundos
		randomInterval := time.Duration(3000+rand.Intn(3000)) * time.Millisecond

		select {
		case <-time.After(randomInterval):
			cs.sendHello()
		case <-cs.stopCh:
			return
		}
	}
}

// sendHello envia mensagem HELLO
func (cs *ControlSystem) sendHello() {
	// Cria mensagem HELLO
	msg := HelloMessage{
		ID:      cs.droneID,
	}

	// Serializa para JSON
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[CONTROL] Erro ao serializar HELLO: %v", err)
		return
	}

	// Broadcast via UDP
	cs.udpSender.Broadcast(data)

	log.Printf("[CONTROL] %s enviou HELLO", cs.droneID)
}

// ProcessMessage processa mensagem de controle recebida (placeholder para futuro)
func (cs *ControlSystem) ProcessMessage(data []byte, senderIP string) {
	// Por enquanto apenas registra que recebeu uma mensagem
	log.Printf("[CONTROL] %s recebeu mensagem de %s", cs.droneID, senderIP)
}

// GetStats retorna estatísticas do sistema de controle
func (cs *ControlSystem) GetStats() map[string]interface{} {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	return map[string]interface{}{
		"drone_id": cs.droneID,
		"running":  cs.running,
	}
}
