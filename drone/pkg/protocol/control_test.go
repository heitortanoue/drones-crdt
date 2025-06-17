package protocol

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/heitortanoue/tcc/pkg/sensor"
)

// Mock implementations para testes

// MockSendError implementa error para testes
type MockSendError struct {
	URL string
}

func (e *MockSendError) Error() string {
	return "mock send error to " + e.URL
}

type MockSensorAPI struct {
	allDeltaIDs     []uuid.UUID
	missingDeltas   []uuid.UUID
	availableDeltas []sensor.SensorDelta
	mutex           sync.RWMutex
}

func NewMockSensorAPI() *MockSensorAPI {
	return &MockSensorAPI{
		allDeltaIDs:     make([]uuid.UUID, 0),
		missingDeltas:   make([]uuid.UUID, 0),
		availableDeltas: make([]sensor.SensorDelta, 0),
	}
}

func (m *MockSensorAPI) GetAllDeltaIDs() []uuid.UUID {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make([]uuid.UUID, len(m.allDeltaIDs))
	copy(result, m.allDeltaIDs)
	return result
}

func (m *MockSensorAPI) GetMissingDeltas(haveIDs []uuid.UUID) []uuid.UUID {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make([]uuid.UUID, len(m.missingDeltas))
	copy(result, m.missingDeltas)
	return result
}

func (m *MockSensorAPI) GetDeltasByIDs(wantedIDs []uuid.UUID) []sensor.SensorDelta {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make([]sensor.SensorDelta, len(m.availableDeltas))
	copy(result, m.availableDeltas)
	return result
}

func (m *MockSensorAPI) SetAllDeltaIDs(ids []uuid.UUID) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.allDeltaIDs = ids
}

func (m *MockSensorAPI) SetMissingDeltas(ids []uuid.UUID) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.missingDeltas = ids
}

func (m *MockSensorAPI) SetAvailableDeltas(deltas []sensor.SensorDelta) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.availableDeltas = deltas
}

type MockUDPSender struct {
	broadcastMessages [][]byte
	sentMessages      []SentMessage
	shouldFail        bool
	mutex             sync.RWMutex
}

type SentMessage struct {
	Data       []byte
	TargetIP   string
	TargetPort int
}

func NewMockUDPSender() *MockUDPSender {
	return &MockUDPSender{
		broadcastMessages: make([][]byte, 0),
		sentMessages:      make([]SentMessage, 0),
	}
}

func (m *MockUDPSender) Broadcast(data []byte) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.broadcastMessages = append(m.broadcastMessages, data)
}

func (m *MockUDPSender) SendTo(data []byte, targetIP string, targetPort int) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.shouldFail {
		return &MockSendError{URL: targetIP}
	}

	m.sentMessages = append(m.sentMessages, SentMessage{
		Data:       data,
		TargetIP:   targetIP,
		TargetPort: targetPort,
	})
	return nil
}

func (m *MockUDPSender) GetBroadcastMessages() [][]byte {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make([][]byte, len(m.broadcastMessages))
	copy(result, m.broadcastMessages)
	return result
}

func (m *MockUDPSender) GetSentMessages() []SentMessage {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	result := make([]SentMessage, len(m.sentMessages))
	copy(result, m.sentMessages)
	return result
}

func (m *MockUDPSender) SetShouldFail(fail bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.shouldFail = fail
}

func (m *MockUDPSender) Reset() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.broadcastMessages = make([][]byte, 0)
	m.sentMessages = make([]SentMessage, 0)
	m.shouldFail = false
}

// Testes

func TestControlSystem_NewControlSystem(t *testing.T) {
	droneID := "test-drone"
	sensorAPI := NewMockSensorAPI()
	udpSender := NewMockUDPSender()

	cs := NewControlSystem(droneID, sensorAPI, udpSender)

	if cs == nil {
		t.Fatal("NewControlSystem não deveria retornar nil")
	}

	if cs.droneID != droneID {
		t.Errorf("Esperado droneID %s, obtido %s", droneID, cs.droneID)
	}

	if cs.running {
		t.Error("ControlSystem não deveria estar running na criação")
	}

	if cs.advertiseInterval != 5*time.Second {
		t.Errorf("Esperado advertiseInterval 5s, obtido %v", cs.advertiseInterval)
	}

	if len(cs.reqCounters) != 0 {
		t.Errorf("reqCounters deveria estar vazio, obtido %d itens", len(cs.reqCounters))
	}
}

