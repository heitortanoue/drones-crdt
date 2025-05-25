package logging

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/heitortanoue/tcc/sensor"
)

// DroneLogger gerencia logs estruturados do drone
type DroneLogger struct {
	droneID string
	logger  *log.Logger
}

// NewDroneLogger cria um novo logger para o drone
func NewDroneLogger(droneID string) *DroneLogger {
	logger := log.New(os.Stdout, fmt.Sprintf("[%s] ", droneID), log.LstdFlags|log.Lmicroseconds)
	return &DroneLogger{
		droneID: droneID,
		logger:  logger,
	}
}

// LogSensorReading registra uma nova leitura de sensor
func (l *DroneLogger) LogSensorReading(delta *sensor.SensorDelta) {
	l.logger.Printf("SENSOR_ADD: sensor=%s value=%.2f timestamp=%d received_at=%d",
		delta.SensorID, delta.Value, delta.Timestamp, time.Now().UnixMilli())
}

// LogDeltaReceived registra recebimento de deltas de peers
func (l *DroneLogger) LogDeltaReceived(senderID string, deltas []sensor.SensorDelta, mergedCount int) {
	receivedAt := time.Now().UnixMilli()
	l.logger.Printf("DELTA_RECEIVED: sender=%s total_deltas=%d merged=%d received_at=%d",
		senderID, len(deltas), mergedCount, receivedAt)

	// Log individual dos deltas merged
	for _, delta := range deltas {
		l.logger.Printf("DELTA_DETAIL: from=%s sensor=%s value=%.2f original_ts=%d received_at=%d",
			delta.DroneID, delta.SensorID, delta.Value, delta.Timestamp, receivedAt)
	}
}

// LogGossipSent registra envio de gossip para peers
func (l *DroneLogger) LogGossipSent(peerURL string, deltaCount int, success bool) {
	status := "SUCCESS"
	if !success {
		status = "FAILED"
	}
	l.logger.Printf("GOSSIP_SENT: peer=%s deltas=%d status=%s sent_at=%d",
		peerURL, deltaCount, status, time.Now().UnixMilli())
}

// LogGossipReceived registra recebimento via gossip
func (l *DroneLogger) LogGossipReceived(deltaCount int) {
	l.logger.Printf("GOSSIP_RECEIVED: deltas=%d received_at=%d",
		deltaCount, time.Now().UnixMilli())
}

// LogGossipEvent registra eventos gerais de gossip (handshakes, etc.)
func (l *DroneLogger) LogGossipEvent(message string) {
	l.logger.Printf("GOSSIP_EVENT: %s event_at=%d", message, time.Now().UnixMilli())
}

// LogStateSnapshot registra snapshot do estado atual
func (l *DroneLogger) LogStateSnapshot(totalDeltas int, uniqueSensors int) {
	l.logger.Printf("STATE_SNAPSHOT: total_deltas=%d unique_sensors=%d snapshot_at=%d",
		totalDeltas, uniqueSensors, time.Now().UnixMilli())
}

// LogPeerJoin registra entrada de novo peer
func (l *DroneLogger) LogPeerJoin(peerID string) {
	l.logger.Printf("PEER_JOIN: peer=%s joined_at=%d",
		peerID, time.Now().UnixMilli())
}

// LogPeerLeave registra saída de peer
func (l *DroneLogger) LogPeerLeave(peerID string) {
	l.logger.Printf("PEER_LEAVE: peer=%s left_at=%d",
		peerID, time.Now().UnixMilli())
}

// LogConflictResolution registra resolução de conflitos
func (l *DroneLogger) LogConflictResolution(sensorID string, oldValue, newValue float64, reason string) {
	l.logger.Printf("CONFLICT_RESOLVED: sensor=%s old_value=%.2f new_value=%.2f reason=%s resolved_at=%d",
		sensorID, oldValue, newValue, reason, time.Now().UnixMilli())
}

// LogError registra erros
func (l *DroneLogger) LogError(operation string, err error) {
	l.logger.Printf("ERROR: operation=%s error=%s occurred_at=%d",
		operation, err.Error(), time.Now().UnixMilli())
}

// LogMetrics registra métricas de performance
func (l *DroneLogger) LogMetrics(operation string, duration time.Duration, count int) {
	l.logger.Printf("METRICS: operation=%s duration_ms=%.2f count=%d ops_per_sec=%.2f measured_at=%d",
		operation, float64(duration.Microseconds())/1000.0, count,
		float64(count)/duration.Seconds(), time.Now().UnixMilli())
}
