package protocol

import (
	"encoding/json"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/heitortanoue/tcc/pkg/sensor"
)

// ControlSystem manages the sending of HELLO messages
type ControlSystem struct {
	droneID   string
	sensorAPI *sensor.FireSensor
	udpSender UDPSender
	
	// Hello message configuration
	helloInterval time.Duration
	helloJitter   time.Duration

	// Execution control
	running bool
	stopCh  chan struct{}
	mutex   sync.RWMutex
}

// UDPSender interface for sending UDP messages
type UDPSender interface {
	Broadcast(data []byte)
	SendTo(data []byte, targetIP string, targetPort int) error
}

// NewControlSystem creates a new control system
func NewControlSystem(droneID string, sensorAPI *sensor.FireSensor, udpSender UDPSender, helloInterval, helloJitter time.Duration) *ControlSystem {
	return &ControlSystem{
		droneID:       droneID,
		sensorAPI:     sensorAPI,
		udpSender:     udpSender,
		helloInterval: helloInterval,
		helloJitter:   helloJitter,
		running:       false,
		stopCh:        make(chan struct{}),
	}
}

// Start launches the control system
func (cs *ControlSystem) Start() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if cs.running {
		return
	}

	cs.running = true
	log.Printf("[CONTROL] Starting control system for %s", cs.droneID)

	// Start HELLO message loop
	go cs.helloLoop()
}

// Stop shuts down the control system
func (cs *ControlSystem) Stop() {
	cs.mutex.Lock()
	defer cs.mutex.Unlock()

	if !cs.running {
		return
	}

	cs.running = false
	close(cs.stopCh)
	log.Printf("[CONTROL] Stopping control system for %s", cs.droneID)
}

// helloLoop periodically sends HELLO messages
func (cs *ControlSystem) helloLoop() {
	for {
		// Calculate random interval: baseInterval ± jitter
		// jitter is a random value in [-helloJitter, +helloJitter]
		jitter := time.Duration(rand.Int63n(int64(cs.helloJitter)*2)) - cs.helloJitter
		randomInterval := cs.helloInterval + jitter

		select {
		case <-time.After(randomInterval):
			cs.sendHello()
		case <-cs.stopCh:
			return
		}
	}
}

// sendHello sends a HELLO message
func (cs *ControlSystem) sendHello() {
	// Create HELLO message
	msg := HelloMessage{
		ID: cs.droneID,
	}

	// Serialize to JSON
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[CONTROL] Error serializing HELLO: %v", err)
		return
	}

	// Broadcast via UDP
	cs.udpSender.Broadcast(data)

	log.Printf("[CONTROL] %s sent HELLO", cs.droneID)
}

// ProcessMessage processes a received control message (placeholder for future use)
func (cs *ControlSystem) ProcessMessage(data []byte, senderIP string) {
	// For now, just log that a message was received
	log.Printf("[CONTROL] %s received message from %s", cs.droneID, senderIP)
}

// GetStats returns statistics from the control system
func (cs *ControlSystem) GetStats() map[string]interface{} {
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()

	return map[string]interface{}{
		"drone_id": cs.droneID,
		"running":  cs.running,
	}
}