func TestControlSystem_StartStop(t *testing.T) {
	droneID := "test-drone"
	sensorAPI := NewMockSensorAPI()
	udpSender := NewMockUDPSender()
	cs := NewControlSystem(droneID, sensorAPI, udpSender)

	// Sistema deve começar parado
	stats := cs.GetStats()
	if running, ok := stats["running"].(bool); !ok || running {
		t.Error("Sistema deveria começar parado")
	}

	// Inicia sistema
	cs.Start()
	stats = cs.GetStats()
	if running, ok := stats["running"].(bool); !ok || !running {
		t.Error("Sistema deveria estar rodando após Start")
	}

	// Start múltiplos não devem causar problema
	cs.Start()
	stats = cs.GetStats()
	if running, ok := stats["running"].(bool); !ok || !running {
		t.Error("Sistema deveria continuar rodando após múltiplos Start")
	}

	// Para sistema
	cs.Stop()
	stats = cs.GetStats()
	if running, ok := stats["running"].(bool); !ok || running {
		t.Error("Sistema deveria estar parado após Stop")
	}

	// Stop múltiplos não devem causar problema
	cs.Stop()
	stats = cs.GetStats()
	if running, ok := stats["running"].(bool); !ok || running {
		t.Error("Sistema deveria continuar parado após múltiplos Stop")
	}
}

func TestControlSystem_ProcessMessage_IgnoreSelf(t *testing.T) {
	droneID := "test-drone"
	sensorAPI := NewMockSensorAPI()
	udpSender := NewMockUDPSender()
	cs := NewControlSystem(droneID, sensorAPI, udpSender)

	// Cria mensagem do próprio drone
	msg := CreateAdvertiseMessage(droneID, []uuid.UUID{uuid.New()})
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Erro ao serializar mensagem: %v", err)
	}

	// Processa mensagem (deve ser ignorada)
	cs.ProcessMessage(data, "127.0.0.1")

	// Verifica que nenhuma ação foi tomada
	broadcasts := udpSender.GetBroadcastMessages()
	sent := udpSender.GetSentMessages()

	if len(broadcasts) != 0 {
		t.Errorf("Não deveria ter broadcasts para mensagem própria, obtido %d", len(broadcasts))
	}

	if len(sent) != 0 {
		t.Errorf("Não deveria ter mensagens enviadas para mensagem própria, obtido %d", len(sent))
	}
}

func TestControlSystem_ProcessMessage_Advertise(t *testing.T) {
	droneID := "test-drone"
	senderID := "remote-drone"
	sensorAPI := NewMockSensorAPI()
	udpSender := NewMockUDPSender()
	cs := NewControlSystem(droneID, sensorAPI, udpSender)

	// Configura IDs que estão ausentes
	advertiseID := uuid.New()
	missingID := uuid.New()
	sensorAPI.SetMissingDeltas([]uuid.UUID{missingID})

	// Cria mensagem Advertise
	data := map[string]interface{}{
		"sender_id": senderID,
		"have_ids":  []interface{}{advertiseID.String()},
	}

	msg := ControlMessage{
		Type:      AdvertiseType,
		SenderID:  senderID,
		Timestamp: getCurrentTimestamp(),
		Data:      data,
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Erro ao serializar mensagem: %v", err)
	}

	// Processa mensagem
	senderIP := "192.168.1.100"
	cs.ProcessMessage(msgData, senderIP)

	// Verifica que Request foi enviado
	sent := udpSender.GetSentMessages()
	if len(sent) != 1 {
		t.Errorf("Esperado 1 mensagem Request enviada, obtido %d", len(sent))
	}

	if len(sent) > 0 {
		if sent[0].TargetIP != senderIP {
			t.Errorf("Request deveria ser enviado para %s, obtido %s", senderIP, sent[0].TargetIP)
		}

		if sent[0].TargetPort != 7000 {
			t.Errorf("Request deveria ser enviado para porta 7000, obtido %d", sent[0].TargetPort)
		}
	}

	// Verifica que contadores foram incrementados
	counters := cs.GetRequestCounters()
	if len(counters) == 0 {
		t.Error("Contadores deveriam ter sido incrementados")
	}
}

func TestControlSystem_ProcessMessage_Request(t *testing.T) {
	droneID := "test-drone"
	senderID := "remote-drone"
	sensorAPI := NewMockSensorAPI()
	udpSender := NewMockUDPSender()
	cs := NewControlSystem(droneID, sensorAPI, udpSender)

	// Configura deltas disponíveis
	requestedID := uuid.New()
	availableDelta := sensor.SensorDelta{
		ID:        requestedID,
		SensorID:  "test-sensor",
		Value:     42.5,
		Timestamp: time.Now().UnixMilli(),
		DroneID:   droneID,
	}
	sensorAPI.SetAvailableDeltas([]sensor.SensorDelta{availableDelta})

	// Cria mensagem Request
	data := map[string]interface{}{
		"sender_id":  senderID,
		"wanted_ids": []interface{}{requestedID.String()},
	}

	msg := ControlMessage{
		Type:      RequestType,
		SenderID:  senderID,
		Timestamp: getCurrentTimestamp(),
		Data:      data,
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Erro ao serializar mensagem: %v", err)
	}

	// Processa mensagem
	cs.ProcessMessage(msgData, "192.168.1.100")

	// O método apenas loga que enviaria os deltas
	// Em uma implementação completa, verificaríamos se os deltas foram enviados via TCP
	// Por enquanto, verificamos que não houve erros
}

func TestControlSystem_ProcessMessage_SwitchChannel(t *testing.T) {
	droneID := "test-drone"
	senderID := "remote-drone"
	sensorAPI := NewMockSensorAPI()
	udpSender := NewMockUDPSender()
	cs := NewControlSystem(droneID, sensorAPI, udpSender)

	deltaID := uuid.New()

	// Cria mensagem SwitchChannel
	data := map[string]interface{}{
		"sender_id": senderID,
		"delta_id":  deltaID.String(),
	}

	msg := ControlMessage{
		Type:      SwitchChannelType,
		SenderID:  senderID,
		Timestamp: getCurrentTimestamp(),
		Data:      data,
	}

	msgData, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Erro ao serializar mensagem: %v", err)
	}

	// Processa mensagem
	cs.ProcessMessage(msgData, "192.168.1.100")

	// O método apenas loga a recepção da mensagem
	// Em implementação completa, haveria coordenação de transmissão
}

func TestControlSystem_ProcessMessage_InvalidJSON(t *testing.T) {
	droneID := "test-drone"
	sensorAPI := NewMockSensorAPI()
	udpSender := NewMockUDPSender()
	cs := NewControlSystem(droneID, sensorAPI, udpSender)

	// JSON inválido
	invalidJSON := []byte(`{"invalid": json}`)

	// Processa mensagem (deve ser ignorada)
	cs.ProcessMessage(invalidJSON, "127.0.0.1")

	// Verifica que nenhuma ação foi tomada
	broadcasts := udpSender.GetBroadcastMessages()
	sent := udpSender.GetSentMessages()

	if len(broadcasts) != 0 {
		t.Errorf("Não deveria ter broadcasts para JSON inválido, obtido %d", len(broadcasts))
	}

	if len(sent) != 0 {
		t.Errorf("Não deveria ter mensagens enviadas para JSON inválido, obtido %d", len(sent))
	}
}

func TestControlSystem_RequestCounters(t *testing.T) {
	droneID := "test-drone"
	sensorAPI := NewMockSensorAPI()
	udpSender := NewMockUDPSender()
	cs := NewControlSystem(droneID, sensorAPI, udpSender)

	deltaID1 := uuid.New()
	deltaID2 := uuid.New()

	// Testa incremento de contadores
	cs.IncrementRequestCounter(deltaID1)
	cs.IncrementRequestCounter(deltaID1)
	cs.IncrementRequestCounter(deltaID2)

	counters := cs.GetRequestCounters()
	if len(counters) != 2 {
		t.Errorf("Esperado 2 contadores, obtido %d", len(counters))
	}

	if counters[deltaID1] != 2 {
		t.Errorf("Contador para deltaID1 esperado 2, obtido %d", counters[deltaID1])
	}

	if counters[deltaID2] != 1 {
		t.Errorf("Contador para deltaID2 esperado 1, obtido %d", counters[deltaID2])
	}

	// Testa reset de contador
	cs.ResetRequestCounter(deltaID1)
	counters = cs.GetRequestCounters()

	if len(counters) != 1 {
		t.Errorf("Após reset, esperado 1 contador, obtido %d", len(counters))
	}

	if _, exists := counters[deltaID1]; exists {
		t.Error("Contador para deltaID1 deveria ter sido removido")
	}

	if counters[deltaID2] != 1 {
		t.Errorf("Contador para deltaID2 deveria permanecer 1, obtido %d", counters[deltaID2])
	}

	// Testa reset de contador inexistente
	nonExistentID := uuid.New()
	cs.ResetRequestCounter(nonExistentID) // Não deve causar erro
}

func TestControlSystem_GetStats(t *testing.T) {
	droneID := "test-drone"
	sensorAPI := NewMockSensorAPI()
	udpSender := NewMockUDPSender()
	cs := NewControlSystem(droneID, sensorAPI, udpSender)

	deltaID := uuid.New()
	cs.IncrementRequestCounter(deltaID)

	stats := cs.GetStats()
	if stats == nil {
		t.Fatal("GetStats não deveria retornar nil")
	}

	if droneIDStat, ok := stats["drone_id"].(string); !ok || droneIDStat != droneID {
		t.Errorf("Esperado drone_id %s, obtido %v", droneID, stats["drone_id"])
	}

	if running, ok := stats["running"].(bool); !ok || running {
		t.Errorf("Esperado running false, obtido %v", stats["running"])
	}

	if intervalSec, ok := stats["advertise_interval"].(float64); !ok || intervalSec != 5.0 {
		t.Errorf("Esperado advertise_interval 5.0, obtido %v", stats["advertise_interval"])
	}

	if reqCounters, ok := stats["req_counters"].(int); !ok || reqCounters != 1 {
		t.Errorf("Esperado req_counters 1, obtido %v", stats["req_counters"])
	}
}

func TestControlSystem_ConcurrentAccess(t *testing.T) {
	droneID := "test-drone"
	sensorAPI := NewMockSensorAPI()
	udpSender := NewMockUDPSender()
	cs := NewControlSystem(droneID, sensorAPI, udpSender)

	var wg sync.WaitGroup
	numOperations := 50

	// Operações concorrentes de incremento
	wg.Add(numOperations)
	for i := 0; i < numOperations; i++ {
		go func(id int) {
			defer wg.Done()
			deltaID := uuid.New()
			cs.IncrementRequestCounter(deltaID)
		}(i)
	}

	// Operações concorrentes de leitura
	wg.Add(numOperations)
	for i := 0; i < numOperations; i++ {
		go func() {
			defer wg.Done()
			cs.GetRequestCounters()
			cs.GetStats()
		}()
	}

	wg.Wait()

	// Verifica que sistema não está corrompido
	stats := cs.GetStats()
	if stats == nil {
		t.Fatal("Stats não deveria ser nil após operações concorrentes")
	}

	counters := cs.GetRequestCounters()
	if len(counters) != numOperations {
		t.Errorf("Esperado %d contadores após operações concorrentes, obtido %d", numOperations, len(counters))
	}
}
